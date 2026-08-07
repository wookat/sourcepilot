# UX 视觉复核 v10 报告（重点：v9 遗留收口复核 + R137–R161 新面走查）

- 复核日期：2026-08-07
- 复核角色：user-experience-officer / ui-designer（真实卖家走查，Docker 全栈 demo 环境实测，录屏留档）
- 基线：main（R162 时点，#284 v9 报告已归档）+ 本地按序叠加未合并 PR：#318（大屏卡片配置）、#321/#322（R159 线2 升级演练、安全审计）、#323（R160 审计 P2）、#324（R161 竞品对标文档）、#325（R161 收口批次）；`docs/PROGRESS.md` 冲突按 keep-both 处理，无代码冲突
- 环境：`docker compose -f docker-compose.full.yml up -d --build` + `DB_HOST=127.0.0.1 pnpm seed:demo:full`
- 视口：五档精确视口 1920×1080 / 1440×900 / 1024×768 / 768×900 / 375×812（headless Playwright `setViewportSize`，v9 的物理窗口限制本轮已解除）
- 角色：demo_admin 全量、demo_operator 实走、demo_readonly 门控抽查；`/ops/backups`、`/ops/restores` 为平台级路由，另用 bootstrap 平台管理员走查
- 硬指标矩阵：3 角色 × 5 视口 × 29 路由 headless 全扫，console error = 0、pageerror = 0、根节点横向溢出 = 0、NaN / Invalid Date / undefined 文本直出 = 0、403/500 请求噪音 = 0（租户角色访问平台级路由的 40302 属预期门控）

## 一、走查范围

### v9 遗留项逐项复核

| v9 编号 | 页面 | 问题 | v10 结果 |
| --- | --- | --- | --- |
| P2-3 | /orders/finance-report 导出 CSV | TikTok 店行应收/已回款（本位币）列为空，待产品确认 | **维持既定口径**。页面与 CSV 均按「未配置汇率不伪造折算」处理，页面有「未折算」提示，无回退；产品确认仍开放 |
| 覆盖限制 | 全站 | 精确 375/1920 未达 | **收口**。本轮 headless 精确五档视口全量扫描通过 |

另复核 v8/v9 已收口项（财务 CSV 平台中文枚举、操作日志标签动作中文化、订单标签时间格式）：均无回退。

### 重点新面（R137–R161 交付）

| 面 | 路由 | 结果 |
| --- | --- | --- |
| 备份 S3 上传/定时器 Ops 页：上传状态/目标列、校验门控、恢复验证 | /ops/backups、/ops/restores | 通过（确认弹窗英文按钮与 raw ISO 时间见 P2-1/P2-2，已修复） |
| MCP token 管理：purpose 用途、过期档位/即将过期标记、审计卡筛选/刷新 | /settings/mcp-tokens | 通过（文档入口不可点击见 P2-3） |
| 实时经营大屏：折算角标、EUR 未折算行、自定义指标卡配置保存/还原、operator 无配置按钮 | /dashboard/screen | 通过（趋势 tooltip raw ISO 见 P2-4，已修复） |
| 开放 API 文档入口与 token purpose 区分 | /settings/mcp-tokens | 通过 |
| 多语言回复模板管理与买家消息应用模板 | /customer/reply-templates、/customer/buyer-messages | 通过 |
| 草稿工作台切换、readonly 只读 banner/禁用控件 | /product/drafts | 通过 |
| 币种设置未保存提示 | /settings/report-currency | **P1-1**：内联提示有，但侧栏跳转无确认、静默丢弃修改（本轮已修复） |
| 标签/自动化新动作中文化 | /settings/order-tags、/settings/order-automation-rules | 通过 |
| 财务对账：数字渲染、导出 loading/防重复点击（仅 1 个 CSV） | /orders/finance-reconciliation | 通过 |

## 二、问题清单与处置

### P1（本轮已修复）

| 编号 | 页面 | 问题 | 修复 |
| --- | --- | --- | --- |
| P1-1 | /settings/report-currency | 表单 dirty 时仅有内联「有未保存的更改」文案，经侧栏路由跳转无任何确认，修改被静默丢弃 | 新增共享 `useUnsavedChangesGuard` hook（history.block 路由拦截 + beforeunload），dirty 时弹「离开当前页面？」确认后才放行 |

### P2（低成本项本轮顺手修复）

| 编号 | 页面 | 问题 | 处置 |
| --- | --- | --- | --- |
| P2-1 | /ops/backups | 「创建备份记录」确认弹窗按钮为英文 `Cancel/OK` | 已修复：okText「创建」/cancelText「取消」 |
| P2-2 | /ops/backups、/ops/restores | 创建时间列 raw ISO 直出（`2026-08-07T14:11:40.600209Z`） | 已修复：统一 `formatDateTime` |
| P2-4 | /dashboard/screen | 订单趋势图 tooltip 标题 raw ISO 时间 | 已修复：tooltip title 复用 `formatHourTick`（与坐标轴一致） |
| P2-5 | /system/operation-logs | 买家消息草稿/规则、客服回复模板、大屏配置、财务费用等新动作显示英文技术 key（如 `客服 · buyer · 消息 · draft · regenerate`、`dashboard · screen · config · 更新`） | 已修复：补 `dashboard` 资源与 18 个动作中文映射 |

### P2（遗留清单，待产品/后续轮次）

| 编号 | 页面 | 问题 | 建议 |
| --- | --- | --- | --- |
| P2-3 | /settings/mcp-tokens | 「配置方法见 docs/mcp.md 与 docs/open-api.md」为纯文本不可点击，站内无可跳转文档地址 | 待产品确认对外文档挂载位置（开源仓库 URL 或站内文档路由）后再改为链接，避免硬编码仓库地址 |
| v9 P2-3 | /orders/finance-report 导出 CSV | TikTok 店行本位币列为空（未配置汇率不伪造折算） | 维持口径待产品确认；如确认，CSV 建议输出「未折算」占位 |

### 覆盖限制（非缺陷）

- 走查基于本地叠加 6 个未合并 PR 的复核分支；若后续 PR 合并顺序变化，大屏卡片配置（#318）相关结论需以合并后为准。
- 色彩对比抽查基于人工目测 + 主题 token（未跑 axe 全站扫描）；暗色主题大屏对比度达标。

## 三、修复清单（本轮 PR）

- `admin/src/hooks/useUnsavedChangesGuard.ts`：新增共享未保存离开确认 hook（P1-1）。
- `admin/src/pages/Settings/ReportCurrency/index.tsx`：接入 hook（P1-1）。
- `admin/src/pages/Ops/Backups/index.tsx`：确认弹窗中文按钮、创建时间 `formatDateTime`（P2-1/P2-2）。
- `admin/src/pages/Ops/Restores/index.tsx`：创建时间 `formatDateTime`（P2-2）。
- `admin/src/pages/Dashboard/Screen/index.tsx`：趋势 tooltip 标题格式化（P2-4）。
- `admin/src/constants/operationLogs.ts`：补资源/动作中文映射（P2-5）。
- `.agents/skills/demo-fullstack-walkthrough/SKILL.md`：沉淀本轮走查经验（平台级路由账号、MCP 审计表名、导出防重复验证法等）。

验证：`pnpm exec tsc --noEmit`、`pnpm test:frontend`（355 通过）、`pnpm build:admin`、`pnpm check:ui-copy --strict` 通过；Docker demo 环境实测录屏留档（证据不入库）。
