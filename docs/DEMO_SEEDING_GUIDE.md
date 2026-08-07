# Demo 数据种子指南（Phase F7）

> **用途**：在本地或预发环境导入全链路演示数据，支撑 16 步 MVP 主链路走查。  
> **状态**：Post-F9 Enhancement · MVP Demo Ready · Tag deferred · 非 Production Ready · 抖店 Release Candidate

## 前置条件

1. PostgreSQL + Redis 已启动（`docker compose up -d` 或等价）
2. 后端 API 可访问（默认 `http://127.0.0.1:8080`）
3. 根目录 `.env` 含 `ADMIN_BOOTSTRAP_EMAIL` / `ADMIN_BOOTSTRAP_PASSWORD`
   - 在宿主机直跑 Go seed（`pnpm seed:demo:full` 等）而 `.env` 的 `DB_HOST` 写的是容器服务名 `postgres` 时，需临时覆盖 `DB_HOST=127.0.0.1`（PowerShell：`$env:DB_HOST="127.0.0.1"`），见 `docs/env.md`
4. （可选）AI Provider 已配置 — 客服 AI 建议样本为 best-effort

## 一键种子

```powershell
# 仓库根目录
.\scripts\seed-demo-data.ps1 -ApiBase http://127.0.0.1:8080 -OutFile docs/demo-dataset.json
.\scripts\seed-demo-permissions.ps1 -ApiBase http://127.0.0.1:8080
```

Linux / macOS：

```bash
./scripts/seed-demo-data.sh
./scripts/seed-demo-permissions.ps1   # 需 PowerShell
```

## 脚本行为

### seed-demo-data

1. 登录 bootstrap 管理员
2. 调用 `a1-prepare-samples.ps1` 补齐 20 商品 slot
3. 创建 **F2 订单**、**F3 库存**、**F4 客服** 样本
4. 探测 **F6/F7 Dashboard** KPI API
5. 汇总 AI / 刊登 / 失败任务 / 工作台待办
6. 写入：
   - `docs/demo-dataset.json`
   - `docs/demo-dataset.orders.json`
   - `docs/demo-dataset.inventory.json`
   - `docs/demo-dataset.customer.json`
   - `docs/demo-dataset.dashboard.json`
   - `docs/demo-dataset.full-project.json`

### seed-demo-permissions

创建 Demo 账号并写入 `docs/demo-dataset.permissions.json`：

| 账号 | 角色 | 用途 |
| --- | --- | --- |
| `demo_admin@trademind.local` | admin | 全权限演示 |
| `demo_operator@trademind.local` | operator | 店铺隔离演示 |
| `demo_readonly@trademind.local` | readonly | 只读阻断演示 |

默认密码见脚本输出或 `demo-dataset.permissions.json`（开发环境）。

> 口径统一：`pnpm seed:demo:full`（Go seeddemo，跨平台、无需 PowerShell）也会幂等保证以上三个账号存在且密码为 `DemoAdmin123!` / `DemoOperator123!` / `DemoReadonly123!`；若密码漂移会重置回文档值并使旧会话失效。仅限非 production。

### 第二演示租户（多租户隔离回归）

`pnpm seed:demo:full` 额外创建独立业务租户「DEMO-第二租户」，用于开箱验证双租户隔离：

| 账号 | 角色 | 用途 |
| --- | --- | --- |
| `demo_tenant2_admin@trademind.local` / `DemoTenant2Admin123!` | admin | 第二租户全权限，登录只见 `DEMO-T2-` 数据 |

数据：1 店铺（`DEMO-T2-SHOP-1`）、2 订单（`DEMO-T2-SO-*`）、1 发货规则、2 自动化规则（含仅推荐物流商 recommend 模式）、3 条自动化执行日志（`DEMO-T2-AT-*`，成功/失败/跳过各 1，成功样本覆盖「仅推荐，发货时人工确认」文案）。正向：第二租户账号可见自己的数据；负向：主租户账号看不到 `DEMO-T2-` 数据、第二租户账号看不到主租户数据。clean/verify 覆盖该租户及账号（零残留），重跑幂等。

主租户另有挂在手工渠道店（operator/readonly 授权店）的自动化执行日志样本 `DEMO-AT-1301`~`DEMO-AT-1303`（成功/跳过/失败各 1，成功样本为 recommend 仅推荐模式），保证 operator 视角执行日志页不空态。

### 买家消息多语言模板样本（round152）

`pnpm seed:demo:full` 的客服样本包含多语言话术模板与买家消息草稿语言口径样本：

- 话术模板（`DEMO-` 前缀）带英/西/葡语言变体（默认语言 `zh-CN`，变量占位符与默认正文一致）。
- 正样本：`DEMO-BM-1005`（收货地 US → `en`，`langSource=order_country`）、`DEMO-BM-1006`（收货地 BR → `pt`，`langSource=order_country`）。
- 负样本：`DEMO-BM-1001`（订单无收货地国家 → 回退默认语言 `zh-CN`，`langSource=fallback`）。
- clean/verify 覆盖 `customer_reply_template_variants`（零残留），重跑幂等。

## 验证

```powershell
# 读 validation 段
Get-Content docs/demo-dataset.json | ConvertFrom-Json | Select-Object -ExpandProperty validation
```

期望 `passed: true`（至少 20 slot、7 task samples、订单/库存/客服各 ≥3）。

专项 smoke（需 API 在线）：

```powershell
.\scripts\demo-dashboard-smoke.ps1
.\scripts\demo-order-inventory-customer-smoke.ps1
.\scripts\demo-rbac-smoke.ps1
```

## F8 dev-only edge-case 样本

在 **非 production** 环境，管理员可调用：

```http
POST /api/v1/dev/demo-seed/full-project-edge-cases
Authorization: Bearer <admin token>
```

写入（**不调用真实外部平台**）：

- 订单同步 `partial_success` + 页级错误
- 库存同步 `failed`（SKU 未绑定）
- 客服发送失败 + 失败任务中心记录（**故意构造的演示样例，非真实故障**：会话/消息/失败事件均带「演示样例·非真实故障」标注）
- 平台未授权店铺样本

操作写入 **operationlog**（`dev.demo_seed.full_project_edge_cases`）。

`seed-demo-data.ps1` 在 API 在线时会自动探测此接口。

## 注意事项

- **不写入真实平台数据**；抖店步骤预期 `blocked_by_real_credentials` 或 `local_draft_only`
- 重复运行会追加/更新样本，演示前可清空 dev 库或接受增量
- 商品标题含 `R1 demo` / `F3 demo` 等前缀便于检索

## 相关文档

- [`DEMO_DATASET.md`](DEMO_DATASET.md) — slot 与样本明细
- [`FULL_PROJECT_DEMO_DATASET.md`](FULL_PROJECT_DEMO_DATASET.md) — 全项目数据集索引
- [`DEMO_AUTO_ACCEPTANCE_GUIDE.md`](DEMO_AUTO_ACCEPTANCE_GUIDE.md) — 自动化回归
- [`../DEMO_CHECKLIST.md`](../DEMO_CHECKLIST.md) — 验收勾选
