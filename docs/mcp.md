# MCP 只读入口

TradeMind 提供一个符合 MCP（Model Context Protocol）标准的只读 server 入口，让 Claude 等 MCP 客户端可以直接查询订单、库存、经营摘要与异常待办。入口只暴露查询工具，**不包含任何写操作**。

> 说明：这是 TradeMind 自建的 MCP server，属于本系统的对外查询入口；它不代表任何第三方电商平台的 API 已经接通。

## 端点与协议

- 端点：`POST /api/mcp`（与主 API 同域，如 `http://localhost:8080/api/mcp`）
- 传输：MCP Streamable HTTP（stateless、JSON response），实现基于官方 Go SDK `github.com/modelcontextprotocol/go-sdk`
- 鉴权：`Authorization: Bearer <sp_mcp_ro_...>`，token 为租户级只读 API token
- 开关与限流：`MCP_ENABLED`、`MCP_RATE_RPS`、`MCP_RATE_BURST`（见 `docs/env.md`）

## 获取只读 token

1. 登录管理后台，进入「系统设置 → MCP 只读接入」（`/settings/mcp-tokens`）。
2. 点击「创建只读 token」，输入用途名称（如 `claude-desktop`），选择 token 用途（R152 起：「MCP 只读」（默认）/「开放 API」/「MCP + 开放 API」；MCP 入口只接受 `mcp`/`both`，开放 API 入口（`GET /api/open/v1/*`，见 `docs/open-api.md`）只接受 `openapi`/`both`，存量 token 均为 MCP 用途），可选设置有效期（7/30/90/180/365 天，默认不过期）。
3. **明文 token 只在创建时展示一次**，请立即复制保存；数据库中只保存 SHA-256 哈希。
4. 列表中 token 以脱敏形式展示（如 `sp_mcp_ro_ab…cdef`），可随时吊销；创建与吊销都会写入操作日志。

安全性质：

- token 与创建它的租户绑定，所有工具查询强制该租户范围；涉及店铺的数据只会返回该租户名下店铺的数据。
- token 只有 `readonly` scope，不能调用任何写接口；鉴权时强制校验 scope，非 `readonly` 的 token 行一律按无效处理。
- 租户被禁用 / 清退后，其名下所有 token 立即失效（鉴权时校验租户状态，返回 401，与未知 token 不可区分），无需逐个吊销。
- 每来源 IP 的鉴权失败预算按客户端 IP 计费：默认取 TCP peer 地址；仅当 `TRUSTED_PROXIES` 列出当前代理时才采信 `X-Forwarded-For`（见 [`docs/env.md`](env.md)）。部署在 nginx / LB 之后需正确配置，否则所有请求按代理 IP 共享同一预算。
- token 可选设置有效期（创建时 `expiresInDays`，1-730 天；默认不过期保持兼容）：到期后鉴权即拒绝（401）；管理页展示过期状态与即将过期（≤7 天）提示。也可随时显式吊销（吊销即时生效）。请按用途拆分 token，不再使用时及时吊销。
- 每个租户最多同时持有 20 个未吊销 token（超出时创建返回 400，需先吊销），因为每个 token 自带限流桶。
- 三层限流：每 token（默认 5 req/s、突发 10）、每租户聚合（token 额度的 2 倍，防止多 token 叠加绕过）、每来源 IP 的鉴权失败预算（1 req/s、突发 10，仅失败请求计费，合法流量不受影响）。超限统一 `429`，envelope `code=42901`。
- 限流状态存储：Redis 可用时（复用队列 `REDIS_URL`，无新依赖 / 新变量）三层限流桶统一走 Redis（Lua 令牌桶，key 前缀 `ratelimit:mcp_readonly*`），多副本部署共享同一份额度；Redis 不可用或调用失败时自动降级为进程内令牌桶（不会 fail-open），此时多副本下总额度会按副本数放大，需相应调低 `MCP_RATE_RPS` / `MCP_RATE_BURST`。
- 工具调用逐次审计：每次 `tools/call` 落一条审计日志（租户、token（脱敏）、工具名、时间、成功/失败、耗时），**不记录查询参数与查询结果内容**；管理页「MCP 只读接入」可按工具/状态筛选查看（`GET /api/v1/mcp/audit-logs`）。审计写入为 fail-closed：审计行写入失败时该次工具调用会被拒绝（错误信息 `audit log unavailable, tool call rejected`），保证没有任何调用绕过审计；工具均为只读、无副作用，被拒的调用可安全重试。后端同时记录 Error 级日志 `mcp_tool_audit_write_failed` 供告警链路发现审计库故障。
- 入口级拒绝也留痕（R154）：未通过鉴权（401）与被限流（429）的请求写 `mcp:auth` 审计行（状态 `auth_failed` / `rate_limited`；未认证来源记在租户 0 下），按「工具+状态+来源」每分钟至多一条，防止攻击流量放大审计表；不记录任何 token 内容。该写入为 best effort，不阻断 401/429 响应本身（与开放 API 入口同口径）；fail-closed 只作用于 `tools/call` 的逐次审计。
- 输出经过脱敏：不含密钥、密码、内部 UUID，客户姓名只保留首字符。

## 只读工具列表

| 工具 | 说明 |
| --- | --- |
| `orders_query` | 订单列表查询：按状态 / 支付状态 / 平台 / 关键词 / 创建日期范围过滤，返回订单号、状态、金额（原币种）、脱敏客户名等摘要 |
| `inventory_query` | SKU 库存查询：按关键词过滤，支持仅看低库存，返回 SKU 编码、名称、库存与预警线 |
| `report_summary` | 经营摘要：时间范围内订单量、已支付订单量、按币种已支付销售额（不做汇率折算），以及当前未处理异常数与低库存 SKU 数 |
| `exceptions_pending` | 异常待办：SKU 未匹配、库存不足、同步失败等，返回异常类型、级别、关联订单号与建议动作 |

## 在 Claude Desktop / Claude Code 中配置

Claude Desktop（`claude_desktop_config.json`）或其他支持 Streamable HTTP 的客户端：

```json
{
  "mcpServers": {
    "trademind": {
      "type": "http",
      "url": "https://your-trademind-host/api/mcp",
      "headers": {
        "Authorization": "Bearer sp_mcp_ro_xxxxxxxx..."
      }
    }
  }
}
```

Claude Code：

```bash
claude mcp add --transport http trademind https://your-trademind-host/api/mcp \
  --header "Authorization: Bearer sp_mcp_ro_xxxxxxxx..."
```

## 用 MCP Inspector 本地验证

```bash
npx @modelcontextprotocol/inspector
```

在 Inspector 界面中选择 `Streamable HTTP`，URL 填 `http://localhost:8080/api/mcp`，并在 Authentication 中添加 Header `Authorization: Bearer sp_mcp_ro_...`，连接后即可在 Tools 页调用 `orders_query` 等工具。

## 故障排查

| 现象 | 处理 |
| --- | --- |
| `401 invalid or revoked token` | token 拼写错误、已吊销或未带 `Bearer` 前缀；到设置页重建 token |
| `429 rate limit exceeded`（`code=42901`） | 触发 token / 租户 / 鉴权失败任一限流；稍后重试或调大 `MCP_RATE_RPS` / `MCP_RATE_BURST`。连续使用无效 token 也会触发 |
| `400 活跃 token 数量已达上限` | 该租户未吊销 token 已达 20 个，先到设置页吊销不再使用的 token |
| `404`（入口不存在） | `MCP_ENABLED=false` 时入口不注册，确认环境变量后重启 backend |
| `audit log unavailable, tool call rejected` | 审计库写入失败，工具调用被 fail-closed 拒绝；检查数据库可用性与后端 `mcp_tool_audit_write_failed` 日志，故障恢复后重试即可（工具只读、重试无副作用）；此期间管理页「MCP 只读接入」的审计列表也可能加载失败（页面会展示错误提示与重试按钮） |
