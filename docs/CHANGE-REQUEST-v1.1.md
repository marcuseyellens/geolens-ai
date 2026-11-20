# 变更单 CR-v1.1｜客户可见版（第五批次）开工授权

| 项 | 内容 |
| --- | --- |
| 变更单号 | CR-v1.1 |
| 日期 | 2026-11-20 |
| 状态 | **已批准 · 执行中** |
| 依据 | `CodeBuddy_Engineering_Spec_CustomerFacing_20261120_v1.0.md`（v1.0，84 人日，ECF-01～16） |
| 发起人 | 产品负责人（用户 2026-11-20 指令） |
| 冲突优先级 | D-1～D-6 / EX-01～14 / EC-01～08 / ES-01～10 定稿 > 本变更单 > 客户可见版 PM 文档 |

## 1. 四项决策（用户已确认）

| # | 议题 | 决策 | 影响 |
| - | --- | --- | --- |
| 1 | 工程范围 | 主平台 `geolens` 优先，移动端 / 插件次之 | 本单只改主平台；三端产物在阶段 7 重新构建 |
| 2 | 第三批基线 | **按最小基线重建**（不等待缺失的 ES-01～10 原文） | 仅建 `aeo_social_signals` flag、`methodology_version=1.3`、`source_profile_by_model` 六口径；不等同于原 ES-01～10 定稿，拿到原文后另行对齐（走 CR-v1.2） |
| 3 | 数据层 | **引入 SQLite（`modernc.org/sqlite`，纯 Go）+ 迁移体系** | 16 张表按规格 DDL 等价落地（JSONB→TEXT，BIGINT→INTEGER，TIMESTAMP→TEXT），带 `workspace_id` 多租户、可重复执行迁移、空启动、DOWN 回滚 |
| 4 | 命名与套餐 | **品牌改名 AnswerLens；套餐额度改 50/300/700 prompt** | 价格沿用现价（$0 / $49 / $199 / $410），不改价、不写死任何海外产品数字 |

## 2. 关键假设（消除歧义，不猜默认值）

- **套餐价格**：用户回复「价格 X/Y/Z」为占位；按硬约束「不改价格」，取现有 `$49 / $199 / $410` 分别对应 `50 / 300 / 700 prompt`；Free 仍为每日 5 次。任何海外产品自述数字均不作参数。
- **口径版本**：第三批最小基线把 `methodology_version` 从 `1.2 → 1.3`；第五批保持 `1.3`，历史 1.0～1.2 报告不改写。
- **模块路径**：Go module 路径由 `github.com/geolens/geolens` 改为 `github.com/answerlens/answerlens`，全部 import 同步替换；GitHub 仓库 URL 保留 `geolens-ai`（避免已发布 Release 与外链失效），仓库描述与 README 品牌名改为 AnswerLens。
- **基线文档缺失**：`PM_AnswerLens_CustomerFacing_*` 等四份原文缺失，按启动提示词「以本文引述为准」执行，不臆造数值。

## 3. 不变量（红线，全程 CI 拦截）

1. 客户可见导航恰为 9 项 + 账户；IA-04～09 六页无客户路由、不进导航。
2. 三态 `mention / recommendation / link` 分列；不出任何聚合可见度分值；`finance_health_score` 属财务域、分区展示。
3. 数字可溯源：`source_ref` + `figure_status` / `methodology_version`；无来源数字高亮阻断、不出报告。
4. 写操作门禁：草案 / 待批起步，须人工批准；广告上线只到 `paused_live`；财务不自动过账。
5. 前后对比固定文案：`compare_notice`。
6. 回放真实：`evidence_ref` 真实，禁 mock；失败运行不进 `n_valid`。
7. 四 flag 正交；口径版本本批不变（1.3）。
8. 溯源只读：SQL 禁写语句，集成 `scope` 恒 `read_only`。
9. 三个固定 notice 逐字、独立、不可配置覆盖。
10. `RunAnswer` 不可变，本批无写操作。

## 4. 阶段计划

| 阶段 | 内容 | 出口 |
| - | - | - |
| 1 | 变更单 + 最小基线 + 第五批契约 | Go 契约测试绿 |
| 2 | SQLite 数据层 + 16 表迁移 + workspace_id | 迁移测试绿、可重复执行 |
| 3 | 改名 AnswerLens + 套餐 50/300/700 | 构建绿、E2E 绿 |
| 4 | ECF-01/13/14/15 横切 | 路由骨架 + 状态机 + CI 断言 |
| 5 | ECF-02/03/04 + 四大模块 | 10 路由可用、主流程闭环 |
| 6 | ECF-05/06/07/12/16 | P1 页 + 回滚演练 |
| 7 | 全量验证 + 三端产物 | TC-F 全绿、Release 更新 |

---

*本变更单授权对规格文档 v1.0 的四项偏离；其余全部按 v1.0 执行。*
