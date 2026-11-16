// Package governance 实现基线 WBS WP-3「运行治理三硬规则」：
// 成本分级路由（3.1）、运行五元组落库（3.2）、预算熔断在路由层强制（3.3）、
// 限额 wait-and-resume（3.4）。
//
// 关键不变量：Dispatch 是所有运行（含重试）的唯一入口。预算状态是引擎级持久
// 状态，重试携带 RetryOf 也必须先过熔断检查，因此「重试不得绕过熔断」由构造保证。
package governance

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/geolens/geolens/domain"
	"github.com/google/uuid"
)

var (
	ErrBudgetTripped = errors.New("budget circuit tripped: 运行被路由层拒绝，重试不可绕过")
	ErrRateLimited   = errors.New("rate limited: 进入限速窗口排队，恢复后自动续跑")
	ErrUnknownTask   = errors.New("unknown task: 路由表未配置该任务")
	ErrQualityLocked = errors.New("quality critical task cannot be downgraded")
)

// Clock 可注入，便于测试模拟时间推进。
type Clock func() time.Time

// RunRequest 是一次运行申请。
type RunRequest struct {
	Operator string
	Task     domain.TaskKind
	Change   string
	Platform string // 采集平台，用于 429 判定
	// RetryOf 非空表示这是一次重试；重试同样经过 Dispatch 的熔断检查。
	RetryOf string
	// CustomCost 覆盖路由表预估成本（实际运行回填真实成本）。
	CustomCost *float64
}

// RunResult 是运行裁决结果。
type RunResult struct {
	RunID    string
	Rule     domain.RouteRule
	Outcome  string
	Cost     float64
	ResumeAt time.Time // 被限速时给出恢复时间
}

// PendingRun 是 wait-and-resume 队列里的一条。
type PendingRun struct {
	RunID    string     `json:"run_id"`
	Request  RunRequest `json:"request"`
	QueuedAt time.Time  `json:"queued_at"`
	ResumeAt time.Time  `json:"resume_at"`
}

// OperatorSummary 是审计页按 operator 汇总的一行。
type OperatorSummary struct {
	Operator string  `json:"operator"`
	Runs     int     `json:"runs"`
	CostUSD  float64 `json:"cost_usd"`
	Rejected int     `json:"rejected"`
}

// Engine 是运行治理引擎。
type Engine struct {
	mu            sync.Mutex
	clock         Clock
	rules         map[domain.TaskKind]domain.RouteRule
	limit         float64
	spent         float64
	tripped       bool
	trippedAt     time.Time
	rejectedRuns  int
	trippedEvents int

	pentads    []domain.RunPentad
	rateLimits map[string]time.Time // platform -> resumeAt
	pending    []PendingRun
	alerts     []BudgetEvent
}

// BudgetEvent 是 F-7xx 告警事件；熔断与恢复都记录。
type BudgetEvent struct {
	Kind      string    `json:"kind"` // tripped / rejected / resumed
	SpentUSD  float64   `json:"spent_usd"`
	LimitUSD  float64   `json:"limit_usd"`
	RunID     string    `json:"run_id,omitempty"`
	At        time.Time `json:"at"`
	NonCausal string    `json:"non_causal,omitempty"`
}

// NewEngine 创建引擎；budgetLimit <= 0 表示不限预算（测试可用）。
func NewEngine(budgetLimit float64, clock Clock) *Engine {
	if clock == nil {
		clock = time.Now
	}
	e := &Engine{
		clock: clock, rules: map[domain.TaskKind]domain.RouteRule{},
		limit: budgetLimit, rateLimits: map[string]time.Time{},
	}
	for _, r := range domain.DefaultRouteTable {
		e.rules[r.Task] = r
	}
	return e
}

// SetRouteRule 更新可配置路由表；QualityCritical 任务不得配置为低成本通道。
func (e *Engine) SetRouteRule(r domain.RouteRule) error {
	if r.QualityCritical && r.Channel != domain.ChannelStrongModel {
		return fmt.Errorf("%w: %s", ErrQualityLocked, r.Task)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules[r.Task] = r
	return nil
}

// RouteTable 返回当前路由表（按任务名排序），供审计页展示。
func (e *Engine) RouteTable() []domain.RouteRule {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sortedRules()
}

func (e *Engine) sortedRules() []domain.RouteRule {
	out := make([]domain.RouteRule, 0, len(e.rules))
	for _, r := range e.rules {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Task < out[j].Task })
	return out
}

// Downgrade 尝试把任务切到低成本通道；质量关键任务一律拒绝（同批次判定质量不降级）。
func (e *Engine) Downgrade(task domain.TaskKind) (domain.RouteRule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.rules[task]
	if !ok {
		return domain.RouteRule{}, ErrUnknownTask
	}
	if r.QualityCritical {
		return domain.RouteRule{}, ErrQualityLocked
	}
	r.Channel = domain.ChannelLowCost
	r.Model = "collector-batch-v1"
	e.rules[task] = r
	return r, nil
}

// Budget 返回预算状态快照。
func (e *Engine) Budget() domain.BudgetState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return domain.BudgetState{
		LimitUSD: e.limit, SpentUSD: e.spent, Tripped: e.tripped,
		TrippedAt: e.trippedAt, RejectedRuns: e.rejectedRuns,
		TrippedEvents: e.trippedEvents,
	}
}

// Dispatch 是路由层唯一入口：所有运行与重试都必须经过它。
// 检查顺序刻意固定为「熔断 → 限速 → 路由 → 预算」，保证熔断优先级最高。
func (e *Engine) Dispatch(req RunRequest) (RunResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.clock()
	runID := uuid.NewString()

	// 1) 熔断检查：优先级最高，重试、换 operator、换任务都不可绕过。
	if e.tripped {
		e.rejectedRuns++
		e.pentads = append(e.pentads, domain.RunPentad{
			RunID: runID, Operator: req.Operator, Task: req.Task, Change: req.Change,
			Cost: 0, Outcome: "budget_tripped", At: now, RetryOf: req.RetryOf,
		})
		e.alerts = append(e.alerts, BudgetEvent{
			Kind: "rejected", SpentUSD: e.spent, LimitUSD: e.limit, RunID: runID, At: now,
		})
		return RunResult{RunID: runID, Outcome: "budget_tripped"}, ErrBudgetTripped
	}

	// 2) 限速窗口：撞 429 时排队，不立即重试、不烧配额。
	if resumeAt, ok := e.rateLimits[req.Platform]; ok && now.Before(resumeAt) {
		e.pending = append(e.pending, PendingRun{
			RunID: runID, Request: req, QueuedAt: now, ResumeAt: resumeAt,
		})
		e.pentads = append(e.pentads, domain.RunPentad{
			RunID: runID, Operator: req.Operator, Task: req.Task, Change: req.Change,
			Cost: 0, Outcome: "rate_limited", At: now, RetryOf: req.RetryOf,
		})
		return RunResult{RunID: runID, Outcome: "rate_limited", ResumeAt: resumeAt}, ErrRateLimited
	}

	// 3) 路由：判定任务强制走强模型。
	rule, ok := e.rules[req.Task]
	if !ok {
		return RunResult{RunID: runID, Outcome: "failed"}, ErrUnknownTask
	}
	if rule.QualityCritical {
		rule.Channel = domain.ChannelStrongModel
	}
	cost := rule.EstimatedCost
	if req.CustomCost != nil {
		cost = *req.CustomCost
	}

	// 4) 预算强制：预估成本会击穿上限即熔断并告警，本次运行拒绝。
	if e.limit > 0 && e.spent+cost > e.limit {
		e.tripped = true
		e.trippedAt = now
		e.trippedEvents++
		e.rejectedRuns++
		e.pentads = append(e.pentads, domain.RunPentad{
			RunID: runID, Operator: req.Operator, Task: req.Task, Change: req.Change,
			Cost: 0, Outcome: "budget_tripped", Channel: rule.Channel, At: now, RetryOf: req.RetryOf,
		})
		e.alerts = append(e.alerts, BudgetEvent{
			Kind: "tripped", SpentUSD: e.spent, LimitUSD: e.limit, RunID: runID, At: now,
			NonCausal: "预算上限由本平台设定，熔断不代表任何外部平台规则",
		})
		return RunResult{RunID: runID, Rule: rule, Outcome: "budget_tripped"}, ErrBudgetTripped
	}

	e.spent += cost
	e.pentads = append(e.pentads, domain.RunPentad{
		RunID: runID, Operator: req.Operator, Task: req.Task, Change: req.Change,
		Cost: cost, Outcome: "ok", Channel: rule.Channel, At: now, RetryOf: req.RetryOf,
	})
	return RunResult{RunID: runID, Rule: rule, Outcome: "ok", Cost: cost}, nil
}

// ReportRateLimit 上报平台 429，登记恢复时间；不产生任何重试动作。
func (e *Engine) ReportRateLimit(sig domain.RateLimitSignal) {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.clock()
	resumeAt := sig.ResumeAt
	if resumeAt.IsZero() {
		resumeAt = now.Add(sig.RetryAfter)
	}
	e.rateLimits[sig.Platform] = resumeAt
}

// Resume 在时间到达后自动续跑队列：只续跑已到恢复时间的条目，且仍走 Dispatch
// 的熔断检查，避免恢复过程中绕过预算。
func (e *Engine) Resume() []RunResult {
	e.mu.Lock()
	now := e.clock()
	var ready []PendingRun
	keep := e.pending[:0]
	for _, p := range e.pending {
		if !now.Before(p.ResumeAt) {
			ready = append(ready, p)
		} else {
			keep = append(keep, p)
		}
	}
	e.pending = keep
	e.mu.Unlock()

	out := []RunResult{}
	for _, p := range ready {
		res, err := e.Dispatch(p.Request)
		if err == nil {
			out = append(out, res)
		}
	}
	return out
}

// Pending 返回当前排队条目快照。
func (e *Engine) Pending() []PendingRun {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]PendingRun, len(e.pending))
	copy(out, e.pending)
	return out
}

// Pentads 返回全部五元组（按时间倒序）。
func (e *Engine) Pentads() []domain.RunPentad {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]domain.RunPentad, len(e.pentads))
	copy(out, e.pentads)
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// PentadByRun 让「任一 run 可查五元组」成立。
func (e *Engine) PentadByRun(runID string) (domain.RunPentad, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, p := range e.pentads {
		if p.RunID == runID {
			return p, true
		}
	}
	return domain.RunPentad{}, false
}

// ByOperator 是审计页按 operator 汇总（WP-3.2 验收）。
func (e *Engine) ByOperator() []OperatorSummary {
	e.mu.Lock()
	defer e.mu.Unlock()
	m := map[string]*OperatorSummary{}
	for _, p := range e.pentads {
		s, ok := m[p.Operator]
		if !ok {
			s = &OperatorSummary{Operator: p.Operator}
			m[p.Operator] = s
		}
		s.Runs++
		s.CostUSD += p.Cost
		if p.Outcome != "ok" {
			s.Rejected++
		}
	}
	out := make([]OperatorSummary, 0, len(m))
	for _, s := range m {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Operator < out[j].Operator })
	return out
}

// BudgetEvents 返回熔断/拒绝/恢复事件，供 WP-8.2 监控看板消费。
func (e *Engine) BudgetEvents() []BudgetEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]BudgetEvent, len(e.alerts))
	copy(out, e.alerts)
	return out
}

// ResetBudget 是运维显式复位（不自动复位，避免重试绕过）。
func (e *Engine) ResetBudget(newLimit float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tripped = false
	e.spent = 0
	if newLimit > 0 {
		e.limit = newLimit
	}
	e.alerts = append(e.alerts, BudgetEvent{
		Kind: "resumed", SpentUSD: 0, LimitUSD: e.limit, At: e.clock(),
	})
}
