package repository

import (
	"context"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/model"
	"gorm.io/gorm"
)

// VesselCallRepository owns all persistence operations for 船舶靠泊.
type VesselCallRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.VesselCall], error)
	Get(context.Context, uint) (model.VesselCall, error)
	Create(context.Context, *model.VesselCall) error
	Update(context.Context, uint, uint, *model.VesselCall) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type vesselCallRepository struct {
	store *Store[model.VesselCall]
}

func NewVesselCallRepository(db *gorm.DB) VesselCallRepository {
	return &vesselCallRepository{store: NewStore[model.VesselCall](db)}
}

func (r *vesselCallRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.VesselCall], error) {
	return r.store.List(ctx, q)
}
func (r *vesselCallRepository) Get(ctx context.Context, id uint) (model.VesselCall, error) {
	return r.store.Get(ctx, id)
}
func (r *vesselCallRepository) Create(ctx context.Context, item *model.VesselCall) error {
	return r.store.Create(ctx, item)
}
func (r *vesselCallRepository) Update(ctx context.Context, id, version uint, item *model.VesselCall) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *vesselCallRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *vesselCallRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
