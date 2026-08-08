# MCP 入口（只读 + 受控写白名单）

TradeMind 提供一个符合 MCP（Model Context Protocol）标准的 server 入口，让 Claude 等 MCP 客户端可以直接查询订单、库存、经营摘要与异常待办。默认只暴露查询工具；R179 起新增一个**默认全关**的受控写白名单（首个动作：订单打标），须三层闸门同时打开并通过 dry-run→确认 token→执行流程才可写，见下方「[写白名单（write:ops）](#写白名单writeops)」。**消息发送 / 任何外部平台动作永远不在 MCP 面上（绝不自动外发红线）。**

> 说明：这是 TradeMind 自建的 MCP server，属于本系统的对外查询入口；它不代表任何第三方电商平台的 API 已经接通。

## 端点与协议

- 端点：`POST /api/mcp`（与主 API 同域，如 `http://localhost:8080/api/mcp`）
- 传输：MCP Streamable HTTP（stateless、JSON response），实现基于官方 Go SDK `github.com/modelcontextprotocol/go-sdk`
- 鉴权：`Authorization: Bearer <sp_mcp_ro_...>`，token 为租户级 API token（默认 `readonly` scope；写白名单需 `write:ops` scope）
- 开关与限流：`MCP_ENABLED`、`MCP_RATE_RPS`、`MCP_RATE_BURST`、`MCP_WRITE_ENABLED`（见 `docs/env.md`）

## 获取只读 token

1. 登录管理后台，进入「系统设置 → MCP 只读接入」（`/settings/mcp-tokens`）。
2. 点击「创建只读 token」，输入用途名称（如 `claude-desktop`），选择 token 用途（R152 起：「MCP 只读」（默认）/「开放 API」/「MCP + 开放 API」；MCP 入口只接受 `mcp`/`both`，开放 API 入口（`GET /api/open/v1/*`，见 `docs/open-api.md`）只接受 `openapi`/`both`，存量 token 均为 MCP 用途），可选设置有效期（7/30/90/180/365 天，默认不过期）。
3. **明文 token 只在创建时展示一次**，请立即复制保存；数据库中只保存 SHA-256 哈希。
4. 列表中 token 以脱敏形式展示（如 `sp_mcp_ro_ab…cdef`），可随时吊销；创建与吊销都会写入操作日志。

安全性质：

- token 与创建它的租户绑定，所有工具查询强制该租户范围；涉及店铺的数据只会返回该租户名下店铺的数据。
- scope 为独立权限轴（R179）：`readonly` 授予查询工具，`write:ops` 只授予写白名单工具，两者互不放宽。存量 / 默认 token 均为 `readonly`，看不到也调不到任何写工具；`write:ops` token 不含 `readonly` 时看不到查询工具，也永远不能访问开放 API（`/api/open/v1/*` 仅认 `readonly`）。scope 列为空 / 未知值的 token 行一律按无效处理（fail-closed）。
- 租户被禁用 / 清退后，其名下所有 token 立即失效（鉴权时校验租户状态，返回 401，与未知 token 不可区分），无需逐个吊销。
- 每来源 IP 的鉴权失败预算按客户端 IP 计费：默认取 TCP peer 地址；仅当 `TRUSTED_PROXIES` 列出当前代理时才采信 `X-Forwarded-For`（见 [`docs/env.md`](env.md)）。部署在 nginx / LB 之后需正确配置，否则所有请求按代理 IP 共享同一预算。
- token 可选设置有效期（创建时 `expiresInDays`，1-730 天；默认不过期保持兼容）：到期后鉴权即拒绝（401）；管理页展示过期状态与即将过期（≤7 天）提示。也可随时显式吊销（吊销即时生效）。请按用途拆分 token，不再使用时及时吊销。
- 每个租户最多同时持有 20 个未吊销 token（超出时创建返回 400，需先吊销），因为每个 token 自带限流桶。
- 三层限流：每 token（默认 5 req/s、突发 10）、每租户聚合（token 额度的 2 倍，防止多 token 叠加绕过）、每来源 IP 的鉴权失败预算（1 req/s、突发 10，仅失败请求计费，合法流量不受影响）。超限统一 `429`，envelope `code=42901`。
- 限流状态存储：Redis 可用时（复用队列 `REDIS_URL`，无新依赖 / 新变量）三层限流桶统一走 Redis（Lua 令牌桶，key 前缀 `ratelimit:mcp_readonly*`），多副本部署共享同一份额度；Redis 不可用或调用失败时自动降级为进程内令牌桶（不会 fail-open），此时多副本下总额度会按副本数放大，需相应调低 `MCP_RATE_RPS` / `MCP_RATE_BURST`。
- 工具调用逐次审计：每次 `tools/call` 落一条审计日志（租户、token（脱敏）、工具名、时间、成功/失败、耗时），**不记录查询参数与查询结果内容**；管理页「MCP 只读接入」可按工具/状态筛选查看（`GET /api/v1/mcp/audit-logs`）。审计写入为 fail-closed：审计行写入失败时该次工具调用会被拒绝（JSON-RPC error code `-32603` internal error，错误信息 `audit log unavailable, tool call rejected`），保证没有任何调用绕过审计；工具均为只读、无副作用，被拒的调用可安全重试。后端同时记录 Error 级日志 `mcp_tool_audit_write_failed` 供告警链路发现审计库故障。
- 入口级拒绝也留痕（R154）：未通过鉴权（401）与被限流（429）的请求写 `mcp:auth` 审计行（状态 `auth_failed` / `rate_limited`；未认证来源记在租户 0 下），按「工具+状态+来源」每分钟至多一条，防止攻击流量放大审计表；不记录任何 token 内容。该写入为 best effort，不阻断 401/429 响应本身（与开放 API 入口同口径）；fail-closed 只作用于 `tools/call` 的逐次审计。
- 输出经过脱敏：不含密钥、密码、内部 UUID，客户姓名只保留首字符。

## 写白名单（write:ops）

R179 起 MCP 提供极小的受控写面。设计口径：D1 P0 最小动作集、D2 独立 `write:ops` scope、D3 三层闸门默认全关 + 强制 dry-run→确认 token→执行、D4 fail-closed 审计 + 限额。

### 三层闸门（默认全关，逐层独立 403）

| 层 | 开关 | 默认 | 生效方式 |
| --- | --- | --- | --- |
| 全局 | 环境变量 `MCP_WRITE_ENABLED` | `false` | 关闭时写工具完全不注册（列表里都看不到） |
| 租户 | 设置项 `mcp` 组 `write_enabled=true`（仅管理员可改设置） | 关 | 关闭 / 读取失败时每次写调用被拒（fail-closed） |
| token | token scope 含 `write:ops` | 无 | 无该 scope 的 token 看不到写工具 |

### write:ops token 治理（管理员专属）

- 仅 `admin` 角色可创建带 `write:ops` 的 token（operator / readonly 返回 403）；scope 只能在创建时授予，**没有任何升级已有 token 的入口**。
- 写 token **必须过期**：默认 30 天，最长 90 天，不支持不过期。
- 吊销、租户禁用、到期语义与只读 token 完全一致。

### dry-run → 确认 token → 执行

每个写工具入参必须带 `mode`：

1. `mode=dry_run`：校验目标存在性 / 租户归属 / 限额，返回结构化影响预览（`preview`）、人话摘要（`summary`）和一次性 `confirmationToken`（TTL 5 分钟）。
2. `mode=execute` + `confirmationToken`：确认 token 与「租户 + 调用 token + 工具名 + 参数哈希」四元绑定，原子消费（单次有效）。缺失 / 过期 / 已使用 / 换调用者 / 参数漂移一律拒绝，需重新 dry_run。执行成功后重放同一确认 token 返回 `alreadyExecuted=true` 且**不会重复变更**。

数据库中只保存确认 token 的 SHA-256；明文只在 dry_run 响应中出现一次，不进审计。

### fail-closed 审计与限额

- 每次写调用（dry_run 与 execute 各一行）写入审计：租户、token（脱敏）、工具、mode、白名单化参数摘要（订单号 / 标签名等业务键，绝不含密钥或自由文本）、结果摘要、确认哈希、耗时。execute 的业务变更与审计行在**同一事务**提交：审计写不进去则整个执行回滚（`audit log unavailable, tool call rejected`）。
- 限额（按成功 execute 审计行计数，计数失败即拒绝）：每 token 30 次/小时、每租户 200 次/天；超限拒绝 dry_run 与 execute。
- 跨租户目标（订单号 / 标签名不属于本租户）统一返回「不存在」，与真正不存在不可区分（404 语义，无存在性探测）。
- 每次调用单目标对象（一个订单 + 一个标签），无批量入口。

### 写工具列表（P0）

| 工具 | 说明 |
| --- | --- |
| `orders_add_tag` | 为一个订单添加一个**已存在**的租户标签（幂等：已有该标签时为无操作，`applied=0`） |
| `orders_remove_tag` | 移除一个订单上的一个标签（幂等：本就没有时为无操作，`removed=0`） |
| `exceptions_mark` | 异常标记（R180 W2）：`action` 为 `handle`（标记已处理）/ `ignore`（标记已忽略）/ `unmark`（清除标记）。幂等：重复同向标记不产生新行，`handle`↔`ignore` 互斥切换，`unmark` 无标记时为无操作。目标为 `sourceType` + `sourceId`（订单 / 订单项等），跨租户 / 不存在统一「记录不存在」 |
| `procurement_mark_placed` | 采购单 mark-placed（R180 W2）：回填 1688 外部单号，走既有状态机 `placing → placed`，非法状态在 dry_run 即拒绝；跨租户 / 不存在统一「采购单不存在」 |
| `procurement_fill_logistics` | 物流运单号回填（R180 W2）：`paid → shipped` 并创建物流记录（运单号 + 承运商）；同一状态机 / 404 口径 |
| `procurement_mark_paid` | 采购单支付登记（R181 W3）：`placed → paid` 人工付款回填，**不动真实资金**，仅登记操作员已线下支付的事实。除 W1 全套管道外附加金额前提，见下方「mark-paid 金额前提」 |

### mark-paid 金额前提（R181 W3，fail-closed）

`procurement_mark_paid` 在 W1 管道之上强制三项前提，任一不满足即拒绝（403 语义）并写审计行：

1. **租户金额上限必须先配置**：设置项 `mcp` 组 `mark_paid_single_limit`（单笔上限）与 `mark_paid_daily_limit`（日累计上限），仅管理员可改。缺失、非数字、`0` 或负数一律视为**未配置 = 工具不可用**（默认关）。
2. **dry-run 预览回显金额与订单明细**：预览含采购单 ID、供应商、当前/目标状态、金额、币种、支付渠道、两项上限、当日已用额度，以及全部明细行（商品名 / SKU / 数量 / 单价），供人工确认后再 execute。
3. **金额 / 币种必须与采购单完全一致**：入参 `amount`（必填，>0、至多两位小数，按“分”精确比较，无浮点误差豁免）与 `currency`（必填，大小写不敏感）与采购单 `totalAmount` / `currency` 任一不符即拒绝；0 / 负数 / 超两位小数 / 超大金额直接判非法入参。

日累计额度按当日成功 execute 审计行的金额求和（与次数限额同一条 fail-closed 审计链），并且在 **execute 事务内**再次校验——dry_run 时领取的确认 token 不能绕过其后被占满的额度。求和失败即拒绝。金额同时落入审计行 `amount` 字段，供后台审计列表与对账使用。

### 后台治理 UI（R180 W2）

管理页「MCP 只读接入」新增管理员专属的「MCP 写白名单」卡片（operator / readonly 完全不可见）：

- 租户级 `mcp / write_enabled` 开关：默认关，开启前弹风险确认（说明三层闸门与全局 `MCP_WRITE_ENABLED` 的关系）。
- 写 token 创建 / 吊销：独立列表，只列 `write:ops` token；明文仅创建时展示一次；默认 30 天 / 最长 90 天。后端同步收紧：非 admin 的 token 列表**看不到**写 token，非 admin 吊销写 token 返回 404（不可见即不可操作）。
- 审计列表补充展示写管道字段：`mode`（dry_run / execute）、`paramsSummary`（白名单参数摘要）、`confirmHash`（确认 token 绑定哈希），读工具行为空。

## 只读工具列表

| 工具 | 说明 |
| --- | --- |
| `orders_query` | 订单列表查询：按状态 / 支付状态 / 平台 / 关键词 / 创建日期范围过滤，返回订单号、状态、金额（原币种）、脱敏客户名等摘要 |
| `inventory_query` | SKU 库存查询：按关键词过滤，支持仅看低库存，返回 SKU 编码、名称、库存与预警线 |
| `report_summary` | 经营摘要：时间范围内订单量、已支付订单量、按币种已支付销售额（不做汇率折算），以及当前未处理异常数与低库存 SKU 数 |
| `exceptions_pending` | 异常待办：SKU 未匹配、库存不足、同步失败等，返回异常类型、级别、关联订单号与建议动作 |

### 分页参数口径（与开放 API 的差异）

MCP 工具的 `page` / `pageSize` 为**钳制语义**：`page < 1` 归一为 1，`pageSize < 1` 归一为默认 20，`pageSize > 100` 截断为 100，不会因分页参数越界返回错误。这与开放 API（`GET /api/open/v1/*`）的口径**不同**——开放 API 对非正整数分页参数返回 `400`（`code=40001`），仅对 `pageSize > 100` 做截断（见 [`docs/open-api.md`](open-api.md)）。差异原因：MCP 参数由 LLM 客户端生成，钳制语义可减少一轮工具报错重试；开放 API 面向程序化调用方，显式 400 更利于尽早暴露集成错误。编写同时对接两个入口的客户端时请勿假设两者报错行为一致。

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
