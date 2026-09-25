package repository

import (
	"context"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/model"
	"gorm.io/gorm"
)

// WeatherWindowRepository owns all persistence operations for 风浪窗口.
type WeatherWindowRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.WeatherWindow], error)
	Get(context.Context, uint) (model.WeatherWindow, error)
	FindByCode(context.Context, string, string) (model.WeatherWindow, error)
	Create(context.Context, *model.WeatherWindow) error
	Update(context.Context, uint, uint, *model.WeatherWindow) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type weatherWindowRepository struct {
	store *Store[model.WeatherWindow]
}

func NewWeatherWindowRepository(db *gorm.DB) WeatherWindowRepository {
	return &weatherWindowRepository{store: NewStore[model.WeatherWindow](db)}
}

func (r *weatherWindowRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.WeatherWindow], error) {
	return r.store.List(ctx, q)
}
func (r *weatherWindowRepository) Get(ctx context.Context, id uint) (model.WeatherWindow, error) {
	return r.store.Get(ctx, id)
}
func (r *weatherWindowRepository) FindByCode(ctx context.Context, code, facility string) (model.WeatherWindow, error) {
	return r.store.FindByCodeAndFacility(ctx, code, facility)
}
func (r *weatherWindowRepository) Create(ctx context.Context, item *model.WeatherWindow) error {
	return r.store.Create(ctx, item)
}
func (r *weatherWindowRepository) Update(ctx context.Context, id, version uint, item *model.WeatherWindow) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *weatherWindowRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *weatherWindowRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
