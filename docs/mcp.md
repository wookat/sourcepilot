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
2. 点击「创建只读 token」，输入用途名称（如 `claude-desktop`）。
3. **明文 token 只在创建时展示一次**，请立即复制保存；数据库中只保存 SHA-256 哈希。
4. 列表中 token 以脱敏形式展示（如 `sp_mcp_ro_ab…cdef`），可随时吊销；创建与吊销都会写入操作日志。

安全性质：

- token 与创建它的租户绑定，所有工具查询强制该租户范围；涉及店铺的数据只会返回该租户名下店铺的数据。
- token 只有 `readonly` scope，不能调用任何写接口。
- 每个 token 独立限流（默认 5 req/s、突发 10）。
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
| `429 rate limit exceeded` | 触发每 token 限流，稍后重试或调大 `MCP_RATE_RPS` / `MCP_RATE_BURST` |
| `404`（入口不存在） | `MCP_ENABLED=false` 时入口不注册，确认环境变量后重启 backend |
