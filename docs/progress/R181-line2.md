# R181 线2：验收包补 R176–R180 增量 + MCP 写演示点并入 + 三角色/view-only 实跑

- 轮次：R181 线2（qa-engineer）
- 日期：2026-08-08
- 范围：纯文档（验收对照表 + DEMO_SCRIPT + 本进度文档）；实测基于 main 叠加 #361 分支构建（#361 已含 #360 全部提交）
- 分支基线：`main`（00cc577c，#359 合并后），PR → main，不自行 merge
- Actions CI 不作依据，全部结论来自本地门禁与 Docker 实测；证据外置不入库

## 1. 验收对照表（ACCEPTANCE_R123.md）

新增「§一/21 R176–R180 增量能力（R181 增补）」：R176 安全审计 P1×2 修复（#353 ✅）、UX v12（#354 ✅）、settings 敏感 key 服务端注册表（#355 ✅）、大回归 v32 + modal 防重复提交（#356 ✅）、R178 生产演练（#358 ✅）、竞品复评 v10（#357 ⏳ OPEN）、存量明文惰性收编（#359 ✅）、MCP 写白名单 W1（#360 ⏳ OPEN）/W2（#361 ⏳ OPEN）、大回归 v33（#362 ⏳ OPEN）、本轮 R181 线2。合并状态均按 `git log origin/main` + PR 权威元数据核对（2026-08-08 时点），未合并一律标 ⏳。竞品章节补 v10 增补段（超越 4 / 达到 12 / 落后 0，对话式写差距随 W1/W2 合入收敛）。

## 2. DEMO_SCRIPT 并入 MCP 写演示点

- 新增第 23b 步（约 2 分钟）：写白名单卡片/租户开关风险确认 → 创建 write:ops token → curl dry_run（preview + summary + 一次性 confirmationToken，TTL 5 分钟）→ execute → 重放 alreadyExecuted 幂等 → 审计三列（mode/paramsSummary/confirmHash）。
- 时长守恒：第 23 步只读 MCP 现场 curl 压缩为仅 `orders_query` 一次（`tools/list` 口头带过），开放 API 保留 orders 一条 curl；总长维持 ~30 分钟。
- 前置口径如实标注：23b 需 main 叠加 #361 构建且 `MCP_WRITE_ENABLED=true`（默认 false 时写工具完全不注册，`tools/list` 为空亦实测确认）。

## 3. Docker 全栈实跑（main 叠加 #361）

`docker-compose.full.yml` 全栈（postgres/redis/collector/backend/admin，`MCP_WRITE_ENABLED=true`、`BACKUP_ENABLED=true`/`BACKUP_MODE=local`）+ `seed:demo:full`。

### 3.1 MCP 写链路 API 实测（全通过）

- 写 token 创建 admin-only：demo_readonly 40301「只读权限」、demo_operator 40301「仅管理员可创建带 write:ops 作用域的 token」；scope 白名单仅 readonly/write:ops（`read:all` 40001 拒）。
- 三层闸门逐层独立拒绝：env 关 → 写工具不注册（tools/list 空）；env 开租户关 → `mcp write: 写操作未开启（租户级开关关闭）`；全开 → 5 个写工具注册（W1 `orders_add_tag`/`orders_remove_tag` + W2 `exceptions_mark`/`procurement_mark_placed`/`procurement_fill_logistics`）。
- dry_run 返回 preview/summary/confirmationToken（TTL 5 分钟）+ 限额余量（token 30/时、租户 200/天）；execute 成功变更（applied=1）；同 token 重放 `alreadyExecuted=true` 不重复变更。
- 审计 `/api/v1/mcp/audit-logs`：dry_run（success）、execute（success，confirmHash 有值）、重放（error）各一行，paramsSummary=`orderNo=… tag=…`。

### 3.2 三角色 + view-only UI 实跑（录屏为证，全脚本通过）

- admin 主线 1–19、双租户隔离、operator 范围 + 批量收款、readonly 控件、operator/readonly 不见写白名单卡片；
- view-only 临时账号：读可见、写被拒中文 toast「店铺无操作权限」、DB 无变更，账号事后硬删验证 count=0；
- 23b UI：写白名单卡片（admin 专属）、三层闸门说明、租户开关风险确认弹窗、订单列表标签真实变更、审计表三列有值；
- 22 步响应式：375px 底部 5 tab 无侧栏、≥768px 侧栏恢复；23 步平台管理员备份 completed/仅本地、批量批准弹窗（为保 clean 验证仅核对弹窗未实际批准）。

### 3.3 失实即修（本轮修正）

- 审计行数口径：重放 execute 记 error 行（非仅 dry_run/execute 两行），DEMO_SCRIPT 23b 已按实测修正；paramsSummary 键名为 `tag=`（非 tagName）。
- settings PUT body 为 `items[].groupKey/itemKey/itemValue`；登录 body 为 `account/password`（脚本既有口径一致，未改）。

## 4. 门禁与清理

- 前端门禁全绿：`check:ui-copy --strict` / `test:frontend` / `test:contracts` / `build:admin` / `build:collector` 退出码全 0（本轮纯文档改动，无后端 Go 变更，go 面门禁不适用）。
- `seed:demo:full:clean` + `verify`：**zero DEMO- residual rows**；另手工清理本轮测试残留（write token、7 行 `mcp_tool_call_logs`、`settings mcp/write_enabled`、备份 job/artifact 记录、临时 view-only 账号），复查全 0。demo_* 登录账号与平台管理员按既有口径保留。

## 5. 遗留与风险

- #357/#360/#361/#362 仍 OPEN：对照表相应行与 DEMO_SCRIPT 23b 标 ⏳，合并后建议下一轮把状态改 ✅ 并去除「叠加分支」前置。
- 合并顺序沿用 R180 线2 结论：#360 → #361（含 #360）→ #362 → #357。
- `mark-paid` 刻意留 W3；MCP 无外发工具面维持断言。

## 6. 证据（外置，不 commit）

UI 实跑录屏 + 关键截图（写卡片/风险弹窗/标签变更/审计三列/view-only 拒绝/备份/响应式）作为 PR 附件交付；API 实测响应片段见本文 §3.1。
