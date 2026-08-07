# Round 156 线2：经营大屏汇率折算与自定义指标

日期：2026-08-07。承接复评 v7（`docs/COMPETITIVE_BENCHMARK_R151.md` 建议③）认定的大屏与竞品剩余差距点：多币种折算口径可视化 + 大屏卡片可配置。基线：最新 main（R145 #296、R149 #303 today 口径、R93/R137 #285「未折算」口径均已合入）。分支 `feat/round156-dashboard-metrics` → main。

## 1. 大屏销售额/毛利多币种折算（沿既有租户汇率设置）

- 后端 `operationdashboard`：`/api/v1/dashboard/screen` 的今日 KPI 继续复用 `/reports/profit` SQL 下推口径（租户 `report_currency` 手工汇率 decimal 精确折算本位币；无汇率币种不计入 `salesBase`），新增显式口径字段：
  - `today.unconvertedRevenue:[{currency, amount}]`——无汇率币种的原币金额显式列出（此前仅有币种代码列表 `unconvertedCurrencies`，现两者都返回）；
  - `today.convertedCurrencies:[string]`——已折算入合计的非本位币列表。
- 前端大屏销售额/毛利卡：新增折算口径角标（InfoCircle tooltip：「按租户『报表币种设置』的手工汇率折算为本位币；未配置汇率的币种按原币单独展示，不计入合计」）；未折算币种从「未折算：EUR」升级为「未折算（不计入合计）：EUR 320.50」原币金额展示；全折算时显示「已折算：CNY」。
- 毛利口径不变：CNY→本位币汇率缺失时 `grossProfitBase/marginPercent` 缺省并提示「缺少汇率或成本，暂无法计算」，不伪造毛利。

## 2. 租户级自定义大屏指标卡片配置

- 卡片池（8 张，覆盖既有指标）：`kpi_orders/kpi_sales/kpi_profit/kpi_alerts/todos/funnel/trend/alerts`。
- 新端点：
  - `GET /api/v1/dashboard/screen/config`：所有角色可读（readonly 含）；未配置返回默认布局（全部启用、原顺序），保证默认现状不变。
  - `PUT /api/v1/dashboard/screen/config`：`settings.manage` 权限（`adminperm.RequireWrite`，readonly/operator 403）；body `{cards:[{key, enabled}]}` 按数组顺序生效；未知/重复 key 400，至少启用一张卡；写入租户 settings `dashboard_screen.cards`（tenant scope 隔离，复用既有 settings 表与事务）并记操作日志 `dashboard.screen_config.update`。
  - 旧版本存量配置向前兼容：缺失的新卡片按默认顺序补为启用。
- `/dashboard/screen` 响应新增 `cards`，禁用卡片跳过对应聚合（订单数/漏斗/趋势/待办/告警 各自仅在启用时查询；三张 KPI 全禁用时跳过利润报表计算），性能只减不增。
- 前端大屏改为按 `cards` 顺序分段渲染（相邻 KPI 合并为一行、漏斗/趋势合并为一行）；头部新增「自定义大屏指标」入口（仅 `settings.manage` 可见），弹窗内开关 + 上移/下移排序，保存后即时生效；readonly 无入口、无写请求。

## 3. Scope 与安全

- 未放宽任何 scope：`/dashboard/screen` 角色矩阵不变（四角色可读）；配置读取走 `adminperm.TenantIDFromGin`（租户上下文缺失 403 fail-closed）；写入走既有 `RequireWrite(settings.manage)`。
- 权限矩阵登记两条新端点（GET 四角色 allow；PUT operator/readonly forbid）。

## 4. demo seed 多币种大屏样本

- `fulldemo_round156.go`：新增今日多币种已付款订单 `DEMO-FX-USD-0001`（USD 199.99，经 seed 默认 USD=7.20 汇率折算入合计）与 `DEMO-FX-EUR-0001`（EUR 88.00，故意无汇率 → 大屏「未折算」显式展示），时间戳落在 seed 当天，开箱即可演示两种口径；订单号 DEMO- 前缀纳入既有 clean/verify 零残留链路。
- 回归测试 `fulldemo_round156_test.go`：断言样本存在、币种/金额/已付款/当日时间戳/订单行，且 Cleanup 后零残留。

## 5. 测试与门禁

- 后端单测：`screen_config_test.go`（默认布局、顺序与补卡、未知/重复 key 拒绝、JSON 解析回退、启用集合）；`go build/vet/fmt/test ./...` 全绿。
- 前端单测：`screenCards.test.ts`（默认分段、禁用隐藏、自定义顺序分段、上移/下移）；`pnpm test:frontend` 354 全绿。
- E2E：新增 `round156-dashboard-screen-config.spec.ts`（折算角标与未折算原币金额、禁用卡隐藏+顺序生效、配置弹窗保存 payload 断言（写请求拦截）、readonly 无入口无写请求、1920/1280/375 视口无溢出）；`round145-dashboard-screen.spec.ts` 同步新口径文案，17 用例全绿。
- 契约：`api-contracts.json/test` 登记两条新端点（116→118）；`pnpm test:contracts` 绿。
- `pnpm check:ui-copy --strict`、`pnpm build:admin` 绿；架构基线无新增违规（现存 2 条 MEDIUM 为 main 上既有，非本轮引入）。

## 6. 边界与遗留

- 汇率维护入口沿用「设置 → 报表币种设置」，本轮不新增汇率管理界面。
- 卡片配置为租户级（非用户级），readonly 可读配置但无编辑入口，符合任务口径。
- 接口延迟对比与 Docker 三角色三视口实测证据外置不入库（见 PR 评论）；Actions CI 不作为验收依据。
