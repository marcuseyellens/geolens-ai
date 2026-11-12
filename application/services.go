package application

import (
	"context"

	"github.com/geolens/geolens/domain"
)

// BrandRepository 品牌仓储接口
type BrandRepository interface {
	Create(ctx context.Context, brand *domain.Brand) error
	GetByID(ctx context.Context, id string) (*domain.Brand, error)
	List(ctx context.Context) ([]*domain.Brand, error)
	Update(ctx context.Context, brand *domain.Brand) error
	Delete(ctx context.Context, id string) error
}

// QueryRepository 检测查询仓储接口
type QueryRepository interface {
	Create(ctx context.Context, query *domain.DetectionQuery) error
	ListByBrand(ctx context.Context, brandID string) ([]*domain.DetectionQuery, error)
	Delete(ctx context.Context, id string) error
}

// ResultRepository 检测结果仓储接口
type ResultRepository interface {
	Save(ctx context.Context, result *domain.DetectionResult) error
	ListByBrand(ctx context.Context, brandID string) ([]*domain.DetectionResult, error)
	ListByBrandAndModel(ctx context.Context, brandID string, model domain.AIModelID) ([]*domain.DetectionResult, error)
	ListRecent(ctx context.Context, brandID string, limit int) ([]*domain.DetectionResult, error)
}

// DetectionService 是检测编排服务
type DetectionService struct {
	brands  BrandRepository
	queries QueryRepository
	results ResultRepository
	engine  AIEngine
}

// AIEngine 是 AI 模型检测引擎接口
type AIEngine interface {
	Detect(ctx context.Context, query *domain.DetectionQuery, brand *domain.Brand, model domain.AIModelID) (*domain.DetectionResult, error)
	DetectAll(ctx context.Context, query *domain.DetectionQuery, brand *domain.Brand) ([]*domain.DetectionResult, error)
}

// NewDetectionService 创建检测服务
func NewDetectionService(brands BrandRepository, queries QueryRepository, results ResultRepository, engine AIEngine) *DetectionService {
	return &DetectionService{brands: brands, queries: queries, results: results, engine: engine}
}

// ---- Brand 代理方法 ----

func (s *DetectionService) CreateBrand(ctx context.Context, brand *domain.Brand) error {
	return s.brands.Create(ctx, brand)
}

func (s *DetectionService) ListBrands(ctx context.Context) ([]*domain.Brand, error) {
	return s.brands.List(ctx)
}

func (s *DetectionService) UpdateBrand(ctx context.Context, brand *domain.Brand) error {
	return s.brands.Update(ctx, brand)
}

func (s *DetectionService) DeleteBrand(ctx context.Context, id string) error {
	return s.brands.Delete(ctx, id)
}

// ---- Query 代理方法 ----

func (s *DetectionService) CreateQuery(ctx context.Context, query *domain.DetectionQuery) error {
	return s.queries.Create(ctx, query)
}

func (s *DetectionService) ListQueries(ctx context.Context, brandID string) ([]*domain.DetectionQuery, error) {
	return s.queries.ListByBrand(ctx, brandID)
}

func (s *DetectionService) DeleteQuery(ctx context.Context, id string) error {
	return s.queries.Delete(ctx, id)
}

// ---- Result 代理方法 ----

func (s *DetectionService) ListResults(ctx context.Context, brandID string) ([]*domain.DetectionResult, error) {
	return s.results.ListByBrand(ctx, brandID)
}

// RunDetection 执行一次全模型检测
func (s *DetectionService) RunDetection(ctx context.Context, brandID string, queryID string) ([]*domain.DetectionResult, error) {
	brand, err := s.brands.GetByID(ctx, brandID)
	if err != nil {
		return nil, err
	}
	queries, err := s.queries.ListByBrand(ctx, brandID)
	if err != nil {
		return nil, err
	}
	var target *domain.DetectionQuery
	for _, q := range queries {
		if q.ID == queryID {
			target = q
			break
		}
	}
	if target == nil {
		return nil, ErrNotFound
	}
	results, err := s.engine.DetectAll(ctx, target, brand)
	if err != nil {
		return nil, err
	}
	for _, r := range results {
		if err := s.results.Save(ctx, r); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// GetReport 获取品牌检测报告
func (s *DetectionService) GetReport(ctx context.Context, brandID string) (*domain.ReportSummary, error) {
	brand, err := s.brands.GetByID(ctx, brandID)
	if err != nil {
		return nil, err
	}
	results, err := s.results.ListByBrand(ctx, brandID)
	if err != nil {
		return nil, err
	}
	return buildReport(brand, results), nil
}

func buildReport(brand *domain.Brand, results []*domain.DetectionResult) *domain.ReportSummary {
	report := &domain.ReportSummary{
		BrandID:        brand.ID,
		BrandName:      brand.Name,
		ModelBreakdown: make(map[domain.AIModelID]domain.ModelStat),
		Trend:          []domain.TrendPoint{},
	}
	total := len(results)
	if total == 0 {
		return report
	}
	mentioned := 0
	var scoreSum, posSum float64
	for _, r := range results {
		stat := report.ModelBreakdown[r.Model]
		stat.Total++
		if r.BrandMentioned {
			stat.Mentioned++
			mentioned++
		}
		stat.AvgScore += r.Score
		report.ModelBreakdown[r.Model] = stat
		scoreSum += r.Score
		posSum += domain.PositionWeight(r.MentionPosition)
	}
	for m, stat := range report.ModelBreakdown {
		if stat.Total > 0 {
			stat.MentionRate = float64(stat.Mentioned) / float64(stat.Total) * 100
			stat.AvgScore /= float64(stat.Total)
			report.ModelBreakdown[m] = stat
		}
	}
	report.TotalDetections = total
	report.MentionRate = float64(mentioned) / float64(total) * 100
	report.AvgScore = scoreSum / float64(total)
	report.AvgPosition = posSum / float64(total)
	return report
}

// ErrNotFound 通用未找到错误
var ErrNotFound = &appError{msg: "resource not found"}

type appError struct{ msg string }

func (e *appError) Error() string { return e.msg }

// GetBrand 封装 GetByID，转换存储层错误
func (s *DetectionService) GetBrand(ctx context.Context, id string) (*domain.Brand, error) {
	brand, err := s.brands.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return brand, nil
}


