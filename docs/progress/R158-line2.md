# R158 线2：验收包补 R153–R157 增量（fullstack-engineer）

日期：2026-08-07。承接 R153 线2（#310）验收包增量模式，把 R153–R157 交付纳入验收包与演示脚本，并按更新后脚本 Docker 全栈三角色实跑留证。基线：最新 main（`68268864`，#315 已合入）。

## 1. ACCEPTANCE_R123.md 增量

- 新增 §一/16「R153–R157 增量能力（R158 增补）」六行：
  1. 开放 API/MCP 安全加固三项行为变更（租户禁用 token 立即失效 / `TRUSTED_PROXIES` 默认不信任代理 / 开放 API 审计 fail-closed）——R153 线1 / #311，⏳ 待合并；
  2. 入口级审计（401/429 落 `auth_failed`/`rate_limited` 审计行 + 节流防放大）+ 分页非法参数 400 口径——R154 线1 / #312，⏳ 待合并；
  3. v25 P2×4 收口 + 生产演练新配置面（`.env.prod.example` 补 `TRUSTED_PROXIES`/`OPENAPI_*`、production-deployment 客户端 IP 口径、permmatrix harness 修复）——#313/#315/#316 已合并，✅；
  4. MCP 错误码规范化（fail-closed 拒绝改规范 JSON-RPC `-32603`）+ `--pre-upgrade-check` 备份目录清晰报错 + demo clean 覆盖面核实——R156 线1 / #317，⏳ 待合并；
  5. 大屏多币种折算显式口径（`unconvertedRevenue`/`convertedCurrencies`）+ 租户级自定义指标卡片配置（GET 全角色 / PUT 仅 `settings.manage`）——R156 线2 / #318，⏳ 待合并；
  6. R157 集成预演（`integration/r157-regression-v26` 按序叠加 #311→#312→#315→#317→#318）+ R157 线1 交叉 QA 13/13 无 P0/P1（证据登记于 PR #318 评论，不入库）。
- §一/15 合入状态收口：R158 时点 #303–#309 已全部合入 main，七个 ⏳ 条目全部转 ✅（§五/10 划线存档）。
- §三 证据索引补 R153–R157 合入面条目；§五 新增第 11 条 R158 时点待办：建议合并顺序 #312（已含 #311）→ #317 → #318，#311 内容随 #312 合入后可直接关闭（其 base feat/round152-open-api 已合入 main 且有冲突）。

## 2. DEMO_SCRIPT.md 增量（保持 ~30 分钟）

- 新增「R158 增补」段：大屏折算/自定义指标卡片（#318 ⏳）并入第 1b 步，不新增独立步骤；第 1b 步时长约 1 分钟 → 约 1.5 分钟，相应压缩第 2 步采集任务列表展示；卡片配置演示仅改一次并手动改回默认。安全加固/入口级审计/MCP 错误码/生产配置面为安全/工程面交付不占演示时长，第 23 步口头带过（入口级审计可顺手展示审计筛选新增状态/工具名）。
- 第 1b 步扩写：折算口径角标 tooltip、「未折算（不计入合计）：EUR 88.00」显式列出（seed 样本 DEMO-FX-USD-0001 / DEMO-FX-EUR-0001）、admin 齿轮卡片配置（8 张卡启用/排序，保存即时生效持久化，readonly/operator 无入口 PUT 403）。
- 常见坑更新：R153 时点「需叠加 #303/#305/#307/#308/#309 分支构建」已失效（均已合入 main）；#318 合入前第 1b 步新演示点需基于 main 叠加 #312→#317→#318 构建（或用集成预演分支合最新 main）。

## 3. Docker 全栈三角色实跑（失实即修）

构建：最新 main 本地合入 `integration/r157-regression-v26`（含 #311/#312/#317/#318），`docker-compose.full.yml` 全栈 + `seed:demo:full`。三角色（demo_admin / demo_operator / demo_readonly）按更新后脚本实跑，全程录屏，证据外置不入库；PUT 403 用对应角色 JWT 直接调接口验证。结果与两处失实修正详见 `docs/acceptance/DEMO_SCRIPT.md`「实跑验证记录」2026-08-07（R158 线2）条目：

- 失实①：「已折算：USD」与「未折算…」两行互斥（`admin/src/pages/Dashboard/Screen/index.tsx` 仅在无未折算币种时渲染已折算行），seed 多币种样本下只显未折算行——脚本措辞已改，并登记进 demo SKILL 常见坑；
- 失实②：卡片配置弹窗无「恢复默认」按钮，需手动改回再保存——脚本措辞已改；
- 其余动线（大屏基础、折算 tooltip、未折算显式行、卡片配置保存/持久化/权限边界、operator scope、readonly 只读、第 1/23 步抽查、clean+verify `zero DEMO- residual rows`）全部符合。

## 4. 验证

- `pnpm check:ui-copy --strict` 通过；文档内路由/命令/账号与 Docker 实跑核对一致；本轮为纯文档变更（含 demo SKILL 常见坑登记），未改代码，后端/前端构建与测试门禁不适用（未执行原因登记于 PR 说明）。
- 证据外置不入库；Actions CI 不作验收依据。

## 遗留

- #311/#312/#317/#318 待老板按 §五/11 建议顺序合并；#318 合入后 DEMO_SCRIPT 第 1b 步「⏳ 待合并」标注与构建前置说明可在下轮验收包更新时转正。
