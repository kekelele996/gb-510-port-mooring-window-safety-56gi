package repository

import (
	"context"
	"time"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/model"
	"gorm.io/gorm"
)

// SafetyClearanceRepository owns all persistence operations for 安全许可.
type SafetyClearanceRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.SafetyClearance], error)
	Get(context.Context, uint) (model.SafetyClearance, error)
	Create(context.Context, *model.SafetyClearance) error
	Update(context.Context, uint, uint, *model.SafetyClearance) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
	ListPendingByWindow(context.Context, string, string) ([]model.SafetyClearance, error)
	ListPendingLinkedToExpiredWindow(context.Context, time.Time) ([]model.SafetyClearance, error)
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

// ListPendingByWindow returns the still-pending clearances frozen against the
// window identified by facility and window code (related_code).
func (r *safetyClearanceRepository) ListPendingByWindow(ctx context.Context, facility, code string) ([]model.SafetyClearance, error) {
	var items []model.SafetyClearance
	err := r.store.DB().WithContext(ctx).
		Where("status = ? AND facility = ? AND related_code = ?", "pending", facility, code).
		Find(&items).Error
	return items, err
}

// ListPendingLinkedToExpiredWindow returns pending clearances whose frozen
// validity has lapsed or whose live window disappeared or was expired, so a
// read path can lazily invalidate them even without an explicit transition.
func (r *safetyClearanceRepository) ListPendingLinkedToExpiredWindow(ctx context.Context, now time.Time) ([]model.SafetyClearance, error) {
	var items []model.SafetyClearance
	err := r.store.DB().WithContext(ctx).
		Table("safety_clearances AS sc").
		Joins("LEFT JOIN weather_windows AS ww ON ww.facility = sc.facility AND ww.code = sc.related_code AND ww.deleted_at IS NULL").
		Where("sc.deleted_at IS NULL AND sc.status = ? AND (sc.window_expire_at < ? OR ww.id IS NULL OR ww.status = ?)",
			"pending", now, "expired").
		Find(&items).Error
	return items, err
}
