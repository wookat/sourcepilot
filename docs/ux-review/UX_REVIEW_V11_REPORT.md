# UX 视觉复核 v11 报告（重点：v10 遗留回归 + R163–R168 view-only 新面走查）

- 复核日期：2026-08-08
- 复核角色：user-experience-officer（真实卖家/协作账号走查，Docker 全栈 demo 环境实测，录屏留档不入库）
- 基线：`origin/main`（R169 时点，#327/#332/#336 已合入）+ 本地按序叠加未合并 PR 分支：#335（审单整批 403 + view-only P2 收口，`devin/1786135000-r167-line1-review-batch-403`）、#337（40303 文案统一 + R168 巡检，`devin/1786139380-r168-line1-merge-cleanup`）；无代码冲突
- 环境：`docker compose -f docker-compose.full.yml up -d --build` + `DB_HOST=127.0.0.1 pnpm seed:demo:full`，Admin dev server 8001
- 视口：五档精确视口 1920×1080 / 1440×900 / 1024×768 / 768×900 / 375×812（headless Playwright `viewport` 精确设定）
- 角色：demo_admin / demo_operator / demo_readonly 三角色全量 + 临时 view-only persona（仅 view scope 店铺授权，走查后清理不入库）；`/ops/backups`、`/ops/restores` 平台级路由另用 bootstrap 平台管理员走查
- 硬指标矩阵：5 persona × 5 视口 × 74 路由（含平台管理员 2 平台级路由）共 1850 组合 headless 全扫，console error = 0、pageerror = 0、根节点横向溢出 = 0、NaN / Invalid Date / undefined 文本直出 = 0、403/500 请求噪音 = 0（每 persona×视口 独立登录规避 token 过期误报）

## 一、走查范围

### v10 遗留项逐项复核

| v10 编号 | 页面 | 问题 | v11 结果 |
| --- | --- | --- | --- |
| P1-1 | /settings/report-currency | 未保存修改经侧栏跳转静默丢弃（v10 已修复） | **无回退**。dirty 后侧栏跳转弹「离开当前页面？汇率设置有未保存的更改」，取消/放行均正常 |
| P2-1/P2-2 | /ops/backups、/ops/restores | 英文确认按钮 / raw ISO 时间（v10 已修复） | **无回退**。确认弹窗「创 建/取 消」中文、说明完整；时间列格式化（本轮空数据下由平台管理员实走确认无英文/ISO 直出） |
| P2-4 | /dashboard/screen | 趋势 tooltip raw ISO（v10 已修复） | **无回退**。tooltip 标题显示 `21:00` 本地小时格式，与坐标轴一致 |
| P2-5 | /system/operation-logs | 英文技术 key 动作（v10 已修复） | **无回退**。登录/创建 MCP 只读 token 等动作、资源、状态全中文 |
| P2-3（遗留） | /settings/mcp-tokens | 文档入口纯文本不可点击 | **维持遗留**。待产品确认文档挂载位置，本轮不改 |
| v9 P2-3（遗留） | /orders/finance-report CSV | 未配置汇率本位币列空 | **维持口径**。大屏/报表「未折算（不计入合计）：EUR 88.00」提示一致，无伪造折算 |

### 重点新面（R163–R168 交付）

| 面 | 路由 / 方式 | 结果 |
| --- | --- | --- |
| view-only 审单按钮预禁用 + tooltip「店铺无操作权限」（#335） | /orders/review（view-only persona 实走） | 通过：批量放行/批量拒绝灰置，tooltip 文案正确 |
| 审单整批 403（40303）定案口径（#335） | API 实测：view-only token POST `/api/v1/order-review/approve` 混批 3 单 | 通过：整批 `HTTP 403 {"code":40303,"message":"店铺无操作权限"}`，无部分生效 |
| 同步任务重试 403 toast（#335/#337） | /orders/sync-tasks（view-only 实走，确认弹窗后触发） | 通过：中文 toast「店铺无操作权限」，先确认后请求，无静默失败 |
| 删除店铺弹窗中文化（#335） | /shops/manage（实走至确认弹窗后取消） | 通过：中文标题/按钮，未执行真实删除 |
| 40303 用户可见文案统一「店铺无操作权限」（#337） | API + UI toast 抽验 | 通过：envelope message 与前端 tooltip/toast 一致 |
| view-only 演示点（验收包） | 临时 view-only persona 全路由 5 视口扫描 | 通过：0 console error / 0 溢出 / 无裸英文裸枚举，预期 40303 之外无 403 噪音 |
| UX v10 修复项（#327，已合入 main） | 见上表 v10 遗留项复核 | 通过，无回退 |

### 移动端与视觉现代感（375/768 重点目测）

- 375×812：/m/home 移动工作台卡片化布局、底部 tab 导航正常；订单列表/审单工作台/店铺管理/采集中心/客服会话/商品草稿/币种设置均为单列自适应，无横向滚动、无遮挡截断；买家名等敏感字段脱敏显示。
- 768×900：侧栏自动收起为图标栏，表格横向滚动容器内滚动（根节点无溢出），筛选区两列自适应。
- 视觉现代感：统一圆角卡片 + 淡色背景 + 语义色 tag（平台/状态），空态插画与引导文案齐全（新手入门进度、AI 未配置引导卡），达标。

## 二、问题清单与处置

### P1

本轮未发现 P1 问题。

### P2

本轮未发现新增 P2 问题。既有遗留维持：

| 编号 | 页面 | 问题 | 建议 |
| --- | --- | --- | --- |
| v10 P2-3 | /settings/mcp-tokens | 「配置方法见 docs/mcp.md 与 docs/open-api.md」纯文本不可点击 | 待产品确认对外文档挂载位置后改链接 |
| v9 P2-3 | /orders/finance-report CSV | 未配置汇率本位币列空 | 维持「不伪造折算」口径，待产品确认 |

### 覆盖限制（非缺陷）

- 走查基于本地叠加 #335/#337 两个未合并 PR 的复核分支；若合并顺序变化，view-only 相关结论以合并后为准。
- 备份/恢复页为空数据集（demo 环境无备份产物），时间列格式化以 v10 修复代码 + 空态走查确认，未覆盖有数据行的实渲染。
- 色彩对比为人工目测（未跑 axe 全站扫描）。
- 临时 view-only persona 为本地 demo 库临时账号，走查后已清理，不进入 seed/代码。

## 三、修复清单（本轮 PR）

本轮无 P1/P2 代码修复，仅文档归档：

- `docs/ux-review/UX_REVIEW_V11_REPORT.md`：本报告。
- `docs/progress/R169.md`：轮次进展。
- `docs/PROGRESS.md`：变更记录。

验证：`pnpm check:dev`、`pnpm check:ui-copy --strict`、`pnpm test:frontend`、`pnpm test:contracts`、`pnpm build:admin` 通过（纯文档变更，后端 Go 门禁不适用）；Docker demo 环境实测录屏留档（证据不入库）。
