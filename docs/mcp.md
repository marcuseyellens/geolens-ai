# AnswerLens MCP 接入说明

MCP server 名称 `answerlens`，版本 `1.0.0`。端点 `POST /mcp`，采用 JSON-RPC 2.0。

## 只读令牌

所有请求都需要 `Authorization: Bearer <agent_token>`。令牌只读，仅允许两种 scope：

- `visibility:read` → `query_visibility`
- `evidence:read` → `get_evidence`

非法、撤销或缺少 scope 的令牌会被拒绝（HTTP 401）。任何写方法返回错误，不会落库。
生产环境用管理员端点签发：

```bash
curl -X POST http://localhost:8080/api/v1/mcp/tokens \
  -H "Content-Type: application/json" \
  -H "X-Admin-Token: $GEOLENS_ADMIN_TOKEN" \
  -d '{"owner":"claude-code","scopes":["visibility:read","evidence:read"]}'
```

本地零配置启动时，服务端会在控制台打印一个只读 `MCP read-only agent token`，可直接用于验证。

## 两个工具

### `query_visibility`

入参：`brand`（必填）、`market`、`model`、`region`。

返回逐模型的可见性摘要：三态 `mentioned / recommended / linked` 分列，每个比例字段伴随
`n_valid` 与 `provenance`。刻意不提供综合 AI 分数。当前检测结果没有 recommend/link 的采集
来源时，这两态显示为 0 并在 `sample_note` 里说明，不按推断填充。

```json
{
  "jsonrpc": "2.0", "id": 1, "method": "tools/call",
  "params": {
    "name": "query_visibility",
    "arguments": {"brand": "<brand_id>", "market": "AEO 平台", "region": "us"}
  }
}
```

### `get_evidence`

入参：`run_id`。返回该次运行的原始答案文本与引用 URL，用于两步下钻取证。

```json
{
  "jsonrpc": "2.0", "id": 2, "method": "tools/call",
  "params": {"name": "get_evidence", "arguments": {"run_id": "<result_id>"}}
}
```

`tools/list` 只返回这两个工具。不提供导出、告警订阅或写工具。

## Claude Code 内两步自查（A-WP-4.3）

1. 配置 MCP server（URL 指向部署地址，Header 带只读令牌），然后让 Claude Code 调用
   `query_visibility` 查 AnswerLens 自身的可见性，确认输出是逐模型三态、带 `n_valid`、
   没有综合分。
2. 从结果里取一个 `run_id`，调用 `get_evidence`，确认能拿到原始答案文本与引用 URL。

两步都在 MCP 内部完成，不需要打开控制台。若第 1 步返回「暂无可见性数据」，先在控制台
对目标品牌跑一次检测生成结果，再重试；不得用示例数据冒充自查结果。

## 口径与边界

- 指标只代表本次采样，不代表任何外部平台规则或效果承诺。
- 阈值、方法论版本 `1.0.0` 与 `n_valid` 随响应返回，便于核对历史报告是否被新口径改写。
- 本仓库的 MCP 数据源桥接已落库的真实检测结果；本地模拟采集产物会明确标注 `simulated`。
