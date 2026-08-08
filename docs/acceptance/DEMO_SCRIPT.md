# R123 完整演示动线脚本（30 分钟 · Docker 全栈 + seed:demo:full，R128 增补自动化新动作，R132 增补 R128–R131 交付，R136 增补 R132–R135 交付，R141 增补 R136–R140 交付，R148 增补 R144–R147 交付，R153 增补 R148–R152 交付，R158 增补 R153–R157 交付，R163 增补 R158–R162 交付，R167 增补 view-only 演示点，R170 增补 R167–R169 口径）

> 面向老板验收/对外演示。与 [ACCEPTANCE_R123.md](ACCEPTANCE_R123.md) 配套：本脚本走一遍业务闭环「采集/选品 → 优化 → 草稿 → 货源 → 订单 → 审单 → 采购 → 入库 → 发货 → 消息 → 财务对账 → 报表」，覆盖三角色与移动模式。
> R128 增补：第 11–12 步纳入 R126 自动化新动作（自动应用发货规则/自动分仓，#268+#270，**已合入 main**，基于最新 main 构建即可演示）；为保持 30 分钟，原「客服与话术」独立步骤并入买家消息步骤。
> R132 增补：R128–R131 交付纳入动线——第 9 步补订单号/客户筛选（R129 #274），第 18 步补聚合下推口径说明（R130 #276），第 20 步前新增双租户隔离演示（R128 #272）；#275 执行日志样本/操作日志中文映射与 #277 CSV 全量导出均已合入 main，基于最新 main 构建即可演示。
> R136 增补：R132–R135 交付纳入动线——第 9 步补订单标签列/按标签筛选与批量打标（R135 #282，已合入 main），第 11 步补 `add_tag` 自动打标签规则与成功日志样本，第 18 步补对账 25+ 行分页/合计与跨页全量导出演示（R136 seed，随 fix/round136-p2 PR），第 20 步 operator 补自动化正样本 DEMO-AT-1005 真实触发（同上）；为保持 30 分钟，标签演示并入既有步骤不新增独立步骤。升级演练复跑（R132 #279）/安全审计复跑（R133→#281）/竞品复评 v5（R134 #280）为报告类交付，不占演示时长，口径见验收包 §一/12。
> R141 增补：R136–R140 交付纳入动线——第 23 步治理面收尾补备份对象存储演示点（R138 #287，已合入 main；未配置 S3 时按降级本地路径口径演示上传状态「仅本地」（库内 `uploadStatus=skipped`）并口头说明配置 `BACKUP_S3_*` 后 uploaded/保留策略/取回；需先将 `.env` 改为 `BACKUP_ENABLED=true`、`BACKUP_MODE=local` 并重启 backend）；第 18–19 步口径补充报表 CSV「未折算」显式占位与对账差异 CSV 平台列中文化（R136 #284 / R137 #285）；深分页中文提示与未绑定本地规格口径（R139 #288）不占独立步骤，可在第 9 步口头带过；升级演练季度复检（R137 #286）为报告类交付不占演示时长。为保持 30 分钟，备份演示并入第 23 步不新增独立步骤。
> R148 增补：R144–R147 交付（#294–#300，均已合入 main）纳入动线——第 1 步后新增第 1b 步实时经营大屏（R145 #296，`/dashboard/screen`，约 1 分钟快速展示）；第 23 步治理面收尾并入 MCP 只读接入演示点（R144/#294–#295 + R146/#298，`/settings/mcp-tokens` seed 的 DEMO-MCP token 脱敏样本 + 现场创建 token 用 curl 或 MCP Inspector 调 `orders_query` + 审计日志卡片）；采购单详情中文支付状态/渠道（R147 #299/#300）在第 14 步口头带过不占独立步骤。为保持 30 分钟，大屏限时 1 分钟、MCP 并入第 23 步不新增独立段；大回归 v22/v23 为报告类交付不占演示时长（v23 P2 时间列格式化已由 R148 本轮收口，演示前需基于含 R148 修复的分支构建）。
> R153 增补：R148–R152 交付纳入动线——第 17 步买家消息补多语言模板演示点（R152 #309，⏳ 待合并：语言列/推断来源与回退标注、切换语言重生成、话术模板语言变体）；第 23 步 MCP 演示点扩展为「只读 API 接入（MCP / 开放 API）」，并入开放 REST API curl 演示（R152 #308，⏳ 待合并：创建 token 选用途「开放 API」，curl `GET /api/open/v1/orders` 等）；安全审计 R148+P2 收口（#302 已合入 / #303 ⏳）、升级演练 R149（#304 ⏳）、deploy-prod 修复（#305 ⏳）、竞品复评 v7（#306 ⏳）为报告/工程类交付不占演示时长，口径见验收包 §一/15。取舍：为保持 30 分钟，多语言并入第 17 步、开放 API 并入第 23 步均不新增独立步骤；为腾时长，第 23 步 MCP 现场 curl 演示由「tools/list + tools/call 两次」压缩为「任选其一 + 口头说明」，省出时长给开放 API curl；未合并 PR 演示前需基于 main 叠加 #303/#305/#307/#308/#309 分支构建（#304/#306 纯文档不影响构建）。（R158 时点 #303–#309 已全部合入 main，本段叠加分支要求已失效，见 R158 增补）
> R158 增补：R153–R157 交付中影响演示动线的为大屏多币种折算显式口径 + 租户级自定义指标卡片（R156 线2 #318，⏳ 待合并），并入第 1b 步大屏演示：销售额/毛利卡折算口径角标 tooltip、「未折算（不计入合计）：EUR 88.00」原币显式列出（seed 多币种样本 DEMO-FX-USD-0001 开箱即折算、DEMO-FX-EUR-0001 演示未折算；「已折算：X」行与未折算行互斥，仅在无未折算币种时显示）、admin 齿轮入口配置 8 张卡启用/排序（保存即时生效且持久化，租户级隔离；readonly/operator 无齿轮入口，PUT 403）。安全加固三项行为变更（R153 线1 #311 ⏳）、入口级审计与分页 400 口径（R154 线1 #312 ⏳）、MCP 错误码 -32603（R156 线1 #317 ⏳）、生产配置面（R155 线2 #316 已合入）均为安全/工程面交付不占演示时长，可在第 23 步口头一句带过（入口级审计可顺手展示：审计卡片筛选新增 `auth_failed`/`rate_limited` 状态与 `mcp:auth`/`openapi:auth` 工具名），口径见验收包 §一/16。取舍：为保持 30 分钟，折算/自定义卡片并入第 1b 步不新增独立步骤，第 1b 步时长由约 1 分钟放宽至约 1.5 分钟，相应压缩第 2 步采集任务列表展示；卡片配置演示仅改一次（关漏斗 + 告警上移）并手动改回默认再保存（弹窗无「恢复默认」按钮），避免反复保存占时；#318 合入前演示需基于 main 叠加 #312（含 #311）→#317→#318 分支构建（或直接用集成预演分支 `integration/r157-regression-v26` 合最新 main）。（R163 时点 #312（携带 #311）/#317/#318 已全部合入 main，本段叠加分支要求已失效，见 R163 增补）
> R163 增补：R158–R162 交付以安全/工程/文档面为主（#320/#321/#322/#323/#326 已合入 main；合并期更新：#324/#325/#327 亦已先后合入），不新增独立演示步骤，30 分钟时长不变：①大屏折算/自定义卡片（#318）已合入 main，基于最新 main 构建即可演示第 1b 步，不再需叠加分支；②报表币种设置未保存提示（R158 #320）可在第 19 步后顺带演示：`/settings/report-currency` 编辑汇率后底部出现「有未保存的更改，保存后才会生效」Tag，点「重新加载」还原后消失（约 20 秒，不影响总时长；合并期更新：#327 已合入 main，dirty 时路由离开现有确认弹窗（`useUnsavedChangesGuard`），可一并顺带演示）；③MCP token 页审计卡已改名「MCP / 开放 API 调用审计日志」且工具筛选按「MCP 工具 / 开放 API」分组（#320），第 23 步预期已同步；④view-only 越权修复（#322）与 40303 业务码统一（#323）为安全面交付不占演示时长，可在第 21 步 readonly 演示时口头一句带过（店铺级 view 授权写操作后端 403 兑底）；⑤升级演练 R159（#321）、竞品复评 v8（#324，已合入）、MCP 写白名单设计稿（#326，方案待决策）为报告/决策类交付不占演示时长，口径见验收包 §一/17；⑥UX v10 修复（#327，合并期更新：已合入 main）：基于含 #327 的 main 构建时，第 23 步备份/恢复页创建时间列与确认弹窗已中文化、大屏趋势 tooltip 时间已格式化（本轮实跑时点 #327 尚未合入，当时为 raw ISO/英文 Cancel-OK，属预期；合入后重建即中文化，下轮实跑补验）。
> R167 增补：R163–R166 为安全修复与回归密集期（view-only 权限体系收口 #322/#330/#331 已合入 main，#332/#333 待合），新增第 21b 步 view-only 演示点（约 40 秒，从第 21 步 readonly 演示时长中匀出，30 分钟总长不变）：登录 view-only 授权账号展示写操作 403 中文提示「店铺无操作权限」。seed 未内置 view-only 账号，演示前需手工构造（见第 21b 步说明与常见坑）；#327（UX v10）已合入 main，R163 登记的「下轮实跑补验」项本轮补验（币种设置离开确认弹窗、备份页时间/弹窗中文化）。
> R170 增补：R167–R169 交付不新增演示步骤，仅改口径（30 分钟总长不变）：① #332/#333 已合入 main，第 21b 步 view-only 写控件已按店铺 scope 前置禁用（R167 #335 收口），审单批量含 view-only 店铺时整批 403/40303 零状态变更；② 第 23 步 `/settings/mcp-tokens` 页「配置方法见 docs/mcp.md 与 docs/open-api.md」已改为站内可点击链接（/docs/*.md，UX v10 P2-3 收口），可顺带点开展示；③ MCP 工具分页为钳制口径、开放 API 非法分页 400/40001，口径差异已写入 `docs/mcp.md`，口头带过即可。
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
| 第二租户管理员（R128） | `demo_tenant2_admin@trademind.local` | `DemoTenant2Admin123!` | 双租户隔离演示（仅见 DEMO-T2- 数据） |
| 平台管理员（bootstrap，非 seed） | `admin@example.com` | `admin123456`（`.env.docker.example` 默认） | 备份管理等平台面演示（第 23 步，tenant 0） |

- 演示结束清理：`DB_HOST=127.0.0.1 pnpm seed:demo:full:clean && DB_HOST=127.0.0.1 pnpm seed:demo:full:verify`（期望输出 `zero DEMO- residual rows`）。

## 演示动线（约 30 分钟）

以 `demo_admin` 登录开始。每步给出 入口路由 / 操作 / 预期结果。

### 第 1 段：采集 → 选品 → 优化 → 草稿（约 6 分钟，含第 1b 步大屏限时 1.5 分钟（R158 放宽，含折算/卡片配置演示点）；第 2 步采集任务列表快速带过，为第 3 段留时长）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 1 | 运营总览 | 登录后停留首页 `/dashboard` | 经营概览、待办与新手引导卡片加载 |
| 1b | 实时经营大屏（R145）+ 折算口径与自定义指标卡片（R156 #318，已合入 main） | 菜单进 `/dashboard/screen`，点全屏按钮；限时约 1.5 分钟 | 深色大屏：今日订单/销售额/毛利/告警四张 KPI 大数字卡（seed 数据下有值，缺汇率/成本时显「—」不伪造）、待办五类可点击跳转、近 7 天漏斗与 24h 趋势、告警滚动；右上「更新于」时间随 15/30/60s 轮询刷新；可切浅色；演示完退出全屏回主动线。**折算口径（R156 #318，已合入 main，基于最新 main 构建即可演示）**：销售额/毛利卡折算口径角标 tooltip（按租户报表币种与手工汇率折算）；seed 多币种样本下 USD 订单（DEMO-FX-USD-0001）开箱即折算计入合计（API `convertedCurrencies` 含 USD），EUR 订单（DEMO-FX-EUR-0001）显「未折算（不计入合计）：EUR 88.00」原币金额显式列出不伪造（注：卡面「已折算：X」与「未折算…」两行互斥，存在未折算币种时优先显未折算行，「已折算：EUR、USD」仅在补齐 EUR 汇率后出现）；可口头说明：在 `/settings` 报表币种配置补 EUR 汇率后合计精确变化且与利润报表同口径（不必现场改）。**自定义指标卡片（同 #318）**：admin 右上齿轮入口打开卡片配置（8 张卡启用/排序），演示关闭漏斗 + 告警上移首位→保存即时生效、F5 持久化，随后手动改回默认启用/顺序再保存（弹窗无「恢复默认」按钮）；口头说明：配置租户级隔离，PUT 仅 `settings.manage`（readonly/operator 无齿轮入口、PUT 403，未知/重复/全禁用 400） |
| 2 | 采集 | 「采集中心」`/collect/hub`，展示 1688/拼多多入口与采集任务列表 `/collect/tasks` | DEMO 采集任务留痕（当前 seed 均为 success 样本；失败重试动线可在任务列表口头说明），登录风险提示可见 |
| 3 | 选品 | 「选品任务」进入 DEMO 任务详情 `/selection/tasks/<uuid>` | 候选清单带 AI 评分/预估利润 |
| 4 | 选品数据面（R120） | 候选行点「数据面板」抽屉；多选 2–3 行点「对比所选」 | 面板展示采集价格/销量留痕、同类目基准、价格走势图；对比抽屉可导出 CSV |
| 5 | AI 优化 | 商品草稿 `/product/drafts` 进任一 DEMO 草稿详情，展示 AI 标题/描述面板与违禁词合规面板 | AI 结果对比/应用/撤销入口可见；违禁词命中在 readiness「合规检测」高亮（未配 AI Key 时展示既有 DEMO 结果与显式提示，不阻演示） |
| 6 | 发布检查 | 草稿详情「发布检查」 | passed / warning / failed 三态与阻断原因中文展示 |

### 第 2 段：货源 → 刊登 →（降级边界说明）（约 4 分钟）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 7 | 货源 | 「货源与采购」→ 货源档案，展示 DEMO 商品主货源与 SKU 映射 | 进价/链接/库存参考齐全，「订单→采购」依据可解释 |
| 8 | 刊登 | 草稿详情刊登 Tab / 批量刊登向导创建本地草稿批次，`/product/publish-batches/:id` 查看 | TikTok/Shopee 等显示「仅生成本地草稿」；批次 success；**口径说明**：真实平台连通卡凭证（验收包 §二），闲鱼通道已真实验证 |

### 第 3 段：订单 → 审单 → 自动化（含 R126 新动作）→ 采购 → 入库 → 发货（约 11 分钟）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 9 | 订单 | 「订单列表」筛选 DEMO 订单，展示 SKU 匹配与批量操作；搜索表单「订单号」填 `DEMO-AT-1004` 点查询（R129 修复） | 列表含审单状态、打单状态列；订单号筛选生效 + URL 回写（`orderNo=DEMO-AT-1004`）、刷新深链持久、重置正常；该单「库存扣减」列展示与「SKU 已匹配」前置一致（R132 seed 补 match 行）；标签列展示 seed 样本彩色 Tag，按标签筛选生效 + URL 回写 `tagId=`，勾选多单批量添加/移除标签幂等（R135 #282） |
| 10 | 审单（R114） | 「审单工作台」与 `/settings/order-review-rules` | 待审/挂起样本与命中原因；采购/发货对未过审订单强制阻断 |
| 11 | 自动化规则与真实触发（R119+R126） | `/settings/order-automation-rules` 展示规则列表：除 R119 四动作外，R126 新增「自动应用发货规则」（recommend/apply 参数 Tag）与「自动分仓」（default_warehouse/stock_first 策略 Tag）规则；规则列表另含 R135 `add_tag` 自动打标签规则（tagIds 配置，成功执行日志 seed 样本可查）；回订单列表勾选 `DEMO-AT-1004`（unpaid、审单通过、SKU 已匹配）批量「标记已付款」 | 触发 `order_paid` 自动化：`/orders/automation-logs` 出现新执行记录——自动生成采购单 + 自动应用发货规则（计划物流商落单）+ 自动分仓三动作 success（正向样本）；`DEMO-AT-1001` 同操作演示安全阻断负样本（无本地 SKU 匹配） |
| 12 | 自动化新动作日志/详情落地（R126+R127） | `/orders/automation-logs` 查看 seed 样本 `DEMO-AT-1201`（发货规则直接应用成功正样本）与 `DEMO-AT-1202`（分仓库存不足失败负样本），对 1202 点「重试」；打开第 11 步订单详情 | 失败留痕文案「执行失败（本轮尝试 N 次）」与累计「尝试次数」列口径区分，重试后仍失败原因更新；订单详情「基本信息」新增「计划物流商」（名称+自动推荐/自动应用 Tag+命中规则名）与「分配仓库」（名称+策略 Tag）两项；人工选择不被自动结果覆盖 |
| 13 | 自动化轨迹（R120） | 订单详情 `?tab=automation` 深链 | 成功/失败/跳过留痕 + 跳转全量日志 |
| 14 | 采购 | 侧栏「货源与采购」→ 采购单（含第 11 步新生成单，概览/明细行展示来源销售订单号可跳转，R124），演示 提交/回填 1688 单号/标记付款/签收 流转 | 状态机流转 + 审计留痕；**口径说明**：1688 人工下单过渡模式 |
| 15 | 入库/多仓（R112） | 签收时选仓（默认仓预选）；`/inventory/warehouses` 展示仓库与调拨 | 分仓库存 Tag、按仓扣减（含自动分仓订单锁定已分配仓）、仓间调拨原子流水 |
| 16 | 发货/打单（R111） | 订单发货弹窗「按规则推荐物流商」；`/orders/print` 按面单模板打印预览 + 标记已打单 | 推荐可解释可覆盖（与第 12 步计划物流商互不覆盖）；打印页明示「非电子面单」 |

### 第 4 段：消息 → 财务对账 → 报表（约 5 分钟）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 17 | 买家消息 + 话术（R119/R109） | `/customer/buyer-messages`：待发草稿 tab 编辑/标记已发送/忽略；节点规则 tab；顺带快速展示 `/customer/reply-templates` 分组模板（原独立客服话术步骤并入本步，为 R126 新动作腾时长） | 变量已按订单上下文填充，缺失变量警示；页顶降级说明（人工确认、绝不自动外发）；话术分组/变量填充可见；**多语言（R152 #309，已合入 main）**：待发草稿列表语言列与推断来源标注——`DEMO-BM-1005`（US→en，order_country）/`DEMO-BM-1006`（BR→pt）正样本、`DEMO-BM-1001`（无国家→zh-CN fallback）负样本（注：正常推断来源如「按收货地推断」仅在语言 Tag 悬停 Tooltip 与编辑弹窗展示，列表列内仅回退类来源直接显示文字）；编辑弹窗「按所选语言重新生成」后 `langSource` 变 manual（仅改草稿不外发）；话术模板页语言列与英/西/葡变体可见 |
| 18 | 财务对账（R121+R130） | `/orders/finance-payments` 登记回款；`/orders/finance-reconciliation` 对账工作台；`/orders/finance-report` 报表 | 已结清/少款/多款/未回款样本；实算 vs 估算毛利差异；店铺×月份汇总，CSV 可导出（R130 #276 后聚合已下推 SQL，大数据量下数值口径不变；CSV 全量导出已随 #277 合入，导出不再受单页上限截断；R136 seed 后对账工作台 25+ 行——默认 20/页出第二页，顶部合计区跨页聚合，CSV 导出行数 > 单页可直观证明 #277 跨页全量；对账差异 CSV「平台」列中文平台名（R136 #284）） |
| 19 | 三报表（R110） | `/orders/reports-profit`（另有采购/库存报表页） | 多币种本位币折算、缺进价/未折算显式提示；CSV 导出无汇率列与页面同口径显式「未折算」不再留空（R137 #285） |

### 第 5 段：三角色 + 移动模式 + 治理面（约 4 分钟）

| # | 环节 | 操作 | 预期 |
| --- | --- | --- | --- |
| 19b | 双租户隔离（R128） | 退出，登录 `demo_tenant2_admin`；再回主租户账号搜 `DEMO-T2` | 第二租户仅见 DEMO-T2- 店铺/订单/规则/执行日志；主租户搜 DEMO-T2 空态、零泄漏 |
| 20 | operator | 退出，登录 `demo_operator`；订单列表搜 `DEMO-AT-1005`（授权手工店、unpaid、SKU 已匹配）批量「标记已付款」（R136） | 仅授权店铺数据可见；无权限路由统一「暂无访问权限」语义页；operator 自行触发 order_paid 自动化成功（执行日志新增生成采购单 success，不再依赖 admin 账号） |
| 21 | readonly | 登录 `demo_readonly`，查看任一写入口 | 写入口不渲染（隐藏，UI 无写触发路径）；后端写接口 403 兜底（历史轮次 curl 实测证据，现场演示以写控件禁用为准）；读路径完整 |
| 21b | view-only 店铺授权（R167 增补，#322/#330/#331 安全收口演示点，约 40 秒） | 演示前手工构造：把一个临时 operator 账号的店铺授权改为 `view`（`docker exec trademind-full-postgres-1 psql -U trademind -c "UPDATE user_store_permissions SET permission_scope='view' WHERE user_id=(SELECT id FROM admin_users WHERE email='<临时账号>')"`，勿改 demo_operator 以免影响第 20 步）；登录该账号，订单列表选一单尝试写操作（如标记已付款） | 数据可见（view 管读）；写操作被后端拒绝，UI 展示中文提示「店铺无操作权限」（403/40303），无裸 JSON/英文报错/状态变化；口头带过：客服会话、审单、异常标记、店铺同步/删除/授权、刊登等写面同口径（#331 六面，#332 全站扫尾待合）；演示后改回 `operate` 或删除临时账号 |
| 22 | 移动模式（R113+R124） | DevTools device toolbar 375px（zoom 100%）刷新；拉宽至 768px 对比 | 375：底部 5 tab（首页/订单/采购/库存/我的）且无侧栏汉堡（R124 断点互斥口径），「我的」页含经营报表/异常工作台/告警中心补偿入口；≥768：仅侧栏无底部导航；`/m/home` 指标卡与待办触屏动线；表格横向内滚不溢出 |
| 23 | 治理面收尾 + 备份对象存储（R138）+ 只读 API 接入（MCP R144/R146 + 开放 API R152 #308，已合入 main） | 管理员回登，快速过 `/ops/task-center/operation-tasks`（批量审批）、失败任务中心、操作日志；切平台管理员 `admin@example.com` 登录进 `/ops/backups`，点「创建备份」（前置：`.env` 需 `BACKUP_ENABLED=true`、`BACKUP_MODE=local` 并重启 backend；`.env.docker.example` 默认 `BACKUP_MODE=disabled` 时创建只会得到「待人工复核」） | 运营任务批量批准弹窗、失败深链、审计留痕；备份页可见「上传状态」「上传目标」列，备份 status=completed；演示环境未配 `BACKUP_S3_*` 时上传状态列显「仅本地」（库内 `uploadStatus=skipped`；降级本地路径，产物落 backend 容器本地），**口径说明**：配置 `BACKUP_S3_*` 后自动上传 S3 兼容端点（uploaded + 上传目标列显 bucket/prefix）、失败可行内重试、按保留策略清理、本地缺失自动取回（MinIO 实测证据见 PR #287）。随后切回 `demo_admin` 进 `/settings/mcp-tokens`（MCP 只读接入，R144/R146）：seed 的 `DEMO-MCP 演示只读 token` 脱敏列表样本（`sp_mcp_ro_xxxx…xxxx`，明文不可再现）可见；现场「创建只读 token」（选 30 天有效期；选 7 天会立即命中「即将过期」≤7 天橙标，可顺带演示该提示）取得一次性明文，用 curl（`curl -X POST http://127.0.0.1:8080/api/mcp -H "Authorization: Bearer <明文>" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'` 返回 4 个只读工具，再 `tools/call` `orders_query`（参数用 `pageSize` 而非 `limit`，如 `-d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"orders_query","arguments":{"pageSize":5}}}'`）返回 DEMO 订单）或 MCP Inspector 演示（为腾时长给开放 API，`tools/list` 与 `tools/call` 现场任选其一实跑、另一条口头说明即可）；注意仅 `tools/call` 产生审计记录，`tools/list` 不计入；页内「MCP / 开放 API 调用审计日志」卡片（R158 #320 更名，工具筛选按「MCP 工具 / 开放 API」分组）出现该次调用记录（工具/成败/耗时，时间列为 `YYYY-MM-DD HH:mm:ss` 格式，R148 收口）；演示完点「吊销」立即 401。**开放 API（R152 #308，已合入 main）**：同页再「创建只读 token」用途选「开放 API」（列表用途列显示对应 Tag），取一次性明文后 curl 开放 REST 入口——`curl -H "Authorization: Bearer <明文>" "http://127.0.0.1:8080/api/open/v1/orders?pageSize=5"` 返回 DEMO 订单（客户名仅保留首字符，seed 买家为 DEMO- 前缀故显 `D**`，真实中文名如 `张**`）、`curl -H "Authorization: Bearer <明文>" "http://127.0.0.1:8080/api/open/v1/reports/summary"` 返回经营摘要；口头说明：用途为「MCP」的 token 调开放入口统一 401（入口选择器，存量 token 攻击面不变宽）、审计卡片出现 `openapi:orders_list` 等记录、5 端点全 GET 只读（详见 `docs/open-api.md`）。**口径说明**：只读查询、明文仅展示一次、限流与租户隔离见验收包 §一/14–15 |

## 常见坑（演示前自查）

- seed 需在仓库根目录执行且 PostgreSQL 端口 5432 已映射（full 栈默认已映射）；宿主机执行必须带 `DB_HOST=127.0.0.1`。
- 注册演示依赖 SMTP；未配置时不要演示自助注册，或临时向 Redis 注入验证码（仅测试环境，见 `.agents/skills/demo-fullstack-walkthrough/SKILL.md`）。
- `/purchase/orders` 为别名重定向（R122 起），正式入口是侧栏「货源与采购」（`/procurement/orders`）。
- 移动模式检查用 100% zoom；375px 表格横向内滚属预期，不算根节点溢出。
- 容器跑的是构建时代码：切分支后需 `docker compose -f docker-compose.full.yml up -d --build` 重建。
- ~~R126 新动作依赖 #268（含 #270）代码~~——#268/#270 已合入 main，基于最新 main 构建即含 AT-1201/1202 样本与规则页新动作选项。
- R130 线2 执行日志样本（DEMO-AT-1301/1302/1303）与操作日志中文映射（#275）、对账/毛利 CSV 全量导出（#277）均已合入 main，基于最新 main 构建即可直接演示。
- 订单标签（标签列/筛选/批量打标/add_tag 规则，R135 #282）与对账 25+ 行量样本、operator 正样本 DEMO-AT-1005（R136 #283）均已合入 main，基于最新 main 构建 + seed 即可演示。
- ~~R153 时点开放 API（#308）与买家消息多语言（#309）尚未合入 main~~——R158 时点 #303–#309 已全部合入 main，基于最新 main 构建即可演示。
- ~~R158 时点大屏折算/自定义指标卡片（#318）尚未合入 main~~——R163 时点 #312（含 #311）/#317/#318 已全部合入 main，基于最新 main 构建即可演示第 1b 步全部内容。
- ~~R163 时点 #327（UX v10 修复）尚未合入 main~~——合并期更新：#327 已合入 main，基于最新 main 构建后备份/恢复页时间列与确认弹窗已中文化、币种设置页 dirty 时路由离开有确认弹窗；本轮实跑时点尚未合入，对应项下轮实跑补验——R167 线2 已补验通过（见实跑验证记录 R167 条目）。
- readonly/operator 演示账号仅授权 DEMO-手工渠道店：用这两个账号搜其他店铺订单（如 DEMO-AT-1004，抖店旗舰店）为空态属预期权限设计，非数据丢失；operator 自动化正样本用 DEMO-AT-1005（手工渠道店）。
- 备份管理 `/ops/backups` 仅平台管理员（tenant 0）可进，demo_admin 等租户账号无入口；用 `.env.docker.example` 默认 bootstrap 账号 `admin@example.com / admin123456`。备份演示前置：`.env.docker.example` 默认 `BACKUP_ENABLED=false`、`BACKUP_MODE=disabled`，此时创建备份仅得到「待人工复核」；需改 `BACKUP_ENABLED=true`、`BACKUP_MODE=local` 并重启 backend 才能演示 completed。未配 `BACKUP_S3_*` 时上传状态列显「仅本地」（库内 `uploadStatus=skipped`），属预期降级口径，不是故障。

- 第 21b 步 view-only 演示需临时账号：seed 无内置 view-only 授权账号，临时账号/改 scope 不属于 DEMO- 命名体系，`seed:demo:full:clean` 不会清理，演示后需自行改回/删除。

## 实跑验证记录

- 2026-08-07（R167 线2）：Docker 全栈（最新 main `d645ec96` + 本轮纯文档分支构建，#322/#327/#328/#329/#330/#331 已合入面无需叠加分支）三角色 + view-only persona + 平台管理员按更新后脚本实跑（全程录屏，证据外置不入库）：`DB_HOST=127.0.0.1 pnpm seed:demo:full` 成功；admin 主线抽验——大屏 KPI（38 单 / 7,335.56）与「未折算（不计入合计）：EUR 88.00」、DEMO-AT-1004 标记已付款真实触发 5 条中文自动化轨迹（采购单/发货规则/分仓/标签闭环）、自动化日志 success/failed（尝试 3 次中文原因）/skipped 三态、买家消息 en/pt + zh-CN fallback 标注、MCP token 脱敏样本与审计卡片均通过；**UX v10 补验（#327 已合入，R163 时点未验项）**——币种设置 dirty 后路由离开弹中文确认「离开当前页面？…」（继续编辑/离开），备份页确认弹窗中文按钮（取消/创建）、创建时间列 `YYYY-MM-DD HH:mm:ss`，通过，常见坑对应条目口径已收口；operator——仅授权店可见（11 vs 38 单）、DEMO-AT-1005 批量标记已付款成功（无过度收紧）；readonly——写控件隐藏、读路径完整、大屏无齿轮；**第 21b 步 view-only persona（新增）**——临时账号（operator 角色 + 店铺 scope=view）订单数据可见，添加标签等写操作被拒且 UI 红色中文 toast「店铺无操作权限」（403/40303）零状态变化，客服会话详情完全只读（「当前为只读账号，不可生成建议或发送消息。」写入口全禁用）；375px 底部 5 tab 无汉堡 / 768px 仅侧栏。12 项断言全部 PASS，无 P1/P2；观察项（非缺陷）：view-only 订单页写按钮仍渲染、拦截为提交后 403 中文提示（#332 前端一致性收口合入后可复核前置禁用）。收尾 clean + verify 输出 `zero DEMO- residual rows`，临时账号/探针 token/备份记录已清理。本轮失实 = 0。
- 2026-08-07（R163 线2）：Docker 全栈（最新 main + 本轮纯文档分支构建，#318/#320 等已合入面无需叠加分支）按更新后脚本三角色 + 双租户 + 平台管理员全量实跑（全程录屏，证据外置不入库）：`DB_HOST=127.0.0.1 pnpm seed:demo:full` 成功；第 1b 步大屏（基于 main，#318 已合入）——折算角标 tooltip 文案逐字匹配、「未折算（不计入合计）：EUR 88.00」与已折算行互斥、8 卡配置保存 + F5 持久化 + 手动改回默认（无恢复默认按钮）均通过；币种设置未保存提示（#320）——dirty 时底部「有未保存的更改，保存后才会生效」Tag、「重新加载」还原后消失、路由离开无确认弹窗（与 #327 待合入口径一致）；第 23 步 MCP 审计卡更名「MCP / 开放 API 调用审计日志」+ 分组筛选（#320）、MCP token tools/list（4 工具）/`orders_query`（pageSize）成功并入审计、开放 API token 200、MCP 用途 token 调开放入口 401、演示后吊销；主线第 1–19 步（采集/选品数据面/草稿 AI 与合规/发布检查/货源/订单筛选与标签/审单/自动化规则与真实触发（DEMO-AT-1004 标记已付款→采购单/发货规则/分仓三动作 success）/AT-1201/1202/轨迹深链/采购/多仓/打单/买家消息多语言 en·pt·zh-CN fallback/话术/财务三页（31 行跨页合计）/利润报表未折算提示）全部符合；双租户隔离（tenant2 仅见 5 条 DEMO-T2-）、operator（DEMO-AT-1005 标记已付款触发自动化全部生效、大屏无齿轮）、readonly（写控件全部隐藏、读路径完整）均通过；平台管理员备份已完成/仅本地（raw ISO 时间列与英文 Cancel/OK 为 #327 待合入项，与本脚本常见坑口径一致）；375px 底部 5 tab 无汉堡/768px 仅侧栏；收尾 clean + verify 输出 `zero DEMO- residual rows`。本轮失实 = 0；补一条非失实提示：readonly/operator 仅授权 DEMO-手工渠道店，用这两个账号搜 DEMO-AT-1004（抖店旗舰店）为空态属预期权限设计，已补入常见坑。
- 2026-08-07（R158 线2）：Docker 全栈（最新 main `68268864` 本地合入集成预演分支 `integration/r157-regression-v26`（含 #311/#312/#317/#318）构建）按更新后脚本三角色实跑（全程录屏，证据外置不入库）：`DB_HOST=127.0.0.1 pnpm seed:demo:full` 成功；第 1b 步大屏——深色/浅色、全屏、四 KPI 有值、待办深链、漏斗/趋势/告警、「更新于」随轮询刷新均通过；折算口径（#318）——折算角标 tooltip 文案、「未折算（不计入合计）：EUR 88.00」显式列出、API `convertedCurrencies=["USD"]` 且 salesBase 7,335.56 含 USD 折算（DEMO-FX-USD-0001=USD 199.99 / DEMO-FX-EUR-0001=EUR 88.00 均 paid）；自定义指标卡片（#318）——admin 齿轮配置关漏斗 + 告警上移首位，保存 toast「大屏指标配置已保存」即时生效、F5 持久化，手动改回默认；operator 大屏可读且数据受店铺 scope（11 单 vs admin 38 单）、无齿轮入口，`PUT /api/v1/dashboard/screen/config` 403（40305）、GET 200；readonly 大屏可读、无齿轮、PUT 403（40301），运营总览写按钮不渲染、mcp-tokens 创建/吊销禁用；第 1 步 `/dashboard` 与第 23 步 `/settings/mcp-tokens`（页名「只读 API 接入（MCP / 开放 API）」、DEMO-MCP 与 DEMO-开放 API 脱敏样本、审计卡片）抽查通过；收尾 clean + verify 输出 `zero DEMO- residual rows`。两处失实已即修本脚本：①「已折算：X」行与「未折算…」行互斥，seed 多币种样本下销售额卡只显未折算行（「已折算：EUR、USD」仅补齐 EUR 汇率后出现），第 1b 步与 R158 增补措辞已改；②卡片配置弹窗无「恢复默认」按钮，需手动改回再保存，措辞已改。
- 2026-08-07（R153 线2）：Docker 全栈（main `7f5645c1` 本地叠加 #303/#305/#307/#308/#309 集成分支构建）按更新后脚本三角色实跑（全程录屏，证据外置不入库）：`DB_HOST=127.0.0.1 pnpm seed:demo:full` 成功；第 23 步开放 API（#308）——页面更名「只读 API 接入」+用途列、创建「开放 API」用途 token 一次性明文、curl `GET /api/open/v1/orders?pageSize=5` 返 5 条 DEMO 订单（total 36）且客户名脱敏、`/reports/summary` 正常、MCP 用途 token 调开放入口 401、审计卡片 `openapi:orders_list`/`openapi:reports_summary` 落库；MCP `tools/call orders_query`（pageSize）返 DEMO 订单、吊销后 401、审计时间列 `YYYY-MM-DD HH:mm:ss`；第 17 步多语言（#309）——BM-1005→en（按收货地推断）/BM-1006→pt/BM-1001→zh-CN fallback、「按所选语言重新生成」后来源「人工切换」（manual）、话术模板英/西/葡变体均符合；operator 仅授权店 11 条（admin 36 条）；readonly 写控件全部禁用（含 token 创建/吊销、重生成），写接口 403 本轮无 UI 触发路径未复测（沿历史轮次 curl 实测证据）；收尾 clean + verify 输出 `zero DEMO- residual rows`。四处失实已即修本脚本：①脱敏示例 `张**`→seed 实为 `D**`；②正常推断来源仅 Tooltip/编辑弹窗展示；③#308/#309 存在可解文件冲突（非「无冲突」）；④readonly 403 验证方式注明。

- 2026-08-06（R148 线1）：Docker 全栈（最新 main + 本轮 `feat/round148-acceptance-mcp` 修复构建）按更新后脚本三角色抽查实跑（全程录屏，证据外置不入库）：`DB_HOST=127.0.0.1 pnpm seed:demo:full` 成功；第 1b 步大屏 KPI 有值/待办深链/漏斗趋势告警渲染/「更新于」格式化/浅色与全屏均通过；第 23 步 MCP：seed DEMO-MCP 脱敏样本可见，创建 7 天 token 后「过期时间」列显 `2026-08-13 21:22:27`（非 ISO，v23 P2 修复点①），curl `tools/list` 返回恰 4 工具、`orders_query` 返回 DEMO 订单，审计日志「时间」列 `2026-08-06 21:23:15`（v23 P2 修复点②），吊销后 curl 401；operator 仅 scope 数据、大屏正常；readonly 创建/吊销禁用、写接口 403、大屏可读；收尾 clean + verify 输出 `zero DEMO- residual rows`。两处失实（选 7 天即命中「即将过期」标、`orders_query` 参数为 `pageSize` 非 `limit`）已修正本脚本第 23 步。
- 2026-08-06（R141 线1）：Docker 全栈（最新 main `99fd2e7d` 重建）按更新后脚本三角色实跑（全程录屏，证据外置不入库）：`DB_HOST=127.0.0.1 pnpm seed:demo:full` 成功；admin 动线抽样（标签列/筛选、深分页中文提示、未绑定本地规格「未绑定」、对账 28 行跨页合计、差异 CSV 平台列中文、利润 CSV「未折算」显式占位）全部符合；平台管理员 `/ops/backups` 创建备份 completed，未配 `BACKUP_S3_*` 时上传状态列显「仅本地」（库内 `uploadStatus=skipped`），demo_admin 无入口；operator 标记 `DEMO-AT-1005` 已付款后自动生成采购单/自动打标签/自动分仓 success；readonly 写入口全部不渲染；375px/768px 无横向溢出；收尾 clean + verify 输出 `zero DEMO- residual rows`。两处失实（备份需 `BACKUP_ENABLED=true`/`BACKUP_MODE=local` 前置、上传状态文案为「仅本地」而非「已跳过」）已修正本脚本。
- 2026-08-06（R136 线1）：Docker 全栈（main `7719cb3d` 本地叠加 #280/#281 + `fix/round136-p2`）重建后针对本轮改动点实跑核对：`DB_HOST=127.0.0.1 pnpm seed:demo:full` 成功，对账工作台 API（2026-05-01～2026-09-01）返回 rows=28、summary orderCount=28（结清 7/少款 3/多款 3/未回款 15），`/finance/reconciliation/export.csv` 导出 28 数据行（> 单页 20，跨页全量可演示）；`demo_operator` 登录订单列表可见 `DEMO-AT-1005`（unpaid），PUT 标记已付款后 `order_paid` 自动化真实触发（`DEMO-付款后自动生成采购单` status=success「已自动生成 1 张采购单」）；重复 seed 幂等（已付款 DEMO 单稳定 29 = 28 主租户 + 1 第二租户）；`seed:demo:full:clean` + `verify` 输出 `zero DEMO- residual rows`（含 operator 触发生成的采购单联动清理）。
- 2026-08-06（R132 线1）：Docker 全栈（main `eb626bd9` 本地叠加 `fix/round132-p2`）重建后针对本轮改动点实跑核对：`DB_HOST=127.0.0.1 pnpm seed:demo:full` 成功（`order_item_sku_matches` 计 6）；订单列表 API `orderNo=DEMO-AT-1004` 返回 `skuMatchStatus=all_matched`、`inventoryDeductStatus=none`（「库存扣减」列显示「未扣减」而非「SKU 未就绪」，与「SKU 已匹配」前置一致）；重复 seed 幂等；`seed:demo:full:clean` + `verify` 输出 `zero DEMO- residual rows` 且 `order_item_sku_matches` 归零。第二租户账号/19b 步骤沿用大回归 v19 实测证据（PR #272 评论）。
- 2026-08-05（R128 线2）：更新后脚本在 Docker 全栈（main `f3108e16` 本地叠加 `feat/round126-auto-actions` `339836de`，含 #268/#270）+ `seed:demo:full` 三角色逐步实跑全部 23 步（全程录屏）：22/23 与预期一致；第 11–12 步 R126 新动作全部验证通过（DEMO-AT-1004 三动作 success、AT-1201/1202 正负样本、重试累计 3→6 与「本轮尝试 N 次」口径区分、订单详情计划物流商/分配仓库展示）；唯一偏差为第 2 步采集失败样本描述失实（seed 实为全 success），已修正本脚本该步预期；收尾 clean/verify 零残留。
- 2026-08-05（R123 线1）：本脚本在 Docker 全栈（main `02b6b086` 构建）+ `seed:demo:full` 逐步实跑全部 23 步（三角色 + 375px 移动模式，全程录屏）：步骤 1–19、22、23 与预期一致；步骤 11 正/负样本真实触发验证通过（DEMO-AT-1004 自动生成采购单成功、DEMO-AT-1001 安全阻断留痕）；步骤 20/21 文案口径按实际表现修正（「暂无访问权限」/ 写入口隐藏而非禁用）。
