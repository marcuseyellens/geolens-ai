package aeo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/geolens/geolens/domain"
)

// ---- A-WP-6.1 8 周滚动基线 ----
//
// 只用 valid=true 样本；失败批次排除；样本 < 6 点不出告警。

const (
	BaselineWindowWeeks = 8
	MinBaselinePoints   = 6
	ColdStartBatches    = 6
	DailyAlertLimit     = 3
)

type SeriesKey struct {
	BrandID string
	Model   domain.AIModelID
	Region  string
	Metric  string
}

func (k SeriesKey) String() string {
	return strings.Join([]string{k.BrandID, string(k.Model), k.Region, k.Metric}, "|")
}

// RollingBaseline 维护按 SeriesKey 分组的滚动窗口点集。
type RollingBaseline struct {
	now    func() time.Time
	series map[string][]domain.BaselinePoint
}

func NewRollingBaseline(now func() time.Time) *RollingBaseline {
	if now == nil {
		now = time.Now
	}
	return &RollingBaseline{now: now, series: map[string][]domain.BaselinePoint{}}
}

// Record 写入一个批次点。失败批次（valid=false）直接丢弃，绝不进基线。
func (b *RollingBaseline) Record(key SeriesKey, p domain.BaselinePoint) bool {
	if !p.Valid {
		return false
	}
	k := key.String()
	b.series[k] = append(b.series[k], p)
	b.series[k] = b.withinWindow(b.series[k])
	return true
}

func (b *RollingBaseline) withinWindow(points []domain.BaselinePoint) []domain.BaselinePoint {
	cutoff := b.now().AddDate(0, 0, -7*BaselineWindowWeeks)
	out := make([]domain.BaselinePoint, 0, len(points))
	for _, p := range points {
		t, err := time.Parse("2006-01-02", p.ObservedOn)
		if err != nil || !t.Before(cutoff) {
			out = append(out, p)
		}
	}
	return out
}

// Stats 返回均值与离散度（标准差）。点数不足时 ready=false。
func (b *RollingBaseline) Stats(key SeriesKey) (mean, stddev float64, ready bool) {
	points := b.series[key.String()]
	valid := make([]float64, 0, len(points))
	for _, p := range points {
		if p.Valid {
			valid = append(valid, p.Value)
		}
	}
	if len(valid) < MinBaselinePoints {
		return 0, 0, false
	}
	for _, v := range valid {
		mean += v
	}
	mean /= float64(len(valid))
	for _, v := range valid {
		stddev += (v - mean) * (v - mean)
	}
	stddev = math.Sqrt(stddev / float64(len(valid)))
	return mean, stddev, true
}

func (b *RollingBaseline) BatchCount(key SeriesKey) int {
	return len(b.series[key.String()])
}

// ---- A-WP-6.2 突变判定与 2 批确认 ----

type AnomalyConfig struct {
	SigmaThreshold float64
	MinDeviation   float64
	ConfirmBatches int
}

func DefaultAnomalyConfig() AnomalyConfig {
	return AnomalyConfig{SigmaThreshold: 2.0, MinDeviation: 0.12, ConfirmBatches: 2}
}

// AnomalyInput 是被检验的一个候选批次。
type AnomalyInput struct {
	Key       SeriesKey
	Point     domain.BaselinePoint
	Seasonal  bool    // 已知季节性窗口
	FaultRate float64 // 该批次采集故障比例，>=0.3 视为采集故障
}

// ClassifyShift 区分真实突变 / 季节性 / 采集故障；只有真实突变需要告警。
// 冷启动（< 6 批）静默；偏离需连续 2 批确认。
func ClassifyShift(b *RollingBaseline, in AnomalyInput, cfg AnomalyConfig, pending map[string]int) (domain.AnomalyKind, bool) {
	if !in.Point.Valid {
		return domain.AnomalyCollectFault, false
	}
	if b.BatchCount(in.Key) < ColdStartBatches {
		return "", false // 冷启动静默
	}
	if in.Seasonal {
		return domain.AnomalySeasonal, false
	}
	if in.FaultRate >= 0.3 {
		return domain.AnomalyCollectFault, false
	}
	mean, stddev, ready := b.Stats(in.Key)
	if !ready {
		return "", false
	}
	dev := in.Point.Value - mean
	if math.Abs(dev) < cfg.MinDeviation {
		pending[in.Key.String()] = 0
		return "", false
	}
	if stddev > 0 && math.Abs(dev) < cfg.SigmaThreshold*stddev {
		pending[in.Key.String()] = 0
		return "", false
	}
	// 连续 2 批同向偏离才确认
	prev := pending[in.Key.String()]
	sameDirection := (prev > 0 && dev > 0) || (prev < 0 && dev < 0)
	if !sameDirection {
		pending[in.Key.String()] = signInt(dev)
		return domain.AnomalyRealShift, false
	}
	pending[in.Key.String()] += signInt(dev)
	if absInt(pending[in.Key.String()]) < cfg.ConfirmBatches {
		return domain.AnomalyRealShift, false
	}
	delete(pending, in.Key.String())
	return domain.AnomalyRealShift, true
}

func signInt(v float64) int {
	if v < 0 {
		return -1
	}
	return 1
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ---- A-WP-6.3 告警证据 / 折叠 / CSV ----

type AlertStore struct {
	alerts []domain.AnomalyAlert
}

func NewAlertStore() *AlertStore { return &AlertStore{alerts: []domain.AnomalyAlert{}} }

// Emit 生成一条告警。每条必须带 run_id 与非因果说明；日上限 3 条，第 4 条自动折叠。
func (s *AlertStore) Emit(key SeriesKey, kind domain.AnomalyKind, dev float64, runIDs []string, observedOn string, nValid int) domain.AnomalyAlert {
	if len(runIDs) == 0 {
		runIDs = []string{"run:unavailable"}
	}
	idData := key.String() + "|" + observedOn + "|" + fmt.Sprintf("%.4f", dev)
	sum := sha256.Sum256([]byte(idData))
	dayCount := 0
	for _, a := range s.alerts {
		if a.ObservedOn == observedOn {
			dayCount++
		}
	}
	a := domain.AnomalyAlert{
		ID: hex.EncodeToString(sum[:])[:12], BrandID: key.BrandID,
		Model: key.Model, Region: key.Region, Metric: key.Metric,
		Kind: kind, Deviation: dev, RunIDs: runIDs,
		NonCausal: domain.AnomalyNonCausal, Folded: dayCount >= DailyAlertLimit,
		ObservedOn: observedOn,
		Provenance: domain.ObservedProvenance(nValid),
	}
	s.alerts = append(s.alerts, a)
	return a
}

func (s *AlertStore) All() []domain.AnomalyAlert { return s.alerts }

// ValidateAlerts 是 CI 断言：告警 100% 带 run_id 与非因果说明。
func ValidateAlerts(alerts []domain.AnomalyAlert) []string {
	errs := []string{}
	for _, a := range alerts {
		if len(a.RunIDs) == 0 {
			errs = append(errs, a.ID+" 缺少 run_id")
		}
		if a.NonCausal == "" {
			errs = append(errs, a.ID+" 缺少非因果说明")
		}
		if a.Provenance.NValid < 0 {
			errs = append(errs, a.ID+" n_valid 非法")
		}
	}
	return errs
}

// ExportCSV 导出告警历史（A-WP-6.3）。
func (s *AlertStore) ExportCSV() string {
	var b strings.Builder
	b.WriteString("id,observed_on,brand_id,model,region,metric,kind,deviation,run_ids,n_valid,folded,non_causal\n")
	for _, a := range s.alerts {
		b.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%.4f,%s,%d,%t,%s\n",
			a.ID, a.ObservedOn, a.BrandID, a.Model, a.Region, a.Metric, a.Kind, a.Deviation,
			strings.Join(a.RunIDs, " "), a.Provenance.NValid, a.Folded, csvQuote(a.NonCausal)))
	}
	return b.String()
}

func csvQuote(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// ---- A-WP-6.4 Webhook 幂等投递 ----

type WebhookDelivery struct {
	seen map[string]bool
}

func NewWebhookDelivery() *WebhookDelivery {
	return &WebhookDelivery{seen: map[string]bool{}}
}

// Deliver 返回 (是否实际投递, 是否重复)。重放同一 event_id 不会重复投递。
func (w *WebhookDelivery) Deliver(eventID string) (delivered bool, duplicate bool) {
	if eventID == "" {
		return false, false
	}
	if w.seen[eventID] {
		return false, true
	}
	w.seen[eventID] = true
	return true, false
}

// SortedAlerts 按日期与 ID 稳定排序，便于回放测试。
func SortedAlerts(alerts []domain.AnomalyAlert) []domain.AnomalyAlert {
	out := append([]domain.AnomalyAlert{}, alerts...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ObservedOn != out[j].ObservedOn {
			return out[i].ObservedOn < out[j].ObservedOn
		}
		return out[i].ID < out[j].ID
	})
	return out
}
