package competitor

import (
	"fmt"
	"net"
	"sort"

	"github.com/geolens/geolens/domain"
)

func errInvalid(field, detail string) error {
	return fmt.Errorf("invalid %s: %s", field, detail)
}

// ---- C-2 采样与分母正交（EC-01 / EC-08）----

// ValidateSampleIndices 校验 §2.3-2：sample_rounds >= 1，且同一
// (run, question) 下 sample_index 连续且唯一，取值 0 .. sample_rounds-1。
func ValidateSampleIndices(run domain.CompetitorRun) error {
	if run.SampleRounds < 1 {
		return errInvalid("sample_rounds", "must be >= 1")
	}
	if !domain.ValidSamplePolicyVersion(run.SamplePolicyVersion) {
		return errInvalid("sample_policy_version", run.SamplePolicyVersion)
	}
	seen := map[string]map[int]bool{}
	for _, a := range run.Answers {
		if a.RunID != "" && a.RunID != run.RunID {
			return errInvalid("run_id", "answer belongs to another run")
		}
		if a.SampleIndex < 0 || a.SampleIndex >= run.SampleRounds {
			return errInvalid("sample_index", fmt.Sprintf("%d out of range 0..%d", a.SampleIndex, run.SampleRounds-1))
		}
		if seen[a.QuestionID] == nil {
			seen[a.QuestionID] = map[int]bool{}
		}
		if seen[a.QuestionID][a.SampleIndex] {
			return errInvalid("sample_index", fmt.Sprintf("duplicate %d for question %s", a.SampleIndex, a.QuestionID))
		}
		seen[a.QuestionID][a.SampleIndex] = true
	}
	for q, indices := range seen {
		for i := 0; i < run.SampleRounds; i++ {
			if !indices[i] {
				return errInvalid("sample_index", fmt.Sprintf("missing %d for question %s", i, q))
			}
		}
	}
	return nil
}

// ValidAnswerCount 是 n_valid 的唯一定义：有效答案数。
// 它**不读取 sample_rounds**：多轮采样产生的是多条独立有效答案，各算一次
// observation，不是把一次运行拆成多个分母（C-2 / 契约 1 / DoD 1）。
// capture_channel=unknown 的答案不进任何分母或基线（§2.3-1）。
func ValidAnswerCount(answers []domain.AnswerCompetitorFields) int {
	n := 0
	for _, a := range answers {
		if a.CaptureChannel == domain.CaptureUnknown {
			continue
		}
		if a.RawAnswerText == "" {
			continue
		}
		n++
	}
	return n
}

// NValidUnchangedBySampleRounds 是 C-2 的可执行断言：
// 同一批有效答案，无论声明多少采样轮次，n_valid 恒等。
// 参数 sampleRounds 故意不参与计算，便于 CI 静态扫描确认正交性。
func NValidUnchangedBySampleRounds(sampleRounds int, answers []domain.AnswerCompetitorFields) int {
	_ = sampleRounds
	return ValidAnswerCount(answers)
}

// ---- C-1 采集通道（EC-03）----

// CountsByChannel 只做口径透明分组，不比较通道准确度、不推断平台行为（C-1）。
func CountsByChannel(answers []domain.AnswerCompetitorFields) map[domain.CaptureChannel]int {
	out := map[domain.CaptureChannel]int{}
	for _, a := range answers {
		out[a.CaptureChannel]++
	}
	return out
}

// ChannelCountsTowardsBaseline 明确 unknown 不进分母或基线。
func ChannelCountsTowardsBaseline(c domain.CaptureChannel) bool {
	return c != domain.CaptureUnknown && domain.ValidCaptureChannel(c)
}

// ---- C-3 爬虫访问证据（EC-02）----

// VerifyBotVisit 只核验「已经发生的访问」：不模拟爬虫、不伪造 UA、
// 不去目标站制造访问记录。verifiedCIDRs 必须由用户配置 / 官方已核验 IP 段
// 提供，缺失时绝不臆造 IP 段，一律降级为 ua_only / unverified。
func VerifyBotVisit(operator, uaString, sourceIP string, verifiedCIDRs []string, reverseDNSVerified bool) (domain.BotVerification, bool) {
	if reverseDNSVerified {
		return domain.BotReverseDNSVerified, false
	}
	if sourceIP == "" {
		return domain.BotUnverified, false
	}
	if len(verifiedCIDRs) == 0 {
		if uaString != "" {
			return domain.BotUAOnly, false
		}
		return domain.BotUnverified, false
	}
	if ipInCIDRs(sourceIP, verifiedCIDRs) {
		return domain.BotIPVerified, false
	}
	if operator != domain.CrawlerOperatorUnknown {
		return domain.BotSuspectedSpoof, true
	}
	return domain.BotUnverified, false
}

func ipInCIDRs(ip string, cidrs []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

// CanUseAsPassEvidence 是 §2.3-4 / C-3 的判定优先级：只有 IP 校验或反向
// DNS 校验通过、且非 spoof、且证据源为访问日志的记录才能作为 pass 依据。
// platform_hint 与 observed_evidence 仍为独立字段（Δ-03 / EX-08 不变）。
func CanUseAsPassEvidence(v domain.CrawlerVisitLog) bool {
	if v.SuspectedSpoof {
		return false
	}
	switch v.BotVerification {
	case domain.BotIPVerified, domain.BotReverseDNSVerified:
		return v.EvidenceSource == domain.EvidenceAccessLog
	default:
		return false
	}
}

// ValidatePath 校验 citation_id 必须指向真实 citation（§2.3-6）。
func ValidatePath(p domain.CrawlCitationPath, citationExists func(string) bool) error {
	if p.CitationID == "" || !citationExists(p.CitationID) {
		return errInvalid("citation_id", "citation does not exist; fabricated crawl→citation paths are rejected")
	}
	return nil
}

// ---- C-4 / CE-3 竞品占位 gap 信号（EC-06）----

// GapInput 是 gap 检测器的输入。所有布尔值来自真实 RunAnswer 观测，
// 不推断平台收录或推荐机制。
type GapInput struct {
	ProjectID             string
	QuestionID            string
	RunID                 string
	CompetitorBrandID     string
	BrandPresent          bool
	BrandCited            bool
	BrandRecommended      bool
	CompetitorPresent     bool
	CompetitorCited       bool
	CompetitorRecommended bool
	IntentWeight          float64
	CompetitorFrequency   float64
	TimeDecay             float64
	DetectedAt            string
}

// DetectGaps 生成 CE-3 三类 gap 信号。priority_score 只用于 Action Board
// 内部排序，不进入任何指标聚合、不命名为可见度分数（§2.3-9）。
func DetectGaps(in GapInput) []domain.CompetitorGapSignal {
	out := []domain.CompetitorGapSignal{}
	types := []domain.GapType{}
	if in.CompetitorPresent && !in.BrandPresent {
		types = append(types, domain.GapCompetitorPresentBrandAbsent)
	}
	if in.CompetitorCited && !in.BrandCited {
		types = append(types, domain.GapCompetitorCitedBrandNot)
	}
	if in.CompetitorRecommended && !in.BrandRecommended {
		types = append(types, domain.GapCompetitorRecommendedBrandNot)
	}
	score := in.IntentWeight * in.CompetitorFrequency * in.TimeDecay
	for i, g := range types {
		out = append(out, domain.CompetitorGapSignal{
			SignalID:          fmt.Sprintf("%s-%s-%d", in.RunID, in.CompetitorBrandID, i),
			ProjectID:         in.ProjectID,
			QuestionID:        in.QuestionID,
			RunID:             in.RunID,
			CompetitorBrandID: in.CompetitorBrandID,
			GapType:           g,
			PriorityScore:     score,
			Status:            domain.GapStatusOpen,
			DetectedAt:        in.DetectedAt,
		})
	}
	return out
}

// SortGaps 只按 priority_score 做 Action Board 内部排序。
func SortGaps(gaps []domain.CompetitorGapSignal) []domain.CompetitorGapSignal {
	out := append([]domain.CompetitorGapSignal{}, gaps...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].PriorityScore > out[j].PriorityScore })
	return out
}

// ---- C-4 中立性不变式 ----

// FixArtifactWritableTargets 列出修复件允许写入的集合。
// run / run_answer 不在其中：CI 静态断言用此清单校验零写路径（EC-05）。
var FixArtifactWritableTargets = []string{
	"fix_artifact", "fix_artifact_event",
}

// ReadOnlyEvidenceTargets 列出 GET /v1/evidence 允许读取的集合。
// 该端点不产生任何写操作（§3.2 / EC-07）。
var ReadOnlyEvidenceTargets = []string{
	"run", "run_answer", "citation", "report",
}
