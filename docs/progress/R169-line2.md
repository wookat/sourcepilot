# R169 线2：MCP / 开放 API token 治理季度复查（qa-engineer）

## 结论

Docker 全栈（main + 未合并 #335/#337 叠加，#330 已合入 main）实测 token 全生命周期、限流、审计 fail-closed、TRUSTED_PROXIES/XFF、MCP 4 工具与开放 API 全端点跨租户隔离、R165 view-only 六处修复面联动、R148/R153/R159 历史修复项：**全部通过，未发现 P0/P1 安全缺陷**。本轮无需安全修复代码；仅修掉叠加栈遗留的合并冲突标记（测试与文档文件）。

## 环境与叠加栈

- `docker-compose.full.yml` 全栈（backend/admin/collector/postgres/redis），镜像由本地叠加栈构建。
- 叠加栈：`origin/main`（含已合入的 #330）→ #337 分支（自带 #335 栈）→ merge `origin/main` → merge #335 tip；冲突按 R166/R167 定案口径解（审单整批 403/40303、`EnsureStoreOperable`）。
- 种子：`pnpm seed:demo:full`（租户1 demo 三角色 + 租户2 demo_tenant2_admin）。
- 配置：`MCP/OPENAPI_RATE_RPS=5, BURST=10`，`TRUSTED_PROXIES=`（默认空）。

## 测试矩阵与结果（全部实测，证据留会话附件不入库）

### A. token 全生命周期
| 用例 | 结果 |
| --- | --- |
| 创建仅返回一次明文；列表/DB 均为脱敏与 SHA-256 hash（hash 与明文比对一致） | PASS |
| `expiresInDays` 非法（-1 / 731）→ 400/40001；到期后 MCP/开放 API 均 401，列表 `expired=true` | PASS |
| 吊销后 MCP 与开放 API 双入口 401/40101（幂等） | PASS |
| purpose 双向隔离：`mcp` 仅 MCP、`openapi` 仅开放 API、`both` 双通、非法 purpose 400/40001 | PASS |
| 每租户上限 20：30 并发创建仅放行至 20（advisory lock 生效），第 21 个 400/40001 中文提示 | PASS |

### B. MCP 4 工具 / 开放 API 端点
| 用例 | 结果 |
| --- | --- |
| `tools/list` 恰为 4 只读工具；4 工具双租户全部可用 | PASS |
| 跨租户隔离：双租户订单集合零交集；开放 API 订单详情跨租户 404/40401 | PASS |
| 开放 API orders/inventory/reports/summary/exceptions 全端点 200；写方法（POST/PUT/DELETE）404 | PASS |
| readonly token 打 `/api/v1` 管理路由一律 401（无写通道、无管理面泄漏） | PASS |
| 非法参数：page=0/非整数/非法日期/非法枚举/lowStockOnly 非布尔 → 400/40001；pageSize>100 按文档截断 | PASS |
| 脱敏抽验：输出无密钥/密码/token/内部 UUID（仅 traceId），客户名仅首字符（`D**`） | PASS |

### C. 限流 / 审计 / XFF
| 用例 | 结果 |
| --- | --- |
| 突发 25 请求：MCP 与开放 API 均 10×200 + 15×429（code 42901，`Retry-After: 1`）；Redis 中 6 类 `ratelimit:*` 分层桶（token/租户/authfail × 两入口） | PASS |
| Redis 停机降级：30 并发 12×200 + 18×429（进程内 fallback 生效，不 fail-open）；Redis 恢复后正常 | PASS |
| 逐次审计：每次 tools/call 与开放 API 请求各落 1 行 `mcp_tool_call_logs`（tool/status/token 脱敏） | PASS |
| 审计 fail-closed：审计表不可用时 MCP -32603 `audit log unavailable`、开放 API 500/50000 且不返回数据；恢复后自愈 | PASS |
| `TRUSTED_PROXIES=`（默认）：轮换 XFF 无法绕过每 IP 失败认证限流（40 次伪造 XFF → 30×429）；设为受信后 XFF 生效且同 XFF 仍限流 | PASS |

### D. 历史修复项零回退（R148/R153/R159 + R165 联动）
| 用例 | 结果 |
| --- | --- |
| R153：租户 disabled 即双入口 401，恢复后可用 | PASS |
| R148：限流分层（token/租户聚合/authfail）在位；token 上限并发（含 `TestCreateCapConcurrencyPostgres`） | PASS |
| R159/R165：view 授权店铺审单整批 403/40303「店铺无操作权限」，零落库 | PASS |
| readonly persona：token 列表可读、创建/吊销 403/40301 | PASS |
| 契约套件：`permmatrix`（view-only sweep 30 探针、R165 六用例、R167 整批、矩阵契约）、mcptoken/mcpserver/openapi/mcpaudit/idor/shopscope 全绿 | PASS |

## P0/P1

无。

## 本轮改动（非安全修复）

- 叠加栈遗留合并冲突标记清理：`backend/internal/securitytests/permmatrix/view_only_sweep_test.go`、`docs/PROGRESS.md`、`docs/permission-matrix.md`（保留整批 403/40303 与 R168 文案统一口径）。
- 新增本文件。

## P2 清单

1. MCP 工具分页参数为钳制语义（`NormPage`：page<1 归 1、pageSize>100 截断），与开放 API「非正整数 400/40001」口径不一致；建议在 `docs/mcp.md` 明示或统一口径。
2. Redis 停机期间每请求先等 Redis 超时再走 fallback，串行低速请求因超时间隔天然不触发限流（并发下 fallback 正常拒绝）；建议对 Redis 客户端设置更短超时/熔断，降低降级期延迟放大。
3. `mcp_api_tokens` 上限计数含租户 0（`admin@example.com` 无租户体系）路径未单测覆盖（现网默认租户即 tenant>0，风险低）。
4. 权限矩阵套件仍依赖 `TEST_DATABASE_URL` + `APP_ENV=test` 手工配置（R148 P2 延续）。

## 限制

- Actions CI 不作为依据；以上全部为本地 Docker 全栈与本地 go test 实测。
- 证据（脚本与输出日志）作为会话附件提交，不入库。
