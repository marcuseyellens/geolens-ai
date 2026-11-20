package delta

import (
	"net/url"
	"strings"

	"github.com/answerlens/answerlens/domain"
)

// ---- EX-02 retrieval_locale 判定器（D-1 定稿）----

// LocaleSignals 是采集适配器出站后的一次观测输入。
type LocaleSignals struct {
	QueryLocale     string   // 问题集语言（BCP-47）
	PlatformHeader  string   // 平台响应头里的语言（可观测，最高优先级）
	CitationDomains []string // 引用域名（含 ccTLD 线索）
	RawAnswerText   string   // 答案文本
	RunID           string   // inferred 必须绑定的证据
	InferredLocale  string   // 语言检测器给出的候选
	InferredConf    float64  // 语言检测器置信度
}

// InferLocale 按 D-1 固定优先级判定检索语言：
//
//	platform_header > citation_domain > answer_text > inferred > unknown
//
// 约束：
//   - 只允许可观测信号；禁止输出「模型内部以 X 语言检索」类措辞；
//   - inferred 必须附 run_id 证据与 confidence，且 confidence < 0.700 一律降级 unknown；
//   - unknown 不进任何分母；retrieval_locale=unknown 时 confidence 必须为 NULL。
func InferLocale(s LocaleSignals, minConfidence float64) domain.RunAnswerLocaleFields {
	f := domain.RunAnswerLocaleFields{QueryLocale: s.QueryLocale}

	// 1. platform_header
	if v := normalizeLocale(s.PlatformHeader); v != "" {
		return finishLocale(f, v, domain.LocaleSourcePlatformHeader, nil, s.QueryLocale)
	}
	// 2. citation_domain：ccTLD 推断（如 .pl -> pl-PL）
	if v := localeFromDomains(s.CitationDomains); v != "" {
		return finishLocale(f, v, domain.LocaleSourceCitationDomain, nil, s.QueryLocale)
	}
	// 3. answer_text：答案文本里显式给出的语言标签（可观测字符串，不做模型机制声明）
	if v := localeFromAnswerText(s.RawAnswerText); v != "" {
		return finishLocale(f, v, domain.LocaleSourceAnswerText, nil, s.QueryLocale)
	}
	// 4. inferred：答案文本语言检测，需 run_id 证据 + confidence >= min
	if s.InferredLocale != "" && s.RunID != "" && s.InferredConf >= minConfidence {
		c := s.InferredConf
		v := normalizeLocale(s.InferredLocale)
		if v != "" {
			return finishLocale(f, v, domain.LocaleSourceInferred, &c, s.QueryLocale)
		}
	}
	// 5. unknown：confidence 必须为 NULL，locale_mismatch=false
	f.RetrievalLocale = string(domain.LocaleSourceUnknown)
	f.RetrievalLocaleSource = domain.LocaleSourceUnknown
	f.RetrievalConfidence = nil
	f.LocaleMismatch = false
	return f
}

func finishLocale(f domain.RunAnswerLocaleFields, locale string, src domain.RetrievalLocaleSource, conf *float64, queryLocale string) domain.RunAnswerLocaleFields {
	f.RetrievalLocale = locale
	f.RetrievalLocaleSource = src
	f.RetrievalConfidence = conf
	// 不变式：locale_mismatch=true 仅当 query_locale != retrieval_locale 且 retrieval_locale != unknown。
	f.LocaleMismatch = queryLocale != "" && locale != "" &&
		!strings.EqualFold(queryLocale, locale) && locale != string(domain.LocaleSourceUnknown)
	return f
}

var ccTLDLocale = map[string]string{
	"pl": "pl-PL", "de": "de-DE", "fr": "fr-FR", "es": "es-ES",
	"it": "it-IT", "ja": "ja-JP", "ko": "ko-KR", "pt": "pt-BR",
	"nl": "nl-NL", "sv": "sv-SE", "uk": "en-GB", "us": "en-US",
	"cn": "zh-CN", "tw": "zh-TW",
}

func localeFromDomains(domains []string) string {
	for _, d := range domains {
		h := HostOf("https://" + d)
		if i := strings.LastIndex(h, "."); i >= 0 {
			tld := strings.ToLower(h[i+1:])
			if v, ok := ccTLDLocale[tld]; ok {
				return v
			}
		}
	}
	return ""
}

// localeFromAnswerText 只识别答案里显式、可观测的语言标签（如 HTML lang="de-DE"），
// 绝不推断「模型内部以 X 语言检索」。无标签时返回空，交给 inferred / unknown。
func localeFromAnswerText(text string) string {
	low := strings.ToLower(text)
	for _, marker := range []string{`lang="`, `content-language:`, `content-language="`} {
		idx := strings.Index(low, marker)
		if idx < 0 {
			continue
		}
		rest := text[idx+len(marker):]
		end := strings.IndexAny(rest, `"' \n\r\t,;>`)
		if end < 0 {
			end = len(rest)
		}
		if v := normalizeLocale(strings.TrimSpace(rest[:end])); v != "" && bcp47Like(v) {
			return v
		}
	}
	return ""
}

// bcp47Like 只放行形如 xx 或 xx-XX 的 BCP-47 标签，避免把普通词当语言。
func bcp47Like(v string) bool {
	parts := strings.Split(v, "-")
	if len(parts) > 2 || len(parts) == 0 {
		return false
	}
	if len(parts[0]) < 2 || len(parts[0]) > 3 {
		return false
	}
	for _, p := range parts {
		for _, r := range p {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
				return false
			}
		}
	}
	return true
}

func normalizeLocale(raw string) string {
	v := strings.TrimSpace(strings.ReplaceAll(raw, "_", "-"))
	if v == "" || strings.EqualFold(v, "unknown") {
		return ""
	}
	return v
}

// ---- EX-05 citation host 精确归一 + 注册域（DE 主责）----

// HostOf 返回精确 host（含子域名，小写、去端口），不合并到根域。
func HostOf(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return strings.ToLower(strings.TrimSpace(rawURL))
	}
	return strings.ToLower(u.Hostname())
}

// RegistrableDomain 仅作展示分组：取最后两段，禁止用于反推 host、禁止继承权重。
func RegistrableDomain(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// NormalizeCitations 给每条 citation 精确填充 host / registrable_domain，
// 并保留子域独立性（blog.example.com 与 example.com 是两条独立 citation）。
func NormalizeCitations(cites []domain.Citation) []domain.Citation {
	out := make([]domain.Citation, len(cites))
	copy(out, cites)
	for i := range out {
		out[i].Host = HostOf(out[i].URL)
		out[i].RegistrableDomain = RegistrableDomain(out[i].Host)
		// Domain 保持旧值以兼容 1.0；host 独立于 registrable_domain 存储。
	}
	return out
}

// CitationHostCounts 按精确 host 计数（TC-16-08：两条独立计数、不合并、不继承权重）。
func CitationHostCounts(cites []domain.Citation) map[string]int {
	out := map[string]int{}
	for _, c := range NormalizeCitations(cites) {
		if c.Host != "" {
			out[c.Host]++
		}
	}
	return out
}

// ---- EX-04 指标分组（DE 主责 / 契约 9）----

// LocaleSplit 把运行按语言观测分成：默认分母 / mismatch 单独分组 / unknown 排除。
type LocaleSplit struct {
	Default  []domain.RunAnswer
	Mismatch []domain.RunAnswer
	Unknown  []domain.RunAnswer
}

// SplitByLocale 实现 EX-04：
//   - locale_mismatch=true 单独分组，不并入默认 mention rate / SoV / Citation Rate 分母；
//   - retrieval_locale=unknown 不进分母，UI 显示「检索语言未判定」而非空白；
//   - suspect 批次由 caller 先行过滤（EX-12 四数隔离）。
func SplitByLocale(answers []domain.RunAnswer) LocaleSplit {
	var s LocaleSplit
	for _, a := range answers {
		switch {
		case a.RetrievalLocale == "" || a.RetrievalLocale == string(domain.LocaleSourceUnknown):
			s.Unknown = append(s.Unknown, a)
		case a.LocaleMismatch:
			s.Mismatch = append(s.Mismatch, a)
		default:
			s.Default = append(s.Default, a)
		}
	}
	return s
}

// LocaleUINotice 返回 unknown 态的 UI 文案（禁止留白）。
func LocaleUINotice(retrievalLocale string) string {
	if retrievalLocale == "" || retrievalLocale == string(domain.LocaleSourceUnknown) {
		return domain.RetrievalLocaleUndetermined
	}
	return retrievalLocale
}
