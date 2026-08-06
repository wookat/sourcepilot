# UX 视觉复核 v9 报告（重点：v8 遗留收口复核 + R130–R135 新面走查）

- 复核日期：2026-08-06
- 复核角色：user-experience-officer（真实卖家走查，Docker 全栈 demo 环境实测，录屏留档）
- 基线：main `7719cb3d`（R130–R135 之 #274/#275/#276/#277/#278/#282 均已合并）+ 本地叠加未合并 PR #281（R134 收口批次），无冲突
- 环境：`docker compose -f docker-compose.full.yml up -d --build` + `DB_HOST=127.0.0.1 pnpm seed:demo:full`
- 视口：五档抽查（受测试环境物理显示与 Chrome 最小窗口限制，实测宽度约 485 / 721 / 1083 / 1440 / 1600；移动断点行为已覆盖，精确 375 / 1920 无法达到，为覆盖限制而非产品问题）
- 角色：demo_admin 全量、demo_operator 实走、demo_readonly 门控抽查
- 硬指标：走查页面 console error = 0、pageerror = 0、根节点横向溢出 = 0、403/500 请求噪音 = 0

## 一、走查范围

### v8 历史 P2 逐项复核

| v8 编号 | 页面 | 问题 | v9 结果 |
| --- | --- | --- | --- |
| P2-A | /orders/automation-logs | operator/第二租户视角空态（seed 范围） | **收口**。operator 视角执行日志非空且按店铺范围隔离 |
| P2-B | /system/operation-logs | 操作类型列英文技术 key | **收口**（#275）。既有操作类型中文化生效；#282 新增的标签动作 key 见本轮 P2-1（本轮已顺手修复） |

另复核 v7 遗留 `/purchase/orders` → `/procurement/orders` 重定向、375 档执行日志行高/ellipsis：均无回退。

### 重点新面（R130–R135 交付）

| 面 | 路由 | 结果 |
| --- | --- | --- |
| 订单标签全动线（#282）：标签管理 CRUD/颜色/长标签、列表标签列与筛选、手工打标/去标、批量打标、自动化「自动打标签」动作 | /settings/order-tags、/orders、/settings/order-automation-rules | 通过 |
| 自动化正向触发：DEMO-AT-1004 标记已付款 → 自动打标签 + 自动分仓 + 应用发货规则 + 生成采购单 | /orders、`?tab=automation` | 通过 |
| 对账/毛利 CSV 全量导出交互（#277）：loading 态、防重复点击、成功提示、UTF-8 BOM、中文表头、csvsafe 防注入 | /finance/reconciliation、/finance/report | 通过（「平台」列英文枚举见 P1-1，已修复） |
| 财务聚合下推数字渲染（#276）：无 NaN/异常 | /finance/* | 通过 |
| 执行日志/操作日志中文化一致性（#275） | /orders/automation-logs、/system/operation-logs | 通过（标签动作 key 见 P2-1，已修复） |
| readonly 门控与来源平台中文化（#281）：草稿详情只读提示、写控件禁用、采集规则非法 JSON 中文报错 | /product/drafts、/collect/rules、/settings/order-tags | 通过 |

### 全站抽查

- 三角色 × 多视口探索式抽查（非穷举矩阵）；走查页面硬指标全零。
- 移动端底部导航触屏目标 56px、其余 ≥40px，触达性达标。
- 长标签溢出 ellipsis 正常、标签颜色对比度可读。

## 二、问题清单与处置

### P1（本轮已修复）

| 编号 | 页面 | 问题 | 修复 |
| --- | --- | --- | --- |
| P1-1 | /finance/reconciliation 导出 CSV | 「平台」列输出英文技术枚举（`douyin_shop`/`manual`/`tiktok`） | 后端 `ExportReconciliationCSV` 复用 `opslabels.PlatformLabel` 输出中文平台名（抖店/手动/TikTok Shop），补回归测试 |

### P2（低成本项本轮顺手修复）

| 编号 | 页面 | 问题 | 处置 |
| --- | --- | --- | --- |
| P2-1 | /system/operation-logs | #282 标签操作显示英文 key（`订单 · tag · attach` 等） | 已修复：补 `order_tag` 资源与 create/update/delete/attach/detach/batch 动作中文映射 + 单测 |
| P2-2 | /settings/order-tags | 创建时间英文 locale（`8/6/2026, 7:10:08 AM`） | 已修复：改用统一 `formatDateTime` |
| P2-3 | /finance/report 导出 CSV | TikTok 店行应收/已回款（本位币）列为空 | 待产品确认：疑似非 CNY 未配置汇率时不伪造折算的预期口径（与页面「未折算」提示一致）；如确认预期，建议 CSV 中输出「未折算」占位而非空白 |

### 覆盖限制（非缺陷）

- demo 数据单页即可容纳，「全量导出 > 当前页」未能用数据差异实证（代码路径 #277 已复核为全量查询）。
- `/orders/finance-payments` 无导出按钮，#277 范围是否含该页待产品确认。
- 精确 375 / 1920 视口受测试环境窗口限制未达（断点行为已验证）。

## 三、回归结论

- v8 遗留 P2 两项全部收口，无回退；v7 遗留抽查无回退。
- 本轮发现 P1×1、P2×3：P1-1、P2-1、P2-2 已随本报告 PR 修复，P2-3 待产品确认。
- 走查页面硬指标全零：console error=0、pageerror=0、根节点横向溢出=0、403/500 噪音=0。
- **#281（本地叠加）readonly 门控、来源平台中文化、采集规则中文报错验证通过；#282 订单标签全动线验证通过。**
