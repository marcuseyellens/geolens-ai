package customerfacing

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/answerlens/answerlens/domain"
)

// TestTCF23_BannedWords 扫描本批相关源码与文案，命中综合分或因果词即失败。
// 沿用既有 CI 通道，不新建第二套检查器。
func TestTCF23_BannedWords(t *testing.T) {
	roots := []string{"domain", "internal/customerfacing", "api/handlers", "frontend/src"}
	re := regexp.MustCompile(`//|\*|/\*`)
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") {
				return nil
			}
			if strings.Contains(path, "node_modules") || strings.Contains(path, "ci.go") || strings.Contains(path, "ci_test.go") {
				return nil // 检查器自身含禁词字面量，跳过
			}
			b, _ := os.ReadFile(path)
			line := string(b)
			// 只检查非注释行（粗略：去掉 // 与块注释行）
			for _, l := range strings.Split(line, "\n") {
				if re.MatchString(strings.TrimSpace(l)) {
					continue
				}
				for _, w := range append(compositeBanned, causalBanned...) {
					if strings.Contains(l, w) {
						t.Errorf("%s 命中禁词 %q: %s", path, w, strings.TrimSpace(l))
					}
				}
			}
			return nil
		})
	}
}

// TestTCF23_FixedNotices 三个文案逐字、独立、不可覆盖。
func TestTCF23_FixedNotices(t *testing.T) {
	if domain.CompareNotice != "两次独立观察，不证明因果关系。" {
		t.Fatal("compare_notice 被改写")
	}
	if domain.WriteGateNotice != "本操作生成草案，须经人工批准后生效；广告以暂停态上线，财务不自动过账。" {
		t.Fatal("write_gate_notice 被改写")
	}
	if domain.PeekNotice != "引用与提名仅为观测，引用不等于答案依据。" {
		t.Fatal("peek_notice 被改写")
	}
	if domain.CompareNotice == domain.WriteGateNotice || domain.WriteGateNotice == domain.PeekNotice {
		t.Fatal("三个 notice 不得互相替代")
	}
}

// TestTCF22_WriteGate 所有写操作不得跳过审批直达终态；广告只到 paused_live。
func TestTCF22_WriteGate(t *testing.T) {
	g := Gate{}
	if !g.CanTransition(domain.AdDraft, domain.AdPendingApproval) {
		t.Fatal("draft 应可提交待批")
	}
	if g.CanTransition(domain.AdDraft, domain.AdApproved) {
		t.Fatal("draft 不得直接 approved")
	}
	if g.CanTransition(domain.AdDraft, domain.AdLive) {
		t.Fatal("draft 不得直接 live")
	}
	if g.CanTransition(domain.AdApproved, domain.AdLive) {
		t.Fatal("approved 只到 paused_live")
	}
	if !g.CanTransition(domain.AdApproved, domain.AdPausedLive) {
		t.Fatal("approved 应可暂停上线")
	}
	if !g.CanTransition(domain.AdPausedLive, domain.AdLive) {
		t.Fatal("paused_live 经人工复开才 live")
	}
	if g.FinancePostingAllowed(false) {
		t.Fatal("未批准财务不得过账")
	}
}

// TestTCF8_ReadOnlySQL 溯源 SQL 禁写语句，integration scope 恒 read_only。
func TestTCF8_ReadOnlySQL(t *testing.T) {
	if !IsReadOnlySQL("SELECT x FROM t WHERE y=1") {
		t.Fatal("只读 SELECT 应放行")
	}
	for _, bad := range []string{
		"DROP TABLE t", "UPDATE t SET x=1", "DELETE FROM t",
		"SELECT 1; DROP TABLE t", "PRAGMA table_info(t)", "INSERT INTO t VALUES(1)",
	} {
		if IsReadOnlySQL(bad) {
			t.Fatalf("写语句应被拒: %s", bad)
		}
	}
	if ReadOnlyScope() != domain.IntegrationReadOnly || string(ReadOnlyScope()) != "read_only" {
		t.Fatal("integration scope 恒 read_only")
	}
}

// TestTCF2_TriState 三态分列，无聚合分字段；recommendation 默认未验证。
func TestTCF2_TriState(t *testing.T) {
	var ts domain.TriState
	if ts.Recommendation.Verified {
		t.Fatal("recommendation 默认应未验证")
	}
	// 数字必须带来源与口径才可出（FC-3）
	f := domain.Figure{Label: "x", Value: 1, FigureStatus: domain.FigureActual, MethodologyVersion: "1.3"}
	if f.Valid() {
		t.Fatal("无 source_ref 的数字不得出")
	}
	f.SourceRef = "run://123"
	if !f.Valid() {
		t.Fatal("带来源的数字应可出")
	}
	f.FigureCheckFlag = true
	if f.Valid() {
		t.Fatal("figure_check_flag=true 应阻断发布")
	}
}

// TestTCF7_FlagsOrthogonal 四 flag 正交、口径不变（FC-7）。
func TestTCF7_FlagsOrthogonal(t *testing.T) {
	cf := NewFlag(true)
	cf.SetEnabled(false)
	if cf.Enabled() {
		t.Fatal("customer_facing flag 关闭失败")
	}
	if domain.CustomerFacingFeatureFlag == domain.SocialFeatureFlag ||
		domain.CustomerFacingFeatureFlag == domain.DeltaFeatureFlag ||
		domain.CustomerFacingFeatureFlag == domain.CompetitorFeatureFlag {
		t.Fatal("四 flag 名不得相同")
	}
	if domain.MethodologyVersionCustomerFacing != "1.3" {
		t.Fatal("第五批口径应保持 1.3")
	}
}
