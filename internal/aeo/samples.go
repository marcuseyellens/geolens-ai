package aeo

import (
	"strings"

	"github.com/geolens/geolens/domain"
)

// ---- A-WP-2.4 固定样本集 ----
//
// 移到生产代码是为了让 CI 测试与运行时验收清单共用同一份真相，
// 避免 API 端把结果硬编码成「已通过」。

// AdmissionSample 是一个标注了预期检出类型的公开页面样本。
type AdmissionSample struct {
	Page   PageSnapshot
	Expect domain.AdmissionDetector
}

// GoodPageHTML 是一个 8 类检测器都应判 pass 的健康页面样本。
const GoodPageHTML = `<html lang="zh-CN"><head><title>示例产品页</title></head><body>` +
	`<h1>示例产品</h1><p>面向团队的 AEO 平台，提供站点检查与报告。</p>` +
	`<p>价格 $39，规格与参数见下表。常见问题：如何开始？</p>` +
	`<p>` + FillerText + `</p></body></html>`

// FillerText 是避免关键词堆砌误判的中性长文本（每个词只出现一次）。
const FillerText = "团队 评估 方案 时 关注 能力 边界 接入 成本 可验证 结果 记录 每次 提问 原始 回答 " +
	"模型 版本 日期 搜索 开关 状态 复扫 同一批 问题 只比较 两次 都有效 答案 面向 增长 与品牌 " +
	"公开 页面 技术 条件 抓取 理解 引用 结构化 数据 对比 选型 事实 清单 报告 证据 下钻 " +
	"修复 建议 优先级 验收 口径 分栏 并列 不合成 综合 数值 采样 窗口 采集 批次 失败 排除 " +
	"独立 计数 三态 提及 推荐 链接 位置 情感 混淆 单独 标注 来源 归一 域名 页面 路径 运行 " +
	"标识 成本 归集 配额 限流 熔断 恢复 监控 告警 折叠 静默 导出 订阅 项目 视图 快照 时间 " +
	"旅行 分支 合入 图谱 本体 市场 品牌 关系 竞品 引用 来源 结构 切片 样本量 采集窗口 方法论"

// NoSignalText 不含任何「有用信息」信号词（价格/规格/参数/FAQ/对比/$）。
const NoSignalText = "团队 评估 方案 时 关注 能力 边界 接入 计算量 可验证 结果 记录 每次 提问 原始 回答 " +
	"模型 版本 日期 搜索 开关 状态 复扫 同一批 问题 只比较 两次 都有效 答案 面向 增长 与品牌 " +
	"公开 页面 技术 条件 抓取 理解 引用 结构化 数据 选型 事实 清单 报告 证据 下钻 " +
	"修复 建议 优先级 验收 口径 分栏 并列 不合成 综合 数值 采样 窗口 采集 批次 失败 排除 " +
	"独立 计数 三态 提及 推荐 链接 位置 情感 混淆 单独 标注 来源 归一 域名 页面 路径 运行 " +
	"标识 成本 归集 配额 限流 熔断 恢复 监控 告警 折叠 静默 导出 订阅 项目 视图 快照 时间 " +
	"旅行 分支 合入 图谱 本体 市场 品牌 关系 竞品 引用 来源 结构 切片 样本量 采集窗口 方法论"

// AdmissionSamples 返回 8 类 × 3 = 24 个固定样本。
// 每个样本只注入目标检测器的特征，其余维度保持健康，从而能统计误报。
func AdmissionSamples() []AdmissionSample {
	good := PageSnapshot{URL: "https://example.com", Status: 200, Rendered: true, Body: GoodPageHTML}
	withBody := func(body string) PageSnapshot {
		p := good
		p.Body = body
		return p
	}
	withStatus := func(status int) PageSnapshot {
		p := good
		p.Status = status
		return p
	}
	wrap := func(inner string) string {
		return `<html><head><title>示例页面</title></head><body><h1>示例</h1><p>` +
			inner + `</p><p>价格 $39，规格与参数见下。常见问题：如何开始？</p>` +
			`<p>` + FillerText + `</p></body></html>`
	}
	out := []AdmissionSample{}
	add := func(d domain.AdmissionDetector, pages ...PageSnapshot) {
		for _, p := range pages {
			out = append(out, AdmissionSample{Page: p, Expect: d})
		}
	}
	add(domain.DetectErrorPage, withStatus(500), withStatus(404),
		withBody(`<html><head><title>404 Not Found</title></head><body><h1>示例</h1><p>价格 $39 常见问题</p><p>`+FillerText+`</p></body></html>`))
	add(domain.DetectAntiBotWall,
		withBody(wrap(`<div class="cf-challenge">Checking your browser</div>`)),
		withBody(wrap(`<div>Datadome protection enabled</div>`)),
		withBody(wrap(`<div>Access Denied</div>`)))
	add(domain.DetectCaptcha,
		withBody(wrap(`<div class="g-recaptcha"></div>`)),
		withBody(wrap(`<div class="h-captcha" data-sitekey="x"></div>`)),
		withBody(wrap(`<div id="captcha-container"></div>`)))
	add(domain.DetectThinContent,
		withBody(`<html><body><h1>标题</h1><p>价格 $39 常见问题</p></body></html>`),
		withBody(`<html><body><h1>标题</h1><p>规格与参数</p></body></html>`),
		withBody(`<html><body><h1>标题</h1><p>价格 规格</p></body></html>`))
	add(domain.DetectSEOFiller,
		withBody(wrap(strings.Repeat("最佳 ", 200))),
		withBody(wrap(strings.Repeat("推荐 ", 200))),
		withBody(wrap(strings.Repeat("便宜 ", 200))))
	add(domain.DetectNavigationOnly,
		withBody(`<html><body><nav><a href="/a">A</a><a href="/b">B</a></nav><span>价格 $39 常见问题 `+FillerText+`</span></body></html>`),
		withBody(`<html><body><ul><li><a href="/a">A</a></li></ul><span>规格 参数 价格 `+FillerText+`</span></body></html>`),
		withBody(`<html><body><div class="links"><a href="/a">A</a></div><span>价格 常见问题 `+FillerText+`</span></body></html>`))
	add(domain.DetectUsefulInfo,
		withBody(`<html><body><h1>标题</h1><p>`+NoSignalText+`</p></body></html>`),
		withBody(`<html><body><h1>标题</h1><p>`+NoSignalText+` 更多说明文字</p></body></html>`),
		withBody(`<html><body><main><h1>标题</h1><p>`+NoSignalText+` 补充段落</p></main></body></html>`))
	add(domain.DetectPIIExposure,
		withBody(wrap("联系电话 13800138000")),
		withBody(wrap("证件号 110101199001011234")),
		withBody(wrap("手机 13912345678")))
	return out
}

// AdmissionRegression 返回 (检出类别数 / 8, 误报数 / 24, SPA 是否误判 thin_content)。
func AdmissionRegression() (detected int, falsePositives int, spaFalsePositive bool) {
	samples := AdmissionSamples()
	hit := map[domain.AdmissionDetector]bool{}
	for _, s := range samples {
		for _, c := range InspectAdmission(s.Page, DefaultAdmissionConfig()) {
			if c.Status == domain.AdmissionFail || c.Status == domain.AdmissionPartial {
				if c.Detector == s.Expect {
					hit[s.Expect] = true
				} else {
					falsePositives++
				}
			}
		}
	}
	detected = len(hit)
	spa := PageSnapshot{
		URL: "https://example.com", Status: 200, Rendered: false,
		Body: `<html><head></head><body><div id="root"></div><script type="module" src="/app.js"></script></body></html>`,
	}
	for _, c := range InspectAdmission(spa, DefaultAdmissionConfig()) {
		if c.Detector == domain.DetectThinContent && c.Status == domain.AdmissionFail {
			spaFalsePositive = true
		}
	}
	return
}
