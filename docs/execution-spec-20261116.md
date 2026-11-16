# AEO 增量批次执行说明（Δ-01~Δ-07 / 2026-11-16）

> 本文是《CodeBuddy 执行规格书｜2026-11-16 AEO 增量批次 v1.0》在本仓库的落地说明。
> 命名、字段、枚举、事件、CI 规则一律按规格书拼写，禁止同义改名。
> 模板见 [`increment-template.md`](increment-template.md)。

## 一、范围与硬指令

[事实] 规格书批准范围：Δ-01 数据契约、Δ-02 跨语言检索、Δ-03 爬虫访问证据、Δ-04 MCP 工具清单、Δ-05 suspect 批次、Δ-06 采纳留存、Δ-07 证据可追溯，共 14 张任务卡（EX-01~EX-14）、19 条自动用例（TC-16-01~19）。

[事实] 三条硬指令：① 已冻结的默认值与既有自检口径不改动；② 所有新增字段、枚举、事件、CI 规则按本文拼写落库；③ 所有新行为挂在 feature flag `aeo_delta_20261116` 之下。

[观察] 本平台的数据层是 **答案层（answerlens_observation）**：可观测的是一次运行的原始回答文本、引用 URL、平台响应头与访问日志；`run_id` 可下钻。证据来自哪一层：答案层。

[推导] 规格书 §2 的 SQL 为示意。本项目以 JSON 文件持久化，故新增列以「Go 结构体追加字段 + `json` tag + `omitempty`」落地，迁移 / 回填 / 回滚由 `internal/delta/config.go` 的幂等函数实现，语义等价。此为概率性工程判断，风险提示：物理 down 脚本需先备份 `user_provided_metric` / `fix_artifact_event` / `tool_manifest` 三张新表。来源：`internal/delta/config.go`、`domain/delta_contract.go`。

## 二、M0 决策（D-1~D-6 定稿）

| 编号 | 决策 | 代码落点 |
|---|---|---|
| D-1 | `retrieval_locale` 判定优先级 `platform_header > citation_domain > answer_text > inferred > unknown`；`inferred` 需 `run_id` 证据与 confidence，`< 0.700` 降级 unknown | `internal/delta/retrieval.go` |
| D-2 | `user_provided` 只进独立参考区，免责声明固定不可为空 | `internal/delta/crawler.go` |
| D-3 | suspect 批次 = `platform × model × 1h`；五条规则；阈值配置首版直接实现 | `internal/delta/suspect.go` |
| D-4 | tool manifest 保留最近 5 版，版本 `major.minor`；破坏性变更触发 `tool_manifest_changed` | `internal/delta/manifest.go` |
| D-5 | 结果页「平台提示 / 实际观察」两行独立，平台提示不作 pass/fail 依据 | `internal/delta/crawler.go`、`frontend/src/routes/check.tsx` |
| D-6 | 同一品类多语言按不同采集任务分别计费，每套问题集独立计入 prompt quota | `internal/delta/questionset.go`、`frontend/src/routes/pricing.tsx` |

## 三、14 张任务卡落地对照

| 卡 | 内容 | 主责 | 落点 | 用例 |
|---|---|---|---|---|
| EX-01 | 迁移 / 回填 / 回滚、flag | DE | `internal/delta/config.go` | TC-16-01/02 |
| EX-02 | `retrieval_locale` 判定器 | DE+PM | `internal/delta/retrieval.go` | TC-16-03/04 |
| EX-03 | 分语言问题集与独立配额 | DE | `internal/delta/questionset.go` | TC-16-05/06 |
| EX-04 | 指标按语言分组、跨语言报告 | DE+FE | `internal/delta/retrieval.go`、报告页 | TC-16-07 |
| EX-05 | `citation.host` 精确归一、SEO 字段剥离 | DE | `internal/delta/retrieval.go` | TC-16-08 |
| EX-06 | 独立参考区 | FE+PM | `internal/delta/crawler.go`、`diagnostic-report.tsx` | TC-16-09 |
| EX-07 | `crawler_visit_evidence` 判定器 | DE+合规 | `internal/delta/crawler.go` | TC-16-11 |
| EX-08 | 结果页分行 + 修复清单改写 | FE+PM | `check.tsx`、`crawler.go` | TC-16-12 |
| EX-09 | ToolManifest 版本表 + 自检工具 | DE | `internal/delta/manifest.go` | TC-16-13/14 |
| EX-10 | `tools/list` 固定包装 + Webhook 幂等 | DE | `internal/delta/manifest.go` | TC-16-10/15 |
| EX-11 | suspect 批次标记器 + 配置 | DE+Ops | `internal/delta/suspect.go` | TC-16-16 |
| EX-12 | 四数隔离 + 健康看板分栏 | DE+FE | `suspect.go`、`monitor-page.tsx`、报告页 | TC-16-17 |
| EX-13 | 采纳埋点与留存 | DE+PM | `internal/delta/retention.go` | TC-16-18 |
| EX-14 | 三级标记模板 + 评审检查表 | PM+QA | `docs/increment-template.md`、`ci.go` | TC-16-19 |

## 四、上线顺序与回滚（§7）

[事实] 上线顺序：M0 迁移 + flag=off → M1 数据链路 → M2 判定器 → M3 报告与结果页 → M4 爬虫证据 → M5 MCP 清单 → M6 flag 全开 + 四数告警隔离 + 模板/CI。

[事实] 回滚顺序（`internal/delta/config.go` 的 `Rollback`）：
1. flag `aeo_delta_20261116=off`（秒级）；
2. 新 API 字段不再返回（`omitempty`）；
3. 报告模板 / 新 CI 规则停用；
4. **不删列、不删表**，保留数据；
5. 物理 down 脚本需先备份三张新表。

[观察] `GEOLENS_FLAG_AEO_DELTA_20261116=off` 启动时，日志显示 `on=false`，`/v1/reports/{id}` 返回 `{"error":"delta feature disabled","on":false}`，`/api/v1/delta/status` 仍可读用于运维核对，既有 `/api/v1/brands/:id/diagnose` 不返回任何新字段。证据：`api/handlers/delta_handler_test.go`、`api/handlers/delta_handler.go`。

[推导] 因 flag 只控制「是否填充新增字段」而非数据结构，回滚不需要数据迁移，风险最低。风险提示：`tool_manifest` 与 `fix_artifact_event` 一旦产生数据，物理删除需人工确认。来源：`domain/delta_contract.go`。

## 五、验收与 CI

[事实] §6 共 10 项静态断言在 `internal/delta/ci.go` 实现，接入既有 `.github/workflows/ci.yml`（不新建第二套检查器）：禁词表覆盖、`overall_score` 零命中、传统 SEO 词不进聚合、`citation.host` 独立落库、四数齐全、`tools/list` 固定包装、裁切提示、RunAnswer 无 UPDATE 路径、三级标记模板、结果页快照分行。

[事实] TC-16-01~19 全部为可执行 Go 单测（`internal/delta/tc16_test.go`），DoD 要求全绿；§3.1 端点另有 HTTP 集成测试（`api/handlers/delta_handler_test.go`）与 Playwright 契约用例（`frontend/e2e/specs/platform-api.spec.ts`）。

[观察] 本地验证记录：`go test ./...` 13 个包全绿；`npm run build` 通过；Playwright 30/30 通过；英文模式 18 个主要页面零中文残留（含 /app/competitor）；8 种界面语言的首页标题逐语言本地化；§6 端点返回 `all_pass=true`。

## 六、合规红线

[事实] 爬虫访问证据检查项（第 9 项）不得模拟爬虫、不得伪造 UA 刷目标站日志；主动探测仅限用户自有站点且需显式授权，触碰即整组下线。

[事实] 所有结论禁止确定性收录表述、禁止声明模型内部检索机制、禁止「因平台故障导致品牌下降」类因果表述。禁词表见 [`../banned-phrases.txt`](../banned-phrases.txt)。

## 排除项清单

- [x] 不声明模型内部检索机制（契约 9）
- [x] 不把 `user_provided` 指标混入图表、API 聚合、排序或均值（契约 10）
- [x] 不把 `suspect` 并入 `usable` / `failed`，四数互不折算（契约 11）
- [x] 不由 `registrable_domain` 反推 `host`，子域独立计数、不继承权重（EX-05）
- [x] 不把一种语言问题集直接翻译复用为另一种语言（EX-03）
- [x] 不用平台提示文案作为 pass / fail 依据（EX-08）
- [x] 不模拟爬虫、不伪造 UA 刷目标站日志（EX-07）

## 评审检查表

| 检查项 | 否决权 | 状态 |
|---|---|---|
| 三级标记齐全且 `[推导]` 带风险提示与来源链接 | 是 | 通过 |
| 写明「证据来自哪一层」= 答案层 | 是 | 通过 |
| 排除项清单完整 | 是 | 通过 |
| 新报告 `methodology_version=1.1`、历史保留生成时版本 | 否 | 通过 |
| flag 可一键回滚，off 时不返回新字段 | 否 | 通过 |
| TC-16 全部在 CI 中通过 | 否 | 通过 |
