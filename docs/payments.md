# 海外收款接入说明（Airwallex / PayPal）

GeoLens 的收款走供应商**托管收银台**：卡号、钱包授权都在第三方页面完成，本服务只保存订单号与会话 ID，不接触支付凭据。

## 一、代码结构

| 文件 | 职责 |
|---|---|
| `internal/billing/providers.go` | `HostedGateway` 端口、Airwallex / PayPal / Sandbox 实现、`ProviderRegistry` |
| `internal/billing/engine.go` | 订单状态机、会话保存、`ApplyWebhook` 幂等开通与自动开票 |
| `api/handlers/platform_handler.go` | 支付方式目录、下单、订单查询、买家回跳、匿名回调验签 |
| `frontend/src/routes/subscribe.tsx` | 支付方式选择、打开收银台、回跳确认与订单轮询 |
| `internal/billing/providers_test.go` | httptest 模拟两家供应商的下单 / 验签 / 捕获 |
| `api/handlers/payment_handler_test.go` | HTTP 级鉴权、降级、旧流程兼容、越权与验签回归 |

所有外部调用只用 Go 标准库 `net/http`，没有引入支付 SDK 依赖。HTTP 客户端可注入，测试全部指向本地 `httptest`，**不发起任何真实支付请求**。

## 二、环境变量

凭证按供应商成对启用；缺任意一个，该供应商就不会出现在 `/api/v1/billing/providers` 里。

| 变量 | 说明 |
|---|---|
| `AIRWALLEX_CLIENT_ID` | 空中云汇 `x-client-id` |
| `AIRWALLEX_API_KEY` | 空中云汇 `x-api-key` |
| `AIRWALLEX_API_BASE` | 默认 `https://api-demo.airwallex.com`；生产 `https://api.airwallex.com` |
| `AIRWALLEX_WEBHOOK_SECRET` | Webhook HMAC-SHA256 密钥；缺失时所有回调一律拒绝 |
| `PAYPAL_CLIENT_ID` / `PAYPAL_CLIENT_SECRET` | PayPal REST 应用凭证 |
| `PAYPAL_API_BASE` | 默认 `https://api-m.sandbox.paypal.com`；生产 `https://api-m.paypal.com` |
| `PAYPAL_WEBHOOK_ID` | 商户后台创建的 Webhook ID；缺失时所有回调一律拒绝 |

未配置凭证时，`ProviderRegistry` 只注册 `sandbox`，前端显示「演示收银台 · 不扣款」，点击后立即开通，不跳转第三方。

## 三、下单时序

```
用户在 /subscribe 选套餐与付款方式
  → POST /api/v1/billing/checkout  { plan, kind, provider }   （需登录）
      账户身份只从会话取，body 里的 account_id 一律忽略
  → Engine.Checkout 创建 pending 订单（免费层直接 succeeded）
  → Registry.CreateSession 调用供应商创建会话
      Airwallex: POST /api/v1/authentication/login → /api/v1/pa/payment_links/create
      PayPal:    POST /v1/oauth2/token → /v2/checkout/orders
  → 返回 { order, next, checkout_url, session_id }
      next=redirect → 前端打开托管收银台
      next=pay      → 演示网关沿用 /api/v1/billing/pay
      next=done     → 免费层，直接进入完成页
```

回跳地址由 `publicBaseURL()` 依据浏览器 `Origin`（或 `X-Forwarded-Proto` + `Host`）构造：

- 成功：`/subscribe?order=<id>&payment=return`
- 取消：`/subscribe?order=<id>&payment=cancel`

回跳后 `POST /api/v1/billing/orders/:id/return`：PayPal 在此调用 `/v2/checkout/orders/{id}/capture` 完成扣款，Airwallex 已在托管页扣款，两者都通过 `ApplyWebhook` 幂等开通权益。前端同时每 2.5 秒轮询 `GET /api/v1/billing/orders/:id`，覆盖 Webhook 比页面回跳稍晚的情况。

## 四、回调与验签

回调端点 `POST /api/v1/billing/webhook/{provider}` 是**匿名**的（供应商无法携带用户会话），因此验签是唯一入口，失败一律返回 401，绝不据 body 改订单。

- **Airwallex**：校验 `x-timestamp` + `x-signature`，签名是 `HMAC-SHA256(webhook_secret, timestamp + raw_body)` 的 hex。同时拒绝 5 分钟外的重放。事件里取 `merchant_order_id`（或 `metadata.order_id`）定位订单。
- **PayPal**：把 `PAYPAL-TRANSMISSION-ID`、`PAYPAL-TRANSMISSION-TIME`、`PAYPAL-TRANSMISSION-SIG`、`PAYPAL-CERT-URL`、`PAYPAL-AUTH-ALGO` 与原始事件一起提交给 `/v1/notifications/verify-webhook-signature`，只有 `verification_status=SUCCESS` 才继续。事件里取 `purchase_units[0].reference_id` 定位订单。
- **Sandbox**：不接受任何外部回调。

`Engine.ApplyWebhook` 用 `processedEvents` 做事件级幂等，并对已成功订单做终态保护：Webhook 与回跳重复投递不会重复开通或重复开票。对账脚本 `Reconcile()` 可补齐漏单。

## 五、上线前核对清单

[事实] 供应商下单、验签、捕获、降级、幂等全部有自动化测试：`internal/billing/providers_test.go` 与 `api/handlers/payment_handler_test.go`，CI 里随 `go test ./...` 执行。

[观察] 测试用 `httptest` 模拟供应商响应，仓库内没有真实商户凭证，也没有对 Airwallex / PayPal 生产环境发起过请求。证据来自哪一层：本地测试层。

[推导] 两家供应商的 REST 路径与字段名按公开 API 文档实现，但商户账号开通的产品能力、API 版本、Webhook 事件名可能不同。上线前必须在商户后台核对以下项，必要时调整 `providers.go` 里的路径与解析字段。此为概率性工程判断，风险提示：路径不匹配会导致下单 502，事件名不匹配会导致订单一直停在 pending；两者都不产生错误扣款，但会影响开通。来源：`internal/billing/providers.go`。

- [ ] Airwallex 账号已开通 Payment Links，`/api/v1/pa/payment_links/create` 返回的 URL 字段名（`url` / `payment_link_url` / `link`，代码已做容错）
- [ ] Airwallex Webhook 事件名包含 `succeeded` 或状态字段为 `SUCCEEDED`，并带 `merchant_order_id`
- [ ] Airwallex Webhook 签名头确认为 `x-timestamp` + `x-signature`；若商户后台提供不同头名，需同步调整
- [ ] PayPal 应用已开通 Checkout Orders v2，Webhook 已订阅 `PAYMENT.CAPTURE.COMPLETED`
- [ ] `PAYPAL_WEBHOOK_ID` 填写的是后台 Webhook 的 ID，不是 Client ID
- [ ] 生产域名使用 `api.airwallex.com` 与 `api-m.paypal.com`，沙箱凭证不要混用
- [ ] 用真实小额订单走一遍：下单 → 托管页付款 → 回跳 → 权益开通 → 重复回调不重复开票

## 六、安全边界

- 账户身份只来自登录会话；`checkout`、`pay`、`orders/:id`、`orders/:id/return` 都校验订单归属，防止代付或越权查询。
- 匿名回调必须先验签；签名密钥缺失时 fail-closed。
- 供应商请求统一 10 秒超时，响应体限制 1 MiB。
- 订单创建失败时保持 `pending` 且不产生任何权益，可换支付方式重试。
- 演示网关不产生真实扣款，界面明确标注，避免被误认为已接入真实收款。
