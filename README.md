# GeoLens AI

> 把「AI 回答里到底提了谁」做成可核验的证据，而不是一个主观分数。

GeoLens AI 是一个 AEO / GEO（Answer / Generative Engine Optimization）可见性检查与诊断平台。它读取公开页面、采集真实 buyer question 的模型回答，保留逐条原始答案与引用证据，输出**逐模型、逐意图**的指标与修复清单。

本项目刻意不提供「综合分」，也不承诺任何模型的收录、推荐、排名或转化结果。**技术就绪 ≠ 被推荐**：所有分数只描述可观测的技术条件与回答内容。

## 本次增量批次（2026-11-16 / Δ-01~Δ-07）

依据《CodeBuddy 执行规格书｜2026-11-16 AEO 增量批次》实现，详见 [`docs/execution-spec-20261116.md`](docs/execution-spec-20261116.md)。

| 契约 | 能力 | 落点 |
|---|---|---|
| 契约 9 | `retrieval_locale` 观测判定（只记录可观测信号，不声明模型内部机制） | `internal/delta/retrieval.go` |
| 契约 10 | `user_provided` 独立参考区，严格隔离出图表 / API 聚合 / 排序 / 均值 | `internal/delta/crawler.go` |
| 契约 11 | `suspect` 批次四数隔离，健康告警与品牌表现告警分栏 | `internal/delta/suspect.go` |
| EX-01 | 幂等迁移 / 回填 / 回滚，全部新行为挂在 feature flag 下 | `internal/delta/config.go` |
| EX-03 | 分语言问题集分别生成、分别计入 prompt quota（D-6 打包） | `internal/delta/questionset.go` |
| EX-05 | `citation.host` 精确归一，子域独立计数、不继承权重 | `internal/delta/retrieval.go` |
| EX-07/08 | 第 9 项爬虫访问证据，结果页「平台提示 / 实际观察」分行 | `internal/delta/crawler.go`、`frontend/src/routes/check.tsx` |
| EX-09/10 | ToolManifest 版本治理、`tools/list` 固定包装与裁切提示 | `internal/delta/manifest.go` |
| EX-13 | 修复件采纳埋点与 30/60 天留存 | `internal/delta/retention.go` |
| EX-14 | 三级证据标记模板 `[事实]/[观察]/[推导]` + 排除项清单 + 评审否决项 | `docs/increment-template.md` |
| §6 | 10 项 CI 静态断言（禁词 / 四数 / host / 工具包装 / 结果页快照） | `internal/delta/ci.go` |
| §5 | TC-16-01~19 自动用例 | `internal/delta/tc16_test.go` |

**Feature flag**：`aeo_delta_20261116`。默认开启；设置 `GEOLENS_FLAG_AEO_DELTA_20261116=off` 即秒级回到 v1.0 口径——新端点停用、既有 API 不返回任何新字段。历史报告的 `methodology_version=1.0.0` 永不重算，新报告为 `1.1`。

## 核心功能

- **免注册站点检查（L0）**：输入网址得到 /100 就绪度与 1 条首要修复建议，可生成免登录只读分享链接。
- **逐模型三态指标**：品牌提及 / 推荐 / 链接分别统计，每个比例都伴随 `n_valid`，可下钻到 `run_id` 与原始答案。
- **四数隔离**：`planned / usable / failed / suspect` 互不折算；可疑批次只触发「采集健康」告警。
- **跨语言并列观察**：不同语言问题集分别生成、分别计费；检索语言未判定与 `locale_mismatch` 的运行不进默认分母。
- **独立参考区**：DR / DA 等用户自述的传统 SEO 指标单独成表，标注「用户自述，未经核验」。
- **中 / 英双语**：全站切换、`<html lang>` 同步、`localStorage` 持久化，英文模式零中文残留（E2E 断言）。
- **登录与鉴权**：bcrypt 密码哈希 + 随机会话；公开 / 需登录 / 管理员三组路由，`account_id` 只从会话取。
- **订阅开通**：套餐 CTA 跳 `/subscribe` 三步开通；`pricing` 与 `billing` 共用运行时套餐目录。
- **MCP 只读接入**：独立 Agent token 与 `tools/list` 固定包装，裁切时明确提示「能力集不完整」。

## 技术栈

| 层 | 技术 | 说明 |
|---|---|---|
| 后端 | Go + Gin | DDD + 六边形架构（Ports & Adapters） |
| 前端 | React + Vite + TypeScript + Tailwind CSS | 自建设计系统 |
| 图表 | Recharts | 柱状图、雷达图 |
| 存储 | JSON 文件 | 零配置；`delta.json` 承载增量批次配置与埋点 |
| 认证 | `golang.org/x/crypto/bcrypt` | 内存会话 + 用户持久化 |
| AI 模型 | OpenAI / Anthropic / Google / Perplexity | 多模型适配器 |

## 架构

```
geolens/
├── cmd/server/          # 主程序入口（组合运行时、鉴权、delta runtime）
├── domain/              # 领域契约：contract.go / aeo_contract.go / delta_contract.go
├── application/         # 应用服务 + 仓储接口（Ports）
├── internal/
│   ├── auth/            # 用户、会话、bcrypt
│   ├── aeo/             # 基线自检、验收清单、禁词扫描、问题集
│   ├── delta/           # Δ-01~Δ-07 判定器、迁移、CI 断言、TC-16
│   ├── siteaudit/       # 站点就绪度检查（8+1 项）
│   ├── metrics/         # 指标聚合
│   ├── billing/         # 套餐 / 配额 / 订单
│   ├── mcpserver/       # MCP 只读服务
│   └── storage/         # JSON 文件存储（Adapter）
├── api/handlers/        # HTTP API（Gin）：public / protected / admin 三组
├── docs/                # 执行规格、增量模板、MCP 文档
├── frontend/
│   └── src/
│       ├── i18n/        # 中英字典与翻译边界
│       ├── auth/        # 会话上下文与路由守卫
│       ├── routes/      # 页面（落地页、订阅、控制台、验收等）
│       ├── ds/          # 自建设计系统
│       └── api.ts       # API 客户端（自动带 Bearer）
└── .github/workflows/   # CI：Go 测试 / Δ 静态断言 / 前端构建 / Playwright
```

## 快速开始

```bash
# 前端
cd frontend && npm install && npm run build && cd ..

# 后端
go build -o geolens ./cmd/server/

# 启动
GEOLENS_FRONTEND_DIST=./frontend/dist ./geolens
```

打开 http://localhost:8080 。零配置启动会进入演示模式，并在控制台打印：

```
Delta feature aeo_delta_20261116: on=true (GEOLENS_FLAG_AEO_DELTA_20261116=off 可秒级回滚)
Demo login: demo@geolens.ai / demo1234 (admin)
MCP read-only agent token: ...
```

演示账号具备管理员角色，可操作预算复位、回滚与令牌签发。生产部署请通过 `GEOLENS_DEMO_EMAIL` / `GEOLENS_DEMO_PASSWORD` 覆盖，并接入独立身份服务与 Stripe Checkout。

### 真实检测

```bash
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export GEMINI_API_KEY=AIza...
export PERPLEXITY_API_KEY=pplx-...
./geolens
```

未配置任何 Key 时使用明确标注的模拟回答跑通完整流程；配置后自动切换为真实检测。

## API 参考（节选）

所有 `/api/v1` 与 `/v1` 接口除显式匿名端点外均需 `Authorization: Bearer <token>`。

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/login` | 登录，返回会话 token |
| POST | `/api/v1/auth/register` | 注册（普通账户） |
| GET | `/api/v1/auth/me` | 当前会话 |
| POST | `/api/v1/auth/logout` | 注销会话 |
| POST | `/api/v1/site-audits` | 免注册站点检查 |
| GET | `/api/v1/brands` | 品牌列表 |
| GET | `/api/v1/brands/:id/diagnose` | 诊断报告（flag on 时带四数 / 跨语言说明 / 独立参考区） |
| GET | `/api/v1/plans` | 运行时套餐目录 |
| POST | `/api/v1/billing/checkout` | 创建订单（`account_id` 只取会话） |
| GET | `/api/v1/delta/status` | flag 状态、口径版本、阈值配置、迁移结果 |
| GET | `/api/v1/delta/ci-assertions` | §6 全部静态断言结果 |

### Δ 批次契约端点（§3.1，flag on 时可用）

| 方法 | 路径 | 关键响应字段 |
|---|---|---|
| GET | `/v1/runs/{run_id}` | `query_locale`、`retrieval_locale`、`retrieval_locale_source`、`retrieval_confidence`、`locale_mismatch` |
| GET | `/v1/reports/{report_id}` | `methodology_version=1.1`、`counts{planned,usable,failed,suspect}`、`language_notice`、`reference_only` |
| GET | `/v1/audits/{audit_id}/checks` | `platform_hint`、`observed_evidence`、`evidence_source`、`verdict`、`crawler_visit_evidence` |
| POST | `/v1/audits/{audit_id}/crawler-evidence` | 上传访问日志；`user_provided` 标注未经核验、不判 pass |
| GET | `/v1/market/{market_id}/question-sets` | 分语言问题集、独立 `bundle_notice` 与 `total_quota` |
| GET | `/v1/usage/retention?window=30\|60` | 采纳率、30/60 天留存、按 `fix_item` 分组 |
| GET | `/v1/mcp/manifest` | `manifest_version`（major.minor） |
| GET | `/v1/mcp/tools?limit=N` | `manifest_version`、`returned_count`、`total_count`、`truncated`、`client_notice` |

## 验证

```bash
# 全量 Go 测试（含 TC-16-01~19 与 §6 静态断言）
go test ./...

# 前端类型检查与构建
cd frontend && npm run build

# Playwright 回归（需本地服务，见 frontend/playwright.config.ts）
BASE_URL=http://127.0.0.1:8080 PW_CHANNEL=chrome npx playwright test
```

CI 在每次 push / PR 运行：Go 测试、Δ 静态断言、禁词扫描、前端构建、Playwright 25 个用例。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | HTTP 服务端口 |
| `GEOLENS_DB` | `~/.geolens/geolens.json` | 主数据文件 |
| `GEOLENS_FRONTEND_DIST` | `./frontend/dist` | 前端构建产物目录 |
| `GEOLENS_DEMO` | 未设置 | 设为 `1` 强制演示模式 |
| `GEOLENS_DEMO_EMAIL` / `GEOLENS_DEMO_PASSWORD` | `demo@geolens.ai` / `demo1234` | 首次启动写入的演示管理员 |
| `GEOLENS_ADMIN_TOKEN` | 启动时生成 | 管理端点的 `X-Admin-Token` |
| `GEOLENS_FLAG_AEO_DELTA_20261116` | `on` | 设为 `off` 回滚到 v1.0 口径 |
| `OPENAI_API_KEY` 等 | 未设置 | 各模型 API Key |

## 文档

- [`docs/EXECUTION_SPEC_v1.0.md`](docs/EXECUTION_SPEC_v1.0.md)：原始执行规格书（Δ/EX/TC-16/DoD 全文）
- [`docs/execution-spec-20261116.md`](docs/execution-spec-20261116.md)：Δ 批次落地说明与验收清单
- [`docs/increment-template.md`](docs/increment-template.md)：三级证据标记模板与评审检查表
- [`docs/mcp.md`](docs/mcp.md)：MCP 只读接入
- [`banned-phrases.txt`](banned-phrases.txt)：全站禁词表（CI 扫描）
