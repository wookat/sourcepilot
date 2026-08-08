# R180 线2：全站大回归 v33（OPEN PR #348–#360 叠加）

- 轮次：R180 线2（qa-engineer）
- 日期：2026-08-08
- 范围：最新 main 叠加全部 OPEN PR #348–#360 的全站回归 + Docker 全栈实测 + 合并顺序终版结论
- 基线：`origin/main`（`3fd5a35a`），集成分支 `devin/1786183509-r180-line2`
- Actions CI 不作依据，全部结论来自本地门禁与 Docker 实测；证据外置作附件不入库

## 1. PR 权威状态与叠加

#348–#360 共 13 个 PR 经权威状态核实全部 OPEN、mergeable，无一 merged，全部纳入叠加。

分支叠加关系（以远端 PR ref ancestry 实测，非仅凭描述）：

- #348 ⊂ #350、#356、#360
- #349 ⊂ #352、#353、#354、#355、#356、#359、#360
- #351 ⊂ #354、#356、#360
- #352 ⊂ #353、#354、#355、#356、#359、#360
- #353 ⊂ #355、#356、#359、#360
- #354 ⊂ #356、#360
- #355 ⊂ #359、#360
- #356 ⊂ #360
- #350、#357、#358 相互独立，也不包含于 #360

叠加结果：按依赖序合并至集成分支后，`git cherry` 逐一验证 13 个 PR 内容均已含于 HEAD。

冲突登记：仅 1 处**文书性**冲突——#357 与既有栈在 `docs/PROGRESS.md` 变更记录段冲突，按保留双方记录消解（HEAD 侧 R174/R176/R177/R179 + #357 侧 R178）。**未发现语义冲突。**

## 2. 合并顺序终版结论（最省事顺序）

`#360` 已完整包含 #348/#349/#351/#352/#353/#354/#355/#356/#359 的全部提交（R177 两线 + R179 两线叠加顶点），因此最省事顺序为：

1. **merge #360**（一次带入 9 个叠加 PR 的全部内容）
2. **merge #350**（独立线，无冲突）
3. **merge #357**（独立线，仅 `docs/PROGRESS.md` 文书冲突，按保留双方记录消解）
4. **merge #358**（独立线，无冲突）
5. #348/#349/#351/#352/#353/#354/#355/#356/#359 在 #360 合入后 diff 为空，直接 close（或标记已随 #360 合入）

如坚持逐个合并，等价依赖序：#348 → #349 → #350 → #351 → #352 → #353 → #354 → #355 → #356 → #357 → #358 → #359 → #360。

## 3. 全套门禁（叠加栈上全绿）

| 门禁 | 结果 |
| --- | --- |
| `go vet ./...` / `gofmt -l .` | 通过 / 无输出 |
| Go 全量 `go test ./...`（APP_ENV=test，隔离测试库/Redis DB15） | 103 packages 全部通过 |
| securitytests + permmatrix | 113/113 通过 |
| `pnpm check:dev` | 通过 |
| `pnpm check:ui-copy --strict` | 通过 |
| `pnpm test:frontend` | 57 文件 368/368 通过 |
| `pnpm test:contracts` | 17/17 通过 |
| `pnpm build:admin` / `pnpm build:collector` | 通过 |
| `pnpm test:collector` | 18/18 通过 |
| 全量 E2E `pnpm test:e2e` | 359 passed / 3 skipped（0 failed），约 34 分钟 |

## 4. Docker 重建镜像全栈实测

`docker-compose.full.yml` 重建全栈（postgres/redis/collector/backend/admin 全部 healthy），`seed:demo:full` 双租户演示数据。用例 × 结果矩阵：

| 场景 | 结果 |
| --- | --- |
| R57 主链路：订单列表/详情/SKU 匹配/自动化日志/库存影响/日报统计 | PASS |
| settings 敏感项写入加密、GET 脱敏（`ai/deepseek_api_key` → `sk-****-123`、isEncrypted:true） | PASS |
| settings 存量明文惰性收编：SQL 注入明文行 → GET 脱敏回显 → DB 自动变密文（`is_encrypted=t`） | PASS |
| readonly 账号写操作 → 403/40301 | PASS |
| operator 对不可见店铺订单写 → 404/40401（无存在性泄露） | PASS |
| operator 对 view-only 店铺订单写 → 403/40303；读仍 200 | PASS |
| 跨租户读他租户订单 → 404 | PASS |
| 双租户隔离：t2 店铺/订单列表零 t1 数据 | PASS |
| W1 闸门关（`MCP_WRITE_ENABLED=false`）：write 工具不注册、`orders_add_tag` 调用被拒 | PASS |
| W1 环境闸门开、租户闸门默认关：dry_run 被拒「租户级开关关闭」 | PASS |
| W1 租户闸门开、readonly-scope token：不可见 write 工具 | PASS |
| W1 execute 未携确认 token → 拒绝 | PASS |
| W1 dry_run → 一次性确认 token → execute（applied=1、限额扣减）→ 标签实际落到订单 | PASS |
| W1 确认 token 重放 → alreadyExecuted、不重复执行 | PASS |
| W1 审计落库：`GET /api/v1/mcp/audit-logs` 含全部 `orders_add_tag` 成功/拒绝行 | PASS |
| Modal 防重：Playwright 双击创建标签确定钮 → 仅 1 次 POST、服务端仅 1 行 | PASS |
| 双租户零残留：`seed:demo:full:clean` + `verify` → zero DEMO- residual rows | PASS |

## 5. P0 / P1 / P2

- **P0：无**（本轮未发现）。
- **P1：无**（R177 modal 防重修复、R179 W1 链路在叠加栈上均无回退）。
- **P2 清单**（继承为主，本轮无新增）：
  1. （承 R177）`docs/mcp-tokens` 相关文档对 token 展示口径为纯文本说明，待补脱敏示例；
  2. （承 R177）finance-report CSV 导出缺按结算币种折算列；
  3. （承 R177）`pnpm audit --prod` 15 条 admin 构建工具链告警（构建期依赖，非运行时暴露）；
  4. （承 R179 W2）设置页缺租户 `mcp/write_enabled` 开关 UI 与写 token 创建 UI，其余写动作待 W2+ 接入，金额上限为 W5 设计项。

## 6. 说明

- 本轮为 QA 回归集成演练，PR #348–#360 不自行 merge，合并顺序结论供负责人执行。
- 证据（门禁输出、Docker 场景脚本与结果 JSON、防重脚本输出）外置演练机 `/home/ubuntu/evidence/r180/`，作附件不 commit。
