# 竞品功能采集批次（CF-01~CF-08）实现说明

> 依据：`CodeBuddy_Engineering_Spec_Competitor_Features_20261116_v1.0.md`（唯一实现依据）
> 启动提示词：`STARTUP_PROMPT_Competitor_Features.md`
> 与 11/16 增量批次（`aeo_delta_20261116`）正交，独立 flag `aeo_competitor_20261116`。

## 1. 证据分级（EX-14 模板）

[事实] 本批次的字段、枚举、接口、任务卡、测试用例与排期均为 AnswerLens 自身工程要求，由工程规格 v1.0 定稿，可直接实现。本项目为 Go 结构体 + JSON 文件持久化，规格 §2.1 的 SQL 为示意，按项目实际存储调整，表名 / 字段名 / 枚举值拼写保持不变。

[观察] 本地验证记录来自测试层：`go test ./...`、`npm run build`、Playwright、CI 静态断言与 API 端点实测。证据来自哪一层：answerlens_observation（平台可观测回答层）与本地自动化测试层。外部平台公开行情、订单簿或结算数据未接入本批次。

[推导] 竞品功能点的取舍来自产品判断，不是任何外部平台已具备的能力或平台规则。所有采样轮次、定位维度、爬虫证据校验与 gap 信号的实现均为本平台自身口径设计，不构成对第三方平台收录、推荐或效果承诺。

## 2. 排除项清单

- 不实现综合 AI 可见性分数 / 指数 / 0–100 分（R-01 只做 CI + DoD 验收化）。
- 不实现 CMS 自动发布、AXP、本地多门店 per-location 竞品对比、传统 SEO 指标并排。
- 不新增 MCP 业务工具；`get_evidence` 只做 REST 端点与导出文件。
- 不模拟爬虫、不伪造 UA、不虚构 crawl→citation 路径或 citation 记录。
- 不反推历史 `capture_channel`；历史一律 `unknown`，不用 1.2 重算历史报告。
- 不改 D-1~D-6 默认值、不改套餐结构、不改 EX-01~EX-14 / TC-16-01~19 / E1~E7。
- 不把 `[观察]` 级第三方功能点写成竞品能力、平台规则或效果承诺，不做因果归因。
- 不把 `priority_score` 展示为品牌可见度分数，不进任何指标聚合，不触发品牌表现告警。

## 3. 任务卡落地

| 卡 | 实现位置 | 要点 |
|---|---|---|
| EC-08 | `internal/competitor/ci.go`、`banned-phrases.txt` | 追加禁词表，复用既有 `delta.CICheck` 通道；五处输出面综合分禁令、采样/分母正交、中立性不变式等 11 项追加断言 |
| EC-03 | `domain/competitor_contract.go`、`internal/competitor/output.go` | CE-1 采集通道、`methodology_version=1.2`、三口径渲染（1.0.0 / 1.1 / 1.2） |
| EC-01 | `internal/competitor/runtime.go`、`judge.go` | `sample_rounds`（默认 1）、`sample_policy_version`、`sample_index` 连续唯一、`n_valid` 与采样轮次正交 |
| EC-05 | `domain.NeutralityNotice`、报告 API / 报告页 / 落地页 | 逐字固定文案，渲染层常量，不落库、不可配置覆盖 |
| EC-07 | `internal/competitor/output.go`、`api/handlers/competitor_handler.go` | `GET /v1/evidence/{run_id}` 只读，CSV / JSON 导出含原文与完整 citations |
| EC-04 | `domain.PromptTargeting`、`SetTargeting` | CE-2 定位配置显式落库，未设定写 `none`，不改写 `query_locale` |
| EC-02 | `internal/competitor/judge.go`、`runtime.go` | CE-4 IP 核验访问日志、spoof 过滤、逐页真实 `crawl_citation_path`（P1） |
| EC-06 | `internal/competitor/judge.go`、`output.go` | CE-3 三类 gap 信号、`priority_score` 仅内部排序、Action Board 接入（P1） |

## 4. 数据契约与不变式

- 批次内契约 `C-1~C-4`、枚举 `CE-1~CE-4`、测试 `TC-C-01~14`、DoD `DoD-C-1~7` 均为独立编号段，不占 PRD §5 契约 1~11 / §9 DoD 1~21。
- `capture_channel` 与 `retrieval_locale_source`（E1）互相独立，不得互相填充或反推。
- `unknown` 不进任何分母或基线，UI 显示「采集通道未标注」。
- `RunAnswer` 只追加；`fix_artifact` / `fix_artifact_event` 与 `run_answer` 无写路径。
- `methodology_version` 1.0.0 / 1.1 / 1.2 三口径并存，历史报告保留生成时版本，旧口径显式标注。
- `crawl_citation_path.citation_id` 必须指向真实 `citation`；`suspected_spoof` 记录不参与 pass 判定。

## 5. flag 与回滚

`GEOLENS_FLAG_AEO_COMPETITOR_20261116=off` 秒级关闭；管理端点 `POST /api/v1/competitor/flag` 可运行时切换。关闭后所有本批次端点返回 404，既有 `/v1/runs/{id}` 与 `/v1/reports/{id}` 不返回任何新字段，`aeo_delta_20261116` 行为不变。

回滚顺序（§8）：① flag off → ② 撤掉 API 追加字段与新端点 → ③ 撤掉 1.2 渲染与新增 CI 规则（保留 1.0 / 1.1 渲染）→ ④ 不删新增列 / 表，仅停用 → ⑤ 物理回滚按 down 脚本逆序并先备份四张新表。`RunAnswer` 原始证据在任何路径下不删除。

## 6. DoD-C 逐条

- [x] DoD-C-1 五处输出面（报告模板 / 导出序列化器 / PDF 模板 / 告警模板 / Action Board）无综合分字段，既有禁词表与追加项全部生效。
- [x] DoD-C-2 EC-01~EC-08 验收断言通过，TC-C-01~TC-C-14 全绿。
- [x] DoD-C-3 回归基线 TC-16-01~19、PRD DoD 1~14、增量 DoD 15~21 继续全绿。
- [x] DoD-C-4 C-1~C-4 各有对应单元测试，含「多轮采样不改变 n_valid 定义」独立断言。
- [x] DoD-C-5 flag on/off 双向验证：off 时行为与批次前一致，且不影响 delta flag。
- [x] DoD-C-6 迁移 / 回填脚本可重复执行；1.0.0 / 1.1 / 1.2 三口径均可渲染；回滚顺序已实现。
- [x] DoD-C-7 固定中立性文案在报告 API / 报告页 / 落地页三处可见；evidence 端点可回溯原文与完整 citations；D-1~D-6 默认值未被改动，无遗留 `[决策]` 分支。
