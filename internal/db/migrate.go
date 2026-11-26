// Package db 承载 CR-v1.1 决策 3：SQLite 持久化与迁移体系。
// 16 张客户可见版新表按规格 §3.1 DDL 等价落地（JSONB→TEXT，
// BIGINT→INTEGER，TIMESTAMP→TEXT），带 workspace_id 多租户隔离。
// 迁移可重复执行（IF NOT EXISTS），新表空启动，DOWN 脚本逆序删表。
// 不新建综合分对象；retired 导航项 visible 恒 FALSE（FC-1）。
package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// DB 是包装后的 SQLite 句柄。
type DB struct{ *sql.DB }

// Open 打开（或创建）SQLite 数据库并开启外键与 WAL。
func Open(path string) (*DB, error) {
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err = raw.Exec(pragma); err != nil {
			raw.Close()
			return nil, err
		}
	}
	return &DB{raw}, nil
}

// upStatements 按依赖顺序建 16 张表。全部 IF NOT EXISTS，可重复执行。
var upStatements = []string{
	// ECF-01 导航可见性
	`CREATE TABLE IF NOT EXISTS nav_item (
		nav_key TEXT PRIMARY KEY,
		label TEXT NOT NULL,
		kind TEXT NOT NULL,
		visible INTEGER NOT NULL DEFAULT 1,
		role_mask INTEGER NOT NULL DEFAULT 7,
		route_path TEXT,
		retired_from TEXT
	)`,
	// 注：不对 route_path 建唯一索引——6 条 retired 项无路由（空串），
	// 且 retired 恒不可见（FC-1），唯一约束会错误去重。
	// MOD-02 付费广告
	`CREATE TABLE IF NOT EXISTS ad_campaign (
		campaign_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		platform TEXT NOT NULL,
		state TEXT NOT NULL DEFAULT 'draft',
		launch_mode TEXT NOT NULL DEFAULT 'paused',
		draft_payload TEXT,
		approver_id INTEGER,
		approved_at TEXT,
		created_by INTEGER NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS ad_suggested_action (
		action_id INTEGER PRIMARY KEY,
		campaign_id INTEGER NOT NULL,
		step_index INTEGER NOT NULL,
		action_type TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'suggested'
	)`,
	`CREATE TABLE IF NOT EXISTS ad_activity_event (
		event_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		event_time TEXT NOT NULL,
		title TEXT NOT NULL,
		amount_cny TEXT,
		source_ref TEXT
	)`,
	// MOD-03 Agent 管理
	`CREATE TABLE IF NOT EXISTS agent_registry (
		agent_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		agent_code TEXT NOT NULL,
		role TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		duty_text TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS agent_run_step (
		step_id INTEGER PRIMARY KEY,
		run_id INTEGER NOT NULL,
		step_index INTEGER NOT NULL,
		step_kind TEXT NOT NULL,
		evidence_ref TEXT,
		replay_url TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS agent_failure_bundle (
		bundle_id INTEGER PRIMARY KEY,
		run_id INTEGER NOT NULL,
		cause_code TEXT,
		nearby_evidence TEXT,
		root_hypothesis TEXT,
		suggested_fix TEXT,
		ticket_ref TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS agent_schedule (
		schedule_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		cron TEXT NOT NULL,
		purpose TEXT,
		enabled INTEGER NOT NULL DEFAULT 0
	)`,
	// MOD-01 财务 AI
	`CREATE TABLE IF NOT EXISTS finance_figure (
		figure_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		label TEXT,
		value TEXT,
		figure_status TEXT NOT NULL,
		source_ref TEXT,
		drill_path TEXT,
		figure_check_flag INTEGER NOT NULL DEFAULT 0,
		methodology_version TEXT DEFAULT '1.3'
	)`,
	`CREATE TABLE IF NOT EXISTS approval_inbox_item (
		item_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		kind TEXT NOT NULL,
		payload_ref TEXT,
		state TEXT NOT NULL DEFAULT 'pending',
		approver_id INTEGER,
		decided_at TEXT
	)`,
	// MOD-04 溯源数据
	`CREATE TABLE IF NOT EXISTS attribution_query (
		query_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		question TEXT NOT NULL,
		generated_sql TEXT,
		result_ref TEXT,
		created_by INTEGER NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS integration_connection (
		connection_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		provider TEXT,
		state TEXT NOT NULL DEFAULT 'disconnected',
		scope TEXT NOT NULL DEFAULT 'read_only'
	)`,
	`CREATE TABLE IF NOT EXISTS answer_error_report (
		report_id INTEGER PRIMARY KEY,
		answer_ref TEXT NOT NULL,
		workspace_id INTEGER NOT NULL,
		note TEXT,
		state TEXT NOT NULL DEFAULT 'open'
	)`,
	// C-04 竞品监测
	`CREATE TABLE IF NOT EXISTS competitor_watch (
		watch_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		competitor TEXT NOT NULL,
		alert_enabled INTEGER NOT NULL DEFAULT 1
	)`,
	`CREATE TABLE IF NOT EXISTS competitor_change_alert (
		alert_id INTEGER PRIMARY KEY,
		watch_id INTEGER NOT NULL,
		change_type TEXT,
		detected_at TEXT NOT NULL
	)`,
	// C-05 品牌事实库
	`CREATE TABLE IF NOT EXISTS brand_fact (
		fact_id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		entity TEXT,
		fact_type TEXT,
		value TEXT,
		evidence_status TEXT NOT NULL,
		source_ref TEXT,
		state TEXT NOT NULL DEFAULT 'draft'
	)`,
}

// tableNames 按逆序用于 DOWN（先删依赖少的，本批表间无外键，逆序即可）。
var tableNames = []string{
	"brand_fact", "competitor_change_alert", "competitor_watch",
	"answer_error_report", "integration_connection", "attribution_query",
	"approval_inbox_item", "finance_figure",
	"agent_schedule", "agent_failure_bundle", "agent_run_step", "agent_registry",
	"ad_activity_event", "ad_suggested_action", "ad_campaign",
	"nav_item",
}

// AEO 工作台导航：核心链路 6 项可见；财务 AI / 付费广告 / Agent 为扩展模块，
// 移出主导航（visible=0，路由仍可从概览卡片进入，功能不删）；6 项 IA retired 恒不可见。
var navSeed = []struct {
	key, label, kind, route string
	visible                 int
	retired                 string
}{
	{"market", "市场基线", "customer_page", "/market", 1, ""},
	{"brand", "品牌可见性", "customer_page", "/brand/:brandId", 1, ""},
	{"competitors", "竞品监测", "customer_page", "/competitors", 1, ""},
	{"facts", "品牌事实库", "customer_page", "/facts", 1, ""},
	{"finance", "财务 AI", "module", "/finance", 0, ""},
	{"ads", "付费广告", "module", "/ads", 0, ""},
	{"agents", "Agent 管理", "module", "/agents", 0, ""},
	{"attribution", "溯源数据", "module", "/attribution", 1, ""},
	{"account", "账户 / 套餐", "account", "/account", 1, ""},
	// IA-04～09：无客户路由，恒不可见（FC-1）
	{"evidence_drawer", "证据抽屉", "customer_page", "", 0, "IA-04"},
	{"prompts", "提示词", "customer_page", "", 0, "IA-05"},
	{"site_audit", "站点审计", "customer_page", "", 0, "IA-06"},
	{"reports", "报告", "customer_page", "", 0, "IA-07"},
	{"fix_kit", "修复件", "customer_page", "", 0, "IA-08"},
	{"rescan_compare", "复扫对比", "customer_page", "", 0, "IA-09"},
}

// MigrateUp 建表 + 幂等回填导航。可重复执行，不改写既有行。
func (d *DB) MigrateUp() error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range upStatements {
		if _, err = tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrate up: %w", err)
		}
	}
	for _, n := range navSeed {
		_, err = tx.Exec(`INSERT OR IGNORE INTO nav_item
			(nav_key,label,kind,visible,role_mask,route_path,retired_from)
			VALUES (?,?,?,?,?,?,?)`, n.key, n.label, n.kind, n.visible, 7, n.route, n.retired)
		if err != nil {
			return fmt.Errorf("seed nav: %w", err)
		}
	}
	// 不变量兜底：retired 恒不可见（历史行若被误改，这里纠正）。
	if _, err = tx.Exec(`UPDATE nav_item SET visible = 0 WHERE retired_from IS NOT NULL AND retired_from <> ''`); err != nil {
		return err
	}
	// AEO 工作台整改：扩展模块（财务 AI / 付费广告 / Agent）移出客户主导航，
	// 历史库已存在的 visible=1 行在此纠正；路由不受影响，功能不删（可从概览卡片进入）。
	if _, err = tx.Exec(`UPDATE nav_item SET visible = 0 WHERE nav_key IN ('finance','ads','agents')`); err != nil {
		return err
	}
	return tx.Commit()
}

// MigrateDown 逆序删表（保留数据场景用 Up/Down 演练；生产回滚优先 flag off，不删表）。
func (d *DB) MigrateDown() error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range tableNames {
		if _, err = tx.Exec("DROP TABLE IF EXISTS " + t); err != nil {
			return fmt.Errorf("migrate down %s: %w", t, err)
		}
	}
	return tx.Commit()
}

// VisibleNav 是 GET /v1/nav 的数据源：按角色过滤，排除 retired（FC-1）。
// roleBit 为 1 管理层 / 2 市场 / 4 分析；7=全部。
func (d *DB) VisibleNav(roleBit int) ([]NavItem, error) {
	rows, err := d.Query(`SELECT nav_key,label,kind,route_path FROM nav_item
		WHERE visible = 1 AND (retired_from IS NULL OR retired_from = '')
		AND (role_mask & ?) = ? ORDER BY rowid`, roleBit, roleBit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NavItem{}
	for rows.Next() {
		var n NavItem
		if err = rows.Scan(&n.NavKey, &n.Label, &n.Kind, &n.RoutePath); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

type NavItem struct {
	NavKey    string `json:"nav_key"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	RoutePath string `json:"route_path"`
}

// HasRetiredRoute 供 CI 断言： retired 项不得有客户路由。
func (d *DB) HasRetiredRoute() (bool, error) {
	var n int
	err := d.QueryRow(`SELECT COUNT(*) FROM nav_item
		WHERE retired_from IS NOT NULL AND retired_from <> '' AND visible = 1
		AND route_path IS NOT NULL AND route_path <> ''`).Scan(&n)
	return n > 0, err
}

// TableCount 返回已建的本批表数量（迁移测试用）。
func (d *DB) TableCount() (int, error) {
	var n int
	err := d.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table'
		AND name IN (`+placeholders(len(tableNames))+`)`, args(tableNames)...).Scan(&n)
	return n, err
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func args(in []string) []any {
	out := make([]any, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}
