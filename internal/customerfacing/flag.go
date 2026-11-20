// Package customerfacing 承载第五批（ECF-01～16）的横切执行层：
// 独立 feature flag、写操作状态机门禁（ECF-13）、只读 SQL 守卫（FC-8）、
// CI 禁词与不变式断言（ECF-15）。与 delta/competitor/social 三 flag 正交。
//
// 本包不改 methodology_version（保持 1.3）、不新增 MCP 工具/Webhook、
// 不做因果文案、不出任何聚合可见度分（FC-2/FC-5）。
package customerfacing

import (
	"regexp"
	"strings"
	"sync"

	"github.com/answerlens/answerlens/domain"
)

// Flag 是本批总开关。默认 on；GEOLENS_FLAG_AEO_CUSTOMER_FACING_20261120=off
// 秒级关闭，客户可见层回到批次前，不影响另三个 flag。
type Flag struct {
	mu      sync.RWMutex
	enabled bool
}

func NewFlag(enabled bool) *Flag { return &Flag{enabled: enabled} }

func (f *Flag) Enabled() bool {
	if f == nil {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled
}

func (f *Flag) SetEnabled(v bool) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.enabled = v
	f.mu.Unlock()
}

func (f *Flag) Name() string { return domain.CustomerFacingFeatureFlag }

// ---- ECF-13 写操作状态机门禁（FC-4）----

// Gate 拒绝任何跳过审批直达终态的写操作。
type Gate struct{}

// CanTransition 统一状态机。广告 approved 后只到 paused_live，禁止直接 live。
func (Gate) CanTransition(current, next domain.AdCampaignState) bool {
	return domain.CanAdvance(current, next)
}

// FinancePostingAllowed 财务过账须显式批准，禁止自动。
func (Gate) FinancePostingAllowed(approved bool) bool { return approved }

// ---- FC-8 只读 SQL 守卫 ----

var writeSQL = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|REPLACE|PRAGMA)\b`)

// IsReadOnlySQL 判断生成 SQL 是否为只读语句。含写关键字即拒绝执行。
func IsReadOnlySQL(sql string) bool {
	s := strings.TrimSpace(sql)
	if s == "" {
		return false
	}
	// 多语句（分号后还有内容）一律拒绝，防止 ; DROP 注入。
	if strings.Contains(strings.TrimSuffix(s, ";"), ";") {
		return false
	}
	return !writeSQL.MatchString(s)
}

// ReadOnlyScope 恒为 read_only（FE-7）。
func ReadOnlyScope() domain.IntegrationScope { return domain.IntegrationReadOnly }
