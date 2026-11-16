// 与 domain.NeutralityStatement 保持一致，落地页与定价页共用。
// 与 domain.NeutralityNotice 逐字一致：观测数据独立采集，修复件不改变历史运行。
// 渲染层常量，不落库、不可被租户配置 / 模板变量 / API 参数覆盖。
export const COMPETITOR_NEUTRALITY =
  "本报告的观测数据独立采集；修复件与优化建议不改变任何历史运行记录。两次运行仅为独立 observation，不构成因果结论。";

export const NEUTRALITY =
  "本平台只报告可核验的公开事实，不代表任何 AI 平台的官方判定，不保证收录或被推荐。页面质量为概率性风险提示，阈值由本平台设定，非平台规则。";
