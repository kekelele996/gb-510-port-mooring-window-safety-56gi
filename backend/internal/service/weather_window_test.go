package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/config"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/model"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWeatherWindowTransitionInvalidatesPendingClearances(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.WeatherWindow{}, &model.SafetyClearance{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	windowRepo := repository.NewWeatherWindowRepository(db)
	clearanceRepo := repository.NewSafetyClearanceRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	windowService := NewWeatherWindowService(windowRepo, clearanceRepo, security)

	ctx := context.Background()
	window := model.WeatherWindow{
		BaseModel: model.BaseModel{Code: "WW-CASCADE", Name: "Cascade window", Status: "safe", Version: 1},
		Facility:  "Berth C", Owner: "operations", Category: "test", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(6 * time.Hour),
	}
	if err := windowRepo.Create(ctx, &window); err != nil {
		t.Fatalf("create window: %v", err)
	}

	newClearance := func(code, facility, status, related string) model.SafetyClearance {
		item := model.SafetyClearance{
			BaseModel: model.BaseModel{Code: code, Name: code, Status: status, Version: 1},
			Facility:  facility, Owner: "operations", Category: "test", RiskLevel: "medium",
			EffectiveAt: time.Now().UTC(), RelatedCode: related, WindowVersion: 1,
		}
		if err := clearanceRepo.Create(ctx, &item); err != nil {
			t.Fatalf("create clearance %s: %v", code, err)
		}
		return item
	}
	pendingSameArea := newClearance("SC-P1", "Berth C", "pending", "WW-CASCADE")
	pendingOtherArea := newClearance("SC-P2", "Berth D", "pending", "WW-CASCADE")
	clearedSameArea := newClearance("SC-C1", "Berth C", "cleared", "WW-CASCADE")

	updated, err := windowService.Transition(ctx, window.ID, dto.TransitionRequest{
		Status: "restricted", ExpectedVersion: 1, Reason: "gale warning issued",
	}, "operator", "request-restrict")
	if err != nil {
		t.Fatalf("transition window: %v", err)
	}
	if updated.Status != "restricted" {
		t.Fatalf("unexpected window status: %+v", updated)
	}

	expired, err := clearanceRepo.Get(ctx, pendingSameArea.ID)
	if err != nil {
		t.Fatalf("reload pending clearance: %v", err)
	}
	if expired.Status != "expired" || !strings.Contains(expired.InvalidReason, "WW-CASCADE") {
		t.Fatalf("pending clearance must expire with a reason: %+v", expired)
	}

	untouched, err := clearanceRepo.Get(ctx, pendingOtherArea.ID)
	if err != nil {
		t.Fatalf("reload other area clearance: %v", err)
	}
	if untouched.Status != "pending" || untouched.InvalidReason != "" {
		t.Fatalf("clearance of another area must stay pending: %+v", untouched)
	}

	cleared, err := clearanceRepo.Get(ctx, clearedSameArea.ID)
	if err != nil {
		t.Fatalf("reload cleared clearance: %v", err)
	}
	if cleared.Status != "cleared" {
		t.Fatalf("cleared clearance must not be invalidated: %+v", cleared)
	}

	logs, total, err := security.ListAudits(ctx, 1, 20, "clearance_window_invalidated")
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("expected one invalidation audit, total=%d len=%d", total, len(logs))
	}
	audit := logs[0]
	if audit.EntityID != pendingSameArea.ID || audit.AfterState != "expired" || audit.WindowVersion != updated.Version || audit.Detail == "" {
		t.Fatalf("invalidation audit lost context: %+v", audit)
	}
}
