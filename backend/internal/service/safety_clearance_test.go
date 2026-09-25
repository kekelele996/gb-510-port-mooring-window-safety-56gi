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
	windows   repository.WeatherWindowRepository
	clearance repository.SafetyClearanceRepository
	security  SecurityService
	service   SafetyClearanceService
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
	windows := repository.NewWeatherWindowRepository(db)
	clearance := repository.NewSafetyClearanceRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	return clearanceFixture{
		db: db, windows: windows, clearance: clearance, security: security,
		service: NewSafetyClearanceService(clearance, windows, security),
	}
}

func (f clearanceFixture) seedWindow(t *testing.T, status string, expiresAt time.Time) model.WeatherWindow {
	t.Helper()
	window := model.WeatherWindow{
		BaseModel: model.BaseModel{Code: "WW-TEST", Name: "Test window", Status: status, Version: 1},
		Facility:  "Berth A", Owner: "operations", Category: "test", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC(), ExpiresAt: expiresAt, Evidence: "sounding checked",
	}
	if err := f.windows.Create(context.Background(), &window); err != nil {
		t.Fatalf("create window: %v", err)
	}
	return window
}

func (f clearanceFixture) seedClearance(t *testing.T) model.SafetyClearance {
	t.Helper()
	item := model.SafetyClearance{
		BaseModel: model.BaseModel{Code: "SC-TEST", Name: "Test clearance", Status: model.SafetyClearanceInitialStatus, Version: 1},
		Facility:  "Berth A", Owner: "operations", Category: "test", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC(), Evidence: "checked", RelatedCode: "WW-TEST",
	}
	if err := f.clearance.Create(context.Background(), &item); err != nil {
		t.Fatalf("create clearance: %v", err)
	}
	return item
}

func (f clearanceFixture) reviseWindow(t *testing.T, window model.WeatherWindow, mutate func(*model.WeatherWindow)) model.WeatherWindow {
	t.Helper()
	mutate(&window)
	expected := window.Version
	window.Version = expected + 1
	window.UpdatedAt = time.Now().UTC()
	if err := f.windows.Update(context.Background(), window.ID, expected, &window); err != nil {
		t.Fatalf("revise window: %v", err)
	}
	updated, err := f.windows.Get(context.Background(), window.ID)
	if err != nil {
		t.Fatalf("reload window: %v", err)
	}
	return updated
}

func TestSafetyClearanceRequiresIndependentReviewer(t *testing.T) {
	fixture := newClearanceFixture(t)
	fixture.seedWindow(t, "safe", time.Now().UTC().Add(12*time.Hour))
	item := fixture.seedClearance(t)

	first, err := fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: 1, Reason: "operator safety submission",
	}, "operator", model.RoleOperator, "request-submit")
	if err != nil {
		t.Fatalf("first confirmation: %v", err)
	}
	if first.Status != "pending" || first.SubmittedBy != "operator" || first.ConfirmedBy != "" || first.WindowVersion != 1 {
		t.Fatalf("unexpected first confirmation state: %+v", first)
	}
	if first.WindowExpiresAt == nil {
		t.Fatalf("submission must freeze the window expiry: %+v", first)
	}

	_, err = fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: first.Version, Reason: "attempted self approval", WindowVersion: 1,
	}, "operator", model.RoleReviewer, "request-self")
	if !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("expected self approval error, got %v", err)
	}

	_, err = fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: first.Version, Reason: "second operator approval", WindowVersion: 1,
	}, "operator-two", model.RoleOperator, "request-operator")
	if !errors.Is(err, ErrReviewerRequired) {
		t.Fatalf("expected reviewer role error, got %v", err)
	}

	final, err := fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: first.Version, Reason: "independent safety review", WindowVersion: 1,
	}, "reviewer", model.RoleReviewer, "request-review")
	if err != nil {
		t.Fatalf("independent confirmation: %v", err)
	}
	if final.Status != "cleared" || final.SubmittedBy != "operator" || final.ConfirmedBy != "reviewer" {
		t.Fatalf("unexpected final confirmation state: %+v", final)
	}

	logs, total, err := fixture.security.ListAudits(context.Background(), 1, 20, "")
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if total != 2 || len(logs) != 2 {
		t.Fatalf("expected two audit entries, total=%d len=%d", total, len(logs))
	}
	for _, audit := range logs {
		if audit.WindowVersion != 1 || audit.RequestID == "" || audit.Actor == "" {
			t.Fatalf("audit did not preserve confirmation context: %+v", audit)
		}
	}
}

func TestSafetyClearanceRejectsRevisedWindowUntilResubmitted(t *testing.T) {
	fixture := newClearanceFixture(t)
	window := fixture.seedWindow(t, "safe", time.Now().UTC().Add(12*time.Hour))
	item := fixture.seedClearance(t)

	submitted, err := fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: 1, Reason: "operator safety submission",
	}, "operator", model.RoleOperator, "request-submit")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	window = fixture.reviseWindow(t, window, func(w *model.WeatherWindow) { w.Name = "Test window revised" })

	_, err = fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: submitted.Version, Reason: "review after window revision", WindowVersion: submitted.WindowVersion,
	}, "reviewer", model.RoleReviewer, "request-stale-review")
	if !errors.Is(err, ErrWindowVersion) {
		t.Fatalf("expected window version error after revision, got %v", err)
	}

	resubmitted, err := fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: submitted.Version, Reason: "resubmit against revised window",
	}, "operator", model.RoleOperator, "request-resubmit")
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if resubmitted.WindowVersion != window.Version || resubmitted.SubmittedBy != "operator" {
		t.Fatalf("resubmission must freeze the revised window version: %+v", resubmitted)
	}

	final, err := fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: resubmitted.Version, Reason: "review after resubmission", WindowVersion: resubmitted.WindowVersion,
	}, "reviewer", model.RoleReviewer, "request-review")
	if err != nil {
		t.Fatalf("confirm after resubmission: %v", err)
	}
	if final.Status != "cleared" || final.ConfirmedBy != "reviewer" {
		t.Fatalf("unexpected final state after resubmission: %+v", final)
	}
}

func TestSafetyClearanceRejectsRestrictedOrExpiredWindow(t *testing.T) {
	fixture := newClearanceFixture(t)
	window := fixture.seedWindow(t, "safe", time.Now().UTC().Add(12*time.Hour))
	item := fixture.seedClearance(t)

	submitted, err := fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: 1, Reason: "operator safety submission",
	}, "operator", model.RoleOperator, "request-submit")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	window = fixture.reviseWindow(t, window, func(w *model.WeatherWindow) { w.Status = "restricted" })
	_, err = fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: submitted.Version, Reason: "review against restricted window", WindowVersion: submitted.WindowVersion,
	}, "reviewer", model.RoleReviewer, "request-restricted")
	if !errors.Is(err, ErrWindowVersion) {
		t.Fatalf("restricted window revision must surface as version change first, got %v", err)
	}
	_, err = fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: submitted.Version, Reason: "resubmit against restricted window",
	}, "operator", model.RoleOperator, "request-restricted-resubmit")
	if !errors.Is(err, ErrWindowState) {
		t.Fatalf("expected window state error on restricted window, got %v", err)
	}

	window = fixture.reviseWindow(t, window, func(w *model.WeatherWindow) {
		w.Status = "safe"
		w.ExpiresAt = time.Now().UTC().Add(-time.Hour)
	})
	_, err = fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: submitted.Version, Reason: "resubmit against expired window",
	}, "operator", model.RoleOperator, "request-expired-resubmit")
	if !errors.Is(err, ErrWindowExpired) {
		t.Fatalf("expected window expiry error, got %v", err)
	}
}

func TestSafetyClearanceRequiresLinkedWindow(t *testing.T) {
	fixture := newClearanceFixture(t)
	item := model.SafetyClearance{
		BaseModel: model.BaseModel{Code: "SC-ORPHAN", Name: "Orphan clearance", Status: model.SafetyClearanceInitialStatus, Version: 1},
		Facility:  "Berth A", Owner: "operations", Category: "test", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC(), Evidence: "checked", RelatedCode: "WW-MISSING",
	}
	if err := fixture.clearance.Create(context.Background(), &item); err != nil {
		t.Fatalf("create clearance: %v", err)
	}
	_, err := fixture.service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: 1, Reason: "operator safety submission",
	}, "operator", model.RoleOperator, "request-submit")
	if !errors.Is(err, ErrWindowLinked) {
		t.Fatalf("expected linked window error, got %v", err)
	}
}
