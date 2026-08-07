# 权限矩阵契约测试（Permission Matrix）

系统化、可重复执行的「路由 × 角色 × 租户 × 店铺 scope」授权契约测试，把「新端点忘加守卫 / 新查询漏 scope」从人工审查变成自动拦截。

- 套件位置：`backend/internal/securitytests/permmatrix/`
- 矩阵数据：`backend/internal/securitytests/permmatrix/matrix.json`
- 路由级只读守卫（fail-closed 默认）：`backend/internal/pkg/adminperm/write_guard.go`

## 运行方式

需要安全测试库（与其他 PostgreSQL 集成测试一致，库名须含 `test`）：

```bash
cd backend
TEST_DATABASE_URL='postgres://trademind:trademind@127.0.0.1:5432/trademind_test?sslmode=disable' \
APP_ENV=test go test ./internal/securitytests/permmatrix/ -v
```

未设置 `TEST_DATABASE_URL` 时套件自动 skip（不产生误报）。

## 套件内容

| 测试 | 断言 |
| --- | --- |
| `TestRouteRegistryComplete` | 生产路由器（`api.Register`）挂载的**每一条路由**都必须在 `matrix.json` 登记，且 `matrix.json` 无陈旧条目。**新增端点未登记预期时该测试失败**。 |
| `TestAnonymousRequiresAuth` | 所有非 public 路由匿名访问必须 401。 |
| `TestPermissionMatrix` | 逐路由 × {admin, operator(有限店铺), readonly(无店铺), 跨租户 admin} 断言预期类别。 |
| `TestCrossTenantShopIsolation` | 租户 B admin 看不到、读不到租户 A 店铺（列表隔离 + 直读 404）。 |
| `TestOperatorStoreScope` | operator 仅见已授权店铺，未授权店铺读取被拒。 |
| `TestReadonlyStoreScope` | readonly（无店铺授权）列表为空，写既有资源 403。 |
| `TestReadonlyWriteGuardRegression` | round52 P0 修复回归（见下）。 |
| `TestOperationTaskStoreScope` | round61 P2 修复回归：运营任务按店铺 scope 隔离（见下）。 |

## matrix.json 条目格式

```json
{
  "method": "PUT",
  "path": "/api/v1/products/:id",
  "personas": {
    "admin": "allow",
    "operator": "allow",
    "crossTenantAdmin": "allow",
    "readonly": "forbid"
  },
  "note": "可选说明"
}
```

- `personas.<角色>`：
  - `allow`：请求必须**通过认证与授权**（响应非 401/403；400/404 等业务错误视为通过守卫）。
  - `forbid`：必须被守卫以 **403** 拒绝。
- `public: true`：无需登录即可访问（health、登录、公开回调、webhook 接收）。
- `probe: false`：登记但不做通用探测（目前仅 4 条自助 session 管理路由，探测会使 fixture token 失效；由 auth 模块测试覆盖），须在 `note` 写明原因。
- `crossTenantAdmin` 列断言的是「守卫通过性」；**数据级**跨租户隔离由 `TestCrossTenantShopIsolation` 等数据测试断言（列表为空/404，而非 403）。

## 新增端点流程

1. 新路由合入后本地跑套件，`TestRouteRegistryComplete` 会列出未登记路由。
2. 在 `matrix.json` 增加条目并**人工评审**四个角色的预期（默认写端点 readonly=forbid）。
3. 可用 `PERM_MATRIX_GENERATE=1` 运行该测试打印基于实际行为的草稿条目，但草稿必须安全评审后才能提交（观察到的 `allow` 可能正是缺守卫）。
4. 若新写端点确属 readonly 可用（纯计算/检查/预览），须同时加入
   `adminperm.ReadonlyWriteGuard` 的允许清单（`write_guard.go`）并在条目 `note` 说明理由。

## 路由级只读守卫（ReadonlyWriteGuard）

`/api/v1`（authed 组）与 `/api/collector` 上启用了组级中间件：对所有
POST/PUT/PATCH/DELETE 路由，readonly 账号一律 403，除非路径在显式允许清单中
（自助 session 管理 + 纯计算类 POST：calculate/check/preview/validate/estimate）。
新写端点**默认被守卫**（fail-closed），handler/service 级权限与 scope 检查照旧生效。

## round52 发现与修复（P0）

1. `POST /api/v1/settings/test-image`、`POST /api/v1/settings/test-ocr`
   缺 `settings.manage` 权限检查（同组其它 `settings/test-*` 均有）：readonly/operator
   可触发外部图片/OCR 连接测试。已在 handler 增加 `adminperm.RequireWrite(PermSettingsManage)`。
2. 大量写端点无路由级只读守卫，依赖 handler/service 深层检查，且对不存在资源返回
   400/404 而非 403，守卫存在性不可证明；部分路径（如 `PUT /api/v1/products/:id`）
   对持有店铺授权的 readonly 账号存在写入可能。已通过 `ReadonlyWriteGuard` 统一收口。

回归证据：`TestReadonlyWriteGuardRegression` + `TestPermissionMatrix`（readonly 列全部写端点 403）。

## round60 全量重跑核对（integration/round55-preview，收敛 #79–#117）

- 补登记 6 条漏登记路由（`TestRouteRegistryComplete` 报缺）：
  - `GET /api/v1/ops/backups/:id/download`（`backup.download` 仅 admin，#107）
  - `GET /api/v1/orders/stats/daily/export.csv`、`GET /api/v1/products/listing-list/export.csv`（读导出，四角色 allow，均带租户/店铺 scope）
  - `GET /api/v1/product-sources/orphans`（读，四角色 allow）
  - `DELETE /api/v1/product-sources/:id`、`POST /api/v1/procurement/orders/:id/void`（写，readonly forbid）
- 修正 7 条平台配置路由预期：`platform/settings/:platform`（GET/PUT/test-connection）、`platform/publish-settings/:platform`（GET/PUT）、抖音类目 sync × 2 现由 `settings.manage`（仅 admin）守卫（round59 设置中心收口口径），矩阵由 operator/readonly allow 收紧为 forbid。
- 未发现真实权限缺口（无「预期 403/404 实际 200」条目）；全部差异均为守卫比矩阵登记更严，属登记表滞后。

## round61 运营任务店铺 scope（P2）

- `operation_tasks` 表新增 `shop_id`（可空，`char(36)`，索引 `idx_operation_tasks_tenant_shop_updated`）；运营任务从红线外的租户级数据改为店铺维度业务数据，口径与订单/采购/异常一致：
  - admin（`AllowedStoreIDs()==nil`）不受限；
  - operator/readonly 仅见已授权店铺的任务，无授权店铺则列表为空；
  - `shop_id IS NULL`（租户级）任务仅 admin 可见（与订单列表 `ApplyStoreScope` 对未绑定行的处理一致）；
  - 越权/跨租户直读（含 drafts/approvals/attempts/events 等子资源与全部写路径）统一 404，不泄露存在性。
- 存量 backfill（`backfillTaskShopScope`，随迁移自动执行）：`source_reference` 命中同租户店铺 id → 归属该店铺；命中商品 id 且发布关联（publish config/publication）唯一指向一个店铺 → 归属该店铺；推导不出的保持租户级（admin-only，不放大可见性）。
- 创建口径：`POST /api/v1/operation-tasks` 新增可选 `shopId`；admin 可省略（租户级任务）；非 admin 必须绑定已授权店铺（缺失 400，未授权/跨租户店铺 404）。
- 回归证据：`TestOperationTaskStoreScope`（本套件）+ `operationtask` 模块 `api_scope_test.go`（admin/operator/readonly/跨租户四口径、全写路径、backfill）。

## round71 业务子资源 scope（R70 复扫清单收口）

- sourcing 货源/价格历史、imagetask 任务明细、aioperationbatch 批次子资源、productpublish 发布记录/SKU 绑定、ordersync `POST /shops/:id/sync-orders` 统一补父资源 tenant（+店铺）scope，越权/跨租户 404，不泄露存在性；见 docs/api.md「业务子资源 scope 口径（round71）」。
- 数据级回归证据：各模块 `subresource_scope_test.go`（三角色 + 跨租户 404 + 越权写无副作用）。路由级守卫矩阵不变（本轮未新增端点）。

## round72 AI 批次租户建模（R71 遗留收口）

- `ai_operation_batches` 新增 `tenant_id` 列（默认 0，索引 `idx_ai_op_batches_tenant_created`）；创建批次（`POST /ai/batches/product-text|product-images`）写入当前租户。
- `GET /ai/batches` 列表按 `ApplyTenantScope` 过滤（此前无租户过滤）；详情/子资源（`/:id`、`/:id/tasks`、`retry-failed`、`apply-results`）改按 `tenant_id` 列校验，跨租户 404 口径与 round71 一致。批次无店铺维度，admin/operator/readonly 同租户口径一致。
- 存量 backfill（`migrateRound72AIBatchTenant`，随迁移自动执行）：按 `created_by` → `admin_users.tenant_id` 推导；推导不出（无创建人/创建人已删）保持租户 0（legacy 单租户桶，不放大可见性）。未 backfill 的 tenant-0 且有创建人的行，`ensureBatchVisible` 回退按创建人租户校验（与 round71 口径一致）。
- 回归证据：`aioperationbatch` 模块 `TestAIOperationBatchTenantColumnScope`（三角色列表过滤 + 跨租户 404 + backfill/回退口径）与既有 `TestAIOperationBatchScopedByTenant`。路由级守卫矩阵不变（本轮未新增端点）。

## round81 刊登批次租户建模 + 发布任务越权 404 统一（R80 遗留 P2）

- `product_publish_batches` 新增 `tenant_id` 列（默认 0，索引 `idx_publish_batches_tenant_created`）；创建批次（单商品 `publish-targets/create-drafts` 与多商品 `batch-targets/create-drafts`）写入当前租户。存量按 `created_by` → `admin_users.tenant_id` backfill（`migrateRound81PublishBatchTenant`，随迁移自动执行；推导不出保持租户 0，不放大可见性），口径与 round72 `ai_operation_batches` 一致。
- `GET /product-publish/batches` 列表按 `ApplyTenantScope` 过滤（此前无租户过滤）；`GET /batches/:id`、`POST /batches/:id/retry-failed`、`POST /batches/:id/cancel-pending` 及 `retryFailedOnly` 重试回放按 `tenant_id` 列校验（未 backfill 的 tenant-0 行回退按创建人租户），跨租户统一 **404**。DTO 不变（`tenant_id` 不出现在响应中）。同租户创建者隔离（403）口径不变。
- 发布任务越权口径统一：`POST /product-publish/tasks/:id/retry`、`/cancel`、`/recover` 此前对跨租户/不存在对象返回 400（带错误文案），与 `GET /tasks/:id` 的 404 混用；现统一 **404**（不泄露存在性）。`recover` 增加租户归属前置校验（此前内部按裸 id 加载任务）。同租户业务校验错误（如状态不允许重试）仍为 400。
- 回归证据：`productpublish` 模块 `TestPublishBatchScopedByTenant`（批次创建落租户 + 详情/重试/取消跨租户 404 + 列表过滤）、`TestFailedTargetsFromBatchScopedByTenant`（重试回放跨租户 404）、`TestPublishMutationEndpointsCrossTenant404`（handler 层 6 端点跨租户 404 状态码）。路由级守卫矩阵不变（本轮未新增端点）。

## round81 平台租户管理（R80 开租缺口收口）

- 新增 `GET/POST /api/v1/platform/tenants`（平台级租户列表 / 开租）。平台管理员采用**最保守判定**：当前登录账号 `tenant_id = 0` 且角色为 `admin`（现有语义中无独立"平台管理员"角色，tenant 0 为 legacy/平台桶）。
- 矩阵四角色（tenant A admin/operator/readonly + tenant B admin）均非 tenant 0，故两条路由全部登记为 `forbid`（统一 403，收紧优先）；POST 对 readonly 同时被 `ReadonlyWriteGuard` 兜底。
- 平台管理员正向路径与非平台角色 403、越权无副作用由 `platformtenant` 模块 `api_test.go` 覆盖（tenant0 admin 开租成功、初始管理员落新租户 admin、tenant1 admin / tenant0 operator / tenant0 readonly 403）。
- 开租写操作日志 `tenant.create`（不含密码）；`/auth/register` 行为不变（不提供自助开租）。

## round82 租户治理与平台管理员 persona

- harness 新增第五个 persona `platformAdmin`（tenant 0 + 角色 admin，即平台管理员）。它是**可选 persona**：仅在 matrix.json 条目显式声明时参与探测；未声明的路由不强制登记（避免为全部存量路由补平台管理员口径的大改动）。
- 平台租户管理全部 5 条路由（`GET/POST /platform/tenants`、`PUT /platform/tenants/:id`、`POST /platform/tenants/:id/disable|enable`）均登记 `platformAdmin: allow`，矩阵四常规角色一律 `forbid`（统一 403，收紧优先）。
- 治理语义：租户可停用/启用/改名（不提供删除）；tenant 0 不可停用/改名（handler 400）；不存在的租户 404。停用后该租户所有账号登录拒绝、已有会话下次请求失效（错误码 `AUTH_TENANT_DISABLED`），由登录 / refresh / `ValidateSessionAccess` 三处统一强制，模块级证据：`auth` 包 `tenant_state_test.go`、`platformtenant` 包 `api_test.go`。
- 全部治理操作写操作日志（`tenant.rename` / `tenant.disable` / `tenant.enable`）。

## round83 仪表盘/库存聚合租户收口（双租户实测 P1）

- 运营仪表盘（`/dashboard/product-operations|overview|todos|health`）聚合查询补租户过滤：`operationdashboard.Scope` 新增 `applyTenantColumn` / `applyTenantViaProduct` / `applyTenantViaShop` 助手，产品、采集、AI 任务/批次、图片任务、选品、货源、采购、订单、客服会话/建议、刊登任务/发布记录、库存同步、任务中心失败与告警等 Summary/Exceptions/Recent 查询全部按可信 `tenant_id` 限定（`TenantID` 为 nil 保持 legacy 内部调用行为）。无 `tenant_id` 列的 `ai_tasks` / `image_tasks` 经商品关联限定（口径同 taskcenter `applyTenantListFilterVia`）；`product_publications` 经店铺关联限定；任务中心 Summary 透传 `TenantID`（沿用其 tenant-0 legacy 桶口径）。
- 库存共享基础查询 `buildSKUAlertBaseTX` 支持可选 `TenantID`，`GET /inventory/alerts`、`POST /inventory/stock-settings/batch-preview|batch-update` handler 注入当前租户（此前三端点无租户过滤，batch-update 可跨租户改库存阈值）。
- 回归证据：`operationdashboard` `TestScopeApplyTenantColumn` / `TestScopeApplyTenantViaProductAndShop`、`inventory` `TestBuildSKUAlertBaseTXTenantScope`。路由级守卫矩阵不变（本轮未新增端点）。

## round83 手工建草稿限管理员（R82 双租户回归 P2 收口）

- `POST /api/v1/products`（手工新建草稿）operator 由 `allow` 改为 `forbid`：新建草稿无店铺关联，按商品可见性口径（未关联店铺的草稿仅 admin 可见）operator 创建后自己永远不可见；修复前校验发生在写入后，返回 400 `record not found` 且残留孤儿行。
- 现口径：`product.Service.Create` 在写入前做 principal 校验，非 admin 统一 403（明确文案），不落库；readonly 仍由 `ReadonlyWriteGuard` 兜底 403。
- 回归证据：`product` 模块 `create_scope_test.go`（operator/readonly 403 且 0 残留行、admin 正常创建）。
- round88 已恢复 operator 手工建草稿（带店铺归属），本节口径被下文「round88」替代。

## round88 恢复 operator 手工建草稿（带店铺归属）

- `POST /api/v1/products` operator 由 `forbid` 改回 `allow`：operator 建草稿必须选择归属店铺（`shopId`），后端在写入前校验并在同一事务内写入 `product_platform_publish_configs` 关联，草稿按既有商品可见性口径对创建者可见，杜绝 round83 前的"创建后自己不可见"脏数据。
- 校验口径（全部发生在写入前，被拒绝零落库）：operator 不传 `shopId` → 400（中文引导文案）；`shopId` 不在授权范围 → 404「资源不存在」（不泄露存在性，与店铺资源现口径一致）；仅 `view` 授权 → 403（店铺数据无权访问口径 40303）；店铺不存在/跨租户 → 404。admin 保持现口径：`shopId` 可选。readonly 仍由路由级守卫 403。
- 前端：新建草稿弹窗增加「归属店铺」选择（operator 必填、下拉只列其授权店铺——由后端 `GET /api/v1/shops` 店铺 scope 保证；admin 可选、全量可选）。
- 回归证据：`product` 模块 `create_scope_test.go`（operator 无店铺 400、越权店铺 404、view-only 403、授权店铺 200 且列表可见、readonly 403、admin 有/无店铺、未知店铺 404，均含零脏数据断言）；permmatrix 契约测试 operator=allow、readonly=forbid。

## round89 平台租户清退删除（测试租户留存收口）

- 新增 `POST /api/v1/platform/tenants/:id/purge`（提交清退后台任务）与 `GET /api/v1/platform/tenants/:id/purge`（任务状态 / 逐表零残留报告），两条路由登记 `platformAdmin: allow`，矩阵四常规角色（tenant A admin/operator/readonly + tenant B admin）一律 `forbid`（统一 403，收紧优先）；POST 对 readonly 同时被 `ReadonlyWriteGuard` 兜底。
- 安全门：仅 tenant 0 admin 可用；前置条件租户已停用（未停用 400）；请求体 `confirmName` 必须与租户名完全一致（不一致 400）；tenant 0 永不可清退（400）；前端另有二次确认弹窗。production 模式同样可用，安全门不因环境放宽。
- 审计保留策略：清退目标租户的业务操作日志随租户一并删除；平台侧开租/清退审计（`tenant.create` / `tenant.purge.start` / `tenant.purge.done|failed`）与 `tenant_purge_tasks` 任务记录保留在 tenant 0。
- 回归证据：`platformtenant` 模块 `purge_api_test.go`（停用前 400、confirmName 不一致 400、tenant 0 400、不存在 404、三类非平台 persona 403 且无副作用、成功清退后逐表零残留 + tenant 0 审计保留）。前端入口仅平台管理员可见（页面级 `isPlatformAdmin` 403 兜底），且仅对已停用租户展示「清退删除」。

## round91 物流商与打单发货

- 新增 6 条路由登记：`GET /api/v1/carriers`（四角色 allow，租户隔离）；`POST /api/v1/carriers`、`PUT /api/v1/carriers/:id`、`DELETE /api/v1/carriers/:id`（写，readonly forbid）；`GET /api/v1/orders/print/sheets`（读，四角色 allow，订单店铺 scope 同订单详情：operator 仅授权店铺订单，越权 404）；`POST /api/v1/orders/:id/shipments/:shipmentId/refresh-tracking`（写，readonly forbid，订单店铺 scope）。
- carriers 数据租户隔离（`tenant_id` + `ApplyTenantScope`），预置物流商按租户幂等 seed，不跨租户共享启停状态；预置不可删除只可停用。
- 批量发货 `POST /orders/shipments/batch` 契约扩展（新增可选 `carrier`/`carrierCode` 列与 `defaultCarrierCode`），路由与角色登记不变；旧两列格式兼容。
- 回归证据：`carrier` 模块 `service_test.go`（租户隔离、预置幂等、启停/删除口径）、`order` 模块 `carrier_shipment_test.go`（carrier 关联、运单号校验、批量新旧格式、manual 轨迹推动订单流转）；permmatrix 契约测试覆盖上述新端点。

## docs/api.md 口径差异说明

- docs/api.md 「只读账号写操作 403」：修复前部分写端点对 readonly 返回 400/404（bind/查找先于守卫）或直接放行（test-image/test-ocr）。现按文档口径 + 安全原则统一为路由级 403。
- 纯计算类 POST（`pricing/calculate`、各 `*/check`、`batch-preview`、`cost-estimates/batch`、`douyin_shop/validate`）按现网行为保留 readonly 可用，属「读语义但用 POST 传参」端点，已在允许清单集中登记。
- collector 登录探测 `POST */check-login` 会触发外部浏览器自动化，按安全原则划为写操作（readonly 403），与修复前行为不同。

## round102 注册/租户生命周期安全复验（tenant 0 平台数据隔离收口）

- 自助注册（`POST /api/v1/auth/register`）为每个注册账号新建独立租户（#214）已复验：Docker 双租户实测注册落 `tenant_id = 5`（非 0），账号与租户 1:1，无 tenant 0 归属。
- 平台级运维路由 `/api/v1/ops/backups|restores|releases|dr/*`（共 18 条）由「租户角色 + 运维权限位」改为**平台租户专属**：这些操作作用于整个部署（全库备份可导出所有租户数据、恢复/发布可改写全库），任何业务租户 admin 一律 403。矩阵四常规角色全部改为 `forbid`，`platformAdmin: allow`。
- 共享提示词目录 `ai_prompts` 无 `tenant_id`（按 `code` 全局唯一），写操作（`POST /ai/prompts`、`PUT/DELETE /ai/prompts/:id`、`enable`/`disable`，共 5 条）收紧为平台租户专属；round103 起读（`GET /ai/prompts`、`GET /ai/prompts/:id`）同样收紧为平台租户专属（目录可能含平台自定义提示词内容，业务租户 AI 能力经服务端 `GetEnabledByCode` 消费、不受影响）。矩阵登记：7 条路由四常规角色 `forbid`，`platformAdmin: allow`；前端 `/ai/prompts` 登记入 `PLATFORM_ADMIN_ROUTES`。
- `GET /api/v1/settings` 由「本租户 + tenant 0 平台默认值」改为**仅返回本租户行**：tenant 0 存放平台凭据（平台应用 key、存储、邮件），业务租户不得读取；平台默认值继续由服务端 `PlainByGroup(ctx, 0, ...)` 内部消费。`PUT /api/v1/settings` 忽略请求体 `tenantId`，一律写当前租户；显式传其他租户（含 0）返回 403。
- `collect_rules` / `collect_browser_profiles` 新增 `tenant_id`（默认 0，索引），列表、详情、创建、更新、删除、启停、规则测试、自定义采集规则解析（`ResolveEnabledRuleForCustom`）与浏览器 profile 注入全部按可信租户限定，跨租户统一 404（不泄露存在性）。
- 商品子资源统一租户守卫：`adminperm.ProductRouteTenantGuard` 在 authed 组对所有 `/products/:id` 前缀路由校验商品租户归属，跨租户 404；`product-skus/search` 联表补 `products.tenant_id` 限定；AI 运营工作台与客服仪表盘聚合改为按可信租户（无 `tenant_id` 的子表经商品/会话关联限定）。
- legacy JWT 请求期账号校验：`auth.EnsureAccountActive` 在每次请求校验账号存在、未软删、状态 active 且 `token_version` 未被提升，使改密码/停用/删除账号对旧 access token 立即生效（此前仅 secure_session 模式生效）。
- 契约与回归证据：`permmatrix` 新增 `tenant_zero_test.go`（平台专属运维路由、提示词写、settings tenant 0 读写、采集规则/profile 跨租户、商品子资源守卫）；`middleware` 新增 `jwt_account_state_test.go`（停用 / 缺失 / `token_version` 提升三类旧 token 401）。

## round105 settings 租户化第二批 + 告警租户来源闭环

- `GET /task-center/alert-notifications` 补租户过滤：通知审计行经 `alert_id IN (SELECT id FROM task_alerts WHERE tenant_id = 调用方租户)` 限定，业务租户不再能读取其他租户/平台的通知目标（邮箱、webhook）与错误详情。路由与角色登记不变。
- `POST /task-center/failures/:taskType/:id/generate-alert` 补来源租户校验：来源任务行归属租户与调用方租户不一致时统一 404（不泄露存在性），与 round104 告警单条操作口径一致。
- 告警行 `tenant_id` 自 round105 起由来源任务行归属租户写入（`resolveSourceTenant`），历史 tenant-0 行由 `migrateRound105AlertTenant` 回填；round104 的 `GET /task-center/alerts` 租户过滤由此对业务租户真正生效。
- settings 写侧（`PUT /api/v1/settings`）口径不变（一律写调用方租户、显式传别租户 403）；前端 Inventory / AlertNotify / Pricing / AI 设置页移除硬编码 `tenantId: 0`，业务租户 admin 保存自己的租户配置不再 403。
- 回归证据：`taskcenter` `alert_tenant_source_test.go`（Upsert 落租户 + 通知审计租户隔离）、`tenantsettings` 单测（inventory/pricing/sourcing 逐 key 合并、alert_notify 整组回退）、集成 `alert_tenant_backfill_test.go`。

## round106 settings 租户化第三批收尾（image + 告警扫描/通知触发策略）

- `image` 组整组回退租户化（口径同 ai）：租户配置任一自有凭据（任一 `*_api_key` 或 ComfyUI 的 `comfyui_base_url`）则整组以租户配置为准，否则整组回退平台默认；平台凭据不与租户参数混流。读写路由与角色登记不变（`GET/PUT /api/v1/settings` 沿用 #216 口径：写一律落调用方租户、显式传别租户 403；读仅返回本租户行）。
- `taskcenter` 告警策略组逐 key 合并租户化：告警生成与外发通知触发策略按告警来源/归属租户解析，租户在 `GET /task-center/alerts` 看到自身聚合告警（round104/105 已按 `tenant_id` 过滤），tenant 0 为平台桶。扫描 worker 运行键（`enable_alert_scan_worker` / `alert_scan_interval_seconds`）保留平台级，系统设置页仅平台管理员可见可存。
- 残留平台级 `PlainByGroup(ctx, 0, ...)` 保留项：storage、mail/email、system、platform_*（平台应用凭据/发布配置 schema）、inventory 平台限流键、taskcenter 扫描 worker 运行键。

## round116 R109–R115 新端点登记（安全审计复跑）

- 补登记 40 条 `TestRouteRegistryComplete` 报缺的路由，覆盖 R109–R115 新增面：客服话术模板（`/customer-service/reply-templates`、`/customer/reply-templates` 各 5 条）、深度报表（`/reports/profit|profit/export.csv|procurement|inventory`）、面单模板与发货规则（`/waybill-templates` 4 条、`/shipping-rules` 5 条）、多仓与调拨（`/inventory/warehouses*` 7 条、`/inventory/transfers`、`/inventory/sku-warehouse-stocks`）、打单与发货推荐（`/orders/print/mark`、`/orders/shipping-recommendations`）、数据搬家映射方案与模板/导出（`/imports/mappings*`、`/imports/templates/:kind`、`/imports/export/:kind`），以及公开健康检查 `GET /healthz`（`public: true`）。
- 预期口径按人工评审写入，未采用生成器的 observed allow：读路由四角色 `allow`（租户/店铺 scope 在查询内应用），写路由 `readonly: forbid`；`shipping-rules/recommend`、`orders/shipping-recommendations` 虽为只读计算，但未列入只读白名单，契约按现网守卫保持 `forbid`。
- `POST /api/v1/products/banned-words/check-batch` 此前矩阵登记 `readonly: allow` 而守卫 403（登记与实现不一致）。该端点是纯扫描计算（不落业务数据，仅写操作日志），语义与已允许的 `GET /products/:id/banned-words/check` 及 `products/ai-text/batches/check` 一致，故加入 `write_guard.go` 只读白名单，契约保持 `allow`。
- 本轮矩阵复跑未发现「预期 403/404 实际 200」的路由级缺口；数据级缺口（审单决定与发货推荐的店铺 scope）见 `order` 模块 `review_store_scope_test.go`。

## round117 R116 审计 P2 收口

- **只读试算口径**：`POST /shipping-rules/recommend`、`POST /orders/shipping-recommendations` 均为纯计算（仅承载参数、不落业务数据），按「纯计算例外」加入 `write_guard.go` 只读白名单，矩阵两条登记改为 `readonly: allow`。发货推荐的订单解析仍走 `EnsureStoreVisible` / `ApplyStoreScope`，readonly 无店铺授权时解析不到任何订单。
- **订单按 id 写路径通用契约测试**（审计观察项 8）：新增 `order_write_scope_test.go` `TestOrderByIDWriteStoreScope`，对全部 `/orders/:id*` 写路由与 `order-items/:itemId` 路由做「未授权店铺订单 / 跨租户订单 → 404 且无副作用」断言，并带路由完整性检查（新增此类路由未登记探针即失败）。该测试暴露并修复了四处数据级缺口：
  - `DELETE /orders/:id`：service `Delete` 原查单无 tenant / 店铺条件，跨租户可删单；改走 `findOrderBare`。
  - `POST /orders/:id/match-skus`：直接进撮合引擎（引擎按裸 id 查单），未授权/跨租户订单可触发重建；handler 先 `findOrderBare`。
  - `POST /orders/:id/deduct-inventory`、`restore-inventory`：`Inv.*ForOrder` 按裸 id 执行，未授权订单可扣/回补库存；handler 先 `findOrderBare`。
  - `POST /order-items/:itemId/bind-sku` 与 sku-candidates 两条读路由（`GET /order-items/:itemId/sku-candidates`、`POST /orders/:id/sku-candidates/batch`）：订单行/候选查询无租户与店铺条件；`GetOrderItemByID` 补父订单 `findOrderBare`，skucandidate 模块新增 `EnsureOrderVisible`（tenant + `EnsureStoreVisible`）。

## round125 安全审计季度复跑（R117 后新增面）

- **矩阵补登记**：`TestRouteRegistryComplete` 报缺 4 条选品数据面读路由（`GET /selection/compare`、`/selection/market-sources`、`/selection/candidates/:id/insights|price-trend`），按读路由口径四角色 `allow` 登记（租户 scope 在查询内应用）。
- **规则 shopIds 越权（P1，已修复）**：自动化规则、审单规则、买家消息规则的 create/update 原样存储 `shopIds`，可写入跨租户/未授权店铺 id 使规则作用于越权店铺。新增 `adminperm.EnsureStoresOperable`（跨租户/不存在/不可见 → 404 不泄露存在性；仅可见不可操作 → 403），三处 create/update 统一接入。
- **dry-run 数据泄露（P1，已修复）**：自动化/审单规则 dry-run 原扫描全租户订单，operator 可预览未授权店铺订单号与金额；两处补 `ApplyStoreScope`。
- **执行日志越权（P1，已修复）**：`GET /order-automation-logs` 原仅按租户过滤，operator 可见未授权店铺订单的日志；补按订单店铺 scope 子查询过滤。`POST /order-automation-logs/:id/retry` 原不校验订单店铺授权即可重放动作；补 `findOrderBare`（404 口径）。`GET /orders/:id/automation-logs` 越权时误返 500，改 404。
- **买家消息草稿越权（P1，已修复）**：草稿 list 原仅按租户过滤，泄露未授权店铺草稿内容；详情写路径（update/mark-sent/ignore/batch-mark-sent）原仅校验租户。list/batch 补 `ApplyStoreScope`，详情查找补 `EnsureStoreVisible`（404 口径）。
- **新增契约测试** `automation_message_scope_test.go`：`TestAutomationRuleShopIDsScope`、`TestAutomationDryRunStoreScope`、`TestAutomationLogStoreScope`（含 404 后无副作用断言）、`TestBuyerMsgScope`（含草稿内容不泄露断言）、`TestSelectionCandidateTenantScope`。
- **加固**：`config.NormalizeEnv` 将 `prod` 归一为 `production`，demoseed/perfseed 生产拒绝判定统一走 `config.IsProduction`；浏览器端 CSV 导出（`admin/src/utils/csv.ts`）补公式注入前缀中和（与后端 `csvsafe` 同口径）。

## round144 MCP 只读入口

- **矩阵登记**：`GET /api/v1/mcp/tokens` 四角色 `allow`（租户 scope 在查询内应用）；`POST /api/v1/mcp/tokens`、`POST /api/v1/mcp/tokens/:id/revoke` 为写路由，`readonly: forbid`。
- **`POST /api/mcp`** 登记为 `probe: false`：该入口不走后台 JWT persona，鉴权为租户级只读 API token（`sp_mcp_ro_*`，SHA-256 哈希存储、可吊销、每 token 限流）。租户隔离、吊销失效、只读工具面与 401/429 行为由 `mcpserver` 模块测试（`server_test.go`、`hardening_test.go`）与 `mcptoken` 服务测试（`service_test.go`、`hardening_test.go`）覆盖。
- **R145 安全交叉审查补强**：鉴权强制 `scope=readonly`；限流除每 token 外增加每租户聚合桶与每 IP 鉴权失败预算；每租户活跃 token 上限 20；429 envelope 用 `code=42901`。
- MCP 工具全部只读（`orders_query` / `inventory_query` / `report_summary` / `exceptions_pending`），无任何写操作暴露；使用说明见 `docs/mcp.md`。
- **R146 加固**：token 可选过期（到期鉴权拒绝）；新增 `GET /api/v1/mcp/audit-logs`（工具调用逐次审计，只读路由，四角色 `allow`，租户 scope 在查询内应用）；限流状态 Redis 可用时走 Redis，不可用降级进程内。

## round152 开放 API 只读入口

- **`GET /api/open/v1/orders` / `GET /api/open/v1/orders/:orderNo` / `GET /api/open/v1/inventory` / `GET /api/open/v1/reports/summary` / `GET /api/open/v1/exceptions`** 均登记为 `probe: false`：入口不走后台 JWT persona，鉴权为同一租户级只读 API token 体系（token 用途须为 `openapi`/`both`，存量/默认 `mcp` 用途 token 不能访问）。租户隔离、跨租户 404、用途限制、吊销失效、只读方法面（仅 GET）与 401/429 行为由 `openapi` 模块测试（`server_test.go`、`spec_test.go`）覆盖。
- token 管理路由不变（`/api/v1/mcp/tokens*`），创建 body 新增可选 `purpose`，权限预期不变。使用说明见 `docs/open-api.md`。

## round160 R159 审计 P2 收口

- **view-only persona（防 #322 同类漂移）**：harness 新增第六个 persona `viewOnlyOperator`（tenant A、角色 operator，唯一店铺授权为 `ShopViewOnly` 的 `view` scope）。它是**可选 persona**（同 `platformAdmin`）：matrix.json 条目显式声明时才参与路由级探测；数据级覆盖由专用契约测试承担。
- **新增契约测试** `view_only_persona_test.go` `TestViewOnlyPersonaStoreWriteScope`：对全部 `/orders/:id*` 与 `/order-items/:itemId*` 写路由（带路由完整性检查，新增此类路由未登记探针即失败）及买家消息草稿写路径（update/regenerate/mark-sent/ignore）断言 view-only → 403 且业务码 **40303**、零落库；纯计算 POST（`sku-candidates/batch`）与读路由保持可用（view 授权可见）。
- **业务码统一（40301 → 40303）**：店铺级「可见但仅 view 授权」的 403 统一为 `40303 CodeStorePermissionDenied`（改动面：order 各写 handler、customerchat 草稿、finance 对账/费用）。取舍：`40303` 已是 R125 商品创建线与 `adminperm.DenyStorePermission` 的既有口径，且语义最精确（店铺数据权限）；`40301 CodeForbidden` 保留给全局/租户级 forbidden（readonly 账号写操作、跨租户 settings、生产闸门等），`40302` 保留给权限位拒绝。前端无按 40301/40303 分支的逻辑（`httpErrorCopy` 按 HTTP 403 文案），契约 fixtures 无引用，无前端改动。

## round164 客服会话 view-only 写收口

- **新增契约测试** `view_only_conversation_test.go` `TestViewOnlyPersonaConversationWriteScope`：客服会话族写路径（编辑/删除会话、创建绑定店铺会话、添加消息、`mark-replied`、`ai/generate-reply`、建议编辑/采纳/丢弃/apply/reject、`send-platform-message`）对 view-only 授权断言 403 + 40303、零落库；会话/消息读路径保持可用，会话详情 `canWrite=false`。
- 实现口径：`customerchat` 写路径经 `findScopedConversationForWrite`/`findScopedSuggestionForWrite`（`adminperm.EnsureStoreOperable`），不可见店铺仍 404 不泄露存在性。

## round165 线2 店铺写路由 scope 收口（安全审计）

- **新增契约测试** `r165_store_write_scope_test.go`（6 用例）：订单审单决定（approve/reject）、异常工作台标记族（handle/ignore/mark）、店铺删除、店铺授权写（`PUT /shops/:id/auth`、抖店 refresh/revoke/sync-shop-info、TikTok callback）、同步创建与重试（`sync-orders`、`sync-customer-messages`、`order-sync/tasks/:id/retry`）、商品刊登目标店（publish、单品/批量 create-drafts）对 view-only 授权断言 403 + 40303 且零落库；跨租户断言 404；授权 operator/admin 正例保持放行。
- **实现口径**：写路径统一 `adminperm.EnsureStoreOperable`（不可见/跨租户仍 404 不泄露存在性）——`order` 审单批量前置 `ensureReviewBatchOperable`（整批拒绝，避免部分生效）、`orderexception` 新增 `source_scope.go` 按 source 类型解析真实租户/店铺（未知 source fail-closed 404）、`shop` 新增 `findScopedShopForWrite`/`ensureShopOperable`（授权写与 OAuth 写路径）、`ordersync`/`customersync` 的 `CreateShopSync` 与 `RetryFailed`、`productpublish` 的 `ensureTargetsOperable` 与 `loadDouyinPublicationForWrite`。
- **口径定调**：店铺同步（订单/客服消息）视为**店铺业务写**，需 operate 授权（闭合 R164 线2 P2 第 4 项）；`*/check`、`test-connection` 等纯探测/纯计算路径保持 view 可用。

## round165 view-only 店铺写入口全站扫尾

- **统一口径**：带 shop_id 维度（含经订单/商品/采购单/发布记录/任务行间接关联）的写路径一律走可操作性 scope（`adminperm.EnsureStoreOperable` / `EnsureStoresOperable` / `ApplyStoreOperateScope`）：可见但仅 `view` 授权 → **403 业务码 40303**；不可见/跨租户 → **404** 不泄露存在性。可见性 scope（`AllowedStoreIDs`/`ApplyStoreScope`/`EnsureStoreVisible`）仅用于读。错误映射统一走 `adminperm.FailStoreWriteScope`。
- **本轮收口面**：手动订单/客服消息同步及各类同步任务 retry（含任务中心委托 retry）、库存同步任务（创建/retry/批量）、inventory-sync P9 run、productpublish 全写族（任务/目标草稿/抖音建品/SKU 绑定/卡死恢复）、运营任务写族、订单异常 handle/ignore、采购单写族（经关联销售订单店铺）、审单 approve/reject 行级校验、店铺记录 Update/Delete、店铺凭证 `PUT /shops/:id/auth` 与 OAuth refresh/revoke/callback/sync-shop-info。
- **同步定性**：手动 sync-orders / sync-customer-messages 及 retry 定性为写操作（创建任务并 upsert 业务行），view-only 不再放行（R164 P2-4 定案）。
- **批量 API 取舍**：`/order-review/approve|reject` 保持 200 envelope + 行级失败（view-only 行报「店铺无操作权限」且状态不翻转），不改批量语义。OAuth authorize-url / 连接测试类只读路径保持可见性口径。
- **新增契约测试** `view_only_sweep_test.go` `TestViewOnlyPersonaShopWriteSweep`：30 条写探针 403+40303、审单行级失败、不可见店铺 404、零落库（任务状态不变、无新建任务/运营任务/P9 run）、读路径保持可用。
- **matrix.json 补全**：为 shops / order-sync / message-sync / inventory-sync / product-publish / task-center / operation-tasks / orders / order-items / order-review / procurement / customer 系非 GET 探测路由补 `viewOnlyOperator` 期望 113 条（路由级防漂移；数据级由专项契约测试承担）。
- **前端一致性**：`operableStoreIds`/`canOperateAnyStore`/`canOperateStore`（与后端 `Principal.OperableStoreIDs` 同口径）；会话列表 readonly/view-only 隐藏「新建会话/拉取平台消息」，写表单店铺选择器仅列可操作店铺，详情「废弃建议」readOnly 禁用。后端 403 仍为最终边界。
