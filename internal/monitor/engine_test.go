package monitor

import (
	"testing"
	"time"

	"github.com/geolens/geolens/domain"
)

func monitorClock() func() time.Time {
	t := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

// WP-8.2：五块看板数值正确，所有数字带 provenance。
func TestFiveBoardsAndProvenance(t *testing.T) {
	e := NewEngine(monitorClock())
	e.Update(Sources{
		CollectionValid: 96, CollectionTotal: 100, DailyCostUSD: 12.5,
		BudgetTrips: 0, PaymentFailed: 1, PaymentTotal: 50, FunnelConversion: 0.062,
	})
	s := e.Snapshot()
	if s.CollectionSuccessRate != 0.96 || s.DailyCostUSD != 12.5 || s.PaymentFailureRate != 0.02 {
		t.Fatalf("看板数值错误: %+v", s.MonitorSnapshot)
	}
	rows := Boards(s)
	if len(rows) != 5 {
		t.Fatalf("应有五块看板，实际 %d", len(rows))
	}
	for _, r := range rows {
		if r.Provenance.NValid != 100 || r.Provenance.SourceType != domain.SourceObserved {
			t.Errorf("看板 %s 缺少口径标注: %+v", r.Key, r.Provenance)
		}
	}
	if len(s.Alerts) != 0 {
		t.Errorf("未越阈值不应告警: %+v", s.Alerts)
	}
	if len(e.History()) != 1 {
		t.Error("快照未留存历史")
	}
}

// WP-8.2：采集有效率 < 95% 与成本超日预算触发告警；熔断/支付失败同样告警。
func TestThresholdAlerts(t *testing.T) {
	e := NewEngine(monitorClock())
	e.Update(Sources{
		CollectionValid: 80, CollectionTotal: 100,
		DailyCostUSD: domain.ThresholdDailyCostUSD + 1,
		BudgetTrips:  2, PaymentFailed: 3, PaymentTotal: 10,
	})
	s := e.Snapshot()
	seen := map[string]bool{}
	for _, a := range s.Alerts {
		seen[a.Metric] = true
	}
	for _, k := range []string{"collection_success_rate", "daily_cost_usd", "budget_trips", "payment_failure_rate"} {
		if !seen[k] {
			t.Errorf("缺少 %s 告警: %+v", k, s.Alerts)
		}
	}
	for _, a := range s.Alerts {
		if a.Threshold <= 0 || a.Message == "" {
			t.Errorf("告警缺少阈值或说明: %+v", a)
		}
	}
}

// 无数据时不得把「无数据」显示成 0% 并误报告警。
func TestNoDataDoesNotFalseAlarm(t *testing.T) {
	e := NewEngine(monitorClock())
	s := e.Snapshot()
	if s.CollectionSuccessRate != 0 {
		t.Error("无数据时采集有效率应为 0 并标注样本不足")
	}
	for _, a := range s.Alerts {
		if a.Metric == "collection_success_rate" {
			t.Error("无数据时不应报采集有效率告警")
		}
	}
	if s.Provenance.NValid != 0 {
		t.Error("n_valid 应为 0")
	}
}

// WP-8.3：发布清单状态由证据计算，缺证据即 fail，不硬编码通过。
func TestReleaseChecklistDrivenByEvidence(t *testing.T) {
	full := ReleaseEvidence{
		LegalPagesReady: true, MethodologyNotesPresent: true,
		BootstrapFiles:   []string{"llms.txt", "robots.txt", "sitemap.xml"},
		SelfChecksPassed: true, AcceptanceChecklistGreen: true,
		FullChainRegressionPassed: true, BudgetCircuitTested: true,
		CostReconciled: true, MCPDocsPublished: true, MCPToolsReadOnly: true,
		MonitoringAlertsConfigured: true,
		RollbackPlan:               "按分支整体回滚 + 显式复位预算",
		RateLimitGrayRollout:       "先 10% 流量灰度，观察 429 队列",
	}
	items := ReleaseChecklist(full)
	if len(items) != 13 || !AllReleaseChecksPassed(items) {
		t.Fatalf("证据齐全时清单应全绿: %+v", items)
	}
	empty := ReleaseChecklist(ReleaseEvidence{})
	if AllReleaseChecksPassed(empty) {
		t.Error("缺证据时清单不应全绿")
	}
	for _, it := range empty {
		if it.Status != "fail" {
			t.Errorf("%s 无证据却标为 pass", it.ID)
		}
	}
}
