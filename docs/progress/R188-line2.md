# R188 线2：安全审计季度复跑前哨——R184–R187 合入面渗透抽验 + 验收包增量（security-engineer + qa-engineer）

日期：2026-08-08。R189+ 安全审计全量复跑前的前哨轮：对 R184–R187 期间合入的四个安全面（审计权限收紧 / mark-paid 限额 UI / 404 遮蔽 / deploy-prod 同机多栈告警）做定向渗透抽验，确认 R183 审计修复面零回退，并补验收包 R185–R187 增量。

## 【结论】

**P0 = 0；P1 = 1（已先红后绿修复）；P2 新登记 3 项。**

- **P1（本轮发现并即修）——写工具「管道前拒绝」审计盲区**：`orders_add_tag` 等写工具在参数校验失败、或被无 `write:ops` 的 token 探测（未注册工具）时，请求在进入 `mcpwrite` 管道前即被拒，而入口审计中间件对写工具整体 `return next(...)`（避免与管道内 dry_run/execute 行重复），导致这类调用**完全不落审计**——攻击者可用非法参数无限探测写白名单成员与租户闸门状态而不留痕。已先红（`r188_write_audit_gap_test.go`）后绿修复（入口层与管道以 context signal 协同：管道未接管时由入口补一行 error 审计，fail-closed），Docker 实测 11 类拒绝路径全部落 1 行、成功链路仍严格 dry_run/execute 各 1 行（零重复）。
- **#369 审计权限收紧**：无旁路。operator/readonly 在 `GET /api/v1/mcp/audit-logs` 全维度（直接 `tool=` 写工具名、`mode=dry_run|execute`、编码/注入型取值、深分页、pageSize 放大）均 0 写行；写行仅 `settings.manage` 可见，且过滤在 SQL 层（非前端隐藏）。非 admin 亦不见 admin 创建的写 token。
- **#372 mark-paid 限额 UI**：服务端权威成立。operator 403/40305、readonly 403/40301，非法/负值/零值/空白/`NaN`/`1e999` 全部使消费侧 fail-closed 拒绝放款，UI 不是唯一防线。登记 P2-1（限额值本身缺服务端范围校验，可存 `1e20` 使单笔上限形同虚设；实际暴露被 `amount ≤ 1e10` 兜底）。
- **#375 404 遮蔽**：无存在性泄露。store-scope 外的**真实存在**会话与**从不存在**的 UUID 在 6 条路由 × 2 角色 × 25 次采样下状态码、响应体（归一化 traceId 后逐字节相同）、响应头完全一致，时序 p50 差 ≤0.5ms（比值 1.03–1.29，均在 ~1.5ms 基线噪声内），无差分信号。
- **#371 deploy-prod 告警**：不产生新攻击面（只读 `docker ps` 标签、仅打印、不改变退出码与部署语义）。登记并**即修**一处终端注入：容器标签值原样打印，含 `ESC`/`CR` 时可重绘该行伪造「一切正常」绿字提示——已加控制字符剥离（`tr -d '\000-\037'`）。
- **#367 / R183 审计修复面零回退**：`TestWriteToolsAuditExactlyOncePerCall`、`TestIsWriteToolCoversWholeWhitelist`、`TestAuditWriteFailureRejectsToolCall`、`TestWriteAuditFailClosed`、`TestWriteQuotasFailClosed`、`TestProcurementWriteCrossTenant404` 全 PASS；Docker 实测一次完整 dry_run→execute 仅新增 2 行审计（本轮修复未引入重复行）。

## 【口径】

- 基线：main（`cce97efa` 后）+ 本地叠加唯一 OPEN 的 #376（R187 线2 性能审计报告，纯文档）；#369–#375 均已合入 main（权威 PR 状态核实：#376 OPEN/mergeable，#369/#370/#371/#372/#373/#374/#375 closed-merged）。
- 环境：`docker-compose.full.yml` 全栈（PostgreSQL 16 / Redis / backend / admin / collector），`MCP_WRITE_ENABLED=true`，`DB_HOST=127.0.0.1 pnpm seed:demo:full` 双租户 demo 数据。
- 探针为本机 HTTP / JSON-RPC 直压 + psql 复核；JWT、MCP token 明文、探针脚本与输出**全部外置附件，不入库**。Actions CI 不作为验收依据。

## 【1. P1：写工具管道前拒绝审计盲区（已修）】

复现（修复前，Docker 实测）：以合法写 token 调 `orders_add_tag {"orderNo":"","tagName":""}`，返回业务错误但 `mcp_tool_call_logs` **零新增**；以 `readonly` scope 的 token 调任一写工具，返回「工具不存在」（避免枚举白名单的既有设计），同样零审计。

根因：`backend/internal/modules/mcpserver/server.go` 审计中间件对 `isWriteTool(name)` 直接放行，把审计责任整体委托给 `mcpwrite.Service.Run`（同事务写 dry_run/execute 行）；但「参数校验失败」「工具未注册」等拒绝发生在 `Run` 之前，两侧都不写。

修复（最小面）：新增 `mcpwrite.WithSignal`/`markReached`（`signal.go`），`Run` 入口即标记「本次调用由管道负责审计」；入口中间件在写工具返回后检查 signal——未被接管则补写一行 `status=error`、`resultSummary="rejected before write pipeline"` 的审计，且**审计写失败即拒绝调用**（沿用 R183 fail-closed 语义）。成功链路完全不变（仍 dry_run/execute 各 1 行）。

回归：`r188_write_audit_gap_test.go`——`TestWriteToolPreflightRejectionAudited`（5 类非法参数，每类恰好 1 行）、`TestWriteToolProbeByReadonlyTokenAudited`（无 `write:ops` token 探测写工具，恰好 1 行）。修复后 Docker 复验 11 类拒绝路径（空参数 ×3、金额负值/三位小数/超 1e10、不存在采购单、payChannel 超长、缺 currency、非法 mode、读工具非法分页）全部 1 行且错误文案未变。

## 【2. #369 审计权限收紧渗透抽验（无旁路）】

| 探针 | admin | operator | readonly | 判定 |
|---|---|---|---|---|
| 默认列表（pageSize=100） | 29 行含 dry_run/execute | 仅读行，0 写行 | 仅读行，0 写行 | PASS |
| `tool=procurement_mark_paid` / `orders_add_tag` 直接指名 | 可见 | 0 行 | 0 行 | PASS |
| `mode=dry_run` / `mode=execute` | 可见 | 0 行 | 0 行 | PASS |
| 深分页 + pageSize 放大（翻遍全部页） | — | 0 写行 | 0 写行 | PASS |
| 编码/注入型取值（URL-encoded 引号、`' OR 1=1`、`%00`） | — | 0 写行、无 500 | 同左 | PASS |
| 旁路面：`/api/v1/mcp/tokens` 列表 | 见写 token | 不见写 token | 不见写 token | PASS |
| 跨租户：tenant2 admin 读审计 | — | — | total=0（tenant1 29 行不可见） | PASS |

过滤位于 `mcpaudit` service 的 SQL（`mode` 为空 + `tool NOT IN (写白名单)`），非前端隐藏；`hideWrite` 由 `settings.manage` 决定，与角色名解耦。

## 【3. #372 mark-paid 限额 UI 渗透抽验】

| 探针 | 结果 | 判定 |
|---|---|---|
| operator `PUT /api/v1/settings`（mcp 限额） | 403 / 40305「仅管理员可管理系统配置」 | PASS |
| readonly 同上 | 403 / 40301「当前账号为只读权限」 | PASS |
| 非 admin 尝试后复核存储值 | 仍为 admin 所设 500 / 2000，未被篡改 | PASS |
| admin 写入 `-1` / `0` / `abc` / `NaN` / `Infinity` / `1e999` / 空白 | settings 接受字符串，但消费侧解析失败 → mark-paid **fail-closed** 拒绝（「金额上限未配置」） | PASS（拒付） |
| admin 写入 `12.3456`（超两位小数） | 同上 fail-closed | PASS |
| admin 写入 `99999999999999999999`（1e20） | 被当作有效上限使用（dry_run 提示「单笔上限 100000000000000000000.00」） | **P2-1** |
| 合法上限 `500` + 真实 dry_run→execute | 正常放行、placed→paid、审计 2 行 | PASS |

结论：写权限与金额判定均在服务端，UI 表单仅为入口；唯一缺口是限额值自身无范围校验（P2-1），且其影响被 `amountCents` 的 `amount ≤ 1e10` 硬上限兜住，不构成越权或绕过。

## 【4. #375 404 遮蔽差分探针（无泄露）】

对象：`1f47c05b…`（角色可见店铺的会话，in-scope）、`ad4b9a1f…`（**真实存在**但属未授权店铺，out-of-scope）、`00000000-0000-4000-8000-000000000000`（从不存在）、`not-a-uuid`（畸形）。路由：`GET /conversations/:id`、`GET …/messages`、`PUT …`、`POST …/mark-replied`、`DELETE …`、`GET /customer-service/conversations/:id`（别名路由）。角色：operator / readonly，各 25 次采样。

- **状态码**：out-of-scope 与 missing 完全一致（operator 404/40401；readonly 写操作先命中只读 403/40301，同样与存在性无关）。
- **响应体**：归一化随机 `traceId` 后**逐字节相同**（`{"code":40401,"message":"not found","data":null}`）。
- **响应头**：无差异（对比集已剔除 `Date`/`Content-Length` 等天然可变项）。
- **时序**：p50 差值 ≤0.5ms、比值 1.03–1.29，落在 ~1.5ms 基线抖动内，无稳定可分辨信号。
- 畸形 UUID 统一 400/40001「invalid id」，不进入存在性判定。
- 副产物：探针的 `DELETE` 循环会真实软删除 in-scope 会话（首次成功、其后 404），已在库内还原并把 `DELETE` 的采样限定在两个 404 用例；这是探针设计问题，非应用缺陷。

## 【5. #371 deploy-prod 同机多栈告警攻击面】

- 告警逻辑只读 `docker ps -a --filter label=com.docker.compose.project=<name>` 的 `working_dir` 标签，**不执行**任何标签内容，不打印 `.env` 或密钥，不改变退出码（`--pre-upgrade-check` 后续必检项照常拦截），也不新增网络/文件访问面。
- 实测：伪造一个同项目名、`working_dir=/tmp/fakestack` 的容器 → 告警如期触发；无同名容器 → 不触发。
- 发现（P2 即修）：标签值由容器创建者控制，含 `ESC`/`CR` 时被原样写入终端，可清行并重绘为绿色「[deploy] 一切正常，请继续」，把警示伪装成放行提示。前提是已能在本机创建容器（≈ docker 守护进程权限），故不属提权，但属可低成本消除的输出注入——已加 `tr -d '\000-\037'` 剥离控制字符，复验注入串以纯文本呈现、告警语义不变。

## 【6. Docker 双租户实测】

`demo_tenant2_admin` 与租户 1 admin 并行核对：订单 5 条（全 `DEMO-T2-` 前缀）vs 38 条（无 T2 前缀）；审计日志 total=0 vs 29；MCP token 列表为空；`mcp` settings 仅见自身租户行；读取租户 1 会话/采购单 404；**租户 1 的 MCP 写 token 对租户 2 订单 `orders_add_tag` dry_run 返回「订单不存在」**（404 语义，不泄露存在性）。

## 【7. 门禁】

`go fmt ./...` / `gofmt -l .`（无输出）/ `go vet ./...` 全绿；`APP_ENV=test TEST_DATABASE_URL=…` `go test ./...` 全包 PASS（含 securitytests、permmatrix 与 `*Postgres` 并发/隔离用例）；`pnpm check:dev`、`check:ui-copy --strict`、`test:frontend`（57 文件 375 例）、`test:contracts`（17）、`test:collector`（18）、`build:admin`、`build:collector` 全绿。

## 【P2 清单（本轮登记，未修）】

1. **mark-paid 限额值缺服务端范围校验**：`PUT /api/v1/settings` 对 `mcp/mark_paid_single_limit`、`mcp/mark_paid_daily_limit` 只做原样存储，UI 的 `min=0.01`/两位小数约束不在 API 侧对应；存入 `1e20` 会使单笔上限失效（仍受 `amount ≤ 1e10` 兜底）。建议在 settings 敏感/受控 key 注册表上挂值域校验，写入即拒。
2. **审计可见性只有「写行/读行」一档**：`hideWrite` 由 `settings.manage` 单一权限决定，非 admin 无法被授予"只看本 token 调用"之类的中间视图；租户内多运营协作场景下要么全看要么全不看。建议后续按 token 归属或独立 `audit.read` 权限细化。
3. **deploy 类脚本的外部字符串输出未统一净化**：本轮只修了 `deploy-prod.sh` 的容器标签一处；`scripts/` 下其他把 `docker`/`git` 输出直接 `printf` 到终端的位置未逐一审。建议下一轮全量复跑时统一收口为一个 `sanitize()` 帮助函数。
4. **大屏卡片配置弹窗在全屏态不可见**（三角色实跑发现）：`/dashboard/screen` 进入浏览器全屏后点齿轮，配置弹窗挂载在 `body` 上被全屏元素遮蔽，需退出全屏才看得到。属演示体验缺陷（非安全面），本轮仅在 DEMO_SCRIPT 注明前置，建议后续把弹窗 `getContainer` 指向全屏容器。

R187 线2 登记的 4 项性能 P2（大屏聚合线性扫描、`mcp_tool_call_logs` 缺 `(tenant_id, created_at)` 复合索引、seedperf 前缀清理、首包预算护栏）本轮未纳入范围，仍挂账。

## 【8. 三角色 DEMO_SCRIPT 实跑（~30 分钟）】

Docker 全栈实跑 admin / operator / readonly + 临时 view-only 账号 + tenant2，逐条对照脚本（录屏与截图外置不入库），结果与失实点见 `docs/acceptance/DEMO_SCRIPT.md`「实跑验证记录」2026-08-08（R188 线2）条目。要点：本轮 P1 修复在 UI 侧坐实——四类拒绝路径各在审计卡片落 1 行 `error`（含 `rejected before write pipeline`），而成功链路仍严格各 1 行无重复；#372 限额表单 admin-only 与 #369 审计收敛在三角色下全部成立。脚本失实 3 项（全屏内弹窗不可见、23b 表格仍写 curl、临时账号 UUID 坑）已即修，并把可复用踩坑沉淀为 `.agents/skills/mcp-write-acceptance/SKILL.md`。

## 【下一步】

- R189+ 安全审计全量复跑时优先覆盖：本轮修复的入口审计协同路径（signal 未被误用于成功链路）、上述 P2-1 值域校验、写配额计数语义。
- #376 合并后本分支的叠加基线即失效，PR 说明中已标注合并顺序。
