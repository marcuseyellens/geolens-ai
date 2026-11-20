package aeo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/answerlens/answerlens/domain"
)

// ---- A-WP-1.5 / A-WP-2 专属验收：中立性声明与全站禁词 CI ----

// BannedPhrases 是因果承诺与确定性收录表述的禁用词。
// 出现即视为对客文案违规；「不保证 / 不预测 / ≠」等否定式措辞不在禁用范围。
var BannedPhrases = []string{
	"保证被推荐", "保证收录", "一定会被推荐", "一定收录",
	"必然被推荐", "确保排名", "提升排名", "带来转化提升",
	"会拒绝收录", "拒绝收录", "肯定收录", "保证排名",
	"导致排名上升", "修复带来 +", "保证被 AI 推荐",
}

// DeterministicIndexingPhrases 是「平台会怎样」的确定性表述，页面质量结论禁用（A-WP-2）。
var DeterministicIndexingPhrases = []string{
	"perplexity 会拒绝收录", "google 会拒绝收录", "ai 会拒绝收录",
	"一定会被索引", "必然被索引", "保证被索引",
}

// NegationMarkers 让禁用词检查不误伤「不保证」「本服务不提供」这类合规否定句。
// 中文免责短语常在命中词之后，因此同时检查前后窗口。
var NegationMarkers = []string{
	"不", "未", "无", "绝不", "不会", "无法", "不代表", "不预测", "不承诺",
	"不提供", "不保证", "并非", "不意味着", "非", "≠",
}

// Violation 是一条禁词命中记录。
type Violation struct {
	File   string
	Line   int
	Phrase string
	Text   string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s:%d 命中禁词 %q：%s", v.File, v.Line, v.Phrase, v.Text)
}

// ScanTextForBannedPhrases 扫描一段文本，返回禁用词命中（跳过否定式句子）。
func ScanTextForBannedPhrases(text string) []string {
	low := strings.ToLower(text)
	hits := []string{}
	for _, phrase := range append(append([]string{}, BannedPhrases...), DeterministicIndexingPhrases...) {
		p := strings.ToLower(phrase)
		idx := 0
		for {
			at := strings.Index(low[idx:], p)
			if at < 0 {
				break
			}
			pos := idx + at
			if !isNegated(low, pos) {
				hits = append(hits, phrase)
				break
			}
			idx = pos + len(p)
			if idx >= len(low) {
				break
			}
		}
	}
	return hits
}

// isNegated 检查命中词附近是否有否定/免责措辞。
// 只看同一小句，避免段落里其他位置的「不」把真正的承诺句豁免掉。
func isNegated(low string, pos int) bool {
	// 按 rune 取窗口：中文一个字占 3 字节，用字节切会漏掉后面的免责短语。
	runes := []rune(low)
	at := len([]rune(low[:pos]))
	start, end := at-16, at+24
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	// 命中词之前：截到最近的小句边界
	before := string(runes[start:at])
	if cut := strings.LastIndexAny(before, "。；;！!？?\n|"); cut >= 0 {
		before = before[cut+1:]
	}
	for _, n := range NegationMarkers {
		if strings.Contains(before, strings.ToLower(n)) {
			return true
		}
	}
	// 命中词之后：只允许「本服务不提供/也不承诺」这类紧邻免责，遇小句边界即停止
	after := string(runes[at:end])
	if cut := strings.IndexAny(after, "。；;！!？?\n"); cut >= 0 {
		after = after[:cut]
	}
	for _, n := range []string{"不提供", "不承诺", "不保证", "不代表", "不预测", "不意味着", "并非"} {
		if strings.Contains(after, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// CustomerFacingGlobs 是需要进 CI 的对客文案文件。
var CustomerFacingGlobs = []string{
	"frontend/src/routes/*.tsx",
	"frontend/src/lib/*.ts",
	"frontend/index.html",
	"frontend/public/*.txt",
	"internal/siteaudit/artifacts.go",
	"application/diagnostics.go",
	"domain/*.go",
}

// ScanRepoForBannedPhrases 扫描仓库对客文案，返回全部禁词命中（A-WP-1.5 CI 入口）。
func ScanRepoForBannedPhrases(root string) ([]Violation, error) {
	seen := map[string]bool{}
	out := []Violation{}
	for _, pattern := range CustomerFacingGlobs {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return nil, err
		}
		for _, path := range matches {
			if seen[path] {
				continue
			}
			seen[path] = true
			violations, err := scanFile(path)
			if err != nil {
				return nil, err
			}
			out = append(out, violations...)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

func scanFile(path string) ([]Violation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []Violation{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "//") || strings.HasPrefix(text, "*") {
			continue
		}
		for _, phrase := range ScanTextForBannedPhrases(text) {
			out = append(out, Violation{File: path, Line: line, Phrase: phrase, Text: truncate(text, 120)})
		}
	}
	return out, scanner.Err()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// NeutralityVisible 检查中立性声明是否出现在给定内容中（落地页 / 报告）。
func NeutralityVisible(content string) bool {
	return strings.Contains(content, domain.NeutralityStatement)
}

// ReportMethodologyNotes 返回报告必须带的中立性与口径说明（A-WP-1.5 / A-WP-5.3）。
func ReportMethodologyNotes() []string {
	return []string{
		domain.NeutralityStatement,
		"任一含数字的组件都标注 source_type 与 methodology_version；三类来源分表展示，不合并。",
		"任一占比都伴随 n_valid 与采集窗口；结论仅代表本次采样。",
		"页面质量为概率性风险提示，阈值由本平台设定，非平台规则。",
		"失败批次不进基线；告警仅显示变化，不证明修复导致变化。",
	}
}
