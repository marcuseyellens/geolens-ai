package governance

import (
	"errors"
	"testing"
	"time"

	"github.com/answerlens/answerlens/domain"
)

func fixedClock(t time.Time) Clock { return func() time.Time { return t } }

// WP-3.1：采集走低成本通道，三态判定走强模型，路由表可配置。
func TestCostTieredRouting(t *testing.T) {
	e := NewEngine(100, fixedClock(time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)))
	rules := map[domain.TaskKind]domain.RouteRule{}
	for _, r := range e.RouteTable() {
		rules[r.Task] = r
	}
	if rules[domain.TaskCollect].Channel != domain.ChannelLowCost {
		t.Error("采集任务应走低成本通道")
	}
	for _, task := range []domain.TaskKind{domain.TaskJudgeMention, domain.TaskJudgeRecommend, domain.TaskJudgeLink} {
		if rules[task].Channel != domain.ChannelStrongModel {
			t.Errorf("%s 应走强模型通道", task)
		}
	}
	if err := e.SetRouteRule(domain.RouteRule{
		Task: domain.TaskCollect, Channel: domain.ChannelStrongModel,
		Model: "custom", EstimatedCost: 0.01,
	}); err != nil {
		t.Fatalf("路由表应可配置: %v", err)
	}
	if e.RouteTable()[0].Model != "custom" && !containsModel(e.RouteTable(), "custom") {
		t.Error("配置更新未生效")
	}
}

func containsModel(rs []domain.RouteRule, m string) bool {
	for _, r := range rs {
		if r.Model == m {
			return true
		}
	}
	return false
}

// WP-3.1 验收：同批次判定质量不降级，质量关键任务不能被切到低成本通道。
func TestQualityCriticalNotDowngraded(t *testing.T) {
	e := NewEngine(100, fixedClock(time.Now()))
	if _, err := e.Downgrade(domain.TaskJudgeMention); !errors.Is(err, ErrQualityLocked) {
		t.Fatalf("判定任务不应允许降级，实际 %v", err)
	}
	if err := e.SetRouteRule(domain.RouteRule{
		Task: domain.TaskJudgeLink, Channel: domain.ChannelLowCost, QualityCritical: true,
	}); !errors.Is(err, ErrQualityLocked) {
		t.Fatalf("质量关键任务不得配置为低成本通道: %v", err)
	}
	res, err := e.Dispatch(RunRequest{Operator: "scheduler", Task: domain.TaskJudgeLink})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rule.Channel != domain.ChannelStrongModel {
		t.Error("判定任务实际执行必须走强模型")
	}
}

// WP-3.2：任一 run 可查五元组；审计页按 operator 汇总。
func TestPentadRecordedAndAggregated(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	e := NewEngine(100, fixedClock(now))
	r1, err := e.Dispatch(RunRequest{Operator: "alice", Task: domain.TaskCollect, Change: "batch-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = e.Dispatch(RunRequest{Operator: "alice", Task: domain.TaskGenerateFix, Change: "batch-1"})
	_, _ = e.Dispatch(RunRequest{Operator: "bob", Task: domain.TaskCollect, Change: "batch-2"})
	p, ok := e.PentadByRun(r1.RunID)
	if !ok || p.Operator != "alice" || p.Task != domain.TaskCollect || p.Change != "batch-1" || p.Outcome != "ok" {
		t.Fatalf("五元组查询失败: %+v", p)
	}
	if p.Cost <= 0 {
		t.Error("五元组成本应为正数")
	}
	var alice *OperatorSummary
	for _, s := range e.ByOperator() {
		if s.Operator == "alice" {
			cp := s
			alice = &cp
		}
	}
	if alice == nil || alice.Runs != 2 {
		t.Fatalf("按 operator 汇总错误: %+v", e.ByOperator())
	}
}

// WP-3.3 硬验收：预算熔断在路由层强制，重试不得绕过熔断（写测试证明）。
func TestBudgetCircuitCannotBeBypassedByRetry(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	// 上限 0.023：两次采集 0.004，再加一次判定 0.02 = 0.024 > 上限，触发熔断。
	e := NewEngine(0.023, fixedClock(now))
	for i := 0; i < 2; i++ {
		if _, err := e.Dispatch(RunRequest{Operator: "worker", Task: domain.TaskCollect}); err != nil {
			t.Fatalf("前两次运行不应失败: %v", err)
		}
	}
	// 再用一次高成本判定击穿上限，触发熔断
	tripped, err := e.Dispatch(RunRequest{Operator: "worker", Task: domain.TaskJudgeMention})
	if !errors.Is(err, ErrBudgetTripped) {
		t.Fatalf("应触发预算熔断，实际 %v", err)
	}
	state := e.Budget()
	if !state.Tripped || state.TrippedEvents != 1 {
		t.Fatalf("熔断状态未正确记录: %+v", state)
	}
	spentBefore := state.SpentUSD

	// 重试：换 operator、换任务、换通道、带 RetryOf，全部必须被拒绝。
	retries := []RunRequest{
		{Operator: "worker", Task: domain.TaskCollect, RetryOf: tripped.RunID},
		{Operator: "retry-daemon", Task: domain.TaskGenerateFix, RetryOf: tripped.RunID},
		{Operator: "scheduler", Task: domain.TaskJudgeLink, RetryOf: tripped.RunID},
		{Operator: "worker", Task: domain.TaskCollect}, // 不带 RetryOf 的重试同样拒绝
	}
	for i, req := range retries {
		res, err := e.Dispatch(req)
		if !errors.Is(err, ErrBudgetTripped) {
			t.Errorf("重试 %d 绕过了熔断！结果 %+v err=%v", i, res, err)
		}
		if res.Cost != 0 {
			t.Errorf("重试 %d 仍产生了成本", i)
		}
	}
	after := e.Budget()
	if after.SpentUSD != spentBefore {
		t.Errorf("熔断后成本仍在增长: %.4f -> %.4f", spentBefore, after.SpentUSD)
	}
	if after.RejectedRuns != 5 {
		t.Errorf("拒绝计数应为 1 次击穿 + 4 次重试 = 5，实际 %d", after.RejectedRuns)
	}
	// 熔断事件进 F-7xx 告警
	events := e.BudgetEvents()
	if len(events) == 0 || events[0].Kind != "tripped" {
		t.Errorf("熔断未入告警: %+v", events)
	}
	// 只有显式运维复位才能恢复
	e.ResetBudget(1)
	if e.Budget().Tripped {
		t.Error("显式复位后仍熔断")
	}
}

// WP-3.4：模拟 429 场景不立即重试、不烧配额；恢复后自动续跑。
func TestRateLimitWaitAndResume(t *testing.T) {
	base := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	current := base
	clock := func() time.Time { return current }
	e := NewEngine(100, clock)

	before := e.Budget().SpentUSD
	e.ReportRateLimit(domain.RateLimitSignal{Platform: "chatgpt", RetryAfter: 30 * time.Second})
	res, err := e.Dispatch(RunRequest{Operator: "collector", Task: domain.TaskCollect, Platform: "chatgpt"})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("撞 429 应进入等待，实际 %v", err)
	}
	if res.ResumeAt.IsZero() {
		t.Error("应返回恢复时间")
	}
	if e.Budget().SpentUSD != before {
		t.Error("等待期间不得烧配额/成本")
	}
	if len(e.Pending()) != 1 {
		t.Fatalf("应排队 1 条，实际 %d", len(e.Pending()))
	}
	// 窗口内再次调用：仍排队，不立即重试
	current = base.Add(10 * time.Second)
	if _, err := e.Dispatch(RunRequest{Operator: "collector", Task: domain.TaskCollect, Platform: "chatgpt"}); !errors.Is(err, ErrRateLimited) {
		t.Fatal("限速窗口内不应立即重试")
	}
	// 窗口未到时 Resume 不续跑
	if got := e.Resume(); len(got) != 0 {
		t.Errorf("未到恢复时间不应续跑，实际 %d 条", len(got))
	}
	// 时间到达后自动续跑
	current = base.Add(31 * time.Second)
	got := e.Resume()
	if len(got) != 2 || got[0].Outcome != "ok" {
		t.Fatalf("恢复后应自动续跑 2 条，实际 %+v", got)
	}
	if len(e.Pending()) != 0 {
		t.Errorf("队列应清空，实际 %d", len(e.Pending()))
	}
	if e.Budget().SpentUSD <= before {
		t.Error("续跑后应正常计费")
	}
}

// 续跑仍受熔断约束，防止借恢复过程绕过预算。
func TestResumeStillHonorsBudgetCircuit(t *testing.T) {
	base := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	current := base
	// 上限 0.001 低于单次采集 0.002：续跑时应被熔断拦截。
	e := NewEngine(0.001, func() time.Time { return current })
	e.ReportRateLimit(domain.RateLimitSignal{Platform: "claude", RetryAfter: time.Second})
	if _, err := e.Dispatch(RunRequest{Operator: "c", Task: domain.TaskCollect, Platform: "claude"}); !errors.Is(err, ErrRateLimited) {
		t.Fatal(err)
	}
	current = base.Add(2 * time.Second)
	if got := e.Resume(); len(got) != 0 {
		t.Errorf("预算已耗尽时续跑应被熔断拦截，实际成功 %d 条", len(got))
	}
	if !e.Budget().Tripped {
		t.Error("续跑触发的熔断未记录")
	}
}
