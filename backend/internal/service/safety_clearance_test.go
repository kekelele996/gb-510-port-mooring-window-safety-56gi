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

func TestSafetyClearanceRequiresIndependentReviewer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.SafetyClearance{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	clearanceRepository := repository.NewSafetyClearanceRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	svc := NewSafetyClearanceService(clearanceRepository, security)
	item := model.SafetyClearance{
		BaseModel: model.BaseModel{Code: "SC-TEST", Name: "Test clearance", Status: model.SafetyClearanceInitialStatus, Version: 1},
		Facility:  "Berth A", Owner: "operations", Category: "test", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC(), Evidence: "checked", RelatedCode: "WW-TEST", WindowVersion: 1,
	}
	if err := clearanceRepository.Create(context.Background(), &item); err != nil {
		t.Fatalf("create clearance: %v", err)
	}

	first, err := svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: 1, Reason: "operator safety submission", WindowVersion: 7,
	}, "operator", model.RoleOperator, "request-submit")
	if err != nil {
		t.Fatalf("first confirmation: %v", err)
	}
	if first.Status != "pending" || first.SubmittedBy != "operator" || first.ConfirmedBy != "" || first.WindowVersion != 7 {
		t.Fatalf("unexpected first confirmation state: %+v", first)
	}

	_, err = svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: first.Version, Reason: "attempted self approval", WindowVersion: 7,
	}, "operator", model.RoleReviewer, "request-self")
	if !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("expected self approval error, got %v", err)
	}

	_, err = svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: first.Version, Reason: "second operator approval", WindowVersion: 7,
	}, "operator-two", model.RoleOperator, "request-operator")
	if !errors.Is(err, ErrReviewerRequired) {
		t.Fatalf("expected reviewer role error, got %v", err)
	}

	final, err := svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "cleared", ExpectedVersion: first.Version, Reason: "independent safety review", WindowVersion: 7,
	}, "reviewer", model.RoleReviewer, "request-review")
	if err != nil {
		t.Fatalf("independent confirmation: %v", err)
	}
	if final.Status != "cleared" || final.SubmittedBy != "operator" || final.ConfirmedBy != "reviewer" {
		t.Fatalf("unexpected final confirmation state: %+v", final)
	}

	logs, total, err := security.ListAudits(context.Background(), 1, 20, "")
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
