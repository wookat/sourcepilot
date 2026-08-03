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

## round81 平台租户管理（R80 开租缺口收口）

- 新增 `GET/POST /api/v1/platform/tenants`（平台级租户列表 / 开租）。平台管理员采用**最保守判定**：当前登录账号 `tenant_id = 0` 且角色为 `admin`（现有语义中无独立"平台管理员"角色，tenant 0 为 legacy/平台桶）。
- 矩阵四角色（tenant A admin/operator/readonly + tenant B admin）均非 tenant 0，故两条路由全部登记为 `forbid`（统一 403，收紧优先）；POST 对 readonly 同时被 `ReadonlyWriteGuard` 兜底。
- 平台管理员正向路径与非平台角色 403、越权无副作用由 `platformtenant` 模块 `api_test.go` 覆盖（tenant0 admin 开租成功、初始管理员落新租户 admin、tenant1 admin / tenant0 operator / tenant0 readonly 403）。
- 开租写操作日志 `tenant.create`（不含密码）；`/auth/register` 行为不变（不提供自助开租）。

## round82 仪表盘/库存聚合租户收口（双租户实测 P1）

- 运营仪表盘（`/dashboard/product-operations|overview|todos|health`）聚合查询补租户过滤：`operationdashboard.Scope` 新增 `applyTenantColumn` / `applyTenantViaProduct` / `applyTenantViaShop` 助手，产品、采集、AI 任务/批次、图片任务、选品、货源、采购、订单、客服会话/建议、刊登任务/发布记录、库存同步、任务中心失败与告警等 Summary/Exceptions/Recent 查询全部按可信 `tenant_id` 限定（`TenantID` 为 nil 保持 legacy 内部调用行为）。无 `tenant_id` 列的 `ai_tasks` / `image_tasks` 经商品关联限定（口径同 taskcenter `applyTenantListFilterVia`）；`product_publications` 经店铺关联限定；任务中心 Summary 透传 `TenantID`（沿用其 tenant-0 legacy 桶口径）。
- 库存共享基础查询 `buildSKUAlertBaseTX` 支持可选 `TenantID`，`GET /inventory/alerts`、`POST /inventory/stock-settings/batch-preview|batch-update` handler 注入当前租户（此前三端点无租户过滤，batch-update 可跨租户改库存阈值）。
- 回归证据：`operationdashboard` `TestScopeApplyTenantColumn` / `TestScopeApplyTenantViaProductAndShop`、`inventory` `TestBuildSKUAlertBaseTXTenantScope`。路由级守卫矩阵不变（本轮未新增端点）。

## docs/api.md 口径差异说明

- docs/api.md 「只读账号写操作 403」：修复前部分写端点对 readonly 返回 400/404（bind/查找先于守卫）或直接放行（test-image/test-ocr）。现按文档口径 + 安全原则统一为路由级 403。
- 纯计算类 POST（`pricing/calculate`、各 `*/check`、`batch-preview`、`cost-estimates/batch`、`douyin_shop/validate`）按现网行为保留 readonly 可用，属「读语义但用 POST 传参」端点，已在允许清单集中登记。
- collector 登录探测 `POST */check-login` 会触发外部浏览器自动化，按安全原则划为写操作（readonly 403），与修复前行为不同。
