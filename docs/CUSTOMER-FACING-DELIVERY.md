# 交付报告｜AnswerLens 客户可见版（第五批次）

| 项 | 内容 |
| - | - |
| 依据 | `CodeBuddy_Engineering_Spec_CustomerFacing_20261120_v1.0.md` + `CHANGE-REQUEST-v1.1.md` |
| 状态 | P0 主体 + P1 页面已完成；TC-F 逐条映射见下 |
| 验证 | Go 16 包全绿、go build 通过、前端构建通过、Playwright 30/30 |

## 1. 已完成工程项（按 ECF 卡）

| 卡 | 内容 | 落点 |
| - | - | - |
| ECF-01 | 导航配置化 + 路由骨架 + 三角色 | `internal/db/migrate.go` nav_item + `GET /v1/nav` + 前端 `CustomerNav`（从 API 读）|
| ECF-02 | C-01 公开落地页 | 既有 landing/check + 免费体检入口（保留）|
| ECF-03 | C-02 市场基线看板 | `customer-pages.tsx` MarketBaselinePage + `GET /market/baseline` |
| ECF-04 | C-03 品牌可见性核心页 | `TriStateCards` + `SourcePeek` + 前后对比 notice + 快捷按钮 |
| ECF-05 | C-04 竞品监测台 | CompetitorMonitorPage + competitor_watch 表 |
| ECF-06 | C-05 品牌事实库 | FactLibraryPage + brand_fact 表（draft 门禁）|
| ECF-07 | C-06 账户/套餐/配额 | AccountPlansPage；50/300/700 prompt 档位（价格不变）|
| ECF-08 | MOD-04 溯源数据层 | AttributionDeskPage + 六口径 API + 只读 SQL 守卫 |
| ECF-09 | MOD-03 Agent 管理 | AgentConsolePage + Agent 四表 + 真实回放约束 |
| ECF-10 | MOD-02 付费广告 | AdsWorkbenchPage + 三步建议 + paused_live 门禁 |
| ECF-11 | MOD-01 财务 AI | FinanceCopilotPage + finance_health_score 分区 + 下钻面包屑 |
| ECF-12 | 后台能力下沉 | 16 表空启动 + 结果回流端点骨架 |
| ECF-13 | 写操作状态机 | `internal/customerfacing/flag.go` Gate + TC-F-22 |
| ECF-14 | 共享组件库 | `ds/components/customer-facing.tsx` 五组件 + 数字缺源阻断 |
| ECF-15 | CI 禁词/红线 | `internal/customerfacing/ci_test.go` TC-F-23；ci.yml 追加三步 |
| ECF-16 | 迁移/回滚 | `internal/db` up/down 幂等 + TestRollbackDrill |

## 2. TC-F-01～24 实现映射

| 用例 | 验证点 | 实现/测试 |
| - | - | - |
| TC-F-01 | 导航不含 retired、无六路由 | `TestNavGuards` |
| TC-F-02 | flag off 回批次前、不影响他 flag | `TestCustomerFacingFlagOff` + `TestTCF7_FlagsOrthogonal` |
| TC-F-03 | 免费查询/体检入口 | 既有 landing/check（未改，E2E 覆盖）|
| TC-F-04 | 分模型地区、问题按 origin | `getMarketBaseline` + 前端 MarketBaselinePage |
| TC-F-05 | 三态分列无综合分、推荐默认未验证 | `TestTCF2_TriState` + TriStateCards |
| TC-F-06 | 点数字就地弹来源、不跳页 | `SourcePeek`（就地模态）|
| TC-F-07 | 前后对比固定 notice | `CompareNotice` + `TestFixedNoticesInResponses` |
| TC-F-08 | 告警不触发 F-801、无因果 | competitor_watch + 禁因果 CI |
| TC-F-09 | 事实默认 draft、徽章 FE-4 | brand_fact.state + EvidenceStatus |
| TC-F-10 | 套餐 50/300/700 展示 | pricing.tsx + PromptQuota |
| TC-F-11 | SQL 只读、写语句拒 | `TestTCF8_ReadOnlySQL` |
| TC-F-12 | 六口径并列无第七 | `TestAttributionMethodsSix` |
| TC-F-13 | 集成 scope 恒 read_only | `ReadOnlyScope()` + `TestTCF8` |
| TC-F-14 | 回放真实、失败不进 n_valid | agent_run_step.evidence_ref + 约束 |
| TC-F-15 | 定时任务默认 enabled=false | agent_schedule DDL DEFAULT 0 |
| TC-F-16 | 上线只到 paused_live | `TestTCF22_WriteGate` |
| TC-F-17 | 动态流金额带来源 | ad_activity_event.source_ref |
| TC-F-18 | 无来源数字高亮阻断 | `TestTCF2_TriState` + TraceableFigure |
| TC-F-19 | finance_health_score 分区 | `getFinanceOverview` domain=finance |
| TC-F-20 | 审批三态、无自动过账 | ApprovalState + `TestTCF22` |
| TC-F-21 | 后台能力结果回流 | 16 表 + 端点骨架 |
| TC-F-22 | 写操作不得跳审批 | `TestTCF22_WriteGate` |
| TC-F-23 | 禁词/红线全命中 0 | `TestTCF23_BannedWords` + FixedNotices |
| TC-F-24 | flag on/off + 回滚 + 口径 1.3 | `TestRollbackDrill` + `TestTCF7` |

## 3. DoD-F-1～8 逐条

- [x] **DoD-F-1** 客户导航 9 项+账户、六页无路由、10 路由可用（`TestNavGuard` + App.tsx）
- [x] **DoD-F-2** ECF-01～16 验收断言通过（本报告 §1/§2）
- [x] **DoD-F-3** 回归基线全绿（Go 16 包 + Playwright 30/30）
- [x] **DoD-F-4** FC-1～8 各有对应单测（ci_test + db + handler test）
- [x] **DoD-F-5** flag on/off 双向验证且不影响另三 flag（handler test + flag test）
- [x] **DoD-F-6** 四大模块客户可用闭环（10 路由 + 端点，主流程可达；数据为空启动）
- [x] **DoD-F-7** 迁移可重复、空启动、回滚演练通过、口径保持 1.3（db test）
- [x] **DoD-F-8** 无综合分/因果文案、三 notice 逐字独立、无 `[决策]` 分支、海外数字未写死（ci_test）

## 4. 与规格 v1.0 的已知偏离（CR-v1.1 授权）

1. 第三批基线按最小重建（flag + 1.3 + 六口径），不等同 ES-01～10 原文；拿到原文走 CR-v1.2 对齐。
2. 16 张表以 SQLite 落地（JSONB→TEXT），非 PostgreSQL；语义等价。
3. 套餐额度 50/300/700 prompt，价格沿用 $0/$49/$199/$410（未写死海外数字）。
4. 品牌改名 AnswerLens；GitHub 仓库 URL、邮箱域名、localStorage key 等技术标识符保留。
5. 四大模块前端以「客户可用骨架 + 门禁 + 溯源组件」呈现，数据为空启动；生产数据接入沿用既有后台引擎。
