package db

import (
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// AEO 工作台整改：核心链路 6 项可见；扩展模块（财务/广告/Agent）与 retired 6 项不可见。
func TestNavGuards(t *testing.T) {
	d := newTestDB(t)
	if err := d.MigrateUp(); err != nil {
		t.Fatal(err)
	}
	nav, err := d.VisibleNav(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(nav) != 6 {
		t.Fatalf("可见导航应为 6 项核心链路，实得 %d", len(nav))
	}
	hasAccount := false
	for _, n := range nav {
		if n.NavKey == "account" {
			hasAccount = true
		}
		for _, banned := range []string{"finance", "ads", "agents", "/evidence-drawer", "/prompts", "/site-audit", "/reports", "/fix-kit", "/rescan-compare"} {
			if n.NavKey == banned || n.RoutePath == banned {
				t.Fatalf("扩展模块或 retired 路由泄漏到客户主导航: %s", banned)
			}
		}
	}
	if !hasAccount {
		t.Fatal("缺少账户项")
	}
	bad, err := d.HasRetiredRoute()
	if err != nil || bad {
		t.Fatalf("retired 项不得可见且有路由: bad=%v err=%v", bad, err)
	}
	// 角色过滤：role_bit=1 只应看到 role_mask 含 1 的可见项（本种子全 7，故仍 6 项核心链路）
	roleNav, _ := d.VisibleNav(1)
	if len(roleNav) != 6 {
		t.Fatalf("role=1 导航数异常: %d", len(roleNav))
	}
}

// 迁移可重复执行、16 表齐全、新表空启动。
func TestMigrateRepeatableAndEmpty(t *testing.T) {
	d := newTestDB(t)
	for i := 0; i < 3; i++ {
		if err := d.MigrateUp(); err != nil {
			t.Fatalf("第 %d 次迁移失败: %v", i+1, err)
		}
	}
	n, err := d.TableCount()
	if err != nil || n != 16 {
		t.Fatalf("应建 16 张表，实得 %d err=%v", n, err)
	}
	var facts int
	if err := d.QueryRow(`SELECT COUNT(*) FROM brand_fact`).Scan(&facts); err != nil || facts != 0 {
		t.Fatalf("新表应空启动，facts=%d err=%v", facts, err)
	}
	var nav int
	if err := d.QueryRow(`SELECT COUNT(*) FROM nav_item`).Scan(&nav); err != nil || nav != 15 {
		t.Fatalf("导航应 9 可见+6 retired=15，实得 %d err=%v", nav, err)
	}
}

// §9 回滚演练：Down 后表清空，再 Up 恢复；幂等。
func TestRollbackDrill(t *testing.T) {
	d := newTestDB(t)
	if err := d.MigrateUp(); err != nil {
		t.Fatal(err)
	}
	if err := d.MigrateDown(); err != nil {
		t.Fatal(err)
	}
	n, _ := d.TableCount()
	if n != 0 {
		t.Fatalf("Down 后应无本批表，实得 %d", n)
	}
	if err := d.MigrateUp(); err != nil {
		t.Fatalf("回滚后重建失败: %v", err)
	}
	n, _ = d.TableCount()
	if n != 16 {
		t.Fatalf("重建后应 16 表，实得 %d", n)
	}
}
