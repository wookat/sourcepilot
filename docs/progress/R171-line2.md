# R171 线2：遗留 OPEN PR 核查（#245/#247/#248）+ 全仓 OPEN PR 盘点

- 日期：2026-08-08
- 角色：fullstack-engineer（company-os）
- 基线：`origin/main`（`1c9c2b5a`）
- 验证方式：权威 PR 状态（commit 祖先关系）+ main 代码实态核对 + 本地 `go test` 实测；Actions CI 不作依据；证据外置不入库

## 结论（SOP-04）

**#245/#247/#248 三个 2026-08-05 遗留 OPEN PR 的内容已经 100% 在 main 中，全部判定 A（建议关闭，不需合并，无需 rebase）。** 三个 PR 是一条 stacked 链（#244→#245→#247→#248，base 均非 main），2026-08-05 当天 #250（`fix/round117-audit-p2`）直接基于栈顶分支创建并合并进 main（merge commit `b82277ae`），整栈随之进入 main；三个 PR 因 base 是中间分支而未被 GitHub 自动关闭，属纯挂账。已在各 PR 评论区登记关闭依据（引用 #250 / `b82277ae` 与实测），按流程不自行关闭，待老板/负责人执行。

## 一、#245/#247/#248 逐个核查

判定依据（权威）：三个 PR 的 head commit 均为 `origin/main` 祖先，`git log origin/main..<PR分支>` 均为空（零领先 commit）：

| PR | head commit | 祖先关系 | main 实态核对 | 实测 | 判定 |
| --- | --- | --- | --- | --- | --- |
| #245 fix(order) 审单决定与发货推荐店铺 scope | `c283b475` | ✅ 在 main | `print_workbench.go` `lookupRecommendationOrder` 的 `EnsureStoreVisible`/`ApplyStoreScope` 原样在 main；审单决定路径已被 R165/R167 **进一步加强**为 `EnsureStoreOperable` + `ensureReviewBatchOperable`（整批 403/40303，#331/#335 定案），比 #245 的 `EnsureStoreVisible` 更严 | `go test ./internal/modules/order/... -run 'StoreScope|Review'` 绿（含 #245 带入的 `review_store_scope_test.go`） | **A 关闭** |
| #247 test(security) R109–R115 权限矩阵登记 + banned-words 白名单 | `ea1c9d21` | ✅ 在 main | `write_guard.go` 已含 `POST /api/v1/products/banned-words/check-batch` 白名单；`matrix.json` 已含 `/healthz` 与 R109–R115 登记（后续已扩至含 R160/R165 view-only 契约行）；`docs/permission-matrix.md` round116 小节在 main | `go test ./internal/securitytests/permmatrix/... -count=1` 全绿 | **A 关闭** |
| #248 chore(deps) axios 0.33.0 / immer 9.0.21 覆盖 | `cec578af` | ✅ 在 main | `package.json` `pnpm.overrides` 与 `pnpm-workspace.yaml` `overrides` 两处均含 `axios: 0.33.0`、`immer: 9.0.21`；#250 还在其上追加 `isomorphic-fetch>node-fetch 2.6.7`。PR 描述中「immer 跨大版本取舍待定」已被 #250 合并事实定案，且 main 上运行 3 天历经 v29 大回归 / R169 / R170 门禁均绿 | 前端门禁由 R169/R170 报告背书（本轮无前端改动） | **A 关闭** |

R170 线2 的判断「#245 疑似已被 R165/R167 收口覆盖」核实为**部分成立但非全貌**：#245 不只是被覆盖——它的 commit 本身就在 main（经 #250 整栈带入），R165/R167 是在其之上继续加严。#247/#248 同理在 main，与「覆盖」无关，纯属 stacked PR 未随 #250 合并自动关闭的挂账。

## 二、全仓 OPEN PR 全量盘点（截至 2026-08-08，共 4 个）

| PR | 标题 | 创建 | 一句话状态 / 建议 |
| --- | --- | --- | --- |
| #342 | R170 线1：R169 线2 P2×4 + UX v10 P2-3 收口 + 验收包补 R167–R169 增量 | 2026-08-08 | 盘点时活跃、门禁自述全绿（含 Docker 实跑，已叠加 #339/#340 内容）；**本轮登记期间已合并进 main（`f6889ebe`），不再挂账** |
| #248 | chore(deps) axios/immer 覆盖 | 2026-08-05 | 内容 100% 在 main（经 #250）；**建议直接关闭** |
| #247 | test(security) R116 权限矩阵登记 | 2026-08-05 | 内容 100% 在 main（经 #250）；**建议直接关闭** |
| #245 | fix(order) 审单/发货推荐店铺 scope | 2026-08-05 | 内容 100% 在 main（经 #250），且审单面已被 R165/R167 加严取代；**建议直接关闭** |

注：R167–R170 报告中提及的 #332/#333/#334/#335/#336/#337/#338/#339/#340 均已合并（#339/#340 内容亦随 #342 分支叠加），open 列表已无其余挂账。

## 三、待老板/负责人执行清单

1. 关闭 #245（评论已登记依据：https://github.com/wookat/sourcepilot/pull/245#issuecomment-5223839926）
2. 关闭 #247（https://github.com/wookat/sourcepilot/pull/247#issuecomment-5223841342）
3. 关闭 #248（https://github.com/wookat/sourcepilot/pull/248#issuecomment-5223841378）
4. ~~评审合并 #342~~（登记期间已合并，`f6889ebe`）
5. 本轮登记 PR（docs-only）合并

## 四、门禁说明

本轮为纯文档新增（本文件 + PROGRESS 变更记录），无代码改动；相关权威实测已在核查过程完成：main 上 `go test ./internal/modules/order/...`（scope 回归）与 `go test ./internal/securitytests/permmatrix/...` 全绿（本地 Docker PG/Redis + `trademind_test` 隔离库，2026-08-08）。证据（命令输出）留会话附件不入库。
