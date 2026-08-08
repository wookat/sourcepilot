# R174 线2：全站大回归 v31（qa-engineer）

日期：2026-08-08。#347（40303 回归 + handle/ignore 先 scope 后 body，含 #346）已合入 main。本轮基于最新 `origin/main`（`3fd5a35a`，含 #347）核实开放 PR 权威状态、叠加剩余 OPEN PR、跑全套门禁 + Docker 全栈实测。录屏/日志证据外置作会话附件不入库；Actions CI 不作依据（结论基于本地实测）。

## 1. 开放 PR 权威状态核实与合并顺序结论

| PR | 权威状态 | 处置 |
| --- | --- | --- |
| #342 | merged | 跳过 |
| #343 | merged | 跳过 |
| #344 | merged | 跳过 |
| #345 | merged | 跳过 |
| #348（R173 线2 报告，纯文档） | open，mergeable | 本轮叠加实测 |
| #245 / #247 / #248 | open 但 head 均为 main 祖先（内容已在 main） | 建议直接关闭，不再合并（与 R171 结论一致） |

**合并顺序结论：直接合并 #348 即可（纯文档、无代码冲突）；#245/#247/#248 建议关闭。** 测试栈 = main `3fd5a35a` + #348 分支 merge（`2f6fca00`），仅 docs 变更，无代码冲突。

## 2. 全套门禁（本地实测，全绿）

- Go：`go fmt`（无 diff）、`go vet`、全量 `go test ./...` 103 包 ok。
- securitytests / permmatrix：107 PASS，含 #347 新增 `TestExceptionMarkScopeBeforeBody`（view-only 空 body → 403/40303；不可见 → 404；可操作空 body → 400）单独复核通过。
- 前端门禁：`pnpm check:dev`、`pnpm check:ui-copy --strict`、`pnpm test:frontend`、`pnpm test:contracts`、`pnpm test:collector`、`pnpm build:admin`、`pnpm build:collector` 全部通过。
- 全量 Admin E2E：**359 passed / 3 skipped / 0 failed**（38.9m；3 skipped 为 `@p8-real-backend` 条件跳过，需真实后端环境，历轮口径一致）。

## 3. Docker 全栈实测（backend 镜像重建，35/35 断言通过）

`docker-compose.full.yml` 重建 backend 镜像后全栈拉起 + `seed:demo:full`，脚本化断言 35/35 PASS：

- **R57 主链路正向**：DEMO-AT-1004 标记已付款 → 自动化轨迹落痕 → 采购单自动生成（PO 计数 10→11）。
- **异常 handle/ignore 新校验顺序（空 body 三档，#347 修复面实弹）**：view-only 源 403 + 40303 +「店铺无操作权限」；不可见源 404；可操作源 400（缺 exceptionType）。handle/ignore 两动作 × 三档全过。
- **view-only 修复面写探针**：shop update/delete、`sync-orders`、`sync-customer-messages` 均 403/40303/统一文案；readonly 写 403。
- **跨租户 404**：tenant-B 读/写 tenant-A 订单、读店铺、异常 handle 一律 404，不泄露存在性。
- **MCP / 开放 API 抽验**：创建 token（purpose=both，plaintext 一次性下发）→ `GET /api/open/v1/orders|inventory` 200 → MCP initialize + tools/list（4 tools）→ revoke 后开放 API 401。
- **双租户零残留**：tenant-B 停用 → `confirmName` 确认清退 → users/shops/orders 残留全 0；demo 数据 `seed:demo:full:clean` + `verify` 输出 `zero DEMO- residual rows`。

实测中的注意点（均为预期行为，非缺陷）：店铺授权变更会吊销该用户会话（40101，需重新登录）；租户清退需先停用并按租户名确认；自动化规则按 `tenant:rule:order:event` 去重，同一订单同一事件只执行一次。

## 4. P0/P1

无。本轮无需修复代码。

## 5. P2 清单（登记不阻塞）

1. `/admin/users/:id/store-permissions` 仅有 PUT，无对应 GET（授权明细需经 `GET /admin/users/:id` 的 `storePermissions` 读取）：接口不对称，工具化/开放场景略不便，建议后续补 GET 或在 api 文档明确读取口径。

## 6. 清理与残留核对

- r174 临时 order_sync_tasks（cursor=`r174-line2`）已删；operator 授权已还原；tenant-B 已清退并核残留为 0；demo seed clean+verify 零 `DEMO-` 残留。
