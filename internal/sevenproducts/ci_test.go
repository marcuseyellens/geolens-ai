package sevenproducts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/answerlens/answerlens/domain"
)

// EXD-13 / TC-D-19：CI 静态检查（追加项，沿用第五批机制，不新建检查器）。
// 扫描本批新增源码，命中综合分字段 / 效果叙事 / 已删路由 / 自动发布 / 合成分母即失败。

var bannedScoreFields = []string{
	"visibility_score", "overall_success", "health_score", "composite_score",
	"ai_visibility_index", "visibility_index", "improvement_pct", "uplift",
	"优化后提升", "综合可见度分",
}

// finance_health_score 是第五批财务域白名单，不在本批源码出现。
var deletedRoutes = []string{
	"/prompts", "/site-audit", "/fix-kit", "/rescan-compare", "/evidence-drawer", "/reports",
}

func TestTC19_NoCompositeOrNarrativeFields(t *testing.T) {
	roots := []string{
		"../../domain/seven_products_contract.go",
		"../../internal/sevenproducts",
		"../../api/handlers/seven_products_handler.go",
		"../../frontend/src/routes/seven-products-pages.tsx",
		"../../frontend/src/routes/seven-enhancements.tsx",
	}
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".tsx") && !strings.HasSuffix(path, ".ts") {
				return nil
			}
			// 检查器自身含禁词字面量，跳过。
			if strings.HasSuffix(path, "ci_test.go") {
				return nil
			}
			b, _ := os.ReadFile(path)
			for _, line := range strings.Split(string(b), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
					continue
				}
				for _, w := range bannedScoreFields {
					if strings.Contains(line, w) {
						t.Errorf("%s 命中综合分/效果叙事字段 %q: %s", path, w, trimmed)
					}
				}
			}
			return nil
		})
	}
	// 本批响应确有 finance_health_score 白名单吗？应无（属第五批）。
	if _, err := os.Stat("../../domain/finance_contract.go"); err == nil {
		// 白名单存在也不影响本批源码扫描。
	}
}

func TestTC19_NoDeletedRoutes(t *testing.T) {
	path := "../../frontend/src/App.tsx"
	b, _ := os.ReadFile(path)
	for _, r := range deletedRoutes {
		// App.tsx 的 Route path 中不得出现这些字面量。
		if strings.Contains(string(b), `path="`+r+`"`) || strings.Contains(string(b), `path='`+r+`'`) {
			t.Errorf("App.tsx 命中已删路由 %s", r)
		}
	}
	// 意图问题集走 /facts 内嵌，不建 /prompts。
	if strings.Contains(string(b), `path="/prompts"`) {
		t.Error("不得建 /prompts 路由")
	}
}

func TestTC19_FixedNoticesVerbatim(t *testing.T) {
	if domain.NoSourceBlock != "无原始答案来源，该数字已阻断，不进入报告" {
		t.Fatal("NO_SOURCE_BLOCK 被改写")
	}
	if domain.ApprovalRequired != "需人工批准后方可发布，AI 不自动上线" {
		t.Fatal("APPROVAL_REQUIRED 被改写")
	}
	if domain.ForbiddenScope != "该 agent 无发送/删除权限，请求已被拒绝" {
		t.Fatal("FORBIDDEN_SCOPE 被改写")
	}
	if domain.DenominatorSplit != "调用成功率与任务完成率分母不同，分开呈现，不可合并" {
		t.Fatal("DENOMINATOR_SPLIT 被改写")
	}
}

func TestTC19_NoAutoPublish(t *testing.T) {
	g := ApprovalGate{}
	// 无自动发布端点：proposed 不得直达 published；rejected 也不得发布。
	if g.CanTransition(domain.DPProposalProposed, domain.DPProposalPublished) {
		t.Error("存在 proposed→published 直接边")
	}
	if g.CanTransition(domain.DPProposalRejected, domain.DPProposalPublished) {
		t.Error("rejected 不得发布")
	}
}

func TestTC19_CapabilityBlacklist(t *testing.T) {
	for _, scope := range []string{"send", "delete"} {
		if !domain.IsForbiddenScope(scope) {
			t.Errorf("%s 应在硬黑名单", scope)
		}
		if GrantDecision(scope, true) != domain.DPGrantForbidden {
			t.Errorf("%s 即使用人批准也不得 granted_once", scope)
		}
	}
}

func TestTC19_FlagOrthogonal(t *testing.T) {
	f := NewFlag(true)
	if f.Name() != "aeo_seven_products_20261123" {
		t.Fatal("flag 名不符")
	}
	if f.Name() == domain.CustomerFacingFeatureFlag || f.Name() == domain.SocialFeatureFlag ||
		f.Name() == domain.DeltaFeatureFlag || f.Name() == domain.CompetitorFeatureFlag {
		t.Fatal("第六批 flag 与前五 flag 不得同名")
	}
	// 本批不改口径。
	if domain.MethodologyVersionCustomerFacing != "1.3" || domain.MethodologyVersionSocial != "1.3" {
		t.Fatal("methodology_version 应保持 1.3")
	}
}
