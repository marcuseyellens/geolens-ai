package db

import (
	"path/filepath"
	"testing"
)

// EXD-14 / TC-D-20：12 张新表迁移幂等 + down 回滚演练。
func TestSevenMigrationRollbackDrill(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "seven.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// up 幂等：连续两次不报错。
	if err := d.MigrateUpSeven(); err != nil {
		t.Fatal(err)
	}
	if err := d.MigrateUpSeven(); err != nil {
		t.Fatalf("up 应可重复执行: %v", err)
	}

	// 12 张表全部存在。
	for _, table := range sevenProductsTables {
		var name string
		err := d.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil || name != table {
			t.Errorf("表 %s 未建", table)
		}
	}

	// DB CHECK：published 无 approved_by 应被拒（INV-2）。
	_, err = d.Exec(`INSERT INTO content_change_proposal (brand_id, change_type, proposed_by, payload, status, approved_by)
		VALUES (1, 'content', 'agent:x', '{}', 'published', NULL)`)
	if err == nil {
		t.Error("published 无 approved_by 应被 DB CHECK 拒绝（INV-2）")
	}

	// DB CHECK：send/delete 非 forbidden 应被拒（INV-4）。
	_, err = d.Exec(`INSERT INTO agent_permission_grant (agent_id, scope, resource_ref, decision, decided_by, decided_at)
		VALUES (1, 'send', 'r', 'granted_once', 'user', '2026-01-01')`)
	if err == nil {
		t.Error("send granted_once 应被 DB CHECK 拒绝（INV-4）")
	}

	// down 逆序删表，再 up 可恢复。
	if err := d.MigrateDownSeven(); err != nil {
		t.Fatal(err)
	}
	for _, table := range sevenProductsTables {
		var name string
		err := d.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err == nil {
			t.Errorf("表 %s 未删除", table)
		}
	}
	if err := d.MigrateUpSeven(); err != nil {
		t.Fatalf("回滚后重新 up 失败: %v", err)
	}
}

// 表数量口径：以 §9 迁移顺序的 12 张为准（§3.1 标题 11 为笔误）。
func TestSevenTableCount(t *testing.T) {
	if len(sevenProductsTables) != 12 {
		t.Fatalf("应为 12 张表，实得 %d", len(sevenProductsTables))
	}
}
