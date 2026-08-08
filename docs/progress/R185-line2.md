# R185 线2：全站视觉/UX 复核 v13（ux-designer + qa-engineer）

- 日期：2026-08-08
- 角色：ux-designer + qa-engineer
- 距 v12（R176）：8 轮，期间 MCP 写白名单 W1–W3、治理 UI、R184 安全收口/升级演练交付

## 口径

- **合并状态权威核实**（`git_view_pr`）：#369（R184 P2 安全收口）、#370（R184 line2 升级演练）均 **OPEN、mergeable**，未合入。本轮以 `origin/main`（#368 合入后 `135f2c5e`）本地按序叠加两分支为复核基线；仅 `docs/PROGRESS.md` 文档冲突，已保留双方条目解决。
- Docker 全栈 demo（`docker-compose.full.yml` + `seed:demo:full`）+ Admin dev server 8001 实测；浏览器走查全程拦截非 GET 写请求；截图留档不入库；Actions CI 不作依据。

## 1. 硬指标矩阵

- 5 persona（demo_admin / demo_operator / demo_readonly / 临时 view-only / bootstrap）× 5 视口（1920/1440/1024/768/375）× 102 路由 headless 全扫。
- console error = 0（预期权限/跨租户 404 网络日志除外，同 v12 口径）、pageerror = 0、根节点横向溢出 = 0、NaN / Invalid Date / undefined = 0、redirect-login = 0。
- v12 基线**无回退**；v10 P2-3（mcp-tokens 文档入口）已闭环为站内可点击链接。

## 2. 重点新面结论（MCP 治理/安全）

- 写开关风险确认、写 token 管理 admin-only、审计 mode/参数摘要/确认哈希/金额列与调用模式筛选、R184 权限收紧后 operator/readonly 最小暴露视图：全部通过（三角色实走）。
- settings 敏感 key 脱敏回显：通过（掩码回显，无明文泄露）。
- **mark-paid 限额配置入口缺失（v13 P2-1，已修）**：后端 `procurement_mark_paid` fail closed 依赖租户设置 `mcp/mark_paid_single_limit`、`mcp/mark_paid_daily_limit`，但 Admin UI 无配置入口。已在写白名单卡片（admin-only）内新增限额表单（回显/校验/保存），先红后绿补回归测试。

## 3. P2 即修清单

本轮硬指标新增裸枚举/裸英文检测，发现并全部即修 6 处（详见 `docs/ux-review/UX_REVIEW_V13_REPORT.md`）：

1. mark-paid 限额入口（McpTokens）；
2. /ops/observability 告警级别/状态/模块中文化；
3. /ops/workers/monitor 三个安全类 worker 类型中文名；
4. /orders/sku-matches 匹配状态/匹配类型中文映射；
5. /settings/config-status、/settings/security 状态枚举与认证模式中文化；
6. /orders/reports-profit 店铺平台后缀 platformLabel。

接受为技术标识（维持）：collect hub 采集源 key 副标题、observability 规则 ID、AI Prompt 模板 code、密钥重加密审计的表/字段名列。

## 4. 验证

`pnpm check:ui-copy --strict`、`pnpm test:frontend`（375 通过，含新增回归）、`pnpm test:contracts`（17 通过）、`pnpm build:admin` 全通过；修复后 demo_admin 全视口复扫相关页面裸枚举清零。未改后端 Go 代码。

## 5. 遗留与建议

- P1：无。
- 遗留：v9 P2-3 finance-report CSV 未折算口径维持；observability 告警摘要为后端事件原文（demo 数据英文），如需中文化建议在告警规则定义侧解决。
- 临时 view-only 账号走查后已清理。
