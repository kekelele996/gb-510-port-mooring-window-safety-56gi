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

type weatherWindowService struct {
	repository repository.WeatherWindowRepository
	clearances repository.SafetyClearanceRepository
	security   SecurityService
}

func NewWeatherWindowService(repo repository.WeatherWindowRepository, clearances repository.SafetyClearanceRepository, security SecurityService) WeatherWindowService {
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
	item := model.WeatherWindow{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.WeatherWindowInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), ExpiresAt: input.ExpiresAt.UTC(),
		Evidence: strings.TrimSpace(input.Evidence), RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
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
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.ExpiresAt = input.ExpiresAt.UTC()
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
	if target == "restricted" || target == "expired" {
		if err := s.invalidatePendingClearances(ctx, current, target, actor, requestID); err != nil {
			return model.WeatherWindow{}, err
		}
	}
	return s.repository.Get(ctx, id)
}

// invalidatePendingClearances voids every pending clearance in the same
// operational area that references this window, recording why it expired.
func (s *weatherWindowService) invalidatePendingClearances(ctx context.Context, window model.WeatherWindow, target, actor, requestID string) error {
	pending, err := s.clearances.ListPendingByWindow(ctx, window.Facility, window.Code)
	if err != nil {
		return fmt.Errorf("list pending clearances for window %s: %w", window.Code, err)
	}
	if len(pending) == 0 {
		return nil
	}
	stateLabel := "受限"
	if target == "expired" {
		stateLabel = "过期"
	}
	reason := fmt.Sprintf("关联风浪窗口 %s 已%s，安全许可自动失效，需重新提交", window.Code, stateLabel)
	for _, item := range pending {
		before := item.Status
		expected := item.Version
		item.Status = string(constants.ClearanceStateExpired)
		item.InvalidReason = reason
		item.Version = expected + 1
		item.UpdatedAt = time.Now().UTC()
		if err := s.clearances.Update(ctx, item.ID, expected, &item); err != nil {
			return fmt.Errorf("expire pending clearance %d: %w", item.ID, err)
		}
		if err := s.security.AuditWithWindowVersion(ctx, actor, requestID, "clearance_window_invalidated", "SafetyClearance", item.ID, before, item.Status, reason, window.Version); err != nil {
			return fmt.Errorf("persist clearance invalidation audit: %w", err)
		}
	}
	return nil
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
