// Package siteaudit 实现 F-401~F-404 的免注册站点就绪度检查。
//
// 它只回答「页面技术条件是否具备」，不推断 AI 是否推荐该品牌。
// 分数与 AI 可见性分并列展示，绝不合成综合分（PRD F-404 / DoD 5）。
package siteaudit

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/geolens/geolens/domain"
)

// FetchResult 是一次页面抓取的原始证据。
type FetchResult struct {
	Status    int
	Body      string
	ElapsedMS int
	Err       error
}

// Fetcher 抽象抓取，便于离线演示与单元测试注入。
type Fetcher interface {
	Fetch(rawURL string) FetchResult
}

// HTTPFetcher 是生产抓取器，带超时与内网地址防护。
type HTTPFetcher struct {
	Client *http.Client
}

func (f HTTPFetcher) Fetch(rawURL string) FetchResult {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	if err := validatePublicURL(rawURL); err != nil {
		return FetchResult{Err: err}
	}
	start := time.Now()
	resp, err := client.Get(rawURL)
	if err != nil {
		return FetchResult{Err: err, ElapsedMS: int(time.Since(start).Milliseconds())}
	}
	defer resp.Body.Close()
	buf := make([]byte, 0, 32768)
	tmp := make([]byte, 4096)
	for len(buf) < 256*1024 {
		n, readErr := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			break
		}
	}
	return FetchResult{
		Status: resp.StatusCode, Body: string(buf),
		ElapsedMS: int(time.Since(start).Milliseconds()),
	}
}

// validatePublicURL 阻止用站点检查功能访问内网地址（SSRF 防护）。
func validatePublicURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("仅支持 http/https 的公开地址")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return fmt.Errorf("不允许访问内网地址")
	}
	if ip == nil && (strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".local")) {
		return fmt.Errorf("不允许访问本机地址")
	}
	return nil
}

// NormalizeURL 补全协议并去掉末尾斜杠。
func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("请填写网址")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("网址格式不正确")
	}
	return strings.TrimSuffix(u.Scheme + "://" + u.Host, "/"), nil
}

func lower(s string) string { return strings.ToLower(s) }

// disallowsAll 只把「Disallow: /」单独一行视作全站禁止，避免误判 /blog 之类的路径。
func disallowsAll(body string) bool {
	for _, line := range strings.Split(lower(body), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "disallow: /" {
			return true
		}
	}
	return false
}

func check(id string, group domain.AuditGroup, label, fix string, ok bool, evidence string) domain.AuditCheck {
	status := domain.CheckFail
	if ok {
		status = domain.CheckPass
	}
	return domain.AuditCheck{ID: id, Group: group, Label: label, Status: status, Evidence: evidence, Fix: fix}
}

func partial(c domain.AuditCheck) domain.AuditCheck {
	c.Status = domain.CheckPartial
	return c
}

var aiBots = []string{"GPTBot", "ClaudeBot", "PerplexityBot", "Google-Extended", "OAI-SearchBot"}

// Audit 对站点做一次就绪度检查，返回 /100 分数、逐项证据与最高优先级修复建议。
func Audit(rawURL string, f Fetcher) (domain.SiteScore, error) {
	base, err := NormalizeURL(rawURL)
	if err != nil {
		return domain.SiteScore{}, err
	}
	home := f.Fetch(base + "/")
	robots := f.Fetch(base + "/robots.txt")
	llms := f.Fetch(base + "/llms.txt")
	sitemap := f.Fetch(base + "/sitemap.xml")

	checks := []domain.AuditCheck{}
	reachable := home.Err == nil && home.Status >= 200 && home.Status < 400
	robotsOK := robots.Err == nil && robots.Status == 200
	robotsBody := lower(robots.Body)
	body := lower(home.Body)

	if !reachable {
		errText := "首页无法访问"
		if home.Err != nil {
			errText = fmt.Sprintf("首页无法访问：%v", home.Err)
		} else {
			errText = fmt.Sprintf("首页返回 HTTP %d", home.Status)
		}
		// 站点不可达时，所有依赖页面内容的检查都记为 fail，并写清证据，绝不猜测。
		checks = append(checks,
			check("https", domain.GroupHealth, "HTTPS 可访问", "配置 HTTPS 并保证首页返回 200", false, errText),
			check("robots_txt", domain.GroupCrawlability, "robots.txt 可访问", "在站点根目录发布 /robots.txt", false, errText),
			check("ai_crawlers", domain.GroupCrawlability, "未禁止 AI 爬虫", "开放 GPTBot、ClaudeBot、PerplexityBot 等抓取", false, errText),
			check("llms_txt", domain.GroupCrawlability, "llms.txt 已发布", "按 llmstxt.org 规范生成 /llms.txt", false, errText),
			check("sitemap", domain.GroupCrawlability, "sitemap.xml 已发布", "生成并提交 /sitemap.xml", false, errText),
			check("json_ld", domain.GroupStructured, "JSON-LD 结构化数据", "在 <head> 注入 Organization/Product JSON-LD", false, errText),
			check("canonical", domain.GroupStructured, "canonical 与 meta description", "补齐 canonical 与 description", false, errText),
			check("value_prop", domain.GroupContent, "价值主张与实体描述", "在首页首屏写清品牌定位与适用对象", false, errText),
			check("comparison", domain.GroupContent, "对比/选择类内容", "产出可引用的对比页与 FAQ 页", false, errText),
		)
	} else {
		checks = append(checks,
			check("https", domain.GroupHealth, "HTTPS 可访问", "改用 HTTPS 并修复证书", strings.HasPrefix(base, "https://"), "首页 HTTP "+fmt.Sprint(home.Status)),
			check("http_status", domain.GroupHealth, "首页返回 200", "修复重定向或错误状态码", home.Status == 200, fmt.Sprintf("HTTP %d", home.Status)),
			check("response_time", domain.GroupHealth, "首屏响应 < 3s", "优化 TTFB 与静态资源缓存", home.ElapsedMS > 0 && home.ElapsedMS < 3000, fmt.Sprintf("%d ms", home.ElapsedMS)),
			check("lang", domain.GroupHealth, "声明页面语言", "为 <html> 添加 lang 属性", strings.Contains(body, "<html") && strings.Contains(body, "lang="), "检查 <html lang>"),
		)

		crawlability := []domain.AuditCheck{
			check("robots_txt", domain.GroupCrawlability, "robots.txt 可访问", "在站点根目录发布 /robots.txt", robotsOK && !disallowsAll(robots.Body), fmt.Sprintf("/robots.txt HTTP %d", robots.Status)),
		}
		blocked := []string{}
		for _, bot := range aiBots {
			idx := strings.Index(robotsBody, "user-agent: "+lower(bot))
			if idx >= 0 {
				segment := robotsBody[idx : min(len(robotsBody), idx+160)]
				if disallowsAll(segment) {
					blocked = append(blocked, bot)
				}
			}
		}
		crawl := check("ai_crawlers", domain.GroupCrawlability, "未禁止 AI 爬虫", "在 robots.txt 中允许 GPTBot、ClaudeBot、PerplexityBot 等", len(blocked) == 0, fmt.Sprintf("被禁止的爬虫：%v", blocked))
		if robotsOK && len(blocked) > 0 {
			crawl = partial(crawl)
		}
		crawlability = append(crawlability, crawl,
			check("llms_txt", domain.GroupCrawlability, "llms.txt 已发布", "用下方成品文件生成 /llms.txt", llms.Err == nil && llms.Status == 200 && len(llms.Body) > 40, fmt.Sprintf("/llms.txt HTTP %d", llms.Status)),
			check("sitemap", domain.GroupCrawlability, "sitemap.xml 已发布", "生成并提交 /sitemap.xml", sitemap.Err == nil && sitemap.Status == 200, fmt.Sprintf("/sitemap.xml HTTP %d", sitemap.Status)),
		)
		checks = append(checks, crawlability...)

		hasJSONLD := strings.Contains(body, "application/ld+json")
		hasSchemaType := hasJSONLD && (strings.Contains(body, "\"organization\"") || strings.Contains(body, "\"product\"") || strings.Contains(body, "\"faqpage\""))
		checks = append(checks,
			check("json_ld", domain.GroupStructured, "JSON-LD 结构化数据", "在 <head> 注入 Organization/Product JSON-LD", hasJSONLD, "查找 application/ld+json"),
			check("schema_types", domain.GroupStructured, "Organization/Product/FAQPage 类型", "使用下方 JSON-LD 成品补齐实体类型", hasSchemaType, "检查 schema.org @type"),
			check("canonical", domain.GroupStructured, "canonical 链接", "为页面添加 <link rel=\"canonical\">", strings.Contains(body, "rel=\"canonical\"") || strings.Contains(body, "rel=canonical"), "查找 rel=canonical"),
			check("meta_description", domain.GroupStructured, "meta description", "补齐页面 description", strings.Contains(body, "name=\"description\"") || strings.Contains(body, "name=description"), "查找 meta description"),
		)

		hasCompare := strings.Contains(body, "/compare") || strings.Contains(body, "/vs-") || strings.Contains(body, "alternatives") || strings.Contains(body, "对比")
		hasFAQ := strings.Contains(body, "faq") || strings.Contains(body, "常见问题") || strings.Contains(body, "frequently asked")
		hasFacts := strings.Contains(body, "price") || strings.Contains(body, "价格") || strings.Contains(body, "规格") || strings.Contains(body, "参数")
		hasValueProp := strings.Contains(body, "<h1") && len(body) > 500
		checks = append(checks,
			check("value_prop", domain.GroupContent, "价值主张与实体描述", "在首屏写清「谁、解决什么、与谁不同」", hasValueProp, "检查 <h1> 与正文长度"),
			check("comparison", domain.GroupContent, "对比/选择类内容", "用下方对比页成品产出可被 AI 引用的对比内容", hasCompare, "查找对比页链接或 /vs- 页面"),
			check("faq", domain.GroupContent, "FAQ 与可引用问答", "用下方 FAQ 成品补齐高频问题", hasFAQ, "查找 FAQ 区块"),
			check("facts", domain.GroupContent, "价格/规格等可引用事实", "把关键事实写成可见文本，不放在图片里", hasFacts, "查找价格/规格文本"),
		)
	}

	score := domain.SiteScore{
		URL: base, Caveat: domain.ReadinessCaveat, AuditedAt: time.Now(),
		Checks: checks, Source: "live",
	}
	if !reachable {
		score.Source = "unreachable"
	}
	score.Groups = groupScores(checks)
	for _, g := range score.Groups {
		score.Score += g.Earned
	}
	score.TopFix = topFix(checks)
	return score, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var groupLabels = map[domain.AuditGroup]string{
	domain.GroupCrawlability: "可抓取性",
	domain.GroupStructured:   "结构化数据",
	domain.GroupContent:      "内容信号",
	domain.GroupHealth:       "基础健康",
}

var groupOrder = []domain.AuditGroup{
	domain.GroupCrawlability, domain.GroupStructured, domain.GroupContent, domain.GroupHealth,
}

func groupScores(checks []domain.AuditCheck) []domain.GroupScore {
	out := []domain.GroupScore{}
	for _, g := range groupOrder {
		weight := domain.AuditGroupWeights[g]
		var total float64
		count := 0
		for _, c := range checks {
			if c.Group != g {
				continue
			}
			count++
			switch c.Status {
			case domain.CheckPass:
				total += 1
			case domain.CheckPartial:
				total += 0.5
			}
		}
		gs := domain.GroupScore{Group: g, Label: groupLabels[g], Weight: weight}
		if count > 0 {
			gs.Score = total / float64(count)
			gs.Earned = weight * gs.Score
		}
		out = append(out, gs)
	}
	return out
}

// topFix 按对 AI 可见性影响最大的顺序给出 1 条 P1 建议（F-403）。
func topFix(checks []domain.AuditCheck) string {
	priority := []string{"llms_txt", "ai_crawlers", "json_ld", "comparison", "robots_txt", "faq", "value_prop", "schema_types"}
	byID := map[string]domain.AuditCheck{}
	for _, c := range checks {
		byID[c.ID] = c
	}
	for _, id := range priority {
		if c, ok := byID[id]; ok && c.Status != domain.CheckPass {
			return c.Fix
		}
	}
	return "当前检查项均通过，可继续补充可被 AI 引用的对比与事实内容"
}