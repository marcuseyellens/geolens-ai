package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geolens/geolens/domain"
	"github.com/google/uuid"
)

// Store JSON 文件存储实现
type Store struct {
	mu       sync.RWMutex
	filePath string
	data     *storeData
}

type storeData struct {
	Brands  []*domain.Brand          `json:"brands"`
	Queries []*domain.DetectionQuery `json:"queries"`
	Results []*domain.DetectionResult `json:"results"`
}

// NewStore 创建 JSON 文件存储
func NewStore(path string) (*Store, error) {
	s := &Store{
		filePath: path,
		data:     &storeData{
			Brands:  []*domain.Brand{},
			Queries: []*domain.DetectionQuery{},
			Results: []*domain.DetectionResult{},
		},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.save()
		}
		return err
	}
	return json.Unmarshal(data, s.data)
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// ---- BrandRepository ----

func (s *Store) CreateBrand(b *domain.Brand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.CreatedAt = time.Now()
	b.UpdatedAt = time.Now()
	s.data.Brands = append(s.data.Brands, b)
	return s.save()
}

func (s *Store) GetBrand(id string) (*domain.Brand, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.data.Brands {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, ErrNotFound
}

func (s *Store) ListBrands() ([]*domain.Brand, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.Brand, len(s.data.Brands))
	copy(result, s.data.Brands)
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) UpdateBrand(b *domain.Brand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.data.Brands {
		if existing.ID == b.ID {
			b.CreatedAt = existing.CreatedAt
			b.UpdatedAt = time.Now()
			s.data.Brands[i] = b
			return s.save()
		}
	}
	return ErrNotFound
}

func (s *Store) DeleteBrand(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	newBrands := make([]*domain.Brand, 0, len(s.data.Brands))
	for _, b := range s.data.Brands {
		if b.ID != id {
			newBrands = append(newBrands, b)
		}
	}
	s.data.Brands = newBrands
	s.data.Queries = filterQueries(s.data.Queries, func(q *domain.DetectionQuery) bool {
		return q.BrandID != id
	})
	s.data.Results = filterResults(s.data.Results, func(r *domain.DetectionResult) bool {
		return r.BrandID != id
	})
	return s.save()
}

// ---- QueryRepository ----

func (s *Store) CreateQuery(q *domain.DetectionQuery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if q.ID == "" {
		q.ID = uuid.New().String()
	}
	q.CreatedAt = time.Now()
	s.data.Queries = append(s.data.Queries, q)
	return s.save()
}

func (s *Store) ListQueriesByBrand(brandID string) ([]*domain.DetectionQuery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.DetectionQuery
	for _, q := range s.data.Queries {
		if q.BrandID == brandID {
			result = append(result, q)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) DeleteQuery(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Queries = filterQueries(s.data.Queries, func(q *domain.DetectionQuery) bool {
		return q.ID != id
	})
	s.data.Results = filterResults(s.data.Results, func(r *domain.DetectionResult) bool {
		return r.QueryID != id
	})
	return s.save()
}

// ---- ResultRepository ----

func (s *Store) SaveResult(r *domain.DetectionResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	s.data.Results = append(s.data.Results, r)
	return s.save()
}

func (s *Store) ListResultsByBrand(brandID string) ([]*domain.DetectionResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.DetectionResult
	for _, r := range s.data.Results {
		if r.BrandID == brandID {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].DetectedAt.After(result[j].DetectedAt)
	})
	return result, nil
}

func (s *Store) ListResultsByBrandAndModel(brandID string, model domain.AIModelID) ([]*domain.DetectionResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.DetectionResult
	for _, r := range s.data.Results {
		if r.BrandID == brandID && r.Model == model {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *Store) ListRecentResults(brandID string, limit int) ([]*domain.DetectionResult, error) {
	all, err := s.ListResultsByBrand(brandID)
	if err != nil {
		return nil, err
	}
	if len(all) > limit {
		return all[:limit], nil
	}
	return all, nil
}

// ---- 辅助 ----

func filterQueries(queries []*domain.DetectionQuery, keep func(*domain.DetectionQuery) bool) []*domain.DetectionQuery {
	result := make([]*domain.DetectionQuery, 0, len(queries))
	for _, q := range queries {
		if keep(q) {
			result = append(result, q)
		}
	}
	return result
}

func filterResults(results []*domain.DetectionResult, keep func(*domain.DetectionResult) bool) []*domain.DetectionResult {
	result := make([]*domain.DetectionResult, 0, len(results))
	for _, r := range results {
		if keep(r) {
			result = append(result, r)
		}
	}
	return result
}

// joinStrs / splitStrs 保留以备兼容
func joinStrs(ss []string) string {
	return strings.Join(ss, ",")
}

func splitStrs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// ErrNotFound 通用未找到错误
var ErrNotFound = &storeError{msg: "not found"}

type storeError struct{ msg string }

func (e *storeError) Error() string { return e.msg }
