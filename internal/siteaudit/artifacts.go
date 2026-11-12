package siteaudit

import (
	"fmt"
	"strings"

	"github.com/geolens/geolens/domain"
)

func joinKeywords(words []string) string {
	if len(words) == 0 {
		return "选型、对比与落地"
	}
	return strings.Join(words, "、")
}

func competitorLine(brand domain.Brand) string {
	if len(brand.Competitors) == 0 {
		return ""
	}
	return "常见对比对象：" + strings.Join(brand.Competitors, "、") + "。"
}

func llmsTxt(brand domain.Brand) string {
	return fmt.Sprintf(`# %s

> %s

%s 面向正在评估 %s 的团队。%s

## 核心页面

- 首页：%s/
- 产品与能力：%s/product/
- 对比与选型：%s/compare/
- 常见问题：%s/faq/

## 关键事实

- 品牌名称：%s
- 官网：%s
- 适用场景：%s
- %s

## 对比与选型

选择 %s 时，团队通常关注能力边界、接入成本与可验证的结果。对比页按同一组维度逐项列出事实，便于直接引用。

## 常见问题

- %s 适合谁？
- %s 与同类方案的差别是什么？
- 如何验证 %s 的效果？

## 引用说明

本文件列出可公开引用的事实页面。内容以 HTML 页面为准，两者保持一致。
`, brand.Name, brand.Description, brand.Name, joinKeywords(brand.Keywords), competitorLine(brand),
		brand.Website, brand.Website, brand.Website, brand.Website,
		brand.Name, brand.Website, joinKeywords(brand.Keywords), competitorLine(brand),
		brand.Name, brand.Name, brand.Name, brand.Name)
}

func jsonLD(brand domain.Brand) string {
	return fmt.Sprintf(`{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      "name": %q,
      "url": %q,
      "description": %q
    },
    {
      "@type": "Product",
      "name": %q,
      "brand": { "@type": "Brand", "name": %q },
      "description": %q,
      "url": %q
    },
    {
      "@type": "FAQPage",
      "mainEntity": [
        {
          "@type": "Question",
          "name": "%s 适合谁？",
          "acceptedAnswer": {
            "@type": "Answer",
            "text": "适合正在评估 %s 的团队，尤其是需要按同一组维度比较方案的场景。"
          }
        },
        {
          "@type": "Question",
          "name": "%s 与同类方案的差别是什么？",
          "acceptedAnswer": {
            "@type": "Answer",
            "text": "差别集中在能力边界、接入成本与可验证的结果。对比页逐项列出事实。"
          }
        }
      ]
    }
  ]
}
`, brand.Name, brand.Website, brand.Description, brand.Name, brand.Name,
		brand.Description, brand.Website,
		brand.Name, joinKeywords(brand.Keywords), brand.Name)
}

func comparisonPage(brand domain.Brand) string {
	rows := ""
	if len(brand.Competitors) == 0 {
		rows = "| 待补充同类方案 | — | — |\n"
	} else {
		for _, c := range brand.Competitors {
			rows += fmt.Sprintf("| %s | 待核事实 | 待核事实 |\n", c)
		}
	}
	return fmt.Sprintf(`# %s 与同类方案对比

用同一组维度比较，事实逐项列出。%s

| 方案 | 能力边界 | 接入成本 |
| --- | --- | --- |
| %s | 待核事实 | 待核事实 |
%s
## 怎么选

- 需要 %s：优先看能力边界。
- 需要快速上线：优先看接入成本。
- 需要可验证结果：要求提供复扫前后的原始答案。

> 占位文字「待核事实」必须替换成可公开验证的内容后再发布。
`, brand.Name, competitorLine(brand), brand.Name, rows, joinKeywords(brand.Keywords))
}

func faqPage(brand domain.Brand) string {
	return fmt.Sprintf(`# %s 常见问题

## %s 适合谁？

适合正在评估 %s 的团队。

## %s 与同类方案的差别是什么？

差别集中在能力边界、接入成本与可验证的结果。对比页按同一组维度列出事实。%s

## 如何验证效果？

保存每次提问的原始答案、模型版本、日期与搜索开关状态，复扫同一批问题后只比较两次都有效的答案。%s

> 发布时在页面 <head> 同步注入 FAQPage JSON-LD（见 organization.jsonld）。
`, brand.Name, brand.Name, joinKeywords(brand.Keywords), brand.Name, competitorLine(brand), domain.ObservationNote)
}

func robotsSnippet() string {
	return `# 允许 AI 抓取公开页面
User-agent: GPTBot
Allow: /

User-agent: OAI-SearchBot
Allow: /

User-agent: ClaudeBot
Allow: /

User-agent: PerplexityBot
Allow: /

User-agent: Google-Extended
Allow: /

Sitemap: /sitemap.xml
`
}

// GenerateArtifacts 生成可直接粘贴或下载的成品文件（F-503 / DoD 7）。
// 每个成品都带放置路径与验证方式，用户不需要再自己组织内容。
func GenerateArtifacts(brand domain.Brand) []domain.FixArtifact {
	return []domain.FixArtifact{
		{
			Type: domain.ArtifactLLMsTxt, Filename: "llms.txt",
			TargetPath: brand.Website + "/llms.txt",
			Validation: "部署后访问 /llms.txt 应返回 200，且内容与页面一致；在站点就绪度检查中该项变为 pass。",
			Content:    llmsTxt(brand),
		},
		{
			Type: domain.ArtifactJSONLD, Filename: "organization.jsonld",
			TargetPath: "首页 <head> 中的 <script type=\"application/ld+json\">",
			Validation: "用 Google Rich Results Test 或 schema.org validator 校验无错误；就绪度检查的 JSON-LD 项变为 pass。",
			Content:    jsonLD(brand),
		},
		{
			Type: domain.ArtifactComparison, Filename: "compare.md",
			TargetPath: brand.Website + "/compare/",
			Validation: "发布后把「待核事实」全部替换为可验证内容，页面需含 <h1> 与可见文本；就绪度检查的对比内容项变为 pass。",
			Content:    comparisonPage(brand),
		},
		{
			Type: domain.ArtifactFAQ, Filename: "faq.md",
			TargetPath: brand.Website + "/faq/",
			Validation: "发布后确认页面含 FAQPage JSON-LD；就绪度检查的 FAQ 项变为 pass。",
			Content:    faqPage(brand),
		},
		{
			Type: domain.ArtifactRobots, Filename: "robots-ai-allow.txt",
			TargetPath: brand.Website + "/robots.txt",
			Validation: "合并到现有 robots.txt 后，用各爬虫 UA 抓取首页应返回 200；就绪度检查的 AI 爬虫项变为 pass。",
			Content:    robotsSnippet(),
		},
	}
}

var artifactForCheck = map[string][]domain.ArtifactType{
	"llms_txt":      {domain.ArtifactLLMsTxt},
	"ai_crawlers":   {domain.ArtifactRobots},
	"robots_txt":    {domain.ArtifactRobots},
	"json_ld":       {domain.ArtifactJSONLD},
	"schema_types":  {domain.ArtifactJSONLD},
	"comparison":    {domain.ArtifactComparison},
	"faq":           {domain.ArtifactFAQ},
	"value_prop":    {domain.ArtifactLLMsTxt, domain.ArtifactComparison},
	"facts":         {domain.ArtifactComparison},
}

var p1Checks = map[string]bool{
	"llms_txt": true, "ai_crawlers": true, "robots_txt": true, "json_ld": true, "https": true,
}
var p2Checks = map[string]bool{
	"comparison": true, "faq": true, "value_prop": true, "schema_types": true, "sitemap": true, "facts": true,
}

// FixList 生成优先级修复清单（F-502）。每条都带证据、要改动的成品、目标指标与验收口径。
func FixList(audit *domain.SiteScore, visibility domain.MetricSet) []domain.FixItem {
	items := []domain.FixItem{}
	if audit != nil {
		for _, c := range audit.Checks {
			if c.Status == domain.CheckPass {
				continue
			}
			priority := "P3"
			if p1Checks[c.ID] {
				priority = "P1"
			} else if p2Checks[c.ID] {
				priority = "P2"
			}
			items = append(items, domain.FixItem{
				Priority:      priority,
				Problem:       c.Label + "：" + c.Evidence,
				Evidence:      []string{"check:" + c.ID},
				Change:        c.Fix,
				TargetMetric:  "站点就绪度 /100",
				Acceptance:    "复扫后该检查项变为 pass。仅显示变化，不证明修复导致变化。",
				ArtifactTypes: artifactForCheck[c.ID],
			})
		}
	}
	if visibility.Usable > 0 && visibility.MentionRate < 0.5 {
		items = append(items, domain.FixItem{
			Priority:      "P1",
			Problem:       fmt.Sprintf("Buyer Question 提及率 %.0f%%（有效答案 %d 条），多数答案未出现本品牌", visibility.MentionRate*100, visibility.Usable),
			Evidence:      []string{"metric:buyer_question_mention_rate"},
			Change:        "补齐可引用的对比页、FAQ 与实体结构化数据，让答案有可引用的公开事实。",
			TargetMetric:  "Buyer Question 提及率",
			Acceptance:    fmt.Sprintf("复扫同一批问题后，用 matched comparison 比较两次都 valid 的答案。%s", domain.ObservationNote),
			ArtifactTypes: []domain.ArtifactType{domain.ArtifactComparison, domain.ArtifactFAQ, domain.ArtifactJSONLD},
		})
	}
	// P1 在前，其次 P2、P3。
	ordered := make([]domain.FixItem, 0, len(items))
	for _, p := range []string{"P1", "P2", "P3"} {
		for _, it := range items {
			if it.Priority == p {
				ordered = append(ordered, it)
			}
		}
	}
	return ordered
}