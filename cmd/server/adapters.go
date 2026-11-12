package main

import (
	"context"

	"github.com/geolens/geolens/application"
	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/storage"
)

// brandRepoAdapter 适配 storage.Store 到 application.BrandRepository
type brandRepoAdapter struct{ store *storage.Store }

func (a *brandRepoAdapter) Create(ctx context.Context, brand *domain.Brand) error {
	return a.store.CreateBrand(brand)
}
func (a *brandRepoAdapter) GetByID(ctx context.Context, id string) (*domain.Brand, error) {
	return a.store.GetBrand(id)
}
func (a *brandRepoAdapter) List(ctx context.Context) ([]*domain.Brand, error) {
	return a.store.ListBrands()
}
func (a *brandRepoAdapter) Update(ctx context.Context, brand *domain.Brand) error {
	return a.store.UpdateBrand(brand)
}
func (a *brandRepoAdapter) Delete(ctx context.Context, id string) error {
	return a.store.DeleteBrand(id)
}

// queryRepoAdapter 适配 storage.Store 到 application.QueryRepository
type queryRepoAdapter struct{ store *storage.Store }

func (a *queryRepoAdapter) Create(ctx context.Context, query *domain.DetectionQuery) error {
	return a.store.CreateQuery(query)
}
func (a *queryRepoAdapter) ListByBrand(ctx context.Context, brandID string) ([]*domain.DetectionQuery, error) {
	return a.store.ListQueriesByBrand(brandID)
}
func (a *queryRepoAdapter) Delete(ctx context.Context, id string) error {
	return a.store.DeleteQuery(id)
}

// resultRepoAdapter 适配 storage.Store 到 application.ResultRepository
type resultRepoAdapter struct{ store *storage.Store }

func (a *resultRepoAdapter) Save(ctx context.Context, result *domain.DetectionResult) error {
	return a.store.SaveResult(result)
}
func (a *resultRepoAdapter) ListByBrand(ctx context.Context, brandID string) ([]*domain.DetectionResult, error) {
	return a.store.ListResultsByBrand(brandID)
}
func (a *resultRepoAdapter) ListByBrandAndModel(ctx context.Context, brandID string, model domain.AIModelID) ([]*domain.DetectionResult, error) {
	return a.store.ListResultsByBrandAndModel(brandID, model)
}
func (a *resultRepoAdapter) ListRecent(ctx context.Context, brandID string, limit int) ([]*domain.DetectionResult, error) {
	return a.store.ListRecentResults(brandID, limit)
}

// 确保适配器实现了接口
var _ application.BrandRepository = (*brandRepoAdapter)(nil)
var _ application.QueryRepository = (*queryRepoAdapter)(nil)
var _ application.ResultRepository = (*resultRepoAdapter)(nil)
