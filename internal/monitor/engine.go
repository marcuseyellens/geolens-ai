// Package monitor 实现基线 WBS WP-8.2「监控告警五块看板」与 WP-8.3「发布检查清单」。
//
// 看板阈值由本平台设定，不代表任何外部平台规则；所有告警文案只描述观测到的
// 数值，不做因果推断或效果承诺（16 款文案 §5.2）。
package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/geolens/geolens/domain"
)

// Sources 是五块看板的输入快照，由 handlers 层从各运行时实例采集后填入。
type Sources struct {
	CollectionValid  int     `json:"collection_valid"`
	CollectionTotal  int     `json:"collection_total"`
	DailyCostUSD     float64 `json:"daily_cost_usd"`
	BudgetTrips      int     `json:"budget_trips"`
	PaymentFailed    int     `json:"payment_failed"`
	PaymentTotal     int     `json:"payment_total"`
	FunnelConversion float64 `json:"funnel_conversion"`
}

const (
	ThresholdPaymentFailureRate = 0.10
	ThresholdBudgetTrips        = 1
)

// Engine 汇总监控快照并保留历史，供看板按天回看。
type Engine struct {
	mu      sync.Mutex
	clock   func() time.Time
	sources Sources
	history []Snapshot
}

// Snapshot 是带时间戳的看板快照。
type Snapshot struct {
	domain.MonitorSnapshot
	At time.Time `json:"at"`
}

func NewEngine(clock func() time.Time) *Engine {
	if clock == nil {
		clock = time.Now
	}
	return &Engine{clock: clock, history: []Snapshot{}}
}

func (e *Engine) Update(s Sources) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sources = s
}

func (e *Engine) Sources() Sources {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sources
}

// Snapshot 计算一次五块看板快照，并生成阈值告警。
func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sources
	rate := 0.0
	if s.CollectionTotal > 0 {
		rate = float64(s.CollectionValid) / float64(s.CollectionTotal)
	}
	payFailRate := 0.0
	if s.PaymentTotal > 0 {
		payFailRate = float64(s.PaymentFailed) / float64(s.PaymentTotal)
	}
	snap := Snapshot{
		At: e.clock(),
		MonitorSnapshot: domain.MonitorSnapshot{
			CollectionSuccessRate: rate,
			DailyCostUSD:          s.DailyCostUSD,
			BudgetTrips:           s.BudgetTrips,
			PaymentFailureRate:    payFailRate,
			FunnelConversion:      s.FunnelConversion,
			Alerts:                []domain.MonitorAlert{},
			Provenance:            domain.ObservedProvenance(s.CollectionTotal),
		},
	}
	if s.CollectionTotal > 0 && rate < domain.ThresholdCollectionRate {
		snap.Alerts = append(snap.Alerts, domain.MonitorAlert{
			Metric: "collection_success_rate", Value: rate,
			Threshold: domain.ThresholdCollectionRate,
			Message:   fmt.Sprintf("采集有效率 %.1f%% 低于本平台阈值 %.1f%%", rate*100, domain.ThresholdCollectionRate*100),
		})
	}
	if s.DailyCostUSD > domain.ThresholdDailyCostUSD {
		snap.Alerts = append(snap.Alerts, domain.MonitorAlert{
			Metric: "daily_cost_usd", Value: s.DailyCostUSD,
			Threshold: domain.ThresholdDailyCostUSD,
			Message:   fmt.Sprintf("当日运行成本 $%.2f 超过本平台日预算 $%.2f", s.DailyCostUSD, domain.ThresholdDailyCostUSD),
		})
	}
	if s.BudgetTrips >= ThresholdBudgetTrips {
		snap.Alerts = append(snap.Alerts, domain.MonitorAlert{
			Metric: "budget_trips", Value: float64(s.BudgetTrips),
			Threshold: ThresholdBudgetTrips,
			Message:   fmt.Sprintf("预算熔断触发 %d 次，路由层已拒绝超限运行", s.BudgetTrips),
		})
	}
	if s.PaymentTotal > 0 && payFailRate > ThresholdPaymentFailureRate {
		snap.Alerts = append(snap.Alerts, domain.MonitorAlert{
			Metric: "payment_failure_rate", Value: payFailRate,
			Threshold: ThresholdPaymentFailureRate,
			Message:   fmt.Sprintf("支付失败率 %.1f%% 超过本平台阈值 %.1f%%", payFailRate*100, ThresholdPaymentFailureRate*100),
		})
	}
	e.history = append(e.history, snap)
	return snap
}

func (e *Engine) History() []Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Snapshot, len(e.history))
	copy(out, e.history)
	return out
}

// Boards 把五块看板拆成可直接渲染的指标行，避免前端自行计算比例。
func Boards(s Snapshot) []BoardRow {
	return []BoardRow{
		{Key: "collection_success_rate", Label: "采集有效率", Value: s.CollectionSuccessRate * 100, Unit: "%", Provenance: s.Provenance},
		{Key: "daily_cost_usd", Label: "当日运行成本", Value: s.DailyCostUSD, Unit: "USD", Provenance: s.Provenance},
		{Key: "budget_trips", Label: "预算熔断次数", Value: float64(s.BudgetTrips), Unit: "次", Provenance: s.Provenance},
		{Key: "payment_failure_rate", Label: "支付失败率", Value: s.PaymentFailureRate * 100, Unit: "%", Provenance: s.Provenance},
		{Key: "funnel_conversion", Label: "漏斗转化率", Value: s.FunnelConversion * 100, Unit: "%", Provenance: s.Provenance},
	}
}

// BoardRow 是看板的一行；所有数字都带 provenance。
type BoardRow struct {
	Key        string            `json:"key"`
	Label      string            `json:"label"`
	Value      float64           `json:"value"`
	Unit       string            `json:"unit"`
	Provenance domain.Provenance `json:"provenance"`
}
