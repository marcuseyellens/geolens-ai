package db

// 第六批（七款新品能力）12 张新表迁移。独立文件、独立函数，零修改第五批 migrate.go。
// 规格 §9 Up 顺序：citation_source → intent_gap → answer_share_snapshot →
// content_change_proposal → customer_agent → agent_instruction →
// agent_permission_grant → agent_account_isolation → commitment_ledger →
// agent_trace → agent_span → ai_ad_spot。
// SQLite 等价：JSONB→TEXT，BIGSERIAL→INTEGER PRIMARY KEY AUTOINCREMENT，
// TIMESTAMPTZ→TEXT，生成列（is_white_gap/share_pct）用普通列 + 应用层断言维护。
// 全部新表初始为空，不回填历史数据；flag 关闭时读取路径统一返回空集。

import (
	"fmt"

	"github.com/answerlens/answerlens/domain"
)

// sevenProductsTables 按 §9 依赖顺序用于 DOWN 逆序删表。
var sevenProductsTables = []string{
	"ai_ad_spot", "agent_span", "agent_trace", "commitment_ledger",
	"agent_account_isolation", "agent_permission_grant", "agent_instruction",
	"customer_agent", "content_change_proposal", "answer_share_snapshot",
	"intent_gap", "citation_source",
}

var sevenProductsUpStatements = []string{
	// NF-39/40 引用来源溯源 + ghost citation（DC-1/DC-2，DP-1a/1b）
	`CREATE TABLE IF NOT EXISTS citation_source (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		observation_id INTEGER NOT NULL,
		answer_ref TEXT NOT NULL,
		source_type TEXT NOT NULL,
		domain TEXT NOT NULL,
		url TEXT,
		verbatim_excerpt TEXT,
		citation_kind TEXT NOT NULL,
		read_at TEXT NOT NULL,
		created_at TEXT DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_citation_source_obs ON citation_source(observation_id)`,
	`CREATE INDEX IF NOT EXISTS idx_citation_source_answer ON citation_source(answer_ref)`,
	// NF-41 白空格/意图缺口（DP-2，DC-3）
	`CREATE TABLE IF NOT EXISTS intent_gap (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		brand_id INTEGER NOT NULL,
		intent_cluster TEXT NOT NULL,
		demand_signal REAL,
		claimant_brands TEXT,
		is_white_gap INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'open',
		converted_task_id INTEGER,
		detected_at TEXT NOT NULL,
		UNIQUE(brand_id, intent_cluster, detected_at)
	)`,
	// NF-44 answer share 份额快照（份额，非综合分，INV-10）
	`CREATE TABLE IF NOT EXISTS answer_share_snapshot (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		brand_id INTEGER NOT NULL,
		intent_cluster TEXT NOT NULL,
		model TEXT NOT NULL,
		region TEXT,
		share_numerator INTEGER NOT NULL,
		share_denominator INTEGER NOT NULL,
		share_pct REAL,
		competitor_shares TEXT,
		measured_at TEXT NOT NULL
	)`,
	// NF-47 内容变更提议（DP-3，DC-5，INV-2）
	`CREATE TABLE IF NOT EXISTS content_change_proposal (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		brand_id INTEGER NOT NULL,
		change_type TEXT NOT NULL,
		proposed_by TEXT NOT NULL,
		payload TEXT NOT NULL,
		priority_rank INTEGER,
		status TEXT NOT NULL DEFAULT 'proposed',
		approved_by TEXT,
		decided_at TEXT,
		reason TEXT,
		published_at TEXT,
		created_at TEXT DEFAULT (datetime('now')),
		CHECK (status <> 'published' OR approved_by IS NOT NULL)
	)`,
	// NF-48/49 客户面 Agent 定义（DP-4，DC-6）
	`CREATE TABLE IF NOT EXISTS customer_agent (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		brand_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		role_type TEXT NOT NULL,
		description TEXT NOT NULL,
		approval_step TEXT NOT NULL DEFAULT 'described',
		source_binding TEXT NOT NULL,
		orchestrator_config TEXT,
		live_at TEXT,
		created_at TEXT DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS agent_instruction (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL REFERENCES customer_agent(id),
		seq INTEGER NOT NULL,
		text TEXT NOT NULL,
		approved INTEGER NOT NULL DEFAULT 0,
		edited_by_human INTEGER NOT NULL DEFAULT 0,
		UNIQUE(agent_id, seq)
	)`,
	// NF-50 权限逐次批准（DP-5，DC-7，INV-4）
	`CREATE TABLE IF NOT EXISTS agent_permission_grant (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		scope TEXT NOT NULL,
		resource_ref TEXT NOT NULL,
		decision TEXT NOT NULL,
		decided_by TEXT NOT NULL,
		decided_at TEXT NOT NULL,
		CHECK (scope NOT IN ('send','delete') OR decision = 'forbidden')
	)`,
	// NF-50 账户隐藏（INV-5）
	`CREATE TABLE IF NOT EXISTS agent_account_isolation (
		agent_id INTEGER NOT NULL,
		account_ref TEXT NOT NULL,
		hidden_from_agents INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT DEFAULT (datetime('now')),
		PRIMARY KEY(agent_id, account_ref)
	)`,
	// NF-51 承诺账本（DP-6，DC-7 延伸，read-only by design）
	`CREATE TABLE IF NOT EXISTS commitment_ledger (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		brand_id INTEGER NOT NULL,
		source_ref TEXT NOT NULL,
		commitment_text TEXT NOT NULL,
		owner TEXT,
		due_at TEXT,
		cluster_key TEXT,
		status TEXT NOT NULL DEFAULT 'extracted',
		settled_at TEXT,
		extracted_at TEXT NOT NULL
	)`,
	// NF-52 trace / span / 双分母（DP-7，DC-8，INV-6/8）
	`CREATE TABLE IF NOT EXISTS agent_trace (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		execution_id TEXT NOT NULL,
		agent_id INTEGER NOT NULL,
		prompt_version TEXT NOT NULL,
		model_version TEXT NOT NULL,
		task_success INTEGER,
		started_at TEXT NOT NULL,
		ended_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_trace_exec ON agent_trace(execution_id)`,
	`CREATE TABLE IF NOT EXISTS agent_span (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		trace_id INTEGER NOT NULL REFERENCES agent_trace(id),
		seq INTEGER NOT NULL,
		span_type TEXT NOT NULL,
		tool_name TEXT,
		input_ref TEXT,
		output_ref TEXT,
		latency_ms INTEGER,
		token_count INTEGER,
		cost REAL,
		status TEXT NOT NULL,
		retry_of INTEGER REFERENCES agent_span(id),
		error TEXT,
		evidence_ref TEXT,
		waiting_since TEXT,
		decided_by TEXT,
		decision TEXT,
		changed_after TEXT,
		UNIQUE(trace_id, seq)
	)`,
	// NF-43 AI 广告位（每条带 answer_ref，DC-1）
	`CREATE TABLE IF NOT EXISTS ai_ad_spot (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		brand_id INTEGER NOT NULL,
		competitor_ref TEXT NOT NULL,
		model TEXT NOT NULL,
		answer_ref TEXT NOT NULL,
		ad_claim TEXT,
		detected_at TEXT NOT NULL,
		UNIQUE(competitor_ref, answer_ref)
	)`,
}

// MigrateUpSeven 建 12 张新表 + 索引，可重复执行。
func (d *DB) MigrateUpSeven() error {
	for _, stmt := range sevenProductsUpStatements {
		if _, err := d.Exec(stmt); err != nil {
			return fmt.Errorf("migrate seven up: %w", err)
		}
	}
	return nil
}

// MigrateDownSeven 逆序删表（回滚前须确认 flag 关闭、无在途写入）。
func (d *DB) MigrateDownSeven() error {
	for _, table := range sevenProductsTables {
		if _, err := d.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("migrate seven down %s: %w", table, err)
		}
	}
	return nil
}

// SevenFeatureFlag 返回本批 flag 名（供网关层与测试引用）。
func SevenFeatureFlag() string { return domain.SevenProductsFeatureFlag }
