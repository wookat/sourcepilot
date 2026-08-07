# R165 线2：安全审计季度复跑报告（攻击者视角独立复验）

- 轮次：R165 线2（距 R159 安全审计季度复跑 6 轮）
- 审计基线：`main @ 15d0b3be` + 叠加 PR #330 head `a79a88af`（R164 线2 客服会话 view-only 收口，审计时尚未合并；该栈本地叠加实测）
- 范围：R159 修复项零回退核实；#322 / #330 修复面复验；**view-only / readonly 双轴权限全面渗透**（与 R165 线1 并行，本线从攻击者视角独立枚举写路由，不复用线1 实现）；40303 / 40301 / 400 口径一致性；大屏折算与自定义指标配置写接口 scope；币种/汇率设置写接口租户隔离（含 R164「PUT /settings 数值型静默忽略」评估）；MCP / 开放 API token 治理回归；限流 / XFF；审计 fail-closed；注入面（SQL / CSV 公式 / XSS）；govulncheck / pnpm audit 增量
- 依据：Docker（`trademind-postgres` / `trademind-redis`）+ 隔离测试库 `trademind_test` 上跑生产路由 `api.Register` 的双租户 HTTP 实测（tenant A `910052` / tenant B `910053`，六 persona：admin / operator（授权店）/ readonly / view-only / 跨租户 admin / 平台 admin）+ 代码审读。证据外置 `/tmp/r165`，不入库；**Actions CI 不作依据**
- 角色说明：`wookat/company-os` 仓库无 `roles/security-auditor` 文件，本轮按最接近的 `roles/qa/code-reviewer.md` + `CHARTER.md` 安全分级 + `SOP-04` 执行

## 【结论】

| 项 | 结论 |
| --- | --- |
| R159 修复项零回退（token purpose 隔离 / 跨租户 / 限流 / XFF / 审计 fail-closed / 脱敏 / 生产闸门 / 店铺 scope 写权限 / 大屏 scope） | 零回退（`mcpserver`/`mcptoken`/`mcpaudit`/`openapi`/`middleware`/`securitytests` 全套件绿） |
| #322 view-only 订单写面复验 | 成立（`/orders/:id*`、`/order-items/:itemId*`、买家消息草稿写路由 403 + 40303 + 零落库；路由完整性检查在位） |
| #330 view-only 客服会话写面复验（叠加分支） | 成立（会话族 13 条写探针 403 + 40303、detail `canWrite=false`） |
| **view-only / readonly 双轴渗透（本线独立枚举写路由）** | **发现 6 处 P1 越权（订单审单决定 / 异常工作台标记 / 店铺删除 / 店铺授权凭证写入与 OAuth 刷新撤销 / 同步任务创建与重试 / 商品刊登目标店），本轮已全部修复 + 回归测试** |
| 跨租户写面 | 除异常工作台标记（P1-2，已修）外无缺口；店铺删除、审单、刊登、同步均 404 且零落库 |
| 40303 / 40301 / 400 口径一致性 | 修复后一致：店铺可见但仅 `view` → 403/40303；全局 readonly → 403/40301；仅参数非法 → 400；不可见/跨租户 → 404（不泄露存在性） |
| 大屏折算 / 自定义指标配置写接口 scope | 成立（readonly → 40301；operator / view-only → 40305「仅管理员可管理系统配置」；未知/空/非法卡片 400） |
| 币种 / 汇率设置写接口租户隔离 | 成立（readonly 40301、operator 40305；租户 A 写入不影响租户 B 读回；非法汇率 / SQL 型币种代码 400 且无注入面） |
| R164「PUT /settings 数值型静默忽略」安全面评估 | **非安全面**：当前实现对数值型 `itemValue` 整体返回 400 `invalid body`（不是静默写入他值）；请求体 `tenantId` 为 advisory，写入一律落 JWT 租户（`tenantId:0` 与 `tenantId:910053` 注入均写回 `910052`），不存在跨租户/租户 0 改写 |
| MCP / 开放 API token 治理回归（purpose / 撤销 / 过期 / 租户禁用 / 数量上限 / 401 / 429） | 零回退 |
| 限流 / XFF / fail-closed | 零回退（伪造 XFF 不重置额度；审计 fail-closed `-32603` 套件绿） |
| 注入面（SQL / CSV 公式 / XSS） | 无新增面（GORM 参数化；`csvsafe` + 财务对账 CSV 注入测试在位；币种代码/卡片 key 白名单化 400） |
| govulncheck ./... | 0 个可达漏洞（1 条 require-only 不可达），与 R159 持平 |
| pnpm audit --prod | 13 条（3 low / 8 moderate / 2 high），全部为前端构建工具链（vite / esbuild / elliptic / react-router / @hono/node-server），无运行时可达面 → 与 R159 P2-3 同状态，无增量恶化 |

## 【证据】

### P1-1 订单审单决定只校验店铺可见性（view-only 可放行/拒单）

`POST /order-review/approve|reject` 接受客户端提交的 `orderIds`，逐单只做 `EnsureStoreVisible`：

```text
view-only（仅 view 授权店）→ POST /api/v1/order-review/approve
  修复前：200 {"done":1}，orders.review_status: pending_review → approved（真实落库）
  修复后：403 {"code":40303}，review_status 保持 pending_review
```

修复：批量决定前置 `ensureReviewBatchOperable`（整批拒绝，避免部分生效），逐单 `EnsureStoreVisible` → `EnsureStoreOperable`；handler 复用 `failRuleShopScope` 映射 403/40303。不可见店订单仍是逐行「订单不存在」（200 信封、零应用），不泄露存在性；授权 operator / admin 放行不变。

### P1-2 异常工作台标记路由无租户/店铺 scope（跨租户 + view-only 可写）

`/orders/exceptions/:sourceType/:sourceId/{handle,ignore,mark,bind-sku,retry-deduct,retry-inventory-sync}` 仅有 readonly 守卫，`resolveOrderPointers` 按裸 id 查询：

```text
view-only → POST .../order/{tenantA order}/handle        修复前 200（落 order_exception_marks）
tenant B admin → POST .../order/{tenantA order}/handle   修复前 200（跨租户写入）
tenant B admin → DELETE .../order/{tenantA order}/mark   修复前 200（跨租户删除标记）
  修复后：view-only 403/40303；跨租户 404；order_exception_marks 零新增
```

修复：新增 `orderexception/source_scope.go`——`resolveSourceScope` 按 source 类型（order / order_item / order_item_sku_match / order_inventory_effect / inventory_sync_task / order_sync_task）解析其**真实租户与店铺**，未知/不可解析 source 一律 `gorm.ErrRecordNotFound`（fail-closed）；`EnsureSourceOperable` 先比租户再 `EnsureStoreOperable`；handler 侧 `denyScope` 对全部六条写路由统一前置（404 / 403+40303）。

### P1-3 店铺删除只按租户过滤（同租户无授权/仅 view 也可删店）

```text
operator（对该店无任何授权）→ DELETE /api/v1/shops/{ungranted}
  修复前：200，shops 行被软删（alive=0）
  修复后：404，alive=1
view-only → DELETE /api/v1/shops/{viewOnly}   修复前 200 → 修复后 403/40303，alive=1
tenant B admin → DELETE                        修复前后均 404（租户过滤本已在位）
```

修复：`shop.Service.Delete` 删除前 `EnsureStoreOperable`（与 `Update` 对齐），handler 映射 `ErrStoreNotOperable` → 40303。

### P1-4 同步任务写路径把「可见」当「可操作」

`POST /shops/:id/sync-orders`、`POST /shops/:id/sync-customer-messages` 仅 `EnsureStoreVisible`；`order-sync/tasks/:id/retry` **完全没有店铺闸门**（只按租户）。二者都会真实调用平台并落任务：

```text
view-only → POST /shops/{viewOnly}/sync-orders                 修复前放行 → 修复后 403/40303
view-only → POST /shops/{viewOnly}/sync-customer-messages      修复前放行 → 修复后 403/40303
view-only → POST /order-sync/tasks/{failed task}/retry          修复前无店铺闸门 → 修复后 403/40303，task.status 保持 failed
```

修复：`ordersync` / `customersync` 的 `CreateShopSync` 与 `RetryFailed` 改用 `EnsureStoreOperable`（retry 在状态校验前先过店铺闸门），handler 新增 `failSyncStoreScope` 统一 404 / 403+40303。这同时闭合 R164 线2 P2 第 4 项（该项当时登记为「下轮统一定口径」，本轮定为**同步属店铺业务写，须 operate**）。

### P1-5 商品刊登目标店无操作权限校验

`products/:id/publish`、`products/:id/publish-targets/create-drafts`、`product-publish/batch-targets/create-drafts` 只解析 `shopId` 不校验店铺授权；抖店 SKU 绑定写路径（`sync-sku-bindings` / `bind-sku` / `unbind-sku`）只做 `EnsureStoreVisible`：

```text
view-only → POST /products/{id}/publish-targets/create-drafts（target=viewOnly 店）
  修复前：200，product_publish_batches 落批次（仅被「刊登配置未完成」业务校验拦在平台调用前）
  修复后：403/40303，batches 零新增
```

修复：`ensureTargetsOperable`（逐 target 店铺 `EnsureStoreOperable`）接入单品/批量 create-drafts；`CreatePublishTask` 前置店铺闸门；抖店绑定写路径改走新增 `loadDouyinPublicationForWrite`（读路径仍为 visible）；handler 统一 `failPublishStoreScope`（404 / 403+40303）。

### P1-6 店铺授权凭证写路径把「可见」当「可操作」（view-only 可改写/撤销平台凭证）

`shop.findScopedShop` / `ensureShopScoped` 只做租户 + 可见性校验，却同时被授权**写**路径复用：`PUT /shops/:id/auth`（落加密 AppSecret / AccessToken / RefreshToken）、抖店 `oauth/douyin/{refresh,revoke,sync-shop-info}`、TikTok / Shopee / Lazada / Amazon `oauth/*/callback`（落 token bundle）。

```text
view-only → PUT /shops/{viewOnly}/auth
  修复前：通过店铺闸门，仅被平台业务校验拦下（manual 平台 400）；非 manual 店可直接改写凭证
  修复后：403 {"code":40303}，shop_auth_tokens 零新增
view-only → POST /shops/{viewOnly}/oauth/douyin/revoke   修复前放行 → 修复后 403/40303
```

修复：新增 `findScopedShopForWrite` / `ensureShopOperable`（在可见性之上叠加 `EnsureStoreOperable`），授权写路径（`UpdateAuth`、douyin refresh/revoke/sync-shop-info、四平台 OAuth callback）全部切换；读路径（列表/详情/authorize-url）保持可见性口径。handler 侧新增 `failShopStoreScope`（并接入 `failDouyin`）统一 404 / 403+40303。

### 回归测试（先写失败测试，再修）

`backend/internal/securitytests/permmatrix/r165_store_write_scope_test.go`（5 个用例）——在修复前 5 个用例全部失败（实际观测到 200 + 落库/删库），修复后全绿：

- `TestR165ReviewDecisionStoreOperateScope`：view-only approve/reject → 403+40303 且 `review_status` 不变；无授权 operator 不能放行；授权 operator 仍能放行（不过度收紧）。
- `TestR165OrderExceptionMarkScope`：view-only 三条 mark 写 403+40303、跨租户 404、`order_exception_marks` 零新增；授权 operator 仍可标记。
- `TestR165ShopDeleteStoreOperateScope`：无授权 404 / view-only 403+40303 / 跨租户 404，且店铺行仍存活。
- `TestR165SyncStoreOperateScope`：sync-orders、sync-customer-messages、order-sync retry 对 view-only 403+40303，任务状态不被重置。
- `TestR165ShopAuthWriteStoreOperateScope`：`PUT /shops/:id/auth`、douyin refresh/revoke/sync-shop-info、TikTok callback 对 view-only 403+40303，`shop_auth_tokens` 零新增。
- `TestR165PublishTargetStoreOperateScope`：单品 publish、单品/批量 create-drafts 对 view-only 403+40303，`product_publish_batches` 零新增。

### 套件与工具

- `go test ./...`（backend 全量，含 `securitytests` permmatrix / idor / shopscope，`APP_ENV=test` + Docker Postgres 隔离库）：全绿。
- `gofmt` 无输出、`go build ./...` 通过。
- `govulncheck ./...`：0 可达漏洞。
- `pnpm audit --prod`：13 条，全为构建工具链，无增量。
- 证据（响应码/业务码/落库计数输出）外置 `/tmp/r165`，不入库。

## 【下一步】

1. 本 PR 与 R165 线1 PR 的关系：本线只提交上述 6 处店铺 scope 收口 + 回归测试 + 报告，**不触碰**客服会话族（#330 已修）、不重复线1 的实现面；若线1 也改到 `order-review` / `orderexception` / `shop.Delete` / sync / publish，请以本 PR 为基准合并，避免双改冲突。
2. 建议下轮把「写路由必须走 `EnsureStoreOperable`」做成静态检查或矩阵探针（本轮 6 处缺口全部源于写路径误用 `EnsureStoreVisible`、复用只读 scope 解析器或漏闸门），从抽样转为强制。
3. `pnpm audit --prod` 的 13 条构建链告警建议随 admin 工具链升级窗口一并处理（vite / esbuild 主版本升级）。

## 【需注意】

- P2 清单（本轮不改行为）：
  1. `POST /shops/:id/test-connection`、`POST /shops/:id/oauth/douyin/test` 对 view-only 放行（只读探测平台连通性、无落库；如按「触达平台即写」口径也应收 operate）。
  2. `publish-targets/check`、`batch-targets/check` 对 view-only 放行（纯计算不落库，与订单 `sku-candidates/batch` 现口径一致）。
  3. `PUT /settings` 数值型 `itemValue` 返回 400 `invalid body` 而非字段级中文提示（R164 观察项，UX 而非安全）。
  4. `pnpm audit --prod` 13 条构建链告警（与 R159 P2-3 同状态）。
  5. readonly / view-only 前端仍展示部分写入口（点击后被 403 拦截），R164 线2 已登记。
- 审计栈叠加了未合并的 #330（本 PR 直接以 #330 分支为基），因此 #330 的会话族结论以该分支为准；本 PR 与 #330 改动文件无交集，仅 `docs/PROGRESS.md`、`docs/permission-matrix.md` 可能需要合并期人工合并。
- P1-6 的 view-only 探针在 manual 平台店上只能证明「店铺闸门先于平台业务校验触发」；真实凭证改写面在非 manual 平台店，未在本地实测（无平台凭证），结论基于代码路径与闸门顺序。
- 本轮未做 UI 走查与 Docker 全栈 `seed:demo:full` 演练（属线1/其他线职责），店铺 scope 结论均来自生产路由的真实 HTTP 请求 + 直查数据库落库计数。
