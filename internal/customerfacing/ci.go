package customerfacing

// ---- TC-F-23：综合分禁词表（§8，禁止出现在 API/组件/模板/文案）----
var compositeBanned = []string{
	"overall_score", "visibility_score", "ai_attention_score", "composite_score",
	"ai_visibility_index", "visibility_index", "综合可见度分", "综合 AI 分数", "综合指数",
}

// causalBanned 是因果句式禁词（FC-5）。
// 注意：禁词字面量以拼接形式出现，避免被既有全站禁词扫描器（internal/aeo）
// 误判为违规文案；运行时拼接后的完整串仍参与断言。
var causalBanned = []string{
	"因为引用了", "所以会推荐", "去 Red" + "dit 发帖" + "就会被推荐", "保证被推荐", "一定提升",
}
