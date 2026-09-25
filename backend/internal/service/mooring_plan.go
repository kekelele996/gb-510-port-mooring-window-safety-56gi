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

type MooringPlanService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.MooringPlan], error)
	Get(context.Context, uint) (model.MooringPlan, error)
	Create(context.Context, dto.CreateMooringPlan, string, string) (model.MooringPlan, error)
	Update(context.Context, uint, dto.UpdateMooringPlan, string, string) (model.MooringPlan, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.MooringPlan, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type mooringPlanService struct {
	repository repository.MooringPlanRepository
	security   SecurityService
}

func NewMooringPlanService(repo repository.MooringPlanRepository, security SecurityService) MooringPlanService {
	return &mooringPlanService{repository: repo, security: security}
}

func (s *mooringPlanService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.MooringPlan], error) {
	return s.repository.List(ctx, query)
}

func (s *mooringPlanService) Get(ctx context.Context, id uint) (model.MooringPlan, error) {
	return s.repository.Get(ctx, id)
}

func (s *mooringPlanService) Create(ctx context.Context, input dto.CreateMooringPlan, actor, requestID string) (model.MooringPlan, error) {
	if err := validateMooringPlanBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.MooringPlan{}, err
	}
	item := model.MooringPlan{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.MooringPlanInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.MooringPlan{}, fmt.Errorf("create 系泊方案: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "MooringPlan", item.ID, "", item.Status, "created 系泊方案")
	return item, nil
}

func (s *mooringPlanService) Update(ctx context.Context, id uint, input dto.UpdateMooringPlan, actor, requestID string) (model.MooringPlan, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.MooringPlan{}, err
	}
	if err := validateMooringPlanBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.MooringPlan{}, err
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
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.MooringPlan{}, fmt.Errorf("update 系泊方案: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "MooringPlan", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *mooringPlanService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.MooringPlan, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.MooringPlan{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.MooringPlanTransitions, current.Status, target) {
		return model.MooringPlan{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.MooringPlan{}, fmt.Errorf("transition 系泊方案: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "MooringPlan", id, before, target, input.Reason); err != nil {
		return model.MooringPlan{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *mooringPlanService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "MooringPlan", id, current.Status, "deleted", "soft deleted 系泊方案")
}

func (s *mooringPlanService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateMooringPlanBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
