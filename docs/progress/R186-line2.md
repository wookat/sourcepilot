# R186 线2：客服/AI 工作流季度复查（qa-engineer + security-engineer）

日期：2026-08-08。距 R173 客服线上次全面验收已 12 轮，期间合入 view-only 客服线修复（#330 系列）、MCP 写白名单全量（R179–R183）、审计权限收紧（R184）。

## 【结论】

客服/AI 工作流全链路复查 **全部 PASS，无 P0/P1**：人工确认发送闸门与「绝不自动外发」红线无违背；MCP 读/写工具对消息/客服线零触点；readonly/view-only 写探针 403（40301/40303）且零副作用；R173 三项 P2 遗留均已修复/不复现；#330 零回退；双租户隔离与三角色三视口 UI 走查通过。新增 P2 一项（登记不阻塞）。

## 【口径与基线】

- 基线：main（`f9695c86`）本地叠加 OPEN PR #371、#372（R185 线1/线2，仅 `docs/PROGRESS.md` 文书性冲突，keep-both 解决）。
- 环境：`docker compose -f docker-compose.full.yml`（admin :8000 / backend :8080），`DB_HOST=127.0.0.1 pnpm seed:demo:full` 灌 demo 数据。
- AI 调用走本地 mock provider（settings `ai.openai_compatible_base_url` 临时指向本机，收尾已清空还原）；全程未触碰任何外部平台。
- Actions CI 不作依据，结论全部基于本地实测；录屏/截图/JSON 证据外置为附件，不入库。

## 【安全红线复验】

1. **MCP 写白名单零消息触点（静态审计）**：`backend/internal/modules/mcpserver/write_tools*.go` 白名单恒为 6 个工具（`orders_add_tag` / `orders_remove_tag` / `exceptions_mark` / `procurement_mark_placed` / `procurement_fill_logistics` / `procurement_mark_paid`）；读工具恒为 4 个查询（`orders_query` / `inventory_query` / `report_summary` / `exceptions_pending`）。**不存在任何能创建/发送/修改买家消息的 MCP 工具**；`write_tools.go` 注释明确「message sending and external-platform actions are permanently excluded」。R183 注册表守护测试（`r183_write_registry_test.go` 等）全绿，白名单外写不可达。
2. **人工发送闸门（UI 实测）**：AI 建议生成带「需人工确认不会外发」提示；「标记已发送」需 Popconfirm 且仅内部 PATCH；发送闸门取消时零写请求；demo 会话无 `external_conversation_id`，`send-platform-message` 直发被 400 拒绝（中文文案）。UI 中不存在任何自动外发路径。
3. **readonly 探针**：`POST /api/v1/customer/reply-templates` → 403 + 40301（中文只读文案）。
4. **view-only 探针**：PUT conversation、POST messages、generate-reply、send-platform-message、suggestion accept 五个写端点全部 403 + 40303；Postgres 前后计数/字段比对零副作用；#330 修复零回退；临时 `user_store_permissions` view 行已删净。

## 【功能复查（客服全链路）】

- 买家消息待发列表（8 条 seed 草稿）、草稿语言切换 + 重新生成（变体随语言变化、无发送请求）— PASS。
- 消息节点规则：仅新订单命中、回溯预估确认弹窗、回溯生成、重复回溯幂等（草稿数不变）— PASS。
- 会话详情：客户消息录入 → AI 建议（intent/sentiment/risk 标签）→ 语言切换重生成 → 模板插入变量填充 → 发送闸门 — PASS。
- 话术模板库 CRUD（新建含 en 变体/编辑/启停/排序/删除）— PASS。

## 【回归与隔离】

- R173 P2 遗留：① send-platform-message 英文/raw 报错已中文化；② `POST /api/v1/imports/{parse,validate,commit}` 无 body 时 scope 先行（403/40301 先于 400）；③ DEMO delivered_at 时间戳正常且未被「仅新订单」规则误命中 — 三项均闭环。
- 双租户隔离：tenant2 与 tenant1 互不可见；跨租户模板 PUT/DELETE 404/40401；会话详情跨租户 404；临时 tenant2 数据已物理删净。
- 三角色（admin/operator/readonly）× 三视口（1440×900/1024×768/375×812）走查客服 4 页面全组合：根节点无横向溢出、console 无 error、readonly 全部写按钮禁用 + 只读 Alert。覆盖说明：会话详情页 operator 视角受 store scope 限制，验证的是权限遮蔽行为而非布局。

## 【门禁】

- backend：`go vet` + `gofmt -l` 干净；`APP_ENV=test` + `TEST_DATABASE_URL`（Docker PostgreSQL `trademind_test`）下 `go test ./...` 全绿，含 permmatrix（view-only sweep/conversation/persona、buyermsg draft scope、R165–R176 scope-order 系列）与 mcpaudit/mcptoken/mcpwrite RacePostgres 系列。

## 【P2 清单（登记不阻塞）】

- **P2-1（新增）**：operator 在其 view-scope 店铺外打开会话详情显示「会话不存在或已被删除」，属权限遮蔽的预期行为，但与真实 404 文案不区分，可考虑差异化提示（保持不泄漏存在性的前提下）。

## 【下一步】

- 本报告 + SKILL 经验沉淀 PR → main，不自行 merge。
- P2-1 留待后续轮次评估文案方案。

## 【需注意】

- 无不可逆动作：mock AI 设置已清空、CDP 移动仿真已清除、tenant2/临时权限数据已删净、无真实外发。
- 证据（录屏、截图、探针 JSON/文本）作为会话附件交付，未 commit。
