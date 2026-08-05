# R123 完整演示动线脚本（30 分钟 · Docker 全栈 + seed:demo:full）

> 面向老板验收/对外演示。与 [ACCEPTANCE_R123.md](ACCEPTANCE_R123.md) 配套：本脚本走一遍业务闭环「采集/选品 → 优化 → 草稿 → 货源 → 订单 → 审单 → 采购 → 入库 → 发货 → 消息 → 财务对账 → 报表」，覆盖三角色与移动模式。
> 历史 R1 阶段演示脚本见 [../DEMO_SCRIPT.md](../DEMO_SCRIPT.md)（已过时，保留存档）。

## 前置准备（演示前 10 分钟完成，不计入 30 分钟）

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build   # 首次构建约 5–10 分钟
DB_HOST=127.0.0.1 pnpm seed:demo:full                     # 全链路演示数据（DEMO- 前缀，幂等）
```

- Admin：<http://127.0.0.1:8000>；后端健康检查：<http://127.0.0.1:8080/health>
- 演示账号（seed 自动创建，同租户三角色）：

| 角色 | 账号 | 密码 | 用途 |
| --- | --- | --- | --- |
| 管理员 | `demo_admin@trademind.local` | `DemoAdmin123!` | 主演示动线 |
| 运营 | `demo_operator@trademind.local` | `DemoOperator123!` | 店铺 scope 演示 |
| 只读 | `demo_readonly@trademind.local` | `DemoReadonly123!` | 权限边界演示 |

- 演示结束清理：`DB_HOST=127.0.0.1 pnpm seed:demo:full:clean && DB_HOST=127.0.0.1 pnpm seed:demo:full:verify`（期望输出 `zero DEMO- residual rows`）。

## 演示动线（约 30 分钟）

以 `demo_admin` 登录开始。每步给出 入口路由 / 操作 / 预期结果。

### 第 1 段：采集 → 选品 → 优化 → 草稿（约 7 分钟）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 1 | 运营总览 | 登录后停留首页 `/dashboard` | 经营概览、待办与新手引导卡片加载 |
| 2 | 采集 | 「采集中心」`/collect/hub`，展示 1688/拼多多入口与采集任务列表 `/collect/tasks` | DEMO 采集任务留痕（成功/失败样本），登录风险提示可见 |
| 3 | 选品 | 「选品任务」进入 DEMO 任务详情 `/selection/tasks/<uuid>` | 候选清单带 AI 评分/预估利润 |
| 4 | 选品数据面（R120） | 候选行点「数据面板」抽屉；多选 2–3 行点「对比所选」 | 面板展示采集价格/销量留痕、同类目基准、价格走势图；对比抽屉可导出 CSV |
| 5 | AI 优化 | 商品草稿 `/product/drafts` 进任一 DEMO 草稿详情，展示 AI 标题/描述面板与违禁词合规面板 | AI 结果对比/应用/撤销入口可见；违禁词命中在 readiness「合规检测」高亮（未配 AI Key 时展示既有 DEMO 结果与显式提示，不阻演示） |
| 6 | 发布检查 | 草稿详情「发布检查」 | passed / warning / failed 三态与阻断原因中文展示 |

### 第 2 段：货源 → 刊登 →（降级边界说明）（约 4 分钟）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 7 | 货源 | 「货源与采购」→ 货源档案，展示 DEMO 商品主货源与 SKU 映射 | 进价/链接/库存参考齐全，「订单→采购」依据可解释 |
| 8 | 刊登 | 草稿详情刊登 Tab / 批量刊登向导创建本地草稿批次，`/product/publish-batches/:id` 查看 | TikTok/Shopee 等显示「仅生成本地草稿」；批次 success；**口径说明**：真实平台连通卡凭证（验收包 §二），闲鱼通道已真实验证 |

### 第 3 段：订单 → 审单 → 自动化 → 采购 → 入库 → 发货（约 9 分钟）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 9 | 订单 | 「订单列表」筛选 DEMO 订单，展示 SKU 匹配与批量操作 | 列表含审单状态、打单状态列 |
| 10 | 审单（R114） | 「审单工作台」与 `/settings/order-review-rules` | 待审/挂起样本与命中原因；采购/发货对未过审订单强制阻断 |
| 11 | 自动化规则（R119） | `/settings/order-automation-rules` 展示规则；回订单列表勾选 `DEMO-AT-1004`（unpaid、审单通过、SKU 已匹配）批量「标记已付款」 | 触发 `order_paid` 自动化：`/orders/automation-logs` 出现新执行记录，自动生成采购单成功（正向样本）；`DEMO-AT-1001` 同操作演示安全阻断负样本（无本地 SKU 匹配） |
| 12 | 自动化轨迹（R120） | 订单详情 `?tab=automation` 深链 | 成功/失败/跳过留痕 + 跳转全量日志 |
| 13 | 采购 | 侧栏「货源与采购」→ 采购单（含第 11 步新生成单），演示 提交/回填 1688 单号/标记付款/签收 流转 | 状态机流转 + 审计留痕；**口径说明**：1688 人工下单过渡模式 |
| 14 | 入库/多仓（R112） | 签收时选仓（默认仓预选）；`/inventory/warehouses` 展示仓库与调拨 | 分仓库存 Tag、按仓扣减、仓间调拨原子流水 |
| 15 | 发货/打单（R111） | 订单发货弹窗「按规则推荐物流商」；`/orders/print` 按面单模板打印预览 + 标记已打单 | 推荐可解释可覆盖；打印页明示「非电子面单」 |

### 第 4 段：消息 → 财务对账 → 报表（约 6 分钟）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 16 | 买家消息（R119） | `/customer/buyer-messages`：待发草稿 tab 编辑/标记已发送/忽略；节点规则 tab | 变量已按订单上下文填充，缺失变量警示；页顶降级说明（人工确认、绝不自动外发） |
| 17 | 客服与话术 | `/customer/reply-templates` 与会话详情「插入话术模板」 | 分组模板、变量自动填充、AI 建议回复入口 |
| 18 | 财务对账（R121） | `/orders/finance-payments` 登记回款；`/orders/finance-reconciliation` 对账工作台；`/orders/finance-report` 报表 | 已结清/少款/多款样本；实算 vs 估算毛利差异；店铺×月份汇总，CSV 可导出 |
| 19 | 三报表（R110） | `/orders/reports-profit`（另有采购/库存报表页） | 多币种本位币折算、缺进价/未折算显式提示 |

### 第 5 段：三角色 + 移动模式 + 治理面（约 4 分钟）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 20 | operator | 退出，登录 `demo_operator` | 仅授权店铺数据可见；无权限路由统一「暂无访问权限」语义页 |
| 21 | readonly | 登录 `demo_readonly`，查看任一写入口 | 写入口不渲染（隐藏），直接调写接口返回 403；读路径完整 |
| 22 | 移动模式（R113） | DevTools device toolbar 375px（zoom 100%）刷新 | 底部 5 tab（首页/订单/采购/库存/我的）；`/m/home` 指标卡与待办触屏动线；表格横向内滚不溢出 |
| 23 | 治理面收尾 | 管理员回登，快速过 `/ops/task-center/operation-tasks`（批量审批）、失败任务中心、操作日志 | 运营任务批量批准弹窗、失败深链、审计留痕 |

## 常见坑（演示前自查）

- seed 需在仓库根目录执行且 PostgreSQL 端口 5432 已映射（full 栈默认已映射）；宿主机执行必须带 `DB_HOST=127.0.0.1`。
- 注册演示依赖 SMTP；未配置时不要演示自助注册，或临时向 Redis 注入验证码（仅测试环境，见 `.agents/skills/demo-fullstack-walkthrough/SKILL.md`）。
- `/purchase/orders` 为别名重定向（R122 起），正式入口是侧栏「货源与采购」（`/procurement/orders`）。
- 移动模式检查用 100% zoom；375px 表格横向内滚属预期，不算根节点溢出。
- 容器跑的是构建时代码：切分支后需 `docker compose -f docker-compose.full.yml up -d --build` 重建。

## 实跑验证记录

- 2026-08-05（R123 线1）：本脚本在 Docker 全栈（main `02b6b086` 构建）+ `seed:demo:full` 逐步实跑全部 23 步（三角色 + 375px 移动模式，全程录屏）：步骤 1–19、22、23 与预期一致；步骤 11 正/负样本真实触发验证通过（DEMO-AT-1004 自动生成采购单成功、DEMO-AT-1001 安全阻断留痕）；步骤 20/21 文案口径按实际表现修正（「暂无访问权限」/ 写入口隐藏而非禁用）。
