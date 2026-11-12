# GeoLens AI

> 把「被大模型推荐」做成可优化指标（AEO/GEO）

GeoLens AI 是一个 AI Engine Optimization（AEO）/ Generative Engine Optimization（GEO）平台。它检测你的品牌在各大语言模型（ChatGPT、Claude、Gemini、Perplexity）中的推荐表现，将「被 AI 推荐」从不可控的黑盒变成可量化、可追踪、可优化的指标。

## 核心功能

- **多模型检测**：同时向 ChatGPT、Claude、Gemini、Perplexity 发送提问，获取回答
- **品牌提及分析**：检测品牌是否被提及、在回答中的位置（第1位/前3/前5/提及/未提及）
- **情感分析**：判断品牌提及的情感倾向（正面/中性/负面）
- **竞品追踪**：检测回答中哪些竞品被提及
- **AEO 综合评分**：基于位置和情感计算 0-100 的综合得分
- **可视化报告**：柱状图对比各模型提及率、雷达图展示得分分布
- **趋势追踪**：持续检测，观察品牌在 AI 模型中的推荐变化

## 技术栈

| 层 | 技术 | 说明 |
|---|---|---|
| 后端 | Go + Gin | DDD + 六边形架构 |
| 前端 | React + Vite + Tailwind CSS | 自建设计系统 |
| 图表 | Recharts | 柱状图、雷达图 |
| 存储 | JSON 文件 | 零配置，开箱即用 |
| AI 模型 | OpenAI / Anthropic / Google / Perplexity | 多模型适配器 |

## 架构设计

借鉴了两个参考项目：
- **withHuman** 的 DDD + 六边形架构（Ports & Adapters）和自建设计系统模式
- **CopilotKit** 的多 LLM Provider 适配器模式

```
geolens/
├── cmd/server/          # 主程序入口
├── domain/              # 领域模型（Brand、Query、Result、Report）
├── application/         # 应用服务 + 仓储接口（Ports）
├── internal/
│   ├── aimodel/         # AI 模型适配器（OpenAI/Anthropic/Gemini/Perplexity）
│   └── storage/         # JSON 文件存储实现（Adapter）
├── api/handlers/        # HTTP API 处理器（Gin）
└── frontend/            # React 前端
    └── src/
        ├── ds/          # 自建设计系统（tokens + 组件）
        ├── routes/      # 页面（Dashboard、Brands、BrandDetail、Settings）
        ├── api.ts       # API 客户端
        └── types.ts     # TypeScript 类型
```

## 快速开始

### 1. 构建

```bash
./dev.sh
```

或手动：
```bash
# 前端
cd frontend && npm install && npm run build && cd ..

# 后端
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go build -o geolens ./cmd/server/

# 启动
GEOLENS_FRONTEND_DIST=./frontend/dist ./geolens
```

### 2. 零配置体验（演示模式）

未设置任何 API Key 时，服务自动进入演示模式：用模拟回答跑通完整的分析、评分与报告链路，可直接查看提及率、位置分布、情感分析和图表。页面会显示「演示模式」提示。

也可用 `GEOLENS_DEMO=1` 在已配置 Key 时强制启用。

### 3. 配置 AI 模型 API Key（真实检测）

在启动前设置环境变量：

```bash
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export GEMINI_API_KEY=AIza...
export PERPLEXITY_API_KEY=pplx-...

./geolens
```

只检测已配置的模型，未配置的会跳过，不影响其他模型运行。配置任意一个 Key 后即自动退出演示模式。

### 4. 使用

打开 http://localhost:8080

1. 在「品牌」页面添加你的品牌和竞品
2. 在品牌详情页添加检测问题（或生成推荐问题）
3. 点击「运行检测」，系统将向所有已配置的 AI 模型发送提问
4. 查看检测报告：提及率、位置分布、情感分析、AEO 得分

## AEO 得分计算

得分 = 位置权重 × 情感系数

| 位置 | 权重 |
|---|---|
| 第1位 | 100 |
| 前3位 | 75 |
| 前5位 | 50 |
| 被提及 | 25 |
| 未提及 | 0 |

| 情感 | 系数 |
|---|---|
| 正面 | 1.0 |
| 中性 | 0.8 |
| 负面 | 0.5 |

## API 参考

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/brands` | 列出所有品牌 |
| POST | `/api/v1/brands` | 创建品牌 |
| GET | `/api/v1/brands/:id` | 获取品牌详情 |
| PUT | `/api/v1/brands/:id` | 更新品牌 |
| DELETE | `/api/v1/brands/:id` | 删除品牌 |
| GET | `/api/v1/brands/:id/queries` | 列出检测问题 |
| POST | `/api/v1/brands/:id/queries` | 创建检测问题 |
| POST | `/api/v1/brands/:id/queries/:qid/detect` | 运行检测 |
| GET | `/api/v1/brands/:id/report` | 获取检测报告 |
| GET | `/api/v1/brands/:id/results` | 列出检测结果 |
| DELETE | `/api/v1/brands/:id/queries/:qid` | 删除检测问题 |
| GET | `/api/v1/health` | 健康检查（含 `demo` 状态） |
| GET | `/api/v1/models` | 列出 AI 模型状态（含 `demo` 状态） |

## 开发模式（前后端分离）

```bash
# 终端1：前端 dev server
cd frontend && npm run dev

# 终端2：后端
go run ./cmd/server/
```

前端运行在 http://localhost:5173，API 请求自动代理到 http://localhost:8080。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | HTTP 服务端口 |
| `GEOLENS_DB` | `~/.geolens/geolens.json` | 数据文件路径 |
| `GEOLENS_FRONTEND_DIST` | `./frontend/dist` | 前端构建产物目录 |
| `GEOLENS_DEMO` | 未设置 | 设为 `1` 强制启用演示模式 |
| `OPENAI_API_KEY` 等 | 未设置 | 各模型 API Key；全部为空时自动进入演示模式 |

## 数据存储

数据存储在 `~/.geolens/geolens.json`，可通过 `GEOLENS_DB` 环境变量修改路径。删除该文件即可清空所有数据。
