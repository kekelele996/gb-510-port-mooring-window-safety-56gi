package repository

import (
	"context"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/model"
	"gorm.io/gorm"
)

// SafetyClearanceRepository owns all persistence operations for 安全许可.
type SafetyClearanceRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.SafetyClearance], error)
	Get(context.Context, uint) (model.SafetyClearance, error)
	ListPendingByWindow(context.Context, string, string) ([]model.SafetyClearance, error)
	Create(context.Context, *model.SafetyClearance) error
	Update(context.Context, uint, uint, *model.SafetyClearance) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type safetyClearanceRepository struct {
	store *Store[model.SafetyClearance]
}

func NewSafetyClearanceRepository(db *gorm.DB) SafetyClearanceRepository {
	return &safetyClearanceRepository{store: NewStore[model.SafetyClearance](db)}
}

func (r *safetyClearanceRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.SafetyClearance], error) {
	return r.store.List(ctx, q)
}
func (r *safetyClearanceRepository) Get(ctx context.Context, id uint) (model.SafetyClearance, error) {
	return r.store.Get(ctx, id)
}

// ListPendingByWindow returns the pending clearances of one operational area
// whose related code points at the given weather window.
func (r *safetyClearanceRepository) ListPendingByWindow(ctx context.Context, facility, windowCode string) ([]model.SafetyClearance, error) {
	return r.store.ListByRelatedCode(ctx, facility, windowCode, "pending")
}
func (r *safetyClearanceRepository) Create(ctx context.Context, item *model.SafetyClearance) error {
	return r.store.Create(ctx, item)
}
func (r *safetyClearanceRepository) Update(ctx context.Context, id, version uint, item *model.SafetyClearance) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *safetyClearanceRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *safetyClearanceRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
