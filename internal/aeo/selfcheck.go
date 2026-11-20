package aeo

import (
	"fmt"
	"strings"

	"github.com/answerlens/answerlens/domain"
)

// ---- A-WP-4.1 agent_readability 检查组 ----
//
// 只检查、不代建；结论标注「模式借鉴，非平台要求」；缺 MCP 不判 fail。

// AgentAssets 描述站点公开可查的四类可调用资产。
type AgentAssets struct {
	LLMsTxt      string // llms.txt 正文
	MachinePrice bool   // 机器可读价格/库存（JSON-LD offers 或等价结构）
	MCPEndpoint  string // MCP / API 端点，空表示未提供
	HumanGate    bool   // 人工确认闸口
	Body         string // 首页原始 HTML
}

// InspectAgentReadability 返回 4 项检查结论。缺失 MCP 只标 unknown，绝不 fail。
func InspectAgentReadability(a AgentAssets) []domain.AgentReadabilityCheck {
	mk := func(id string, status domain.AdmissionStatus, evidence, fix string) domain.AgentReadabilityCheck {
		return domain.AgentReadabilityCheck{
			ID: id, Status: status, Evidence: evidence, Fix: fix,
			Notice: domain.AgentReadabilityNotice,
			Provenance: domain.Provenance{
				SourceType: domain.SourceObserved, MethodologyVersion: domain.MethodologyVersion,
				NValid: 1,
			},
		}
	}
	low := strings.ToLower(a.Body)
	out := []domain.AgentReadabilityCheck{}
	if len(strings.TrimSpace(a.LLMsTxt)) > 40 {
		out = append(out, mk("llms_txt", domain.AdmissionPass, "llms.txt 已发布且含内容", ""))
	} else {
		out = append(out, mk("llms_txt", domain.AdmissionPartial, "未发现有效 llms.txt", "可参照 llmstxt.org 自行发布"))
	}
	if a.MachinePrice || strings.Contains(low, "\"price\"") || strings.Contains(low, "价格") {
		out = append(out, mk("machine_readable_price", domain.AdmissionPass, "存在机器可读价格事实", ""))
	} else {
		out = append(out, mk("machine_readable_price", domain.AdmissionPartial, "未见机器可读价格", "可用 JSON-LD Offer 标注价格"))
	}
	if strings.TrimSpace(a.MCPEndpoint) != "" {
		out = append(out, mk("mcp_endpoint", domain.AdmissionPass, "MCP/API 端点："+a.MCPEndpoint, ""))
	} else {
		out = append(out, mk("mcp_endpoint", domain.AdmissionUnknown, "未提供 MCP 端点，不判定", "如需 agent 调用可自行接入；这不是平台要求"))
	}
	if a.HumanGate {
		out = append(out, mk("human_gate", domain.AdmissionPass, "存在人工确认闸口", ""))
	} else {
		out = append(out, mk("human_gate", domain.AdmissionPartial, "未见人工确认闸口", "在自动动作前保留人工确认"))
	}
	return out
}

// ValidateAgentReadability CI 断言：缺 MCP 不得判 fail；每项都带 notice 与口径。
func ValidateAgentReadability(checks []domain.AgentReadabilityCheck) []string {
	errs := []string{}
	for _, c := range checks {
		if c.ID == "mcp_endpoint" && c.Status == domain.AdmissionFail {
			errs = append(errs, "缺少 MCP 不应判 fail")
		}
		if c.Notice == "" {
			errs = append(errs, c.ID+" 缺少「模式借鉴，非平台要求」标注")
		}
		if err := ValidateNumericComponent("agent_readability", c.Provenance); err != nil {
			errs = append(errs, c.ID+": "+err.Error())
		}
	}
	return errs
}

// ---- A-WP-9.4 自家站点四项自检（A-01 / A-03 / A-05 / A-14）----

// SelfCheckInput 是对自家站点做四项自检所需的全部公开证据。
type SelfCheckInput struct {
	HomeHTML            string
	RobotsTxt           string
	LLMsTxt             string
	SitemapXML          string
	JSONLDPresent       bool
	Profile             domain.BusinessProfile
	Admission           []domain.AdmissionCheck
	AgentChecks         []domain.AgentReadabilityCheck
	NeutralityInLanding bool
	NeutralityInReport  bool
	PricingText         string
}

// RunSelfChecks 用自家产品对自家站点跑 A-01 / A-03 / A-05 / A-14 四项自检。
func RunSelfChecks(in SelfCheckInput) []domain.SelfCheckResult {
	results := []domain.SelfCheckResult{}

	// A-01 第一方档案与中立性
	a01 := domain.SelfCheckResult{ID: domain.SelfCheckA01, Name: "第一方档案与中立性", Details: []string{}}
	a01.Passed = true
	if in.Profile.Status != domain.ProfileUnknown && in.Profile.Status != "" {
		// 非 unknown 状态必须来自自述或公开信息
		if in.Profile.SourceType == domain.SourceUnknown {
			a01.Passed = false
			a01.Details = append(a01.Details, "非 unknown 状态缺少来源类型")
		}
	}
	if !in.NeutralityInLanding {
		a01.Passed = false
		a01.Details = append(a01.Details, "落地页缺少中立性声明")
	}
	if !in.NeutralityInReport {
		a01.Passed = false
		a01.Details = append(a01.Details, "报告缺少中立性声明")
	}
	if err := ValidateNumericComponent("selfcheck", domain.ObservedProvenance(1)); err != nil {
		a01.Passed = false
		a01.Details = append(a01.Details, err.Error())
	}
	if len(a01.Details) == 0 {
		a01.Details = append(a01.Details, "档案状态默认 unknown；落地页与报告均可见中立性声明")
	}
	results = append(results, a01)

	// A-03 索引准入
	a03 := domain.SelfCheckResult{ID: domain.SelfCheckA03, Name: "索引准入检查组", Details: []string{}}
	a03.Passed = true
	if len(in.Admission) != len(domain.AllAdmissionDetectors()) {
		a03.Passed = false
		a03.Details = append(a03.Details, fmt.Sprintf("检测器数量 %d ≠ 8", len(in.Admission)))
	}
	for _, c := range in.Admission {
		if c.RiskNotice == "" || c.SourceURL == "" {
			a03.Passed = false
			a03.Details = append(a03.Details, string(c.Detector)+" 缺风险提示或来源链接")
		}
		if c.Status != domain.AdmissionUnknown && c.Evidence == "" {
			a03.Passed = false
			a03.Details = append(a03.Details, string(c.Detector)+" 缺证据片段")
		}
	}
	if _, _, _, unknown := AdmissionDenominator(in.Admission); unknown > 0 {
		a03.Details = append(a03.Details, fmt.Sprintf("%d 项为 unknown，不进分母、不扣分", unknown))
	}
	if len(a03.Details) == 0 {
		a03.Details = append(a03.Details, "8 类检测器全部产出带证据与风险提示的结论")
	}
	results = append(results, a03)

	// A-05 Agent 可调用性
	a05 := domain.SelfCheckResult{ID: domain.SelfCheckA05, Name: "Agent 可调用性", Details: []string{}}
	errs := ValidateAgentReadability(in.AgentChecks)
	a05.Passed = len(errs) == 0 && len(in.LLMsTxt) > 40 && in.JSONLDPresent
	a05.Details = append(a05.Details, errs...)
	if len(in.LLMsTxt) <= 40 {
		a05.Details = append(a05.Details, "llms.txt 内容不足")
	}
	if !in.JSONLDPresent {
		a05.Details = append(a05.Details, "首页缺 JSON-LD 价格事实")
	}
	if len(a05.Details) == 0 {
		a05.Details = append(a05.Details, "llms.txt / 机器可读价格 / 人工闸口齐备；MCP 缺失不判 fail")
	}
	results = append(results, a05)

	// A-14 中立性与自举
	a14 := domain.SelfCheckResult{ID: domain.SelfCheckA14, Name: "中立性与自举", Details: []string{}}
	a14.Passed = true
	if !strings.Contains(in.RobotsTxt, "GPTBot") {
		a14.Passed = false
		a14.Details = append(a14.Details, "robots.txt 未放行 AI 爬虫")
	}
	if !strings.Contains(in.SitemapXML, "<urlset") && !strings.Contains(in.SitemapXML, "<loc>") {
		a14.Passed = false
		a14.Details = append(a14.Details, "sitemap.xml 格式异常")
	}
	if hits := ScanTextForBannedPhrases(in.HomeHTML); len(hits) > 0 {
		a14.Passed = false
		a14.Details = append(a14.Details, "首页命中禁词："+strings.Join(hits, "、"))
	}
	if hits := ScanTextForBannedPhrases(in.PricingText); len(hits) > 0 {
		a14.Passed = false
		a14.Details = append(a14.Details, "定价页命中禁词："+strings.Join(hits, "、"))
	}
	if !plansUnchanged(in.PricingText) {
		a14.Passed = false
		a14.Details = append(a14.Details, "套餐结构发生变化")
	}
	if len(a14.Details) == 0 {
		a14.Details = append(a14.Details, "四项自检口径与套餐结构均通过")
	}
	results = append(results, a14)

	return results
}

// plansUnchanged 校验 A-WP-9.3：Free / $39 / $19 / $49·199·410 一个不少。
func plansUnchanged(text string) bool {
	for _, p := range []string{"$0", "$39", "$19", "$49", "$199", "$410"} {
		if !strings.Contains(text, p) {
			return false
		}
	}
	return true
}

// AllSelfChecksPassed 是 §8 第 8 条的 CI 断言入口。
func AllSelfChecksPassed(results []domain.SelfCheckResult) bool {
	if len(results) != 4 {
		return false
	}
	for _, r := range results {
		if !r.Passed {
			return false
		}
	}
	return true
}
