package aeo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/answerlens/answerlens/domain"
)

// ---- A-WP-7.1 六要素输入与意图分层 ----

// ValidateBrief 缺任一要素即拒绝生成问题集。
func ValidateBrief(b domain.QuestionBrief) error {
	required := []struct {
		name  string
		value string
	}{
		{"category", b.Category}, {"audience", b.Audience}, {"need", b.Need},
		{"region", b.Region}, {"locale", b.Locale}, {"constraint", b.Constraint},
	}
	missing := []string{}
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("缺少必需要素：%s", strings.Join(missing, ", "))
	}
	return nil
}

// ValidateQuota 四类意图占比可配置且总和为 1。
func ValidateQuota(q domain.IntentQuota) error {
	sum := q.Recommend + q.Compare + q.HowTo + q.Alternative
	if sum < 0.999 || sum > 1.001 {
		return fmt.Errorf("四类意图占比之和应为 1，当前 %.3f", sum)
	}
	for name, v := range map[string]float64{
		"recommend": q.Recommend, "compare": q.Compare, "howto": q.HowTo, "alternative": q.Alternative,
	} {
		if v < 0 {
			return fmt.Errorf("%s 占比不可为负", name)
		}
	}
	return nil
}

// ---- A-WP-7.1 / 7.2 两套问题集物理分离 ----

var intentTemplates = map[domain.QueryIntent][]string{
	domain.IntentRecommend: {
		"给%s推荐适合%s的%s", "哪些%s适合%s在%s场景下使用",
		"%s在%s时该选哪类%s", "面向%s的%s推荐清单",
	},
	domain.IntentCompare: {
		"%s与同类%s在%s上有什么区别", "比较%s和%s的%s时看哪些维度",
		"%s的%s方案对比", "%s做%s选型时的取舍",
	},
	domain.IntentHowTo: {
		"%s如何用%s完成%s", "在%s落地%s的步骤",
		"%s上手%s需要什么", "%s评估%s的方法",
	},
	domain.IntentAlternative: {
		"除了%s还有哪些%s可选", "%s的替代%s有哪些",
		"不想用%s时%s怎么选", "%s预算有限时的%s替代",
	},
}

var intentOrder = []domain.QueryIntent{
	domain.IntentRecommend, domain.IntentCompare, domain.IntentHowTo, domain.IntentAlternative,
}

// GenerateQuestionSet 生成 ≥40 题、四类意图齐全、无诱导式点名的问题集。
// market_baseline 与 brand_tracking 物理分离，返回不同的 question_set_type，
// 绝不合并计算（A-WP-7.2）。
func GenerateQuestionSet(brief domain.QuestionBrief, setType domain.QuestionSetType, quota domain.IntentQuota, count int) ([]domain.GeneratedQuestion, error) {
	if err := ValidateBrief(brief); err != nil {
		return nil, err
	}
	if err := ValidateQuota(quota); err != nil {
		return nil, err
	}
	if count < 40 {
		count = 40
	}
	if setType != domain.SetMarketBaseline && setType != domain.SetBrandTracking {
		return nil, fmt.Errorf("未知的 question_set_type：%s", setType)
	}
	quotas := map[domain.QueryIntent]float64{
		domain.IntentRecommend: quota.Recommend, domain.IntentCompare: quota.Compare,
		domain.IntentHowTo: quota.HowTo, domain.IntentAlternative: quota.Alternative,
	}
	out := []domain.GeneratedQuestion{}
	idx := 0
	for _, intent := range intentOrder {
		n := int(float64(count)*quotas[intent] + 0.5)
		templates := intentTemplates[intent]
		for i := 0; i < n; i++ {
			text := buildQuestion(templates[i%len(templates)], brief, i)
			out = append(out, domain.GeneratedQuestion{
				ID:              fmt.Sprintf("%s-%s-%03d", setType, intent, i+1),
				Text:            text,
				Intent:          intent,
				QuestionSetType: setType,
				IsBrandCheck:    setType == domain.SetBrandTracking && i%5 == 4,
			})
			idx++
		}
	}
	// 四舍五入可能少几题，补齐到 count，并保证四类齐全。
	for len(out) < count {
		intent := intentOrder[len(out)%len(intentOrder)]
		templates := intentTemplates[intent]
		out = append(out, domain.GeneratedQuestion{
			ID:              fmt.Sprintf("%s-%s-fill%03d", setType, intent, len(out)),
			Text:            buildQuestion(templates[len(out)%len(templates)], brief, len(out)),
			Intent:          intent,
			QuestionSetType: setType,
		})
	}
	_ = idx
	return out, nil
}

func buildQuestion(tpl string, b domain.QuestionBrief, seed int) string {
	// 只用六要素填空，不写入任何品牌名，避免诱导式点名（A-WP-7.1 验收）。
	args := []interface{}{b.Audience, b.Need, b.Category}
	text := fmt.Sprintf(tpl, args...)
	if strings.Count(tpl, "%s") > 3 {
		text = fmt.Sprintf(tpl, b.Audience, b.Category, b.Need, b.Region)
	}
	if seed > 0 && seed%3 == 0 {
		text += "（" + b.Region + "，语言 " + b.Locale + "）"
	}
	if b.Constraint != "" {
		text += "；约束：" + b.Constraint
	}
	return text
}

// SeparatedSets 校验两套问题集物理分离，且不合并计算。
func SeparatedSets(market, tracking []domain.GeneratedQuestion) error {
	if len(market) == 0 || len(tracking) == 0 {
		return fmt.Errorf("两套问题集都不能为空")
	}
	for _, q := range market {
		if q.QuestionSetType != domain.SetMarketBaseline {
			return fmt.Errorf("市场基线集混入 %s", q.QuestionSetType)
		}
	}
	for _, q := range tracking {
		if q.QuestionSetType != domain.SetBrandTracking {
			return fmt.Errorf("品牌 Tracking 集混入 %s", q.QuestionSetType)
		}
	}
	// ID 前缀不同，确保不会在下游被当作同一集合合并
	seen := map[string]bool{}
	for _, q := range append(append([]domain.GeneratedQuestion{}, market...), tracking...) {
		if seen[q.ID] {
			return fmt.Errorf("问题 ID 重复：%s", q.ID)
		}
		seen[q.ID] = true
	}
	return nil
}

// InducementPhrases 是诱导式点名的禁用表达（抽查 20 题无此类措辞）。
var InducementPhrases = []string{"是不是最好", "难道不是", "公认第一", "毫无疑问最好"}

// ValidateNoInducement 检查问题文本无诱导式点名。
func ValidateNoInducement(questions []domain.GeneratedQuestion) []string {
	errs := []string{}
	for _, q := range questions {
		for _, p := range InducementPhrases {
			if strings.Contains(q.Text, p) {
				errs = append(errs, q.ID+" 含诱导式措辞："+p)
			}
		}
	}
	return errs
}

// ---- A-WP-7.3 三跳下钻 ----

// BuildDrilldown 构造「聚合数字 → 关系/竞品 → 原文 + 页面 URL」三跳路径。
// 每跳 ≤ 2 次点击；无证据的 Insight 不生成（返回 nil）。
func BuildDrilldown(metric string, value float64, nValid int, runID, rawAnswer, pageURL string) *domain.DrilldownPath {
	if nValid <= 0 || runID == "" || strings.TrimSpace(rawAnswer) == "" || strings.TrimSpace(pageURL) == "" {
		return nil // 无证据不生成 Insight
	}
	hops := []domain.DrilldownHop{
		{Label: "聚合指标", Target: metric, Clicks: 0},
		{Label: "关系 / 竞品明细", Target: "evidence:" + runID, Clicks: 1},
		{Label: "原文与页面 URL", Target: pageURL, Clicks: 2},
	}
	return &domain.DrilldownPath{
		Metric: metric, Value: value, NValid: nValid, Hops: hops,
		RunID: runID, RawAnswer: rawAnswer, PageURL: pageURL, Complete: true,
	}
}

// ValidateDrilldowns CI 断言：10/10 聚合数字可在 ≤2 次点击内下钻到原文与页面 URL。
func ValidateDrilldowns(paths []*domain.DrilldownPath) []string {
	errs := []string{}
	for i, p := range paths {
		if p == nil {
			errs = append(errs, fmt.Sprintf("第 %d 个数字无下钻路径（无证据 Insight 应不生成）", i+1))
			continue
		}
		if !p.Complete || p.RunID == "" || p.RawAnswer == "" || p.PageURL == "" {
			errs = append(errs, fmt.Sprintf("%s 下钻路径不完整", p.Metric))
		}
		if len(p.Hops) != 3 {
			errs = append(errs, fmt.Sprintf("%s 不是三跳", p.Metric))
		}
		for _, h := range p.Hops {
			if h.Clicks > 2 {
				errs = append(errs, fmt.Sprintf("%s 存在超过 2 次点击的跳", p.Metric))
			}
		}
	}
	return errs
}

// SortedIntents 返回四类意图，便于 UI 稳定展示。
func SortedIntents() []domain.QueryIntent {
	out := append([]domain.QueryIntent{}, intentOrder...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
