package delta

import (
	"sync"

	"github.com/answerlens/answerlens/domain"
)

// Runtime 聚合 Δ 批次全部运行时状态，作为 HTTP 层与 CI 断言的统一入口。
// 本项目为本地演示 / 自托管部署，数据以 JSON 文件持久化（等价规格 §2 的 DDL）。
type Runtime struct {
	Flag         *Flag
	Config       *Config
	QuestionSets *QuestionSetStore
	UserMetrics  *UserProvidedStore
	Retention    *RetentionStore
	Manifests    *ManifestStore
	Webhooks     *WebhookDedup

	mu      sync.Mutex
	runs    map[string]domain.RunAnswer
	reports map[string]ReportRecord
	audits  map[string][]AuditCheckView
	crawler map[string]CrawlerVerdict
}

// AuditCheckView 对应 §3.1 GET /v1/audits/{id}/checks 的一行。
type AuditCheckView struct {
	CheckID              string                      `json:"check_id"`
	Label                string                      `json:"label"`
	Status               domain.CheckStatus          `json:"status"`
	PlatformHint         string                      `json:"platform_hint"`
	ObservedEvidence     string                      `json:"observed_evidence"`
	EvidenceSource       domain.EvidenceSource       `json:"evidence_source"`
	Verdict              string                      `json:"verdict"`
	CrawlerVisitEvidence domain.CrawlerVisitEvidence `json:"crawler_visit_evidence"`
	FixAdvice            string                      `json:"fix_advice,omitempty"`
	RiskNotice           string                      `json:"risk_notice"`
	SourceURL            string                      `json:"source_url"`
}

// NewRuntime 建立运行时；dataPath 为空时用内存态（单测 / 临时部署）。
func NewRuntime(dataPath string) *Runtime {
	cfg := NewConfig(dataPath)
	rt := &Runtime{
		Flag:         NewFlag(),
		Config:       cfg,
		QuestionSets: NewQuestionSetStore(),
		UserMetrics:  NewUserProvidedStore(),
		Retention:    NewRetentionStore(),
		Manifests:    NewManifestStore(cfg),
		Webhooks:     NewWebhookDedup(),
		runs:         map[string]domain.RunAnswer{},
		reports:      map[string]ReportRecord{},
		audits:       map[string][]AuditCheckView{},
		crawler:      map[string]CrawlerVerdict{},
	}
	rt.seed()
	return rt
}

// seed 写入可下钻的演示数据，保证 §3.1 端点在本地零配置下有真实响应。
// 所有数据标注「本地模拟产物」，不冒充第三方实测。
func (rt *Runtime) seed() {
	// 一条跨语言运行：query_locale=de-DE 但平台头为 pl-PL → mismatch=true。
	fields := InferLocale(LocaleSignals{
		QueryLocale: "de-DE", PlatformHeader: "pl-PL", RunID: "run_demo_1",
	}, rt.Config.Get(domain.ThresholdInferredMinConfidence))
	run := domain.RunAnswer{
		RunID: "run_demo_1", QuestionID: "q1", Platform: domain.ModelChatGPT,
		Locale: "de-DE", Valid: true,
		Citations: []domain.Citation{
			{URL: "https://blog.example.com/a"}, {URL: "https://example.com/b"},
		},
	}
	fields.ApplyTo(&run)
	run.Citations = NormalizeCitations(run.Citations)
	rt.runs[run.RunID] = run

	// 一条 unknown 运行：不进任何分母。
	unknownFields := InferLocale(LocaleSignals{QueryLocale: "pl-PL", RunID: "run_demo_2"},
		rt.Config.Get(domain.ThresholdInferredMinConfidence))
	run2 := domain.RunAnswer{RunID: "run_demo_2", Locale: "pl-PL", Valid: true}
	unknownFields.ApplyTo(&run2)
	rt.runs[run2.RunID] = run2

	// 报告：1.1 口径四数 + 并列观察 + 独立参考区。
	rt.UserMetrics.Add("dr", 72, 1)
	rec := BuildReportRecord("report_demo_1", []domain.RunAnswer{run, run2}, false, rt.UserMetrics.ReferenceOnly())
	rec.Counts = domain.Counts{Planned: 4, Usable: 2, Failed: 1, Suspect: 1}
	rt.reports[rec.ReportID] = rec

	// 历史报告：1.0 口径三数 + 旧口径标注。
	hist := BuildReportRecord("report_legacy_1", []domain.RunAnswer{run}, true, nil)
	rt.reports[hist.ReportID] = hist

	// 爬虫访问证据：平台报错但无日志 → unknown，两行独立。
	verdict := JudgeCrawlerVisit(CrawlerSignals{
		PlatformHint: "Couldn't fetch", PlatformHintIsError: true,
	})
	rt.crawler["audit_demo_1"] = verdict
	rt.audits["audit_demo_1"] = []AuditCheckView{
		{
			CheckID: "crawler_visit_9", Label: "AI crawler 访问证据",
			Status: domain.CheckUnknown, PlatformHint: verdict.PlatformHint,
			ObservedEvidence: verdict.ObservedEvidence, EvidenceSource: verdict.EvidenceSource,
			Verdict: string(verdict.Verdict), CrawlerVisitEvidence: verdict.Verdict,
			FixAdvice:  FixAdviceForCrawler(verdict),
			RiskNotice: "风险提示：概率性判断，平台提示仅为线索，非任何 AI 平台收录规则。",
			SourceURL:  "/docs/increment-template.md",
		},
	}

	// 默认工具清单（含两个自检工具）。
	tools := []ToolDescriptor{{Name: "query_visibility"}, {Name: "evidence_by_run"}}
	for _, n := range SelfCheckToolNames {
		tools = append(tools, ToolDescriptor{Name: n})
	}
	_, _, _ = rt.Manifests.Publish("1.1", tools, false, rt.Config)
}

func (rt *Runtime) Enabled() bool { return rt.Flag.Enabled() }

func (rt *Runtime) Run(id string) (domain.RunAnswer, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	r, ok := rt.runs[id]
	return r, ok
}

func (rt *Runtime) Report(id string) (ReportRecord, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	r, ok := rt.reports[id]
	return r, ok
}

func (rt *Runtime) AuditChecks(id string) ([]AuditCheckView, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	v, ok := rt.audits[id]
	return v, ok
}

// SetCrawlerEvidence 处理用户上传访问日志片段（§3.1 POST）。
// user_provided 一律标注「用户自述，未经核验」，不判 pass。
func (rt *Runtime) SetCrawlerEvidence(auditID string, src domain.EvidenceSource, content, note string) (string, string) {
	signals := CrawlerSignals{PlatformHint: rt.crawler[auditID].PlatformHint}
	switch src {
	case domain.EvidenceAccessLog:
		signals.AccessLogHitAIUA = content != ""
	case domain.EvidenceUserProvided:
		signals.UserProvidedLog = content
	default:
		signals.UserProvidedLog = content
	}
	verdict := JudgeCrawlerVisit(signals)
	_ = note
	rt.mu.Lock()
	rt.crawler[auditID] = verdict
	checks := rt.audits[auditID]
	if len(checks) > 0 {
		checks[0].ObservedEvidence = verdict.ObservedEvidence
		checks[0].EvidenceSource = verdict.EvidenceSource
		checks[0].Verdict = string(verdict.Verdict)
		checks[0].CrawlerVisitEvidence = verdict.Verdict
		rt.audits[auditID] = checks
	}
	rt.mu.Unlock()
	return "crawler_visit_9", string(verdict.Verdict)
}

// Persist 把配置与数据落盘。
func (rt *Runtime) Persist() error {
	return rt.Config.Persist(func(s *persistShape) {
		s.UserProvided = rt.UserMetrics.ReferenceOnly()
		s.FixEvents = rt.Retention.Events()
		var manifests []domain.ToolManifest
		for _, v := range rt.Manifests.Versions() {
			manifests = append(manifests, domain.ToolManifest{ManifestVersion: v})
		}
		s.Manifests = manifests
		s.QuestionSets = rt.QuestionSets.List("")
		for _, rec := range rt.reports {
			s.Reports = append(s.Reports, rec)
		}
	})
}
