package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/constants"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/model"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/repository"
	"gorm.io/gorm"
)

type SafetyClearanceService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.SafetyClearance], error)
	Get(context.Context, uint) (model.SafetyClearance, error)
	Create(context.Context, dto.CreateSafetyClearance, string, string) (model.SafetyClearance, error)
	Update(context.Context, uint, dto.UpdateSafetyClearance, string, string) (model.SafetyClearance, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string, string) (model.SafetyClearance, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type safetyClearanceService struct {
	repository repository.SafetyClearanceRepository
	windows    repository.WeatherWindowRepository
	security   SecurityService
}

func NewSafetyClearanceService(repo repository.SafetyClearanceRepository, windows repository.WeatherWindowRepository, security SecurityService) SafetyClearanceService {
	return &safetyClearanceService{repository: repo, windows: windows, security: security}
}

func (s *safetyClearanceService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.SafetyClearance], error) {
	return s.repository.List(ctx, query)
}

func (s *safetyClearanceService) Get(ctx context.Context, id uint) (model.SafetyClearance, error) {
	return s.repository.Get(ctx, id)
}

func (s *safetyClearanceService) Create(ctx context.Context, input dto.CreateSafetyClearance, actor, requestID string) (model.SafetyClearance, error) {
	if err := validateSafetyClearanceBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.SafetyClearance{}, err
	}
	windowVersion := input.WindowVersion
	if windowVersion == 0 {
		windowVersion = 1
	}
	item := model.SafetyClearance{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.SafetyClearanceInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode:   strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
		WindowVersion: windowVersion,
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("create 安全许可: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "SafetyClearance", item.ID, "", item.Status, "created 安全许可")
	return item, nil
}

func (s *safetyClearanceService) Update(ctx context.Context, id uint, input dto.UpdateSafetyClearance, actor, requestID string) (model.SafetyClearance, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.SafetyClearance{}, err
	}
	if err := validateSafetyClearanceBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.SafetyClearance{}, err
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
	if input.WindowVersion > 0 && input.WindowVersion != current.WindowVersion {
		current.WindowVersion = input.WindowVersion
		current.SubmittedBy = ""
		current.SubmittedAt = nil
		current.ConfirmedBy = ""
		current.ConfirmedAt = nil
	}
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("update 安全许可: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "SafetyClearance", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *safetyClearanceService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.SafetyClearance, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.SafetyClearance{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.SafetyClearanceTransitions, current.Status, target) {
		return model.SafetyClearance{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	if current.Status == string(constants.ClearanceStatePending) && target == string(constants.ClearanceStateCleared) {
		return s.confirmClearance(ctx, current, input, actor, role, requestID)
	}
	if role != model.RoleReviewer && role != model.RoleAdmin {
		return model.SafetyClearance{}, ErrReviewerRequired
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("transition 安全许可: %w", err)
	}
	if err := s.security.AuditWithWindowVersion(ctx, actor, requestID, "transition", "SafetyClearance", id, before, target, input.Reason, current.WindowVersion); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

// confirmClearance drives the two-person safety rule. The first call freezes
// the linked weather window version and expiry onto the clearance; the second
// call re-reads the live window and refuses to release when the window was
// revised, restricted or expired. A call with WindowVersion == 0 resubmits
// against the current window after such a rejection.
func (s *safetyClearanceService) confirmClearance(ctx context.Context, current model.SafetyClearance, input dto.TransitionRequest, actor, role, requestID string) (model.SafetyClearance, error) {
	now := time.Now().UTC()
	if current.SubmittedBy == "" || input.WindowVersion == 0 {
		action := "clearance_submit"
		if current.SubmittedBy != "" {
			action = "clearance_resubmit"
		}
		return s.submitClearance(ctx, current, input, actor, requestID, action, now)
	}
	if current.WindowVersion != input.WindowVersion {
		return model.SafetyClearance{}, ErrWindowVersion
	}
	window, err := s.linkedWindow(ctx, current)
	if err != nil {
		return model.SafetyClearance{}, err
	}
	if window.Version != current.WindowVersion {
		return model.SafetyClearance{}, ErrWindowVersion
	}
	if err := ensureWindowClearable(window, now); err != nil {
		return model.SafetyClearance{}, err
	}
	if current.SubmittedBy == actor {
		return model.SafetyClearance{}, ErrSelfApproval
	}
	if role != model.RoleReviewer && role != model.RoleAdmin {
		return model.SafetyClearance{}, ErrReviewerRequired
	}
	before := current.Status
	current.Status = string(constants.ClearanceStateCleared)
	current.ConfirmedBy = actor
	current.ConfirmedAt = &now
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	if err := s.repository.Update(ctx, current.ID, input.ExpectedVersion, &current); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("confirm safety clearance: %w", err)
	}
	if err := s.security.AuditWithWindowVersion(ctx, actor, requestID, "clearance_confirm", "SafetyClearance", current.ID, before, current.Status, input.Reason, current.WindowVersion); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("persist safety confirmation audit: %w", err)
	}
	return s.repository.Get(ctx, current.ID)
}

// submitClearance freezes the live window version and expiry onto the
// clearance and records who submitted. It serves both the first submission
// and resubmission after the window was revised, restricted or expired.
func (s *safetyClearanceService) submitClearance(ctx context.Context, current model.SafetyClearance, input dto.TransitionRequest, actor, requestID, action string, now time.Time) (model.SafetyClearance, error) {
	window, err := s.linkedWindow(ctx, current)
	if err != nil {
		return model.SafetyClearance{}, err
	}
	if err := ensureWindowClearable(window, now); err != nil {
		return model.SafetyClearance{}, err
	}
	current.WindowVersion = window.Version
	current.WindowExpiresAt = windowExpiresAt(window)
	current.SubmittedBy = actor
	current.SubmittedAt = &now
	current.ConfirmedBy = ""
	current.ConfirmedAt = nil
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	if err := s.repository.Update(ctx, current.ID, input.ExpectedVersion, &current); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("submit safety confirmation: %w", err)
	}
	if err := s.security.AuditWithWindowVersion(ctx, actor, requestID, action, "SafetyClearance", current.ID, current.Status, current.Status, input.Reason, current.WindowVersion); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("persist safety submission audit: %w", err)
	}
	return s.repository.Get(ctx, current.ID)
}

// linkedWindow resolves the weather window a clearance depends on through the
// operational area (facility) and the window code (relatedCode).
func (s *safetyClearanceService) linkedWindow(ctx context.Context, item model.SafetyClearance) (model.WeatherWindow, error) {
	code := strings.ToUpper(strings.TrimSpace(item.RelatedCode))
	if code == "" {
		return model.WeatherWindow{}, ErrWindowLinked
	}
	window, err := s.windows.FindByCode(ctx, code, item.Facility)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.WeatherWindow{}, ErrWindowLinked
		}
		return model.WeatherWindow{}, fmt.Errorf("load linked weather window: %w", err)
	}
	return window, nil
}

// ensureWindowClearable rejects windows that are restricted, expired by
// status, or past their expiry time.
func ensureWindowClearable(window model.WeatherWindow, now time.Time) error {
	if window.Status == "restricted" || window.Status == "expired" {
		return ErrWindowState
	}
	if !window.ExpiresAt.IsZero() && !window.ExpiresAt.After(now) {
		return ErrWindowExpired
	}
	return nil
}

func windowExpiresAt(window model.WeatherWindow) *time.Time {
	if window.ExpiresAt.IsZero() {
		return nil
	}
	expiresAt := window.ExpiresAt
	return &expiresAt
}

func (s *safetyClearanceService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "SafetyClearance", id, current.Status, "deleted", "soft deleted 安全许可")
}

func (s *safetyClearanceService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateSafetyClearanceBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
