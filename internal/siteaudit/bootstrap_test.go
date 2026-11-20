package siteaudit_test

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/siteaudit"
)

// staticFetcher 把前端构建输入当作线上响应返回，让自举断言既不依赖网络，
// 也不需要先把 npm 产物跑起来。Vite 会原样保留 index.html 的 <head> 与
// <noscript> 静态证据，并把 public/ 下的文件复制到站点根目录，因此这里读取的
// 就是 F-403 抓取自家落地页时真正能看到的证据。
type staticFetcher struct {
	files map[string]string
}

func (f staticFetcher) Fetch(rawURL string) siteaudit.FetchResult {
	u, err := url.Parse(rawURL)
	if err != nil {
		return siteaudit.FetchResult{Err: err}
	}
	path := u.Path
	if path == "" || path == "/" {
		path = "/index.html"
	}
	body, ok := f.files[path]
	if !ok {
		return siteaudit.FetchResult{Status: 404}
	}
	return siteaudit.FetchResult{Status: 200, Body: body, ElapsedMS: 120}
}

func readFrontend(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(append([]string{"..", "..", "frontend"}, parts...)...)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读取 AEO 静态证据失败 %s: %v", p, err)
	}
	return string(b)
}

func bootstrapFetcher(t *testing.T) staticFetcher {
	t.Helper()
	return staticFetcher{files: map[string]string{
		"/index.html":  readFrontend(t, "index.html"),
		"/robots.txt":  readFrontend(t, "public", "robots.txt"),
		"/llms.txt":    readFrontend(t, "public", "llms.txt"),
		"/sitemap.xml": readFrontend(t, "public", "sitemap.xml"),
	}}
}

// TestBootstrapAuditScore85 对应 WBS §WP-6.4 与 §8 发布验收清单：
// 「自举：用自家产品检查自家落地页 ≥ 85 分」。
func TestBootstrapAuditScore85(t *testing.T) {
	score, err := siteaudit.Audit("https://geolens.ai", bootstrapFetcher(t))
	if err != nil {
		t.Fatalf("自举审计返回错误: %v", err)
	}
	if score.Score < 85 {
		failed := []string{}
		for _, c := range score.Checks {
			if c.Status != domain.CheckPass {
				failed = append(failed, c.ID+"("+string(c.Status)+"): "+c.Evidence)
			}
		}
		t.Fatalf("自家落地页就绪度 %.1f < 85，未通过项：\n  %s", score.Score, strings.Join(failed, "\n  "))
	}

	// 分数达标还不够：静态证据覆盖的 12 项必须逐项 pass，防止以后误删
	// canonical / JSON-LD / llms.txt 等任一资产后仍靠其他分组凑够 85。
	mustPass := []string{
		"robots_txt", "ai_crawlers", "llms_txt", "sitemap",
		"json_ld", "schema_types", "canonical", "meta_description",
		"value_prop", "comparison", "faq", "facts",
	}
	byID := map[string]domain.AuditCheck{}
	for _, c := range score.Checks {
		byID[c.ID] = c
	}
	for _, id := range mustPass {
		c, ok := byID[id]
		if !ok {
			t.Errorf("检查项 %s 缺失", id)
			continue
		}
		if c.Status != domain.CheckPass {
			t.Errorf("检查项 %s 应为 pass，实际 %s（证据：%s）", id, c.Status, c.Evidence)
		}
	}
}

// TestBuiltDistAuditScore85 用构建产物复验同一断言：服务端实际 serve 的是
// frontend/dist，而不是源码。dist 未生成时跳过，避免让 go test 依赖 npm build。
func TestBuiltDistAuditScore85(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "frontend", "dist", "index.html")); err != nil {
		t.Skip("frontend/dist 尚未构建，跳过产物复验")
	}
	f := staticFetcher{files: map[string]string{
		"/index.html":  readFrontend(t, "dist", "index.html"),
		"/robots.txt":  readFrontend(t, "dist", "robots.txt"),
		"/llms.txt":    readFrontend(t, "dist", "llms.txt"),
		"/sitemap.xml": readFrontend(t, "dist", "sitemap.xml"),
	}}
	score, err := siteaudit.Audit("https://geolens.ai", f)
	if err != nil {
		t.Fatalf("构建产物自举审计返回错误: %v", err)
	}
	if score.Score < 85 {
		t.Fatalf("构建产物落地页就绪度 %.1f < 85，请重新执行 npm run build", score.Score)
	}
}

// TestLLMsTxtCoversCorePages 对应 WP-6.4「llms.txt 覆盖核心问题」：
// 核心漏斗页面与高频问题都要能被 AI 直接引用。
func TestLLMsTxtCoversCorePages(t *testing.T) {
	body := readFrontend(t, "public", "llms.txt")
	if len(body) <= 40 {
		t.Fatalf("llms.txt 内容过短（%d 字节），不足以覆盖核心问题", len(body))
	}
	for _, needle := range []string{
		"/check", "/pricing", "/compare", "/privacy", "/terms",
		"常见问题", "$39", "技术就绪 ≠ 被推荐",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("llms.txt 缺少核心事实 %q", needle)
		}
	}
}
