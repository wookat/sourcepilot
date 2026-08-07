# R165 线2：安全审计季度复跑（security-auditor）

日期：2026-08-07。距 R159 安全审计季度复跑 6 轮，期间合入 #318/#320/#321/#322/#323/#325，R164 线2 又发现客服会话 view-only 越权（#330，审计时未合并，本轮以其分支为基叠加实测）。与 R165 线1 并行，本线从攻击者视角独立枚举写路由，不复用线1 实现。证据外置 `/tmp/r165` 不入库；Actions CI 不作依据（结论基于本地 Docker 双租户实测 + `go test`）。

## 1. 环境与方法

- Docker `trademind-postgres` / `trademind-redis` + 隔离库 `trademind_test`；跑生产路由注册（`api.Register`）的 permmatrix harness。
- 双租户：tenant A `910052` / tenant B `910053`；六 persona：admin、operator（operate 授权店）、readonly、view-only（仅 `view` 授权店）、跨租户 admin、平台 admin。
- 方法：先按「写路径是否只做 `EnsureStoreVisible` / 是否漏店铺闸门」审读全部店铺相关写路由，再对可疑点发真实 HTTP 请求并直查数据库落库计数；确认后先补失败测试再修。

## 2. 结论概要

- R159 修复项零回退（token purpose 隔离、跨租户、限流/XFF、审计 fail-closed、脱敏、生产闸门、大屏 scope）。
- #322 订单写面、#330 客服会话写面复验成立。
- **发现并修复 6 处 P1 view-only/跨租户越权**（详见 `docs/SECURITY_AUDIT_R165.md`）：
  1. 订单审单决定（approve/reject）只校验可见性 → view-only 可放行/拒单并真实改 `review_status`；
  2. 异常工作台标记族（handle/ignore/mark/bind-sku/retry-deduct/retry-inventory-sync）无租户与店铺 scope → 跨租户可写、view-only 可写；
  3. 店铺删除只按租户过滤 → 同租户无授权/仅 view 的 operator 可删店；
  4. 店铺授权凭证写入（`PUT /shops/:id/auth`）与抖店 refresh/revoke/sync-shop-info、四平台 OAuth callback 只校验可见性 → view-only 可改写/撤销平台凭证；
  5. `sync-orders` / `sync-customer-messages` 只校验可见性、`order-sync/tasks/:id/retry` 无店铺闸门 → view-only 可触发平台同步（同时闭合 R164 线2 P2 第 4 项，定口径为「同步属店铺业务写，须 operate」）；
  6. 商品刊登目标店（publish / create-drafts / 批量 create-drafts）与抖店 SKU 绑定写路径无操作权限校验。
- 40303 / 40301 / 400 口径修复后一致；大屏折算与自定义指标配置、币种/汇率设置写接口 scope 与租户隔离成立。
- R164「PUT /settings 数值型静默忽略」评估为**非安全面**：数值型 `itemValue` 返回 400，请求体 `tenantId` 为 advisory，写入一律落 JWT 租户（注入 `tenantId:0` / 他租户均写回本租户）。
- govulncheck 0 可达；`pnpm audit --prod` 13 条全为前端构建工具链，与 R159 持平无增量。

## 3. 回归测试

`backend/internal/securitytests/permmatrix/r165_store_write_scope_test.go`（6 个用例）：审单决定、异常标记、店铺删除、店铺授权写、同步创建与重试、刊登目标店。均为「修复前失败（观测到 200 + 落库/删库）→ 修复后 403+40303 或 404 且零落库」，并含「授权 operator/admin 不被过度收紧」正例。

## 4. P2 清单（本轮不改行为）

1. `POST /shops/:id/test-connection`、`oauth/douyin/test` 对 view-only 放行（只读探测、无落库）。
2. `publish-targets/check`、`batch-targets/check` 对 view-only 放行（纯计算不落库，与订单 `sku-candidates/batch` 现口径一致）。
3. `PUT /settings` 数值型 `itemValue` 返回 400 `invalid body` 而非字段级中文提示（UX 而非安全）。
4. `pnpm audit --prod` 13 条构建链告警（与 R159 P2-3 同状态，随 admin 工具链升级窗口处理）。
5. readonly / view-only 前端仍展示部分写入口（点击后 403 拦截），R164 线2 已登记。
6. 建议把「写路由必须走 `EnsureStoreOperable`」做成静态检查或矩阵探针——本轮 6 处缺口全部源于写路径误用只读 scope 或漏闸门。

## 5. 与线1 的关系

本 PR 只含上述 6 处店铺 scope 收口 + 回归测试 + 报告，不触碰客服会话族（#330 已修），不重复线1 实现面。若线1 也改到 `order-review` / `orderexception` / `shop` / sync / publish，请以本 PR 为基准合并。

## 6. 验证

- `go build ./...`、`go fmt ./...`（无输出）、`go vet ./...` 通过。
- `go test ./...`（backend 全量，含 permmatrix / idor / shopscope，`APP_ENV=test` + Docker Postgres 隔离库）全绿。
- `pnpm test:backend:integration` 通过；`govulncheck ./...` 0 可达；`pnpm audit --prod` 无增量。
- 未做 UI 走查与 `seed:demo:full` 全栈演练（属其他线职责）。
