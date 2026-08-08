---
name: mcp-write-acceptance
description: MCP 写白名单（write:ops / mark-paid 限额 / 审计过滤）三角色实跑验收的环境准备、探针技巧与清理口径
---

# MCP 写白名单实跑验收（补充 demo-fullstack-walkthrough / admin-e2e-testing）

适用：验证 `/settings/mcp-tokens` 写白名单卡片、mark-paid 限额表单、`mcp_tool_call_logs` 审计过滤与写工具拒绝路径。

## 环境前置

- Docker 全栈 `docker-compose.full.yml`；admin `http://127.0.0.1:8000`，backend `http://127.0.0.1:8080`。
- `.env` 需 `MCP_WRITE_ENABLED=true` 且 backend 镜像基于被测分支重建（容器跑构建时代码）。
- 租户级开关 `settings(group_key='mcp', item_key='write_enabled')` 需为 true（UI 写白名单卡片内开关）。

## 关键事实（易踩坑）

- 审计表 `mcp_tool_call_logs` 列名是 `tool` / `mode` / `status` / `params_summary` / `confirm_hash` / `amount`，**不是** `tool_name` / `call_mode`。
- 订单表 `orders` 支付列是 `payment_status`（值 `unpaid`），采购单表 `purchase_orders` 用 `pay_status`。
- seed 通常只有一张 `placed` 采购单（如 `DEMO-1688-2001`，64.50 CNY）。execute 成功后即变 paid，后续「超限/币种不符」拒绝路径会先撞「状态不允许」而非配额。复跑前：
  `UPDATE purchase_orders SET status='placed', pay_status='unpaid', paid_at=NULL WHERE external_order_id='DEMO-1688-2001';`
- 金额必须与采购单严格一致，否则拒绝原因是「金额或币种与采购单不一致」；要测日累计上限，请用真实金额并把日累计上限调到已用额度附近。

## view-only 临时账号构造

seed 无 view-only 账号。手工插入时 **`admin_users.id` 必须是合法 UUID 格式**（`char(36)` 但 GORM 按 UUID 扫描），否则登录只报 401「账号或密码不正确」，真实原因只在 backend 日志里（`Scan error ... invalid UUID format`）。

```sql
INSERT INTO admin_users (id, created_at, updated_at, tenant_id, username, email, password_hash, display_name, role, status, token_version, must_change_password)
SELECT '<合法UUID>', now(), now(), 1, 'tmp_viewonly', 'tmp_viewonly@trademind.local', password_hash, 'TMP-ViewOnly', 'operator', 'active', 1, false
FROM admin_users WHERE email='demo_operator@trademind.local';
INSERT INTO user_store_permissions (id, user_id, store_id, platform, permission_scope, created_at, updated_at)
VALUES ('<合法UUID>', '<上面的UUID>', '<demo_operator 授权的 store_id>', 'manual', 'view', now(), now());
```

密码沿用 demo_operator 的 hash（`DemoOperator123!`）。写操作（订单列表「批量标记已付款」）预期 toast「N 单标记失败：店铺无操作权限」且状态零变更。

## 角色断言速查

- admin：写白名单卡 + 单笔/日累计上限表单 + 「调用模式」筛选 + 模式/参数摘要/确认哈希/金额列全可见。
- operator / readonly：以上全部不渲染，审计仅剩读工具行（后端 SQL 过滤，不是前端隐藏）。
- 第二租户 admin：token 列表/写 token/审计卡片均空，业务数据仅本租户前缀。

## 清理（不跑 seed clean）

```sql
UPDATE mcp_api_tokens SET revoked_at=now(), updated_at=now() WHERE name IN (...临时 token...);
DELETE FROM user_store_permissions WHERE user_id='<临时账号UUID>';
DELETE FROM admin_users WHERE id='<临时账号UUID>';
DELETE FROM settings WHERE group_key='mcp';
UPDATE purchase_orders SET status='placed', pay_status='unpaid', paid_at=NULL WHERE external_order_id='DEMO-1688-2001';
```

`seed:demo:full:clean` 不覆盖自建账号、自建 mcp token 与 `mcp_tool_call_logs`，需手工处理。

## Devin Secrets Needed

无（全部为本地 demo seed 账号）。
