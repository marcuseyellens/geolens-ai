package handlers

import (
	"context"
	"fmt"

	"github.com/answerlens/answerlens/application"
	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/mcpserver"
)

// NewMCPRuntime 组装 MCP server：只读 token 注册表 + 真实数据源适配器。
// 返回的本地 agent token 只读，便于在 Claude Code 里做 2 步自查；生产环境应改用
// /api/v1/mcp/tokens 由管理员签发并自行分发。
func NewMCPRuntime(detection *application.DetectionService) (*mcpserver.Server, domain.AgentToken) {
	reg := mcpserver.NewTokenRegistry()
	tok, _ := reg.Issue("local-claude-code", domain.ScopeVisibilityRead, domain.ScopeEvidenceRead)
	return mcpserver.NewServer(reg, &visibilitySource{detection: detection}, &evidenceSource{detection: detection}), tok
}

// visibilitySource 把已落库的检测结果适配成 MCP query_visibility 的逐模型摘要。
// 只输出真实存储的数据；推荐/链接三态在现有结果集里没有采集来源时明确标注，不编造。
type visibilitySource struct {
	detection *application.DetectionService
}

type modelAggregate struct {
	nValid      int
	mentioned   int
	recommended int
	linked      int
}

func (v *visibilitySource) QueryVisibility(q mcpserver.VisibilityQuery) (mcpserver.VisibilityResult, error) {
	if v.detection == nil {
		return mcpserver.VisibilityResult{}, fmt.Errorf("detection service unavailable")
	}
	results, err := v.detection.ListResults(context.Background(), q.BrandID)
	if err != nil {
		return mcpserver.VisibilityResult{}, err
	}
	agg := map[domain.AIModelID]*modelAggregate{}
	for _, r := range results {
		if q.Model != "" && string(r.Model) != q.Model {
			continue
		}
		a, ok := agg[r.Model]
		if !ok {
			a = &modelAggregate{}
			agg[r.Model] = a
		}
		a.nValid++
		if r.BrandMentioned {
			a.mentioned++
		}
	}
	out := mcpserver.VisibilityResult{
		BrandID: q.BrandID, Market: q.Market, Models: []mcpserver.ModelVisibility{},
		Note: "逐模型三态分列，刻意不提供综合 AI 分数；每个比例伴随 n_valid。" +
			"recommend/link 两态在现有检测结果里暂无采集来源，显示为 0 而非推断值。仅代表本次采样。",
	}
	models := make([]domain.AIModelID, 0, len(agg))
	for m := range agg {
		models = append(models, m)
	}
	for _, m := range models {
		a := agg[m]
		rate := func(n int) float64 {
			if a.nValid == 0 {
				return 0
			}
			return float64(n) / float64(a.nValid)
		}
		out.Models = append(out.Models, mcpserver.ModelVisibility{
			Model: string(m), Region: q.Region, NValid: a.nValid,
			Mentioned: a.mentioned, Recommended: a.recommended, Linked: a.linked,
			MentionRate: rate(a.mentioned), RecommendRate: rate(a.recommended),
			LinkRate:   rate(a.linked),
			SampleNote: "recommend/link 暂无采集来源，未按推断填充",
			Provenance: domain.ObservedProvenance(a.nValid),
		})
	}
	if len(out.Models) == 0 {
		return mcpserver.VisibilityResult{}, fmt.Errorf("品牌 %s 暂无可见性数据", q.BrandID)
	}
	return out, nil
}

// evidenceSource 按 run_id 从落库结果里取回原始答案文本。
type evidenceSource struct {
	detection *application.DetectionService
}

func (e *evidenceSource) GetEvidence(runID string) (mcpserver.EvidenceResult, error) {
	if e.detection == nil {
		return mcpserver.EvidenceResult{}, fmt.Errorf("detection service unavailable")
	}
	brands, err := e.detection.ListBrands(context.Background())
	if err != nil {
		return mcpserver.EvidenceResult{}, err
	}
	for _, b := range brands {
		results, err := e.detection.ListResults(context.Background(), b.ID)
		if err != nil {
			continue
		}
		for _, r := range results {
			if r.ID != runID {
				continue
			}
			return mcpserver.EvidenceResult{
				RunID: r.ID, RawAnswerText: r.ResponseText,
				CitationURLs: []string{}, ObservedOn: r.DetectedAt.Format("2006-01-02"),
				Provenance: domain.ObservedProvenance(1),
			}, nil
		}
	}
	return mcpserver.EvidenceResult{}, fmt.Errorf("run %s 不存在", runID)
}
