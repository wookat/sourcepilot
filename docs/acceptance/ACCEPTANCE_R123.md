# R123 预验收对照表（R91–R122 功能轮验收包，R128 增补 R124–R127，R132 增补 R128–R131，R136 增补 R132–R135，R141 增补 R136–R140，R148 增补 R144–R147，R153 增补 R148–R152，R158 增补 R153–R157）

- 轮次：R123 线1（technical-writer / product-manager）；R128 线2 增量更新（§一·补、§四、§五）；R132 线1 增量更新（§一/10 合入状态收口、§一/11、§五）；R136 线1 增量更新（§一/12、§四、§五）；R141 线1 增量更新（§一/12 合入状态收口、§一/13、§五）；R148 线1 增量更新（§一/14、§三、§五）；R153 线2 增量更新（§一/15、§三、§五）；R158 线2 增量更新（§一/15 合入状态收口、§一/16、§三、§五）
- 日期：2026-08-07
- 基线：main `02b6b086`（#260 已合并）；演示脚本实跑也基于该基线。**#261（R122 线1 性能收口 v2，perf/round122）已于本轮验收包提交后合入 main（合并提交 `60e09b19`）**，性能收口条目已随之收口。
- 口径：按 CHARTER §7 验收制整理——可运行成果（Docker 全栈）+ 演示（[DEMO_SCRIPT.md](DEMO_SCRIPT.md)）+ 需求（业务闭环）逐条对照 + 竞品对比结论（§四）。
- 证据类型说明：轮次报告 = `docs/PROGRESS.md` 对应轮次条目与归档报告；E2E = `admin/e2e/specs/` 下对应 Playwright 用例（CI `admin-e2e` 门禁常跑）；实测 = 大回归（v14/v16）Docker 全栈手工走查记录。

## 一、业务闭环逐环节对照

图例：✅ 完成（站内闭环可演示）；🟡 降级完成（有明确降级路径，真实通道卡外部凭证）；🔶 依赖凭证（代码就绪，需凭证解锁）；⏳ 待合并（能力已交付并过门禁，对应 PR 尚未合入 main）。

### 1. 采集 / 选品

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 商品采集（1688 / 拼多多 / 淘宝天猫 / 自定义规则，单条+批量） | MVP 主线（F 系列）；采集规则 AI 生成 | R57 主链路实测（大回归 v16）；`collect` 相关 E2E | ✅ |
| 采集运维（任务状态/失败重试/监控/浏览器登录态检测） | F 系列 + R84 季度回归修复 | 大回归 v14/v16 实测 | ✅ |
| AI 选品任务与可上架清单（AI 评分，未配 Provider 走规则兜底） | 选品主线 + R120 线1 增强 | `round120-selection-insights.spec.ts`；operations-manual 每日流程实测 | ✅ |
| 选品数据面（候选数据面板/价格走势/多候选对比 CSV/外部数据源 Provider 预留） | R120 线1 / #257 | `round120-selection-insights.spec.ts`（五档视口）；大回归 v16 实测 | ✅（外部热销榜数据源为 🔶 预留槽位，缺凭证明确降级不虚构） |

### 2. AI 优化（文案 / 图片 / 合规）

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| AI 标题优化 / 描述生成 / Prompt 模板 / 应用与撤销（幂等） | MVP 主线 + P2.2 幂等收口 | 后端幂等单测；demo:auto-acceptance 历史 run | ✅（需租户自配 AI Provider Key；未配置有显式提示，选品评分自动规则兜底） |
| AI 图片（抠图/翻译，removebg / OpenAI Image / ComfyUI Provider） | MVP 主线 | 同上 | ✅（同上，需 Image Provider Key） |
| 批量 AI 工作台（批量文案/图片 + 批次复核） | P8 运营任务中心 + 工作台线 | `ops/task-center` E2E；大回归实测 | ✅ |
| 违禁词合规（租户词库+预置库、扫描引擎、readiness 阻断、批量检测） | R109 / #229 | round109 E2E；后端单测 | ✅ |
| AI 优化联动违禁词（prompt 注入规避 + 结果自动复检提示） | R109 / #232 | 后端单测；草稿详情面板实测 | ✅ |

### 3. 商品草稿 → 货源 / SKU

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 草稿管理（SKU/图片/库存阈值/运营进度/发布前检查 readiness） | MVP 主线 | 大回归实测；readiness E2E | ✅ |
| 货源管理（供应商 + 商品货源档案 + SKU 映射，报价刷新） | 采购主线 | operations-manual 实测 | ✅（报价刷新当前为 mock 服务，接 1688 官方 API 后切真实 🔶） |
| 数据搬家（店小秘/马帮格式+通用模板，商品/订单/库存期初/货源四类导入；真实 .xlsx；万行性能；回款导入 R121） | R92 #198、R115 #243、R116 #249、R121 #259 | 后端导入回归；导入 E2E；R116 万行性能记录 | ✅ |

### 4. 刊登 / 店铺

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 多平台刊登中心（单商品+批量草稿、发布任务、失败恢复） | P8/P9 主线 | 刊登批次 E2E；demo 走查步骤 9–11 | ✅（站内草稿闭环） |
| 6 平台 adapter（抖店/TikTok/Shopee/Lazada/Amazon/闲鱼） | P3 抖店 + 各预研轮 + Goofish Beta #226 | 契约门禁；Goofish 真实 E2E（唯一已真实验证通道） | 🔶 真实刊登连通卡平台凭证（抖店为最高杠杆项）；无凭证时统一 `local_draft_only` 降级 |
| 店铺授权（OAuth 闭环、凭据加密存储、连接测试） | P3/P4 | 安全测试；设置页实测 | ✅（真实授权需平台 App Key 🔶） |

### 5. 订单 → 审单

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 订单全生命周期（同步/手动创建/批量导入/SKU 匹配/批量付款/批量发货/导出） | 订单主线 + R95 #206 租户唯一订单号 | 订单 E2E；大回归实测 | ✅（真实平台订单同步卡凭证 🔶，站内手动+导入通道完整） |
| 订单异常工作台（未匹配/扣库存失败/采购受阻/负毛利等聚合） | 订单主线 + R101 #213 | 异常工作台 E2E | ✅ |
| 审单规则（金额/地址/黑名单/超量/重复收件人 → 通过/待审/挂起；采购发货强制阻断；dry-run） | R114 #240、R118 #253 | round114 E2E；审单工作台实测 | ✅ |
| 自动化订单规则（事件触发 → 站内动作：确认付款/生成采购单/打标签/挂起；执行日志；防环） | R119 #254、R120 #256、R122 #260 | round119/120/122 E2E；大回归 v16 实测（含 DEMO-AT-1004 正向生成采购单样本） | ✅ |

### 6. 采购 → 入库 → 发货 → 签收

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 采购协同（按主货源聚合生成采购单、批量提交/确认/付款/签收、作废） | 采购主线 + R48 | 采购 E2E；operations-manual 实测 | ✅（1688 人工下单过渡模式：导出 CSV 人工下单回填单号；API 直连下单卡资质 🔶） |
| 轻量多仓库存（仓库/分仓库存/调拨/签收选仓/按仓扣减/报表按仓筛选/默认仓切换） | R112 #236/#237、R115 #242 | round112 E2E（10 例）；Docker 实测调拨原子性 | ✅ |
| 库存协同（库存中心/预警/扣减记录/流水/平台同步任务） | 库存主线 + P9 | 库存 E2E | ✅（平台库存真实同步卡凭证 🔶） |
| 打单发货（面单模板/发货规则/打单工作台/物流商推荐/拣货单打印） | R91 #196/#199、R111 #234 | round111 E2E（13 例） | ✅（电子面单 API 直连取号卡凭证 🔶，页面明确「非电子面单」口径） |
| 物流跟踪（物流商/运单/轨迹 URL；轨迹 Provider 预留） | R91 #196 | 同上 | ✅（轨迹 API 自动更新为预留 🔶，杠杆最低可后置） |

### 7. 消息（客服 / 买家自动消息）

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 客服中心（多平台会话、AI 建议回复、人工确认外发） | customerchat 主线 | 客服 E2E | ✅（真实平台消息通道卡凭证 🔶；绝不自动外发） |
| 话术模板库（分组模板+变量填充+会话插入） | R109 #228 | round109-reply-templates E2E | ✅ |
| 买家自动消息（订单节点规则 × 模板 → 待发草稿工作台，人工回执闭环） | R119 #255 | round119-buyer-messages E2E（10 例）；大回归 v16 实测 | ✅（降级闭环设计：站内草稿+人工发送回执，真实通道到位后可升级自动） |

### 8. 财务对账 → 报表

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 回款记录（登记/CSV 导入/自动对账 未回款/少款/多款/已结清） | R121 #259 | round121-finance E2E（8 例）；后端 finance 单测 | ✅ |
| 费用记账（订单级+店铺月度费用；采购实付价） | R121 #259 | 同上 | ✅ |
| 实算毛利与对账差异工作台 + 对账报表（店铺×月份，CSV 导出） | R121 #259 | 同上 | ✅ |
| 报表（利润/采购/库存三报表 + 经营趋势；多币种本位币折算） | R110 #231、R93 #201、R97 #208 | round110 E2E；升级演练报表口径逐字段比对 | ✅ |

### 9. 横切能力（平台治理 / 工程化）

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 多租户与权限（三角色、店铺 scope、租户治理/清退、自助注册独立租户） | P4 + R82/R89 #192 + R101 #214 + R116 #250 安全批次 | 权限矩阵契约测试（109 端点）；IDOR 55 例回归 | ✅ |
| 移动端（移动首页/底部导航/触屏动线/PWA manifest/移动审单） | R113 #238/#239、R115 #242 | round113 E2E；五档视口硬指标 | ✅ |
| 安全工程（凭据 AES-GCM 加密+轮换、审计链、备份/容灾、依赖审计） | P4/P6 + R116/R117 #245/#247/#248/#250 | 安全审计批次报告；升级/回滚演练记录（upgrade-guide §五） | ✅ |
| 部署/升级/回滚（Docker 全栈、生产 Caddy HTTPS、升级 SOP + 迁移预检、备份恢复演练） | 生产部署线 + R95 #210、R118 #253 | upgrade-guide 演练记录（2026-08-04/05 两次全流程）；production-deployment §八 | ✅（Let's Encrypt 公网签发未实测，属首次上线确认项） |
| 性能收口（万级列表/导出/并发压测、慢查询治理） | R116 导入万行 #249；R122 线1 #261（已合并） | #261 压测报告（`PERF-` 种子，见 docs/development.md） | ✅ |

### 10. R124–R127 增量能力（R128 增补）

预验收包基线为 R123（main `02b6b086`）；此后 R124–R127 新增能力并入本表。#264/#265/#267/#270 已合入（#270 随 #268 分支收口），#266/#268/#269 尚未合入 main（R127 线1 大回归 v18 已给出合并顺序结论：#266 → #269 → #268，见证据列）。

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 768 断点口径互斥（<768 仅底部导航、≥768 仅侧栏；移动「我的」页补报表/异常/告警补偿入口） | R124 线2 / #264（已合并） | `round124-p2.spec.ts`（375/767/768/769/1440 五档互斥）；Docker 三视口实测（768±1px） | ✅ |
| 采购单来源销售订单号透出（Detail 回填 `salesOrderNo`，概览/明细行展示可跳转） | R124 线2 / #264（已合并） | 后端 `TestDetailFillsSalesOrderNo`；同上 E2E | ✅ |
| 生产升级演练复跑（R118 `e9b27309` + 4 万订单/12 万流水存量 → main `314fc1ed`）：从零部署 232s、R119–R122 新表 + #261 索引全部落地（AutoMigrate ~3s）、迁移前后指纹 0 差异、故障注入→pg_restore 恢复→重跑闭环；修正 upgrade-guide 冲突表失实表述 | R124 线2 / #265（已合并） | `docs/upgrade-guide.md` §迁移点表 R119–R122 行与 2026-08-05（R124 复跑）演练记录；演练报告（会话附件） | ✅ |
| 竞品对标复评 v4：16 项矩阵 **超越 3 / 达到 13 / 落后 0**（R118 为超3/达10/落3；4 个纯产品差距经 R119–R124 全部收口，原 3 个落后项确认为纯外部凭证依赖，重分类「凭证待解锁」） | R125 线2 / #266（已合并） | `docs/COMPETITIVE_BENCHMARK_R125.md`（Docker 全栈实测逐项复评）；实测截图（PR #266 评论） | ✅ |
| 安全审计季度复跑：4 组 P1 数据级越权修复（规则 `shopIds` 越权写入、dry-run 跨店铺数据泄露、自动化执行日志越权读/重放、买家消息草稿越权）+ 2 项加固（`APP_ENV=prod` 归一、CSV 导出公式注入中和）+ 5 个新增权限矩阵契约测试 | R125 线1 / #267（已合并） | permmatrix `automation_message_scope_test.go` 等（双租户×三角色）；`docs/permission-matrix.md` round125 章节 | ✅ |
| R125 审计 P2×4 收口（依赖补丁 audit 23→21、CSV 导入控制字符/超宽表头校验、执行日志 `shop_id` 冗余列+scope 改造、hono 补丁；跨 major 依赖升级项列表交老板决策） | R126 线1 / #269（已合并） | PR #269 全套门禁记录（go 97 包/契约/前端/E2E 31 例/Docker 冒烟）；`govulncheck` 前后 0 命中 | ✅ |
| 自动化订单规则新动作：`apply_shipping_rule` 自动应用发货规则（recommend/apply 两模式，计划物流商落单、guarded 不覆盖人工）与 `assign_warehouse` 自动分仓（default_warehouse/stock_first 策略，联动 R112 按仓扣减；库存不足失败留痕可重试）；沿用 DedupKey 幂等/审单安全边界/dry-run/店铺 scope | R126 线2 / #268（已合并） | `round126-auto-actions.spec.ts`（11 例五档视口）；Docker 全栈实测 7/7（DEMO-AT-1004 真实触发、AT-1201 正样本、AT-1202 负样本，PR #268 评论）；大回归 v18 实弹复验 | ✅ |
| R127 交叉 QA P1×2 修复：失败留痕「本轮尝试 N 次」口径与累计列区分；订单详情透出计划物流商/分配仓库展示位（DetailDTO + 前端两项） | R127 线2 / #270（已合入 #268 分支，随 #268 合并） | `TestOrderDetailExposesAutomationFields`；QA 全链路走查录屏（会话附件） | ✅ |
| 大回归 v18（大批合入前集成预演：main 叠加 #266/#269/#268）：全套门禁全绿（go 97 包+集成+redis+securitytests、契约 15、前端 322、E2E 296/1/3——唯一失败定级 P2 flake 单跑全绿）、Docker 全栈 17/18；结论 0 P0/P1、按 #266→#269→#268 合入；另判定 #245/#247/#248 内容已在 main，PR 冗余建议关闭 | R127 线1 | PR #268 评论（v18 报告与截图） | ✅（#266/#269/#268 已按序合入 main） |

### 11. R128–R131 增量能力（R132 增补）

#266/#268/#269/#270 已按大回归 v18 结论合入 main（§一/10 相应条目已转 ✅）；R128–R131 新增交付并入本表。#275/#277 尚未合入 main（R131 线2 大回归 v19 已对 #275 给出可合入结论，见 PR #272 评论）。

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| demo seed 第二演示业务租户（`demo_tenant2_admin@trademind.local`，1 店铺/2 订单/发货规则/自动化规则，双租户隔离开箱可测；clean/verify 零残留、幂等）+ 自动化操作日志中文化 + E2E 下拉竞态收口 | R128 线1 / #272（已合并） | `fulldemo_round128_test.go`；大回归 v19 Docker 实测（T2 隔离零泄漏、主租户搜 DEMO-T2 空态） | ✅ |
| UX 视觉复核 v8：v7 P2 四项全收口无回退，五档视口 + 三角色 + tenant2 隔离抽查硬指标全零；无 P0/P1，P2×2 登记（均由 #275 收口） | R129 线2 / #273（已合并） | `docs/ux-review/UX_REVIEW_V8_REPORT.md` | ✅ |
| 订单列表订单号/客户筛选修复（`orderNo`/`customerName` 接入 URL query 单一来源链路，深链/重置/刷新持久） | R129 线1 / #274（已合并） | `round129-order-filters.spec.ts`（3 例）；大回归 v19 实测（`orderNo=DEMO-AT-1004` 点查询/URL 回写/深链） | ✅ |
| 财务对账/利润报表聚合下推 SQL GROUP BY（R122 性能 P2 收口，页面数值与 psql 直算逐项一致） | R130 线1 / #276（已合并） | finance/reports 单测；大回归 v19 报表数值 SQL 对照（对账 16/13/1/1/1、利润 16 单毛利 -8,250.07） | ✅ |
| UX v8 P2 收口：执行日志 demo seed 样本（DEMO-AT-1301 成功 / 1302 规则未命中跳过 / 1303 库存不足失败）+ 操作日志操作类型/资源列中文映射（Tooltip 保留原始值） | R130 线2 / #275 | 大回归 v19 实测（operator 视角非空、中文映射）；`operationLogLabels.test.ts` | ✅ #275 已合并 |
| 对账/毛利 CSV 导出改为全量（keyset 分批加载，不再受单页上限截断） | R131 线1 / #277 | finance/reports 后端单测（PR #277） | ✅ #277 已合并 |
| 大回归 v19（R128–R130 合入前集成预演：main 叠加 #272→#275→#276）：全套门禁全绿、全量 E2E 298/3/3（3 失败为 smoke 首批 8s 超时环境性 flake，复跑全绿）、Docker 全栈走查 8 组全过；结论 0 P0/P1、按序合入；P2×2（DEMO-AT-1004 库存扣减列口径、smoke 首批超时）由 R132 线1 收口 | R131 线2 | PR #272 评论（v19 报告与截图） | ✅（#272/#275/#276 均已合入） |

### 12. R132–R135 增量能力（R136 增补）

#278/#279/#282 已合入 main；#280（竞品复评 v5 归档）/#281（R133 P2×7 收口）已按大回归 v20 结论（R135 线2，PR #281 评论）依序合入 main，对应条目已转 ✅。v20 P2×3 由 R136 线1（#283，已合并）收口。

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 大回归 v19 P2 收口（DEMO-AT-1004 补 `order_item_sku_matches` matched 行、smoke 首屏放宽 30s）+ 验收包增补 R128–R131（§一/11 即该轮产出） | R132 线1 / #278（已合并） | round119 seed 测试补 match 行断言；大回归 v20 Docker 实测（DEMO-AT-1004 规格匹配 1/1，不再显示「SKU 未就绪」） | ✅ |
| 生产升级演练季度复跑（R124 基线 `314fc1ed` + 存量 → main `eb626bd9`）：从零部署 251s、R125–R131 新列（自动化新动作参数 / planned_carrier / assigned_warehouse / 执行日志 shop_id 回填 2 万行 0 偏差）落地、指纹 0 差异、故障注入 → pg_restore 恢复闭环；upgrade-guide 迁移点表补 R125–R131 行 | R132 线2 / #279（已合并） | `docs/upgrade-guide.md` §五 2026-08-06（R132 复跑）演练记录；演练报告（会话附件） | ✅ |
| 安全审计 + 交叉 QA 复跑（R133 两线）产出 P2×7 → R134 线1 收口批次：finance 聚合 tenant 防御纵深（跨租户同 order_id 注入回归测试）、对账 CSV Currency 列补 `csvsafe.Cell`、`@umijs/max` 构建链 advisories 登记待决策（`DEPENDENCY_ADVISORIES_R134.md`）、E2E globalSetup 预热收口冷启动 flake、readonly 草稿详情前端门控、采集规则报错中文化（30+ 映射）、草稿来源平台枚举中文直出 | R133 两线（报告见 PR 评论）→ R134 线1 / #281 | `TestReconciliationCrossTenantAggIsolation`、`TestReconciliationCSVCurrencyEscaped`、`localize_test.go`、`round134-p2.spec.ts`；大回归 v20 修复点逐项抽查（PR #281 评论） | ✅ #281 已合并 |
| 竞品对标复评 v5：16 项矩阵维持 **超越 3 / 达到 13 / 落后 0**，8 轮维护期实测零回退；对照竞品 2026 近期更新未发现新的结构性缺口；发现项 2 个（非阻断，均已由 R135 #282 收口） | R134 线2 / #280 | `docs/COMPETITIVE_BENCHMARK_R134.md`（Docker 全栈实测） | ✅ #280 已合并 |
| 订单标签闭环：`order_tags`/`order_tag_links` 表 + 标签管理 API/页面、订单列表标签列与 `?tagId=` 筛选（URL 单一来源）、批量/详情打标、自动化规则新动作 `add_tag`（沿用幂等/安全闸门/scope）；R134 复评发现项收口（SKU 提示口径、demo 采集规则样本） | R135 线1 / #282（已合并） | `round135-order-tags.spec.ts`（13 例）；Docker 全栈实测（PR #282）；`fulldemo_round135_*` seed 测试 | ✅ |
| 大回归 v20（R132–R134 合入前集成预演：main 叠加 #280→#281）：全套门禁全绿、全量 E2E 306 例 303 passed / 0 failed / 3 skipped、Docker 全栈 8 组走查全过；结论 0 P0/P1、按序合入；P2×3（对账 seed 行数、operator 触发正样本、CSV 注入 E2E 层验证）由 R136 线1 收口 | R135 线2 | PR #281 评论（v20 报告与截图） | ✅ |
| v20 P2×3 收口：demo seed 对账数据补足 25+ 行（结清/少款/多款/未回款样本齐备，UI 分页/合计与 #277 跨页全量导出可演示）、operator 授权店自动化正样本 `DEMO-AT-1005`（operator 可自行标记付款真实触发采购单生成）、对账 CSV Currency 注入防护补 HTTP E2E 层验证（真实路由+鉴权+CSV 导出全链路） | R136 线1 / #283（已合并） | `fulldemo_round136_test.go`；`TestRound136ReconciliationCSVCurrencyInjectionE2E`（integration，实库）；Docker 全栈实测（PR #283） | ✅ |

### 13. R136–R140 增量能力（R141 增补）

#284/#285/#286/#288 已合入 main；#287（对象存储备份上传）经 R139 安全审计 4 条 S3 加固（R140 线1 并入该 PR）后已合入 main（合并提交 `99fd2e7d`）。本节全部条目为 ✅。

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| UX 视觉复核 v9 收口：v8 遗留 P2×2 无回退，#282 标签全动线/#277 CSV 导出/#275 日志中文化/#281 readonly 门控走查通过、硬指标全零；随报告修复 P1×1（对账差异 CSV「平台」列英文枚举 → `opslabels.PlatformLabel` 中文化）+ P2×2（操作日志 `order_tag.*` 动作/资源中文映射、标签页时间统一 `formatDateTime`） | R136 线2 / #284（已合并） | `docs/ux-review/UX_REVIEW_V9_REPORT.md`；`TestReconciliationCSVPlatformLocalized`；`operationLogs` 单测 | ✅ |
| 报表 CSV「未折算」显式口径（UX v9 P2-3 收口）：非 CNY 无手工汇率时，利润报表/经营逐日/对账报表/差异工作台四类 CSV 的折算与本位币列由留空统一为显式「未折算」，与页面渲染口径一致；店铺月度费用「缺记录」仍留空、可折算真实 0 仍输出 `0.00` 不误标 | R137 线1 / #285（已合并） | `TestFinanceCSVUnconvertedExplicit`、`TestProfitCSVExport`、`TestExportDailyStatsCSV*`；`docs/api.md` 四处导出说明同步 | ✅ |
| 生产升级演练季度复检（R132 后第 5 轮）：从零部署 234s / 从零到登录 251s（<15 分钟达标）；旧版本 `eb626bd9` + 双业务租户存量 → main `516e6863` 升级指纹 0 差异、shop_id 回填 0 偏差、`pg_restore` 备份恢复闭环 + 迁移幂等重跑；迁移点表补 R135（#282）行 | R137 线2 / #286（已合并，纯文档） | `docs/upgrade-guide.md` §五 2026-08-06（R137 季度复检）演练记录；`docs/production-launch-checklist.md` §五计时 | ✅ |
| 备份对象存储上传（收掉最后一个部署债）：`backupstore` S3 兼容 Provider（AWS S3 / MinIO / 阿里 OSS），备份完成自动上传（有界重试、失败落库可重试）+ `BACKUP_OBJECT_RETENTION_COUNT` 保留策略 + 本地缺失自动取回校验 SHA-256；`BACKUP_S3_*` 留空为降级模式（仅本地路径不阻塞部署）；含 R139 审计 4 条 S3 加固（AK/SK 落库脱敏、endpoint 校验拒回环/元数据地址、清理收窄防整桶删除、取回落地路径 containment） | R138 线1 + R140 线1 / #287（已合并） | backupstore 假 S3 服务测试与加固回归测试；`round138-backup-upload.spec.ts`；MinIO 全链路实测（PR #287 评论）；permmatrix/tenant-zero 安全测试 | ✅ |
| R139 线1 P2 收口：深分页静默失败 → `TmProTable` 统一 `pagination_offset_too_deep` 中文可读提示（全站列表页共享收口）；订单详情未匹配行「本地规格编号」不再回显录入 `sku_code`，未绑定行显式「未绑定」 | R139 线1 / #288（已合并） | `round139-p2.spec.ts`；`paginationError`/`localSkuCodeDisplay` 单测；Docker 实测（PR #288 评论） | ✅ |

### 14. R144–R147 增量能力（R148 增补）

#294/#295（MCP 只读入口）、#296（实时经营大屏）、#297（R146 QA/中文化收口）、#298（MCP 安全加固）、#299/#300（R147 杂项收口）均已合入 main。本节全部条目为 ✅。

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| MCP 只读入口 + token 治理（竞品对标 R143 差异化第一杠杆）：官方 Go SDK Streamable HTTP `POST /api/mcp`，4 个只读工具（orders_query / inventory_query / report_summary / exceptions_pending）强制租户 scope；租户级只读 token（`sp_mcp_ro_` 前缀，明文仅创建展示一次、库存 SHA-256 哈希、脱敏列表、幂等吊销）；令牌桶限流 + fail-closed 鉴权 + 输出脱敏；管理页 `/settings/mcp-tokens`；R145 线2 安全交叉审查 P1×3 已修复（#295） | R144 线1 / #294 + R145 线2 / #295（已合并） | `mcptoken/service_test.go`、`mcpserver/server_test.go`（MCP SDK 真实客户端握手）；`round144-mcp-tokens.spec.ts`；权限矩阵登记；双租户 Docker 实测（PR #295 评论）；`docs/mcp.md` | ✅ |
| 实时经营大屏（差异化第二杠杆）：`GET /api/v1/dashboard/screen` 单次 SQL 聚合（今日 KPI / 待办五类 / 近 7 天漏斗 / 近 24h 趋势 / 告警滚动，scope 不回退、operator 空授权 fail closed）；`/dashboard/screen` 深色大屏页（KPI 大数字自适应、可切浅色、15/30/60s 轮询、全屏投屏、1920→768 五档不溢出） | R145 线1 / #296（已合并） | `screen_test.go`；`round145-dashboard-screen.spec.ts`（9 例，五档视口）；契约登记 | ✅ |
| MCP 安全加固（R145 P2×3 收口）：token 可选过期（`expiresInDays` 1–730，过期即 401 fail-closed，管理页过期/即将过期标识）；工具调用逐次审计（`mcp_tool_call_logs` 不落查询参数与结果，管理页审计卡片 + `GET /api/v1/mcp/audit-logs`）；限流多副本口径（Redis Lua 令牌桶共享额度，不可用自动降级进程内不 fail-open） | R146 线1 / #298（已合并） | `mcptoken/expiry_test.go`、`mcpaudit/service_test.go`、`mcpserver/audit_test.go`、`ratelimit/redis_test.go`；契约 114→116；双租户 Docker 实测（PR #298） | ✅ |
| R146 QA 收口 + R147 杂项收口：MCP 列表时间格式化与移动端表格内滚、操作日志 MCP 动作/资源中文映射（#297）；裸枚举中文化收口（采购单详情支付状态/渠道、订单异常处理状态、客服会话 role/source/type）+ 采购支付渠道 alipay/bank/other 映射补齐（#300）；MCP token 纳入 demo seed（DEMO-MCP 样本 + 审计样本 + clean/verify 零残留） | R146 QA / #297 + R147 线1 / #299、#300（已合并） | `statusEnumLabels.test.ts`、`fulldemo_round147_test.go`；Docker 实测（PR #299） | ✅ |
| 大回归 v22 / v23（R144–R147 合入面集成回归证据）：全套门禁 + Docker 全栈走查；v23 P2（MCP 管理页「过期时间」与审计「时间」列 ISO 直出缺 `formatDateTime`）已由 R148 线1 收口（本轮 PR，含 E2E 回归用例） | 大回归 v22 / v23（质量回归轮报告，证据不入库）→ R148 线1 收口 | `round144-mcp-tokens.spec.ts` 新增 ISO 时间格式化用例；R148 Docker 实跑记录（DEMO_SCRIPT 实跑验证记录） | ✅ |

### 15. R148–R152 增量能力（R153 增补）

#301（验收包 R144–R147 增量）/#302（R148 安全审计复跑：权限矩阵 registry 漂移修复 + `docs/SECURITY_AUDIT_R148.md` 归档）已合入 main；~~#303/#304/#305/#306/#307/#308/#309 尚未合入 main~~——R158 时点 #303–#309 已全部合入 main，本节原 ⏳ 条目已全部转 ✅（R158 线2 收口）。

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 安全审计季度复跑（R148）+ P2 批次收口：审计报告归档与权限矩阵 registry 漂移修复（#302，已合并）；P2 收口批次——MCP 审计写失败 fail-closed（审计完整性优先，只读工具可安全重试）、MCP token 上限竞态收口（事务内检查 + `pg_advisory_xact_lock` 跨副本串行化，30 并发恰 20 成功回归测试）、大屏 today 口径对齐 `ProfitReportFiltered`（过滤只收窄不放宽）、非法 shopId 显式 400、权限矩阵 CI 漂移预警（`pnpm test:permmatrix` 入 CI） | R148 线2 / #302（已合并）+ R149 线1 / #303 | `docs/SECURITY_AUDIT_R148.md`；mcptoken 并发回归测试（SQLite/PostgreSQL）；双租户 Docker 实测（PR #303）；operator 管 MCP token 是否收紧 admin-only 已登记待老板拍板 | ✅ #303 已合并 |
| 生产升级演练季度复跑（R149，R142 基线 `99fd2e7d` + 存量 → main `7f5645c1`）：从零部署到登录 350s（升级部署 248s）、`--pre-upgrade-check` 0 行、指纹逐项 0 差异、MCP token 创建/调用/审计与大屏三角色升级后可用、MinIO 自签 https 与本地降级、故障→恢复→重跑闭环；upgrade-guide 迁移点表补 R144–R148 行（`mcp_api_tokens`/`mcp_tool_call_logs`）；`.env.prod.example` 补 MCP 配置块 | R149 线2 / #304（纯文档） | `docs/upgrade-guide.md` §五 R149 演练记录（随 #304）；演练报告（会话附件，证据不入库） | ✅ #304 已合并 |
| deploy-prod.sh 重跑保 compose override 挂载（R149 演练 P2 收口）：`docker-compose.prod.override.yml` 存在即自动叠加 + `COMPOSE_OVERRIDE_FILES` 显式多 override（缺失 fail-fast），重跑/升级/回滚不再丢挂载（如 `BACKUP_S3_CA_BUNDLE` 自签 CA）；附审计小项巡检（#300 alipay 中文化确认无残留、前端工具链 22 条 advisories 逐项评估均需跨 major 登记不动、MCP/大屏 E2E repeat-each 3 轮零 flake） | R150 线1 / #305 | localhost 生产全栈重跑演练（首跑/重跑/基线对照/fail-fast 四场景，PR #305，证据外置）；`docs/production-deployment.md`/`docs/env.md` 同步 | ✅ #305 已合并 |
| 竞品对标复评 v7（R151）：16 项矩阵维持 **超越 3 / 达到 13 / 落后 0**，8 轮差异化期零回退；MCP 只读入口协议级端到端实测坐实「超越」（店小秘/马帮均无）、实时大屏对店小秘 2026-06 大屏「达到/齐平」不宣称超越；确认结构性差距候补：竞品均有开放 API 体系 → R152 第一杠杆 | R151 线2 / #306（纯文档） | `docs/COMPETITIVE_BENCHMARK_R151.md`（随 #306，Docker 全栈实测） | ✅ #306 已合并 |
| 大回归 v24 P2×2 收口：MCP 审计卡片轻刷新时序（请求序号护栏防迟到旧响应覆盖，含并发乱序回归单测）、自动化执行日志 readonly 店铺 scope 空态引导（`useListEmptyLocale` 权限范围提示）；`docs/mcp.md` 审计口径改为 fail-closed（依赖 #303 合入生效，PR 内已作避让说明） | R151 线1 / #307 | `McpTokens.test.tsx` 时序回归；round119/round144 E2E 20 例（含新增空态 2 例） | ✅ #307 已合并 |
| 对外开放 REST API 只读入口 `GET /api/open/v1/*`（竞品复评 v7 路线①第一杠杆）：订单列表/详情、库存、报表摘要、异常待办 5 端点，仅 GET 无写操作；token 治理复用 MCP（`purpose` 字段 mcp/openapi/both 入口选择器，存量 token 攻击面不变宽）、哈希/过期/吊销/每租户 20 上限/三层限流/逐次审计沿 MCP 口径；共享只读查询层 `readonlyquery`（MCP 4 工具与开放 API 共用防漂移）；OpenAPI 3 规范 + spec↔实现双向契约测试；管理页更名「只读 API 接入」补用途列；`docs/open-api.md` | R152 线1 / #308 | openapi/readonlyquery/mcptoken 单测与 `spec_test.go`；契约 17 例；双租户 Docker 实测（跨租户 401/404、限流 429，PR #308）；demo seed 开放 API 用途 token 样本 | ✅ #308 已合并（与 #309 合并后契约受保护端点 122） |
| 买家消息多语言模板（竞品复评 v7 路线②）：话术模板 `default_language` + 语言变体表（15 语种可扩展，历史数据零迁移）；草稿语言推断链 order_country→shop_language→platform→模板默认 fallback（缺变体标 `no_variant`，全程可解释）；工作台切换语言重生成端点（仅 pending、readonly 403、绝不自动外发口径不变）；demo seed 英/西/葡变体与 DEMO-BM-1005（US→en）/1006（BR→pt）/1001（fallback）正负样本 | R152 线2 / #309 | `template_lang_test.go` 等后端单测；`round152-multilang-templates.spec.ts` 8/8；三角色 Docker 实测（录屏见 PR #309 验收评论）；契约受保护端点 117 | ✅ #309 已合并 |

### 16. R153–R157 增量能力（R158 增补）

#310（验收包 R148–R152 增量）/#313（permmatrix OpenAPI harness 修复）/#315（R155 线1 v25 P2×4 收口，含 #307 时序补验 2 用例）/#316（R155 线2 生产配置面）已合入 main；#311/#312/#317/#318 尚未合入 main，对应行按既有口径标 ⏳ 待合并（能力已交付并过门禁）。

| 能力点 | 实现轮次 / PR | 验证证据 | 状态 |
| --- | --- | --- | --- |
| 开放 API / MCP 安全加固三项行为变更（R153 交叉安全审查 P1×3）：①被禁用/清退租户的 MCP/开放 API token 立即失效（`mcptoken.AuthenticateFor` 统一补 `auth.EnsureTenantActive`，两入口 401，状态不可判定按失败处理）；②新增 `TRUSTED_PROXIES`（逗号分隔 IP/CIDR，默认空=不信任任何代理、客户端 IP 取 TCP peer 地址，杜绝伪造 `X-Forwarded-For` 绕过每 IP 鉴权失败预算/限流，非法值 fail-fast）；③开放 API 审计 fail-closed（响应先缓存、审计行落库成功才输出，审计不可用返回 500，与 MCP #303 口径对齐） | R153 线1 / #311 | `mcptoken/tenantstate_test.go`、`openapi/audit_failclosed_test.go`；双租户三角色 Docker 实测修复前后对比（PR #311）；`.env.example`/`.env.docker.example`/`docs/env.md` 同步 | ⏳ #311 待合并 |
| 入口级审计 + 分页口径（R153 安全审查 P2×6 批次收口）：MCP/开放 API 401/429 写入口级审计行（工具名 `mcp:auth`/`openapi:auth`，新状态 `auth_failed`/`rate_limited`；未认证来源记租户 0，不落 token 内容；按 来源×状态×工具 每分钟节流防审计表放大；管理页审计筛选同步）；开放 API 分页非法参数统一 400/40001（orders/inventory/exceptions 三端点，与日期口径一致）；both 双入口双份额度（产品决策）与 operator regenerate 404（防资源存在性泄露）登记不改；token 上限竞态与 MCP fail-closed 已随 #303 闭合 | R154 线1 / #312 | `openapi/p2_hardening_test.go`、`mcpserver/authaudit_test.go`、`mcpaudit/throttle_test.go`；Docker 实测（401 落 auth_failed、429 落 rate_limited、`page=abc` 400，PR #312）；#311 三项行为变更文档收口随本 PR | ⏳ #312 待合并 |
| v25 P2×4 收口 + 生产演练新配置面：`/api/health` 404 核对为预期（规范路径 `/health` 等，登记不改）、登录 body `account` 字段口径文档化、#307 审计卡时序补验 2 确定性用例、#308 `purpose` 签名冲突预案登记；`.env.prod.example` 补 `TRUSTED_PROXIES`/`OPENAPI_*` 生产配置块 + `docs/production-deployment.md` 客户端 IP 口径说明（Caddy 单入口代理下需显式配置信任代理）；permmatrix harness 启用 OpenAPI（#313，修复 5 条 `/api/open/v1/*` 矩阵条目 CI 误判） | R155 线1 / #315 + R155 线2 / #316 + R154 / #313（均已合并） | `docs/api.md` 登录 body 说明；`McpTokens.test.tsx` 补验 2 用例；`.env.prod.example`；permmatrix 门禁（`pnpm test:permmatrix` 全绿） | ✅ |
| MCP 错误码规范化 + 运维杂项（R155 登记 P2 收口）：MCP 审计 fail-closed 拒绝从 go-sdk `WireError{Code:0}`（非法 JSON-RPC 码）改规范 `-32603 internal error`（开放 API 侧 500/50000 本就规范，口径一致）；deploy-prod `--pre-upgrade-check` 备份目录不可创建/不可写启动即清晰报错（提示 root 或 `BACKUP_DIR` 覆盖）；demo clean 不覆盖非 DEMO- 前缀 token 核实登记（SKILL 常见坑，不扩大删除面防误删真实 token）；#308 purpose 签名冲突预案落地 | R156 线1 / #317 | `TestAuditWriteFailureRejectsToolCall` 补 -32603 断言（改回 code 0 测试红）；Docker 实测制造审计库故障返 `-32603`/HTTP 500 `50000`、恢复后调用恢复（PR #317 评论）；非 root deploy 场景实测报错文案 | ⏳ #317 待合并 |
| 大屏多币种折算显式口径 + 租户级自定义指标（竞品复评 v7 建议③）：`/dashboard/screen` 今日 KPI 新增 `today.unconvertedRevenue`（无汇率币种原币金额显式列出）与 `today.convertedCurrencies`，前端折算口径角标 tooltip 与「未折算（不计入合计）：EUR 88.00」展示，毛利缺汇率仍缺省提示不伪造；新增 `GET/PUT /dashboard/screen/config` 租户级卡片配置（8 张卡启用/排序，PUT 仅 `settings.manage`，readonly/operator 403；未知/重复 key、全禁用 400；禁用卡跳过对应聚合性能只减不增）；demo seed 多币种样本 `DEMO-FX-USD-0001`（可折算）/`DEMO-FX-EUR-0001`（未折算演示）；契约 116→118 | R156 线2 / #318 | `screen_config_test.go`、`fulldemo_round156_test.go`、`round156-dashboard-screen-config.spec.ts` 17 例（五档+1920 视口）；R157 线1 交叉 QA Docker 双租户三角色实测 13/13 无 P0/P1（含 EUR=7.8 配置后合计精确 8,021.96 与利润报表同口径、卡片配置持久化与权限边界，PR #318 评论）；实测发现 seed 汇率写错租户缺陷已随分支修复（`e6375d33`）并补回归断言 | ⏳ #318 待合并 |
| R157 集成预演（R153–R156 合入面）：`integration/r157-regression-v26` 分支按序叠加 #311→#312→#315→#317→#318 验证合并期冲突可解（PROGRESS/契约计数 union，合并后契约受保护端点 124）；R157 线1 交叉 QA 13/13 通过、无 P0/P1 | R157（质量回归轮，报告与证据不入库） | `integration/r157-regression-v26` 分支合并记录；R157 交叉 QA 报告与截图（PR #318 评论） | ✅（#315 已合入；其余按 §五 建议顺序合入） |

## 二、外部凭证依赖项清单（按杠杆排序，含降级路径）

全部依赖项均已做到「代码就绪 + 明确降级 + 凭证到位即插队真实化」，符合 CHARTER §3.7 资源缺口不阻塞：

| 序 | 凭证 | 解锁能力 | 当前降级路径（已实现） |
| --- | --- | --- | --- |
| ① | 抖店开放平台应用 + 测试店铺 | 真实刊登/订单/库存/客服 E2E | 站内草稿闭环 + `blocked_by_real_credentials` 显式态；E2E 清单已备（docs/DOUYIN_E2E_*） |
| ② | 电子面单账号（快递100 / 菜鸟任一） | 打单取号闭环 | 面单模板+发货规则+打单工作台，人工面单，页面明示「非电子面单」 |
| ③ | 1688 采购 API 资质 | 采购直连下单 | 人工下单过渡模式：CSV 导出 → 1688 人工下单 → 回填单号（operations-manual） |
| ④ | TikTok / Shopee 开发者账号 | 第二平台真实化 | adapter+签名/OAuth 就绪，`local_draft_only` 降级 |
| ⑤ | 物流轨迹 API（17TRACK 等） | 轨迹自动更新 | 手动轨迹 URL；Provider 槽位预留 |
| ⑥ | SMTP 邮箱（注册验证码） | 自助注册邮箱验证 | `AUTH_REGISTER_SKIP_EMAIL_VERIFY` 显式开关或 Redis 注入验证码（仅测试环境） |
| ⑦ | 生产服务器 + 域名 | 公网 HTTPS 上线 + Let's Encrypt 正式签发 | Docker 全栈本地/内网可运行；Caddy 自签 HTTPS 已验证（production-deployment §八） |

## 三、验证证据索引

- 大回归：v14（R114 轮，`docs/PROGRESS.md` 第 114 轮）、v16（R121 轮，Docker 全栈+seed 全链路实测，双租户三角色三视口硬指标全零）。
- UX 视觉复核：v6（R114）、v7（R121，`docs/ux-review/UX_REVIEW_V7_REPORT.md`，无 P0，P2×4 已在 R122 #260 收口）。
- E2E：CI `admin-e2e` 门禁 268 passed / 3 skipped（v16 口径）；契约测试 109 端点（R121 口径）。
- 升级/回滚演练：`docs/upgrade-guide.md` §五（2026-08-04、2026-08-05 两次全流程，含故意制造迁移中断+备份恢复）。
- 安全：权限矩阵契约测试、IDOR 矩阵、R109–R117 每轮安全批次（#244–#250 等）。
- 本轮演示动线实跑：见 [DEMO_SCRIPT.md](DEMO_SCRIPT.md)「实跑验证记录」。
- R144–R147 合入面集成回归：大回归 v22 / v23（质量回归轮报告，证据不入库，见对应会话/PR 评论）；v23 P2（MCP 管理页时间列 ISO 直出）已由 R148 线1 收口。
- R148–R152 合入面：大回归 v24（报告登记于 PR #303 评论，证据不入库）；v24 P2×2 由 R151 线1（#307）收口；R153 线2 Docker 全栈实跑记录见 [DEMO_SCRIPT.md](DEMO_SCRIPT.md)「实跑验证记录」2026-08-07（R153 线2）条目。
- R153–R157 合入面：集成预演分支 `integration/r157-regression-v26`（main 按序叠加 #311→#312→#315→#317→#318）；R157 线1 交叉 QA 13/13 无 P0/P1（报告与截图登记于 PR #318 评论，证据不入库）；R158 线2 Docker 全栈实跑记录见 [DEMO_SCRIPT.md](DEMO_SCRIPT.md)「实跑验证记录」2026-08-07（R158 线2）条目。

## 四、竞品对照结论（R125 复评 v4 为准，全文见 [../COMPETITIVE_BENCHMARK_R125.md](../COMPETITIVE_BENCHMARK_R125.md)；R118 版见 [../COMPETITIVE_BENCHMARK_R118.md](../COMPETITIVE_BENCHMARK_R118.md)）

**R136 增补（R134 复评 v5，#280）**：16 项矩阵 Docker 全栈实测复评维持 **超越 3 / 达到 13 / 落后 0**（vs 店小秘/马帮，补充 AutoDS，含竞品 2026 年近期更新调研），8 轮维护期实测零回退，第 6 项订单管理坐实为超越；未发现新的结构性产品缺口，新差距均为凭证依赖或可选增量。结论：维护期可收束，建议「等凭证为主 + 小步差异化」双轨，预验收包可按沙箱口径正式提交。全文见 [../COMPETITIVE_BENCHMARK_R134.md](../COMPETITIVE_BENCHMARK_R134.md)。

**R128 增补（R125 复评 v4，#266）**：16 项矩阵 Docker 全栈实测复评结论 **超越 3 / 达到 13 / 落后 0**；R118 识别的 4 个纯产品差距（自动化订单规则、买家自动消息、选品数据面、财务对账）经 R119–R124 交付全部实测收口；原 3 个落后项（真实刊登连通、电子面单、1688 直采）确认为纯外部凭证依赖，重分类「凭证待解锁」（即 §二 ①②③）。**正式验收前结论：真实抖店 E2E 前，纯产品侧无必须补齐项。** 以下为 R123 时点的 R118 复评存档口径：


- 16 项能力矩阵（+2 增项）：**超越 3（AI 商品运营、多租户权限、数据安全/自托管，另增项安全工程体系）/ 达到 10 / 落后 3**（对标店小秘 / 马帮 / AutoDS，R118 逐项以 main 代码重验）。
- R119–R122 在 R118 之后又完成：自动化订单规则（矩阵第 6 项「自动化规则弱于马帮」短板补齐）、买家自动消息（矩阵第 12 项缺口补齐）、选品数据面（第 3 项增强）、财务对账（第 11 项深化）——**纯产品侧结构性落后项为 0**。
- 剩余落后 3 项（真实刊登连通、电子面单直连、1688 采购直连）**全部且仅仅卡外部凭证**（见 §二 ①②③），属 CHARTER §3.7 资源缺口而非能力缺口。
- **结论：达到或超越可比竞品的纯产品能力门槛，符合 CHARTER §7 预验收提交条件。** 正式验收建议在凭证 ①（抖店）注入并跑通一轮真实 E2E 后进行；若老板接受「凭证类以沙箱/降级口径验收」，本预验收包即可作为验收包。

## 五、遗留与建议

1. ~~#261（R122 性能收口 v2）待合并~~——已合入 main（`60e09b19`），本表 §一/9「性能收口」已转 ✅。
2. UX v7 P2 清单已全部在 R122 #260 收口；当前无已知 open P0/P1。
3. ~~建议下一轮（R124/R125）：凭证 ① 到位后插队抖店真实 E2E；否则直接进入老板验收轮~~——R124–R127 已完成断点收口、升级演练复跑、竞品复评 v4（落后 0）、安全审计+P2 收口与自动化动作面扩展（见 §一/10）。
4. ~~R128 时点待办：按大回归 v18 结论将 #266 → #269 → #268 依序合入 main~~——已完成（§一/10 三个 ⏳ 条目已转 ✅）；#245/#247/#248 内容已在 main，仍建议直接关闭冗余 PR。
5. 凭证 ①（抖店）注入后插队真实 E2E 仍为正式验收前唯一外部前置项；其余口径同 §四结论。
6. ~~R132 时点待办：按大回归 v19 结论合入 #275；#277（CSV 全量导出）合入后 §一/11 两个 ⏳ 条目转 ✅~~——已完成（#275/#277 已合入 main，§一/11 两条已转 ✅）。
7. ~~R136 时点待办：按大回归 v20 结论将 #280 → #281 依序合入 main~~——已完成（§一/12 两个 ⏳ 条目已转 ✅）；`@umijs/max` 构建链 advisories 跨 major 升级仍登记待老板决策（`DEPENDENCY_ADVISORIES_R134.md`）。
8. R141 时点：R136–R140 交付（#283–#288，含 #287 备份对象存储）已全部合入 main，无 open ⏳ 条目；凭证 ①（抖店）注入后插队真实 E2E 仍为正式验收前唯一外部前置项。备份对象存储生产建议 crontab + `BACKUP_S3_*` 上传双路径同时启用（production-launch-checklist §说明）。
9. R148 时点：R144–R147 交付（#294–#300，MCP 只读入口+token 治理、实时经营大屏、MCP 安全加固、杂项收口）已全部合入 main，无 open ⏳ 条目（见 §一/14）；大回归 v23 P2（MCP 时间列 ISO 直出）已由 R148 线1 收口。凭证 ①（抖店）仍为正式验收前唯一外部前置项。
10. ~~R153 时点待办~~——R158 时点 #303–#309 已全部合入 main（§一/15 七个 ⏳ 条目已全部转 ✅），以下为 R153 时点存档口径：§一/15 七条中 #303/#304/#305/#306/#307/#308/#309 均为 ⏳ 待合并（#301/#302 已合入）；#307 的 `docs/mcp.md` fail-closed 口径依赖 #303 先行合入。合并顺序建议 #303 → #304 → #305 → #306 → #307 → #308 → #309（#303 为 #307 文档口径前置；#308 与 #309 在 `docs/module-map.md`、`docs/progress/R152.md`、`tests/contracts/api-contracts.test.ts` 存在可解冲突，后合入者需手工解决，契约受保护端点数合并后为 122；其余无文件级冲突）。operator 管理 MCP token 是否收紧 admin-only、前端工具链跨 major 依赖升级（`DEPENDENCY_ADVISORIES_R134.md` + #305 登记）仍待老板决策。凭证 ①（抖店）仍为正式验收前唯一外部前置项。
11. R158 时点待办：§一/16 中 #311/#312/#317/#318 为 ⏳ 待合并。#312（fix/round154-audit-p2）已携带 #311 三项行为变更内容、#317（fix/round156-misc）已携带 #312 内容，建议合并顺序 #312 → #317 → #318（各 PR 与 main 的 PROGRESS/契约计数冲突可按 union 解决，集成预演 `integration/r157-regression-v26` 已验证可解，合并后契约受保护端点 124）；#311 的 base 为已合入 main 的 feat/round152-open-api 且有冲突，其内容随 #312 合入后可直接关闭。both 双入口额度语义、operator 管 MCP token 是否收紧 admin-only、前端工具链跨 major 升级决策事项沿 §五/10 存档口径不变。
