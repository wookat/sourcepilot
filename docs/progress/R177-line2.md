# R177 线2：全站大回归 v32（qa-engineer）

- 日期：2026-08-08
- 基线：最新 `origin/main` + 按依赖顺序本地叠加全部 OPEN PR #348/#349/#350/#351/#352/#353/#354（经权威 `git_view_pr` 核实均 OPEN、mergeable）
- 集成分支：`devin/1786174692-r177-line2`；冲突仅 `docs/PROGRESS.md` 文书性（三次，均保留双方轮次记录），无语义冲突

## 一、合并顺序结论

- 依赖关系：#350 已携带 #348；#352 基于 #349；#354 已携带 #349/#351/#352；#353 独立（仅与他 PR 在 `docs/PROGRESS.md` 有文书性冲突）。
- 建议合并顺序：**#348 → #350 → #349 → #352 → #351 → #354 → #353**（#353 也可提前，任意相邻顺序仅需解 PROGRESS.md 文书冲突）。本轮 R177 PR（含 P1 修复）须在 #352/#354 之后合并。
- 无语义冲突登记项。

## 二、门禁（集成分支全量）

| 门禁 | 结果 |
| --- | --- |
| Go 全量 `go test ./...`（隔离 test DB/Redis） | 103 包 ok，0 FAIL |
| securitytests（含 permmatrix/idor/shopscope） | 111/111 PASS（permmatrix 6.2s 全过；含 R176 修复面回归 5 项） |
| `gofmt` / `go vet` | 空 / 通过 |
| `pnpm check:dev` / `pnpm check:ui-copy --strict` | 通过 |
| `pnpm test:frontend` | 368/368（含本轮新增 3 条防重复提交回归） |
| `pnpm test:contracts` | 17/17 |
| `pnpm build:admin` / `pnpm build:collector` | 通过 |
| 全量 Admin E2E | 修复前 358 过 / 1 失败 / 3 跳过；失败为本轮 P1（见下），修复后 inventory-center 17/17 全过 |

## 三、Docker 全栈实测（backend 镜像重建 + `docker-compose.full.yml`）

`DB_HOST=127.0.0.1 pnpm seed:demo:full` 种子后 API/UI 实测 30+ 断言全过：

- **R57 主链路**：DEMO-AT-1004 标记付款 → 自动化触发 `generate_procurement` success（「已自动生成 1 张采购单」）+ 打标 + 分仓 + 发货规则；DEMO-AT-1002 负样本 blocked/skipped 正常。
- **#353 修复面 1**：租户 B 用租户 A `shopId` 调 `/imports/validate|commit` → 404/40401；DB 内跨租户 `import_jobs` 残留 0 行。
- **#353 修复面 2**：已加密 `deepseek_api_key` 再次 PUT 省略 `isEncrypted` → DB `is_encrypted` 保持 true、密文落库、GET 不回显明文。
- **modal 失败路径（UI 实测 :8000）**：调整库存注入后端 500 → 中文 toast、弹窗保持打开、无 error-overlay、无 pageerror。
- **migrationimport 中文文案**：`/settings/migration` 向导中文；API shape 校验中文（「表头列（columns）不能为空」「kind 需为 product、order…」）。
- **view-only/40303/跨租户 404**：view-only 订单写/打标/客服发送/标记已回复 → 403+40303「店铺无操作权限」（先 scope 后 body）；会话详情 canWrite=false；readonly 写 → 403+40301；租户 B 读/写租户 A 订单 → 404；租户 B 店铺列表 0 条 A 租户数据。
- **MCP/开放 API 抽验**：purpose=both token 走 `/api/open/v1/orders` 200、MCP initialize 200；purpose=mcp token 走开放 API → 401。
- **双租户零残留**：临时租户 B、店铺权限、MCP token、mcp_tool_call_logs 全部清理；`seed:demo:full:clean` + `verify: zero DEMO- residual rows`。

## 四、问题与修复

| 级别 | 问题 | 处置 |
| --- | --- | --- |
| P1-1 | #352 将静态 Modal.confirm onOk 从 async 改为「手动 close」后，丢失 antd 原生 pending 防重（await 期间禁用确定钮），双击敏感确认可产生 2 次写请求（E2E `inventory-center` 防重断言实测 2≠1） | 已修：`modalOk` / `confirmSensitiveAction` 增加 in-flight pending 守卫（提交中忽略重入、失败后复位可重试）；新增 3 条单测；E2E inventory-center 17/17 复绿 |
| P2 | 遗留维持：v10 P2-3（mcp-tokens 文档纯文本）、v9 P2-3（finance-report CSV 未折算列）、R176 `pnpm audit --prod` 15 条（均 admin 构建工具链） | 维持口径，本轮不改 |

## 五、收尾

- 证据（API 断言输出、E2E 报告、UI 走查日志）留档外部附件不入库；Actions CI 不作依据。
- 源 PR #348–#354 不自行 merge；本轮修复以 R177 PR 提交。
