package repository

import (
	"context"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/model"
	"gorm.io/gorm"
)

// MooringPlanRepository owns all persistence operations for 系泊方案.
type MooringPlanRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.MooringPlan], error)
	Get(context.Context, uint) (model.MooringPlan, error)
	Create(context.Context, *model.MooringPlan) error
	Update(context.Context, uint, uint, *model.MooringPlan) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type mooringPlanRepository struct {
	store *Store[model.MooringPlan]
}

func NewMooringPlanRepository(db *gorm.DB) MooringPlanRepository {
	return &mooringPlanRepository{store: NewStore[model.MooringPlan](db)}
}

func (r *mooringPlanRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.MooringPlan], error) {
	return r.store.List(ctx, q)
}
func (r *mooringPlanRepository) Get(ctx context.Context, id uint) (model.MooringPlan, error) {
	return r.store.Get(ctx, id)
}
func (r *mooringPlanRepository) Create(ctx context.Context, item *model.MooringPlan) error {
	return r.store.Create(ctx, item)
}
func (r *mooringPlanRepository) Update(ctx context.Context, id, version uint, item *model.MooringPlan) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *mooringPlanRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *mooringPlanRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
