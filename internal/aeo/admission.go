package aeo

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/geolens/geolens/domain"
)

// ---- A-WP-2.1 8 项索引准入检测器 ----
//
// 合规红线：只依据公开可访问形态的原始 HTML 做判定，不尝试绕过任何反爬机制。
// anti_bot_wall / captcha 只报告「检测到公开的挑战页特征」，绝不发起挑战或模拟登录。

// PageSnapshot 是一次公开抓取的原始证据（不执行 JS，不做绕过）。
type PageSnapshot struct {
	URL        string
	Status     int
	Body       string
	Headers    map[string]string
	Rendered   bool // 是否由真实渲染得到；false 时依赖渲染的检测器返回 unknown
	FetchedURL string
}

// AdmissionConfig 允许单配置下线整组或单个检测器（A-WP-2.4）。
type AdmissionConfig struct {
	Enabled  bool
	Disabled map[domain.AdmissionDetector]bool
	Weights  map[domain.AdmissionDetector]float64
}

func DefaultAdmissionConfig() AdmissionConfig {
	return AdmissionConfig{Enabled: true, Disabled: map[domain.AdmissionDetector]bool{}}
}

// InspectAdmission 对页面跑 8 类检测，返回带证据、风险提示与来源链接的结论。
func InspectAdmission(p PageSnapshot, cfg AdmissionConfig) []domain.AdmissionCheck {
	out := make([]domain.AdmissionCheck, 0, len(domain.AllAdmissionDetectors()))
	for _, d := range domain.AllAdmissionDetectors() {
		c := runDetector(d, p)
		c.Provenance = domain.Provenance{
			SourceType: domain.SourceObserved, MethodologyVersion: domain.MethodologyVersion,
			NValid: 1, Window: p.URL,
		}
		c.RiskNotice = domain.AdmissionRiskNotice
		c.SourceURL = p.URL
		if !cfg.Enabled || cfg.Disabled[d] {
			c.Status = domain.AdmissionUnknown
			c.Evidence = "该检测器已在配置中下线，结论不参与分母"
		}
		out = append(out, c)
	}
	return out
}

func newCheck(d domain.AdmissionDetector, status domain.AdmissionStatus, evidence, fix string) domain.AdmissionCheck {
	return domain.AdmissionCheck{Detector: d, Status: status, Evidence: evidence, Fix: fix}
}

func runDetector(d domain.AdmissionDetector, p PageSnapshot) domain.AdmissionCheck {
	low := strings.ToLower(p.Body)
	switch d {
	case domain.DetectErrorPage:
		if p.Status >= 400 || p.Status == 0 {
			return newCheck(d, domain.AdmissionFail, fmt.Sprintf("HTTP %d", p.Status), "修复服务端错误，公开页面应返回 200")
		}
		if title := extractTitle(low); title != "" && (strings.Contains(title, "404") || strings.Contains(title, "not found") || strings.Contains(title, "error")) {
			return newCheck(d, domain.AdmissionPartial, "标题含错误页特征："+title, "确认 URL 是否指向真实内容页")
		}
		return newCheck(d, domain.AdmissionPass, fmt.Sprintf("HTTP %d，无错误页特征", p.Status), "")

	case domain.DetectAntiBotWall:
		// 只识别公开的 challenge 特征，不尝试通过。
		markers := []string{"cf-challenge", "checking your browser", "enable javascript and cookies", "px-captcha", "datadome", "access denied"}
		for _, m := range markers {
			if strings.Contains(low, m) {
				return newCheck(d, domain.AdmissionFail, "检测到公开反爬挑战页特征："+m, "为公开页面提供无需挑战即可访问的内容；不要绕过反爬")
			}
		}
		if !p.Rendered {
			return newCheck(d, domain.AdmissionUnknown, "未做真实渲染，无法判定 JS 挑战页", "接入渲染采集后可判定")
		}
		return newCheck(d, domain.AdmissionPass, "渲染结果未见挑战页特征", "")

	case domain.DetectCaptcha:
		markers := []string{"g-recaptcha", "hcaptcha", "h-captcha", "cf-turnstile", "type=\"captcha\"", "captcha-container"}
		for _, m := range markers {
			if strings.Contains(low, m) {
				return newCheck(d, domain.AdmissionPartial, "检测到验证码组件："+m, "确保核心事实内容无需通过验证码即可读取")
			}
		}
		return newCheck(d, domain.AdmissionPass, "未见验证码组件", "")

	case domain.DetectThinContent:
		text := VisibleText(p.Body)
		// SPA 不误判：存在挂载点 + 模块脚本时，交给渲染结果判定（A-WP-2.1 验收）。
		if !p.Rendered && looksLikeSPA(p.Body) {
			return newCheck(d, domain.AdmissionUnknown, "检测到 SPA 挂载结构，原始 HTML 不足以判定正文厚度", "用真实渲染结果复核")
		}
		runes := utf8.RuneCountInString(text)
		if runes < 200 {
			return newCheck(d, domain.AdmissionFail, fmt.Sprintf("可见正文仅 %d 字", runes), "补充可被直接引用的事实正文")
		}
		return newCheck(d, domain.AdmissionPass, fmt.Sprintf("可见正文约 %d 字", runes), "")

	case domain.DetectSEOFiller:
		text := VisibleText(p.Body)
		words := strings.Fields(strings.ToLower(text))
		if len(words) < 80 {
			return newCheck(d, domain.AdmissionPass, "正文过短，不做堆砌判定", "")
		}
		counts := map[string]int{}
		for _, w := range words {
			counts[w]++
		}
		top, topN := "", 0
		for w, n := range counts {
			if n > topN {
				top, topN = w, n
			}
		}
		if float64(topN)/float64(len(words)) > 0.18 {
			return newCheck(d, domain.AdmissionPartial, fmt.Sprintf("高频词 %q 占 %.0f%%", top, float64(topN)/float64(len(words))*100), "用具体事实替换重复关键词")
		}
		return newCheck(d, domain.AdmissionPass, "未见明显关键词堆砌", "")

	case domain.DetectNavigationOnly:
		// 只看正文结构标签，不复用「有用信息」的文本信号，避免检测器互相耦合。
		hasStructure := strings.Contains(low, "<h1") || strings.Contains(low, "<p") ||
			strings.Contains(low, "<dl") || strings.Contains(low, "<article") ||
			strings.Contains(low, "<main") || strings.Contains(low, "<table")
		if !hasStructure {
			return newCheck(d, domain.AdmissionPartial, "页面未出现标题或正文结构标签，疑似纯导航", "在首屏提供可见正文与关键事实")
		}
		return newCheck(d, domain.AdmissionPass, "存在标题/正文结构", "")

	case domain.DetectUsefulInfo:
		text := VisibleText(p.Body)
		signals := []string{"价格", "规格", "参数", "常见问题", "faq", "price", "对比", "$"}
		hits := []string{}
		for _, s := range signals {
			if strings.Contains(strings.ToLower(text), strings.ToLower(s)) {
				hits = append(hits, s)
			}
		}
		if len(hits) == 0 {
			return newCheck(d, domain.AdmissionPartial, "未找到可引用的事实信号", "补充价格、规格、FAQ 等结构化事实")
		}
		return newCheck(d, domain.AdmissionPass, "命中事实信号："+strings.Join(hits, "、"), "")

	case domain.DetectPIIExposure:
		// 只检测明显个人标识（身份证/手机号），不把公开客服邮箱算作 PII。
		if idCard.MatchString(p.Body) || mobileCN.MatchString(p.Body) {
			return newCheck(d, domain.AdmissionPartial, "正文疑似包含个人标识信息", "移除或脱敏后再对外发布")
		}
		return newCheck(d, domain.AdmissionPass, "未见明显个人标识", "")
	}
	return newCheck(d, domain.AdmissionUnknown, "未知检测器", "")
}

var (
	idCard   = regexp.MustCompile(`\b\d{17}[\dXx]\b`)
	mobileCN = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]+>`)
)

// VisibleText 剥离脚本/样式/标签，得到用于内容判定的可见文本。
func VisibleText(html string) string {
	s := scriptRe.ReplaceAllString(html, " ")
	s = styleRe.ReplaceAllString(s, " ")
	s = tagRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func looksLikeSPA(html string) bool {
	low := strings.ToLower(html)
	return (strings.Contains(low, "id=\"root\"") || strings.Contains(low, "id='root'") || strings.Contains(low, "id=\"app\"")) &&
		strings.Contains(low, "type=\"module\"")
}

func extractTitle(low string) string {
	start := strings.Index(low, "<title")
	if start < 0 {
		return ""
	}
	gt := strings.Index(low[start:], ">")
	if gt < 0 {
		return ""
	}
	rest := low[start+gt+1:]
	end := strings.Index(rest, "</title>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// ---- A-WP-2.2 / 2.3 状态模型与权重 ----

// AdmissionDenominator 只统计可判定项；unknown 不进分母（§8 第 2 条）。
func AdmissionDenominator(checks []domain.AdmissionCheck) (pass, partial, fail, unknown int) {
	for _, c := range checks {
		switch c.Status {
		case domain.AdmissionPass:
			pass++
		case domain.AdmissionPartial:
			partial++
		case domain.AdmissionFail:
			fail++
		default:
			unknown++
		}
	}
	return
}

// AdmissionGroupScore 返回该检查组的得分贡献。unknown 不扣分：
// 分母只含可判定项，全为 unknown 时整组不参与评分。
func AdmissionGroupScore(checks []domain.AdmissionCheck, weight float64) (earned float64, scored float64) {
	pass, partial, fail, _ := AdmissionDenominator(checks)
	n := pass + partial + fail
	if n == 0 {
		return 0, 0
	}
	earned = weight * (float64(pass) + 0.5*float64(partial)) / float64(n)
	return earned, weight
}

// AdmissionFixOrder 把索引准入类问题排在 P1（A-WP-2.3）。
func AdmissionFixOrder(checks []domain.AdmissionCheck) []string {
	bad := []string{}
	for _, c := range checks {
		if c.Status == domain.AdmissionFail || c.Status == domain.AdmissionPartial {
			bad = append(bad, string(c.Detector)+"："+c.Fix)
		}
	}
	return bad
}
