# R148 线2：安全审计季度复跑报告

- 轮次：R148（距 R139 全量安全审计 8 轮）
- 审计基线：`main @ dde63c61`（#289–#300 均已合入 main；任务下发时提到的 #296/#297/#298/#300 实际已合并，非本地叠加，特此注明）
- 范围：MCP 入口（R145 修复复验 + R146 新增边界）、大屏 API、备份定时器/恢复开关、消息回溯开关、常规安全复跑、R139 修复项零回退、双租户 Docker 实测
- 依据：本地实测与代码审读（Actions CI 不作依据）

## 一、结论总览

| 项 | 结论 |
| --- | --- |
| MCP R145 修复项（scope 校验/限流分层/token 上限） | 零回退，测试在位并通过 |
| MCP R146 新增（过期/逐次审计/Redis 限流） | 边界行为符合预期（详见 §2） |
| 大屏 `/dashboard/screen` | 聚合 SQL 全参数化，tenant+shop 双 scope；无注入面 |
| 备份定时器/恢复开关 | 平台租户 only（`RequirePlatformAdminMW`），schedule 表达式白名单解析，无配置注入 |
| 消息回溯开关 | 规则写操作 readonly forbid；estimate 只读且 tenant 强 scope |
| 权限矩阵契约 | **P1：矩阵登记漂移导致契约套件失败（本轮已修复）** |
| R139 4 条 S3 加固 | 零回退（含回归测试） |
| govulncheck | 0 个可达漏洞 |
| pnpm audit | 13 条（2 high），全部为前端/构建工具链依赖，列入 P2 |
| 双租户 Docker 实测 | 见 §6 |

## 二、新攻击面审计

### 2.1 MCP 入口（`POST /api/mcp`）

R145 修复项复验（零回退）：
- scope 强校验：`Authenticate` SQL 条件 `scope = readonly` + 常数时间 hash 比较 + `TenantID >= 0` 双重校验（`mcptoken/service.go`），`TestAuthenticateRejectsNonReadonlyScope` 通过。
- 限流分层：per-token bucket + per-tenant 聚合 bucket（2×）+ 每 IP 鉴权失败预算（1 rps/10 burst），`TestTenantBucketCapsMultiTokenTraffic`、`TestInvalidTokenAttemptsAreRateLimited`、`TestValidTokenNotChargedByAuthFailureBudget` 通过。
- token 上限：`MaxActiveTokensPerTenant = 20`，`TestCreateEnforcesActiveTokenCap` 通过。

R146 新增边界：
- 过期边界：SQL 层 `expires_at > now` 严格判定 + Go 层 `row.Expired(now)` 二次判定，边界时刻（now == expires_at）拒绝，fail-closed；Stateless 模式下每次请求重新认证，无会话内过期残留。`TestExpiredTokenRejectedAtEntry`、`TestAuthenticateHonorsExpiry` 通过。
- 审计链：每次 `tools/call` 逐次落库（工具名/租户/token/状态/耗时，不记参数与结果）。审计写失败为 best-effort（仅 `slog.Warn`，不阻断调用）——列 P2-1。
- Redis 宕机降级：`RedisLimiter` 脚本执行失败时回落到同策略进程内 bucket，**不 fail-open**；`TestRedisLimiterFallsBackWhenRedisDown`、`TestRedisLimiterSharedAcrossInstances`、`TestRedisLimiterHasBudgetDoesNotConsume` 通过。限流 key 经 SHA-256 截断，无 PII 入 Redis key。
- `DisableLocalhostProtection: true`：SDK 的 localhost/Origin 检查针对浏览器 DNS-rebinding；本入口每请求都要求 Bearer token（无 cookie 鉴权），关闭该检查风险可接受，代码内已注明理由。

### 2.2 大屏 `GET /api/v1/dashboard/screen`

- 所有聚合 SQL（漏斗/趋势/待办/告警）均为参数化查询，`shopId`/`platform` 只进入绑定参数；tenant 经 `applyTenantColumn`、shop 经 `applyShopColumn(IN allowed)` 双重限定；非 admin 无店铺授权时 `1 = 0` fail-closed。
- 权限矩阵：四 persona 均 allow（只读接口），已登记。
- 双租户实测：租户 B 管理员读取租户 A 订单 ID → 404；`shopId` 传入租户 B 的店铺 UUID（对租户 A 会话）→ 订单计数 0，无跨租户数据回显；`shopId=1' OR 1=1--` → 200 且无异常放大，SQL 层未受影响。
- 例外（口径不一致，非跨租户泄露）：
  - `today.paidOrderCount/salesBase/grossProfitBase` 由 handler 调 `Reports.ProfitReport` 补齐，未传入 `shopId`/`platform`，因此这三项忽略店铺筛选（实测：同一会话切换 4 个 shopId，`orderCount` 变化 12/0/0，而 `paidOrderCount` 恒为 28）——P2-2。
  - 非法 `shopId`（如 `bogus`、不存在 UUID）被 `shopFilterUUID` 静默丢弃而非 400，库存告警块退化为「全部授权店铺」（实测 `bogus` 时告警恢复为租户级）——P2-6。租户与授权店铺 scope 仍生效（readonly 会话 11 vs admin 28），无越权放大。

### 2.3 备份定时器与恢复开关

- 路由：`/ops/*`（backup/restore/release）挂在 `RequirePlatformAdminMW` 下，仅平台租户（tenant 0）管理员可达；矩阵中其余 persona 全部 forbid，探针通过。
- `BACKUP_SCHEDULE` 由自研 `backupsched.Parse` 解析（cron 5 段/duration 白名单语法），非法表达式启动即报错退出，无 shell/注入面；调度幂等以 UTC 分钟级 `schedule_key` + 唯一约束兜底。
- 恢复开关：生产 target 硬拒绝；生产演练需 `BACKUP_RESTORE_ALLOW_PRODUCTION=true` 且 target 库名强制 `trademind_p6v_restore_` 前缀、非空、隔离，需二次认证 + 高危确认，backup 需 completed+verified+checksum；加密备份需密钥引用。`production_gate_test.go` 通过。

### 2.4 消息回溯开关

- 规则创建/更新（含 `backfill` 字段）走 `RequireWritable`，readonly 403（矩阵探针 + 实测 `POST /customer/buyer-message-rules` → 403）。
- `GET .../backfill-estimate` 只读、不产生草稿，tenant 经 `adminperm.TenantIDFromGin` 强 scope，node 白名单校验；`buyermsg_scope_test.go` 双租户隔离通过。实测租户 A readonly `estimated=28`、租户 B admin `estimated=1`，租户隔离成立。
- **发现 P1-1**：该路由（两个挂载点）未登记进权限矩阵 registry，且矩阵中 `POST /api/mcp` 因 harness 未启用 `MCPEnabled` 被判 stale，导致 `TestRouteRegistryComplete` 失败（契约套件红）。本轮已修复：登记两条 backfill-estimate 条目（4 persona allow，只读）+ harness 启用 `MCPEnabled`。该漂移自 R142/R144 引入（矩阵套件需 `TEST_DATABASE_URL` 才执行，因此长期未暴露）。

## 三、常规复跑结果

| 检查 | 结果 |
| --- | --- |
| 跨租户/越权契约（permmatrix 633+2 条 route）| 修复 P1-1 后全绿 |
| readonly 403 路由级写保护 | 通过（矩阵探针） |
| tenant 0 闸门（tenant_zero_test） | 通过 |
| IDOR / shop scope 套件 | 通过 |
| CSV 注入 | 6 个导出模块均经 `internal/pkg/csvsafe` 处理，通过 |
| XSS | admin/src 无 `dangerouslySetInnerHTML`/`innerHTML`/`eval`，通过 |
| 密钥脱敏与日志泄露 grep | 无完整密钥/token/密码输出；备份错误信息 `[redacted]` 替换在位 |
| seed 生产拒绝 | `ErrProductionForbidden` + seed/clean/verify 三态生产拒绝测试通过 |
| govulncheck | 0 可达漏洞（1 个 require-only 漏洞不可达） |
| pnpm audit（含 MCP SDK 依赖面） | Go MCP SDK（v1.7.0）无告警；13 条 JS 告警全在 vite/esbuild/launch-editor/react-router/elliptic/@hono/node-server 等构建/开发工具链，无生产运行时暴露 → P2-3 |
| go vet / gofmt / go test ./... | 全绿 |

## 四、R139 修复项零回退核对

1. 备份上传错误落库前 S3 AK/SK 替换 `[redacted]`：在位（`backup/upload.go`），`upload_test.go` 断言通过。
2. 生产 S3 endpoint 强制 HTTPS + 拒绝 loopback/link-local（169.254 元数据）：在位（`config/p6_config.go`）。
3. 保留清理要求非空 `BACKUP_STORAGE_PREFIX` 且只删 `bk_*.dump(.enc)`：在位（`backup/upload.go`）。
4. 对象存储取回校验 `LocalPath` 在备份工作目录内：在位（`ensureUnderWorkRoot`），`object_fetch_test.go` 通过。

## 五、问题清单

### P1（本轮已修复，随 PR 提交）
- **P1-1 权限矩阵 registry 漂移**：`backfill-estimate` 两路由未登记 + harness 缺 `MCPEnabled` 致 `POST /api/mcp` 被判 stale，安全契约套件 `TestRouteRegistryComplete` 失败。修复：矩阵登记 + harness 启用 MCP。

### P2（列清单，不在本轮修改）
- **P2-1 MCP 审计写失败仅告警**：审计为 best-effort，审计存储故障期间工具调用继续且无审计行（有 `mcp_tool_audit_write_failed` 日志）。若要求强审计链，可改为失败即拒绝调用或补偿重试。
- **P2-2 大屏 today 销售/毛利口径忽略 shopId/platform 筛选**：`handler.go` 调 `ProfitReport` 未下传筛选，导致同一响应中 `orderCount` 已按店铺过滤而 `paidOrderCount/salesBase/grossProfitBase` 仍是租户级，属口径不一致（非越权）。
- **P2-3 前端工具链依赖告警 13 条（2 high）**：launch-editor（dev server 命令注入，仅 Windows dev）、vite `server.fs.deny` bypass 等，均为构建/开发期依赖；建议随下次依赖升级窗口统一提升 umi/vite 工具链版本。
- **P2-4 MCP token 创建数量上限存在 count→insert 竞态**：同租户并发创建可短暂超过 20 上限（自伤性，攻击价值低）。可用唯一约束/事务锁收口。
- **P2-5 MCP token 创建并发上限竞态缺回归测试**：建议补并发回归测试固化 P2-4 的修复口径。
- **P2-6 大屏非法 shopId 静默降级**：非法/不存在的 `shopId` 不报 400，库存告警块退化为全部授权店铺，易造成看板读数误判；建议 `shopFilterUUID` 解析失败时返回 400。

## 六、双租户 Docker 实测

- `docker compose -f docker-compose.full.yml up -d --build` + `DB_HOST=127.0.0.1 pnpm seed:demo:full`（含 DEMO-第二租户）。
实测结果（HTTP 探针，全部在本地容器栈上执行）：

| 探针 | 结果 |
| --- | --- |
| 租户 B admin 读租户 A 订单 ID ×2 | 404 / 404 |
| 租户 B `/orders` 列表 | 200，仅 `DEMO-T2-*` 数据 |
| readonly `POST /products` / `PUT /settings` / `POST buyer-message-rules` | 403 / 403 / 403 |
| readonly `POST /mcp/tokens` | 403 |
| 租户 A/B admin `GET /ops/backups`、`/ops/restores`、`/ops/dr/status` | 全 403（平台管理员闸门成立）|
| MCP 无 token / 伪造 token | 401；连续失败 4 次后进入 429（每 IP 鉴权失败预算生效）|
| MCP `tools/list` | 仅 4 个只读工具：`orders_query`/`inventory_query`/`exceptions_pending`/`report_summary`；`order_update` → `unknown tool` |
| MCP `orders_query` 双租户 | A 返回 36 条 `DEMO-*`，B 返回 5 条 `DEMO-T2-*`，无交叉；`customerName` 脱敏为 `D**` |
| MCP 参数注入 | `status="paid' OR 1=1--"` → 空结果集；未声明参数 `shopId` 被 JSON Schema 拒绝 |
| MCP per-token 限流 | 连续 20 次有效调用出现 429（分层限流生效）|
| MCP 审计 | 每次调用逐条落库，含失败调用（`order_update` → `status=error`）；租户 A/B 审计日志互不可见；仅记 masked token，无明文 |
| MCP token 吊销 | 吊销后复用 → 401；租户 B 吊销租户 A token → 404 |
| 后端容器日志 grep（400 行）| 无明文 token/JWT/Bearer/密码/AK-SK/Set-Cookie |

收尾：`seed:demo:full:clean` + `seed:demo:full:verify` → `zero DEMO- residual rows`。

## 七、遗留与建议

- R139 全量审计报告与 R145 线2 MCP 安全交叉审查报告未在 docs/ 归档（仅 PROGRESS.md/PR 记录），本轮起以 `docs/SECURITY_AUDIT_R148.md` 形式归档，建议后续轮次沿用。
- 权限矩阵契约套件依赖 `TEST_DATABASE_URL`，本地/CI 未配置时静默 skip，这是 P1-1 漂移长期未暴露的根因；建议在 CI 中提供测试库使该套件常绿（Actions CI 虽不作审计依据，但可作为漂移预警）。
