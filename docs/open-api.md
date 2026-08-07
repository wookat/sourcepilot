# 开放 API（只读）

R152 起，TradeMind 对外提供只读 REST API：`GET /api/open/v1/*`，供第三方系统（ERP 对账、BI 报表、自动化脚本等）以编程方式查询订单、库存、经营摘要与异常待办。规范文件见 [`docs/openapi/open-api.v1.json`](openapi/open-api.v1.json)（OpenAPI 3，契约测试保证与实现同步）。

## 与 MCP 的关系（token 体系取舍）

开放 API 与 MCP 只读入口（`POST /api/mcp`，见 [`docs/mcp.md`](mcp.md)）**复用同一 token 治理基建**：同一张 `mcp_api_tokens` 表、同一 SHA-256 哈希校验、同一过期/吊销/上限（每租户 20 个活跃 token）机制、同一审计表和同款三层限流。

区分方式是 token 的 **用途（purpose）** 字段，而不是再造一套 token 体系：

| purpose | `POST /api/mcp` | `GET /api/open/v1/*` |
| --- | --- | --- |
| `mcp`（默认，存量 token 均为此值） | ✅ | ❌ |
| `openapi` | ❌ | ✅ |
| `both` | ✅ | ✅ |

取舍说明：`scope` 保持单一权限轴（`readonly`，两个入口都只读），`purpose` 只做入口选择器。这样存量 MCP token 的攻击面不会因为新入口上线而变宽（默认 `mcp` 不能调开放 API），同时管理页、审计、限流等治理能力零重复建设。用途不符的 token 与未知 token 返回完全一致（401），不泄露 token 是否存在。

## 鉴权

在管理后台「设置 → 只读 API 接入」创建 token（用途选「开放 API」或「MCP + 开放 API」），明文仅创建时展示一次。请求携带：

```text
Authorization: Bearer sp_mcp_ro_...
```

- token 校验：SHA-256 哈希匹配 + 未吊销 + 未过期 + `scope=readonly` + 用途为 `openapi`/`both`；任何一项不满足统一 401（业务码 `40101`）。
- 所有查询强制租户隔离：token 绑定租户，只能读到本租户数据；跨租户订单号返回 404（业务码 `40401`），与不存在的订单不可区分。
- 输出脱敏：客户名仅保留首字符（如 `张**`）；不返回内部 UUID、客户邮箱/电话、平台原始报文（rawData）、任何密钥/凭证字段。
- 只读语义：入口只注册 GET 路由，POST/PUT/PATCH/DELETE 一律 404。

## 端点

| 端点 | 说明 |
| --- | --- |
| `GET /api/open/v1/orders` | 订单列表；query：`status`、`paymentStatus`、`platform`、`keyword`（订单号模糊）、`startDate`/`endDate`（YYYY-MM-DD）、`page`、`pageSize` |
| `GET /api/open/v1/orders/{orderNo}` | 订单详情（含行项目），按订单号查询 |
| `GET /api/open/v1/inventory` | SKU 库存；query：`keyword`、`lowStockOnly`、`page`、`pageSize` |
| `GET /api/open/v1/reports/summary` | 经营摘要：订单量、已支付订单量、按币种已支付销售额（不做汇率折算）、未处理异常数、低库存 SKU 数；query：`startDate`/`endDate`（默认近 30 天） |
| `GET /api/open/v1/exceptions` | 异常待办；query：`exceptionType`、`severity`、`page`、`pageSize` |

分页沿全站惯例：`page` 从 1 起，`pageSize` 默认 20、最大 100；响应 `data` 内为 `list` + `total`（异常另有 `totalOpen`）。

## curl 示例

```bash
TOKEN="sp_mcp_ro_..."   # 用途为 openapi/both 的 token
BASE="http://localhost:8080"

# 订单列表（已支付、近 7 天、第一页）
curl -s -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/open/v1/orders?paymentStatus=paid&startDate=2026-08-01&page=1&pageSize=20"

# 订单详情
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/open/v1/orders/ORD20260801001"

# 低库存 SKU
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/open/v1/inventory?lowStockOnly=true"

# 经营摘要（默认近 30 天）
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/open/v1/reports/summary"

# 未处理异常待办
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/open/v1/exceptions?severity=error"
```

成功响应为全站统一 envelope：

```json
{ "code": 0, "message": "ok", "data": { "list": [], "total": 0 } }
```

## 错误码表

| HTTP | 业务码 | 含义 |
| --- | --- | --- |
| 400 | 40001 | 参数非法（如日期格式不是 YYYY-MM-DD） |
| 401 | 40101 | 缺失 / 非法 / 过期 / 已吊销 / 用途不符的 token |
| 404 | 40401 | 订单不存在或不属于当前租户 |
| 429 | 42901 | 超出限流，响应带 `Retry-After` 头 |
| 500 | 50000 | 服务内部错误（不泄露内部细节） |

## 限流

沿 MCP 口径的三层限流（桶与 MCP 入口相互独立，policy 前缀 `openapi_readonly*`）：

1. **每 token**：`OPENAPI_RATE_RPS`（默认 5 req/s）+ `OPENAPI_RATE_BURST`（默认突发 10）。
2. **每租户聚合**：token 额度的 2 倍，防止多 token 放大。
3. **每 IP 鉴权失败预算**：1 req/s、突发 10，仅对失败请求计费，抑制暴力猜 token。

超限返回 `429` + 业务码 `42901`。Redis 可用时限流状态走 Redis（多副本共享额度），不可用时降级进程内限流。

## 审计

每次端点调用记录一条审计（与 MCP 工具调用同一日志表，工具名形如 `openapi:orders_list`）：租户、token（脱敏）、端点、结果状态、耗时。**不记录**查询参数与响应内容。审计为 best effort，写入失败不影响查询本身。

## 配置

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `OPENAPI_ENABLED` | `true` | 开放 API 入口开关 |
| `OPENAPI_RATE_RPS` | `5` | 每 token 持续速率 |
| `OPENAPI_RATE_BURST` | `10` | 每 token 突发上限 |
