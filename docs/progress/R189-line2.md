# R189 线2 — 安全审计季度全量复跑（security-engineer）

## 【1. 结论】

季度全量复跑（距 R183 六轮）发现并即修 **2 项 P1**（同源：店铺授权集"空集 = 无限制"混淆导致租户内跨店越权读；`LoadPrincipal` 数据库错误返回 nil principal 使调用点降级为无限制），**无 P0**，新登记 / 承接 **6 项 P2**。MCP 写全链 W1–W3、四 persona 权限矩阵、settings 敏感 key 与 token 治理、开放 API 面、R187/R188 性能改动面（SQL 下推 / 复合索引）均未发现新的 P0 / P1，`govulncheck` 0 个可达漏洞。完整报告见 `docs/SECURITY_AUDIT_R189.md`。

## 【2. 审计口径】

`#374`、`#375` 已合入 `main`；`#376`（R187 线2）、`#377`（R188 线1）、`#378`（R188 线2）仍 open 且 mergeable，审计分支自 `origin/main` 按 `#376 → #377 → #378` 顺序叠加，冲突仅 `docs/PROGRESS.md`（段落并存，无代码冲突）。Actions CI 不作依据；证据文件（红绿测试输出、govulncheck / pnpm audit 日志、Docker 镜像红绿探针日志）作附件不入库。

## 【3. P1 修复（先红后绿）】

`AllowedStoreIDs()` / `OperableStoreIDs()` 的契约是 `nil` = 无限制（admin）、非 nil 空集 = 无任何店铺授权。本轮发现契约在两侧同时被破坏：

- **P1-1 空授权集被当作无限制**（无需任何故障即可触发，`readonly` 账号默认零授权）：
  - `taskcenter/service.go` 仅在 `len(AllowedShopIDs) > 0` 时过滤店铺 → 零授权账号读到本租户全部店铺的失败任务（六类任务的店铺 ID / 平台 / 错误详情 / 关联资源标题）；
  - `operationdashboard/service.go` 库存计数、`operationdashboard/screen.go` 大屏库存预警在同样条件下落回租户级查询。
  - 修复：三处按 `nil` / 空集分别处理，空集一律零结果。
- **P1-2 principal 解析失败降级为无限制**：`adminperm.LoadPrincipal` 在非 `ErrRecordNotFound` 的数据库错误上返回 `(nil, err)`，忽略 error 的调用点把 nil 当作 admin/无限制——`inventory/handler.go` 的 `operableShopIDs` 更是**写侧**（返回 nil = 可写全部店铺）。修复：改为返回 `RoleReadonly + Disabled` 的拒绝一切 principal 与原 error（不入 context 缓存）；`reports/procurement_report.go` 的 `allowedShopIDs` 由"错误即 nil 作用域"改为传播 error。

先红证据：HTTP 层用例 `internal/securitytests/permmatrix/r189_empty_store_scope_test.go`（走生产 `api.Register` 全路由，修复前 readonly 读到两个店铺的失败行）+ 六包 fail-closed 单元用例（修复前 `principal=<nil> err=sql: database is closed`）。新增测试助手 `internal/testing/faildb`（已关闭的连接池，无真实凭据）提供确定性数据库故障，不依赖外部服务。

## 【4. Docker 重建镜像实测】

`docker build` 分别重建修复前（`trademind-backend:r189-pre`，基线 HEAD）与修复后（`trademind-backend:r189`）镜像，接 Docker PostgreSQL（独立库）+ Redis 启动，走真实登录链路（bootstrap admin → 创建零授权 `readonly` 账号 → 两店铺各 1 条失败订单同步任务）后调 `GET /api/v1/task-center/failures`：

| 镜像 | readonly（零授权） | tenant admin |
| --- | --- | --- |
| `r189-pre` | rows=2 / 2 个店铺（越权坐实） | rows=2 |
| `r189` | rows=0 / 0 个店铺 | rows=2 |

## 【5. 复验面（零回退）】

- MCP W1–W3：三层闸门、scope 隔离、确认 token 四元绑定与重放 / 参数漂移、advisory lock + 事务内额度、审计恰一次与 fail-closed、`#378` 管道前拒绝补写（成功链路无重复行、未进入管道才补写）全部保持；`markReached` 位置确认无新盲区。
- 四 persona × 写接口探针、跨租户 404、40303 / 40301 口径、审计页写行 admin-only：`permmatrix` + `idor` + `shopscope` 全量 192 例通过、0 skip。
- settings 敏感 key 注册表 / 加密黏性 / 掩码不回写 / 惰性收编；token purpose 双向隔离、过期、吊销、限流与受信代理 XFF；开放 API 仅 5 个 GET 且租户闭合。
- R187/R188 性能面：`summary_sql.go` 全参数化、租户与店铺谓词齐备、空集 fail-closed、`NOT EXISTS` 标记过滤无跨租户读取、`migrate_round188.go` 固定标识符且幂等——未引入注入 / 越权 / 租户闭合缺口。

## 【6. 门禁】

`gofmt -l .` 无输出、`go vet ./...`、`go build ./...`、`go test ./...`（APP_ENV=test + Docker PostgreSQL + Redis DB15）全包通过；MCP + securitytests verbose 复跑 192 PASS / 0 FAIL / 0 SKIP；`govulncheck ./...` 0 可达漏洞；`pnpm audit --prod` 16 项（全为 `@umijs/max` 传递链）；`pnpm test:contracts` 通过。本轮未改前端，故未跑 UI 门禁与构建。

## 【7. P2 清单（未修）】

1. `mcpwrite.auditReject` 写审计失败仅落日志：请求本就被拒（无未审计的写），但审计库故障期间可无痕探测闸门 / 限额。建议补记失败时清除 signal，让入口层 fail-closed 语义覆盖拒绝路径。
2. `SummaryOpenExceptions` 在 `TenantID == nil` 且授权集为 nil 时无租户谓词（当前调用点均显式传租户）：建议聚合入口强制非 nil 租户。
3. `GO-2026-5932`（`x/crypto/openpgp` 无维护、无修复版本，不可达）随 `x/crypto` 升级复查。
4. 前端 16 项 advisory 全为 `@umijs/max@4.6.83` 传递链（含 umi 自带 `react-router@6.3.0`），需评估大版本升级或 overrides。
5. `operationlog` / `product` / `order` 的游标指纹在 principal 解析失败时按空集计算（实际过滤由传播 error 的 `ApplyStoreScope*` 负责，非越权），建议统一传播 error。
6. 承接 R188 未修 P2：mark-paid 限额值缺服务端值域校验（本轮复核仍未加）、审计可见性只有"写行/读行"一档、`scripts/` 外部字符串输出未统一净化。

## 【8. 下一步】

- 本 PR 不自行 merge；`#376`–`#378` 合并后叠加基线即失效，需按最终合入内容重跑 `permmatrix` 与 MCP 写用例。
- 建议下一轮优先处理 P2-1（拒绝路径审计 fail-closed）与 P2-6 的限额值域校验，并把"nil = 无限制 / 空集 = 无授权"契约写入 `adminperm` 包注释与 code-quality 检查项，防止新调用点再次用 `len(ids) > 0` 判断。
