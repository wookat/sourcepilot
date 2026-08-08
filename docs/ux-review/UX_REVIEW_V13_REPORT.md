# UX 视觉复核 v13 报告（重点：v12 基线回退确认 + R177–R184 MCP 治理/安全新面走查）

- 复核日期：2026-08-08
- 复核角色：ux-designer + qa-engineer（Docker 全栈 demo 环境实测，录屏/截图留档不入库）
- 基线：复核时点 `origin/main`（#368 合入后 `135f2c5e`）+ 本地按序叠加当时未合并 PR（经 `git_view_pr` 确认均 OPEN、mergeable）：#369（R184 P2 安全收口）、#370（R184 line2 升级演练）；仅 `docs/PROGRESS.md` 文档冲突。复核完成后 #369/#370 已合入 main（`f9695c86`），本报告随 PR 基于合入后的 main 提交，叠加内容与合入结果一致
- 环境：`docker compose -f docker-compose.full.yml up -d --build` + `DB_HOST=127.0.0.1 pnpm seed:demo:full`，Admin dev server 8001
- 视口：五档精确视口 1920×1080 / 1440×900 / 1024×768 / 768×900 / 375×812（headless Playwright `viewport` 精确设定）
- 角色：demo_admin / demo_operator / demo_readonly 三角色 + 临时 view-only persona（operator 角色 + 仅 view scope 店铺授权，走查后清理不入库）+ bootstrap 平台管理员（tenant0）
- 硬指标矩阵：5 persona × 5 视口 × 102 路由（含真实 seed ID 详情路由）headless 全扫：console error = 0（预期权限/跨租户 404 网络日志除外，同 v12 口径）、pageerror = 0、根节点横向溢出 = 0、NaN / Invalid Date / undefined 文本直出 = 0、redirect-login = 0；本轮新增裸枚举/裸英文检测项，发现 6 处（全部即修，见下）

## 一、走查范围

### v12 结论零回退抽验

| v12 项 | 页面 | v13 结果 |
| --- | --- | --- |
| v12 P2-1 会话详情 375 视口 Descriptions span | /customer/conversations/:id | **无回退**。375 视口 0 console error、0 溢出 |
| 五档视口根节点溢出 = 0 | 全站矩阵 | **无回退**。5 persona × 5 视口全扫 0 溢出 |
| NaN / Invalid Date / undefined 直出 = 0 | 全站矩阵 | **无回退** |
| 权限外详情路由优雅空态 | readonly / 跨租户 | **无回退**。仍为空态 + 预期 404 网络日志，无技术信息泄露 |
| v10 P2-3 mcp-tokens 文档入口 | /settings/mcp-tokens | **已闭环**（站内自托管 docs/mcp.md、docs/open-api.md 可点击链接，回归测试在库） |
| v9 P2-3 finance-report CSV 未折算列 | 遗留 | **维持口径**（不伪造折算） |

### 重点新面（R177–R184 交付：MCP 治理与安全）

| 面 | 路由 / 方式 | 结果 |
| --- | --- | --- |
| 写开关风险确认 | /settings/mcp-tokens（admin 实走） | 通过：开启前弹「确认开启本租户的 MCP 写白名单？」风险确认，未确认不落库（saveSettings 未调用）；关闭直接生效 |
| 写 token 管理 admin-only | admin / operator / readonly 三视角实走 | 通过：写白名单卡片（写开关 + 写 token 列表 + 创建入口）仅 admin 可见；operator / readonly 完全不可见且不拉取写开关设置 |
| 审计列表 mode / 金额列 / 筛选 | /settings/mcp-tokens 审计卡片 | 通过：admin 视角有「模式」（dry_run 预览 / execute 执行 Tag）、「参数摘要」「确认哈希」「金额（仅支付登记）」列与「调用模式」筛选 |
| 权限收紧后 readonly / operator 视图（R184） | 同页非管理员视角 | 通过：无 mode / 参数摘要 / 确认哈希 / 金额列，无调用模式筛选，写审计行 SQL 级隐藏（服务端 `settings.manage` 门） |
| settings 敏感 key 脱敏回显 | /settings/ai 等 | 通过：已存密钥回显为掩码（`****` 形态），DOM / input 值无 `sk-` 明文泄露 |
| mark-paid 限额配置入口 | /settings/mcp-tokens | **发现缺失，本轮已补**（P2-7，见下）：后端 `procurement_mark_paid` 强制要求租户设置 `mcp/mark_paid_single_limit` 与 `mcp/mark_paid_daily_limit` 为正数才可用（fail closed），但 Admin UI 无任何配置入口，管理员只能直调 settings API。已在写白名单卡片内新增 admin-only「mark-paid 金额限额」表单（单笔上限 / 日累计上限，回显 + 校验 + 保存），先红后绿补回归测试 |

### 移动端与视觉（375/768 重点目测）

- 375×812：/settings/mcp-tokens 含新增限额表单无横向溢出，写开关/表格可用；全站矩阵 375 档 0 溢出。
- 768×900：侧栏收起，表格容器内横向滚动，根节点无溢出。

## 二、问题清单与处置

### P1

本轮未发现 P1 问题（无 pageerror、无功能不可用、无权限泄露）。

### P2（本轮即修，含硬指标新增的裸枚举/裸英文检测项）

| 编号 | 页面 | 问题 | 处置 |
| --- | --- | --- | --- |
| v13 P2-1 | /settings/mcp-tokens | mark-paid 限额无 UI 配置入口（后端 fail closed 依赖该配置） | **已修**：写白名单卡片新增 admin-only 限额表单，保存 `mcp/mark_paid_single_limit`、`mcp/mark_paid_daily_limit`；先红后绿补 `McpTokens.test.tsx` 回归 |
| v13 P2-2 | /ops/observability | 告警表级别（critical/warning）、状态（firing）、模块（http/security）裸枚举直出 | **已修**：中文 Tag / 标签映射 |
| v13 P2-3 | /ops/workers/monitor | worker 类型 `task_alert_scan` / `security_secret_reencrypt` / `file_security_scan` 未映射直出 | **已修**：补 `TASK_CENTER_TASK_TYPE_LABEL` 中文名 |
| v13 P2-4 | /orders/sku-matches | 匹配状态 valueEnum 英文直出（matched/unmatched/…）、匹配类型 `local_sku_code` 等裸枚举 | **已修**：中文 valueEnum + `MATCH_TYPE_LABEL` 映射 |
| v13 P2-5 | /settings/config-status、/settings/security | 状态枚举 `ready_with_warning` / `manual_required` / `blocked_by_contract_verification` 与认证模式 `legacy_local_storage` 裸枚举直出 | **已修**：`COMMON_STATUS_LABEL` 增补 + 页面状态映射 + 认证模式中文标签 |
| v13 P2-6 | /orders/reports-profit | 维度行店铺平台后缀 `（douyin_shop）` 裸枚举 | **已修**：`platformLabel` 映射为「抖店」 |

### 既有遗留 / 接受为技术标识（非缺陷）

| 项 | 页面 | 说明 |
| --- | --- | --- |
| 采集器 source key 副标题 | /collect/hub | `taobao_tmall` / `shein_temu` 为采集源技术标识（卡片副标题、灰字），中文名称在主标题，维持 |
| 告警规则 ID 与英文摘要 | /ops/observability | `http_5xx_elevated` 等为规则技术 ID（规则列语义即 ID）；摘要为后端告警事件原文（demo 数据为英文），后续如需中文化应在告警规则定义侧解决 |
| Prompt 模板编号 | /ai/prompts | `product_title_optimize` 等为模板 code（表单明确「英文标识」），维持 |
| 密钥重加密审计表字段列 | /settings/security | `settings` / `item_value` 为数据表/字段名（重加密对象定位语义），维持 |
| v9 P2-3 finance-report CSV | /orders/finance-report | 维持「不伪造折算」口径 |

### 覆盖限制（非缺陷）

- 详情路由使用 tenant 1 seed ID；operator/readonly 未授权与 bootstrap（tenant0）跨租户访问时产生预期 404 网络日志（权限不泄露设计），页面优雅空态，不计缺陷（同 v12 口径）。
- 走查基于本地叠加 #369/#370 的复核分支；若上游合并顺序变化，结论以合并后为准。
- 色彩对比为人工目测（未跑 axe 全站扫描）。
- 临时 view-only persona 为本地 demo 库临时账号，走查后已清理，不进入 seed/代码。
- 浏览器走查全程拦截非 GET 写请求（登录除外），未执行真实写操作。

## 三、修复清单（本轮 PR）

- `admin/src/pages/Settings/McpTokens.tsx`：v13 P2-1 mark-paid 限额配置入口。
- `admin/src/pages/Settings/__tests__/McpTokens.test.tsx`：限额入口回归测试（先红后绿）。
- `admin/src/pages/Ops/Observability/index.tsx`：v13 P2-2。
- `admin/src/constants/taskCenter.ts`：v13 P2-3。
- `admin/src/pages/Orders/SkuMatches/index.tsx`：v13 P2-4。
- `admin/src/pages/Settings/ConfigStatus/index.tsx`、`admin/src/pages/Settings/Security/index.tsx`、`admin/src/constants/copywriting.ts`：v13 P2-5。
- `admin/src/pages/Reports/Profit/index.tsx`：v13 P2-6。
- `docs/ux-review/UX_REVIEW_V13_REPORT.md`：本报告。
- `docs/progress/R185-line2.md`：轮次进展。
- `docs/PROGRESS.md`：变更记录。

验证：`pnpm check:ui-copy --strict`、`pnpm test:frontend`（375 通过，含新增回归）、`pnpm test:contracts`（17 通过）、`pnpm build:admin` 全通过；修复后 demo_admin 全视口复扫，P2-2～P2-6 页面裸枚举清零；Docker demo 环境实测截图留档（证据不入库）。Actions CI 不作依据。
