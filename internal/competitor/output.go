package competitor

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/geolens/geolens/domain"
)

// 本文件是 §3.2 / EC-08 要求覆盖的五个输出面：
//   报告模板 / 导出序列化器 / PDF 渲染模板 / 告警模板 / Action Board 返回值。
// 五处结构体均不含综合 AI 可见性分数；priority_score 只存在于
// competitor_gap_signal 与 competitor-gaps 端点，Action Board 只消费排序结果。
// CI 静态断言扫描本文件与前端对应文件确认禁令生效。

const LegacyNotice = "旧口径：本报告按生成时版本渲染，未用新口径重算。"

// ---- 1. 报告模板 ----

type ReportTemplate struct {
	ReportID             string        `json:"report_id"`
	MethodologyVersion   string        `json:"methodology_version"`
	Counts               domain.Counts `json:"counts"`
	CountNotice          string        `json:"count_notice,omitempty"`
	NeutralityNotice     string        `json:"neutrality_notice,omitempty"`
	CaptureChannel       string        `json:"capture_channel,omitempty"`
	CaptureChannelNotice string        `json:"capture_channel_notice,omitempty"`
	SourceType           string        `json:"source_type"`
}

// BuildReportTemplate 按 1.0 / 1.1 / 1.2 三口径渲染（§2.4）。
// 历史口径缺字段时只标「旧口径」，绝不补默认值伪装成新口径。
func BuildReportTemplate(rec domain.CompetitorReport) ReportTemplate {
	t := ReportTemplate{
		ReportID:           rec.ReportID,
		MethodologyVersion: rec.MethodologyVersion,
		Counts:             rec.Counts,
		SourceType:         string(domain.SourceObserved),
	}
	switch rec.MethodologyVersion {
	case domain.MethodologyVersionCompetitor: // 1.2
		t.NeutralityNotice = domain.NeutralityNotice
		if rec.CaptureChannel != "" {
			t.CaptureChannel = string(rec.CaptureChannel)
			if rec.CaptureChannel == domain.CaptureUnknown {
				t.CaptureChannelNotice = domain.ChannelUnlabelledNotice
			}
		} else {
			t.CaptureChannelNotice = domain.ChannelUnlabelledNotice
		}
	case domain.MethodologyVersionDelta: // 1.1
		t.CountNotice = LegacyNotice + " 1.1：四数，无采集通道与中立性声明。"
	default: // 1.0.0
		t.CountNotice = LegacyNotice + " 1.0：三数，缺 suspect 按 0 展示。"
	}
	if rec.LegacyCounts {
		t.CountNotice = LegacyNotice + " 缺 suspect 按 0 展示，未用新算法重算。"
	}
	return t
}

// ---- 2. 导出序列化器 ----

// ExportRecord 是 CSV / JSON 导出的单行。含 raw_answer_text 与完整 citations。
type ExportRecord struct {
	RunID              string                `json:"run_id"`
	MethodologyVersion string                `json:"methodology_version"`
	SampleIndex        int                   `json:"sample_index"`
	CaptureChannel     domain.CaptureChannel `json:"capture_channel"`
	RawAnswerText      string                `json:"raw_answer_text"`
	Citations          []domain.Citation     `json:"citations"`
	SourceType         string                `json:"source_type"`
	CollectedAt        string                `json:"collected_at"`
}

// BuildExportRecords 只读组装导出数据，不产生任何写操作。
func BuildExportRecords(rt *Runtime, runID string) ([]ExportRecord, string, error) {
	run, ok := rt.Run(runID)
	if !ok {
		return nil, "", fmt.Errorf("run not found: %s", runID)
	}
	answers := rt.Answers(runID)
	out := make([]ExportRecord, 0, len(answers))
	for _, a := range answers {
		if a.RawAnswerText == "" {
			return nil, "", fmt.Errorf("empty raw_answer_text in %s", a.AnswerID)
		}
		out = append(out, ExportRecord{
			RunID: runID, MethodologyVersion: run.MethodologyVersion,
			SampleIndex: a.SampleIndex, CaptureChannel: a.CaptureChannel,
			RawAnswerText: a.RawAnswerText, Citations: a.Citations,
			SourceType: a.SourceType, CollectedAt: a.CollectedAt,
		})
	}
	return out, run.MethodologyVersion, nil
}

func ExportJSON(records []ExportRecord) ([]byte, error) {
	return json.MarshalIndent(records, "", "  ")
}

func ExportCSV(records []ExportRecord) ([]byte, error) {
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	_ = w.Write([]string{
		"run_id", "methodology_version", "sample_index", "capture_channel",
		"raw_answer_text", "citation_urls", "citation_count", "source_type", "collected_at",
	})
	for _, r := range records {
		urls := make([]string, 0, len(r.Citations))
		for _, c := range r.Citations {
			urls = append(urls, c.URL)
		}
		_ = w.Write([]string{
			r.RunID, r.MethodologyVersion, fmt.Sprint(r.SampleIndex),
			string(r.CaptureChannel), r.RawAnswerText,
			strings.Join(urls, " | "), fmt.Sprint(len(r.Citations)),
			r.SourceType, r.CollectedAt,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return []byte(sb.String()), nil
}

// ---- 3. PDF 渲染模板 ----

type PDFTemplate struct {
	Title              string        `json:"title"`
	ReportID           string        `json:"report_id"`
	MethodologyVersion string        `json:"methodology_version"`
	Counts             domain.Counts `json:"counts"`
	NeutralityNotice   string        `json:"neutrality_notice,omitempty"`
	CountNotice        string        `json:"count_notice,omitempty"`
}

func BuildPDFTemplate(rec domain.CompetitorReport) PDFTemplate {
	t := BuildReportTemplate(rec)
	return PDFTemplate{
		Title: "AnswerLens 诊断报告", ReportID: t.ReportID,
		MethodologyVersion: t.MethodologyVersion, Counts: t.Counts,
		NeutralityNotice: t.NeutralityNotice, CountNotice: t.CountNotice,
	}
}

// ---- 4. 告警模板 ----

// AlertTemplate 只描述观测数值与采集健康，不含品牌可见度分数。
// competitor_gap_signal 不触发 F-801 品牌表现类告警（§3.4）。
type AlertTemplate struct {
	AlertID    string `json:"alert_id"`
	Category   string `json:"category"`
	Message    string `json:"message"`
	ObservedAt string `json:"observed_at"`
	SourceType string `json:"source_type"`
}

func BuildAlertTemplate(id, category, message, observedAt string) AlertTemplate {
	return AlertTemplate{
		AlertID: id, Category: category, Message: message,
		ObservedAt: observedAt, SourceType: string(domain.SourceObserved),
	}
}

// ---- 5. Action Board 返回值 ----

// ActionBoardItem 是「今天值得做的 3 件事」的信号条目。
// 不含 priority_score：该分数仅用于 Action Board 内部排序，不对外展示。
type ActionBoardItem struct {
	SignalID          string         `json:"signal_id"`
	QuestionID        string         `json:"question_id"`
	GapType           domain.GapType `json:"gap_type"`
	CompetitorBrandID string         `json:"competitor_brand_id"`
	RunID             string         `json:"run_id"`
	DetectedAt        string         `json:"detected_at"`
}

// BuildActionBoard 把 gap 信号按 priority_score 内部排序后映射为返回值。
// 每条信号可沿 run_id 两跳下钻（复用 EC-07 的 evidence 端点）。
func BuildActionBoard(rt *Runtime, projectID string, limit int) []ActionBoardItem {
	sorted := SortGaps(rt.Gaps(projectID, domain.GapStatusOpen))
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}
	out := make([]ActionBoardItem, 0, len(sorted))
	for _, g := range sorted {
		out = append(out, ActionBoardItem{
			SignalID: g.SignalID, QuestionID: g.QuestionID, GapType: g.GapType,
			CompetitorBrandID: g.CompetitorBrandID, RunID: g.RunID, DetectedAt: g.DetectedAt,
		})
	}
	return out
}

// ---- EC-07 只读证据端点 ----

// BuildEvidence 返回 run 的全部采样证据。只读：不修改 run / run_answer。
func BuildEvidence(rt *Runtime, runID string) ([]domain.EvidenceRecord, string, error) {
	records, version, err := BuildExportRecords(rt, runID)
	if err != nil {
		return nil, "", err
	}
	out := make([]domain.EvidenceRecord, 0, len(records))
	for _, r := range records {
		out = append(out, domain.EvidenceRecord{
			RunID: r.RunID, MethodologyVersion: r.MethodologyVersion,
			RawAnswerText: r.RawAnswerText, SampleIndex: r.SampleIndex,
			CaptureChannel: r.CaptureChannel, Citations: r.Citations,
			SourceType: r.SourceType, CollectedAt: r.CollectedAt,
		})
	}
	return out, version, nil
}
