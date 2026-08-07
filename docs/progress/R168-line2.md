# Round 168 线2：生产升级演练复跑（R164 基线 → 最新 main + 未合并安全系列叠加）

日期：2026-08-07。演练环境：独立副本（production compose + Caddy，`DOMAIN=localhost` 自签路径）。背景：R164 演练后 view-only 安全大批修复陆续推进——#330/#331 已合入 main（`d645ec96`），#332（round165 view-only 写入口全站扫尾）与 #335（R167 审单批量整批 403 定案）仍未合并。证据文件（部署日志、指纹、探针输出）留存演练机 `/tmp/r168/`，按惯例不入库。

## 0. 演练版本口径（未合并系列叠加，需注明）

- 基线：R164 时代 main `a78e2fb0`（与 R164 线1 演练同点位）。
- 升级目标：本地演练栈 `drill/r168-line2-stack` = main `d645ec96`（含 #330/#331）+ `fix/round165-line1-viewonly-sweep`（#332）+ `devin/1786135000-r167-line1-review-batch-403`（#335），合并提交 `15ff223c`。
- 叠加时 4 处冲突（`order/review_workbench.go`、`permmatrix/view_only_sweep_test.go`、`docs/PROGRESS.md`、`docs/permission-matrix.md`），代码侧按 R167 定案收口（审单决定统一 `EnsureStoreOperable`、整批 403/40303），`go build ./...` 通过。#333/#334/#336 为文档/报告类 PR，不含运行时代码，不叠加。
- 提示：#332/#335 与 main 的文档（PROGRESS/permission-matrix）存在重复段落冲突，合并落地时需人工收口（见 §7 P2③）。

## 1. R164 双租户存量基线升级

基线（`a78e2fb0` 上构造，口径对齐 R164）：双业务租户（tenant 2/3），20000 订单（含跨租户重复订单号 DUP-1..5）、40000 订单行、60000 库存流水、20000 自动化日志（半数 `shop_id` NULL 模拟存量）、4000 SKU、4000 回款、5 枚 MCP token（purpose mcp/openapi/both 混布）、20 话术模板 + 20 多语言变体、view-only 店铺授权样本（operator 仅 view 店铺 A1）。

- **AutoMigrate**：升级部署（`--no-pull`，重建 backend 镜像）195s，六服务 healthy；启动迁移成功，无 ERROR/migrate_failed；日志可见 `UPDATE order_automation_logs … WHERE shop_id IS NULL`（10000 行，约 294ms）。
- **数值指纹**：订单/订单行/库存流水/SKU/回款/token hash/模板/变体/自动化日志业务字段 9 类指纹升级前后 **0 差异**；唯一变化为预期迁移 `order_automation_logs.shop_id` 回填（NULL 10000→0）。跨租户重复订单号样本未误报，同租户 0 重复。

## 2. 升级后六处 view-only 修复面 + 跨租户实测（重建镜像后实测）

view-only 账号（operator，仅 view 店铺 A1）逐面探针（HTTP/业务码/中文文案/零落库四项均核对）：

| # | 修复面 | 路由 | 结果 |
| --- | --- | --- | --- |
| 1 | 审单决定 | `POST /order-review/approve|reject` | **403 + 40303**（整批拒绝，「店铺无操作权限」，R167 定案口径） |
| 2 | 异常标记族 | `POST /orders/exceptions/order/:id/handle|ignore` | **403 + 40303**（「当前账号无该店铺的操作权限」） |
| 3 | 店铺删除 | `DELETE /shops/:id` | **403 + 40303**（「当前账号无权访问该店铺数据」） |
| 4 | 店铺授权写 | `PUT /shops/:id/auth` | **403 + 40303** |
| 5 | 同步创建 | `POST /shops/:id/sync-orders` | **403 + 40303** |
| 6 | 刊登目标店 | `POST /products/:id/publish-targets/create-drafts` | **403 + 40303** |

- **零落库**：探针后 `review_status` 变更 0、软删店铺 0、新建同步任务 0、异常标记 0。
- **跨租户 404**：tenant A admin 访问 tenant B 资源——读/删店铺、`sync-orders`、异常标记均 **404**（40401，不泄露存在性）。批量审单跨租户订单为 **HTTP 200 + 整批 failed（行级「订单不存在」，0 生效）**，与 `docs/api.md` 口径一致（view-only 整批 403；不可见店铺订单逐行「订单不存在」）。
- 探针注意：异常标记/刊登 create-drafts 的参数校验先于 scope 判定（缺 `exceptionType`/`targets` 时先 400），探针需带合法 body（见 §7 P2②）。

## 3. 从零部署最新 main（<15 分钟复验）

- 目标版本 main `d645ec96`；删除应用镜像与 build cache、全新卷，从零 `./scripts/deploy-prod.sh --no-pull`：构建 + 启动 + 健康检查 **223 秒**（R164 为 236s），远小于 15 分钟；六服务全部 healthy，`/health-backend` code=0（database/redis ok），bootstrap 管理员登录 200。

## 4. TRUSTED_PROXIES / OPENAPI 口径抽验

- `.env`：`TRUSTED_PROXIES=172.18.0.0/16`、`OPENAPI_ENABLED=true`。
- XFF：外部伪造 `X-Forwarded-For: 9.9.9.9`（升级栈）/`7.7.7.7`（从零栈）经 Caddy→admin 链，后端 `client_ip=172.18.0.1`（伪造不落地）；可信网段内（admin 容器）直连 backend 带 `X-Forwarded-For: 8.8.8.8` → `client_ip=8.8.8.8` 正确落地。与 `.env.prod.example`/docs 口径一致。
- OPENAPI：无 token 401；`purpose=openapi` token 200；`purpose=mcp` token 调开放 API 401；`OPENAPI_ENABLED=false` 重启后 404；改回 true 恢复 200。

## 5. --pre-upgrade-check 与备份→恢复→幂等重跑

- 非 root 默认 `/var/backups` 不可创建 → 清晰报错并提示 `BACKUP_DIR=<可写目录>` 覆盖（#317 口径，与脚本注释/upgrade-guide 一致）；`BACKUP_DIR` 覆盖后：pg_dump 备份生成（8.4MB 非空）、同租户重复订单号预检 0 行通过。
- 恢复：停 backend → `pg_restore --clean --if-exists` 恢复升级前 dump → 重启 backend 触发 AutoMigrate（`shop_id` 回填重新落地）→ 指纹与首次升级后逐项一致；再次重启幂等重跑：指纹 0 差异、日志无 error/fatal/panic。

## 6. 文档/示例核对

`docs/upgrade-guide.md`（备份→预检→checkout→部署顺序、BACKUP_DIR 口径、AutoMigrate/回填清单）、`docs/api.md`（审单批量整批 403/40303 与不可见店铺逐行「订单不存在」）、`.env.prod.example`（TRUSTED_PROXIES/OPENAPI 注释）与实测行为一致，**未发现 P0/P1 失实**，本轮无需代码/文档修复 PR。

## 7. P2 清单（登记不阻塞）

1. R166-line2/PROGRESS 表述「六面 403/40303 均有中文提示『店铺无操作权限』」，实测文案分三种（店铺无操作权限 / 当前账号无权访问该店铺数据 / 当前账号无该店铺的操作权限），语义一致但措辞与记录不完全一致，建议后续统一文案或修订表述。
2. 异常标记/刊登 create-drafts 等路由参数校验先于店铺 scope 判定（缺参时 view-only 得 400 而非 403），无越权与存在性泄露，但探针/契约测试需带合法 body；文档未明示该顺序。
3. #332/#335 长期未合并，与 main 的 `docs/PROGRESS.md`/`docs/permission-matrix.md` 已产生重复段落冲突，实际合并时需人工收口（本轮演练栈已给出一种收口参考：保留 R167 整批定案 + round165 全站扫尾 113 条矩阵表述）。

## 8. 结论

R164 基线双租户存量数据升级到「最新 main + #332/#335 叠加」全流程通过：AutoMigrate 落地、指纹 0 差异（仅预期回填）、六处 view-only 修复面 403/40303 与跨租户 404 实测生效（backend 镜像重建后验证）；从零部署 223s；`--pre-upgrade-check`、备份→恢复→幂等重跑全部通过；未发现 P0/P1 文档失实，P2×3 登记。
