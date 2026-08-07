# R167 线2：竞品矩阵前哨确认 + 验收包 R163–R166 增量 + Docker 三角色（含 view-only persona）实跑

- 日期：2026-08-07
- 角色：fullstack-engineer（company-os）
- 分支：`docs/round167-line2-acceptance-delta`（基于最新 main `d645ec96`，#330 已合入）
- 背景：R163–R166 为安全修复与回归密集期（view-only 权限体系收口 #322/#330/#331、六处 P1 修复、UX v10 #327、生产演练复检 #329、大回归 v29），本轮确认竞品矩阵关键能力在安全修复期后无回退，并把 R163–R166 增量整理进验收包。

## 结论（SOP-04）

**通过，无回退。** R161 v8 竞品矩阵 16 项中抽验 6 项关键能力（订单管理 / MCP / 开放 API / 实时大屏 / 多语言消息 / 权限体系），API 层探针 + Docker 全栈三角色（含 view-only persona）UI 实跑全部 PASS，**未发现安全修复导致的功能回退，无 P1**。验收包（ACCEPTANCE_R123 §一/18、§三、§五 + DEMO_SCRIPT 第 21b 步）已补 R163–R166 增量；`pnpm check:ui-copy --strict` 通过；实跑全程录屏留证外置不入库。

## 一、竞品矩阵前哨抽验（6 项，基于 R161 v8 基线）

构建：最新 main（`d645ec96`，含 #322/#327/#328/#329/#330/#331）`docker-compose.full.yml` 全栈重建 + `DB_HOST=127.0.0.1 pnpm seed:demo:full`。证据分级：【实测】= 本轮 API 探针 / UI 实跑直接证据；【文档】= 既有报告；【推断】= 未直接实测。

| # | 矩阵项 | 抽验内容与结果 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| 1 | 订单管理 | 订单列表 38 单分页正常；自动化日志 9 条 success/failed/skipped 三态齐全；DEMO-AT-1004 标记已付款真实触发 5 条自动化轨迹（采购单/发货规则/分仓/标签）闭环 | 【实测】API + UI | 无回退 |
| 2 | MCP | mcp 用途 token 创建 → `initialize` → `tools/list`（4 只读工具：orders_query/inventory_query/exceptions_pending/report_summary）→ `tools/call orders_query` 返回 DEMO 订单且客户名脱敏（`D**`）；`mcp_tool_call_logs` 逐次审计 +1 | 【实测】API | 无回退 |
| 3 | 开放 API | openapi 用途 token `GET /api/open/v1/orders` 200；`page=0` → 400；mcp 用途 token 调开放入口 → 401（purpose 隔离）；非法 token → 401 | 【实测】API | 无回退 |
| 4 | 实时经营大屏 | `/dashboard/screen` KPI 有值（38 单 / salesBase 7,335.56 含 USD 折算）、「未折算（不计入合计）：EUR 88.00」显式列出；readonly PUT config → 403 | 【实测】API + UI | 无回退 |
| 5 | 多语言消息模板 | 买家消息草稿 8 条：en / pt（order_country 推断）+ zh-CN fallback（UI 显示「中文（简体）无法推断，已回退默认语言」） | 【实测】API + UI | 无回退 |
| 6 | 权限体系 | view-only persona（operator 角色 + 店铺 scope=view）：读订单 200 / 会话详情 `canWrite=false`；写探针全部 403+40303 且零落库——PUT 订单（#322 面）、会话 mark-replied（#330 面）、审单 approve / sync-orders / 店铺删除（#331 面）；readonly 写 → 403+40301；operator 正向写不受过度收紧（DEMO-AT-1005 批量标记已付款成功） | 【实测】API + UI | 无回退（安全修复未过度收紧） |

其余 10 项（采集、AI 优化、刊登、采购/多仓、财务对账、备份治理等）本轮未逐项复测，以 R161 v8 报告与大回归 v29（`integration/r166-regression-v29` 分支）结论为准【文档】；竞品总结论维持 R161 v8 口径：超越 4 / 达到 12 / 落后 0【文档】。

## 二、验收包 R163–R166 增量

- `docs/acceptance/ACCEPTANCE_R123.md`：新增 §一/18「R163–R166 增量能力」——view-only 权限体系收口（#322/#330/#331 已合入 main；**#332 全站扫尾仍 OPEN 且与 main 冲突**，按 v29 定案解法（审单整批 403、保留 main 侧 helper）合入；**#333 仍 OPEN**，依赖 #332）、演练复检（#329）、大回归 v29（报告在 `integration/r166-regression-v29` 分支，随 #332 合入路径归档；**v28 报告未在仓库/开放 PR 中检索到**，推断为会话侧未归档轮，本表以可检索的 v29 为准）、UX v10 实跑补验行；§三补 R163–R166 证据索引；§五登记 R167 时点待办（#332→#333 合并顺序）。
- `docs/acceptance/DEMO_SCRIPT.md`：新增第 21b 步 view-only 演示点（约 40 秒，从第 21 步 readonly 时长中匀出，30 分钟总长不变）：手工构造临时 view-only 授权账号 → 订单写操作 → 中文提示「店铺无操作权限」（403/40303）；常见坑补 seed 无内置 view-only 账号、临时账号不随 clean 清理。

## 三、Docker 三角色（含 view-only persona）实跑

三角色 + view-only persona + 平台管理员按脚本实跑，全程录屏（证据外置不入库），12 项 UI 断言全部 PASS，无 P1/P2；详见 DEMO_SCRIPT「实跑验证记录」2026-08-07（R167 线2）条目。其中 UX v10（#327，R163 时点未合入未验）本轮补验通过：币种设置 dirty 路由离开中文确认弹窗、备份页时间列 `YYYY-MM-DD HH:mm:ss` 与中文确认按钮。收尾 `seed:demo:full:clean` + `verify` 输出 `zero DEMO- residual rows`，临时 view-only 账号、探针 token、备份记录均已清理。

## 四、质量门禁

- `pnpm check:ui-copy --strict`：PASS（本轮强制项）。
- 本轮为纯文档变更（docs/acceptance ×2 + docs/progress + docs/PROGRESS.md），无代码改动，未跑后端/前端构建与测试套件（无影响面）；Actions CI 不作为本轮结论依据（任务口径）。

## 五、风险与未覆盖项（需注意）

1. #332 与 main 冲突未解，view-only **全站**扫尾（30 写探针面）尚未进 main——当前 main 仅覆盖 #322/#330/#331 修复面；合并顺序须 #332 → #333，勿倒序。
2. 大回归 v28 报告不可检索（推断会话侧未归档），如后续入库发现额外 P2 需另轮跟进。
3. 前哨抽验为轻量口径（6/16 项直接实测），其余 10 项依赖 v29/R161 v8 文档证据，未做本轮直接复测。
4. 观察项（非缺陷，P3 级 UX 建议）：view-only persona 在订单列表/详情仍渲染写按钮，拦截为提交后 403 + 中文 toast；#332 含前端一致性收口，合入后可复核是否前置禁用。

## 下一步

- 解 #332 冲突（按 v29 定案）→ 合入 → 合入 #333 → v29 报告归档进 main。
- 下轮验收包在 #332/#333 合入后收口 §一/18 的 ⏳ 状态。
