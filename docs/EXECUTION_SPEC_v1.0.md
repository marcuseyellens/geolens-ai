# CodeBuddy 执行规格书｜2026-11-16 AEO 增量批次（Δ-01~Δ-07）

| 项目 | 内容 |
| --- | --- |
| 文档版本 | v1.0 |
| 日期 | 2026-11-16 |
| 状态 | **已批准 · 可执行（Approved for Implementation）** |
| 批准结论 | 产品已放行《PRD_Increments_AEO_Signals_20261116_v1.0.md》全部 7 条增量，并**一次性通过 §7 全部 6 项待决策**。M0 视为已拍板，CodeBuddy 按本文实现，**无需再等待评审** |
| 目标读者 | **CodeBuddy（开发实现方）**、后端、采集工程、数据工程、前端、QA |
| 需求层基线 | 《PRD_GEO_AI_Visibility_Platform_v1.0.md》（F 编号 / §5 契约 1~8 / §9 DoD 1~14）+ 《PRD_Increments_AEO_Signals_20261116_v1.0.md》（Δ-01~Δ-07 / 契约 9~11 / DoD 15~21） |
| 本文定位 | **执行层唯一依据**：把已批准的需求翻译成任务卡、DDL、接口、测试用例、CI 断言、迁移与排期。需求层与本文冲突时，**以本文定稿决策为准** |
| 配套 WBS | 《WBS_AEO_Feature_Extraction_v1.0.md》（A-WP-1~9）、《WBS_AnswerLens_MustHave_v1.0.md》（WP-0~8） |
| 范围 | 增量 **27 净人日** 全部并入本轮（P0 20 / P1 7）；全项目 435 → **462 人日**；4 人团队 20 → **约 21 周** |

> **给 CodeBuddy 的三条硬指令**
> 1. 本文所有 `[决策]` 已消除，实现时**不得自行改默认值**；如需变更走变更单，不口头调整。
> 2. 所有新增字段、枚举、事件、CI 规则**按本文拼写落库**，禁止同义改名（历史文档与测试用例按此拼写断言）。
> 3. 新增行为全部挂在 feature flag `aeo_delta_20261116` 下，可一键回到 v1.0 口径；回滚顺序见 §7。

---

## 0. M0 决策定稿（6 项，已放行）

| # | 原待决策 | **定稿结论（实现依据）** | 理由 / 实现约束 |
| --- | --- | --- | --- |
| **D-1** | `retrieval_locale` 信号优先级与 `inferred` 允许范围 | 优先级固定为 `platform_header > citation_domain > answer_text > inferred > unknown`。**允许 `inferred`**（答案文本语言检测），但必须附 `run_id` 证据与 `confidence` 浮点；**`confidence < 0.7` 一律降级为 `unknown`**；`unknown` 不进任何分母。禁止输出「模型内部以 X 语言检索」类措辞 | 只允许可观测信号；宁可标 `unknown` 也不猜。判定器见 EX-02 |
| **D-2** | 是否保留 `user_provided` 传统 SEO 指标入口 | **保留，但严格隔离**。仅出现在报告「独立参考区」，标注「用户自述，未经核验；传统 SEO 指标，不代表 AI 答案可见性」；不进图表、不进 API 聚合、不参与排序、不参与任何均值。API schema 中该字段只存在于 `user_provided_metric` 表 | 代理商客户会自带 SEO 数据，直接拒绝会推高对抗；隔离保证不污染指标（契约 10） |
| **D-3** | `suspect` 批次级判定阈值 | 批次 = 同一 `platform × model × 采集窗口(1 小时)` 的运行集合。**命中任一即标 `suspect`**：① 批次失败率 ≥ `0.20`；② `rate_limit / auth_error / timeout` 占比 ≥ `0.30`；③ 适配器版本变更；④ 平台故障登记；⑤ 人工标记。阈值写入配置 `suspect_threshold_config`，**首版按上述默认值直接实现，不等历史回放**；上线后用 11/09~11/16 数据回放校准，校准只改配置不改代码 | `suspect` 与 `valid` 正交；人工标记必须写 F-903 审计日志 |
| **D-4** | `ToolManifest` 保留版本数 N 与破坏性变更定义 | **N = 5**。版本格式 `major.minor`：**破坏性变更**（删除工具 / 改工具名 / 删必填参数 / 改参数类型 / 改返回 schema 必填字段 / 改鉴权范围）升 `major` 并发 `tool_manifest_changed`；新增工具、新增可选参数、新增返回字段升 `minor`，不发破坏性事件 | 所有变更都要升版本，删工具不升版本 CI 直接失败（DoD 19） |
| **D-5** | 27 人日是否并入本轮 | **全部并入本轮范围**。P0 20 人日排 M0~M5；P1 7 人日（EX-09 / EX-10 共 6 + EX-13 共 1）中，Δ-04 的 6 人日排 **M6 之后小版本**，EX-13 随 M5 回归做。**不砍 P0 换周期，不加人**，团队 4 人不变，发布点顺延至约第 21 周 | Δ-04 只做清单治理、不在关键路径，后置不影响 $39 报告出口 |
| **D-6** | 跨语言问题集是否算独立配额 | **算独立配额**。每套 `question_set`（按 `market_id × locale` 唯一）独立计入 `prompt quota`；同时在 UI 提供「同一品类多语言打包」配额提示，避免用户误判为重复扣费。计费说明写进 F-602 与定价页 | 不同语言是不同采集任务，产生真实成本；口径必须与 F-105 / F-604 对账一致 |

> 以上 6 项已由产品放行，**评审纪要中不再保留 `[决策]` 标记**。实现中若发现默认值不合理，提交变更单并同步更新本文版本号，不得静默调整。

---

## 1. 执行范围与排期

### 1.1 任务卡总表

| 任务卡 | 归属 | 内容 | 人日 | 优先级 | 里程碑 |
| --- | --- | --- | --- | --- | --- |
| EX-01 | Δ-01 | 语言字段 DDL 与历史回填 | 2 | P0 | M0–M1 |
| EX-02 | Δ-01 | `retrieval_locale` 判定器 | 2 | P0 | M1 |
| EX-03 | Δ-01 | 分语言问题集生成与配额 | 2 | P0 | M2 |
| EX-04 | Δ-01 | 指标分组、四态 UI 与跨语言说明 | 2 | P0 | M2 |
| EX-05 | Δ-02 | citation host 精确归一 + SEO 字段剥离 | 2 | P0 | M1 |
| EX-06 | Δ-02 | `user_provided` 参考区 + CI 静态断言 | 1 | P0 | M2 |
| EX-07 | Δ-03 | `crawler_visit_evidence` 检查项 | 3 | P0 | M3 |
| EX-08 | Δ-03 | 结果页分行展示 + 修复清单改写 | 2 | P0 | M3 |
| EX-09 | Δ-04 | ToolManifest 版本、自检工具、变更事件 | 3 | **P1** | M6 后 |
| EX-10 | Δ-04 | 清单裁切显式化 + Webhook 幂等 | 3 | **P1** | M6 后 |
| EX-11 | Δ-05 | `suspect` 批次标记器与阈值配置 | 2 | P0 | M1 |
| EX-12 | Δ-05 | 基线/告警/报告四数隔离 + 健康看板分栏 | 1 | P0 | M3 |
| EX-13 | Δ-06 | 采纳埋点与 30/60 天留存看板 | 1 | **P1** | M5 |
| EX-14 | Δ-07 | 三级证据标记模板 + 评审检查表 + CI 校验 | 1 | P0 | M0 |
| | | **合计** | **27** | **P0 20 / P1 7** | |

### 1.2 依赖关系

```
EX-14（模板/CI 基线，M0）
   │
EX-01（字段 DDL）──┬──► EX-02（判定器）──► EX-04（指标/UI）
                    ├──► EX-03（分语言问题集，依赖 F-103 / A-WP-7.1）
                    └──► EX-11（suspect 标记器，依赖 F-106 / A-WP-6.1）
EX-05（host 归一）──► EX-06（参考区 + CI）
EX-07（访问证据检查）──► EX-08（结果页 / F-502 修复清单）
EX-11 ──► EX-12（基线/告警/四数，依赖 A-WP-6.5）
EX-04 + EX-06 + EX-08 + EX-12 ──► M4 首份 $39 报告出口 ──► EX-13（埋点）
EX-09 ──► EX-10（均 M6 后，依赖基线 WP-5 / A-WP-4）
```

**关键路径**：`EX-01 → EX-07 → EX-08 → M4 报告出口` 为最长链（A-WP-2 本就在关键路径，EX-07/08 共 5 人日直接顺延发布点约 1 周）。EX-09/10 不在关键路径。

### 1.3 里程碑（4 人团队，21 周）

| 里程碑 | 周期 | 交付任务 | 出口 |
| --- | --- | --- | --- |
| M0 | 第 0 周 | EX-14、EX-01（字段定稿与配置默认值落库） | 决策定稿、DDL 评审通过、flag 就位 |
| M1 | 1–2 周 | EX-02、EX-05、EX-11 | 判定器 / host 归一 / suspect 标记器可单测 |
| M2 | 3–5 周 | EX-03、EX-04、EX-06（与 A-WP-9 落地页同步） | 分语言问题集、四态 UI、参考区上线 |
| M3 | 6–8 周 | EX-07、EX-08、EX-12 | 站点检查升级版 + 四数隔离 + 健康分栏 |
| M4 | 9–13 周 | 全量 TC-16 回归，并入首份 $39 报告 | 报告含契约 9~11 全部口径 |
| M5 | 14–15 周 | EX-13 + A-WP-9.4 自举四项回归 | 埋点与留存看板可用 |
| **发布** | **约第 21 周** | DoD 1~21 全过、flag 全开 | 发布 |
| M6 后小版本 | 发布后 | EX-09、EX-10 | MCP 清单治理上线 |

---

## 2. 数据层变更（DDL / 迁移 / 回填）

> 以下为**字段与约束契约**，SQL 为示意，按项目实际 ORM / 数据库调整；**表名、字段名、枚举值拼写不可改**。所有新增列允许 `NULL`，由回填脚本补默认值。

### 2.1 新增列

```sql
-- Δ-01
ALTER TABLE question
  ADD COLUMN query_locale        VARCHAR(16),          -- BCP-47，如 pl-PL
  ADD COLUMN locale              VARCHAR(16),          -- 问题集语言
  ADD COLUMN question_set_type   VARCHAR(32);          -- market_baseline | brand_tracking | brand_check

ALTER TABLE run_answer
  ADD COLUMN query_locale             VARCHAR(16),
  ADD COLUMN retrieval_locale         VARCHAR(16),     -- 观测字段，非平台机制声明
  ADD COLUMN retrieval_locale_source  VARCHAR(24),     -- 见枚举 E1
  ADD COLUMN retrieval_confidence      DECIMAL(4,3),   -- inferred 专用，其余 NULL
  ADD COLUMN locale_mismatch          BOOLEAN DEFAULT FALSE,
  ADD COLUMN crawler_visit_evidence   VARCHAR(16);     -- 见枚举 E3，Δ-03 共用

-- Δ-02
ALTER TABLE citation
  ADD COLUMN host                VARCHAR(255),        -- 精确 host，含子域名
  ADD COLUMN registrable_domain  VARCHAR(255);       -- 仅展示分组，禁止继承权重

-- Δ-03
ALTER TABLE audit_check
  ADD COLUMN platform_hint       TEXT,                -- 平台提示文案，仅线索
  ADD COLUMN observed_evidence   TEXT,                -- 实际观察证据
  ADD COLUMN evidence_source     VARCHAR(24);         -- 见枚举 E2

-- Δ-05
ALTER TABLE run
  ADD COLUMN suspect         BOOLEAN DEFAULT FALSE,
  ADD COLUMN suspect_reason  VARCHAR(32);             -- 见枚举 E4

-- Δ-06
CREATE TABLE fix_artifact_event (
  event_id      BIGINT PRIMARY KEY,
  report_id     BIGINT NOT NULL,
  fix_item_id   BIGINT NOT NULL,
  event_type    VARCHAR(24) NOT NULL,                 -- viewed | downloaded | applied | retest_requested
  occurred_at   TIMESTAMP NOT NULL,
  actor_id      BIGINT
);

-- Δ-02 / D-2
CREATE TABLE user_provided_metric (
  metric_id     BIGINT PRIMARY KEY,
  project_id    BIGINT NOT NULL,
  metric_type   VARCHAR(32) NOT NULL,                 -- dr | da | domain_authority | organic_rank | backlink_count
  value         DECIMAL(12,2),
  source_type   VARCHAR(24) NOT NULL DEFAULT 'user_provided',
  disclaimer    TEXT NOT NULL,                        -- 固定文案，不可为空
  created_at    TIMESTAMP NOT NULL
);

-- Δ-05 / D-3
CREATE TABLE suspect_threshold_config (
  config_key    VARCHAR(48) PRIMARY KEY,
  config_value  DECIMAL(6,3) NOT NULL,
  updated_at    TIMESTAMP NOT NULL,
  updated_by    VARCHAR(64) NOT NULL
);
-- 初始行：failure_rate=0.20, error_ratio=0.30, window_minutes=60, manifest_keep_versions=5(D-4), inferred_min_confidence=0.700(D-1)

-- Δ-04 / D-4（M6 后）
CREATE TABLE tool_manifest (
  manifest_id       BIGINT PRIMARY KEY,
  manifest_version  VARCHAR(16) NOT NULL,            -- major.minor
  published_at      TIMESTAMP NOT NULL,
  tools_json        JSON NOT NULL,
  is_breaking_change BOOLEAN NOT NULL DEFAULT FALSE,
  previous_version  VARCHAR(16)
);
```

### 2.2 枚举契约（**拼写固定**）

```
E1 retrieval_locale_source ∈ {platform_header, citation_domain, answer_text, inferred, unknown}
E2 evidence_source        ∈ {access_log, active_probe, user_provided, platform_hint, none}
E3 crawler_visit_evidence ∈ {pass, partial, fail, unknown}
E4 suspect_reason         ∈ {platform_incident, error_rate_spike, rate_limit, adapter_version_change,
                             timeout_cluster, manual}
E5 question_set_type      ∈ {market_baseline, brand_tracking, brand_check}
E6 audit verdict          ∈ {pass, partial, fail, unknown}     -- 沿用 A-03
E7 source_type            ∈ {platform_first_party, answerlens_observation, user_provided}  -- 沿用 A-02
```

**不变式（必须写成数据库约束或应用层断言）**：
- `locale_mismatch = TRUE` 仅当 `query_locale != retrieval_locale AND retrieval_locale != 'unknown'`；
- `retrieval_locale = 'unknown'` 时 `retrieval_confidence` 必须为 `NULL`；
- `retrieval_locale_source = 'inferred'` 时必须存在 `run_id` 证据且 `confidence >= 0.700`，否则降级为 `unknown`；
- `RunAnswer` **保持不可变**（只追加，不 UPDATE）；
- `citation.host` 不得由 `registrable_domain` 反推填充，二者独立存储；
- `suspect` 与 `valid` 正交，不得互相覆盖。

### 2.3 历史回填（EX-01 / EX-05 / EX-11）

| 表 / 字段 | 回填规则 | **禁止事项** |
| --- | --- | --- |
| `question.query_locale` / `locale` | 从原 `region.locale` 或问题集配置回填 | 不得猜测 |
| `run_answer.query_locale` | 从关联 `question.query_locale` 回填 | — |
| `run_answer.retrieval_locale` | **历史一律 `unknown`**，`locale_mismatch = FALSE` | **禁止反推历史检索语言**；历史报告口径不被改写 |
| `citation.host` / `registrable_domain` | 从已存 `source_url` 解析 | 不得合并子域到根域计数 |
| `run.suspect` | 历史一律 `FALSE`；11/09~11/16 已知故障批次（M365 降级、Claude 错误率升高）可由人工登记补标，每条写 F-903 审计日志 | 不得批量脚本静默标记 |
| `report.methodology_version` | **升到 `1.1`**，仅对新生成报告生效；历史报告保留生成时版本 | 不得用新算法重算历史报告 |

---

## 3. 接口与事件契约

### 3.1 REST（新增 / 变更字段）

```
GET /v1/runs/{run_id}
  → { ..., query_locale, retrieval_locale, retrieval_locale_source,
        retrieval_confidence, locale_mismatch, suspect, suspect_reason }

GET /v1/reports/{report_id}
  → { ..., methodology_version: "1.1",
        counts: { planned, usable, failed, suspect },        // Δ-05 四数必填
        language_notice: "...",                             // 跨语言并列观察说明
        reference_only: [ ... ] }                            // D-2 独立参考区

GET /v1/audits/{audit_id}/checks
  → [ { check_id, ..., platform_hint, observed_evidence,
        evidence_source, verdict, crawler_visit_evidence } ]

POST /v1/audits/{audit_id}/crawler-evidence        // 用户上传访问日志片段
  body: { evidence_source: "user_provided" | "access_log", content, note }
  → 201 { check_id, verdict }                      // 标注「用户自述，未经核验」

GET /v1/market/{market_id}/question-sets
  → [ { question_set_id, locale, question_set_type, quota_consumed, ... } ]  // D-6

GET /v1/usage/retention?window=30|60               // Δ-06 / EX-13
  → { viewed, downloaded, applied, retest_requested, retained_30d, retained_60d }
```

**强制响应约束**：
- `counts` 四数互不折算，`suspect` 不得并入 `failed` 或 `usable`；
- 任何含数字的组件必须带 `source_type` 与 `methodology_version`（沿用 A-02）；
- `reference_only` 与业务指标**不同数组、不同表头**，禁止同列求平均；
- 全量 API schema **不得出现** `overall_score`。

### 3.2 MCP（EX-09 / EX-10，M6 后）

```
GET  /v1/mcp/manifest
  → { manifest_version, published_at, tools[], is_breaking_change, previous_version }

tools/list 响应（固定包装，禁止静默截断）
  → { manifest_version, tools[], returned_count, total_count, truncated }
     truncated=true 时必须同时给出三个计数；客户端据此提示「能力集不完整」

新增自检工具：list_tools / get_manifest
  返回当前清单版本与完整工具列表，供 A-14 两步自举验收
```

### 3.3 Webhook 事件

| 事件 | 触发 | 载荷 | 约束 |
| --- | --- | --- | --- |
| `tool_manifest_changed` | `is_breaking_change=true` 的发布 | `{ manifest_version, previous_version, changed_tools[], published_at, event_id }` | 可重放、幂等（沿用 A-WP-6.4）；`event_id` 去重 |
| `run.completed` / `alert.triggered` / `report.delivered` | 沿用 F-702 | 不变 | — |
| **禁止新增**「品牌表现变化」类事件由 `suspect` 批次触发（契约 11） | | | |

---

## 4. 任务卡（EX-01~EX-14）

每张卡固定：**改动点 / 输入 / 输出 / 验收断言 / 依赖 / 人日 / 角色**。验收断言编号对应 §5 测试用例。

---

### EX-01 语言字段 DDL 与历史回填｜2 人日｜P0｜BE+DE

- **改动点**：§2.1 全部 `ALTER` / `CREATE`、§2.2 不变式、§2.3 回填脚本、`suspect_threshold_config` 初始行（D-1/D-3/D-4 默认值）、feature flag `aeo_delta_20261116`。
- **输入**：现有 `question / run_answer / run / citation / audit_check / report` 表。
- **输出**：可重复执行的 migration + 回填脚本 + 回滚脚本（§7）；`config` 表 5 行初始值。
- **验收断言**：TC-16-01、TC-16-02；全部新增列允许 NULL；`RunAnswer` 无 UPDATE 路径；flag 关闭时 API 不返回任何新字段。
- **依赖**：无（最先做）。**角色**：BE 主责，DE 复核回填。

### EX-02 `retrieval_locale` 判定器｜2 人日｜P0｜COL+BE

- **改动点**：采集适配器出站后新增判定器，按 D-1 优先级 `platform_header > citation_domain > answer_text > inferred > unknown` 写 `retrieval_locale / retrieval_locale_source / retrieval_confidence`；`inferred` 走答案文本语言检测，`confidence < 0.700` 降级 `unknown`；计算 `locale_mismatch`。
- **输入**：`raw_answer_text`、`citations[]`、平台响应头、F-107 原始证据。
- **输出**：`RunAnswer` 四个新字段（不可变、只追加）。
- **验收断言**：TC-16-03、TC-16-04；**禁止**在任何输出字段或日志里写「模型内部以 X 语言检索」。
- **依赖**：EX-01、F-101 适配器接口。**角色**：COL 主责。

### EX-03 分语言问题集生成与配额｜2 人日｜P0｜BE+PM

- **改动点**：F-103 生成入口增加必填 `locale`；同一 `market_id × locale` 唯一一套 `question_set`，**分别生成、分别落库，禁止把一种语言的问题直接翻译复用为另一语言**；每套独立计入 `prompt quota`（D-6）；UI 提供「同一品类多语言打包」配额提示；计费说明写进 F-602 与定价页。
- **输入**：`market_id`、六要素（沿用 A-08）、`locale`。
- **输出**：`question_set` + `Question[]`，带 `query_locale / locale / question_set_type`。
- **验收断言**：TC-16-05、TC-16-06；缺 `locale` 拒绝生成；两语言各 ≥ 40 题、四类意图齐全、无直接翻译复用题；配额扣减与 F-604 对账一致。
- **依赖**：EX-01、A-WP-7.1。**角色**：BE 主责，PM 定文案。

### EX-04 指标分组、四态 UI 与跨语言说明｜2 人日｜P0｜FE+DE

- **改动点**：`locale_mismatch=true` 的运行单独分组、不并入默认 mention rate / SoV / Citation Rate 分母；`retrieval_locale=unknown` 不进分母且 UI 显示「检索语言未判定」而非空白；跨语言报告固定显示「不同语言问题集不可直接比较，仅为并列观察」；报告方法论章节追加契约 9 说明。
- **输入**：`RunAnswer`、`counts`、`methodology_version`。
- **输出**：指标管线分组逻辑 + 报告/看板文案 + UI 态。
- **验收断言**：TC-16-04、TC-16-07；全量 schema 无 `overall_score`。
- **依赖**：EX-02、EX-03、F-206。**角色**：DE 主责，FE 配合。

### EX-05 citation host 精确归一 + SEO 字段剥离｜2 人日｜P0｜DE+BE

- **改动点**：F-108 / F-201 归一改为 **host 精确匹配**；`blog.example.com` 与 `example.com` 两条独立 citation；是否归并同 `brand_id` 只由品牌别名表显式配置；`CompetitorRelation` 排序权重只来自 `RunAnswer` 派生指标；已接入第三方数据源若含 `dr / da / domain_authority` 字段，落库时剥离为展示字段，不进指标管线；`registrable_domain` 仅作展示分组。
- **验收断言**：TC-16-08、TC-16-09；竞品图谱排序结果可逐条下钻 `run_id`（F-304）。
- **依赖**：EX-01、A-WP-5.2。**角色**：DE 主责。

### EX-06 `user_provided` 参考区 + CI 静态断言｜1 人日｜P0｜FE+QA

- **改动点**：报告新增「独立参考区」，渲染 `user_provided_metric`，固定文案「用户自述，未经核验；传统 SEO 指标，不代表 AI 答案可见性」；不进图表、不进 API 聚合、不参与排序；CI 加静态断言（§6）。
- **验收断言**：TC-16-10；注入一条 `user_provided` DR 值只出现在参考区。
- **依赖**：EX-05、F-501 报告模板。**角色**：FE 主责，QA 写断言。

### EX-07 `crawler_visit_evidence` 检查项｜3 人日｜P0｜COL+BE

- **改动点**：A-03 的 `index_eligibility` 组增第 9 项。判定依据优先级 **访问日志命中 AI crawler UA > 主动探测结果 > 平台提示文案**；平台提示（如 sitemap `Couldn't fetch`）只记 `platform_hint`，**不作 pass/fail 依据**；无日志且无法探测 → `unknown`（不扣分）；检查项可配置下线；用户可上传日志片段（EX-08 提供入口）。
- **合规红线**：**不得模拟爬虫、不得伪造 UA 刷目标站日志**；主动探测只针对用户自有站点且需显式授权；触碰即整组下线（A-WP-2 §7）。
- **验收断言**：TC-16-11；三组样本判定正确；`unknown` /100 不扣分。
- **依赖**：EX-01、A-WP-2.1/2.2。**角色**：COL 主责。**在关键路径上**。

### EX-08 结果页分行展示 + 修复清单改写｜2 人日｜P0｜FE+PM

- **改动点**：结果页把「平台提示」与「实际观察」**分行展示**（`platform_hint` 与 `observed_evidence` 独立字段、互不填充，有 CI 快照断言）；F-502 修复清单：当 `crawler_visit_evidence ∈ {unknown, fail}` 且 `platform_hint` 报错时，建议优先给「如何取得访问证据 / 如何提交抓取 / 如何验证爬虫到达」，而不是直接改 sitemap 文件；全部结论带「风险提示」+ 来源链接，无「平台会拒绝收录」类确定性表述。
- **验收断言**：TC-16-12；沿用 A-03 / A-WP-2 全部验收。
- **依赖**：EX-07、F-502。**角色**：FE 主责，PM 定文案。

### EX-09 ToolManifest 版本、自检工具、变更事件｜3 人日｜**P1（M6 后）**｜BE

- **改动点**：`tool_manifest` 表 + 版本 `major.minor`（D-4，N=5）；删工具/改名/删必填参数/改参数类型/改返回必填字段/改鉴权范围 → `major` + `is_breaking_change=true`；新增工具/可选参数/返回字段 → `minor`；**所有变更都要升版本**；新增自检工具 `list_tools` / `get_manifest`；破坏性变更发 `tool_manifest_changed`。
- **验收断言**：TC-16-13；删工具不升版本 CI 直接失败；最近 5 版可回溯。
- **依赖**：基线 WP-5、A-WP-4.3。**角色**：BE。**不在关键路径**。

### EX-10 清单裁切显式化 + Webhook 幂等｜3 人日｜**P1（M6 后）**｜BE+QA

- **改动点**：`tools/list` 固定包装 `{ manifest_version, tools[], returned_count, total_count, truncated }`；因 host/SDK 限制需截断时必须 `truncated=true` + 三个计数齐全，**禁止静默截断**；客户端提示「能力集不完整」；`tool_manifest_changed` 事件可重放、按 `event_id` 幂等（沿用 F-702 / A-WP-6.4）；A-14 自举验收同时校验清单版本一致性。
- **验收断言**：TC-16-14、TC-16-15；350 工具清单返回 `truncated=true` 且计数齐全。
- **依赖**：EX-09。**角色**：BE 主责，QA 写回归。

### EX-11 `suspect` 批次标记器与阈值配置｜2 人日｜P0｜COL+BE

- **改动点**：批次 = `platform × model × 1 小时窗口`；按 D-3 五条规则标 `suspect` 与 `suspect_reason`；阈值读 `suspect_threshold_config`（首版默认值直接实现，不等回放）；适配器版本变更自动触发；人工标记 `manual` 写 F-903 审计日志；`suspect` 与 `valid` 正交。
- **输入**：F-106 失败分类、F-107 原始证据、适配器版本表。
- **输出**：`run.suspect / suspect_reason`。
- **验收断言**：TC-16-16；单次失败不得升级为 `suspect`；配置变更留痕。
- **依赖**：EX-01、F-106、A-WP-6.1。**角色**：COL 主责。

### EX-12 基线/告警/报告四数隔离 + 健康看板分栏｜1 人日｜P0｜DE+FE

- **改动点**：`suspect` 批次不进四类指标分子分母、不进 A-07 八周滚动基线、不参与突变告警判定；**只触发「采集健康」类告警，不触发 F-801 品牌表现类告警**；报告与 UI 单列 `planned / usable / failed / suspect` 四数（互不折算）；F-901 采集健康看板与业务指标看板分栏；告警文案禁「因平台故障导致品牌下降」类因果表述；`suspect` 批次仍可沿 `run_id` 下钻原文（原始证据不删）。
- **验收断言**：TC-16-16、TC-16-17；注入 20 条 `valid=true` 但 `suspect` 的批次，品牌表现告警 0 条、健康告警 ≥ 1 条。
- **依赖**：EX-11、A-WP-6.5、F-901。**角色**：DE 主责，FE 配合。

### EX-13 采纳埋点与 30/60 天留存看板｜1 人日｜**P1**｜BE+DE

- **改动点**：`fix_artifact_event` 埋点区分 `viewed / downloaded / applied / retest_requested`；`GET /v1/usage/retention?window=30|60`；**北极星口径不变**（月度「已交付且被采纳的修复件」数量）；不新增「报告分数 / 检查项数量 / 监测模型数量」类指标；定价页与套餐结构不动。
- **验收断言**：TC-16-18；埋点能区分「仅查看报告」与「下载/采纳修复件」。
- **依赖**：F-503、F-505、A-WP-9.4。**角色**：BE 主责。

### EX-14 三级证据标记模板 + 评审检查表 + CI 校验｜1 人日｜P0｜PM+QA

- **改动点**：新增增量文档模板，强制含 `[事实] / [观察] / [推导]` 三级标记与**排除项清单**；评审检查表新增「是否有 `[观察]` 被写成竞品能力或平台规则」一项；CI 校验后续增量文档必须含三级标记与排除项（§6）。
- **验收断言**：TC-16-19；本文自身通过模板校验。
- **依赖**：无。**角色**：PM 主责，QA 写检查脚本。

---

## 5. 测试用例清单（TC-16-01~19）

> 全部用例必须可在 CI 中自动执行；`flag=off` 时需另跑一遍 v1.0 回归，确认行为完全一致。

| 编号 | 对应 | 前置 | 步骤 / 输入 | 期望结果 |
| --- | --- | --- | --- | --- |
| TC-16-01 | EX-01 / DoD 15 | 空库 | 跑 migration 两次 | 幂等成功；新增列全可 NULL；`RunAnswer` 无 UPDATE 路径 |
| TC-16-02 | EX-01 / DoD 20 | 历史数据 | 跑回填脚本 | `retrieval_locale` 全为 `unknown`；`locale_mismatch` 全 FALSE；历史 `report.methodology_version` 未被改写 |
| TC-16-03 | EX-02 / 契约 9 | 判定器 | `platform_header=de-DE` + `answer_text` 为波兰语 | 取 `platform_header`，`source=platform_header`，不写任何「模型内部检索」措辞 |
| TC-16-04 | EX-02/04 / DoD 15 | `query_locale=pl-PL` | `retrieval_locale=en-US` / `unknown` / 一致 三组 | 第一组 `locale_mismatch=true` 且不进默认分母；第二组分母排除且 UI 显示「检索语言未判定」；第三组正常计入 |
| TC-16-05 | EX-03 / DoD 16 | 单品类 | 分别以 `pl-PL` / `en-US` 生成 | 各 ≥ 40 题、四类意图齐全、`question_set_type` 不同、无直接翻译复用题；缺 `locale` 拒绝生成 |
| TC-16-06 | EX-03 / D-6 | 两套语言问题集 | 查 `prompt quota` | 各独立计数；打包提示可见；与 F-604 对账无差额 |
| TC-16-07 | EX-04 / 契约 9 | 跨语言报告 | 打开报告 | 含「不同语言问题集不可直接比较，仅为并列观察」；全量 schema 无 `overall_score` |
| TC-16-08 | EX-05 / DoD 17 | 两条 URL | `a.example.com` 与 `example.com` 各引一次 | citation 独立计数、不合并、不继承权重 |
| TC-16-09 | EX-05 / 契约 10 | 竞品图谱 | 检查排序字段与代码 | 无 `dr/da/domain_authority` 参与计算；排序结果可下钻 `run_id` |
| TC-16-10 | EX-06 / DoD 17 | 注入 `user_provided` DR=72 | 生成报告 | 仅出现在独立参考区、带固定免责文案；不进图表/聚合/排序 |
| TC-16-11 | EX-07 / DoD 18 | 三组样本 | ① 平台报错但日志有爬虫访问 ② 平台正常但无访问 ③ 无日志 | ① 按证据判 `pass`；② 不判 `pass`；③ `unknown` 且 /100 不扣分 |
| TC-16-12 | EX-08 / DoD 18 | 上一步结果页 | 检查 DOM 与 API | 「平台提示」「实际观察」分行；两字段互不填充；结论带「风险提示」+ 来源链接；无确定性收录表述 |
| TC-16-13 | EX-09 / DoD 19 | 清单 | 删一个工具但版本号不变 | CI 报错；破坏性变更升 `major` 并记 `is_breaking_change`；最近 5 版可回溯 |
| TC-16-14 | EX-10 / DoD 19 | 350 个工具 | 调 `tools/list` | `truncated=true`，`returned_count/total_count/manifest_version` 齐全；客户端提示「能力集不完整」 |
| TC-16-15 | EX-10 / DoD 19 | 破坏性变更 | 重放 `tool_manifest_changed` 两次 | 按 `event_id` 幂等；不重复投递 |
| TC-16-16 | EX-11/12 / DoD 20 | 20 条 `valid=true` 标 `suspect` | 跑指标 + 基线 + 告警 | 四类指标分母不含 20 条；基线不更新；品牌表现告警 0 条；健康告警 ≥ 1 条；单次失败不升级为 `suspect` |
| TC-16-17 | EX-12 / 契约 11 | 报告页 | 检查 `counts` 与文案 | `planned/usable/failed/suspect` 四数同屏互不折算；`suspect` 仍可下钻原文；禁词 CI 通过 |
| TC-16-18 | EX-13 / DoD 21 | 埋点事件 | 查留存看板 | 区分「仅查看」与「下载/采纳」；30/60 天出数；北极星口径未被改写 |
| TC-16-19 | EX-14 / Δ-07 | 任一新增量文档 | 跑模板校验 | 含三级标记与排除项清单；无 `[观察]` 被写成事实/平台规则 |

**回归基线**：PRD §9 DoD 1~14、AEO 提取文档 §8 十条增量验收，**全部必须继续通过**，不得因本次增量回归失败。

---

## 6. CI 静态检查与禁词表

| 检查项 | 规则 | 失败处理 |
| --- | --- | --- |
| 综合分禁令 | 全量 API schema / 代码中不存在 `overall_score` | 阻断合并 |
| 传统 SEO 权重禁令 | 指标计算路径中不存在 `dr` / `da` / `domain_authority` / `organic_rank` / `backlink_count` 参与聚合 | 阻断合并（EX-06） |
| host 归一 | `citation.host` 存在且不由 `registrable_domain` 反推；无 `GROUP BY registrable_domain` 参与 citation 计数 | 阻断合并 |
| 四数完整 | `counts` 必含 `planned/usable/failed/suspect` 四键，`suspect` 不折算 | 阻断合并 |
| 工具版本 | 工具清单 diff 有变更但 `manifest_version` 未升 → 报错；破坏性变更未发事件 → 报错 | 阻断合并（EX-09） |
| 裁切显式 | `tools/list` 返回 `truncated=true` 时三个计数缺一 → 报错 | 阻断合并（EX-10） |
| 不可变 | `RunAnswer` 出现 UPDATE / DELETE 语句 → 报错 | 阻断合并 |
| 三级标记 | 新增量文档缺 `[事实]/[观察]/[推导]` 或缺排除项清单 → 报错 | 阻断合并（EX-14） |
| 结果页快照 | 「平台提示」「实际观察」两行 DOM 快照比对 | 阻断合并（EX-08） |

**禁词表（全站文案、报告模板、告警模板、落地页、定价页）**：

```
保证|一定|提升排名|稳居|确保被推荐
因为改了.*所以|因修复.*提升|预计提升 \d+%
Perplexity 会拒绝收录|AI 平台因此改变收录|平台会拒绝收录
因平台故障导致品牌下降|故障导致可见性下降
模型内部以 .* 语言检索
```

> 禁词检查沿用 A-WP-1.5 的 CI 通道，**不新建第二套检查器**；本表作为该通道的规则输入追加。

---

## 7. 迁移、兼容与回滚

**上线顺序**
1. M0：部署 EX-01 migration + 回填 + `suspect_threshold_config` 初始值；`aeo_delta_20261116 = off`。
2. 逐任务卡上线，每卡灰度开 flag（先内部项目，再 1 个外部项目，再全量）。
3. M4 报告出口前完成 TC-16-01~12、16~17 全绿；M5 补 TC-16-18；M6 发布时 flag 全开。
4. M6 后小版本上线 EX-09/10，跑 TC-16-13~15。

**向后兼容**
- 所有新增 API 字段为**追加**，flag=off 时不返回；旧客户端不受影响；
- `methodology_version` 由 `1.0` 升 `1.1`，**仅对新生成报告生效**；历史报告保留生成时版本，禁止用新算法静默重算（PRD §6）；
- `report` 渲染层需同时支持 `1.0` / `1.1` 两种口径的 `counts`（`1.0` 只有三数，缺 `suspect` 时按 0 展示但标注「旧口径」）。

**回滚顺序（严禁跳步）**
1. `aeo_delta_20261116 = off`（秒级，所有新行为回到 v1.0 口径）；
2. 回滚 API 层（去掉新增字段与 `reference_only` / 四数）；
3. 回滚报告模板与新 CI 规则；
4. **不删新增列/表**（保留数据，仅停用），避免二次迁移丢数据；
5. 若必须物理回滚，按 EX-01 migration 的 down 脚本逆序执行，并先备份 `user_provided_metric` / `fix_artifact_event` / `tool_manifest`。

**数据保全**：`RunAnswer` 原始证据在任何回滚路径下都不得删除；`suspect` 批次始终可沿 `run_id` 下钻。

---

## 8. 交付定义（Definition of Done）

**全部满足才算完成**：
- [ ] EX-01~EX-14 全部任务卡验收断言通过（§4 / §5）
- [ ] TC-16-01~19 全绿，且 PRD §9 DoD 1~14、AEO 提取文档 §8 十条增量验收**继续全绿**
- [ ] §5 契约 9 / 10 / 11 有对应单元测试，且失败用例能复现
- [ ] §6 全部 CI 检查项接入既有流水线，禁词表生效
- [ ] `aeo_delta_20261116` 开关经 on/off 双向验证：off 时行为与 v1.0 完全一致
- [ ] 迁移与回填脚本可重复执行；回滚演练按 §7 顺序走通一次
- [ ] `methodology_version=1.1` 新报告生成正常，历史报告未被改写
- [ ] 6 项 M0 决策按 §0 默认值实现，代码中无遗留 `[决策]` 分支
- [ ] 用自家产品对自家站点跑 A-01 / A-03 / A-05 / A-14 四项自检全绿（沿用 A-WP-9.4）
- [ ] 落地页与报告 PDF 可见中立性声明；全站禁词 CI 通过
- [ ] 套餐结构未变（Free / $39 / $19 / $49·199·410）

---

## 9. 执行层红线（明确不做）

| 项 | 结论 |
| --- | --- |
| 把 Heldy 写成已核实竞品、做竞品对比表 | **不做**：仅 `[观察]`，能力/定价/覆盖不可核实 |
| 因 Ahrefs DR 口径调整改采集逻辑，或宣称「AI 收录变了」 | **不做** |
| 模拟爬虫 / 伪造 UA 制造访问证据 | **不做**：触碰 PRD §10 与 A-WP-2 合规红线，触碰即 EX-07/08 整组下线 |
| 把「没有爬虫来抓」写成「平台拒绝收录」 | **不做**：沿用 A-03「风险提示」措辞 |
| 反推历史 `retrieval_locale`、重算历史报告 | **不做**：历史一律 `unknown`，口径版本不被改写 |
| 新增 MCP 业务工具、承诺兼容所有 host | **不做（本期）**：EX-09/10 只做清单治理；`truncated` 是如实告知而非兼容承诺 |
| 因本批次调整定价或新增套餐 | **不做** |
| 把 M365 故障、AI 代码开源等 `[观察]` 写进产品文案或报告模板 | **不做** |
| 把 `[观察]` 升格为竞品能力或平台规则 | **不做**：EX-14 / CI 校验拦截 |
| 擅自修改 §0 六项默认值 | **不做**：走变更单并升本文版本号 |

---

## 附录 A：编号索引

| 层级 | 编号 | 所在文档 |
| --- | --- | --- |
| 需求功能点 | `F-101~F-903` | PRD v1.0 |
| 数据契约 | `1~8`（PRD v1.0）、`9~11`（增量文档） | PRD v1.0 / PRD_Increments_AEO_Signals_20261116_v1.0.md |
| DoD | `1~14`（PRD v1.0）、`15~21`（增量文档） | 同上 |
| 增量条目 | `Δ-01~Δ-07`、线索 `S16-1~S16-7` | PRD_Increments_AEO_Signals_20261116_v1.0.md |
| AEO 功能 | `A-01~A-14` | PM_AEO_Feature_Extraction_v1.0.md |
| WBS 工作包 | `A-WP-1~9`、`WP-0~8` | WBS_AEO_Feature_Extraction_v1.0.md / WBS_AnswerLens_MustHave_v1.0.md |
| **执行任务卡** | **`EX-01~EX-14`** | **本文** |
| **测试用例** | **`TC-16-01~TC-16-19`** | **本文** |
| **M0 决策** | **`D-1~D-6`（已放行）** | **本文 §0** |

## 附录 B：依据文件

- `PRD_GEO_AI_Visibility_Platform_v1.0.md`（需求基线，未改动）
- `PRD_Increments_AEO_Signals_20261116_v1.0.md`（7 条增量，已批准）
- `PM_AEO_Feature_Extraction_v1.0.md` / `WBS_AEO_Feature_Extraction_v1.0.md`
- `PM_AnswerLens_Feature_Breakdown_v1.0.md` / `WBS_AnswerLens_MustHave_v1.0.md`
- 2026-11-16「今日总结」第七节与工作日志 `.workbuddy/memory/2026-11-16.md`

> 本文为执行层定稿。所有外部线索均为 `[观察]` 级（未读全文、未核实因果），文中功能点均为 AnswerLens 自身工程要求，不构成对任何第三方平台能力、政策或效果的承诺。
