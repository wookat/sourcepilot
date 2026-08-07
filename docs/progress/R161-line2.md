# R161 线2：合并后收尾杂项 + E2E flaky 收口（fullstack-engineer）

## 背景与基线

- 分支：`fix/round161-misc`，基于最新 `main`（`b2d20535`，#317/#319/#320 已合入）。
- 开放 PR 状态（本轮开始时）：#318（mergeable）、#321（mergeable）、#322（mergeable）、#323（与 main 冲突，已代为解决，见下）。
- 说明：任务口径引用的「R160 大回归 v27 报告」未在仓库、开放 PR 描述与评论中检索到（`docs/regressions/`、`docs/progress/`、PR #316–#323 均无 v27 字样），本轮按任务文本给出的结论（round142 768px 用例 flaky，P2）直接定位处理。

## 1. E2E round142 768px 用例 flaky 收口（✅）

- **根因**：`round142-msg-scope.spec.ts`「768px 侧栏含客服入口且可直达客服中心」用例的动线是单次 `hover(客服 submenu)` → `click(客服中心 menuitem)`。hover 弹出的 submenu 弹层可能在 hover 与 click 之间因菜单重渲染而关闭（dashboard 数据到达触发徽标/菜单更新、ProLayout 断点 settle），click 即超时——典型竞态，负载越高窗口越大（本机 CPU 加压下用例耗时 6s → 19s，接近 45s 用例超时）。
- **修法**：将 hover→click 整段包进 `expect(...).toPass({ timeout: 20_000 })` 重试（单次 click 限时 2s），弹层意外关闭时自动重新 hover；不放宽任何断言，不改生产代码。
- **验证**：
  - `--grep "768px" --repeat-each 5` 全绿（约 6s/次）；
  - 整文件 `--repeat-each 3`（12 用例）全绿 ×2 轮；
  - CPU 加压（2 核跑 4 个忙循环）下 `--repeat-each 10` 全绿；
  - `@smoke` 6 用例全绿，无联动回归。

## 2. 合并期红灯处理（✅ 一处）

- **#323 与 main 冲突**（#317/#320 合入后产生）：`docs/mcp.md` + `docs/PROGRESS.md` 两处内容冲突，已在 `fix/round160-audit-p2` 上代为 merge main 并解决（提交 `251c5272`）：
  - `docs/mcp.md`：保留双口径——`tools/call` 逐次审计 fail-closed（JSON-RPC `-32603`，取 #317 合入 main 的口径）+ 入口级 401/429 留痕 best effort 不阻断响应（取 #322/#323 侧补充说明）。
  - `docs/PROGRESS.md`：变更记录并集（main 的第 156 轮线1 条目 + 分支的第 159/160 轮线1 条目，按轮次排序）。
  - 合并树上 `go build`/`go test ./...`、`pnpm test:contracts`（17）全绿后推送。
- #318/#321/#322 当前均 mergeable，无冲突残留与门禁红；若后续合入产生新红灯，按同口径处理。

## 3. R159/R160 合入面前端体验一致性巡检（✅ 未发现问题）

- **错误提示中文化**：`httpErrorCopy`/`errorMessages` 全站兜底健全——envelope 中文 message 优先、英文原文按 `BACKEND_MESSAGE_COPY` 映射、状态码中文兜底（403「没有权限执行该操作」）；view-only 403 的后端 message 本身已是中文（`adminperm.ErrStoreNotOperable`「店铺无操作权限」、`authorize.go`「当前账号无权访问此店铺」），40301→40303 业务码统一不影响前端（无按业务码分支的逻辑）。
- **UI 禁用态**：买家消息（`canWrite=false` 时批量标记/立即生成/单行操作均 disabled + 中文 tooltip）、订单/库存/财务等页面 `canWrite` 禁用态齐全。
- **非法入参 400**：R160 收口面在 Open API/MCP（无 admin 前端交互面）；admin 侧 400 统一走「请求参数有误，请检查后重试」兜底，一致。
- 结论：无需改动，未发现 P2 及以上问题。

## 验证（受影响门禁）

- `pnpm check:dev`（补 `.env` 后通过）、`pnpm check:ui-copy --strict`、`pnpm test:frontend`（349）、`pnpm test:contracts`（17）全绿。
- backend `go build`/`go vet`/`gofmt -l`/`go test ./...` 全绿（main 基线确认；本分支无 Go 改动）。
- E2E：round142 全文件 `--repeat-each 3` ×2 + 768px 用例 `--repeat-each 5` + 加压 `--repeat-each 10` + `@smoke` 全绿。
- **Docker 实测**：`docker compose up -d`（PostgreSQL16+Redis 均 healthy）起真实后端（`:8080`，health 200）+ admin dev（`:8001`），768×900 视口真实登录 → 侧栏「客服」submenu hover → 「客服中心」直达 `/customer/hub`，根节点横向 overflow 0px；截图/日志证据外置 `/tmp/r161/`（不入库）。

## 遗留与下一步

- 「大回归 v27 报告」原文未找到（可能在未归档的会话侧），若后续入库发现 768px 之外的 P2 项，另轮跟进。
- #318/#321/#322/#323 合入期间如出现新的 PROGRESS/mcp.md 冲突或门禁红，按本轮口径继续处理（mcp.md 双口径为准）。
