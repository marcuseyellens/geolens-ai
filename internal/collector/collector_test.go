package collector

import (
	"errors"
	"testing"
	"time"

	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/aeo"
)

// fakeSandbox 是可注入的本地沙箱实现。failEvery 让确定比例的采集失败，
// 用于「同一问题连续 10 次成功率 ≥ 80%」与「30 题批次成功率 ≥ 80%」验收。
type fakeSandbox struct {
	failEvery   int
	calls       int
	seconds     float64
	acquireFail error
	forced      bool
	released    int
}

func (s *fakeSandbox) Acquire(req SandboxRequest) (Session, error) {
	if s.acquireFail != nil {
		return Session{Phase: PhaseFailed}, s.acquireFail
	}
	return Session{
		ID:       "sess-" + time.Now().Format("150405.000000000"),
		Platform: "chatgpt-desktop", Phase: PhaseDriving, StartedAt: time.Now(),
	}, nil
}

func (s *fakeSandbox) Drive(_ Session, question string) DriveResult {
	s.calls++
	if s.failEvery > 0 && s.calls%s.failEvery == 0 {
		return DriveResult{Err: ErrSandboxTimeout}
	}
	return DriveResult{
		RawAnswerText: "公开形态下采集到的答案原文：" + question,
		Citations: []domain.Citation{{
			URL: "https://example.com/public-review", Domain: "example.com",
			PagePath: "/public-review", SourceClass: domain.SourceIndependentRoundup,
		}},
		SearchActivity: domain.SearchOn, ForcedSearch: s.forced,
	}
}

func (s *fakeSandbox) Release(_ Session) error {
	s.released++
	return nil
}

// clockStep 每次调用推进 1 秒，让按秒成本确定可测。
func clockStep() func() time.Time {
	t := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	return func() time.Time {
		out := t
		t = t.Add(time.Second)
		return out
	}
}

func questions(n int) []domain.DetectionQuery {
	out := make([]domain.DetectionQuery, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, domain.DetectionQuery{
			ID:   "q-" + string(rune('a'+i%26)) + "-" + time.Now().Format("150405.000000000"),
			Text: "推荐一个 AEO 平台", Intent: domain.IntentRecommend,
		})
	}
	return out
}

// WP-1.1 PoC：同一问题连续 10 次采集成功率 ≥ 80%，单次成本可精确归集。
func TestPoCTenConsecutiveCollects(t *testing.T) {
	sb := &fakeSandbox{failEvery: 10} // 10 次里失败 1 次 = 90%
	f := NewFramework(sb, clockStep())
	q := domain.DetectionQuery{ID: "q-same", Text: "推荐一个 AEO 平台", Intent: domain.IntentRecommend}
	valid := 0
	for i := 0; i < 10; i++ {
		rec, err := f.Collect(domain.ModelChatGPT, q, "brand:geolens", "us")
		if err != nil {
			t.Fatalf("第 %d 次采集框架错误: %v", i, err)
		}
		if rec.Answer.Valid {
			valid++
		}
		if rec.Answer.Cost <= 0 {
			t.Errorf("第 %d 次成本未归集", i)
		}
	}
	if float64(valid)/10 < 0.8 {
		t.Errorf("PoC 成功率 %.2f < 0.8", float64(valid)/10)
	}
}

// WP-1.3：30 题批次成功率 ≥ 80%，forced_search 状态被正确记录。
func TestBatchThirtyQuestionsAndForcedSearch(t *testing.T) {
	sb := &fakeSandbox{failEvery: 10, forced: true}
	f := NewFramework(sb, clockStep())
	report := f.CollectBatch(domain.ModelChatGPT, questions(30), "brand:geolens", "us")
	if report.Total != 30 {
		t.Fatalf("批次题数应为 30，实际 %d", report.Total)
	}
	if report.SuccessRate < 0.8 {
		t.Errorf("批次成功率 %.2f < 0.8", report.SuccessRate)
	}
	if report.Valid+report.Failed != 30 {
		t.Error("有效 + 失败应等于总题数（失败不得静默丢弃）")
	}
	forcedSeen := 0
	for _, id := range report.RunIDs {
		rec, ok := f.RunByID(id)
		if !ok {
			t.Fatalf("run %s 未落库", id)
		}
		if rec.Answer.Valid && rec.Answer.ForcedSearch {
			forcedSeen++
		}
		if rec.Answer.Valid && rec.Answer.SearchActivity != domain.SearchOn {
			t.Errorf("run %s 搜索开关状态未记录", id)
		}
	}
	if forcedSeen == 0 {
		t.Error("forced_search 状态未被正确记录")
	}
}

// WP-1.2 / 合规红线：沙箱超时/崩溃进入 F-106 失败枚举而非静默丢弃；
// 登录态/付费墙路径直接拒绝。
func TestFailuresEnterEnumAndComplianceRedLine(t *testing.T) {
	sb := &fakeSandbox{acquireFail: ErrSandboxCrash}
	f := NewFramework(sb, clockStep())
	rec, err := f.Collect(domain.ModelChatGPT, domain.DetectionQuery{ID: "q1", Text: "x"}, "b", "us")
	if !errors.Is(err, ErrSandboxCrash) {
		t.Fatalf("应返回沙箱崩溃错误，实际 %v", err)
	}
	if rec.Answer.Valid || rec.Answer.FailureType != domain.FailureBlocked {
		t.Errorf("崩溃应落入失败枚举 blocked，实际 %+v", rec.Answer.FailureType)
	}
	if _, ok := f.RunByID(rec.Answer.RunID); !ok {
		t.Error("失败运行也必须落库，不得静默丢弃")
	}

	sb2 := &fakeSandbox{acquireFail: ErrSandboxTimeout}
	f2 := NewFramework(sb2, clockStep())
	rec2, _ := f2.Collect(domain.ModelChatGPT, domain.DetectionQuery{ID: "q2"}, "b", "us")
	if rec2.Answer.FailureType != domain.FailureTimeout {
		t.Errorf("超时应落入 timeout，实际 %s", rec2.Answer.FailureType)
	}

	// 合规红线：注册一个模拟登录态的适配器必须被拒绝。
	bad := loginAdapter{}
	if err := f.Register(bad); !errors.Is(err, ErrPolicyViolation) {
		t.Errorf("登录态适配器未被拒绝: %v", err)
	}
	if err := (SandboxRequest{Target: "https://chatgpt.com/", Public: true, Paywalled: true}).Validate(DefaultPolicy()); !errors.Is(err, ErrPolicyViolation) {
		t.Error("付费墙路径未被拒绝")
	}
}

type loginAdapter struct{}

func (loginAdapter) Platform() domain.AIModelID { return domain.ModelClaude }
func (loginAdapter) PublicTarget() SandboxRequest {
	return SandboxRequest{Target: "https://claude.ai/", Public: true, LoginState: true}
}
func (loginAdapter) ModelVersion() string { return "invalid" }

// WP-1.2：新增一个桌面型模型只需实现接口并注册，不改框架。
func TestNewDesktopAdapterIsRegistrationOnly(t *testing.T) {
	f := NewFramework(&fakeSandbox{}, clockStep())
	if err := f.Register(publicClaudeAdapter{}); err != nil {
		t.Fatalf("合规的新适配器应可直接注册: %v", err)
	}
	got := f.Adapters()
	if len(got) != 2 || got[0] != "chatgpt" || got[1] != "claude" {
		t.Errorf("注册表应含两个平台，实际 %v", got)
	}
}

type publicClaudeAdapter struct{}

func (publicClaudeAdapter) Platform() domain.AIModelID { return domain.ModelClaude }
func (publicClaudeAdapter) PublicTarget() SandboxRequest {
	return SandboxRequest{Target: "https://claude.ai/public", Public: true}
}
func (publicClaudeAdapter) ModelVersion() string { return "claude-desktop-public" }

// WP-1.4 + A-WP-8.2：按秒成本写入 F-107，明细可拆三项，对账无差额，毛利表含桌面通道。
func TestPerSecondCostReconciliationAndMarginTable(t *testing.T) {
	f := NewFramework(&fakeSandbox{}, clockStep())
	rec, err := f.Collect(domain.ModelChatGPT, domain.DetectionQuery{ID: "q1", Text: "x"}, "b", "us")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Cost.Kind != domain.CostPerSecond {
		t.Errorf("计费类型应为 per_second，实际 %s", rec.Cost.Kind)
	}
	if rec.Cost.SandboxSeconds <= 0 || rec.Cost.SandboxCost <= 0 {
		t.Errorf("沙箱秒数与费用未归集: %+v", rec.Cost)
	}
	if rec.Cost.ModelCallCost <= 0 || rec.Cost.RetrievalCost < 0 {
		t.Errorf("成本未拆成模型调用/检索: %+v", rec.Cost)
	}
	if delta, ok := aeo.ReconcileCost(rec.Cost, rec.Answer.Cost, 1); !ok {
		t.Errorf("对账有差额 %.6f", delta)
	}
	rows := f.Margins()
	if len(rows) != 1 || rows[0].Channel != "desktop" {
		t.Fatalf("毛利表应含「桌面通道」一行，实际 %+v", rows)
	}
	if rows[0].CostUSD != rec.Answer.Cost {
		t.Error("毛利表成本与 F-107 不一致")
	}
}
