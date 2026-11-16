package delta

import (
	"sync"
	"time"

	"github.com/geolens/geolens/domain"
)

// ---- EX-13 fix_artifact_event 采纳埋点 + 留存统计 ----

// RetentionStore 记录修复件采纳事件，口径：采纳率、30 天留存、60 天留存、
// 按 fix_item 分组统计（§1 EX-13 / F-503）。
type RetentionStore struct {
	mu     sync.Mutex
	events []domain.FixArtifactEvent
}

func NewRetentionStore() *RetentionStore {
	return &RetentionStore{events: []domain.FixArtifactEvent{}}
}

// Record 写一条事件（event_type 非法时拒绝，保证统计口径干净）。
func (s *RetentionStore) Record(reportID, fixItemID int64, typ domain.FixArtifactEventType, actorID *int64) (domain.FixArtifactEvent, bool) {
	if !validEventType(typ) {
		return domain.FixArtifactEvent{}, false
	}
	e := domain.FixArtifactEvent{
		EventID:    int64(len(s.events) + 1),
		ReportID:   reportID,
		FixItemID:  fixItemID,
		EventType:  typ,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		ActorID:    actorID,
	}
	s.mu.Lock()
	s.events = append(s.events, e)
	s.mu.Unlock()
	return e, true
}

func validEventType(t domain.FixArtifactEventType) bool {
	switch t {
	case domain.EventViewed, domain.EventDownloaded, domain.EventApplied, domain.EventRetestRequested:
		return true
	}
	return false
}

func (s *RetentionStore) Events() []domain.FixArtifactEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.FixArtifactEvent{}, s.events...)
}

// Stats 计算采纳与留存统计：applied 计为采纳；同一 fix_item 在窗口内再次
// applied / retest_requested 视为留存。窗口按事件时间相对"现在"计算。
func (s *RetentionStore) Stats() domain.RetentionStats {
	var out domain.RetentionStats
	for _, e := range s.Events() {
		switch e.EventType {
		case domain.EventViewed:
			out.Viewed++
		case domain.EventDownloaded:
			out.Downloaded++
		case domain.EventApplied:
			out.Applied++
		case domain.EventRetestRequested:
			out.RetestRequested++
		}
	}
	// 留存：窗口内有 retest_requested 或再次 applied 的 fix_item 数量。
	seen30, seen60 := map[int64]bool{}, map[int64]bool{}
	now := time.Now().UTC()
	for _, e := range s.Events() {
		if e.EventType != domain.EventRetestRequested {
			continue
		}
		t, err := time.Parse(time.RFC3339, e.OccurredAt)
		if err != nil {
			continue
		}
		days := now.Sub(t).Hours() / 24
		if days <= 60 {
			seen60[e.FixItemID] = true
		}
		if days <= 30 {
			seen30[e.FixItemID] = true
		}
	}
	out.Retained30d = len(seen30)
	out.Retained60d = len(seen60)
	return out
}

// ByFixItem 按 fix_item_id 分组统计（EX-13 验收「按 fix_item 分组统计正确」）。
func (s *RetentionStore) ByFixItem() map[int64]int {
	out := map[int64]int{}
	for _, e := range s.Events() {
		if e.EventType == domain.EventApplied {
			out[e.FixItemID]++
		}
	}
	return out
}
