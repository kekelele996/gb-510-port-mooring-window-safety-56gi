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

type VesselCallService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.VesselCall], error)
	Get(context.Context, uint) (model.VesselCall, error)
	Create(context.Context, dto.CreateVesselCall, string, string) (model.VesselCall, error)
	Update(context.Context, uint, dto.UpdateVesselCall, string, string) (model.VesselCall, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.VesselCall, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type vesselCallService struct {
	repository repository.VesselCallRepository
	security   SecurityService
}

func NewVesselCallService(repo repository.VesselCallRepository, security SecurityService) VesselCallService {
	return &vesselCallService{repository: repo, security: security}
}

func (s *vesselCallService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.VesselCall], error) {
	return s.repository.List(ctx, query)
}

func (s *vesselCallService) Get(ctx context.Context, id uint) (model.VesselCall, error) {
	return s.repository.Get(ctx, id)
}

func (s *vesselCallService) Create(ctx context.Context, input dto.CreateVesselCall, actor, requestID string) (model.VesselCall, error) {
	if err := validateVesselCallBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.VesselCall{}, err
	}
	item := model.VesselCall{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.VesselCallInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.VesselCall{}, fmt.Errorf("create 船舶靠泊: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "VesselCall", item.ID, "", item.Status, "created 船舶靠泊")
	return item, nil
}

func (s *vesselCallService) Update(ctx context.Context, id uint, input dto.UpdateVesselCall, actor, requestID string) (model.VesselCall, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.VesselCall{}, err
	}
	if err := validateVesselCallBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.VesselCall{}, err
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
		return model.VesselCall{}, fmt.Errorf("update 船舶靠泊: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "VesselCall", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *vesselCallService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.VesselCall, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.VesselCall{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.VesselCallTransitions, current.Status, target) {
		return model.VesselCall{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.VesselCall{}, fmt.Errorf("transition 船舶靠泊: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "VesselCall", id, before, target, input.Reason); err != nil {
		return model.VesselCall{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *vesselCallService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "VesselCall", id, current.Status, "deleted", "soft deleted 船舶靠泊")
}

func (s *vesselCallService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateVesselCallBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
