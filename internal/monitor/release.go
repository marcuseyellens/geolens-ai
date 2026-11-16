package monitor

// WP-8.3「发布检查清单」：状态由传入证据计算，不硬编码通过结果。

type ReleaseEvidence struct {
	LegalPagesReady            bool
	MethodologyNotesPresent    bool
	BootstrapFiles             []string
	SelfChecksPassed           bool
	AcceptanceChecklistGreen   bool
	FullChainRegressionPassed  bool
	BudgetCircuitTested        bool
	CostReconciled             bool
	MCPDocsPublished           bool
	MCPToolsReadOnly           bool
	MonitoringAlertsConfigured bool
	RollbackPlan               string
	RateLimitGrayRollout       string
}

type ReleaseCheckItem struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"` // pass / fail
	Evidence []string `json:"evidence"`
}

// ReleaseChecklist 生成上线检查清单。每项只有拿到证据才标 pass。
func ReleaseChecklist(e ReleaseEvidence) []ReleaseCheckItem {
	items := []ReleaseCheckItem{}
	add := func(id, title string, ok bool, ev ...string) {
		status := "fail"
		if ok {
			status = "pass"
		}
		items = append(items, ReleaseCheckItem{ID: id, Title: title, Status: status, Evidence: ev})
	}
	add("R1", "隐私政策与服务条款页面可访问", e.LegalPagesReady, "/privacy", "/terms")
	add("R2", "独立口径声明与非因果表述已展示", e.MethodologyNotesPresent, "methodology_notes")
	add("R3", "AEO 自举根目录文件真实返回", len(e.BootstrapFiles) >= 3, e.BootstrapFiles...)
	add("R4", "四项发布自检全绿", e.SelfChecksPassed, "/api/v1/self-checks")
	add("R5", "10 条验收清单全绿", e.AcceptanceChecklistGreen, "/api/v1/acceptance-checklist")
	add("R6", "L0→L3 全链路脚本化回归通过", e.FullChainRegressionPassed, "go test ./internal/regression/...")
	add("R7", "预算熔断与重试不可绕过已验证", e.BudgetCircuitTested, "TestBudgetCircuitCannotBeBypassedByRetry")
	add("R8", "按秒成本与配额对账无差额", e.CostReconciled, "aeo.ReconcileCost")
	add("R9", "MCP 两工具文档已发布", e.MCPDocsPublished, "/mcp docs")
	add("R10", "Agent token 只读鉴权已验证", e.MCPToolsReadOnly, "TestInvalidTokenRejectedAndReadOnly")
	add("R11", "监控五块看板与阈值告警已配置", e.MonitoringAlertsConfigured, "/api/v1/monitor")
	add("R12", "回滚方案已记录", e.RollbackPlan != "", e.RollbackPlan)
	add("R13", "限流灰度方案已记录", e.RateLimitGrayRollout != "", e.RateLimitGrayRollout)
	return items
}

func AllReleaseChecksPassed(items []ReleaseCheckItem) bool {
	for _, it := range items {
		if it.Status != "pass" {
			return false
		}
	}
	return len(items) == 13
}
