# R176 线1：安全审计季度复跑报告

- 轮次：R176 线1（距 R165 安全审计季度复跑 11 轮）
- 审计基线：`origin/main @ 529f4698`（分支 `audit/r176-line1`，直接基于 main；R166–R175 的收口已在 main 内）
- 范围：R165 六处 P1 零回退复验；#322 / #330 复验；新面渗透（#347/#349 校验顺序改动是否引入信息泄露或绕过、40303 envelope 全站一致性、view-only / readonly / 跨租户三轴探针全套）；MCP / 开放 API 抽验（purpose / 限流 / 审计 fail-closed / XFF）；govulncheck 与 pnpm audit 对比基线；密钥脱敏与落库加密抽验
- 依据：① 隔离测试库 `trademind_test`（Docker `trademind-postgres` / `trademind-redis`）上跑生产路由 `api.Register` 的双租户 HTTP 实测（tenant A `910052` / tenant B `910053`，六 persona：admin / operator（授权店）/ readonly / view-only / 跨租户 admin / 平台 admin）；② **Docker 全栈重建镜像实测**（`docker-compose.full.yml` 无缓存重建 postgres/redis/collector/backend/admin 五服务，backend `:18080`、admin `:18000`）；③ 代码审读。证据外置 `/tmp/r176`，不入库；**Actions CI 不作依据**
- 角色说明：`wookat/company-os` 无 `roles/security-auditor`，按最接近的 `roles/qa/code-reviewer.md` + `CHARTER.md` §8 安全口径 + `SOP-04` 执行

## 【结论】

| 项 | 结论 |
| --- | --- |
| R165 六处 P1（审单决定 / 异常标记 / 店铺删除 / 店铺授权凭证与 OAuth / 同步创建与重试 / 刊登目标店） | **零回退**（`securitytests` 三包全绿：permmatrix 35 例、idor、shopscope） |
| #322 view-only 订单写面复验 | 成立（`/orders/:id*`、`/order-items/:itemId*`、买家消息草稿写路由 403 + 40303 + 零落库，路由完整性检查在位） |
| #330 view-only 客服会话写面复验 | 成立（会话族写探针 403 + 40303、detail `canWrite=false`） |
| #335 / #337 / #346 / #347 审单整批 403 与 40303 文案统一 | 零回退（整批拒绝口径在位；全站 `CodeStorePermissionDenied` 出口均为「店铺无操作权限」，`requireCode40303` 断言 message） |
| **#347 / #349「先 scope 后 body」新面渗透** | **发现 1 处 P1 + 1 处 P2 已修**：迁移导入目标店铺缺租户闭合（P1-1）；客服回复族与异常 bind-sku 仍先校验 body（P2-1，本轮一并修） |
| view-only / readonly / 跨租户三轴探针 | 除 P1-1 外无缺口：view-only 写 → 403/40303、readonly 写 → 403/40301、不可见与跨租户 → 404 且零落库 |
| MCP / 开放 API token 治理（purpose 双向隔离 / 撤销 / 非法 purpose / 每 token 限流 / 每 IP 鉴权失败预算 / 审计逐次落库 / fail-closed） | 零回退；补齐一处**测试覆盖缺口**（MCP 入口拒绝 `openapi` 用途 token 此前只有实现无回归测试） |
| XFF / TRUSTED_PROXIES | 零回退（默认空信任列表下逐 IP 伪造 XFF 无法轮换鉴权失败预算：第 11 次起 429） |
| 密钥脱敏 / 落库加密 | **发现 1 处 P1**（`PUT /api/v1/settings` 省略 `isEncrypted` 可把已加密密钥降级为明文落库并原文回显，P1-2，本轮已修）；token 明文仅创建时返回一次、列表只回 `maskedToken`、容器日志无密钥/Token/密码明文 |
| 注入面（SQL / CSV 公式） | 无新增面（GORM 参数化；`csvsafe` 在位） |
| govulncheck ./... | 0 个可达漏洞，与 R165 持平 |
| pnpm audit --prod | 15 条（3 low / 8 moderate / 5 high），全部为 admin 构建工具链（vite / esbuild / image-size / nanoid / elliptic / react-router / @hono/node-server）；相对 R165 基线 13 条 +2（`image-size`×2、`nanoid`、`vite` 新增告警，`react-router` 条目合并），无后端可达面 → 仍为 P2 |

## 【证据】

### P1-1 迁移导入目标店铺缺租户闭合（跨租户 admin 可用他租户 shopId 入库导入批次）

`migrationimport.resolveShop` 对 `principal.IsAdmin()` 直接跳过全部店铺校验，只做 uuid 解析，从未确认该店铺属于调用方租户：

```go
if !principal.IsAdmin() {
    if !principal.CanViewStore(u) { return nil, gorm.ErrRecordNotFound }
    if !principal.CanOperateStore(u) { return nil, errShopNotOperable }
}
return &u, nil   // admin：任意 uuid 直接通过
```

租户 B admin 用租户 A 的 `shopId` 打导入向导（生产路由实测）：

```text
修复前：
POST /api/v1/imports/validate  → 200 {"code":0,...,"validRows":1}      （已进入逐行校验，泄露表头/映射契约）
POST /api/v1/imports/commit    → 200 {"code":0,"data":{"status":"failed"}}（写出 import_jobs 行，shop_id 为他租户店铺）
修复后：
POST /api/v1/imports/validate  → 404 {"code":40401,"message":"店铺不存在或不可见"}
POST /api/v1/imports/commit    → 404 {"code":40401,"message":"店铺不存在或不可见"}
import_jobs 中 tenant_id≠A 且 shop_id=A 店 的行数：0
```

修复：统一以 `adminperm.ApplyTenantScope` 在租户内加载目标店铺（admin 也不豁免），外租户/不存在一律 404，不泄露存在性。业务数据行此前已被 `product.Create` / `order.Create` 的租户校验挡住（故 `status=failed`），本项闭合的是**入口层租户越权与导入批次落库**。

回归测试：`permmatrix.TestMigrationImportShopTenantScope`（先红后绿；红：`validate` 200 / `commit` 200）。

### P1-2 `PUT /api/v1/settings` 可把已加密密钥降级为明文落库

`settings.putOne` 的加密开关完全取决于请求体 `isEncrypted`，省略即按未加密写入并覆盖行上的 `is_encrypted`：

```text
修复前（Docker 全栈实测，tenant 0 admin）：
PUT /api/v1/settings {"items":[{"groupKey":"ai","itemKey":"deepseek_api_key","itemValue":"sk-…"}]}
  → 200；DB settings.is_encrypted = f，item_value = sk-…（明文落库）
GET /api/v1/settings → itemValue 原文回显（不再脱敏）
修复后（重建镜像实测）：
  → 200；DB is_encrypted = t，item_value = 密文；GET 回 "sk-****e222"
```

修复：`putOne` 改为「加密粘性」——`encrypted := it.IsEncrypted || (exists && cur.IsEncrypted)`，请求体无法把已加密项降级；密文/掩码/`PlainByGroup` 解密回读三条链路口径不变。

回归测试：`settings.TestPutBulkCannotDowngradeEncryptedSetting`（先红后绿；红：`is_encrypted` 变 false + 明文落库）。

### P2-1（本轮一并修）回复面与 bind-sku 仍先校验 body 后校验 scope

`#347/#349` 的「先 scope 后 body」在三处未覆盖，view-only / 不可见资源会先拿到载荷校验错误（泄露写入契约、且与全站 40303 口径不一致）：

```text
修复前：
POST /customer/conversations/{view-only 店会话}/mark-replied           {} → 400 40001 回复内容不能为空
POST /customer/conversations/{view-only 店会话}/send-platform-message  {} → 400 40001 回复内容不能为空
POST /orders/exceptions/order_sync_task/{view-only 店任务}/bind-sku    {  → 400 40001 invalid json body
修复后：
  → 403 40303 店铺无操作权限；不可见资源 → 404；可操作店铺仍 400（载荷契约不变）
```

回归测试：`permmatrix.TestCustomerChatReplyScopeBeforeBody`、`permmatrix.TestExceptionBindSKUScopeBeforeBody`（均先红后绿）。

### 复验与抽验（零回退项）

- 权限三轴：`go test ./internal/securitytests/...` 全绿——`permmatrix` 35 例（含 `matrix.json` 路由级四/六 persona 全表探针、R165 六处 P1 契约、view-only 30 条写探针 sweep、审单整批语义、R173/R174 校验顺序）、`idor`、`shopscope`。
- 40303 envelope：全站 `response.CodeStorePermissionDenied` 出口只经 `adminperm.DenyStorePermission` / `FailStoreWriteScope` 或 `店铺无操作权限` 语义 sentinel（`product.ErrDraftShopNotOperable`、`migrationimport.errShopNotOperable`），`requireCode40303` 同时断言 HTTP 403 + 40303 + 文案。
- MCP / 开放 API（Docker 重建镜像实测）：`mcp` 用途 token 打开放 API 401、`openapi` 用途 token 打 `/api/mcp` 401、`both` 双入口 200、非法 purpose 400/40001、撤销后 401、每 token 限流第 11 次起 429 且带 `Retry-After: 1`、鉴权失败预算下逐 IP 伪造 `X-Forwarded-For` 仍在第 11 次起 429、`/api/v1/mcp/audit-logs` 逐次落审计（含 `status=rate_limited` / `openapi:auth`）；审计 fail-closed 由 `openapi.TestAuditWriteFailureRejectsCall`、`mcpserver.TestAuditWriteFailureRejectsToolCall` 覆盖。
- 脱敏：token 明文只在创建响应出现一次，列表/详情只回 `sp_mcp_ro_9d99…0398` 形态；`docker logs` 全量 grep 无 API Key / Bearer token / 密码 / refresh_token 明文。
- 依赖：`govulncheck ./...` 无可达漏洞；`pnpm audit --prod` 15 条见结论表。

## 【下一步】

1. 合并本 PR（不自行 merge，按 CHARTER §8 由项目负责人把关后合并）。
2. 建议把「入口层拿到的 `shopId` / 资源 id 必须经租户 scope 查询确认存在」做成矩阵探针或静态检查：P1-1 与历史多处缺口同源——`IsAdmin()` 提前 return 时漏掉租户维度。
3. `pnpm audit --prod` 的 15 条构建链告警随 admin 工具链升级窗口一并处理（vite / umi 主版本）。

## 【需注意】

- P2 清单（本轮登记不改行为）：
  1. **settings 新建项仍信任 `isEncrypted`**：加密粘性只保护「已加密」的行；首次创建一个敏感 key 且显式传 `isEncrypted:false` 仍会明文落库（现有前端始终传 true，默认种子行已是加密态）。建议下轮引入服务端「声明式敏感 key 注册表」（由各 `*_defaults.go` 的 `encrypted` 声明汇总）强制加密，而非按 key 名启发式（`*_access_key_id` 等非密钥项会被误加密并影响展示）。
  2. `pnpm audit --prod` 15 条 admin 构建链告警（含 `react-router` 开放重定向，仅内部后台、无外部可控跳转入参面）。
  3. 迁移导入行级失败文案（`commit_payments` 行内 error 文本）仍为英文/混排，非 envelope 面（R172 已登记）。
  4. `POST /shops/:id/test-connection`、`publish-targets/check` 等纯探测/纯计算路径对 view-only 放行（R165 已登记，口径未变）。
- 本轮未做：UI 走查与录屏（无前端改动）、`seed:demo:full` 演练（属其他线职责）；view-only / 跨租户三轴的数据级证据来自隔离测试库上的生产路由 HTTP 实测，Docker 全栈实测覆盖 envelope、token 面、两处 P1 的修复前后行为。
- 证据（响应码 / 业务码 / 落库计数 / 容器日志 grep 输出）外置 `/tmp/r176`，不入库；`.env` / `.env.docker` 为本地实测配置，未提交。
