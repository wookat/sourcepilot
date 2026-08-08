# R181 MCP 写白名单安全交叉审查（W1+W2+W3 全写面）

- 审查范围：`POST /api/mcp` 写白名单全部 6 个动作（W1：`orders_add_tag` / `orders_remove_tag`；W2：`exceptions_mark` / `procurement_mark_placed` / `procurement_fill_logistics`；W3：`procurement_mark_paid`）及其共用管道（`mcpwrite` 闸门 / 确认 / 限额、`mcpaudit` 审计、`mcptoken` 鉴权与 scope）。
- 视角：渗透测试视角（token 伪造 / 重放 / 跨 token 消费、闸门绕过、readonly / openapi token 越权写、跨租户写、审计缺记录、限额绕过、金额边界）。
- 方法：代码审读 + 攻击面逐项测试（`backend/internal/modules/mcpserver/r179_write_test.go`、`r180_write_test.go`、`r181_write_test.go`、`hardening_test.go`、`r176_purpose_test.go`、`backend/internal/securitytests/permmatrix`）。
- 结论：**未发现 P0 / P1**。W3 落地时按 fail-closed 设计消除了两处潜在缺口（见「设计期消除的风险」）。2 项 P2 / 建议级观察，见文末。

## 攻击面逐项结论

| # | 攻击项 | 结论 | 证据 |
| --- | --- | --- | --- |
| 1 | token 伪造 | 拒绝。库中只存 SHA-256，鉴权按哈希查找；伪造 / 猜测 token 与不存在 token 均 401 且不可区分；每来源 IP 鉴权失败预算（1 rps / 突发 10）限制爆破 | `mcptoken.Service.Authenticate`；`mcpserver/server.go` authFailLimiter；`authaudit_test.go` |
| 2 | 确认 token 重放 | 拒绝。确认 token 原子消费（`consumed_at` 单次置位）；执行成功后重放返回 `alreadyExecuted=true` 且不重复变更 | `mcpwrite/confirmation.go`；`TestConfirmationHardening` |
| 3 | 跨 token 消费确认 | 拒绝。确认与「租户 + 调用 token + 工具 + 参数哈希」四元绑定，同租户其他 write token 也不能消费 | `TestConfirmationHardening`（cross-caller） |
| 4 | 跨租户消费确认 | 拒绝。除 token 绑定外租户 ID 也在绑定四元组内；W3 新增显式回归 | `TestMarkPaidConfirmationCrossTenantRejected` |
| 5 | 参数漂移（用 A 参数的确认执行 B 参数） | 拒绝。参数哈希绑定 dry_run 与 execute；mark-paid 的金额 / 币种 / 渠道均入哈希，换金额即失效 | `paramsHash`；`TestConfirmationHardening`（drift） |
| 6 | 闸门绕过 | 未发现。全局 `MCP_WRITE_ENABLED=false` 时写工具不注册；租户 `mcp/write_enabled` 非 `"true"`（含读取失败）每次调用拒绝；两层独立 fail-closed | `TestWriteToolsHiddenWhenEnvGateClosed`；`TestTenantGateClosedRejects` |
| 7 | readonly token 越权写 | 拒绝。scope 为独立权限轴：readonly token 看不到也调不到写工具；write-only token 反之 | `TestScopeGateSeparatesSurfaces` |
| 8 | openapi token 越权写 | 拒绝。purpose=openapi 的 token 在 `/api/mcp` 鉴权即 401（入口互斥），到不了工具层 | `mcptoken` purpose 校验；`r176_purpose_test.go` |
| 9 | 跨租户写 | 拒绝。所有目标查找 / 变更强制 `tenant_id`；跨租户与不存在统一 404 口径（无存在性探测）。W3 附带验证被拒目标未被变更 | `TestCrossTenantTargetNotFound`；`TestProcurementWriteCrossTenant404`；`TestMarkPaidCrossTenant404` |
| 10 | 审计缺记录 | 未发现。execute 的业务变更与审计行同一事务（审计写失败整体回滚）；dry_run 审计与确认签发同事务；被拒调用（含超限 / 未配置 / 校验失败）也落审计行 | `TestWriteAuditFailClosed`；`TestMarkPaidUnconfiguredRejectedAndAudited`；`TestMarkPaidOverSingleLimit` |
| 11 | 次数限额绕过 | 未发现。每 token 30 次/时、每租户 200 次/天按成功 execute 审计行计数，计数失败即拒绝；多 token 叠加受租户级计数约束 | `TestWriteQuotasFailClosed` |
| 12 | 金额限额绕过（W3） | 未发现。单笔与日累计上限在 dry_run 与 **execute 事务内**各校验一次——先领确认再等额度被占满的 TOCTOU 路径被封死；日累计从成功 execute 审计行求和（同一 fail-closed 链），求和失败即拒绝 | `TestMarkPaidOverDailyLimit`（确认先发、执行时拒绝并验证未变更） |
| 13 | 金额边界：0 / 负数 | 拒绝（非法入参，进不了管道） | `TestMarkPaidAmountBoundaries` |
| 14 | 金额边界：精度（88.001、浮点尾差） | 拒绝。金额按「分」整数比较（`amountCents`：×100 后需在 1e-6 内为整数），超两位小数即非法；比较无浮点容差豁免 | `TestMarkPaidAmountBoundaries`；`TestMarkPaidMismatchRejected`（±0.01 拒绝） |
| 15 | 金额边界：币种混淆 | 拒绝。`currency` 必填、与采购单不符即拒（大小写不敏感规整后比较）；金额与币种同时校验，不能用「金额相等但币种不同」通过 | `TestMarkPaidMismatchRejected` |
| 16 | 上限配置边界（缺失 / 0 / 负数 / 非数字 / 加密行） | 一律视为未配置 = 拒绝（默认关，需管理员显式配置正数）；设置读取失败同样拒绝 | `TestMarkPaidBadLimitValuesRejected`；`markPaidLimits` |
| 17 | 状态机绕过（对未下单 / 已支付单重复登记） | 拒绝。`placed → paid` 走既有状态机 `transitionTx`，非法状态 dry_run 即拒 | `TestMarkPaidIllegalTransition` |
| 18 | 外发红线 | 保持。mark-paid 不触达任何真实外部平台：执行后仅对本地 Mock provider 推进状态（与后台人工路径同口径），无真实资金 / 外发动作 | `procurement.AfterMarkPaidCommitted` |

## 设计期消除的风险（W3 实现时按 fail-closed 处理，非存量漏洞）

1. **额度 TOCTOU**：若日累计只在 dry_run 校验，攻击者可先批量领取确认 token，等额度耗尽后仍执行。实现将全部前提（上限配置、金额 / 币种匹配、单笔与日累计）在 execute 事务内重跑（`validate` 闭包复用），确认 token 不构成额度豁免。
2. **浮点金额混淆**：`float64` 直接相等比较会因二进制表示产生伪不等 / 伪相等。实现统一折算为整数「分」比较，且超两位小数直接判非法入参，杜绝 `88.0000001` 这类边界。

## P2 / 建议级观察（无需本轮修复）

1. **并发执行下的限额竞态（P2，存量口径一致）**：次数限额与金额日累计均为「事务内读审计行再判断」，PostgreSQL READ COMMITTED 下两笔并发 execute 理论上可同时通过判断，短暂超出上限一笔。缓解因素：每笔写都需先 dry_run 领取一次性确认 token（攻击成本高）、每 token 30 次/时限流、审计行完整可事后对账。若需硬保证可引入租户级 advisory lock，建议与次数限额一并处理。
2. **审计 `amount` 字段口径（建议）**：仅 `procurement_mark_paid` 写入非零金额，其余工具恒为 0。后台审计列表如展示该列，建议标注仅对金额型动作有意义，避免误读。

## 门禁与证据

- 单元 / 集成：`go test ./internal/modules/mcpserver/ ./internal/modules/mcpaudit/... ./internal/modules/procurement/... ./internal/securitytests/...`（全绿，本地执行，不以 Actions CI 为据）。
- 权限矩阵：`backend/internal/securitytests/permmatrix/matrix.json` 中 `POST /api/mcp` 条目已登记 W3 动作与金额前提。
- Docker 双租户实测（mark-paid 成功 / 超限 / 未配置 / 币种不符四路径）：证据作为交付附件提供，不入库。
