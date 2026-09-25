package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/constants"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/model"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/repository"
)

type WeatherWindowService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.WeatherWindow], error)
	Get(context.Context, uint) (model.WeatherWindow, error)
	Create(context.Context, dto.CreateWeatherWindow, string, string) (model.WeatherWindow, error)
	Update(context.Context, uint, dto.UpdateWeatherWindow, string, string) (model.WeatherWindow, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.WeatherWindow, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

// ClearanceInvalidator is the safety-clearance side of the window link. It is
// defined here to keep the window aggregate driving the cascade without a
// circular service dependency.
type ClearanceInvalidator interface {
	InvalidatePendingForWindow(ctx context.Context, facility, code, target, reason, actor, requestID string) error
}

type weatherWindowService struct {
	repository repository.WeatherWindowRepository
	clearances ClearanceInvalidator
	security   SecurityService
}

func NewWeatherWindowService(repo repository.WeatherWindowRepository, clearances ClearanceInvalidator, security SecurityService) WeatherWindowService {
	return &weatherWindowService{repository: repo, clearances: clearances, security: security}
}

func (s *weatherWindowService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.WeatherWindow], error) {
	return s.repository.List(ctx, query)
}

func (s *weatherWindowService) Get(ctx context.Context, id uint) (model.WeatherWindow, error) {
	return s.repository.Get(ctx, id)
}

func (s *weatherWindowService) Create(ctx context.Context, input dto.CreateWeatherWindow, actor, requestID string) (model.WeatherWindow, error) {
	if err := validateWeatherWindowBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.WeatherWindow{}, err
	}
	if !input.ExpireAt.UTC().After(input.EffectiveAt.UTC()) {
		return model.WeatherWindow{}, fmt.Errorf("%w: expireAt must be after effectiveAt", ErrInvalidInput)
	}
	item := model.WeatherWindow{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.WeatherWindowInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), ExpireAt: input.ExpireAt.UTC(),
		Evidence:    strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.WeatherWindow{}, fmt.Errorf("create 风浪窗口: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "WeatherWindow", item.ID, "", item.Status, "created 风浪窗口")
	return item, nil
}

func (s *weatherWindowService) Update(ctx context.Context, id uint, input dto.UpdateWeatherWindow, actor, requestID string) (model.WeatherWindow, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.WeatherWindow{}, err
	}
	if err := validateWeatherWindowBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.WeatherWindow{}, err
	}
	if !input.ExpireAt.UTC().After(input.EffectiveAt.UTC()) {
		return model.WeatherWindow{}, fmt.Errorf("%w: expireAt must be after effectiveAt", ErrInvalidInput)
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.ExpireAt = input.ExpireAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.WeatherWindow{}, fmt.Errorf("update 风浪窗口: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "WeatherWindow", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *weatherWindowService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.WeatherWindow, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.WeatherWindow{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.WeatherWindowTransitions, current.Status, target) {
		return model.WeatherWindow{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.WeatherWindow{}, fmt.Errorf("transition 风浪窗口: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "WeatherWindow", id, before, target, input.Reason); err != nil {
		return model.WeatherWindow{}, fmt.Errorf("persist transition audit: %w", err)
	}
	// 窗口变成受限或过期后，同区域（facility）通过窗口编码关联的待处理许可立即失效并写明原因。
	if target == string(constants.ClearanceStateRestricted) || target == string(constants.ClearanceStateExpired) {
		reason := fmt.Sprintf("关联窗口 %s 已%s，待处理许可自动失效，请重新提交", current.Code, targetText(target))
		if err := s.clearances.InvalidatePendingForWindow(ctx, current.Facility, current.Code, target, reason, actor, requestID); err != nil {
			return model.WeatherWindow{}, err
		}
	}
	return s.repository.Get(ctx, id)
}

func targetText(target string) string {
	if target == "restricted" {
		return "受限"
	}
	if target == "expired" {
		return "过期"
	}
	return target
}

func (s *weatherWindowService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "WeatherWindow", id, current.Status, "deleted", "soft deleted 风浪窗口")
}

func (s *weatherWindowService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateWeatherWindowBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
