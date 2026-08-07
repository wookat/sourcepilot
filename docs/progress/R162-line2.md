# R162 线2：全站视觉/UX 复核 v10（user-experience-officer / ui-designer）

日期：2026-08-07。距 v9（R136 #284）25 轮的全站视觉/UX 复核。基线：main（R162 时点）+ 本地按序叠加未合并 PR #318/#321/#322/#323/#324/#325（`docs/PROGRESS.md` keep-both 处理，无代码冲突）。完整报告：`docs/ux-review/UX_REVIEW_V10_REPORT.md`。

## 1. 走查与硬指标

- Docker 全栈（`docker-compose.full.yml`）+ `seed:demo:full`，三角色真实 GUI 走查录屏留证（证据不入库）。
- headless Playwright 硬指标矩阵：3 角色 × 5 精确视口（1920/1440/1024/768/375）× 29 路由，console error / pageerror / 根节点横向溢出 / NaN / Invalid Date / undefined 直出 / 403/500 噪音全零（租户角色访问平台级 `/ops/*` 的 40302 属预期门控）。v9 的「精确 375/1920 未达」覆盖限制本轮收口。
- v9 遗留复核无回退；R137–R161 新面（备份 Ops、MCP token、实时大屏、开放 API 入口、多语言模板、币种设置、标签/自动化、财务对账）全走查。

## 2. 修复（本轮 PR，P0=0 / P1=1 / P2 顺手修 4）

- **P1**：`/settings/report-currency` dirty 时路由跳转静默丢弃修改 → 新增共享 `useUnsavedChangesGuard`（history.block + beforeunload）并接入。
- P2：`/ops/backups` 确认弹窗英文按钮、`/ops/backups` `/ops/restores` 创建时间 raw ISO、大屏趋势 tooltip raw ISO、操作日志买家消息草稿/规则/回复模板/大屏配置/财务费用动作英文 key（补 `dashboard` 资源 + 18 动作映射）。
- 沉淀：`.agents/skills/demo-fullstack-walkthrough/SKILL.md` 补平台级路由账号、MCP 审计表名、导出防重复验证法等常见坑。

## 3. P2 遗留清单

- `/settings/mcp-tokens` 文档入口纯文本不可点击（待产品确认文档挂载位置）。
- v9 P2-3 财务 report CSV TikTok 行本位币空列口径（维持不伪造折算，待产品确认是否输出「未折算」占位）。

## 4. 验证

- `pnpm exec tsc --noEmit`、`pnpm test:frontend`（355 通过）、`pnpm build:admin`、`pnpm check:ui-copy --strict` 通过；Actions CI 不作验收依据。
- 本轮未改后端 Go 代码，后端测试门禁不适用。
