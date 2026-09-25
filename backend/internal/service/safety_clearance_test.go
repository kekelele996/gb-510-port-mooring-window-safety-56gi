package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/config"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/model"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type clearanceFixture struct {
	db        *gorm.DB
	window    repository.WeatherWindowRepository
	clearance repository.SafetyClearanceRepository
	security  SecurityService
	svc       SafetyClearanceService
}

func newClearanceFixture(t *testing.T) clearanceFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.SafetyClearance{}, &model.WeatherWindow{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	windowRepository := repository.NewWeatherWindowRepository(db)
	clearanceRepository := repository.NewSafetyClearanceRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	svc := NewSafetyClearanceService(clearanceRepository, windowRepository, security, nil)
	return clearanceFixture{db: db, window: windowRepository, clearance: clearanceRepository, security: security, svc: svc}
}

func (f clearanceFixture) createWindow(t *testing.T, code, facility, status string, effectiveAt, expireAt time.Time, version uint) model.WeatherWindow {
	t.Helper()
	window := model.WeatherWindow{
		BaseModel: model.BaseModel{Code: code, Name: "Window " + code, Status: status, Version: version},
		Facility:  facility, Owner: "operations", Category: "test", RiskLevel: "medium",
		EffectiveAt: effectiveAt, ExpireAt: expireAt,
	}
	if err := f.window.Create(context.Background(), &window); err != nil {
		t.Fatalf("create window: %v", err)
	}
	return window
}

func (f clearanceFixture) createClearance(t *testing.T, code, facility, relatedCode string, submittedBy string, windowVersion uint, windowExpireAt *time.Time) model.SafetyClearance {
	t.Helper()
	item := model.SafetyClearance{
		BaseModel: model.BaseModel{Code: code, Name: "Clearance " + code, Status: model.SafetyClearanceInitialStatus, Version: 1},
		Facility:  facility, Owner: "operations", Category: "test", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC(), Evidence: "checked", RelatedCode: relatedCode,
		WindowVersion: windowVersion, WindowExpireAt: windowExpireAt,
	}
	if submittedBy != "" {
		item.SubmittedBy = submittedBy
		submittedAt := time.Now().UTC().Add(-time.Minute)
		item.SubmittedAt = &submittedAt
	}
	if err := f.clearance.Create(context.Background(), &item); err != nil {
		t.Fatalf("create clearance: %v", err)
	}
	return item
}

func TestSafetyClearanceRequiresIndependentReviewer(t *testing.T) {
	f := newClearanceFixture(t)
	now := time.Now().UTC()
	f.createWindow(t, "WW-TEST", "Berth A", "safe", now.Add(-time.Hour), now.Add(time.Hour), 7)
	item := f.createClearance(t, "SC-TEST", "Berth A", "WW-TEST", "", 1, nil)

	first, err := f.svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: 1, Reason: "operator safety submission",
	}, "operator", model.RoleOperator, "request-submit")
	if err != nil {
		t.Fatalf("first confirmation: %v", err)
	}
	if first.Status != "pending" || first.SubmittedBy != "operator" || first.ConfirmedBy != "" || first.WindowVersion != 7 {
		t.Fatalf("unexpected first confirmation state: %+v", first)
	}
	if first.WindowExpireAt == nil {
		t.Fatalf("submission did not freeze window validity")
	}
	if delta := first.WindowExpireAt.Sub(now.Add(time.Hour)); delta > time.Second || delta < -time.Second {
		t.Fatalf("frozen validity %v does not match window expiry", first.WindowExpireAt)
	}

	_, err = f.svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: first.Version, Reason: "attempted self approval",
	}, "operator", model.RoleReviewer, "request-self")
	if !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("expected self approval error, got %v", err)
	}

	_, err = f.svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: first.Version, Reason: "second operator approval",
	}, "operator-two", model.RoleOperator, "request-operator")
	if !errors.Is(err, ErrReviewerRequired) {
		t.Fatalf("expected reviewer role error, got %v", err)
	}

	final, err := f.svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: first.Version, Reason: "independent safety review",
	}, "reviewer", model.RoleReviewer, "request-review")
	if err != nil {
		t.Fatalf("independent confirmation: %v", err)
	}
	if final.Status != "cleared" || final.SubmittedBy != "operator" || final.ConfirmedBy != "reviewer" {
		t.Fatalf("unexpected final confirmation state: %+v", final)
	}

	logs, total, err := f.security.ListAudits(context.Background(), 1, 20, "")
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if total != 2 || len(logs) != 2 {
		t.Fatalf("expected two audit entries, total=%d len=%d", total, len(logs))
	}
	for _, audit := range logs {
		if audit.WindowVersion != 7 || audit.RequestID == "" || audit.Actor == "" {
			t.Fatalf("audit did not preserve confirmation context: %+v", audit)
		}
	}
}

func TestConfirmReleasesRejectWhenWindowChangedRestrictedOrExpired(t *testing.T) {
	ctx := context.Background()

	t.Run("window version drift forces resubmission", func(t *testing.T) {
		f := newClearanceFixture(t)
		now := time.Now().UTC()
		f.createWindow(t, "WW-1", "Berth A", "safe", now.Add(-time.Hour), now.Add(time.Hour), 3)
		expire := now.Add(time.Hour)
		item := f.createClearance(t, "SC-1", "Berth A", "WW-1", "operator", 2, &expire)

		_, err := f.svc.Transition(ctx, item.ID, dto.TransitionRequest{
			Status: "cleared", ExpectedVersion: item.Version, Reason: "review against stale version",
		}, "reviewer", model.RoleReviewer, "request-drift")
		if !errors.Is(err, ErrWindowVersion) {
			t.Fatalf("expected window version error, got %v", err)
		}
		updated, getErr := f.clearance.Get(ctx, item.ID)
		if getErr != nil {
			t.Fatalf("reload clearance: %v", getErr)
		}
		if updated.Status != "pending" || updated.SubmittedBy != "" || updated.InvalidReason == "" {
			t.Fatalf("stale submission should be reset with a hint, got %+v", updated)
		}
		// 仍处 pending 时即可按新版本重新提交，随后双人确认放行。
		resubmitted, err := f.svc.Transition(ctx, item.ID, dto.TransitionRequest{
			Status: "pending", ExpectedVersion: updated.Version, Reason: "resubmit against new version",
		}, "operator", model.RoleOperator, "request-resubmit")
		if err != nil {
			t.Fatalf("resubmit pending clearance after drift: %v", err)
		}
		if resubmitted.WindowVersion != 3 || resubmitted.InvalidReason != "" || resubmitted.SubmittedBy != "" {
			t.Fatalf("resubmission should freeze v3 and clear hints, got %+v", resubmitted)
		}
		submitted, err := f.svc.Transition(ctx, item.ID, dto.TransitionRequest{
			Status: "cleared", ExpectedVersion: resubmitted.Version, Reason: "operator resubmits confirmation",
		}, "operator", model.RoleOperator, "request-resubmit-submit")
		if err != nil {
			t.Fatalf("operator submit after resubmit: %v", err)
		}
		released, err := f.svc.Transition(ctx, item.ID, dto.TransitionRequest{
			Status: "cleared", ExpectedVersion: submitted.Version, Reason: "reviewer releases against v3",
		}, "reviewer", model.RoleReviewer, "request-resubmit-review")
		if err != nil {
			t.Fatalf("reviewer release after resubmit: %v", err)
		}
		if released.Status != "cleared" {
			t.Fatalf("expected clearance to be released, got %s", released.Status)
		}
	})

	t.Run("restricted window rejects release and invalidates clearance", func(t *testing.T) {
		f := newClearanceFixture(t)
		now := time.Now().UTC()
		f.createWindow(t, "WW-2", "Berth B", "restricted", now.Add(-time.Hour), now.Add(time.Hour), 1)
		expire := now.Add(time.Hour)
		item := f.createClearance(t, "SC-2", "Berth B", "WW-2", "operator", 1, &expire)

		_, err := f.svc.Transition(ctx, item.ID, dto.TransitionRequest{
			Status: "cleared", ExpectedVersion: item.Version, Reason: "attempt release on restricted window",
		}, "reviewer", model.RoleReviewer, "request-restricted")
		if !errors.Is(err, ErrWindowRestricted) {
			t.Fatalf("expected restricted window error, got %v", err)
		}
		updated, _ := f.clearance.Get(ctx, item.ID)
		if updated.Status != "restricted" || updated.InvalidReason == "" || updated.SubmittedBy != "" {
			t.Fatalf("clearance should be restricted with reason and submission cleared, got %+v", updated)
		}
	})

	t.Run("window past validity at review rejects release and invalidates clearance", func(t *testing.T) {
		f := newClearanceFixture(t)
		now := time.Now().UTC()
		// The freeze was taken while valid, but the re-read window now ends in
		// the past (e.g. its validity was shortened between submit and review).
		f.createWindow(t, "WW-3", "Berth C", "safe", now.Add(-3*time.Hour), now.Add(-time.Minute), 1)
		futureExpire := now.Add(time.Hour)
		item := f.createClearance(t, "SC-3", "Berth C", "WW-3", "operator", 1, &futureExpire)

		_, err := f.svc.Transition(ctx, item.ID, dto.TransitionRequest{
			Status: "cleared", ExpectedVersion: item.Version, Reason: "attempt release past validity",
		}, "reviewer", model.RoleReviewer, "request-expired")
		if !errors.Is(err, ErrWindowExpired) {
			t.Fatalf("expected expired window error, got %v", err)
		}
		updated, _ := f.clearance.Get(ctx, item.ID)
		if updated.Status != "expired" || updated.InvalidReason == "" {
			t.Fatalf("clearance should be expired with reason, got %+v", updated)
		}
	})

	t.Run("resubmission freezes the new window and returns to pending", func(t *testing.T) {
		f := newClearanceFixture(t)
		now := time.Now().UTC()
		f.createWindow(t, "WW-4", "Berth D", "safe", now.Add(-time.Hour), now.Add(2*time.Hour), 5)
		item := model.SafetyClearance{
			BaseModel: model.BaseModel{Code: "SC-4", Name: "Clearance SC-4", Status: "restricted", Version: 4},
			Facility:  "Berth D", Owner: "operations", Category: "test", RiskLevel: "medium",
			EffectiveAt: now, RelatedCode: "WW-4", WindowVersion: 4,
			InvalidReason: "window restricted earlier",
		}
		if err := f.clearance.Create(ctx, &item); err != nil {
			t.Fatalf("create restricted clearance: %v", err)
		}
		back, err := f.svc.Transition(ctx, item.ID, dto.TransitionRequest{
			Status: "pending", ExpectedVersion: item.Version, Reason: "resubmit after window recovered",
		}, "operator", model.RoleOperator, "request-resubmit")
		if err != nil {
			t.Fatalf("resubmit clearance: %v", err)
		}
		if back.Status != "pending" || back.WindowVersion != 5 || back.InvalidReason != "" || back.SubmittedBy != "" {
			t.Fatalf("resubmitted clearance should be pending against v5, got %+v", back)
		}
	})
}

func TestWindowTransitionCascadesToPendingClearances(t *testing.T) {
	f := newClearanceFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	window := f.createWindow(t, "WW-CASCADE", "Berth Z", "safe", now.Add(-time.Hour), now.Add(time.Hour), 1)
	expire := now.Add(time.Hour)
	first := f.createClearance(t, "SC-A", "Berth Z", "WW-CASCADE", "operator", 1, &expire)
	second := f.createClearance(t, "SC-B", "Berth Z", "WW-CASCADE", "", 1, &expire)
	// Different facility must be untouched.
	other := f.createClearance(t, "SC-C", "Other Berth", "WW-CASCADE", "operator", 1, &expire)

	windowSvc := NewWeatherWindowService(f.window, f.svc, f.security)
	if _, err := windowSvc.Transition(ctx, window.ID, dto.TransitionRequest{
		Status: "restricted", ExpectedVersion: 1, Reason: "wind exceeds safe limit",
	}, "reviewer", "request-cascade"); err != nil {
		t.Fatalf("restrict window: %v", err)
	}

	for _, id := range []uint{first.ID, second.ID} {
		updated, err := f.clearance.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload clearance %d: %v", id, err)
		}
		if updated.Status != "restricted" || updated.InvalidReason == "" {
			t.Fatalf("clearance %d should cascade to restricted with reason, got %+v", id, updated)
		}
		if updated.SubmittedBy != "" {
			t.Fatalf("cascade should discard the stale submission for clearance %d", id)
		}
	}
	untouched, err := f.clearance.Get(ctx, other.ID)
	if err != nil {
		t.Fatalf("reload other clearance: %v", err)
	}
	if untouched.Status != "pending" {
		t.Fatalf("clearance in another facility must not be cascaded, got %s", untouched.Status)
	}
}

func TestListLazilyExpiresClearancesPastFrozenValidity(t *testing.T) {
	f := newClearanceFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	// Window expired in real life; the clearance list read must lazily invalidate it.
	f.createWindow(t, "WW-LAZY", "Berth L", "expired", now.Add(-3*time.Hour), now.Add(-time.Hour), 1)
	pastExpire := now.Add(-time.Hour)
	stale := f.createClearance(t, "SC-LAZY", "Berth L", "WW-LAZY", "operator", 1, &pastExpire)
	f.createClearance(t, "SC-LAZY-2", "Berth L", "WW-LAZY", "operator", 1, nil)

	if _, err := f.svc.List(ctx, dto.PageQuery{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("list clearances: %v", err)
	}
	updated, err := f.clearance.Get(ctx, stale.ID)
	if err != nil {
		t.Fatalf("reload stale clearance: %v", err)
	}
	if updated.Status != "expired" || updated.InvalidReason == "" {
		t.Fatalf("stale clearance should be lazily expired with reason, got %+v", updated)
	}
}

func TestCreateClearanceRejectsUnusableWindow(t *testing.T) {
	f := newClearanceFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	f.createWindow(t, "WW-BAD", "Berth Q", "restricted", now.Add(-time.Hour), now.Add(time.Hour), 1)

	_, err := f.svc.Create(ctx, dto.CreateSafetyClearance{
		Code: "SC-BAD", Name: "Bad clearance", Facility: "Berth Q", Owner: "operations",
		Category: "test", RiskLevel: "medium", EffectiveAt: now, RelatedCode: "WW-BAD",
	}, "operator", "request-create")
	if !errors.Is(err, ErrWindowRestricted) {
		t.Fatalf("expected create to reject restricted window, got %v", err)
	}
}
