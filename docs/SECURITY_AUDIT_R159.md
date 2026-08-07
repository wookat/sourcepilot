# R159 线1：安全审计季度复跑报告

- 轮次：R159（距 R148 安全审计季度复跑 11 轮）
- 审计基线：`main @ 32a9aaea`（#311/#312/#319/#320 已合入；**#317（MCP 审计 fail-closed 错误码 `-32603`）与 #318（大屏折算 + 自定义指标配置）尚未合入**，按建议合并顺序 #317→#318 在本地审计栈 `6cdfab60` 上叠加实测，该栈不进 PR）
- 范围：开放 API `/api/open/v1/*`（R149–R158 新增面）、MCP 零回退复验、多语言客服模板/买家消息草稿注入与授权、大屏折算与自定义指标配置、权限矩阵 registry/harness 漂移（含 `OpenAPIEnabled` 组合）、govulncheck / pnpm audit、seed 生产拒绝
- 依据：本地 Docker 全栈实测 + 隔离测试库 Go 套件 + 代码审读（Actions CI 不作依据）

## 【结论】

| 项 | 结论 |
| --- | --- |
| 开放 API token purpose 双向隔离 | 成立（mcp-only→开放 API 401；openapi-only→MCP 401；`both` 双通） |
| 开放 API 跨租户 / shop scope | 成立（跨租户单号 404，列表仅本租户；租户 A total=0 / 租户 B total=5） |
| 开放 API 限流绕过（多 token / 无效 token / XFF） | 无绕过（伪造 token 连续第 8 次进入 429，XFF 逐 IP 伪造不重置额度；`TRUSTED_PROXIES` 默认空） |
| 开放 API 入口级审计 + 逐次审计 fail-closed | 与文档口径一致（逐次审计 fail-closed 返回 500；入口级 401/429 留痕为 best effort） |
| 开放 API 输出敏感字段 | 无手机号/邮箱/长数字串/密钥；`customerName` 脱敏为 `D**` |
| OpenAPI 契约一致性 | **发现 P2-1 severity 枚举漂移（本轮已随文档/schema 修正）** |
| MCP R145/R148 修复项 | 零回退（scope/限流分层/token 上限/过期/审计 fail-closed 测试全绿） |
| MCP 错误码 | 未知工具 `-32602`（参数域）；审计 fail-closed `-32603` 仅在叠加 #317 后成立，`main` 现状仍是 JSON-RPC `code:0` |
| MCP 租户禁用即失效 | 成立（租户 2 置 disabled → MCP/开放 API 双入口 401；恢复后 200） |
| 多语言模板注入面 / regenerate 授权 / readonly | 无执行面（占位符仅字符串替换、未知变量原样保留、admin 端无 `dangerouslySetInnerHTML`）；readonly 403、跨租户 404 |
| 大屏折算 / 自定义指标配置（叠加 #318） | scope 与 readonly 成立；未知/重复/空卡片 400；非法 `shopId` 400；不支持币种显式列出不静默折算 |
| 店铺 scope 写权限 | **发现 P1-1：仅 `view` 授权的 operator 可写入该店铺数据（本轮已修复 + 回归测试）** |
| 权限矩阵 registry / harness（含 `OpenAPIEnabled`） | 无漂移（644 条 route 全绿）；`OPENAPI_ENABLED=false` 运行时开放 API 404、MCP 不受影响 |
| govulncheck | 0 个可达漏洞（1 条 require-only 不可达） |
| pnpm audit --prod | 13 条（3 low / 8 moderate / 2 high），全部为前端构建工具链依赖 → P2-3 |
| seed 生产拒绝 | 通过（demoseed/restore/config 三处生产闸门测试全绿） |

## 【证据】

### 1. 开放 API `/api/open/v1/*`

purpose 双向越权（同一租户三种 token）：

| token purpose | `POST /api/mcp` | `GET /api/open/v1/orders` |
| --- | --- | --- |
| `mcp` | 200 | 401 |
| `openapi` | 401 | 200 |
| `both` | 200 | 200 |

未知 token / 空 Authorization 同样 401，与 purpose 不符返回完全一致，不泄露 token 存在性（口径见 `docs/open-api.md` §鉴权）。

跨租户与存在性：租户 A token 读租户 B 单号 `DEMO-T2-AT-0003` → 404；读不存在单号 → 同为 404。租户 B token 读自身 → 200，`customerName` 为 `D**`。

限流与 XFF：12 次伪造 token 请求（每次伪造不同 `X-Forwarded-For`）→ `401×7, 429×5`，逐 IP 伪造无法重置每来源鉴权失败预算。

脱敏扫描（`/api/open/v1/orders` 响应正文正则）：中国手机号 0、邮箱 0、15–19 位数字串 0、`sk-` 前缀 0。

审计（`mcp_tool_call_logs`，20 分钟窗口）：

```text
mcp:auth|auth_failed|1        openapi:auth|auth_failed|1
no_such_tool|error|1          openapi:auth|rate_limited|1
orders_query|error|1          openapi:orders_list|success|5
                              openapi:orders_detail|error|2 / success|1
                              openapi:inventory_list|success|1
                              openapi:reports_summary|success|1
                              openapi:exceptions_list|success|2
```

逐次审计 fail-closed 在 `openapi/server.go` `audited()` 中实现（审计失败丢弃已缓存响应并 500），入口级 401/429 留痕为 best effort，两者口径均已在 `docs/open-api.md` 写明；`docs/mcp.md` 原文未区分二者，本轮补齐一句说明。

`OPENAPI_ENABLED=false` 运行时：以同一镜像另起容器（仅改该变量）→ `GET /api/open/v1/orders` 404、`POST /api/mcp` 200，开关不影响 MCP 入口，矩阵 harness 的 `OpenAPIEnabled=true` 组合亦无 stale 路由。

### 2. MCP 零回退复验

- `internal/modules/mcpserver`、`mcptoken`、`mcpaudit` 套件全绿（scope 强校验、三层限流、token 上限、过期边界、审计 fail-closed `-32603`、Redis 降级不 fail-open）。
- 未知工具返回 `-32602 unknown tool "no_such_tool"`（JSON-RPC 参数域错误）；审计 fail-closed 的 `-32603` 由 #317 提供，本轮在叠加栈上复验通过（`mcpserver/audit_test.go` 断言 `jsonrpc.CodeInternalError`），`main` 未合入前该场景仍返回非规范 `code:0`——属既有已知项，随 #317 合入闭合。
- 类型错误参数由 JSON Schema 在工具内拒绝（`isError: true`），不落库、不执行查询。
- 租户禁用即失效：`update tenants set status='disabled' where id=2` → 租户 2 的 MCP token 与开放 API token 双入口 401；恢复 `active` 后 200。

### 3. 多语言模板与买家消息草稿

- 注入面：`FillBuyerMsgTemplate` 用 `\{([^{}]+)\}` 正则做纯字符串替换，未命中的变量（含 `{{__proto__}}`、`{{unknown_var}}`）原样保留并计入 `missing`，无表达式求值、无原型链写入；`<script>` 内容原样存储、JSON 转义输出，admin 端 grep 无 `dangerouslySetInnerHTML`，无渲染执行面。
- 授权：readonly 创建模板 403；租户 B 读/改租户 A 模板均 404（不泄露存在性）；readonly 触发语言变体生成 404/403（资源不可见优先）。
- 草稿写路由（编辑 / regenerate / mark-sent / ignore / 批量）见 P1-1。

### 4. 大屏折算与自定义指标配置（在叠加 #318 的审计栈上实测）

| 探针 | 结果 |
| --- | --- |
| readonly `GET /dashboard/screen` | 200（只读接口） |
| readonly `PUT /dashboard/screen/config` | 403 |
| operator `PUT /dashboard/screen/config` | 403（`40305` 仅管理员可管理系统配置） |
| admin `PUT` 未知卡片 / 重复卡片 / 空数组 | 400（`normalizeScreenCards` 白名单 + 去重校验，零落库） |
| `GET /dashboard/screen?shopId=1' OR 1=1--&currency=XXX` | 400（R148 P2-6 已收口为显式 400，非静默降级） |
| `currency=XXX`（无手工汇率） | 200，不可折算金额经 `ScreenMoneyDTO` 原币种显式列出，不静默按 1:1 折算 |
| `currency=USD` | 200 |
| 租户 B admin 读自身 config | 200，仅本租户配置 |

### 5. 常规复跑

| 检查 | 结果 |
| --- | --- |
| `go fmt ./...` / `gofmt -l .` / `go vet ./...` / `go build ./...` | 全绿 |
| `go test ./...`（`APP_ENV=test` + 隔离库 `trademind_test`） | 全绿（含 permmatrix 644 条 route、idor、shopscope、tenant_zero） |
| `pnpm check:dev`、`pnpm check:ui-copy --strict` | 全绿 |
| `pnpm test:frontend` / `test:collector` / `test:contracts` | 355 / 18 / 17 通过 |
| `pnpm build:admin`、`pnpm build:collector` | 成功 |
| `govulncheck ./...` | 0 可达漏洞 |
| `pnpm audit --prod` | 13 条（3 low / 8 moderate / 2 high）：vite ×7、react-router ×2、esbuild、elliptic、@hono/node-server → P2-3 |
| seed / 恢复 / 配置生产拒绝 | 全绿（`demoseed`、`restore`、`config` 生产闸门测试） |

## 【下一步】

### P1（本轮已修复，随 PR 提交，不自行 merge）

**P1-1 仅 `view` 授权的店铺可被写入（越权写）**

- 现象：`user_store_permissions.permission_scope = 'view'` 的 operator 对该店铺的订单与买家消息草稿仍可执行写操作（实测未修复前 `PUT /api/v1/customer/buyer-messages/drafts/:id` 返回 200 且 `content` 被改成 `tampered by view-only`）。根因是这些写路径只做「店铺可见性」校验（`EnsureStoreVisible` / `ApplyStoreScope`），未做「店铺可操作性」校验；R125 已在商品创建线确立 view-only 必须 403 的口径，本批 R149–R158 新增/改造的写路径未跟随。
- 修复口径（与 R125 一致）：可见但只 `view` → 403；不可见 / 跨租户 / 不存在 → 404（不泄露存在性）；admin 与 `operate`/`manage` 授权不受影响；被拒写入零落库。
- 实现：`adminperm` 新增 `Principal.OperableStoreIDs()`、`EnsureStoreOperable()`、`ApplyStoreOperateScope()`；`order` 新增 `findOrderOperable()` + `failOrderWriteScope()`；`customerchat` 新增 `findTenantDraftForWrite()`。覆盖路由：草稿编辑 / regenerate / mark-sent / ignore / 批量 mark-sent；订单创建（含指定 `shopId`）、更新（含改 `shopId` 迁店）、删除、行项增删改、发货单增删改、手工打标 / 批量打标、物流刷新、自动化失败重试、扣减 / 回滚库存、SKU 匹配与 `bind-sku`、打单标记。
- 回归证据：`backend/internal/securitytests/permmatrix/buyermsg_draft_operate_scope_test.go`（先失败：期望 403 实得 200、草稿被改写；修复后全绿，并断言 `operate` 店铺写入仍 200 不过度拦截、view-only 记录零变更）。
- Docker 实测（重建后端镜像，临时把 operator 对某店铺授权降为 `view`，测后还原 `operate`）：草稿 `PUT` / `regenerate` / `mark-sent`、订单 `PUT`、`deduct-inventory`、`match-skus`、`print/mark`、`tags` 全部 403（envelope `40301` `店铺无操作权限`）。

### P2（列清单，不在本轮修改）

- **P2-1 开放 API `severity` 枚举漂移**：schema/文档写 `error/warning`，实现与库表用 `low/medium/high/critical`（实测 `severity=error` 返回空集而非 400）。本轮已把 `ExceptionsPendingIn.Severity` 的 jsonschema 描述与 `docs/open-api.md` 示例改为实际枚举；**仍建议**后续对非法枚举返回 400，避免调用方把空集误读为「无异常」。
- **P2-2 `lowStockOnly` 非布尔值静默按 false 处理**：`lowStockOnly=notabool` → 200 且不过滤，与 `severity` 同类「非法入参静默降级」问题，建议统一收口为 400。
- **P2-3 前端构建工具链依赖告警 13 条（2 high）**：vite ×7、react-router ×2、esbuild、elliptic、@hono/node-server，均为构建/开发期依赖，无生产运行时暴露；建议随下次依赖升级窗口统一提升 umi/vite 工具链版本（本轮未改依赖约束）。
- **P2-4 view-only 403 的业务码不统一**：本轮沿用既有 `ErrStoreNotOperable` 口径返回 `40301`，而 R125 商品创建线返回 `40303`（`CodeStorePermissionDenied`）。语义相同、码不同，建议后续统一为 `40303` 并同步 `docs/permission-matrix.md`。
- **P2-5 入口级拒绝审计为 best effort**：MCP 与开放 API 的 401/429 留痕写失败仅告警（防审计表放大是有意设计），审计库故障期间入口级拒绝可能无痕；若要求强审计链需补偿重试。
- **P2-6 权限矩阵套件依赖 `TEST_DATABASE_URL`**：未配置时静默 skip（R148 已记录），本轮 P1-1 属数据级越权、矩阵探针本就不覆盖 grant scope 维度，建议在矩阵 harness 增补一档 view-only persona 以便未来自动发现同类缺口。

## 【需注意】

- 本轮 P1-1 是数据级越权（同租户内店铺授权粒度），路由级矩阵探针（admin/operator/readonly 三 persona）不可能发现；R125 已定口径但新增写路径未跟随，属「口径未随新面扩散」类漂移，建议把 view-only 断言纳入新增店铺写路由的收尾清单。
- 证据全部在仓库外（`/tmp/r159/`：token/cookie/审计探针输出、`fix_verification.log`、`pnpm_audit_prod.txt`），不入 commit；报告内不含任何完整 token、密码或真实客户数据。
- 本地审计栈提交 `6cdfab60`（#317 + #318）仅用于跑测，未包含在修复 PR 的改动面内；修复 PR 以 `origin/main` 为基，PR 涉及文件与栈内改动无重叠（已逐文件核对）。
- 大屏与 MCP `-32603` 两处结论依赖上述未合并 PR，合并顺序变化时需按 `#317 → #318` 重新复验。
- 修复 PR 由本轮创建但**不自行 merge**。
