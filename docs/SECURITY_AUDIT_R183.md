# R183 安全审计季度复跑（距 R176 六轮）

- 审计对象：`wookat/sourcepilot`，重点新面为 MCP 写全链 W1–W3、settings 敏感 key 注册表 + 惰性收编，并复验 R176 / R181 已修项零回退。
- 视角：渗透测试视角（token 伪造 / 重放 / 跨租户 / 闸门绕过 / 限额绕过 / 审计缺记录 / 金额边界 / readonly-openapi 越权写）+ 代码审读 + 本地实测。
- 结论：**发现并修复 1 项 P1（MCP 写审计双写与已提交写被误拒）**，其余攻击面未发现 P0 / P1；4 项 P2 / 建议级列清单。`govulncheck` 0 个可达漏洞。
- 依据：Actions CI 不作依据；全部证据为本地执行（Docker PostgreSQL / Redis），证据文件作附件不入库。

## 0. 审计口径（重要）

`#360`、`#364` 已合入 `main`；`#361`、`#362`、`#363`、`#365`、`#366` 仍为 open（`git_view_pr` 权威核实）。因此本轮**不能**只审 `main`：审计分支自 `origin/main` 起，按 v34 依赖顺序叠加

`#361`（R180 W2）→ `#363`（R181 W3）→ `#365`（R182 限额竞态）→ `#362`（R180 线2）→ `#366`（R182 线2），

冲突仅出现在 `docs/PROGRESS.md`（文档段落并存，无代码冲突）。本报告全部结论以该叠加基线为准；若上述 PR 的最终合入内容发生变化，需按变化重跑受影响用例。

## 1. P1 发现与修复（先红后绿）

### P1-1 `procurement_mark_paid` 未登记为写工具 → 审计双写 + 已提交写被误拒

- 位置：`backend/internal/modules/mcpserver/write_tools.go` 的 `isWriteTool`。
- 成因：W3 动作常量定义在 `write_tools_r181.go`，而 `isWriteTool` 的白名单仍是 R179/R180 的 5 个动作，漏了 `procurement_mark_paid`。入口审计中间件用 `isWriteTool` 判断「该调用是否已在写管道内审计」，因此对唯一的金额型动作 **额外**写一行通用审计行（`mode` 为空、`amount=0`、无参数摘要 / 确认哈希）。
- 影响（为何按 P1 处理，而非记账瑕疵）：
  1. 唯一涉及资金登记的动作，其审计轨迹被污染：每次 dry_run / execute 各多一行 mode 空行，`mode` 筛选与 R182 的「金额（仅支付登记）」列口径同时失真，事后对账（谁在何时以何金额登记支付）不可直接依赖审计表；
  2. 该额外审计行在写事务**提交之后**才写：若它失败，中间件将成功结果替换为 `audit log unavailable, tool call rejected`，调用方看到失败而采购单实际已标记为已支付——对资金型动作而言是错误的失败语义（幂等重放虽保证不会重复扣款，但状态与回执不一致）。
- 未构成 P0 的原因：额外行 `mode` 为空，不进入 `CountExecutes*` / `SumExecuteAmountByTenantTool`（均限定 `mode='execute' AND status='success'`），因此**不影响**次数限额与金额日累计；写管道内的正式审计行始终存在，无「审计缺记录」。
- 先红：新增 `TestWriteToolsAuditExactlyOncePerCall`，修复前 `mark_paid audit rows = 4, want 2`（证据 `red-write-audit.txt`）。
- 修复：将 `ToolProcurementMarkPaid` 补入 `isWriteTool` 白名单（一行，最小改动）。
- 后绿：`TestWriteToolsAuditExactlyOncePerCall` 通过；另加 `TestIsWriteToolCoversWholeWhitelist` 守护——今后新增写动作若忘记登记即测试失败（证据 `green-write-audit.txt`）。

## 2. MCP 写全链 W1–W3 攻击面逐项复验

| # | 攻击项 | 本轮结论 | 依据 |
| --- | --- | --- | --- |
| 1 | token 伪造 / 爆破 | 拒绝。库内仅 SHA-256；无效与不存在不可区分（401）；每 IP 鉴权失败预算 1rps/突发 10 | `mcptoken.Service.Authenticate`、`server.go` authFailLimiter、`authaudit_test.go` |
| 2 | 确认 token 重放 | 拒绝。`consumed_at` 原子置位；已执行重放返回 `alreadyExecuted` 不再变更 | `mcpwrite/confirmation.go`、`hardening_test.go` |
| 3 | 跨 token / 跨租户消费确认 | 拒绝。确认与「租户 + 调用 token + 工具 + 参数哈希」四元绑定，且仅存哈希 | `consumeConfirmation`、`TestMarkPaidConfirmationCrossTenantRejected` |
| 4 | 参数漂移 | 拒绝。canonical 参数入哈希（W3 含金额 / 币种 / 渠道），换参即失效 | `paramsHash`、`TestConfirmationHardening` |
| 5 | 三层闸门绕过 | 未发现。`MCP_WRITE_ENABLED=false` 时写工具不注册；租户 `mcp/write_enabled` 非 `"true"`（含读取失败）逐次拒绝；scope `write:ops` 独立轴 | `mcpwrite/gate.go`、`r179_write_test.go` |
| 6 | 租户开关被越权打开 | 未发现。`PUT /api/v1/settings` 要求 `settings.manage`，operator / readonly 无该权限；写 token 的创建 / 可见 / 吊销均限管理员（非管理员 404 隐藏） | `settings/handler.go`、`adminperm` 矩阵与 `perm_test.go`、`mcptoken/handler.go` |
| 7 | readonly / openapi token 越权写 | 拒绝。scope 分离；purpose=openapi 在 `/api/mcp` 入口即 401；openapi 面只有 5 个 GET，无写路由 | `r176_purpose_test.go`、`openapi/server.go` |
| 8 | 跨租户写 / 存在性探测 | 拒绝。目标查找与变更全部 `tenant_id` 强绑，跨租户与不存在统一 404 | `FindPOInTenant` 等、`TestMarkPaidCrossTenant404` |
| 9 | 审计缺记录 | 未发现（并修复了反向的双写，见 P1-1）。execute 的业务变更 + 审计行 + 确认置位同一事务，审计失败整体回滚 | `mcpwrite/service.go`、`TestWriteAuditFailClosed` |
| 10 | 次数限额绕过 | 未发现。每 token 30/时、每租户 200/天，按成功 execute 审计行在**事务内**计数，计数失败即拒绝 | `quotaUsage`、`TestWriteQuotasFailClosed` |
| 11 | 并发限额绕过（R182 面） | 未发现。进程内租户互斥 + `pg_advisory_xact_lock(hashtext('mcpwrite_execute:<tenant>'))` 串行化；非 PostgreSQL 时退化为进程内锁（已在文档登记） | `lockTenantAdvisory`、`RacePostgres` 三例 `-count=3` 通过 |
| 12 | mark-paid 三前提 | 未发现绕过。上限配置 / 金额币种匹配 / 单笔与日累计在 dry_run 与 **execute 事务内**各校验一次，确认 token 不构成额度豁免 | `write_tools_r181.go` `validate` 闭包、`TestMarkPaidOverDailyLimit` |
| 13 | 金额边界（0 / 负 / NaN / Inf / >1e10 / 超两位小数 / 浮点尾差 / 金额或币种不符） | 全部拒绝。`amountCents` 折算整数分比较，无浮点容差豁免；币种大小写不敏感强匹配 | `amountCents`、`TestMarkPaidAmountBoundaries`、`TestMarkPaidMismatchRejected` |
| 14 | 上限配置边界（缺失 / 0 / 负 / 非数字 / 被加密行 / 读取失败） | 一律视为未配置 = 拒绝（默认关） | `markPaidLimits`、`TestMarkPaidBadLimitValuesRejected` |
| 15 | 状态机绕过 | 拒绝。`placed → paid` 复用既有状态机，非法状态 dry_run 即拒 | `TestMarkPaidIllegalTransition` |
| 16 | 外发红线 | 保持。写白名单内无消息外发 / 真实资金动作；mark-paid 仅推进本地状态 | `AfterMarkPaidCommitted` |
| 17 | 治理 UI 权限 | 未发现越权。写白名单卡片与写 token 管理仅管理员可见；只读账号禁用创建 / 吊销；后端为唯一权威 | `admin/src/pages/Settings/McpTokens.tsx` + 后端权限校验 |
| 18 | 审计读接口越权 | 未发现跨租户泄漏。`GET /api/v1/mcp/audit-logs` 强制请求租户，仅返回本租户行，token 明文与参数值不入表 | `mcpaudit/handler.go`、`mcpaudit/service.go` |

## 3. settings 敏感 key 注册表 + 惰性收编回归（R177 / R179 面）

| 项 | 结论 | 依据 |
| --- | --- | --- |
| 明文降级（payload 省略或显式 `isEncrypted=false`） | 拒绝。加密黏性：`it.IsEncrypted \|\| 已加密 \|\| IsSensitiveKey(...)`，注册表键强制服务端加密，客户端标记不能反悔 | `settings/service.go` `putOne`、`r176_encryption_sticky_test.go`、`r177_sensitive_registry_test.go` |
| 掩码值回写覆盖真值 | 拒绝。`LooksMasked` 命中时不改值（新建则直接报错） | 同上 |
| 惰性收编（历史明文敏感行） | 生效。读路径掩码返回并以「观测明文」为条件更新为密文，best-effort 不影响读；条件更新使并发 PUT（写即加密）不会被覆盖 | `adoptLegacyPlaintext`、`r179_legacy_plaintext_test.go` |
| 并发覆盖 | 未发现丢失更新导致的降级：收编为条件更新，PUT 走事务 + 唯一键 upsert，两条路径的终态均为密文 | 同上 |
| Shopee `partner_key` | 已静态注册（不依赖 provider bootstrap 顺序），强制加密 + 掩码 | `sensitive_registry.go` `init`、`r180_shopee_sensitive_test.go` |
| 跨租户 / 平台配置写入 | 拒绝。`PUT` 把 tenantId 钉在请求租户，指定他租户即 403；租户 0 平台配置不经 List 外泄 | `settings/handler.go`、`service.go` `List` 注释与实现 |

## 4. R176 / R181 已修项零回退

- R176 P1「迁移导入租户越权」、P1「加密设置明文降级」、P2「校验先于 scope」及权限 / 跨租户矩阵：`internal/securitytests/permmatrix`、`idor`、`shopscope` 全绿。
- R176 XFF / 可信代理、密钥掩码与加密：相关单测全绿（`go test ./...` 全量通过）。
- R181 全部 18 项攻击面：本轮逐项复验，结论一致（见第 2 节），其 P2 #1（并发限额竞态）已由 R182 的 advisory lock 关闭并有 PostgreSQL 回归；P2 #2（审计 amount 口径）已由 R182 的 UI 列 + 口径说明关闭。
- 无回退项。

## 5. 依赖与漏洞

- `govulncheck ./...`：**0 个可达漏洞**（代码可达 0；依赖模块中 1 个不可达）。与 R176 结论一致。
- `pnpm audit --prod`：**16 项**（3 low / 8 moderate / 5 high），R176 为 15 项。全部位于 admin 构建链（umi / vite / esbuild / launch-editor / image-size / nanoid / react-router / elliptic 等传递依赖），均不进入运行时服务端路径；无生产运行时可利用面。列为 P2-4 跟踪。

## 6. P2 / 建议清单（本轮不修）

1. **P2-1 非 PostgreSQL 部署的限额硬保证**：`lockTenantAdvisory` 仅在 PostgreSQL 生效，其他驱动退化为进程内互斥，多副本下并发限额仍是软保证。建议：若支持非 PostgreSQL 生产部署，改用 Redis 分布式锁或审计行唯一约束兜底。
2. **P2-2 入口审计中间件的兜底写发生在业务事务之后**：即使 P1-1 已修，通用中间件对**读**工具仍是「业务后审计」；读工具无副作用，故仅为语义一致性建议（写面已在事务内）。
3. **P2-3 `mcpaudit` 读接口无独立权限位**：`GET /api/v1/mcp/audit-logs` 只校验租户，未要求 `settings.manage`，只读账号可见写动作审计（含 token 名称 / 掩码 / 确认哈希，无明文密钥）。信息价值有限但与「写 token 仅管理员可见」的治理口径不完全一致，建议后续对齐为管理员可见。
4. **P2-4 admin 构建链依赖告警 16 项**：见第 5 节，建议按 umi/vite 升级窗口统一处理，不在安全审计轮次内单独升级以免影响构建稳定性。

## 7. 本地门禁与证据（Actions CI 不作依据）

| 门禁 | 结果 |
| --- | --- |
| backend `gofmt -l .` | 通过（无差异） |
| backend `go vet ./...` | 通过 |
| backend `go build ./...` | 通过 |
| backend `go test ./...` | 全部通过 |
| backend `go test ./...`（APP_ENV=test + Docker PostgreSQL + 测试 Redis DB15） | 全部通过 |
| MCP 并发回归 `-run RacePostgres -count=3`（Docker PostgreSQL） | 通过（双租户隔离 / 次数硬限 / 金额上限三例 ×3） |
| `govulncheck ./...` | 0 可达漏洞 |
| `pnpm check:dev` | 通过 |
| `pnpm check:ui-copy --strict` | 通过 |
| `pnpm test:frontend` | 通过 |
| `pnpm test:contracts` | 通过 |
| `pnpm test:collector` | 通过 |
| `pnpm build:admin` / `pnpm build:collector` | 通过 |
| `pnpm audit --prod` | 16 项告警（见 P2-4） |

Docker 双租户实测：在 docker-compose 的 `trademind-postgres` 上执行 `TestExecuteTenantIsolationRacePostgres`（租户 1 末位名额被硬限、租户 2 全通过，锁与额度不跨租户泄漏或阻塞）及 mark-paid 跨租户 404 / 确认跨租户拒绝用例。证据文件（`red-write-audit.txt`、`green-write-audit.txt`、`race-postgres-x3.txt`、`r183_govuln.log`、`r183_gotest_pg.log`、`r183_pnpm_audit.log`）作交付附件，不入库。
