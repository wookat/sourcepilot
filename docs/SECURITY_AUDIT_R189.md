# R189 安全审计季度全量复跑（距 R183 六轮）

- 审计对象：`wookat/sourcepilot`。重点面：MCP 写全链 W1–W3、四 persona 权限矩阵、settings 敏感 key 与 token 治理、R187/R188 性能改动面（SQL 下推 / 复合索引）、依赖与密钥处理。
- 视角：渗透测试视角（越权读写 / 跨租户 / 存在性泄露 / 闸门与限额绕过 / 重放与参数漂移 / 审计缺记录 / 注入）+ 代码审读 + 本地实测（Docker PostgreSQL / Redis + 重建后端镜像）。
- 结论：**发现并修复 2 项 P1（店铺授权集"空即无限制"越权读；principal 解析失败降级为无限制）**，无 P0；其余攻击面未发现 P0 / P1，6 项 P2 列清单。`govulncheck` 0 个可达漏洞。
- 依据：Actions CI 不作依据；全部证据为本地执行，证据文件作附件不入库。

## 0. 审计口径（重要）

`#374`、`#375` 已合入 `main`；`#376`、`#377`、`#378` 仍为 open 且 mergeable（`git_view_pr` 权威核实）。因此审计分支自 `origin/main` 起按序叠加

`#376`（R187 线2 性能审计，纯文档）→ `#377`（R188 线1）→ `#378`（R188 线2 管道前拒绝审计），

冲突仅出现在 `docs/PROGRESS.md`（两侧段落并存，无代码冲突）。本报告全部结论以该叠加基线为准；若上述 PR 最终合入内容变化，需按变化重跑受影响用例。

## 1. P1 发现与修复（先红后绿）

两项同源：`AllowedStoreIDs()` / `OperableStoreIDs()` 的契约是 **`nil` = 无限制（admin），非 nil 空集 = 无任何店铺授权**，而部分消费方只判断 `len(ids) > 0`，把「空集」与「无限制」混为一谈；`LoadPrincipal` 在数据库错误时又返回 `nil` principal，使忽略 error 的调用点直接落入该混淆分支。

### P1-1 空店铺授权集被当作"无限制"→ 租户内跨店越权读

- 触发条件：**无需任何故障**。任何非 admin 且不持有店铺授权的账号（`readonly` 账号默认即如此，`operator` 被撤销全部授权后亦然）。
- 位置与影响：
  1. `backend/internal/modules/taskcenter/service.go`（`passesUnifiedFilters`）：`len(p.AllowedShopIDs) > 0` 才过滤店铺 → 无授权账号读到**本租户全部店铺**的失败任务（含店铺 ID、平台、错误详情、关联资源标题）。任务中心是全站失败任务聚合面，泄露面覆盖采集 / 订单同步 / 库存同步 / 商品发布 / AI / 客服六类任务。
  2. `backend/internal/modules/operationdashboard/service.go`：库存计数在非 admin 且授权集为空时跳过 per-store 统计、落回**租户级**统计。
  3. `backend/internal/modules/operationdashboard/screen.go`：大屏库存预警列表同样落回租户级查询。
- 先红（HTTP 层，走生产路由 `api.Register`）：新增 `internal/securitytests/permmatrix/r189_empty_store_scope_test.go`，修复前 `readonly` persona 在 `GET /api/v1/task-center/failures` 读到 `ShopGranted` 与 `ShopUngranted` 两个店铺的失败行（证据 `red-permmatrix-empty-scope.txt`）。
- 修复：三处按 `nil` / 空集分别处理——非 nil 空集一律"零结果"，不再跳过过滤或落回租户级。
- 后绿：同一用例 4 persona 断言全通过（无授权账号 0 行、operator 仅授权店、tenant admin 两店、跨租户 admin 0 行）；`go test ./...` 全包通过。

### P1-2 `LoadPrincipal` 数据库错误返回 nil principal → 调用点降级为无限制

- 位置：`backend/internal/pkg/adminperm/context.go`。`gorm.ErrRecordNotFound` 之外的错误返回 `(nil, err)`。
- 影响：忽略 error 只取 principal 的调用点把 nil 视为"无限制"：
  - `inventory/handler.go` `operableShopIDs`：返回 `nil` = **可写全部店铺**（写侧，`POST /inventory-sync/batches` 等）；
  - `orderexception/handler.go` `requestScope`、`operationdashboard/scope.go`（nil principal 直接 `IsAdmin: true`）、`taskcenter/handler.go`：读侧无限制；
  - `reports/procurement_report.go` `allowedShopIDs`：`err != nil` 时显式返回 `(nil, nil)`，即采购报表按 admin 口径统计全租户。
- 先红：新增 `internal/pkg/adminperm/principal_failclosed_test.go`，以 `internal/testing/faildb`（已关闭的连接池，不含任何真实凭据）触发确定性数据库错误，修复前 `principal=<nil> err=sql: database is closed`（证据 `red-failclosed.txt`）。
- 修复：
  1. `LoadPrincipal` 在数据库错误时返回 `RoleReadonly + Disabled` 的**拒绝一切** principal 与原 error（不入 context 缓存，重试可重新解析）：传播 error 的调用点仍返回 500，只取 principal 的调用点收敛为"无店铺"而非"全部店铺"；
  2. `reports/procurement_report.go` 改为传播 error（该路径本就返回 error）。
- 后绿：`adminperm`、`taskcenter`、`orderexception`、`inventory`、`operationdashboard`、`reports` 六个包的 fail-closed 用例全绿（证据 `green-failclosed.txt`）。

### Docker 重建镜像实测

`docker build -t trademind-backend:r189 backend/` 重建后端镜像，以 `--network host` 接 Docker PostgreSQL（独立库 `trademind_r189`）+ Redis 启动，走真实登录链路：bootstrap admin 创建 `readonly` 账号（零店铺授权），SQL 播种同租户 2 个店铺各 1 条失败订单同步任务，再以两个真实 JWT 调 `GET /api/v1/task-center/failures`：

| 镜像 | readonly（零授权） | tenant admin |
| --- | --- | --- |
| `trademind-backend:r189-pre`（修复前代码） | rows=2，distinct_shop_ids=2（越权坐实） | rows=2 |
| `trademind-backend:r189`（修复后） | rows=0，distinct_shop_ids=0 | rows=2 |

证据 `docker-image-probe-red.log` / `docker-image-probe-green.log`。

## 2. MCP 写全链 W1–W3 攻击面复验（覆盖面 1）

| # | 攻击项 | 本轮结论 | 依据 |
| --- | --- | --- | --- |
| 1 | 读写 scope 隔离 | 保持。`write:ops` 独立轴；readonly token 探测写工具即拒 | `TestScopeGateSeparatesSurfaces`、`TestWriteToolProbeByReadonlyTokenAudited` |
| 2 | 三层闸门（全局 env / 租户 setting / token scope） | 未发现绕过。env 关闭时写工具不注册；租户 `mcp/write_enabled` 非 `"true"`（含读取失败）逐次拒绝 | `TestWriteToolsHiddenWhenEnvGateClosed`、`TestTenantGateClosedRejects` |
| 3 | dry-run → execute 过渡 | 保持。execute 必须持确认 token，先消费后变更 | `TestDryRunConfirmExecuteIdempotent` |
| 4 | 确认 token 重放 / 跨租户 / 跨 token / 参数漂移 | 全部拒绝。确认与「租户 + token + 工具 + 参数哈希」四元绑定，`consumed_at` 原子置位 | `TestConfirmationHardening`、`TestMarkPaidConfirmationCrossTenantRejected` |
| 5 | advisory lock 并发 + 次数 / 金额限额 | 未发现绕过。进程内租户互斥 + `pg_advisory_xact_lock`，额度在事务内计数，计数失败即拒 | `TestExecuteCountQuotaRacePostgres`、`TestExecuteTenantIsolationRacePostgres`、`TestWriteQuotasFailClosed` |
| 6 | fail-closed 审计恰一次 | 保持。execute 的业务变更 + 审计行 + 确认置位同一事务，审计失败整体回滚；一次 dry_run→execute 恰 2 行 | `TestWriteAuditFailClosed`、`TestWriteToolsAuditExactlyOncePerCall` |
| 7 | #378 管道前拒绝审计补写（零回退 / 无新盲区） | 零回退。入口层用 `mcpwrite.Signal` 判定"是否已进入写管道"，未进入才补写 1 行 error；成功链路不受影响、无重复行 | `TestWriteToolPreflightRejectionAudited`、`signal.go`、`TestWriteToolsAuditExactlyOncePerCall` |
| 8 | 跨租户写 / 存在性探测 | 拒绝。目标查找强绑 `tenant_id`，跨租户与不存在统一 404 | `TestCrossTenantTargetNotFound`、`TestMarkPaidCrossTenant404`、`TestExceptionsMarkCrossTenant404`、`TestProcurementWriteCrossTenant404` |
| 9 | mark-paid 金额 / 币种 / 状态机 / 上限 | 未发现绕过。上限未配置即拒付；金额按整数分比较；非法状态 dry_run 即拒 | `TestMarkPaidUnconfiguredRejectedAndAudited`、`write_tools_r181.go` |
| 10 | 外发红线 | 保持。写白名单内无消息外发与真实资金动作 | `mcpserver/write_tools.go` 白名单 + `TestIsWriteToolCoversWholeWhitelist` |

新盲区排查结论：`markReached(ctx)` 位于 `Run` 校验依赖之后、mode / gate 校验之前，因此**所有**进入管道的拒绝都由管道内 `auditReject` 记账，入口层不重复补写；反之未进入管道的拒绝由入口层补写。唯一残余为 `auditReject` 自身失败时仅落日志（无变更、无审计行），列为 P2-1。

## 3. 权限体系四 persona × 写接口探针（覆盖面 2）

- 载体：`internal/securitytests/permmatrix`（用生产 `api.Register` 构建全路由）+ `internal/securitytests/idor` + `internal/securitytests/shopscope`，本轮全量复跑 192 个用例全绿、0 skip（Docker PostgreSQL 可用）。
- view-only（operator + view 授权）：可读不可写，写接口 40303 / `ErrStoreNotOperable`；不可见店铺一律 404，不泄露存在性（`store_list_scope.go`、`TestOperatorStoreScope`）。
- readonly：路由级 40301（`RequireWritable` / `CanWriteOrders`），本轮修复其**读侧**跨店越权（见 P1-1）。
- operator：写作用域受 `OperableStoreIDs` 限制，view 授权不可写；空集经本轮修复后为"零结果 / 不可写"。
- admin：受租户闭合约束，跨租户对象 404（`TestCrossTenantShopIsolation`、`IDOR_*CrossTenant` 系列）。
- 审计页：`GET /api/v1/mcp/audit-logs` 写行仅 `settings.manage`（`TestAuditListWriteRowsAdminOnly[Postgres]`），写 token 对非 admin 404 隐藏。

## 4. settings 敏感 key / token 治理 / 开放 API（覆盖面 3）

| 项 | 结论 | 依据 |
| --- | --- | --- |
| 敏感 key 注册表 | 保持。注册表来自集成 schema 的 `Sensitive` 字段 + 遗留键 + bootstrap 注册；命中即服务端强制加密，客户端 `isEncrypted=false` 不能反悔 | `settings/sensitive_registry.go`、`service.go putOne` |
| 掩码回写 / 空值回写 | 拒绝覆盖真值（新建加密项传掩码直接报错） | `encrypt.LooksMasked` 分支 |
| 惰性加密收编 | 生效。读路径掩码返回 + 条件更新为密文，best-effort 不阻塞读 | `adoptLegacyPlaintext` |
| token purpose 双向隔离 | 保持。`purpose=openapi` 在 `/api/mcp` 入口 401；`purpose=mcp` 不能走开放 API；开放 API 强制 readonly scope | `mcptoken.AuthenticateFor`、`TestOpenAPIPurposeTokenRejectedAtMCPEntry` |
| 过期 / 吊销 / 停用租户 | 均 fail-closed（写 token 强制过期时间） | `TestExpiredTokenRejectedAtEntry`、`TestRevokeBlocksAuth`、`TestWriteTokenForcedExpiry`、`TestAuthenticateRejectsDisabledTenant` |
| 限流与 XFF | 每 IP 鉴权失败预算 + 租户桶封顶多 token 流量；客户端 IP 取自受信代理配置，未配置时不采信 XFF | `TestInvalidTokenAttemptsAreRateLimited`、`TestTenantBucketCapsMultiTokenTraffic`、`TestValidTokenNotChargedByAuthFailureBudget` |
| token 明文 | 仅创建时返回一次，库内仅 SHA-256，列表与审计仅掩码 | `mcptoken/service.go`、`mcpaudit` 写入字段 |
| 开放 API 面 | 只有 5 个 GET（orders / orders/:orderNo / inventory / reports/summary / exceptions），无写路由，全部租户闭合并逐次审计 | `openapi/server.go`、`TestOrdersQueryTenantScopeAndMasking` |

## 5. R187/R188 性能改动面（覆盖面 4）

- `orderexception/summary_sql.go`（SQL 下推 COUNT）：全部用户输入走 `?` 绑定参数（`Raw(q, args...)`），SQL 片段中的表名 / 列名 / 别名 / 常量均为代码常量，`ShopID` / `OrderID` 先 `uuid.Parse` 成功才拼接条件 → **未发现注入面**。
- 租户闭合：`appendOrderFilters` 在 `TenantID != nil` 时强制 `tenant_id = ?`；`shopScopeSQL` 在 `AllowedShopIDs != nil` 时强制 `shop_id IN ?` 且排除 NULL / 零值 UUID；`AllowedShopIDs` 为空集时 `SummaryOpenExceptions` 直接返回全 0（fail-closed）。
- `NOT EXISTS` 标记过滤：`order_exception_marks` 的关联键为 `(exception_type, source_type, source_id)`，与主查询同一租户候选集内匹配，未引入跨租户读取；JOIN 均以主表租户条件收敛，无新增 IDOR 路径。
- 语义等价：`SummaryOpenExceptions` 与列表口径一致（Start/End、Severity、Keyword 历来不影响 summary——列表侧 summary 也在 `filterAggRows` 之前统计），`internal/testing/integration/orderexception_summary_parity_test.go` 覆盖无限制 / 单店 / 空集 / 平台 / 店铺 / 类型六组场景。
- `database/migrate_round188.go`（`mcp_tool_call_logs (tenant_id, created_at DESC)` 复合索引）：固定 SQL、固定表 / 索引名、`IF NOT EXISTS` 幂等、仅 PostgreSQL 生效，无用户可控标识符；索引不改变可见性谓词。
- 结论：R187/R188 性能改动面**未引入注入、越权或租户闭合缺口**。

## 6. 依赖与密钥处理（覆盖面 5）

- `govulncheck ./...`（自行安装 `golang.org/x/vuln@latest`）：**0 个可达漏洞**（Symbol Results: no vulnerabilities found）。模块级 1 项 `GO-2026-5932`（`golang.org/x/crypto/openpgp` 无维护、无修复版本），本仓库未 import 该包，列 P2-3 挂账。
- `pnpm audit --prod`：16 项（3 low / 8 moderate / 5 high），全部经 `admin > @umijs/max@4.6.83` 传递链（`vite` / `esbuild` / `image-size` / `nanoid` / `elliptic` / `react-router@6.3.0`），无直接依赖可单独升级，属构建链与 umi 自带运行时，列 P2-4。
- 密钥处理：敏感 settings 走 `APP_MASTER_KEY` + AES-GCM 密文落库、读取掩码；token 仅存 SHA-256；审计表不落 token 明文与参数值；日志抽查未见完整 key / token / cookie 输出；本轮新增测试助手 `internal/testing/faildb` 使用不可路由地址与占位串（无真实凭据），审计证据与探针脚本不入库。

## 7. P2 清单（本轮登记，未修）

1. **管道内拒绝的审计写失败仅落日志**：`mcpwrite.Service.auditReject` 写审计失败时只 `slog.Error`，请求本就被拒（无变更、无未审计的写），但审计库故障期间可无痕探测闸门与限额。建议让 `Run` 在补记失败时清除 signal，使入口层的 fail-closed 补写与 500 语义同样覆盖拒绝路径。
2. **`SummaryOpenExceptions` 缺租户强约束（纵深防御）**：`TenantID == nil` 且 `AllowedShopIDs == nil` 时无租户谓词，当前全部调用点（admin 路由 / 开放 API / 大屏）都显式传租户，属可用性兜底而非现实越权。建议对聚合入口要求非 nil 租户，否则直接返回错误。
3. **`GO-2026-5932`（`x/crypto/openpgp`）无修复版本**：不可达，随 `golang.org/x/crypto` 升级复查。
4. **前端构建链 16 项 advisory 无直接升级路径**：需评估 `@umijs/max` 大版本升级或 overrides 收敛（注意 `react-router@6.3.0` 为 umi 自带运行时）。
5. **游标指纹在 principal 解析失败时按空集计算**：`operationlog` / `product` / `order` 的 `*CursorScope` 用 `err == nil` 守卫，解析失败即以空授权集算指纹。实际过滤由传播 error 的 `ApplyStoreScope*` 负责，不构成越权，但指纹与实际作用域会不一致（游标失效）。建议统一改为传播 error。
6. **承接 R188 未修 P2**：mark-paid 限额值缺服务端值域校验（本轮复核仍未加）、审计可见性只有"写行/读行"一档、`scripts/` 下外部字符串输出未统一净化。

## 8. 本地门禁与实测记录

| 项 | 结果 |
| --- | --- |
| `gofmt -l .`（backend） | 无输出 |
| `go vet ./...` | 通过 |
| `go build ./...` | 通过 |
| `go test ./...`（APP_ENV=test + Docker PostgreSQL + Redis DB15） | 全包通过 |
| MCP + securitytests 全量 verbose 复跑 | 192 PASS / 0 FAIL / 0 SKIP |
| `govulncheck ./...` | 0 可达漏洞 |
| `pnpm audit --prod` | 16 项（见 P2-4） |
| `pnpm test:contracts` | 通过 |
| Docker 重建镜像红绿实测 | `r189-pre` 越权坐实 / `r189` 修复后零泄露 |

前端未改动（本轮修复全在 backend），故未跑 `build:admin` / `test:frontend` 等 UI 门禁；`pnpm test:contracts` 作为 API 契约兜底已通过。GitHub Actions CI 不作本报告依据。
