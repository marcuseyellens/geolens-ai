package domain

// 本文件承载第三批最小基线（CR-v1.1 决策 2）：社交信号批次的
// feature flag、口径版本 1.3 与 source_profile_by_model 六口径只读表。
//
// 说明：本机缺失 ES-01～10 原文，按第五批规格引述与启动提示词
// 「没有也不要臆造，以本文引述为准」重建最小基线。它不等同于原
// ES-01～10 定稿；拿到原文后走 CR-v1.2 对齐，不静默改默认值。
// 不修改 D-1～D-6 / EX / EC 任何既有定稿，不占用 PRD 契约编号。

const (
	// SocialFeatureFlag 是第三批最小基线总开关，与 delta/competitor/customer_facing 正交。
	SocialFeatureFlag = "aeo_social_signals_20261117"

	// MethodologyVersionSocial 是第三批最小基线口径版本。第五批保持 1.3 不动。
	MethodologyVersionSocial = "1.3"

	// SocialSchemaVersion 是 social.json 的迁移版本号。
	SocialSchemaVersion = "1.3"
)

// SourceProfileMethod 是六并列归因口径之一（ES-06 引述）。只并列，不合成第七种。
type SourceProfileMethod string

const (
	ProfileMention      SourceProfileMethod = "mention"
	ProfileRecommend    SourceProfileMethod = "recommendation"
	ProfileLink         SourceProfileMethod = "link"
	ProfileCitation     SourceProfileMethod = "citation"
	ProfileCoOccurrence SourceProfileMethod = "co_occurrence"
	ProfileSentiment    SourceProfileMethod = "sentiment"
)

// AllSourceProfileMethods 是固定的六口径清单。禁止追加第七种或合成综合值。
var AllSourceProfileMethods = []SourceProfileMethod{
	ProfileMention, ProfileRecommend, ProfileLink,
	ProfileCitation, ProfileCoOccurrence, ProfileSentiment,
}

// SourceProfileByModel 是分模型来源口径的只读投影（ES-06）。
// 每条只陈述某模型下某口径的观测比例与有效样本，不含聚合分。
type SourceProfileByModel struct {
	ModelCode          string              `json:"model_code"`
	Method             SourceProfileMethod `json:"method"`
	Ratio              float64             `json:"ratio"`
	NValid             int                 `json:"n_valid"`
	SourceRef          string              `json:"source_ref"`
	MethodologyVersion string              `json:"methodology_version"`
}

// IsValidSourceProfileMethod 判断口径是否在固定六项内。
func IsValidSourceProfileMethod(m SourceProfileMethod) bool {
	for _, v := range AllSourceProfileMethods {
		if v == m {
			return true
		}
	}
	return false
}
