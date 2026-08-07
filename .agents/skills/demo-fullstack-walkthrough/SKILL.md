---
name: demo-fullstack-walkthrough
description: TradeMind Docker 全栈 demo 环境手工走查要点：seed 账号、关键路由、真实触发方法、清理与常见坑
---

# Docker 全栈 demo 手工走查

## 环境
- `docker compose -f docker-compose.full.yml up -d`：Admin http://127.0.0.1:8000，后端 :8080（容器跑的是构建时的分支代码）。
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
- 响应式检查用 DevTools device toolbar；默认 auto-fit 缩放（29%）截图不可读，把 zoom 改成 100%。硬指标用 `document.documentElement.scrollWidth <= clientWidth` 判断根节点横向溢出。
- 移动端（375）出现底部导航（首页/订单/采购/库存/我的），表格是横向内滚，不算根节点溢出。
- 登录 API 请求体字段为 `{"account","password"}`（不是 `email`），脚本化取 JWT 时注意。
- 大屏卡片配置弹窗（R156 #318）没有「恢复默认」按钮，演示后需手动改回启用/顺序再保存。
- 大屏销售额/毛利卡「已折算：X」与「未折算（不计入合计）…」两行互斥：存在未折算币种时只显未折算行；seed 多币种样本（EUR 无汇率）下看不到「已折算」行，属预期。
