// Package collector 实现基线 WBS WP-1「采集与桌面会话适配器」的可运行框架：
// 沙箱申请 → computer-use 驱动 → 会话生命周期 → 产物落 run_answers（WP-1.2），
// 首个桌面适配器（WP-1.3），按秒成本归集写入 F-107（WP-1.4）。
//
// 合规红线（WBS §7.1 / 16 款文案 §5.2）：沙箱采集仅限公开可访问形态，
// 不绕付费墙、不模拟登录态批量抓取。Policy 在会话启动前强制校验，违规直接
// 拒绝而不是「记录后继续」。Offstage Cloud 是 Sandbox 接口的一个生产实现，
// 本地用可注入的 FakeSandbox 跑通全链路与成功率验收。
package collector

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/aeo"
	"github.com/google/uuid"
)

var (
	ErrPolicyViolation = errors.New("compliance violation: 仅允许公开可访问形态，不绕付费墙、不模拟登录态")
	ErrSandboxTimeout  = errors.New("sandbox timeout")
	ErrSandboxCrash    = errors.New("sandbox crash")
)

// Policy 是采集策略，默认即最严格。
type Policy struct {
	PublicOnly         bool `json:"public_only"`
	AllowLoginState    bool `json:"allow_login_state"`
	AllowPaywallBypass bool `json:"allow_paywall_bypass"`
}

func DefaultPolicy() Policy {
	return Policy{PublicOnly: true, AllowLoginState: false, AllowPaywallBypass: false}
}

func (p Policy) Validate() error {
	if !p.PublicOnly || p.AllowLoginState || p.AllowPaywallBypass {
		return ErrPolicyViolation
	}
	return nil
}

// SandboxRequest 是一次沙箱申请。Target 必须是公开 URL。
type SandboxRequest struct {
	Target     string `json:"target"`
	Public     bool   `json:"public"`
	LoginState bool   `json:"login_state"`
	Paywalled  bool   `json:"paywalled"`
}

func (r SandboxRequest) Validate(p Policy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if !r.Public || r.LoginState || r.Paywalled {
		return fmt.Errorf("%w: target=%s public=%v login=%v paywalled=%v",
			ErrPolicyViolation, r.Target, r.Public, r.LoginState, r.Paywalled)
	}
	return nil
}

// SessionPhase 是会话生命周期状态。
type SessionPhase string

const (
	PhaseAcquiring SessionPhase = "acquiring"
	PhaseDriving   SessionPhase = "driving"
	PhaseClosed    SessionPhase = "closed"
	PhaseFailed    SessionPhase = "failed"
)

// Session 是一次桌面会话，带按秒计时。
type Session struct {
	ID        string       `json:"id"`
	Platform  string       `json:"platform"`
	Phase     SessionPhase `json:"phase"`
	StartedAt time.Time    `json:"started_at"`
	EndedAt   *time.Time   `json:"ended_at,omitempty"`
	Seconds   float64      `json:"seconds"`
}

// DriveResult 是 computer-use 驱动一次的产物。
type DriveResult struct {
	RawAnswerText  string
	Citations      []domain.Citation
	SearchActivity domain.SearchActivity
	ForcedSearch   bool
	// Simulated 为 true 时表示本地模拟产物，不得冒充第三方实测数据。
	Simulated bool
	Err       error
}

// Sandbox 是可插拔端口：生产实现为 Offstage Cloud 按秒计费 macOS 沙箱。
type Sandbox interface {
	Acquire(req SandboxRequest) (Session, error)
	Drive(s Session, question string) DriveResult
	Release(s Session) error
}

// DesktopAdapter 是桌面型模型适配器接口。新增一个模型只需实现该接口并注册，
// 不需改动框架（WP-1.2 验收：新增桌面型模型 ≤ 3 人日）。
type DesktopAdapter interface {
	Platform() domain.AIModelID
	PublicTarget() SandboxRequest
	ModelVersion() string
}

// ---- ChatGPT 桌面端适配器（WP-1.3）----

type ChatGPTDesktopAdapter struct{}

func (ChatGPTDesktopAdapter) Platform() domain.AIModelID { return domain.ModelChatGPT }
func (ChatGPTDesktopAdapter) PublicTarget() SandboxRequest {
	return SandboxRequest{
		Target: "https://chatgpt.com/", Public: true,
		LoginState: false, Paywalled: false,
	}
}
func (ChatGPTDesktopAdapter) ModelVersion() string { return "chatgpt-desktop-public-2026-11" }

// ---- 运行记录与成本 ----

// RunRecord 是落 run_answers 的记录 + 成本明细 + 五元组成本口径。
type RunRecord struct {
	Answer    domain.RunAnswer     `json:"answer"`
	Cost      domain.CostBreakdown `json:"cost_breakdown"`
	Channel   string               `json:"channel"`
	Simulated bool                 `json:"simulated"`
}

// MarginRow 是分模型毛利表的一行，含「桌面通道」。
type MarginRow struct {
	Channel   string  `json:"channel"`
	Platform  string  `json:"platform"`
	Runs      int     `json:"runs"`
	ValidRuns int     `json:"valid_runs"`
	CostUSD   float64 `json:"cost_usd"`
}

// BatchReport 是批次采集报告。
type BatchReport struct {
	Total       int      `json:"total"`
	Valid       int      `json:"valid"`
	Failed      int      `json:"failed"`
	SuccessRate float64  `json:"success_rate"`
	CostUSD     float64  `json:"cost_usd"`
	RunIDs      []string `json:"run_ids"`
}

// Framework 是通用采集框架。
type Framework struct {
	mu       sync.Mutex
	policy   Policy
	sandbox  Sandbox
	adapters map[domain.AIModelID]DesktopAdapter
	runs     map[string]RunRecord
	rates    aeo.CostRates
	clock    func() time.Time
}

func NewFramework(s Sandbox, clock func() time.Time) *Framework {
	if clock == nil {
		clock = time.Now
	}
	f := &Framework{
		policy: DefaultPolicy(), sandbox: s,
		adapters: map[domain.AIModelID]DesktopAdapter{},
		runs:     map[string]RunRecord{}, rates: aeo.DefaultCostRates(), clock: clock,
	}
	// 注册首个桌面适配器
	_ = f.Register(ChatGPTDesktopAdapter{})
	return f
}

// Register 注册新的桌面型模型适配器。
func (f *Framework) Register(a DesktopAdapter) error {
	if err := a.PublicTarget().Validate(f.policy); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.adapters[a.Platform()] = a
	return nil
}

// Adapters 返回已注册平台。
func (f *Framework) Adapters() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []string{}
	for k := range f.adapters {
		out = append(out, string(k))
	}
	sort.Strings(out)
	return out
}

// Collect 采集单题：完整会话生命周期 + 失败枚举 + 成本归集 + 落库。
func (f *Framework) Collect(platform domain.AIModelID, question domain.DetectionQuery, brandID string, region string) (RunRecord, error) {
	f.mu.Lock()
	adapter, ok := f.adapters[platform]
	f.mu.Unlock()
	if !ok {
		return RunRecord{}, fmt.Errorf("未注册桌面适配器: %s", platform)
	}
	if err := adapter.PublicTarget().Validate(f.policy); err != nil {
		return RunRecord{}, err
	}

	session, err := f.sandbox.Acquire(adapter.PublicTarget())
	if err != nil {
		return f.failedRecord(platform, question, brandID, region, err), err
	}
	started := f.clock()
	res := f.sandbox.Drive(session, question.Text)
	ended := f.clock()
	seconds := ended.Sub(started).Seconds()
	if seconds < 0 {
		seconds = 0
	}
	_ = f.sandbox.Release(session)

	runID := uuid.NewString()
	answer := domain.RunAnswer{
		RunID: runID, QuestionID: question.ID, QuestionText: question.Text,
		Intent: question.Intent, Platform: platform, ModelVersion: adapter.ModelVersion(),
		Region: region, Locale: "zh-CN", ObservedOn: started.Format("2006-01-02"),
		CollectedAt: started, Valid: res.Err == nil,
	}
	if res.Err != nil {
		answer.FailureType = classifyFailure(res.Err)
		answer.RawAnswerText = ""
	} else {
		answer.RawAnswerText = res.RawAnswerText
		answer.Citations = res.Citations
		answer.SearchActivity = res.SearchActivity
		answer.ForcedSearch = res.ForcedSearch
		answer.Mention = domain.Mention{BrandID: brandID}
	}

	// WP-1.4 / A-WP-8.1：按秒沙箱费用 + 模型调用 + 检索，统一写入 F-107 cost。
	retrievals := 0
	if !answer.ForcedSearch && res.Err == nil {
		retrievals = 1
	}
	breakdown := aeo.ComputeCost(domain.CostPerSecond, 1, seconds, retrievals, f.rates)
	answer.Cost = breakdown.Total
	// A-WP-8.2 对账：明细之和必须等于写入的 cost。
	if delta, ok := aeo.ReconcileCost(breakdown, answer.Cost, 1); !ok {
		return RunRecord{}, fmt.Errorf("成本对账失败，差额 %.6f", delta)
	}

	rec := RunRecord{Answer: answer, Cost: breakdown, Channel: "desktop", Simulated: res.Simulated}
	f.mu.Lock()
	f.runs[runID] = rec
	f.mu.Unlock()
	return rec, nil
}

// failedRecord 把沙箱超时/崩溃写入 F-106 失败枚举，绝不静默丢弃（WP-1.2 验收）。
func (f *Framework) failedRecord(platform domain.AIModelID, q domain.DetectionQuery, brandID, region string, cause error) RunRecord {
	now := f.clock()
	rec := RunRecord{
		Answer: domain.RunAnswer{
			RunID: uuid.NewString(), QuestionID: q.ID, QuestionText: q.Text,
			Intent: q.Intent, Platform: platform, Region: region, Locale: "zh-CN",
			ObservedOn: now.Format("2006-01-02"), CollectedAt: now,
			Valid: false, FailureType: classifyFailure(cause),
			Mention: domain.Mention{BrandID: brandID},
		},
		Cost:    aeo.ComputeCost(domain.CostPerSecond, 1, 0, 0, f.rates),
		Channel: "desktop",
	}
	f.mu.Lock()
	f.runs[rec.Answer.RunID] = rec
	f.mu.Unlock()
	return rec
}

func classifyFailure(err error) domain.FailureType {
	switch {
	case errors.Is(err, ErrSandboxTimeout):
		return domain.FailureTimeout
	case errors.Is(err, ErrSandboxCrash), errors.Is(err, ErrPolicyViolation):
		return domain.FailureBlocked
	default:
		return domain.FailureLoading
	}
}

// CollectBatch 跑 30 题批次（WP-1.3 验收：成功率 ≥ 80%，forced_search 正确记录）。
func (f *Framework) CollectBatch(platform domain.AIModelID, questions []domain.DetectionQuery, brandID, region string) BatchReport {
	report := BatchReport{Total: len(questions), RunIDs: []string{}}
	for _, q := range questions {
		rec, _ := f.Collect(platform, q, brandID, region)
		report.RunIDs = append(report.RunIDs, rec.Answer.RunID)
		report.CostUSD += rec.Answer.Cost
		if rec.Answer.Valid {
			report.Valid++
		} else {
			report.Failed++
		}
	}
	if report.Total > 0 {
		report.SuccessRate = float64(report.Valid) / float64(report.Total)
	}
	return report
}

// RunByID 让「任一 run 可查其沙箱成本」成立。
func (f *Framework) RunByID(runID string) (RunRecord, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[runID]
	return r, ok
}

// Runs 返回全部落库记录。
func (f *Framework) Runs() []RunRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]RunRecord, 0, len(f.runs))
	for _, r := range f.runs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Answer.RunID < out[j].Answer.RunID })
	return out
}

// Margins 是分模型毛利表，含「桌面通道」一行（WP-1.4 验收）。
func (f *Framework) Margins() []MarginRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	type key struct{ channel, platform string }
	m := map[key]*MarginRow{}
	for _, r := range f.runs {
		k := key{r.Channel, string(r.Answer.Platform)}
		row, ok := m[k]
		if !ok {
			row = &MarginRow{Channel: r.Channel, Platform: k.platform}
			m[k] = row
		}
		row.Runs++
		if r.Answer.Valid {
			row.ValidRuns++
		}
		row.CostUSD += r.Answer.Cost
	}
	out := make([]MarginRow, 0, len(m))
	for _, row := range m {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Platform < out[j].Platform })
	return out
}
