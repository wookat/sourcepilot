---
name: demo-fullstack-walkthrough
description: TradeMind Docker 全栈 demo 环境手工走查要点：seed 账号、关键路由、真实触发方法、清理与常见坑
---

# Docker 全栈 demo 手工走查

## 环境
- `docker compose -f docker-compose.full.yml up -d`：Admin http://127.0.0.1:8000，后端 :8080（容器跑的是构建时的分支代码）。
- 验证未构建进镜像的前端改动（如 PR 回归）：不必重建镜像，直接 `PORT=8001 pnpm --filter @trademind/admin dev` 起 dev server（Umi proxy 已指向 127.0.0.1:8080），浏览器走 http://127.0.0.1:8001。
- `/ops/restores` 常为空态；要验证列渲染可创建一条恢复验证（会被安全门拒绝，属预期：目标库名需 `trademind_p6v_restore_` 前缀），拒绝记录仍会入列可用于检查时间/状态列。
- 灌数据：`DB_HOST=127.0.0.1 pnpm seed:demo:full`；收尾 `seed:demo:full:clean` 后再 `seed:demo:full:verify`（verify 只在 clean 后有意义，期望输出 `zero DEMO- residual rows`）。
- demo 账号：`demo_admin@trademind.local / DemoAdmin123!`、`demo_operator@trademind.local / DemoOperator123!`、`demo_readonly@trademind.local / DemoReadonly123!`（密码各不相同，见 scripts/seed-demo-permissions.ps1）。

## 关键路由
- 自动化规则 `/settings/order-automation-rules`；执行日志 `/orders/automation-logs`；订单详情自动化轨迹深链 `/orders/<uuid>?tab=automation`。
- 买家消息 `/customer/buyer-messages`（tab：待发消息/节点规则）；话术模板 `/customer/reply-templates`。
- 选品 `/selection/tasks/<uuid>`（候选行「数据面板」抽屉、多选后「对比所选」抽屉可导出 CSV 到 ~/Downloads）。
- 运营任务批量审批弹窗在 `/ops/task-center/operation-tasks`（勾选「待审核」行 → 批量批准）。
- 采购列表入口在侧栏「货源与采购」，`/purchase/orders` 是 404，不要用该路径深链。

## 真实触发自动化
- 找 unpaid、审单通过、带商品行的订单 → 订单列表批量「标记已付款」→ automation-logs 出现新记录。seed 样本映射：DEMO-AT-1001 = 自动确认付款成功；DEMO-AT-1002 = 「生成采购单」因 SKU 未匹配货源被安全阻断（负样本）；DEMO-AT-1004 = 商品行已匹配本地 SKU 的正样本，标记已付款后应生成采购单并可跳转。
- pending_review/held 订单（DEMO-AT-1003）会被记录为「跳过」。

## 常见坑
- 注册验证码依赖 SMTP；未配置时可临时注入：`docker exec trademind-full-redis-1 redis-cli set "email_code:register:<email>" "<code>" EX 600`，UI 中填该 code 完成注册（仅测试环境）。
- seed clean 不会删除自行注册的租户账号，需要单独处理。
- 接手已运行的 full 栈环境时先确认 demo 数据在库（`SELECT count(*) FROM orders WHERE order_no LIKE 'DEMO-%'`）；clean 过则需重新 `DB_HOST=127.0.0.1 pnpm seed:demo:full`。MCP `tools/call` 需先 `initialize` 并带 `Accept: application/json, text/event-stream` 头。
- seed clean 只清理 `DEMO-` 前缀命名的 MCP/开放 API token（`mcp_api_tokens.name LIKE 'DEMO-%'`）；e2e 测试临时创建的其他命名 token（如 `e2e-refresh-verify`）clean/verify 都不覆盖，测试收尾需自行吊销或删除。
- 响应式检查用 DevTools device toolbar；默认 auto-fit 缩放（29%）截图不可读，把 zoom 改成 100%。硬指标用 `document.documentElement.scrollWidth <= clientWidth` 判断根节点横向溢出。
- 移动端（375）出现底部导航（首页/订单/采购/库存/我的），表格是横向内滚，不算根节点溢出。
- 登录 API 请求体字段为 `{"account","password"}`（不是 `email`），脚本化取 JWT 时注意。
- 大屏卡片配置弹窗（R156 #318）没有「恢复默认」按钮，演示后需手动改回启用/顺序再保存。
- 大屏销售额/毛利卡「已折算：X」与「未折算（不计入合计）…」两行互斥：存在未折算币种时只显未折算行；seed 多币种样本（EUR 无汇率）下看不到「已折算」行，属预期。
- `/ops/backups`、`/ops/restores` 是平台级路由：租户管理员（demo_admin）会收 40302/「暂无访问权限」，属预期隔离；走查需用 bootstrap 平台管理员 `admin@example.com / admin123456`。
- MCP 审计日志表是 `mcp_tool_call_logs`（不是 mcp_call_audits）；开放 API 可用端点为 `/api/open/v1/orders`、`/api/open/v1/inventory`（`/summary` 404）。填充审计卡时直接用 token 调这两个端点。
- `/settings/report-currency` 的 unsaved 提示只是内联文案，可能没有路由级拦截（v10 走查发现为 P1）：验证时必须真的点侧栏离开再返回确认值是否丢失，不能只看提示出现。
- 导出防重复验证法：快速双击导出按钮后检查 `~/Downloads` 文件数（应只有 1 个 CSV），比只看 toast 可靠。
- 登录页路由是 `/user/login`（直接访问 `/login` 是无效路由）；切换账号时清 `localStorage`/`sessionStorage` 后访问受保护路由会自动跳转登录页。
- 浏览器窗口无法物理缩到 375px 宽（wmctrl 会被 WM 拉宽）；改用 CDP `Emulation.setDeviceMetricsOverride`（ws 连 /json 里的 page target），收尾必须 `Emulation.clearDeviceMetricsOverride`。
- 客服/权限相关表名：`shops`（不是 stores）、`admin_users`、`user_store_permissions`（列 `user_id/store_id/permission_scope`，scope 值 `view`/`operate`）、`customer_conversations.shop_id`。订单表是 `orders`（没有 sales_orders）。
- view-only 客服写端点应返回 403 + code `40303`；readonly 角色是 `40301`；跨租户模板 PUT/DELETE 是 404 + `40401`。
- `POST /customer/conversations/:id/send-platform-message` body 需 `{"reply","clientMessageId"}`（不是 content）；demo 会话无 external_conversation_id，会 400「会话缺少平台外部会话 ID」，不会真实外发。
- migrationimport 路由挂在 `/api/v1/imports/*`（parse/validate/commit），不是 `/api/v1/migration/imports`。
- 避免真实 AI 调用：可临时把 settings `ai.openai_compatible_base_url`（tenant 行）指向本机 mock server（容器内用 `http://172.18.0.1:<port>/v1` 访问宿主）；收尾必须清空该 setting 并 kill mock 进程。
- 验证链接 `target="_blank"` 时不要相信插桩后的自动化浏览器 DOM：Devin/agent 浏览器工具会从渲染后的 `<a>` 上剥离 `target=_blank`（rel 保留），导致假 FAIL。正确做法：检查生产 bundle（`docker exec <admin容器> grep -o 'target:"_blank"' /usr/share/nginx/html/p__*.js`）或用干净 Playwright Chromium（executablePath 指向 `~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome`，在 `collector/` 目录跑脚本以复用其 playwright 依赖）验证 DOM 与 popup 新标签行为。
