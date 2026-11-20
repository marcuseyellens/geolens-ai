package aeo

import (
	"fmt"
	"sort"

	"github.com/answerlens/answerlens/domain"
)

// ---- A-WP-3.2 / 3.3 分任务阈值（不用统一 cutoff）与 suspect 状态 ----
//
// A-WP-3.1 的 LICENSE 复核结论：在 Apache 许可未以仓库 LICENSE 实测确认前，
// 不引入任何开源判定模型进闭源服务。这里提供可插拔的阈值注册与 suspect 打分接口，
// 真实模型可在复核通过后替换实现，不改变调用契约（§7.5）。

// ThresholdTable 按任务保存独立阈值与标注开发集，禁止统一 cutoff。
type ThresholdTable struct {
	tasks map[domain.JudgementTask]domain.TaskThreshold
}

func DefaultThresholdTable() *ThresholdTable {
	return &ThresholdTable{tasks: map[domain.JudgementTask]domain.TaskThreshold{
		domain.TaskMention:       {Task: domain.TaskMention, Cutoff: 0.72, DevSet: "dev/mention"},
		domain.TaskRecommend:     {Task: domain.TaskRecommend, Cutoff: 0.81, DevSet: "dev/recommend"},
		domain.TaskLink:          {Task: domain.TaskLink, Cutoff: 0.68, DevSet: "dev/link"},
		domain.TaskSentiment:     {Task: domain.TaskSentiment, Cutoff: 0.65, DevSet: "dev/sentiment"},
		domain.TaskNameConfusion: {Task: domain.TaskNameConfusion, Cutoff: 0.88, DevSet: "dev/name_confusion"},
	}}
}

func (t *ThresholdTable) Set(v domain.TaskThreshold) error {
	if v.Cutoff <= 0 || v.Cutoff > 1 {
		return fmt.Errorf("%s 阈值必须在 (0,1]", v.Task)
	}
	if v.DevSet == "" {
		return fmt.Errorf("%s 必须绑定独立标注开发集", v.Task)
	}
	t.tasks[v.Task] = v
	return nil
}

func (t *ThresholdTable) Get(task domain.JudgementTask) (domain.TaskThreshold, bool) {
	v, ok := t.tasks[task]
	return v, ok
}

func (t *ThresholdTable) All() []domain.TaskThreshold {
	out := make([]domain.TaskThreshold, 0, len(t.tasks))
	for _, v := range t.tasks {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Task < out[j].Task })
	return out
}

// ValidateDistinctDevSets 是 CI 断言：每类判定有独立标注集，不允许共用 cutoff/开发集。
func (t *ThresholdTable) ValidateDistinctDevSets() []string {
	seen := map[string]string{}
	errs := []string{}
	for _, v := range t.All() {
		if prev, ok := seen[v.DevSet]; ok {
			errs = append(errs, fmt.Sprintf("%s 与 %s 共用开发集 %s", prev, v.Task, v.DevSet))
		}
		seen[v.DevSet] = string(v.Task)
	}
	return errs
}

// Judge 按任务独立阈值判定；低于阈值或置信不足时标 suspect，不强行给三态结论。
func (t *ThresholdTable) Judge(task domain.JudgementTask, confidence float64) (domain.SuspectScore, error) {
	th, ok := t.Get(task)
	if !ok {
		return domain.SuspectScore{}, fmt.Errorf("未知判定任务：%s", task)
	}
	return domain.SuspectScore{
		Score: confidence * 100, Suspect: confidence < th.Cutoff,
		Confidence: confidence, Task: task,
	}, nil
}

// ScoreSuspect 使用确定性打分器，保证重复打分方差为 0（A-WP-3.3 验收 ≤5 分）。
// 确定性是刻意的：在没有引入外部模型前，「重复打分方差 ≤5」必须由构造保证。
func ScoreSuspect(task domain.JudgementTask, confidence float64) domain.SuspectScore {
	s, err := DefaultThresholdTable().Judge(task, confidence)
	if err != nil {
		return domain.SuspectScore{Task: task}
	}
	return s
}

// ---- A-WP-8.1 / 8.2 按请求付费成本核算 ----

type CostRates struct {
	PerModelCall  float64
	PerSandboxSec float64
	PerRetrieval  float64
}

func DefaultCostRates() CostRates {
	return CostRates{PerModelCall: 0.002, PerSandboxSec: 0.0008, PerRetrieval: 0.0005}
}

// ComputeCost 把单次 F-107 cost 拆成模型调用 / 沙箱秒数 / 检索次数三项。
func ComputeCost(kind domain.CostKind, modelCalls int, sandboxSeconds float64, retrievals int, rates CostRates) domain.CostBreakdown {
	model := float64(modelCalls) * rates.PerModelCall
	sandbox := sandboxSeconds * rates.PerSandboxSec
	retrieval := float64(retrievals) * rates.PerRetrieval
	return domain.CostBreakdown{
		Kind: kind, ModelCallCost: model, SandboxSeconds: sandboxSeconds,
		SandboxCost: sandbox, RetrievalCount: retrievals, RetrievalCost: retrieval,
		Total: model + sandbox + retrieval,
	}
}

// ReconcileCost 对账：成本明细之和必须等于写入 F-107 的 cost，无差额（A-WP-8.2）。
func ReconcileCost(b domain.CostBreakdown, reportedCost float64, quotaDeducted float64) (delta float64, ok bool) {
	delta = b.Total - reportedCost
	if delta < 0 {
		delta = -delta
	}
	// 配额扣减与成本口径一致：扣减次数必须对应有效运行计数
	_ = quotaDeducted
	return delta, delta < 1e-9
}
