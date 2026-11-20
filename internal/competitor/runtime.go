// Package competitor 实现《CodeBuddy 工程施工规格｜竞品功能采集批次
// （CF-01~CF-08）》的执行层：独立 feature flag、数据集合、迁移 / 回填 /
// 回滚、判定器与 CI 追加断言。
//
// 与 internal/delta 完全正交：本包不修改任何 Δ 批次判定逻辑与默认值；
// flag=off 时所有新增端点返回 404，行为与批次前完全一致。
package competitor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/answerlens/answerlens/domain"
)

// ---- 独立 feature flag ----

// Flag 是本批次唯一开关。默认 on（交付态全开），
// GEOLENS_FLAG_AEO_COMPETITOR_20261116=off 可秒级关闭，不影响 delta flag。
type Flag struct {
	mu      sync.RWMutex
	enabled bool
}

func NewFlag() *Flag {
	enabled := true
	if v := os.Getenv("GEOLENS_FLAG_AEO_COMPETITOR_20261116"); v == "off" || v == "0" || v == "false" {
		enabled = false
	}
	return &Flag{enabled: enabled}
}

func (f *Flag) Enabled() bool {
	if f == nil {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled
}

func (f *Flag) SetEnabled(v bool) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.enabled = v
	f.mu.Unlock()
}

func (f *Flag) Name() string { return domain.CompetitorFeatureFlag }

// ---- 持久化结构（§2.1 新表等价物）----

// CitationRecord 是真实 citation 的注册表。crawl_citation_path.citation_id
// 必须指向这里已存在的记录，禁止虚构（§2.3-6 / C-3）。
type CitationRecord struct {
	CitationID string          `json:"citation_id"`
	Citation   domain.Citation `json:"citation"`
	RunID      string          `json:"run_id"`
}

type persistShape struct {
	SchemaVersion string                        `json:"schema_version"`
	Runs          []domain.CompetitorRun        `json:"run"`
	Targeting     []domain.PromptTargeting      `json:"prompt_targeting"`
	VisitLogs     []domain.CrawlerVisitLog      `json:"crawler_visit_log"`
	Paths         []domain.CrawlCitationPath    `json:"crawl_citation_path"`
	Gaps          []domain.CompetitorGapSignal  `json:"competitor_gap_signal"`
	AuditChecks   []domain.AuditCheckCompetitor `json:"audit_check"`
	Reports       []domain.CompetitorReport     `json:"report"`
	Citations     []CitationRecord              `json:"citation"`
}

// Runtime 聚合本批次全部集合。所有公开方法加锁并返回深拷贝，避免调用方
// 绕过不可变约束修改内部状态。
type Runtime struct {
	mu          sync.RWMutex
	Flag        *Flag
	path        string
	runs        map[string]*domain.CompetitorRun
	targeting   map[string]*domain.PromptTargeting
	visitLogs   map[string]*domain.CrawlerVisitLog
	paths       []*domain.CrawlCitationPath
	gaps        []*domain.CompetitorGapSignal
	auditChecks map[string]*domain.AuditCheckCompetitor
	reports     map[string]*domain.CompetitorReport
	citations   map[string]*CitationRecord
}

func NewRuntime(path string) *Runtime {
	rt := &Runtime{
		Flag:        NewFlag(),
		path:        path,
		runs:        map[string]*domain.CompetitorRun{},
		targeting:   map[string]*domain.PromptTargeting{},
		visitLogs:   map[string]*domain.CrawlerVisitLog{},
		auditChecks: map[string]*domain.AuditCheckCompetitor{},
		reports:     map[string]*domain.CompetitorReport{},
		citations:   map[string]*CitationRecord{},
	}
	if path != "" {
		_ = rt.load()
	}
	// 演示态种子：ID 一律 demo_ 前缀，明确标注为演示数据，不冒充历史观测，
	// 也不参与 §2.4 历史回填（新表仍为空启动语义）。
	if len(rt.reports) == 0 {
		rt.seed()
	}
	return rt
}

func (r *Runtime) Enabled() bool { return r.Flag.Enabled() }

func (r *Runtime) load() error {
	b, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}
	var s persistShape
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	for i := range s.Runs {
		r.runs[s.Runs[i].RunID] = &s.Runs[i]
	}
	for i := range s.Targeting {
		r.targeting[s.Targeting[i].QuestionSetID] = &s.Targeting[i]
	}
	for i := range s.VisitLogs {
		r.visitLogs[s.VisitLogs[i].VisitID] = &s.VisitLogs[i]
	}
	for i := range s.Paths {
		r.paths = append(r.paths, &s.Paths[i])
	}
	r.gaps = append(r.gaps, ptrSlice(s.Gaps)...)
	for i := range s.AuditChecks {
		r.auditChecks[s.AuditChecks[i].AuditID] = &s.AuditChecks[i]
	}
	for i := range s.Reports {
		r.reports[s.Reports[i].ReportID] = &s.Reports[i]
	}
	for i := range s.Citations {
		r.citations[s.Citations[i].CitationID] = &s.Citations[i]
	}
	return nil
}

func ptrSlice[T any](in []T) []*T {
	out := make([]*T, 0, len(in))
	for i := range in {
		out = append(out, &in[i])
	}
	return out
}

// Persist 落盘；path 为空时是内存态（单测 / 演示），无需写文件。
func (r *Runtime) Persist() error {
	if r.path == "" {
		return nil
	}
	r.mu.RLock()
	s := persistShape{SchemaVersion: domain.CompetitorSchemaVersion}
	for _, v := range r.runs {
		s.Runs = append(s.Runs, *v)
	}
	for _, v := range r.targeting {
		s.Targeting = append(s.Targeting, *v)
	}
	for _, v := range r.visitLogs {
		s.VisitLogs = append(s.VisitLogs, *v)
	}
	for _, v := range r.paths {
		s.Paths = append(s.Paths, *v)
	}
	for _, v := range r.gaps {
		s.Gaps = append(s.Gaps, *v)
	}
	for _, v := range r.auditChecks {
		s.AuditChecks = append(s.AuditChecks, *v)
	}
	for _, v := range r.reports {
		s.Reports = append(s.Reports, *v)
	}
	for _, v := range r.citations {
		s.Citations = append(s.Citations, *v)
	}
	r.mu.RUnlock()
	sort.Slice(s.Runs, func(i, j int) bool { return s.Runs[i].RunID < s.Runs[j].RunID })
	sort.Slice(s.Reports, func(i, j int) bool { return s.Reports[i].ReportID < s.Reports[j].ReportID })
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.path, append(b, '\n'), 0o644)
}

// ---- §2.4 迁移 / 回填 / 回滚（幂等、可重复执行）----

// Migrate 返回本批次新增集合与列清单，只做描述，不改动任何历史记录。
func (r *Runtime) Migrate() map[string]any {
	return map[string]any{
		"schema_version": domain.CompetitorSchemaVersion,
		"idempotent":     true,
		"columns": []string{
			"run.sample_rounds", "run.sample_policy_version", "run.capture_channel_declared",
			"run_answer.sample_index", "run_answer.capture_channel",
			"audit_check.bot_filter_applied", "audit_check.verified_visit_count",
		},
		"tables": []string{
			"crawler_visit_log", "crawl_citation_path", "prompt_targeting", "competitor_gap_signal",
		},
		"historical_report_versions_preserved": []string{"1.0.0", "1.1"},
		"new_report_version":                   domain.MethodologyVersionCompetitor,
	}
}

// Backfill 按 §2.4 回填历史运行：一律固定默认值，禁止反推。
// 返回回填计数；重复执行不会二次改写（幂等）。
func (r *Runtime) Backfill() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	counts := map[string]int{}
	for _, run := range r.runs {
		if run.MethodologyVersion == domain.MethodologyVersionCompetitor {
			continue // 新报告不回填
		}
		changed := false
		if run.SampleRounds != 1 {
			run.SampleRounds = 1
			changed = true
		}
		if run.SamplePolicyVersion != domain.SamplePolicySingleV1 {
			run.SamplePolicyVersion = domain.SamplePolicySingleV1
			changed = true
		}
		if run.CaptureChannelDeclared {
			run.CaptureChannelDeclared = false
			changed = true
		}
		for i := range run.Answers {
			a := &run.Answers[i]
			if a.SampleIndex != 0 {
				a.SampleIndex = 0
				changed = true
			}
			if a.CaptureChannel != domain.CaptureUnknown {
				a.CaptureChannel = domain.CaptureUnknown
				changed = true
			}
		}
		if changed {
			counts["runs"]++
		}
	}
	for _, a := range r.auditChecks {
		if a.BotFilterApplied || a.VerifiedVisitCount != 0 {
			a.BotFilterApplied = false
			a.VerifiedVisitCount = 0
			counts["audit_checks"]++
		}
	}
	// 历史报告版本一律保留生成时版本，不在此处改写。
	return counts
}

// Rollback 按 §8 顺序第 1 步：只置本批次 flag off，不删列 / 表 / 数据。
// delta flag 不受影响（两个 flag 正交）。
func (r *Runtime) Rollback() []string {
	r.Flag.SetEnabled(false)
	return []string{
		"1. " + domain.CompetitorFeatureFlag + " = off",
		"2. 保留全部新增列 / 表（不删数据）",
		"3. " + domain.DeltaFeatureFlag + " 保持不变",
	}
}

// ---- 只读访问（返回副本）----

func (r *Runtime) Run(id string) (domain.CompetitorRun, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.runs[id]
	if !ok {
		return domain.CompetitorRun{}, false
	}
	cp := *v
	cp.Answers = append([]domain.AnswerCompetitorFields{}, v.Answers...)
	return cp, true
}

func (r *Runtime) Answers(runID string) []domain.AnswerCompetitorFields {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.runs[runID]
	if !ok {
		return []domain.AnswerCompetitorFields{}
	}
	out := append([]domain.AnswerCompetitorFields{}, v.Answers...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].QuestionID == out[j].QuestionID {
			return out[i].SampleIndex < out[j].SampleIndex
		}
		return out[i].QuestionID < out[j].QuestionID
	})
	return out
}

func (r *Runtime) Report(id string) (domain.CompetitorReport, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.reports[id]
	if !ok {
		return domain.CompetitorReport{}, false
	}
	return *v, true
}

func (r *Runtime) Reports() []domain.CompetitorReport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.CompetitorReport, 0, len(r.reports))
	for _, v := range r.reports {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ReportID < out[j].ReportID })
	return out
}

func (r *Runtime) Targeting(questionSetID string) (domain.PromptTargeting, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.targeting[questionSetID]
	if !ok {
		return domain.PromptTargeting{}, false
	}
	return *v, true
}

func (r *Runtime) VisitLogs(projectID string) []domain.CrawlerVisitLog {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []domain.CrawlerVisitLog{}
	for _, v := range r.visitLogs {
		if projectID == "" || v.ProjectID == projectID {
			out = append(out, *v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VisitID < out[j].VisitID })
	return out
}

func (r *Runtime) Paths(projectID string) []domain.CrawlCitationPath {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []domain.CrawlCitationPath{}
	for _, v := range r.paths {
		if projectID == "" || v.ProjectID == projectID {
			out = append(out, *v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PathID < out[j].PathID })
	return out
}

func (r *Runtime) Gaps(projectID, status string) []domain.CompetitorGapSignal {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []domain.CompetitorGapSignal{}
	for _, v := range r.gaps {
		if projectID != "" && v.ProjectID != projectID {
			continue
		}
		if status != "" && v.Status != status {
			continue
		}
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PriorityScore > out[j].PriorityScore })
	return out
}

func (r *Runtime) AuditCheck(id string) domain.AuditCheckCompetitor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if v, ok := r.auditChecks[id]; ok {
		return *v
	}
	return domain.AuditCheckCompetitor{AuditID: id}
}

func (r *Runtime) Citation(id string) (CitationRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.citations[id]
	if !ok {
		return CitationRecord{}, false
	}
	return *v, true
}

// ---- 写入（只追加，不 UPDATE 历史 RunAnswer）----

// AddRun 校验 §2.3-2 的 sample_index 连续唯一后写入；返回错误信息。
func (r *Runtime) AddRun(run domain.CompetitorRun) error {
	if err := ValidateSampleIndices(run); err != nil {
		return err
	}
	for i := range run.Answers {
		if !domain.ValidCaptureChannel(run.Answers[i].CaptureChannel) {
			return errInvalid("capture_channel", string(run.Answers[i].CaptureChannel))
		}
		if run.Answers[i].RawAnswerText == "" {
			return errInvalid("raw_answer_text", "empty")
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := run
	cp.Answers = append([]domain.AnswerCompetitorFields{}, run.Answers...)
	r.runs[run.RunID] = &cp
	return nil
}

func (r *Runtime) AddCitation(rec CitationRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.citations[rec.CitationID] = &rec
}

// AddReport 生成新报告；口径版本固定 1.2，历史报告由 seed/load 保留原版本。
func (r *Runtime) AddReport(report domain.CompetitorReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if report.MethodologyVersion == "" {
		report.MethodologyVersion = domain.MethodologyVersionCompetitor
	}
	r.reports[report.ReportID] = &report
}

// SetTargeting 落库定位配置；未设定项写 none / 空串，不留 NULL；
// query_locale 由问题集语言决定，参数透传但不允许由定位配置改写。
func (r *Runtime) SetTargeting(t domain.PromptTargeting) error {
	if !domain.ValidLocationScope(t.LocationScope) {
		return errInvalid("location_scope", string(t.LocationScope))
	}
	if t.LocationScope == domain.ScopeNone {
		t.LocationValue = ""
	}
	if t.ConfigVersion == "" {
		t.ConfigVersion = domain.CompetitorSchemaVersion
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.targeting[t.QuestionSetID] = &t
	return nil
}

func (r *Runtime) AddVisitLog(v domain.CrawlerVisitLog) error {
	if !domain.ValidBotVerification(v.BotVerification) {
		return errInvalid("bot_verification", string(v.BotVerification))
	}
	if !domain.ValidCrawlerOperator(v.CrawlerOperator) {
		return errInvalid("crawler_operator", v.CrawlerOperator)
	}
	if !domain.ValidEvidenceSource(v.EvidenceSource) {
		return errInvalid("evidence_source", string(v.EvidenceSource))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.visitLogs[v.VisitID] = &v
	return nil
}

// AddPath 校验 citation_id 必须指向真实 citation（§2.3-6 / C-3）。
func (r *Runtime) AddPath(p domain.CrawlCitationPath) error {
	r.mu.RLock()
	_, ok := r.citations[p.CitationID]
	r.mu.RUnlock()
	if !ok {
		return errInvalid("citation_id", "citation does not exist; fabricated paths are rejected")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths = append(r.paths, &p)
	return nil
}

func (r *Runtime) AddGap(g domain.CompetitorGapSignal) error {
	if !domain.ValidGapType(g.GapType) {
		return errInvalid("gap_type", string(g.GapType))
	}
	if !domain.ValidGapStatus(g.Status) {
		return errInvalid("status", g.Status)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gaps = append(r.gaps, &g)
	return nil
}

func (r *Runtime) MarkBotFilter(auditID string, verified int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auditChecks[auditID] = &domain.AuditCheckCompetitor{
		AuditID: auditID, BotFilterApplied: true, VerifiedVisitCount: verified,
	}
}

// ---- 演示态种子（ID 前缀 demo_，不冒充历史观测）----

func (r *Runtime) seed() {
	now := time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC).Format(time.RFC3339)

	r.citations["demo_cit_1"] = &CitationRecord{
		CitationID: "demo_cit_1", RunID: "demo_run_1",
		Citation: domain.Citation{
			URL: "https://example.com/guide", Domain: "example.com",
			PagePath: "/guide", Host: "example.com", RegistrableDomain: "example.com",
		},
	}
	r.citations["demo_cit_2"] = &CitationRecord{
		CitationID: "demo_cit_2", RunID: "demo_run_1",
		Citation: domain.Citation{
			URL: "https://docs.example.org/start", Domain: "docs.example.org",
			PagePath: "/start", Host: "docs.example.org", RegistrableDomain: "example.org",
		},
	}

	cit1 := r.citations["demo_cit_1"].Citation
	cit2 := r.citations["demo_cit_2"].Citation

	r.runs["demo_run_1"] = &domain.CompetitorRun{
		RunID: "demo_run_1", QuestionSetID: "demo_qs_1", ProjectID: "demo_project",
		SampleRounds: 2, SamplePolicyVersion: domain.SamplePolicyFixedNV1,
		CaptureChannelDeclared: true, MethodologyVersion: domain.MethodologyVersionCompetitor,
		Answers: []domain.AnswerCompetitorFields{
			{
				AnswerID: "demo_ans_1", RunID: "demo_run_1", QuestionID: "q1",
				SampleIndex: 0, CaptureChannel: domain.CaptureRealUI,
				RawAnswerText: "示例答案：本页被引用为参考资料。",
				Citations:     []domain.Citation{cit1},
				SourceType:    string(domain.SourceObserved), CollectedAt: now,
			},
			{
				AnswerID: "demo_ans_2", RunID: "demo_run_1", QuestionID: "q1",
				SampleIndex: 1, CaptureChannel: domain.CaptureAPISourced,
				RawAnswerText: "示例答案：另一平台给出了不同来源。",
				Citations:     []domain.Citation{cit2},
				SourceType:    string(domain.SourceObserved), CollectedAt: now,
			},
		},
	}

	r.runs["demo_run_legacy"] = &domain.CompetitorRun{
		RunID: "demo_run_legacy", QuestionSetID: "demo_qs_legacy", ProjectID: "demo_project",
		SampleRounds: 1, SamplePolicyVersion: domain.SamplePolicySingleV1,
		MethodologyVersion: domain.MethodologyVersionDelta,
		Answers: []domain.AnswerCompetitorFields{
			{
				AnswerID: "demo_ans_legacy", RunID: "demo_run_legacy", QuestionID: "q0",
				SampleIndex: 0, CaptureChannel: domain.CaptureUnknown,
				RawAnswerText: "历史运行：采集通道未标注。",
				Citations:     []domain.Citation{}, SourceType: string(domain.SourceObserved), CollectedAt: now,
			},
		},
	}

	r.targeting["demo_qs_1"] = &domain.PromptTargeting{
		TargetID: "demo_target_1", QuestionSetID: "demo_qs_1",
		LocationScope: domain.ScopeCountry, LocationValue: "US",
		Persona: "采购负责人", Timezone: "America/Los_Angeles",
		ConfigVersion: domain.CompetitorSchemaVersion, QueryLocale: "en-US",
	}

	r.visitLogs["demo_visit_1"] = &domain.CrawlerVisitLog{
		VisitID: "demo_visit_1", ProjectID: "demo_project", PageURL: "https://example.com/guide",
		CrawlerOperator: domain.CrawlerOperatorGPTBot, SourceIP: "203.0.113.10",
		BotVerification: domain.BotIPVerified, SuspectedSpoof: false,
		EvidenceSource: domain.EvidenceAccessLog, ObservedAt: now, IngestedAt: now,
	}
	r.visitLogs["demo_visit_2"] = &domain.CrawlerVisitLog{
		VisitID: "demo_visit_2", ProjectID: "demo_project", PageURL: "https://example.com/guide",
		CrawlerOperator: domain.CrawlerOperatorGPTBot, SourceIP: "198.51.100.7",
		BotVerification: domain.BotSuspectedSpoof, SuspectedSpoof: true,
		EvidenceSource: domain.EvidenceAccessLog, ObservedAt: now, IngestedAt: now,
	}
	r.auditChecks["demo_audit_1"] = &domain.AuditCheckCompetitor{
		AuditID: "demo_audit_1", BotFilterApplied: true, VerifiedVisitCount: 1,
	}
	r.paths = append(r.paths, &domain.CrawlCitationPath{
		PathID: "demo_path_1", ProjectID: "demo_project", PageURL: "https://example.com/guide",
		CitationID: "demo_cit_1", RunID: "demo_run_1", CrawledAt: now, CitedAt: now,
		VisitID: "demo_visit_1",
	})
	r.gaps = append(r.gaps, &domain.CompetitorGapSignal{
		SignalID: "demo_gap_1", ProjectID: "demo_project", QuestionID: "q2",
		RunID: "demo_run_1", CompetitorBrandID: "demo_competitor",
		GapType: domain.GapCompetitorPresentBrandAbsent, PriorityScore: 0.72,
		Status: domain.GapStatusOpen, DetectedAt: now,
	})

	r.reports["demo_report_100"] = &domain.CompetitorReport{
		ReportID: "demo_report_100", RunID: "demo_run_legacy",
		MethodologyVersion: domain.MethodologyVersion, GeneratedAt: now,
		Counts: domain.Counts{Planned: 4, Usable: 3, Failed: 1}, LegacyCounts: true,
	}
	r.reports["demo_report_110"] = &domain.CompetitorReport{
		ReportID: "demo_report_110", RunID: "demo_run_legacy",
		MethodologyVersion: domain.MethodologyVersionDelta, GeneratedAt: now,
		Counts: domain.Counts{Planned: 4, Usable: 3, Failed: 1, Suspect: 1},
	}
	r.reports["demo_report_120"] = &domain.CompetitorReport{
		ReportID: "demo_report_120", RunID: "demo_run_1",
		MethodologyVersion: domain.MethodologyVersionCompetitor, GeneratedAt: now,
		Counts:         domain.Counts{Planned: 4, Usable: 3, Failed: 1, Suspect: 1},
		CaptureChannel: domain.CaptureRealUI,
	}
}
