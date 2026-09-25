package service

import (
	"context"
	"fmt"
	"log/slog"
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
	InvalidatePendingForWindow(context.Context, string, string, string, string, string, string) error
}

type safetyClearanceService struct {
	repository repository.SafetyClearanceRepository
	windows    repository.WeatherWindowRepository
	security   SecurityService
	logger     *slog.Logger
}

func NewSafetyClearanceService(repo repository.SafetyClearanceRepository, windows repository.WeatherWindowRepository, security SecurityService, logger *slog.Logger) SafetyClearanceService {
	if logger == nil {
		logger = slog.Default()
	}
	return &safetyClearanceService{repository: repo, windows: windows, security: security, logger: logger}
}

func (s *safetyClearanceService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.SafetyClearance], error) {
	if err := s.invalidateTimeExpired(ctx, "system"); err != nil {
		s.logger.Warn("lazy invalidate expired clearances failed", "error", err)
	}
	return s.repository.List(ctx, query)
}

func (s *safetyClearanceService) Get(ctx context.Context, id uint) (model.SafetyClearance, error) {
	if err := s.invalidateTimeExpired(ctx, "system"); err != nil {
		s.logger.Warn("lazy invalidate expired clearances failed", "error", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *safetyClearanceService) Create(ctx context.Context, input dto.CreateSafetyClearance, actor, requestID string) (model.SafetyClearance, error) {
	if err := validateSafetyClearanceBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.SafetyClearance{}, err
	}
	facility := strings.TrimSpace(input.Facility)
	windowCode := strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	window, err := s.loadUsableWindow(ctx, facility, windowCode, time.Now().UTC())
	if err != nil {
		return model.SafetyClearance{}, err
	}
	expireAt := window.ExpireAt.UTC()
	item := model.SafetyClearance{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.SafetyClearanceInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: facility, Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: windowCode, WindowVersion: window.Version, WindowExpireAt: &expireAt,
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("create 安全许可: %w", err)
	}
	_ = s.security.AuditWithWindowVersion(ctx, actor, requestID, "create", "SafetyClearance", item.ID, "", item.Status,
		fmt.Sprintf("created 安全许可 linked to window %s v%d, valid until %s", windowCode, window.Version, expireAt.Format(time.RFC3339)), item.WindowVersion)
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
	facility := strings.TrimSpace(input.Facility)
	windowCode := strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	beforeFacility := current.Facility
	beforeWindowCode := current.RelatedCode
	window, err := s.loadUsableWindow(ctx, facility, windowCode, time.Now().UTC())
	if err != nil {
		return model.SafetyClearance{}, err
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = facility
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	// 关联窗口变化（换窗口、换区域）后，旧版本上的提交与复核结论全部作废，需要重新提交。
	if beforeFacility != facility || beforeWindowCode != windowCode {
		current.RelatedCode = windowCode
		current.WindowVersion = window.Version
		expireAt := window.ExpireAt.UTC()
		current.WindowExpireAt = &expireAt
		current.InvalidReason = ""
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
	_ = s.security.AuditWithWindowVersion(ctx, actor, requestID, "update", "SafetyClearance", id, current.Status, current.Status,
		"updated business fields", current.WindowVersion)
	return s.repository.Get(ctx, id)
}

func (s *safetyClearanceService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.SafetyClearance, error) {
	if err := s.invalidateTimeExpired(ctx, "system"); err != nil {
		s.logger.Warn("lazy invalidate expired clearances failed", "error", err)
	}
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.SafetyClearance{}, err
	}
	target := strings.TrimSpace(input.Status)
	if current.Status == string(constants.ClearanceStatePending) && target == string(constants.ClearanceStateCleared) {
		return s.confirmClearance(ctx, current, input, actor, role, requestID)
	}
	if target == string(constants.ClearanceStatePending) {
		// 重新提交可能来自 pending（窗口换版后提交结论作废）、restricted/expired
		// （窗口失效后重新关联）或 cleared（人工收回重审），因此先于状态图判断。
		return s.resubmitClearance(ctx, current, input, actor, role, requestID)
	}
	if !constants.CanTransition(constants.SafetyClearanceTransitions, current.Status, target) {
		return model.SafetyClearance{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	if role != model.RoleReviewer && role != model.RoleAdmin {
		return model.SafetyClearance{}, ErrReviewerRequired
	}
	before := current.Status
	current.Status = target
	if target == string(constants.ClearanceStateRestricted) || target == string(constants.ClearanceStateExpired) {
		if reason := strings.TrimSpace(input.Reason); reason != "" {
			current.InvalidReason = truncateReason(reason)
		}
	}
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

// resubmitClearance re-freezes a restricted/expired clearance against the
// current safe window and puts it back to pending for the two-person flow.
func (s *safetyClearanceService) resubmitClearance(ctx context.Context, current model.SafetyClearance, input dto.TransitionRequest, actor, role, requestID string) (model.SafetyClearance, error) {
	if role != model.RoleOperator && role != model.RoleAdmin {
		return model.SafetyClearance{}, ErrOperatorRequired
	}
	if current.Status == string(constants.ClearanceStateCleared) {
		return model.SafetyClearance{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, constants.ClearanceStatePending)
	}
	if current.Status == string(constants.ClearanceStatePending) && current.SubmittedBy != "" && strings.TrimSpace(current.InvalidReason) == "" {
		return model.SafetyClearance{}, fmt.Errorf("%w: pending clearance already submitted, withdraw it before resubmitting", ErrInvalidTransition)
	}
	now := time.Now().UTC()
	window, err := s.loadUsableWindow(ctx, current.Facility, current.RelatedCode, now)
	if err != nil {
		return model.SafetyClearance{}, err
	}
	before := current.Status
	current.Status = string(constants.ClearanceStatePending)
	current.WindowVersion = window.Version
	expireAt := window.ExpireAt.UTC()
	current.WindowExpireAt = &expireAt
	current.InvalidReason = ""
	current.SubmittedBy = ""
	current.SubmittedAt = nil
	current.ConfirmedBy = ""
	current.ConfirmedAt = nil
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	if err := s.repository.Update(ctx, current.ID, input.ExpectedVersion, &current); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("resubmit safety clearance: %w", err)
	}
	if err := s.security.AuditWithWindowVersion(ctx, actor, requestID, "clearance_resubmit", "SafetyClearance", current.ID, before, current.Status,
		fmt.Sprintf("resubmitted against window %s v%d, valid until %s", current.RelatedCode, window.Version, expireAt.Format(time.RFC3339)), current.WindowVersion); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("persist resubmission audit: %w", err)
	}
	return s.repository.Get(ctx, current.ID)
}

func (s *safetyClearanceService) confirmClearance(ctx context.Context, current model.SafetyClearance, input dto.TransitionRequest, actor, role, requestID string) (model.SafetyClearance, error) {
	now := time.Now().UTC()
	// 首次提交：重新读取关联窗口，固化当时的版本与有效期。
	if current.SubmittedBy == "" {
		window, err := s.loadUsableWindow(ctx, current.Facility, current.RelatedCode, now)
		if err != nil {
			return model.SafetyClearance{}, err
		}
		current.WindowVersion = window.Version
		expireAt := window.ExpireAt.UTC()
		current.WindowExpireAt = &expireAt
		current.SubmittedBy = actor
		current.SubmittedAt = &now
		current.InvalidReason = ""
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = now
		if err := s.repository.Update(ctx, current.ID, input.ExpectedVersion, &current); err != nil {
			return model.SafetyClearance{}, fmt.Errorf("submit safety confirmation: %w", err)
		}
		if err := s.security.AuditWithWindowVersion(ctx, actor, requestID, "clearance_submit", "SafetyClearance", current.ID, current.Status, current.Status,
			fmt.Sprintf("submitted against window %s v%d, valid until %s", current.RelatedCode, window.Version, expireAt.Format(time.RFC3339)), current.WindowVersion); err != nil {
			return model.SafetyClearance{}, fmt.Errorf("persist safety submission audit: %w", err)
		}
		return s.repository.Get(ctx, current.ID)
	}
	// 复核放行：再次读取窗口，换版/受限/过期一律拒绝并要求重新提交。
	window, err := s.windows.FindByFacilityCode(ctx, current.Facility, current.RelatedCode)
	if err != nil {
		return s.rejectConfirmation(ctx, current, actor, requestID, "expired",
			fmt.Sprintf("关联窗口 %s 已不存在，拒绝放行，请重新提交", current.RelatedCode),
			fmt.Errorf("re-read linked window before release: %w", err))
	}
	if window.Status == "restricted" {
		return s.rejectConfirmation(ctx, current, actor, requestID, "restricted",
			fmt.Sprintf("关联窗口 %s 已受限，拒绝放行，请重新提交", current.RelatedCode), ErrWindowRestricted)
	}
	if window.Status == "expired" || window.ExpireAt.Before(now) {
		return s.rejectConfirmation(ctx, current, actor, requestID, "expired",
			fmt.Sprintf("关联窗口 %s 已过期，拒绝放行，请重新提交", current.RelatedCode), ErrWindowExpired)
	}
	if current.WindowVersion != window.Version {
		// 换版不改变许可状态，但提交结论作废，必须按新版本重新走双人确认。
		return s.invalidateSubmission(ctx, current,
			fmt.Errorf("%w: clearance froze v%d, window %s is now v%d", ErrWindowVersion, current.WindowVersion, current.RelatedCode, window.Version),
			fmt.Sprintf("关联窗口 %s 已换版（v%d → v%d），请重新提交", current.RelatedCode, current.WindowVersion, window.Version),
			actor, requestID)
	}
	if !window.UsableForClearance(now) {
		return s.rejectConfirmation(ctx, current, actor, requestID, "restricted",
			fmt.Sprintf("关联窗口 %s 当前不可用，拒绝放行，请重新提交", current.RelatedCode), ErrWindowNotSafe)
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

// rejectConfirmation marks the pending clearance restricted/expired with an
// explanatory reason (window restricted, expired or missing) and discards the
// stale submission, so it must be resubmitted before it can be released.
func (s *safetyClearanceService) rejectConfirmation(ctx context.Context, current model.SafetyClearance, actor, requestID, target, reason string, cause error) (model.SafetyClearance, error) {
	before := current.Status
	current.Status = target
	current.InvalidReason = truncateReason(reason)
	current.SubmittedBy = ""
	current.SubmittedAt = nil
	current.ConfirmedBy = ""
	current.ConfirmedAt = nil
	current.Version++
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, current.ID, current.Version-1, &current); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("invalidate safety clearance after window check: %w", err)
	}
	auditReason := reason
	if auditErr := s.security.AuditWithWindowVersion(ctx, actor, requestID, "clearance_rejected", "SafetyClearance", current.ID, before, target, auditReason, current.WindowVersion); auditErr != nil {
		return model.SafetyClearance{}, fmt.Errorf("persist rejection audit: %w", auditErr)
	}
	return model.SafetyClearance{}, cause
}

// invalidateSubmission discards only the two-person submission (keeping the
// clearance pending) when the frozen window version no longer matches.
func (s *safetyClearanceService) invalidateSubmission(ctx context.Context, current model.SafetyClearance, cause error, reason, actor, requestID string) (model.SafetyClearance, error) {
	current.SubmittedBy = ""
	current.SubmittedAt = nil
	current.ConfirmedBy = ""
	current.ConfirmedAt = nil
	current.InvalidReason = truncateReason(reason)
	current.Version++
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, current.ID, current.Version-1, &current); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("reset stale safety submission: %w", err)
	}
	if err := s.security.AuditWithWindowVersion(ctx, actor, requestID, "clearance_window_drift", "SafetyClearance", current.ID, current.Status, current.Status, reason, current.WindowVersion); err != nil {
		return model.SafetyClearance{}, fmt.Errorf("persist window drift audit: %w", err)
	}
	return model.SafetyClearance{}, cause
}

// loadUsableWindow re-reads the window linked by facility + code and ensures it
// may back a clearance action right now.
func (s *safetyClearanceService) loadUsableWindow(ctx context.Context, facility, code string, now time.Time) (model.WeatherWindow, error) {
	window, err := s.windows.FindByFacilityCode(ctx, facility, code)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.WeatherWindow{}, ErrWindowNotFound
		}
		return model.WeatherWindow{}, fmt.Errorf("load linked weather window: %w", err)
	}
	if window.Status == "restricted" {
		return model.WeatherWindow{}, ErrWindowRestricted
	}
	if window.Status == "expired" || window.ExpireAt.Before(now) {
		return model.WeatherWindow{}, ErrWindowExpired
	}
	if !window.UsableForClearance(now) {
		return model.WeatherWindow{}, ErrWindowNotSafe
	}
	return window, nil
}

// InvalidatePendingForWindow cascades a window becoming restricted/expired to
// every pending clearance in the same facility linked by window code.
func (s *safetyClearanceService) InvalidatePendingForWindow(ctx context.Context, facility, code, target, reason, actor, requestID string) error {
	pending, err := s.repository.ListPendingByWindow(ctx, facility, code)
	if err != nil {
		return fmt.Errorf("list pending clearances for window %s: %w", code, err)
	}
	for _, clearance := range pending {
		before := clearance.Status
		clearance.Status = target
		clearance.InvalidReason = truncateReason(reason)
		clearance.SubmittedBy = ""
		clearance.SubmittedAt = nil
		clearance.ConfirmedBy = ""
		clearance.ConfirmedAt = nil
		expected := clearance.Version
		clearance.Version++
		clearance.UpdatedAt = time.Now().UTC()
		if err := s.repository.Update(ctx, clearance.ID, expected, &clearance); err != nil {
			return fmt.Errorf("cascade invalidate clearance %s: %w", clearance.Code, err)
		}
		if err := s.security.AuditWithWindowVersion(ctx, actor, requestID, "clearance_window_invalidated", "SafetyClearance", clearance.ID, before, target, reason, clearance.WindowVersion); err != nil {
			return fmt.Errorf("persist cascade audit: %w", err)
		}
	}
	return nil
}

// invalidateTimeExpired lazily flips pending clearances whose frozen window
// expired (or disappeared) since the last read, without waiting for review.
func (s *safetyClearanceService) invalidateTimeExpired(ctx context.Context, actor string) error {
	now := time.Now().UTC()
	items, err := s.repository.ListPendingLinkedToExpiredWindow(ctx, now)
	if err != nil {
		return err
	}
	for _, clearance := range items {
		before := clearance.Status
		clearance.Status = string(constants.ClearanceStateExpired)
		clearance.InvalidReason = truncateReason(fmt.Sprintf("关联窗口 %s 已过期或失效，待处理许可自动失效，请重新提交", clearance.RelatedCode))
		clearance.SubmittedBy = ""
		clearance.SubmittedAt = nil
		clearance.ConfirmedBy = ""
		clearance.ConfirmedAt = nil
		expected := clearance.Version
		clearance.Version++
		clearance.UpdatedAt = now
		if err := s.repository.Update(ctx, clearance.ID, expected, &clearance); err != nil {
			return fmt.Errorf("auto expire clearance %s: %w", clearance.Code, err)
		}
		if err := s.security.AuditWithWindowVersion(ctx, actor, "", "clearance_window_expired", "SafetyClearance", clearance.ID, before, clearance.Status, clearance.InvalidReason, clearance.WindowVersion); err != nil {
			return fmt.Errorf("persist auto expiry audit: %w", err)
		}
	}
	return nil
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

func truncateReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > 480 {
		return string([]rune(reason)[:480])
	}
	return reason
}

func validateSafetyClearanceBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
