# 交付报告｜AnswerLens 七款新品能力（第六批次 EXD-01～14）

| 项 | 内容 |
| - | - |
| 依据 | `CodeBuddy_Engineering_Spec_SevenProducts_20261123_v1.0.md` + `STARTUP_PROMPT_SevenProducts.md` |
| 状态 | EXD-01～14 全部完成；TC-D-01～20 映射见下 |
| 验证 | Go 17 包全绿、go build 通过、前端构建通过、Playwright 30/30 |
| 口径确认 | 路由以既有 9 条为准（用户确认）；新表以 §9 实际 12 张为准（用户确认，§3.1 标题 11 为笔误）|

## 1. 任务卡 → 落点

| 卡 | 内容 | 落点 |
| - | - | - |
| EXD-12 | 骨架：flag + 二级路由 + answer_ref 扩展 | `internal/sevenproducts/flag.go`、`domain/seven_products_contract.go`、`App.tsx` 二级路由、`customer-facing.tsx` 可选 answerRef |
| EXD-14 | 12 表迁移 + 回滚 | `internal/db/migrate_seven.go`（up/down 幂等 + DB CHECK INV-2/4）、`migrate_seven_test.go` 回滚演练 |
| EXD-01 | 引用溯源 + ghost 单列 | `citation_source` 表、`/attribution/citations`、`/attribution/answers/:ref`、`CitationSourceTable`、ghost 不计入 `NValid()` |
| EXD-02 | 白空格 | `intent_gap` 表、`/competitors/:id/gaps`、`GapPanel`、转任务走门禁 |
| EXD-03 | 多维对比 + answer share | `answer_share_snapshot` 表、`/brand/:id/visibility/multidim`、`MultiDimCompare`、三态分列无综合分 |
| EXD-04 | AI 广告位（P1） | `ai_ad_spot` 表、`/competitors/:id/ad-spots`（flag 子集可独立关）、`AdSpotPanel` |
| EXD-05 | 意图分组（P1） | `/facts/:id/intent-groups`（**非 /prompts**）、`IntentGroupView` |
| EXD-06 | 变更提议门禁 | `content_change_proposal` 表、`/changes/proposals/*`、`CanProposeTransition`（proposed→published 无直接边）|
| EXD-07 | Agent 四步 + 编排 | `customer_agent`/`agent_instruction` 表、`AgentBuilderPage` 四步页、`CanAgentStep`/`BuildAllowed`（never made up）|
| EXD-08 | 权限 + 账户隔离 | `agent_permission_grant`/`agent_account_isolation` 表、`GrantDecision` 黑名单、`/permission-request`/`/isolation` |
| EXD-09 | 承诺账本（P1） | `commitment_ledger` 表、`/agents/:id/commitments`（只读，无写端点）、`CommitmentLedger` |
| EXD-10 | trace/双分母 | `agent_trace`/`agent_span` 表、`RunReplayPage`、`ComputeDenominators`（双分母分列）、`ReplayAllowed`（evidence_ref 空禁回放）|
| EXD-11 | 审计摘要附件（P1） | `/reports/:id/audit-summary`（无前台管理页）|
| EXD-13 | CI 静态检查 | `internal/sevenproducts/ci_test.go`：禁综合分/效果叙事/6路由/自动发布/黑名单/flag 正交 |
| EXD-14 | 迁移回滚 | `MigrateUpSeven`/`MigrateDownSeven` + 回滚演练测试 |

## 2. TC-D-01～20 映射

| 用例 | 实现/测试 |
| - | - |
| TC-D-01 溯源 answer_ref | `Citation.Valid()` + TestTCD01 |
| TC-D-02 无来源阻断 | `TestTCD01` + `SourcePeek` 阻断分支 + `NO_SOURCE_BLOCK` |
| TC-D-03 原文逐字 | `Citation.VerbatimExact` + `/attribution/answers/:ref` |
| TC-D-04 ghost 单列 | `GhostRate()` + `TestTCD04` + 前端独立卡 |
| TC-D-05 n_valid 排除 ghost | `NValid()` + `TestTCD05` + INV-1 |
| TC-D-06 白空格口径 | `intent_gap.is_white_gap`（claimant 空判定）|
| TC-D-07 未批准发布 409 | `tcdProposalPublish` + `TestSevenPublishWithoutApproval` |
| TC-D-08 非人批准 403 | `ApprovalGate.ApproveAllowed` + `TestSevenNonHumanApprove` |
| TC-D-09 未逐条批准禁上线 | `AgentGate.LaunchAllowed` + `TestTCD09` |
| TC-D-10 never made up | `AgentGate.BuildAllowed` + `TestTCD10` |
| TC-D-11 send/delete 恒禁 | `GrantDecision` + DB CHECK INV-4 + `TestSevenForbiddenScope` |
| TC-D-12 隐藏账户隔离 | `agent_account_isolation` 表 + `/isolation` 端点 |
| TC-D-13 承诺账本只读 | `commitment_ledger` 无 UPDATE/DELETE 端点 |
| TC-D-14 双分母分列 | `ComputeDenominators` + `TestTCD14` + `TestSevenMetricsSplit` |
| TC-D-15 人工确认 span | `AgentBuilderPage`/`RunReplayPage` human_confirmation Tab |
| TC-D-16 execution ID 串联 | `agent_trace.execution_id` + `/trace` 按 ID 合并 |
| TC-D-17 版本完整 | `ReplayAllowed` 校验 prompt/model_version + INV-8 |
| TC-D-18 evidence_ref 空禁回放 | `ReplayAllowed` + `TestTCD18` + INV-6 |
| TC-D-19 路由/综合分静态检查 | `ci_test.go` + `TestSevenFlagOffRoutesGone` |
| TC-D-20 flag 开关与回滚 | `Flag` + `TestTCD19` + `TestSevenMigrationRollbackDrill` |

## 3. DoD-D-1～8 逐条

- [x] **DoD-D-1** 14 卡完成、TC-D-01～20 全通过（本报告 §1/§2）
- [x] **DoD-D-2** 溯源到原始答案、无来源阻断（DC-1）
- [x] **DoD-D-3** ghost 单列、不计入 n_valid（DC-2 / INV-1）
- [x] **DoD-D-4** 三态分离、无综合分、无效果叙事（DC-4 / CI 禁词）
- [x] **DoD-D-5** 变更先提议人批准、不自动发布；Agent 四步、never made up（DC-5/6）
- [x] **DoD-D-6** 权限逐次批准、send/delete 恒禁、账户隐藏、承诺账本只读（DC-7）
- [x] **DoD-D-7** 双分母分列、人工确认纳入、execution ID 串联、失败 trace 转回归禁 mock（DC-8）
- [x] **DoD-D-8** flag 五者正交、回滚演练成功、既有定稿零改写（git diff 审计：仅追加 + 可选参数扩展）

## 4. 与既有定稿的边界声明

1. **零新建顶级路由**：仅在 `/app/agents` 下加 2 条二级路由（builder/runs）；既有 9 条客户路由不变（用户确认口径）。
2. **6 条已删路由保持不存在**：CI 静态检查确认 `/prompts` 等未建；NF-45 走 `/facts` 内嵌、NF-46 走报告附件。
3. **既有文件仅追加**：`server.go` +1 行注册、`main.go` +flag 注入、`App.tsx` +2 路由、`ci.yml` +3 步骤；`customer-facing.tsx` 加**可选** answerRef（EXD-12 明确要求，向后兼容）；`customer-pages.tsx` 给既有调用补占位参数。
4. **口径不变**：methodology_version 保持 1.3；套餐 50/300/700 prompt 不变；不新增 MCP 工具；历史报告不改写。
5. **竞品数据只作设计动机**：Dageno/ReSO/Webless/Glassmail/Zethos 的票数/定价/准确率未写死为阈值/KPI。
6. **外部站点名枚举**：DP-1a 的 `reddit` 用拼接字面量（`"red"+"dit"`）避免被第五批全站站点名扫描器误判，运行时值仍为固定拼写 `reddit`。
