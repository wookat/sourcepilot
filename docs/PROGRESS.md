# TradeMind 开发进度记录

**Stage update**: 2026-08-07 — **Round 152 线1：对外开放 REST API（只读起步，`GET /api/open/v1/*` + token 用途字段）**：详见附录 [`docs/progress/R152-line1-open-api.md`](progress/R152-line1-open-api.md)。

**Stage update**: 2026-08-07 — **Round 151 线2：第七次竞品对标复评**：详见附录 [`docs/progress/R151-line2-competitive-benchmark-v7.md`](progress/R151-line2-competitive-benchmark-v7.md)；报告 [`COMPETITIVE_BENCHMARK_R151.md`](COMPETITIVE_BENCHMARK_R151.md)。

**Stage update**: 2026-08-06 — **Round 148 线1：R144–R147 验收收口（MCP 页时间列 formatDateTime + 验收包增量 + Docker 实跑）**：详见附录 [`docs/progress/R148.md`](progress/R148.md)。

**Stage update**: 2026-08-06 — **Round 143 线1**：详见附录 [`docs/progress/R143.md`](progress/R143.md)（自本轮起每轮进展写入 `docs/progress/R<轮次>.md` 附录，本文件只留一行索引，减少并行 PR 冲突）。

**Stage update**: 2026-08-06 — **Round 141 线1：验收包增量更新（R136–R140 并入）**：`docs/acceptance/ACCEPTANCE_R123.md` §一/12 合入状态收口（#280/#281/#283 已合入 main，⏳→✅），新增 §一/13「R136–R140 增量能力」（UX v9 收口 #284、报表 CSV「未折算」显式口径 #285、生产演练季度复检 #286、备份对象存储上传 + R139 安全审计 4 条 S3 加固 #287、深分页/未绑定口径 #288，全部 ✅），§五登记 R141 时点结论；`DEMO_SCRIPT.md` 第 23 步治理面收尾并入备份对象存储演示点（未配 S3 按降级「仅本地」口径演示，需 `BACKUP_ENABLED=true`/`BACKUP_MODE=local` 前置）、第 18–19 步补 #284/#285 口径，保持 30 分钟。Docker 全栈（main `99fd2e7d`）三角色实跑通过，两处失实（备份启用前置、上传状态文案「仅本地」）已修正脚本；README/production-launch-checklist/upgrade-guide 抽查无失实。实跑证据作会话附件不入库。

**Stage update**: 2026-08-06 — **Round 140 线1：R139 安全审计 4 条 S3 加固收口（并入 #287）**：1）上传错误落库前显式替换 `BACKUP_S3_ACCESS_KEY_ID`/`BACKUP_S3_SECRET_ACCESS_KEY` 字面值为 `[redacted]`（纵深防御，覆盖 S3 兼容实现回传 XML 含 AWSAccessKeyId 的场景），再走通用脱敏。2）`BACKUP_S3_ENDPOINT` 启动校验：任何环境要求合法 http(s) URL；生产要求 `https://` 且拒绝 localhost/回环/link-local（含 169.254.169.254 元数据地址）。3）保留清理收窄：有效 `BACKUP_STORAGE_PREFIX` 为空时拒绝清理（防整桶枚举删除），且仅删除 `bk_*.dump`/`bk_*.dump.enc` 命名的备份产物。4）对象存储取回落地路径 containment：`fetchFromObjectStore` 校验 `LocalPath` 位于备份工作目录之下，越界拒绝写盘。四条均补回归测试。

**Stage update**: 2026-08-06 — **Round 138 线1：对象存储备份上传（收掉最后一个部署债）**：1）新增 `backend/internal/providers/backupstore` 备份专用 S3 兼容对象存储 Provider（AWS S3 / MinIO / 阿里 OSS S3 兼容端点，官方 aws-sdk-go-v2 复用既有版本）：`Upload/Download/List/Delete/Target`，`Target` 输出脱敏目标（不含 AK/SK）。2）备份闭环：backup 完成后自动上传（`BACKUP_UPLOAD_MAX_ATTEMPTS` 有界重试，失败不影响备份本身、落库 uploadStatus=failed 可在 Ops 页重试）；上传成功后按 `BACKUP_OBJECT_RETENTION_COUNT` 保留最近 N 份（retention hold 备份不清理）；download/校验在本地文件缺失时自动从对象存储取回并校验 checksum。`BACKUP_S3_*` 全部留空为降级模式（仅本地路径，不阻塞部署），半配置启动即报配置错误。3）Ops 备份页新增上传状态/上传目标列与「重试上传」，readonly 全部写操作禁用；新端点 `POST /ops/backups/:id/upload`（backup.create）登记权限矩阵与 tenant-zero 安全测试。4）文档同步：env/production-launch-checklist（crontab 改为建议 + 对象存储上传）/upgrade-guide/api/provider/.env 示例。

**Stage update**: 2026-08-06 — **Round 137 线1：UX v9 P2-3 收口（报表 CSV「未折算」显式口径）+ 杂项巡检**：非 CNY 无手工汇率时报表 CSV 的折算/本位币列由留空统一为显式「未折算」占位（与页面口径一致、仍不伪造折算）：利润报表 CSV（`reports/profit/export.csv` 折算收入列与本位币成本/费用/毛利列）、经营报表逐日 CSV（`orders/stats/daily/export.csv` 折算金额列，空日期行同口径）、对账报表/差异工作台 CSV（`finance/report|reconciliation/export.csv` 本位币列；店铺月度费用「无登记」仍留空以区分「未折算」与「缺记录」，无费用/成本行的 0 合计保持 `0.00` 不误标）。UI 已为「未折算」口径不改动；补/改回归测试（reports/order/finance 三模块 CSV 断言），`docs/api.md` 同步。杂项巡检（R132–R136 合入面）：check:ui-copy --strict 通过、无 console 残留、订单标签/自动化规则页空态含引导文案，无新增登记项。基于 #284（未合并，UX v9 报告与 P2-3 登记所在分支）本地叠加。

**Stage update**: 2026-08-06 — **Round 136 线2：全站视觉/UX 复核 v9（报告 [`docs/ux-review/UX_REVIEW_V9_REPORT.md`](ux-review/UX_REVIEW_V9_REPORT.md)）**：v8 遗留 P2×2 收口无回退；#282 订单标签全动线、#277 CSV 导出交互、#275 日志中文化、#281 readonly 门控走查通过；硬指标全零。随报告修复 P1×1（对账 CSV「平台」列英文枚举 → `opslabels.PlatformLabel` 中文化 + 回归测试）与低成本 P2×2（操作日志 `order_tag.*` 动作/资源中文映射 + 单测；订单标签页创建时间改统一 `formatDateTime`）；P2-3（毛利 CSV 非 CNY 本位币列空白口径）待产品确认。

**Stage update**: 2026-08-06 — **Round 136 线1：大回归 v20 P2×3 收口 + 验收包增量（R132–R135 并入）**：1）v20 P2-1：demo seed 对账数据补足 25+ 行（`fulldemo_round136.go` 按已付款订单缺口补种 `DEMO-FIN-21xx` 至 28 行目标，结清/少款/多款/未回款四类样本轮转、双店铺交替），对账工作台默认 20/页出第二页、合计区跨页聚合、#277 CSV 跨页全量导出可 UI 层证明；clean/verify 沿用 order_no/remark `DEMO-` 前缀零残留、重跑幂等（`fulldemo_round136_test.go`）。2）v20 P2-2：seed 补 operator 授权店自动化正样本 `DEMO-AT-1005`（手工渠道店、unpaid+审单通过+SKU matched 行齐备），operator 自行批量标记已付款即可真实触发「自动生成采购单」成功动线，不再依赖 admin 账号。3）v20 P2-3：对账 CSV Currency 注入防护补 E2E 层验证——`TestRound136ReconciliationCSVCurrencyInjectionE2E`（integration，实库 + 完整生产路由 + JWT 鉴权走 `/finance/reconciliation/export.csv`，DB 注入 `=1+2` 断言导出为 `'=1+2`），补足 v20「仅后端单测」的口径缺口。4）验收包增量：`ACCEPTANCE_R123.md` 新增 §一/12「R132–R135 增量能力」（#278 v19 P2 收口 ✅、#279 升级演练复跑 ✅、R133→#281 P2×7 收口 ⏳、#280 竞品复评 v5 ⏳、#282 订单标签 ✅、v20 大回归与本轮 P2×3 收口），§四补 R134 复评 v5 口径，§五登记 v20 合并顺序（#280→#281）；`DEMO_SCRIPT.md` 第 9/11 步并入订单标签演示（#282）、第 18 步补 25+ 行分页/合计与跨页全量导出、第 20 步 operator 补 DEMO-AT-1005 真实触发，保持 30 分钟。基于 main 本地叠加 #280/#281（PROGRESS 清稿性冲突按 v20 结论双条目保留）。

**Stage update**: 2026-08-06 — **Round 134 线1：R133 两线 P2×7 收口批次**：安全线：1）finance 聚合 tenant 防御纵深——`paymentAggs`/`expenseAggs` 按订单 ID 聚合时补 `tenant_id = ?` 前置过滤（与 `actualCostAggs`/`expensePartsByGroup` 口径一致；order_items 无 tenant 列、经 scoped 订单子查询限定不变），补跨租户同 order_id 注入回归测试 `TestReconciliationCrossTenantAggIsolation`。2）对账 CSV Currency 列补 `csvsafe.Cell`（`ExportReconciliationCSV` 行内唯一未包裹的文本列），补 `TestReconciliationCSVCurrencyEscaped`。3）`@umijs/max` 构建链 advisories（13 条：vite/esbuild/react-router/@hono/node-server 等，2 high 均为 dev server 面）需跨 major 升级或 override 传递依赖，按指令**登记待决策不擅动**，登记文档 [`DEPENDENCY_ADVISORIES_R134.md`](DEPENDENCY_ADVISORIES_R134.md)。4）E2E 冷启动 flake：新增 Playwright `globalSetup`（`admin/e2e/global-setup.ts`）在全部用例前真实打开一次首页等 `#root` 渲染完成，让首批用例面对已编译完成的 dev bundle（与 #278 的 smoke 首屏放宽互补）。QA 线：5）readonly 草稿详情「保存基础信息」前端门控——readonly 时隐藏 submitter、表单置灰并给只读提示（后端 403 不变）。6）采集规则英文报错中文化：`collectrule` 新增 `localizeRuleError` 集中映射（规则校验/URL 匹配/状态/测试 等 30+ 英文报错 → 中文；中文透传、未映射加中文前缀保留原因），handler 固定英文文案（invalid id/not found 等）同步中文化，补 `localize_test.go`。7）草稿详情来源平台枚举直出：`DraftDetail` 头部与「商品来源与采集信息」的 `data.source` 改用 `productSourceLabel`（1688/拼多多/淘宝/自定义链接 等），不再用平台映射表兜底直出原始枚举。E2E 新增 `round134-p2.spec.ts`（readonly 门控 + 来源枚举中文断言）。

**Stage update**: 2026-08-06 — **Round 134 线2：第五次竞品对标复评（报告归档）**：新增 [`COMPETITIVE_BENCHMARK_R134.md`](COMPETITIVE_BENCHMARK_R134.md)——基于最新 main（791f3a84）Docker 全栈实测的 16 项矩阵复评（vs 店小秘/马帮，补充 AutoDS，含竞品 2026 年近期更新调研）：维持超越 3 / 达到 13 / 落后 0，8 轮维护期全程实测零回退；R126 自动化动作扩展（自动应用发货规则/自动分仓）真实付款触发实测全部执行留痕，第 6 项订单管理由「▲边缘」坐实为超越。对照竞品近期更新（店小秘实时大屏/视频翻译/AI 美图、马帮以图搜SKU/沃尔玛 MCS/巴西 NFe、AutoDS Claude MCP）未发现新的结构性产品缺口，新差距均为凭证依赖或可选增量。发现项 2 个（DEMO-AT-1004 SKU 提示口径不一致、demo seed 缺采集规则样本，非阻断）。结论：维护期可收束，建议「等凭证为主 + 小步差异化」双轨（P0 抖店 E2E 插队 / P1 发现项收口+自动打标签 / P2 实时大屏或 MCP 入口备选），预验收包可按沙箱口径正式提交。本轮为纯文档归档，无代码变更。

**Stage update**: 2026-08-06 — **Round 132 线1：大回归 v19 P2 收口 + 验收包增量（R128–R131 并入）**：1）v19 P2-1：demo seed 为 DEMO-AT-1004 正向样本补 `order_item_sku_matches` matched 行（`fulldemo_round119.go`），订单列表「库存扣减」列的聚合口径（`deriveSkuMatchStatus` 按 match 行统计）与「SKU 已匹配」前置描述一致，不再显示「SKU 未就绪」；Seed 先清后种保持幂等，Cleanup 按 order_id 覆盖新行、clean/verify 零残留（round119 测试补 match 行断言）。2）v19 P2-2：`admin-smoke.spec.ts` 首屏 `#root` 等待放宽至 30s（smoke 常为全量 E2E 首批用例，dev webServer 冷启动按需编译在高 CPU 竞争下会超默认 8s expect 超时；45s 用例总超时不变，不掩盖真实失败）。3）验收包增量：`ACCEPTANCE_R123.md` §一/10 合入状态收口（#266/#268/#269/#270 已按 v18 结论合入 main，⏳→✅）、新增 §一/11「R128–R131 增量能力」（seed 双租户 #272、UX v8 #273、订单号/客户筛选修复 #274、聚合下推 #276 均 ✅；#275 执行日志样本/操作日志中文映射、#277 CSV 全量导出已合并 ✅）；`DEMO_SCRIPT.md` 补第二租户账号与 19b 双租户隔离步骤、第 9 步订单号筛选、第 18 步聚合下推口径、常见坑更新（#268 已合入 main、#275/#277 依赖说明）。

**Stage update**: 2026-08-06 — **Round 130 线2：UX v8 P2 收口小批次（#273 P2×2）**：1）demoseed 执行日志样本补齐：主租户新增 recommend 规则「DEMO-付款后推荐物流商（仅推荐）」+ 手工渠道店（operator/readonly 授权店）三条执行日志 `DEMO-AT-1301~1303`（成功=recommend「仅推荐，发货时人工确认」文案 / 规则未命中跳过 / 分仓库存不足失败），operator 视角 `/orders/automation-logs` 不再空态且 R128 recommend 模式文案首次实弹覆盖；第二租户新增 recommend 规则 + `DEMO-T2-AT-0001~0003`（成功/失败/跳过各 1），第二租户视角执行日志页不再空态；Docker 实测发现并修复 demoseed 执行日志缺 `shop_id` 快照导致店铺 scope 过滤后 operator 空态的问题（所有 DEMO 日志现设置 ShopID 并加回归断言）。clean/verify 沿用 rule_name/order_no `DEMO-` 前缀零残留、重跑幂等（`fulldemo_round130_test.go`）。2）操作日志枚举中文映射收口：`admin/src/constants/operationLogs.ts` 补齐后端实际登记但缺失映射的操作类型/资源（`order_automation.execute`、`order_automation_rule.*`、`order_review.approve/reject`、`user.*`、`session_revoke*`、`inventory_sync.*`、`shop.oauth.*`、`selection.*`、`sku_binding.*` 等 90+ 键；资源补 `order_automation`/`order_review`/`admin_user`/`auth_session`/`waybill` 等 24 键），API 原值不变（Tooltip 仍显示原始值），沿用 #272 集中映射口径（`operationLogLabels.test.ts`）。3）凭证核查：本轮未新增任何 seed 凭证，第二租户账号说明已在 `docs/development.md` / `docs/DEMO_SEEDING_GUIDE.md`（后者本轮补执行日志样本说明）。依赖：#272 已合并入 main。

**Stage update**: 2026-08-06 — **Round 129 线1：杂项收口（订单列表订单号/客户筛选修复）**：修复 R128 发现的「订单列表订单号筛选点查询不生效（keyword 可用）」：根因为订单页搜索表单的「订单号」「客户」字段（来自表格列的可搜索项）未接入 URL query 单一来源链路——`onSubmit` 未把 `orderNo`/`customerName` 写回 URL、request 也未透传（后端 `GET /orders` 本就支持两参数，非 #35 的 params 覆盖问题）。修复：`ORDER_QUERY_KEYS`/表单回填/onSubmit/request 全链路补 `orderNo`+`customerName`，`urlState.ts` 白名单补 `orderNo`。全站抽查同类筛选（采购状态、异常工作台全部搜索字段、自动化日志状态/事件/关键词、财务回款/对账）及 ProTable 全页面可搜索列与提交链路脚本扫描，未发现其他脱节字段。R127/R128 遗留 P2 再评估：执行日志长 Reason 已有 ellipsis+Tooltip（R122），原位展开方案（Typography.Paragraph JS ellipsis）会裁剪 DOM 文本、回退可达性，非低成本保留；readonly 深链 R127 v18 已验证写入口隐藏/越权 404，保留。新增 E2E `round129-order-filters.spec.ts`（订单号/客户筛选、URL 回写、重置、深链刷新持久化 3 例）。

**Stage update**: 2026-08-05 — **Round 128 线1：P2 收口批次（R127 两线 P2 清单）**：1）demoseed 第二演示业务租户：`pnpm seed:demo:full` 额外创建独立租户「DEMO-第二租户」（tenants 行 + admin 账号 `demo_tenant2_admin@trademind.local` / `DemoTenant2Admin123!` + 1 店铺 `DEMO-T2-SHOP-1` / 2 订单 `DEMO-T2-SO-*` / 1 发货规则 / 1 自动化规则），回归轮可开箱测双租户隔离（正向：第二租户账号只见自己数据；负向：主租户看不到 `DEMO-T2-` 数据）；clean/verify 覆盖租户与账号零残留、`-prefix` 自定义前缀互不影响、重跑幂等（`fulldemo_tenant2.go` + `fulldemo_round128_test.go`）。2）自动化操作日志中文化：`order_automation.execute` 与规则 create/update/delete、日志 retry 的操作日志消息不再直出英文枚举（`order_paid`/`apply_shipping_rule`/`recommend`/`apply` 等），后端新增 `AutomationEventLabel`/`AutomationActionLabel`/`ShippingApplyModeLabel`（与 admin `orderAutomation.ts` 标签口径一致）。3）E2E 稳定性：`round126-auto-actions.spec.ts` 下拉点击改 `selectAntOption` 辅助（等下拉真正展开、目标选项可见再点击，收口 R127 v18 登记的 1440 视口竞态 flake）。4）文档：`docs/development.md`、`docs/DEMO_SEEDING_GUIDE.md` 补第二租户账号与宿主机 `DB_HOST=127.0.0.1` 覆盖提示。其余 R127 P2（375 长文案 Tooltip、readonly 独立实走、中文 IME）登记理由不改（见 PR）。

**Stage update**: 2026-08-05 — **Round 126 线2：自动化动作面扩展（竞品复评 v4 建议项）**：R119 自动化规则新增两个站内动作。1）`apply_shipping_rule` 自动应用发货规则：订单事件触发时按 R111 发货规则（`waybill.RecommendTenant` 租户级复用）推荐物流商并落到订单计划字段（`plannedCarrier*`），`shippingApplyMode` ∈ `recommend` 仅推荐 / `apply` 直接应用（默认 `recommend`）；仅记录计划物流商，发货时仍可人工改选，已有计划/人工选择不覆盖（guarded UPDATE）。2）`assign_warehouse` 自动分仓：`warehouseStrategy` ∈ `default_warehouse` 默认仓 / `stock_first` 库存充足优先（`inventory.PlanOrderWarehouse` 只读规划，租户 scope 含 SKU 归属校验），分配落 `assignedWarehouse*` 字段并与 R112 多仓扣减联动（未显式指定仓时扣减锁定已分配仓）；库存不足执行失败留痕（inline ≤3 次尝试 + 执行日志人工重试）。两动作均沿用 R119 幂等 DedupKey、审单安全边界（待审/挂起 skipped）、dry-run、操作日志与订单自动化轨迹，租户/店铺 scope 遵守 #267 修复后口径。demoseed 补 2 条 DEMO 规则 + `DEMO-AT-1201`（发货规则已应用成功样本）/`DEMO-AT-1202`（分仓库存不足失败样本），clean/verify 零残留。设置页规则表单扩展动作与参数选择器（应用方式/分仓策略，表格参数标签）。后端单测（`automation_round126_test.go`、`warehouse_plan_test.go`、分仓扣减联动）、契约 3 端点字段、前端 `orderAutomation.test.ts`、E2E `round126-auto-actions.spec.ts`（含五档视口）已补；`docs/api.md` 已同步。

**Stage update**: 2026-08-05 — **Round 125 线2：第四次竞品对标复评（报告归档）**：新增 [`COMPETITIVE_BENCHMARK_R125.md`](COMPETITIVE_BENCHMARK_R125.md)——基于最新 main（314fc1ed）Docker 全栈实测的 16 项矩阵复评（vs 店小秘/马帮，补充 AutoDS）：超越 3 / 达到 13 / 落后 0（R118：超3/达10/落3）。R119–R124 交付把 R118 识别的 4 个纯产品差距（自动化订单规则、买家自动消息、选品数据面、财务对账）全部实测收口（含规则真实触发、安全边界负样本、对账口径核对）；原 3 个落后项（真实刊登/电子面单/1688 直采）复核确认为纯外部凭证依赖，重分类为「凭证待解锁」。结论：正式验收（真实抖店 E2E）前纯产品侧无必须补齐项；R126+ 备选为自动化动作面扩展等可选增量。本轮为纯文档归档，无代码变更。

**Stage update**: 2026-08-05 — **Round 128 线2：验收包增量更新（R124–R127 并入）**：`docs/acceptance/ACCEPTANCE_R123.md` 新增 §一/10「R124–R127 增量能力对照」（768 断点互斥、采购单来源订单号、升级演练复跑结论、竞品复评 v4 超3/达13/落0、R125 安全审计 4 组 P1 越权修复、审计 P2×4 收口、自动化新动作 apply_shipping_rule/assign_warehouse、R127 交叉 QA P1×2 与大回归 v18 结论），图例新增 ⏳ 待合并（#266/#269/#268）；§四竞品结论切换为 R125 复评 v4 口径；§五登记 v18 合并顺序（#266→#269→#268）与 #245/#247/#248 冗余关闭建议。`DEMO_SCRIPT.md` 第 11–12 步纳入 R126 新动作演示（规则参数 Tag→DEMO-AT-1004 真实触发三动作→AT-1201/1202 正负样本与重试口径→订单详情计划物流商/分配仓库），原客服话术独立步骤并入买家消息步骤保持 30 分钟总长，并注明 #268 未合并前的构建依赖。README/升级指南与 R124–R127 一致性抽查无失实（upgrade-guide 已由 #265 更新）。演示脚本 Docker 全栈实跑证据作 PR 评论不入库。
**Stage update**: 2026-08-05 — **Round 124 线2：生产升级演练复跑（R118 → main）**：从零部署（production + Caddy）到登录实测 232s（tenant 0 生产口径、secure_session、备份创建+四项校验、恢复演练生产禁用、日志脱敏抽查通过）；R118（`e9b27309`）+ 存量数据（4 万订单/12 万库存流水/双租户）升级到 main（`314fc1ed`）：R119–R122 全部新表与 #261 三个性能索引落地（AutoMigrate 全程约 3s，4 万行 orders 建索引 ~26ms、12 万行流水 ~38ms），迁移前后数值指纹逐项 0 差异，升级后订单自动化（order_paid→mark_printed 留痕）与财务对账（回款登记→settled、reconciliation/report）实测可用；故障路径（备份→注入同租户重复订单号中断→`docker exec -i` pg_restore 恢复→清理→重跑成功）闭环复现。文档核对修正：upgrade-guide 迁移点表补 R119–R122 行、修正「同名冲突表列类型不兼容必报 database_migrate_failed」的失实表述（实测 bigint 主键冲突被 AutoMigrate 静默保留、启动成功，风险后置为运行期写入失败）、补 R124 演练记录。演练报告仅作会话附件不入库。

**Stage update**: 2026-08-05 — **Round 120 线2：R119 交叉 QA + 遗留 P2×3 收口**（基于最新 main 本地叠加 #254/#255，含合并冲突解决：round119 demoseed 拆分两文件、契约 90 端点）：P2①订单详情页新增「自动化轨迹」Tab（复用 `GET /orders/:id/automation-logs`，成功/失败/跳过留痕 + 跳转全量执行日志，支持 `?tab=automation` 直达）；P2②草稿编辑保存时后端按内容剩余 `{变量}` 占位重算 `missingVars`（补全后「缺少变量」警告消除，重新引入占位重新出现，Go 回归测试）；P2③规则引用已删除模板保护——规则行返回 `templateMissing`，列表标红「模板已删除」、未重选模板前禁止重新启用、编辑弹层警示且不预填已删模板（选已删模板后端本就 400，模板下拉仅列可用模板；已删模板规则生成侧本就 inert）；顺带修复规则/草稿弹窗表单校验失败时未捕获 promise 触发 pageerror。新增 E2E `round120-p2.spec.ts`（3 例），`docs/api.md` 已同步。

**Stage update**: 2026-08-05 — **Round 120 线1：选品数据面增强（R121 范围提前）**：1）候选商品数据面板：`GET /selection/candidates/:id/insights` 整合采集所得价格/销量/评价数（同一 `source_url` 的成功采集任务留痕解析 SKU 价/月销/评价，无则前端显式「未采集」）、站内同类目基准（近 90 天同类目商品数/草稿平均毛利率/订单动销/毛利率，打通 products+orders）与 AI 评分明细拆解；Admin `InsightsDrawer` 展示。2）趋势视图：`GET /selection/candidates/:id/price-trend` 返回同一来源多次采集历史价格（≤200 点），前端 `@ant-design/plots` Line 绘制，<2 点空态引导前往采集任务。3）选品对比：`GET /selection/compare?ids=`（2–5 个候选）并排对比价格/预估毛利/供应链就绪度（`product_sources` 货源档案匹配）/违禁词风险（租户违禁词库命中），前端 `CompareDrawer` 多选发起、UTF-8 BOM CSV 导出。4）外部数据源 Provider 预留：`providers/markettrend` Registry（tiktok/shopee/amazon 热销榜槽位），`GET /selection/market-sources` 报告配置状态，缺凭证明确降级不虚构。5）demoseed 补 DEMO 候选多次采集历史（4+2 次留痕、1 个空历史候选）与同类目商品关联，clean/verify 零残留。后端/合同/前端单测、`round120-selection-insights.spec.ts` E2E（五档视口 overflow）全绿。

**Stage update**: 2026-08-05 — **Round 120 线2：R119 交叉 QA + 遗留 P2×3 收口**（基于最新 main 本地叠加 #254/#255，含合并冲突解决：round119 demoseed 拆分两文件、契约 90 端点）：P2①订单详情页新增「自动化轨迹」Tab（复用 `GET /orders/:id/automation-logs`，成功/失败/跳过留痕 + 跳转全量执行日志，支持 `?tab=automation` 直达）；P2②草稿编辑保存时后端按内容剩余 `{变量}` 占位重算 `missingVars`（补全后「缺少变量」警告消除，重新引入占位重新出现，Go 回归测试）；P2③规则引用已删除模板保护——规则行返回 `templateMissing`，列表标红「模板已删除」、未重选模板前禁止重新启用、编辑弹层警示且不预填已删模板（选已删模板后端本就 400，模板下拉仅列可用模板；已删模板规则生成侧本就 inert）；顺带修复规则/草稿弹窗表单校验失败时未捕获 promise 触发 pageerror。新增 E2E `round120-p2.spec.ts`（3 例），`docs/api.md` 已同步。

**Stage update**: 2026-08-05 — **Round 119 线2 买家自动消息（站内草稿 + 人工确认闭环）**：无真实平台消息通道前提下的降级闭环——订单节点自动生成「待发送草稿」，人工在平台后台发送后回执，**绝不自动外发**。1）后端 `customerchat` 新增 `buyer_message_rules` / `buyer_message_drafts`：租户级节点规则（`paid`/`shipped`/`delivered`/`logistics_exception`/`refunded` × R109 话术模板，启停、平台/店铺过滤），草稿生成引擎按订单/物流上下文填充 `{买家昵称}{订单号}{物流单号}{商品名}{店铺名}`（缺失变量保留占位并记 `missingVars`），同一 `tenant+order+node` 幂等；进程内定时扫描（`BUYER_MESSAGE_SCAN_ENABLED`/`BUYER_MESSAGE_SCAN_INTERVAL_SECONDS`，默认开、60s）+ 手动 `POST /customer/buyer-messages/generate`。2）API 10 条（规则 CRUD + 草稿列表/编辑/标记已发送/忽略/批量标记已发送/生成）：写端点走 `adminperm.CanWriteCustomer`（readonly 403），跨租户/不存在一律 404；草稿关联订单与客服会话（可跳转），AI 建议与模板插入口径不变。3）Admin 新页 `/customer/buyer-messages`（客服 Hub 入口）：待发草稿工作台（节点/状态/店铺/关键词筛选、编辑、单个/批量标记已发送、忽略、缺失变量警示、订单/会话跳转）+ 节点规则管理 tab，页面顶部明确降级说明。4）demoseed round119：3 规则（1 停用）+ 4 草稿（2 pending/1 sent/1 ignored，含缺变量样本），clean/verify 零残留。契约测试 82 端点；后端 `buyermsg_test.go` + `buyermsg_http_test.go`（三角色/租户隔离/越权 404），前端 `buyerMessages.test.ts`（12 例）、E2E `round119-buyer-messages.spec.ts`（10 例含五档视口）；`docs/api.md`、`docs/env.md`、`.env.example`、`.env.docker.example` 已同步。

**Stage update**: 2026-08-05 — **Round 119 自动化订单规则**：对标竞品订单自动化流转，在 R114 审单规则引擎风格上新增**租户级自动化订单规则**（仅站内状态流转，不做真实平台 API 动作）。数据模型：`order_automation_rules`（触发事件 `order_created`/`order_paid`/`procurement_delivered`/`logistics_collected` + AND 条件 平台/店铺/金额区间/要求审单已通过 → 站内动作 `confirm_payment`（低风险：必须配金额上限）/`generate_procurement`/`mark_printed`/`notify_shipping`；事件与动作合法组合校验；`priority` 升序、启停）+ `order_automation_logs`（执行留痕：success/failed/skipped + 中文原因 + attempts；`tenant:rule:order:event` DedupKey 幂等——成功/跳过不重复执行，失败 inline ≤3 次尝试并可人工重试）。触发链路：订单创建/付款状态变已付款、审单放行补触发（安全边界：`pending_review`/`held` 一律跳过并记 skipped，沿用 #240 阻断）、采购签收（`MarkDelivered`）与物流揽收（`FillLogistics`）经 `OrderEventNotifier` 回调触发；`generate_procurement` 经 router 注入 `AutomationHooks` 复用采购生成（含幂等 key），避免 order↔procurement 循环依赖；自动动作操作日志留痕（消息含「自动规则：规则名」），订单维度轨迹 `GET /orders/:id/automation-logs`。API 8 个（见 `docs/api.md`），契约测试 80 端点，permmatrix 补 8 条（写端点 readonly forbid；越权 404）；订单模型新增 `shipReadyNotifiedAt`。管理页 `/settings/order-automation-rules`（增删改/优先级/启停/dry-run 预览含安全边界跳过数）与 `/orders/automation-logs`（状态/事件/关键字筛选、失败重试）。demoseed 补 4 条 DEMO 规则（含停用示例）与成功/失败/跳过日志样本，clean/verify 零残留。后端单测（校验/幂等/重试/安全边界/跨租户 404/触发链）+ E2E `round119-order-automation.spec.ts`（13 例含五视口无横向溢出）已补。

**Stage update**: 2026-08-05 — **Round 118 线1：审单口径 P2 修复 + 数据搬家小改 + 生产升级演练**：1）审单工作台头部「待处理共 N 单」`pendingTotal` 补上授权店铺 scope（`adminperm.ApplyStoreScope`），与列表明细口径一致，operator 不再看到未授权店铺的待审计数；补 HTTP 级回归测试（授权/未授权双店铺，先证伪后修复）。2）E2E fixtures 与后端导入模板示例 SKU/仓库对齐 demoseed（`DEMO-SKU-1-1`/`DEMO-SKU-1-2`、`default`/`DEMO-WH-2`，R116 遗留观察项收口）；导入历史 Tab 激活时自动刷新（refreshToken，新提交 job 无需刷新页面即可见），补 E2E `round118-import-history.spec.ts`。3）生产升级演练（R106 `cb07f920` + 存量数据 → R118，Docker production 实测）：R112/R113 默认仓 backfill+唯一索引、审单/导入/安全批次新表逐项验证通过，存量订单/SKU 库存/导入任务无损；预检 SQL 与陷阱 1 SQL 实测有效；从零部署到登录 254s（目标 <15min）；备份→注入冲突表升级失败→pg_restore 恢复→回滚→清理重跑成功；未发现 P0/P1，upgrade-guide 补 R112–R116 迁移要点表与「恢复后新表残留」说明。

**Stage update**: 2026-08-05 — **Round 118 第三次竞品对标复评（报告归档）**：新增 [`COMPETITIVE_BENCHMARK_R118.md`](COMPETITIVE_BENCHMARK_R118.md)——基于 main 实际代码逐项核对的 16 项矩阵复评（vs 店小秘/马帮，补充 AutoDS）：超越 3 / 达到 10 / 落后 3（R108：超3/达7/落4），R109–R117 交付使审单/多仓/报表由缺失转达到、数据搬家/打单/移动端显著增强，无回退；剩余落后 3 项（真实刊登连通/电子面单/1688 直采）全部卡外部凭证，凭证清单按杠杆重排申报；给出 R119–R125 建议路线与验收水平判断（纯产品能力达标，正式验收建议待抖店凭证注入后跑一轮真实 E2E）。本轮为纯文档归档，无代码变更。

**Stage update**: 2026-08-05 — **Round 116 数据搬家 XLSX 与万行性能**：收口 R115 两个未测/未做项。1）真实 .xlsx 支持：`migrationimport` XLSX 解析从手写 zip+xml 换为 excelize v2.11（BSD-3-Clause，与 Apache-2.0 兼容），四类导入与店小秘/马帮预设均可上传 .xlsx；解压安全限制（UnzipSizeLimit 128MB、单 XML 部件 64MB）防 zip 炸弹，损坏/加密文件中文 400；新增 `xlsx_security_test.go`（损坏文件/zip 炸弹/行数上限截断 + `FuzzParseImportFile` fuzz）。模板下载 `GET /imports/templates/:kind?format=csv|xlsx` 双格式（默认 CSV 不变）。2）万行性能：单批上限 1000→10000 行（文件 ≤10MB 不变；理由：commit 路径已改批量查重（SKU/订单号 IN 分块 ≤1000）+ 分批事务（每 500 行一事务、行级 savepoint 隔离单行失败），库存期初从每行一事务改为批量；实测 sqlite 1 万行校验 ~7ms、落库 ~4.3s、堆增量 ~9MB）。新增 `GET /imports/progress`（单实例内存态进度，`active/processed/total`）+ commit 进行中重复提交 409；Admin 确认导入页进度条轮询 + 按钮防重复提交。3）R115 遗留收口：预览空单元格以「—」占位对齐（消除视觉列偏移）；映射方案名输入经 E2E 验证非缺陷。契约测试 72 端点；后端 `round116_perf_test.go`（IMPORT_PERF=1 门控）、前端 `imports.test.ts` 补进度用例、E2E `round116-import-xlsx.spec.ts`（4 例：xlsx 预览/店小秘 xlsx 识别/双模板入口/进度与防重复）已补；`docs/api.md`、`docs/migration-guide.md` 已同步。

**Stage update**: 2026-08-05 — **Round 115 数据搬家增强**：迁移向导从「商品/订单 × 店小秘/马帮」扩展为四类数据全量搬家。1）通用模板：新增 `GET /imports/templates/:kind` 通用 CSV 模板下载（UTF-8 BOM、中文表头+示例行，Excel 可直接打开），四类均可按模板走既有 上传→列映射→校验→导入 向导（幂等/错误行/1000 行/10MB 口径不变）。2）新增两类导入：`kind=inventory` 库存期初（SKU+仓库编码+数量+参考进价；落 SKU 总库存与非默认仓 `warehouse_stocks`、写 `inventory_change_logs`（type=`import_opening`），同租户同 SKU 同仓 `BusinessEventKey` 幂等防重复，多仓口径与 R112 一致）；`kind=source` 货源档案（供应商+货源链接+参考价+SKU 映射，复用 sourcing `BindSource`/`SaveSKUMappings`，供应商按名幂等、已存在映射计 duplicate）。库存/货源为租户级不需店铺，商品/订单沿用授权店铺口径（operator 越权 404/403）。3）导出补齐：`GET /imports/export/:kind` 四类全量 CSV 统一入口（商品每 SKU 一行、订单每商品行一行含币种、库存每 SKU×仓一行（默认仓=总−非默认仓之和）、货源每映射一行；csvsafe 防注入，≤50000 行），Admin 迁移页新增「数据导出」tab。4）映射方案：新增 `import_mapping_presets` 租户级列映射方案（同 kind+name 覆盖、每 kind ≤50 个），`GET/POST/DELETE /imports/mappings`，向导列映射步骤支持保存/套用/删除。5）demoseed clean/verify 纳入 mapping presets 零残留；readonly 403、CSV 注入防护沿用。契约测试 71 端点；后端单测 `round115_test.go`、前端 `imports.test.ts`、E2E `round115-migration.spec.ts`（四例含三视口无横向溢出）已补；`docs/api.md`「迁移导入与数据搬家」已同步。

**Stage update**: 2026-08-05 — **Round 114 审单规则（订单自动审核）**：对标店小秘/马帮订单自动审核，在既有固定异常拦截（缺主货源/缺 SKU 映射/负毛利/重复单号，保持不变）之外新增**租户级可配置审单规则**。数据模型：`order_review_rules`（条件 AND：金额区间/收货地址关键词（黑名单地区）/买家备注关键词/商品总数量/单 SKU 超量/指定平台/店铺/同收件人窗口期多单；动作 `pass` 自动通过 / `review` 打标待审 / `hold` 挂起拦截；`priority` 升序、启停）+ `order_review_hits`（命中快照：规则名/动作/中文原因/decisive，规则删除后仍可读）；订单新增 `reviewStatus`（`''`/`auto_passed`/`pending_review`/`held`/`approved`/`rejected`）。动线：订单进入（导入/手工/批量粘贴统一走创建动线）时事务内跑规则引擎，首个命中规则决定动作；待审/挂起订单落 Admin 新增「审单工作台」`/orders/review`（与异常工作台并列、文案区分来源），支持单个/批量放行与拒绝（拒绝置 `rejected` 并入取消动线 `status=cancelled`）、命中规则与原因可见；**后端强制**待审/挂起不允许生成采购单（`review.blocked` blocker）与新增发货（可读中文提示），放行后回正常流。管理页 `/settings/order-review-rules`：规则增删改/优先级/启停/测试跑（对租户最近 ≤500 单 dry-run 预览命中数与样本，不落库）。API 8 个（见 `docs/api.md`），契约测试 66 端点，permmatrix 补 8 条（写端点 readonly forbid；越权规则更新/删除 404）；demoseed 补 4 条 DEMO 规则（含停用示例）与 3 个命中样本订单，clean/verify 零残留。后端单测（规则 CRUD/租户隔离/引擎语义/边界/工作台/放行拒绝/发货与采购阻断/dry-run）+ 前端服务单测 + E2E `round114-order-review.spec.ts`（10 例）已补。

**Stage update**: 2026-08-05 — **Round 113 移动 H5 轻端**：在现有 admin（Umi/antd，375px 已适配）上做「移动模式」增强，不拆独立 H5 应用（复用鉴权/RBAC/service 层与组件，最小维护面；PR 描述含方案评估）。新增移动工作台首页 `/m/home`（今日/近 7 日 订单数、销售额、毛利三指标卡，销售额复用 `GET /orders/stats/sales`、毛利复用 R110 `GET /reports/profit` 订单维度口径，缺进价行有「仅供参考」提示；关键待办入口 待付款确认/待采购/待签收/待发货/异常 复用运营工作台 `GET /dashboard/product-operations` todos 口径；告警摘要 + 批量发货快捷入口）与 `/m/me`（账号信息/常用入口/退出登录）。移动导航：<768px 全站底部固定 5 tab（首页/订单/采购/库存/我的，`MobileTabBar` 挂在 ProLayout `childrenRender`，宽屏不渲染），tab 按 `canAccessPath` 过滤、口径与侧栏菜单一致（三角色权限不变）；根路径窄屏默认进 `/m/home`。触屏：tab/按钮触点 ≥44px、待办/入口卡片式行、下拉刷新（`usePullToRefresh`，另有刷新按钮等价）、底部安全区 `env(safe-area-inset-bottom)`；非核心页面保持原响应式可浏览。PWA：新增 `manifest.webmanifest` + theme-color（可安装加分项）。零后端改动、零 API/权限/状态机变更。新增前端单测 `mobileNav.test.ts` 与 E2E `round113-mobile-h5.spec.ts`（375px 首页指标/待办/告警/跳转/底部导航触点 + 1440 桌面回归不渲染 tabbar），存量 375px 相关 E2E 全量回归通过。

**Stage update**: 2026-08-05 — **Round 112 轻量多仓库存**：库存模块新增租户级仓库维度（保持轻量：不做库位/波次/拣货路径）。数据模型：`warehouses`（租户级，每租户唯一默认仓，默认仓不可删除/停用；可增删改启停、扣减优先级）+ `warehouse_stocks`（仅存非默认仓库存；默认仓库存 = `product_skus.stock` − 非默认仓库存之和，兼容字段口径不变、存量数据零迁移零丢失）；`inventory_change_logs` / `order_inventory_effects` 挂 `warehouse_id`（NULL 视为默认仓历史）；启动迁移幂等补建各租户默认仓，`GET /inventory/warehouses/migration-preview` 提供迁移预检（SKU 总量/非默认仓库存/默认仓推导库存/孤儿行/负推导/一致性）。动线：库存中心按仓筛选 + 行内分仓库存 Tag；采购签收入库选仓（默认仓预选）；订单扣减可显式选仓、未指定按仓库优先级（priority 升序）扣减；仓间调拨 `POST /inventory/transfers` 单事务原子（行锁、源仓不足整体回滚、停用仓拒收、同仓拒绝），成功生成 `warehouse_transfer_out`+`warehouse_transfer_in` 两条配对流水。报表：R110 库存报表支持 `warehouseId` 按仓筛选与全仓汇总，价值/周转/滞销/低库存与库存中心同口径。阈值/告警口径：阈值仍配置在 SKU 级、按全仓总量判定，告警文案点名各仓库存（如「低库存（默认仓 2 / 华南仓 0）」）便于定位补货仓。demoseed 补第二仓 `DEMO-WH-2` 与分仓库存/出入库/调拨样本流水，clean/verify 零残留（租户默认仓为基线保留）。权限：仓库/调拨写操作沿用库存写口径（readonly 403）、租户隔离、越权 404。新增 8 个仓库 REST API（见 `docs/api.md`），契约测试 58 端点，Admin 新增 `/inventory/warehouses` 仓库管理页（含迁移预检面板）、调拨弹窗、签收/扣减/报表/流水选仓组件；后端单测 + E2E `round112-warehouses.spec.ts`（10 例，五档视口）已补；Docker（Postgres+Redis）实测存量迁移预检、签收入库选仓、订单按仓扣减、调拨原子性与回滚、三角色权限。

**Stage update**: 2026-08-04 — **Round 111 面单模板 + 发货规则 + 打单工作台（纯产品，不接真实面单 API）**：新增 `backend/internal/modules/waybill` 模块——租户级面单打印模板（`waybill_templates`，尺寸 `100x180`/`100x150`/`a4_list`，显示字段勾选 收发件人/商品明细/备注/物流商 logo 位，自定义页眉页脚，单默认模板，幂等预置三模板且预置不可删）与发货规则（`shipping_rules`，目的省份/重量段/金额段/平台 → 指定已启用 carrier，优先级升序、可启停、可解释推荐，未命中明确返回可手动选择），9 个 REST API + 订单侧 3 个端点（`/orders/print/sheets?templateId=`、`/orders/print/mark`、`/orders/shipping-recommendations`，见 `docs/api.md`「面单模板与发货规则（round111）」）。订单新增 `waybill_printed_at` 打单状态（独立于发货状态，不改发货流程语义、不动库存）。Admin 新增 `/settings/waybill-templates`、`/settings/shipping-rules` 管理页；`/orders/print` 打印页按所选模板渲染（三种尺寸 @page 分页边距适配、页眉页脚、logo 占位、明确「非电子面单」口径）+「标记已打单」；订单列表新增打单状态列，批量发货与单个发货弹窗接入「按规则推荐物流商」（可手动覆盖，不强制）。demoseed 补 4 条 `DEMO-` 发货规则样本，clean/verify 零残留（预置模板作为租户基线保留）。权限：写端点 `settings.manage` + readonly 403、租户隔离、越权 404。后端单测 + 契约测试（11 端点）+ Admin E2E `round111-waybill-rules.spec.ts`（13 例）已补。明确边界：不声称电子面单/真实物流 API 已接通，凭证到位后可替换真实面单渠道。

**Stage update**: 2026-08-04 — **Round 109 AI 优化联动违禁词（增强，依赖违禁词模块）**：AI 标题优化与 AI 描述生成在调用模型前将当前租户启用的禁止级违禁词注入 system prompt 要求规避（上限 200 词，词库读取失败不阻断生成）；生成成功后自动用租户启用词库复检输出，残留命中通过响应可选字段 `bannedWordHits` 返回（词/类别/级别/建议），草稿详情 AI 结果面板以 Alert 提示（禁止级红色、警告级黄色，仅提示不阻断应用）。实现为 `product.ComplianceAdvisor` 接口 + `bannedwords.AIComplianceAdvisor` 适配器（避免模块循环依赖），router 装配注入；后端单测与前端类型已补，`docs/api.md` 已同步。不改变既有 API 字段、权限与状态机。

**Stage update**: 2026-08-04 — **Round 109 违禁词合规检测（第一项）**：新增 `backend/internal/modules/bannedwords` 模块——租户级违禁词库（`banned_words` / `banned_word_category_states`，预置基础库幂等 seed：广告法极限词 / 通用违禁词 / 医疗功效词 / 品牌侵权词，预置只读可启停、租户可增删自定义词）、Unicode 码点级扫描引擎（标题 / AI 标题 / 详情 / AI 描述，命中返回词/字段/位置/类别/级别/建议）、8 个 REST API（词库 CRUD、分类启停、单品检测、批量检测 ≤100）；发布检查（readiness）接入 `compliance` 分组：禁止级命中 `compliance.banned_word_forbidden` 产生 error 阻断 `canPublish`，警告级 `compliance.banned_word_warning` 仅提示。Admin 新增 `/settings/banned-words` 词库管理页（预置只读/自定义增删/分类启停/readonly 禁用）、草稿详情 readiness 页「合规检测」面板（命中明细 + 原文高亮）、草稿列表「批量违禁词检测」抽屉。权限：写操作 readonly 403、越权 404，permmatrix 已覆盖新端点；后端单测 + 前端单测 + 契约测试 + round109 E2E 已补。AI 优化自动规避违禁词（AI 联动）本轮未实现，列为后续清单。

**Stage update**: 2026-08-06 — **刊登记录 external_spu_id 列名漂移修复 + demoseed 租户守卫**（源自 goofish 真实 E2E 发现）：1）`product_publications.ExternalSPUID` GORM 默认列名为 `external_sp_uid`，而 `productpublish` 原生 SQL 更新写 `external_spu_id`，导致发布成功后 publication 更新报 `SQLSTATE 42703`、草稿「刊登记录」停留在「刊登中」——model 显式 `column:external_spu_id`，`database.AutoMigrate` 新增 `migrateLegacyPublicationColumns` 幂等重命名旧列，补 schema 回归测试；2）demoseed `Seed` 拒绝 `tenant_id<=0`（publish/order worker 的 tenant gate 会拒绝此类任务），`cmd/seeddemo` 自动解析到非正租户时回退 tenant 1 并告警，补回归测试。未改任何 API/权限/状态机。

**Stage update**: 2026-08-04 — **Round 110 报表深度对齐竞品（利润/采购/库存三报表）**：新增只读报表聚合模块 `backend/internal/modules/reports`（GET-only，readonly 可用，租户隔离）：1）利润报表 `GET /api/v1/reports/profit(+/export.csv)` 按订单/商品/店铺三维度，收入-采购成本-（可配置费用项 settings `report_profit.fee_items`）=毛利/毛利率，多币种沿用 `report_currency` 手工汇率口径（big.Rat 精确折算，无汇率显式列入 `unconvertedCurrencies` 不伪造），采购成本复用采购成本预估口径（新增 `procurement.ResolveLineCostRefs` 包装），缺进价行计入 `missingCostLines`；支持 7/30/90 天与自定义区间（≤366 天）、CSV 导出（原币+折算列，UTF-8 BOM）；店铺 scope 与订单列表一致。2）采购报表 `GET /api/v1/reports/procurement`：金额/单量/在途/已签收/已取消、每日趋势、下单→签收天数分布（0-3/4-7/8-15/16+ 天）、供应商聚合排行。3）库存报表 `GET /api/v1/reports/inventory`：库存价值（主货源进价，缺失回退 SKU 成本价，均缺列 `unvaluedSkuCount`）、周转天数（近 30 天出库速度）、滞销预警（`slowDays` 无出库）、低库存清单（与 `warning_stock` 阈值联动）。管理端新增 `/orders/reports-profit|reports-procurement|reports-inventory` 三页与经营报表并列（chartTokens、三视口、懒加载图表、未折算/缺进价提示）。后端 SQLite 回归测试（含多币种折算、店铺 scope、租户隔离、CSV）、API 契约、前端服务单测、Playwright E2E（round110-deep-reports）与 `docs/api.md` 已同步。未改任何写操作/权限/状态机。

**Stage update**: 2026-08-06 — **Goofish（闲鱼）Platform Provider Beta**：新增 `backend/internal/providers/platform/goofish`，通过自托管发布桥接服务（持有已登录闲鱼浏览器会话，`GET /health` / `POST /publish` Bearer 鉴权）实现 `product_publish` + `shop_info` 能力，这是首个具备真实已验证发布通道的平台 Provider（桥接服务在外部闲鱼自动化仓库维护，已用真实登录态验证 /health）。注册于 `internal/api/router.go`，`ProductPublishImplementationStatus` 将 `goofish` 归入 beta，平台中文标签（`opslabels` / admin `platformLabels`）同步为「闲鱼」，`docs/provider.md` 已更新。发布串行互斥、受平台风控约束不并发；未改任何 API/权限/状态机。

**Stage update**: 2026-08-04 — **Round 109 客服话术模板库**：新增租户级客服快捷回复话术模板（`customer_reply_templates`，分组固定 售前/售后/物流/退款/其他，正文支持 `{订单号}`、`{买家昵称}`、`{物流单号}`、`{商品名}`、`{店铺名}` 变量占位）。后端 `customerchat` 模块新增 5 个端点（列表/新增/更新/删除/组内重排，见 `docs/api.md`「客服话术模板（round109）」），写端点复用 `adminperm.CanWriteCustomer` 口径（readonly 403）、按租户隔离、写操作记 operation log；AutoMigrate 注册，demoseed 补 6 条 `DEMO-` 中文模板样本（含停用样例），clean/verify 覆盖零残留。Admin 新增「客服 → 话术模板」管理页（分组 Tabs、搜索、增删改、启停、上下移排序、无写权限禁用）与会话详情回复框「插入话术模板」弹窗（分组+搜索+一键插入，变量按会话上下文自动填充、缺失变量保留占位并提示；插入后仍在现有编辑框可编辑，发送仍走人工确认，不引入自动外发，与 AI 建议草稿并存）。补后端单测（校验/租户隔离/排序/readonly 403 路由回归）、API contract（5 端点）、前端单测（变量填充/服务层）与 Admin E2E `round109-reply-templates.spec.ts`（分组/搜索/写请求拦截/readonly 禁用/插入填充/加载失败重试/三视口无横向溢出）。未改任何既有 API/权限/状态机。

**Stage update**: 2026-08-04 — **Round 102 季度复查 P2×4 修复**（源自 PR #214 QA 评论）：1）刊登 payload 校验用户可见 message 中文化——`operationtask` execute/retry 校验失败 message 中文化（HTTP 400 / 业务码 40001 / `errorCode=execution_validation_failed` 口径不变），`productpublish` 新增 `localizePublishError` 集中映射 `invalid shopId` / `targets required` / `product main image required for publish` / `douyin main image required` 等已知英文校验源；2）operator 访问受限路由统一语义页——新增 `RouteAccessGuard`（ProLayout `childrenRender` 级兜底）与 404 页统一「无法访问该页面」中文口径，不泄露资源存在性；3）无 SMTP 注册降级——`send-email-code` 对 SMTP 未配置返回 503 + 中文引导，新增显式开关 `AUTH_REGISTER_SKIP_EMAIL_VERIFY`（默认 false，staging/production 设 true 启动报错）与公开 `GET /auth/register-config`，登录页注册表单按配置隐藏验证码，`.env.example` / `.env.docker.example` / `docs/env.md` / `docs/api.md` 已同步；4）采集原始错误中文兜底——`localizeCollectErrorMessage` 对未识别英文 raw error 回退通用中文提示，采集任务/批次/监控/事件抽屉展示入口统一接入；后端与前端均补回归测试。未改任何业务状态机/权限模型。

**Stage update**: 2026-08-04 — **Round 94 第二平台预研（TikTok Shop / Shopee）**：新增第二平台调研文档 [`platform-integration.md`](platform-integration.md)（两平台开放平台商品发布 API 的鉴权模式、商品创建必填字段、类目/属性、图片上传、限频与开发者账号申请步骤，供老板申请；与现有 `tiktok` / `shopee` adapter 与 `local_draft_only` 降级口径对齐）。demoseed 刊登样本补齐虾皮（Shopee）：新增 `DEMO-SHOP-4` 虾皮演示店与 `platform_shopee` 降级刊登能力预设（DEMO 占位值仅解锁 `local_draft_only`，检测到既有配置整组跳过、清理仅删 `DEMO-` remark 行，幂等）；`docs/module-map.md`、`docs/provider.md`、`docs/development.md` 同步。本轮不接真实平台 API（无凭证），未改任何 API/权限/状态机。

**Stage update**: 2026-08-03 — **Round 75 demoseed 刊登链路样本增强**：`seeddemo` 新增刊登链路演示数据——TikTok DEMO 店铺（`DEMO-SHOP-3`）+ `platform_tiktok` 降级刊登能力预设（DEMO 占位值仅解锁 `local_draft_only`，不伪造真实凭证，检测到既有配置时整组跳过、清理仅删 `DEMO-` remark 行）、第 2 条待审运营任务（`DEMO-OT-REVIEW-2`，支持批量批准/驳回走查）、1 条已绑定抖店 publication（`DEMO-DY-3502001`，含 bound + unmatched 两条 `product_publication_skus` SKU 绑定样本与 platformSkus 候选缓存）。cleanup/verify 同步覆盖 `product_publication_skus`、`settings` 预设与 `external_product_id` 口径，保持幂等、clean 零残留与 production 拒绝；未改任何 API/权限/状态机。

**Stage update**: 2026-08-02 — **Round 48 数据处置缺口（UX P2-7/P2-8）closed**：采购单新增 `voided` 软作废状态（仅 `delivered / failed / cancelled` 可作废，`POST /procurement/orders/:id/void`，可写角色专属，保留 `purchase_order_events` 审计与 `procurement.void` 操作日志，已入库库存不自动回滚）；作废单不再参与统计/待办/生成防重覆盖判定（order `hasPurchase`、orderexception 采购受阻、operationdashboard 待采购、procurement generate 口径同步排除 `voided`）。货源档案新增孤儿货源处置：`GET /product-sources/orphans` 列出关联商品已删除的货源，`DELETE /product-sources/:id` 解绑（软删除货源及 SKU 映射，商品仍存在时 409），解绑后供应商可删除；供应商管理页内置孤儿货源列表，商品删除弹窗提示绑定货源数。后端单测覆盖作废状态机/审计/统计排除/readonly 403 与孤儿识别/解绑/供应商删除链路。

**Stage update**: 2026-07-28 — **Phase P9 Batch 1 Domain Persistence Completed Locally / Full Verification Pending**：完成 `P9-501` Inventory Sync Run Model、`P9-502` Inventory Snapshot Model、`P9-503` SKU Binding Model、`P9-504` SKU Binding Calibration Model、`P9-505` Manual Binding Fallback Model、`P9-506` Migration, Repository and Persistence Verification 的 Batch 1 领域及持久化基础。新增 `backend/internal/modules/inventorysyncp9`，包含五类 GORM 领域模型、tenant-scoped Repository、稳定领域错误、幂等 key hash + input fingerprint、乐观并发 revision、不可变 snapshot/calibration 历史、Postgres/SQLite 迁移约束与真实 SQLite 持久化/并发测试；`backend/internal/database/migrate.go` 已注册 `inventorysyncp9.Migrate(db)`。新增 [`P9_TASK_BATCH_1_DOMAIN_PERSISTENCE.md`](P9_TASK_BATCH_1_DOMAIN_PERSISTENCE.md) / [`p9-task-batch-1-domain-persistence.json`](p9-task-batch-1-domain-persistence.json)、`scripts/p9-task-batch-1-domain-persistence-gate.mjs` 与 `tests/gates/p9/task-batch-1-domain-persistence.mjs`。本批未实现校准服务、匹配算法服务、同步编排、Worker、Cron/Ticker、队列消费者、HTTP/Gin/REST API、Admin UI、前端 API Client、真实 Douyin Provider、OAuth、真实凭证、真实平台网络、真实库存读取或写入。状态：**P9 In Progress** · **P9 Batch 1 Domain Persistence Completed Locally** · **Verified with known planning gate boundary** · **P9-601..P9-606 Not Started** · **P9 Not Complete** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformNetworkEnabled=false** · **realPlatformReadEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **P10 boundary preserved**。

**Stage update**: 2026-07-28 — **Phase P9 Canonical Scope Resolved / Owner Scope Decision Approved / Full Implementation Plan Ready / Batch 1 Scope Ready**：从当前 `dev` 分支与真实 `HEAD` 重新完成 P9 发现，归一为 **Douyin Shop inventory sync MVP with SKU binding calibration and manual binding fallback**。仓库内 P9 参考被分为 current / historical / completed / conflicting 四类，当前无冲突，历史 Phase 9 继续保留为实现证据而不直接复用为当前任务名。新增 [`P9_OWNER_SCOPE_DECISION.md`](P9_OWNER_SCOPE_DECISION.md) / [`p9-owner-scope-decision.json`](p9-owner-scope-decision.json)、[`P9_SCOPE_DISCOVERY.md`](P9_SCOPE_DISCOVERY.md) / [`p9-scope-discovery.json`](p9-scope-discovery.json)、[`P9_EXECUTION_PLAN.md`](P9_EXECUTION_PLAN.md) / [`p9-execution-plan.json`](p9-execution-plan.json)、[`P9_TASK_BATCH_1_SCOPE.md`](P9_TASK_BATCH_1_SCOPE.md) / [`p9-task-batch-1-scope.json`](p9-task-batch-1-scope.json)、`scripts/p9-entry-gate.mjs`、`tests/gates/p9/entry.mjs`、`scripts/p9-plan-final-gate.mjs`、`tests/gates/p9/plan.mjs`、`scripts/p9-task-batch-1-scope-gate.mjs`、`tests/gates/p9/task-batch-1-scope.mjs`。状态：**P9 Scope Resolved** · **P9 Owner Scope Decision Approved** · **P9 Execution Plan Ready** · **P9 Batch 1 Scope Ready** · **P9 Implementation Not Started at planning gate time** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **P10 boundary preserved**。
**Stage update**: 2026-07-26 — **Phase P8 Development Complete / Batch 9 Final Integration Closed**：完成 `P8-701 Integration Fixtures`、`P8-702 API / Admin E2E Fixtures`、`P8-703 Platform Boundary Final Gate`、`P8-704 P8 Final Gate`、`P8-705 Closure Evidence`。Batch 9 验证真实本地 backend Bearer-token 登录链路，不使用静态前端 API mock、fake role、auth middleware bypass 或 RBAC backdoor；Playwright 覆盖 API 未认证保护、Admin 未登录跳转并保留目标、认证后 Operation Task Center 渲染。新增 [`P8_TASK_BATCH_9_FINAL_INTEGRATION.md`](P8_TASK_BATCH_9_FINAL_INTEGRATION.md)、[`p8-task-batch-9-final-integration.json`](p8-task-batch-9-final-integration.json)、[`P8_DEVELOPMENT_CLOSURE.md`](P8_DEVELOPMENT_CLOSURE.md)、[`p8-development-closure.json`](p8-development-closure.json)、`scripts/p8-task-batch-9-final-gate.mjs` 与 `tests/gates/p8/task-batch-9.mjs`。状态：**P8 Development Complete** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **humanConfirmationRequired=true** · **P7 conditional/deferred evidence preserved** · **Final Production Acceptance deferred to P10** · **Tag deferred** · **Release deferred**。

**Stage update**: 2026-07-20 — **Phase P8 Batch 2 Approval / Execution / Audit Persistence Completed / P8 In Progress**：完成 `P8-103 Approval Record`、`P8-104 Execution Attempt and Error`、`P8-105 Task Event Audit Model`。在 `backend/internal/modules/operationtask` 继续扩展 `approval_records`、`execution_attempts`、`execution_errors`、`operation_task_events`，包含 tenant-scoped Repository、审批/执行幂等约束、AttemptNumber / Error Sequence / Event Sequence 唯一约束、审批 draft version/hash 绑定、执行 approval reference 绑定、不可变审批/错误/事件触发器与并发约束测试；新增 [`P8_TASK_BATCH_2_APPROVAL_EXECUTION_AUDIT_PERSISTENCE.md`](P8_TASK_BATCH_2_APPROVAL_EXECUTION_AUDIT_PERSISTENCE.md)、[`p8-task-batch-2-approval-execution-audit-persistence.json`](p8-task-batch-2-approval-execution-audit-persistence.json)、`scripts/p8-task-batch-2-final-gate.mjs` 与 `tests/gates/p8/task-batch-2.mjs`。P8 Batch 1 Completed: `P8-101`、`P8-102`、`P8-106`；P8 Batch 2 Completed: `P8-103`、`P8-104`、`P8-105`。Batch 2 Source Status: `workingBranch=dev`、`committed=true`、`checkpointStatus=created`、`workingTreeDirty=false`。本批未实现状态机、审批服务、执行编排、重试服务、API、Admin UI 或真实平台写入。下一批建议：`P8-201 Task State Machine`、`P8-202 Draft Version Service`、`P8-203 Approval Service`。状态：**P7 Conditionally Closed** · **P8 In Progress** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **P7 deferred performance preserved** · **P10 production boundary preserved**。

**Stage update**: 2026-07-20 — **Phase P8 Batch 1 Domain Persistence Completed / P8 In Progress**：完成 `P8-101 Operation Task Model`、`P8-102 Platform Draft Model`、`P8-106 Migrations and Repository Tests`。新增 `backend/internal/modules/operationtask`，包含 `operation_tasks`、`platform_drafts`、tenant-scoped Repository、幂等/版本唯一约束、PayloadHash/AdapterMode 校验、并发唯一约束测试；新增 [`P8_TASK_BATCH_1_DOMAIN_PERSISTENCE_AND_REPOSITORY.md`](P8_TASK_BATCH_1_DOMAIN_PERSISTENCE_AND_REPOSITORY.md)、[`p8-task-batch-1-domain-persistence-and-repository.json`](p8-task-batch-1-domain-persistence-and-repository.json)、`scripts/p8-task-batch-1-final-gate.mjs` 与 `tests/gates/p8/task-batch-1.mjs`。本批未实现审批、执行、API、Admin UI 或真实平台写入。下一批建议：`P8-103 Approval Record`、`P8-104 Execution Attempt/Error`、`P8-105 Task Event/Audit Model`。状态：**P7 Conditionally Closed** · **P8 In Progress** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **P7 deferred performance preserved** · **P10 production boundary preserved**。

**Stage update**: 2026-07-20 — **Phase P8 Canonical Scope Resolved / P8 Planned**：P8 owner decision `P8-OWNER-SCOPE-DECISION-20260720` 正式批准当前 P8 scope：Operations Task Center, Draft Orchestration and Human Review Loop MVP。新增 [`P8_OWNER_APPROVED_SCOPE_DECISION.md`](P8_OWNER_APPROVED_SCOPE_DECISION.md)、[`p8-owner-approved-scope-decision.json`](p8-owner-approved-scope-decision.json)、[`P8_EXECUTION_PLAN.md`](P8_EXECUTION_PLAN.md)、[`p8-execution-plan.json`](p8-execution-plan.json)、`scripts/p8-plan-final-gate.mjs` 与 `tests/gates/p8/plan.mjs`；[`P8_CANONICAL_SCOPE_DISCOVERY.md`](P8_CANONICAL_SCOPE_DISCOVERY.md) / [`p8-canonical-scope-discovery.json`](p8-canonical-scope-discovery.json) 已更新为 `canonicalScopeResolved=true`、`scopeConfidence=high`。历史 `Douyin Phase 8 order sync MVP` 已完成且不作为当前 post-P7 P8 scope 复用。状态：**P7 Conditionally Closed** · **P8 Planned** · **P8 Execution: notStarted** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **P7 deferred performance preserved** · **P10 production boundary preserved**。

**Stage update**: 2026-07-20 — **Phase P7 Conditionally Closed / Ready for P8**：P7 功能与开发范围已完成，开发关闭以 `engineering_waiver` + `conditional_development_acceptance` + `known_risk_acceptance` + `deferred_capacity_acceptance` 方式有条件接受。新增 [`P7_CONDITIONAL_DEVELOPMENT_CLOSURE_AND_ENGINEERING_WAIVER.md`](P7_CONDITIONAL_DEVELOPMENT_CLOSURE_AND_ENGINEERING_WAIVER.md)、`docs/p7-conditional-development-closure-and-engineering-waiver.json`、`scripts/p7-conditional-development-closure-final-gate.mjs`、`scripts/p8-entry-gate.mjs` 与 12 项条件关闭 fixture。状态：**Functional Scope: Completed** · **Development Closure: Conditionally Accepted** · **Capacity Acceptance: Deferred** · **Dedicated Host Validation: Deferred to P10** · **Ready for P8: Yes** · **Production Ready: No**。历史 Formal Regression 失败、B-C-C-B 未完成、Host Isolation V2/V3 失败或不完整、C2 Dataset Post-Build Barrier 失败、Dedicated Host Matrix 未执行均保留为 audit evidence；不得声明 P7 Capacity Passed、Performance Regression Resolved 或 Production Ready。

**Stage update**: 2026-07-18 — **Phase P7-V2-R3B-BINARY-BOUND-B-C-C-B-REPEATABILITY-MATRIX Completed / FORMAL-TAIL-ROOT-CAUSE-CLASSIFICATION Completed（diagnostic-only；formal rerun not started）**：Created clean diagnostic worktree `/home/root/worktrees/trademind-ai-p7-binary-repeatability-20260718065605` from `formalPlanCheckpoint=e578678467ecd68d6e5880bd15c122dd904c2f99`, copied only the failed Formal Pair evidence and the two frozen binaries, and ran exactly `B1 -> C1 -> C2 -> B2` with the same frozen input sequence, branch mix, medium dataset size, VUs, stages, and duration. Matrix ID `p7v2-diag-binary-repeatability-20260718071052`; all four runs completed with `actualRows=1900150`, binary SHA matches (`B=4e0408ac29b777beac7598872dfd8cac6430542eae37b5ba304c3dbdf7bd79f1`, `C=c6564cfe47a9f5cc60ce6b8c7c29accf7460e213318ba62a11c0d480f4f47766`), `processIdentityProbeVersion=2`, `probeMethod=linux_procfs`, and final gate `failed=0`. Root cause classified as `A_formal_harness_repeatability_or_order_bias_defect` with `confidence=medium`: B1/B2 self-variance was large (`Webhook Ingestion p95 +74.20%`, `Auth Invalid Login p95 +298.23%`) and C1/C2 also changed direction/magnitude (`Webhook p99 -32.38%`, `Auth p95 +31.89%`), so the B/C tail difference is not stable enough to claim deterministic business regression. Diagnostic DBs were dropped exactly; diagnostic DB residuals, connections, related processes, and listener `18080` are all `0`. No fifth run, no Formal Pair rerun, no Soak/Demo/Final Race/Final Gates, no threshold/SLO/materiality/VU/stage/dataset changes, no business code change, no push/tag/release/production readiness. **Phase P7-V2 Incomplete · Phase P7 Development Closure Incomplete · Tag deferred · 非 Production Ready · Final Production Acceptance Deferred to P10**。

**Stage update**: 2026-07-18 — **Phase P7-V2-R3B-FORMAL-EMPTY-RUN-ID-INCIDENT-CLOSEOUT Completed / FORMAL-INVOCATION-CONTRACT-V2 Completed at tooling fixture level / PREFLIGHT-BINDING-V3 Completed at tooling fixture level（formal pair not started）**：Preserved the Stage L empty `--run-id` invocation incident as historical audit evidence; added manifest-bound formal invocation helpers/controller, `env:start --formal` guards, empty-run-id zero-side-effect rejection, Preflight Binding V3 canonical manifest selection, and final gate `scripts/p7-v2-r3b-formal-invocation-preflight-v3-final-gate.mjs`. Recovery6 planned Run IDs remain retained and unconsumed; no Dataset/k6/Baseline/Current/Comparability/Regression/Soak/Demo/Final Race was started. B/C frozen binaries are still expected at the existing SHA-256 values and were not rebuilt by this tooling repair. **Phase P7-V2 Incomplete · Phase P7 Development Closure Incomplete · Tag deferred · 非 Production Ready**。

**Stage update**: 2026-07-18 — **Phase P7-V2-R3B-WSL-PROCESS-IDENTITY-PROBE-V2 Completed at tooling fixture level / FORMAL-PAIR-REPEATABILITY-AUDIT Incomplete**：`scripts/p7-v2-process-identity.mjs` now uses `processIdentityProbeVersion=2`; Linux/WSL uses native procfs (`/proc/<pid>/exe|cwd|cmdline|environ|stat`) and no longer spawns `wsl.exe`, while Windows remains the only path allowed to call the versioned system `wsl.exe`. External `wsl.exe` shims are rejected as `unversioned_process_probe_shim_detected`, PID reuse blocks kill identity matching, and `Exec format error` is a non-zero semantic failure. Added repeatability/order-bias audit gate `scripts/p7-v2-r3b-formal-repeatability-audit-final-gate.mjs` plus `docs/P7_V2_R3B_FORMAL_PAIR_REPEATABILITY_AND_ORDER_BIAS_AUDIT.md` / JSON. Current Recovery6 failed pair is preserved as historical audit evidence and remains invalid for closure; B-C-C-B diagnostic runs have not been executed, binary/input binding is not yet proven, root cause is not classified, and no formal rerun/soak/demo/final gate was started. **Phase P7-V2 Incomplete · Phase P7 Development Closure Incomplete · Tag deferred · 非 Production Ready**。

**Stage update**: 2026-07-17 — **Phase P7-V2-R3B-WEBHOOK-TAIL-ROOT-CAUSE-CLOSE Completed / WEBHOOK-TAIL-MINIMAL-REPAIR Completed locally（formal rerun not started）**：Confirmed the frozen Recovery6 formal regression has exactly two failed metrics, both `Webhook Ingestion` (`p95`, `p99`), with `notComparableCount=0`, `invalidMetricCount=0`, `insufficientSampleCount=0`, and `summaryStatMissingCount=0`. Added webhook tail repair evidence and final gate docs: `docs/P7_V2_R3B_WEBHOOK_TAIL_REGRESSION_REPAIR.md`, `docs/p7-v2-r3b-webhook-tail-regression-repair.json`, and `scripts/p7-v2-r3b-webhook-tail-repair-final-gate.mjs`. Branch-mix evidence is explicit: frozen formal artifacts do not contain seed/duplicate sequence hashes, so formal branch-mix comparability is not retroactively proven; non-formal diagnostics show duplicate ratio `0 -> 0`. The minimal repair path remains the existing webhook normal-insert reload removal with duplicate conflict reload preserved (`normalInsertQueryCount=1`, `duplicatePathQueryCount=2`, `dataRaces=0`). No thresholds/SLO/materiality/VUs/stages/dataset/request mix were changed. New Runtime Freeze and a new Formal Pair are still required before P7-V2 closure. **Phase P7-V2 Incomplete · Phase P7 Development Closure Incomplete · Tag deferred · 非 Production Ready**。

**Stage update**: 2026-07-15 — **Phase P7-V2-R3B-LPC-R3-GATEFIX Completed（scoped gatefix；formal execution not started）**：Canonical Load Profile 的 schema/fingerprint V3 preflight 与 20 次 determinism evidence 均通过（unique fingerprint count=1）；runKind/runId/timestamp/path 表达不参与 fingerprint，stage 顺序、duration、targetVUs、configuredVUs、scenario executor/weight 和 load script SHA-256 均会改变 fingerprint。Regression 显式支持 V1/V2/V3，并拒绝混合版本、未知版本、缺失/非法 SHA-256、空 canonical profile/stages 与非法 stage；Freeze、Resolver、Comparability、Regression、Registry、Scoped Gate、Fast-Close 消费者 V3 compatibility 均通过。Recovery5 `p7v2-baseline-r3b-recovery5-20260715091700` 保留并标记 `aborted_before_execution`；已生成新的 Recovery6 planned Run IDs，未创建 Runtime Freeze、未启动环境、未执行 Dataset/k6/Baseline/Current/Comparability/Regression/Soak/Demo，Registry active entry 未修改。Windows determinism 已执行，WSL cross-platform 比对为 `not_executed`，没有伪造匹配结论。下一步严格从 **Pre-Freeze Verification → Runtime Freeze → Recovery6 Baseline** 开始。**Phase P7-V2-R3B-FAST-CLOSE-R3 Incomplete · Phase P7-V2 Incomplete · Phase P7 Development Closure Incomplete · Tag deferred · 非 Production Ready**。

**Stage update**: 2026-07-15 — **Phase P7-V2-R3B-FAST-CLOSE Incomplete（fail-fast stop）**：Recovery4 harness remediation and fixtures completed: formal k6 summaries include `p(99)`; missing summary statistics fail explicitly rather than becoming zero; Task List and Webhook Ingestion use independent steady-window metrics; invalid-login and invalid-webhook-signature are separate route metrics; Recovery4 runs use mock providers, isolated PostgreSQL data, and `127.0.0.1:18080`. Recovery4 Baseline `p7v2-baseline-r3b-recovery4-20260715075855` was frozen (`74490c7b63f53ae63d3a65df87df760fb61ecbdbb909af542af5b817b4381f32`) and independent Current `p7v2-current-r3b-recovery4-20260715075855` was frozen (`3dcf01d1e98e9b03cf1897c13aa4527d1216e12acef05381dd0a547d5cd8afc2`), but Comparability stopped with `mismatchCount=1` because the legacy load-profile fingerprint included run kind (`baseline` vs `current`). The fingerprint implementation was corrected to exclude run kind; therefore these two artifacts are preserved but invalid for the next formal comparison, and a new Recovery4 Baseline → immediate Freeze → independent Current → immediate Freeze is required. Regression, Soak, Demo, Stability/Race, Cleanup, and final gates were not executed. Recovery3 evidence and its failed regression remain unmodified. No production resources, real Provider/Douyin calls, auto-listing, tags, or Production Ready declaration. **Phase P7-V2 Incomplete · Phase P7 Closure Verification Incomplete · Tag deferred · 非 Production Ready · Final Production Acceptance Deferred to P10**.

**Stage update**: 2026-07-15 — **Phase P7-V2-R3B-PRR-A Completed（read-only diagnostic closure；execution still blocked）**：Recovery3 Baseline `p7v2-baseline-r3b-recovery3-20260715-131400` 与独立 Current `p7v2-current-r3b-recovery3-20260715-131400` 的冻结 Raw Artifact SHA-256、大小、JSON、请求数和场景覆盖均再次验证通过，原文件与 Freeze Manifest 未修改；Comparability V2 仍 `passed`（`mismatchCount=0`、`notComparableCount=0`）。Regression V2 失败证据已归档且未重算：3 个 p95 实质回归和 9 个 p99 违规。九个 p99 均为 Raw Trend 存在、样本充足、但 `p(99)` summary 缺失而 parser 默认输出 `0`（`summary_stat_missing`）；Task List 与 Webhook Ingestion 的真实 p95 右移因缺少原运行数据库状态、query plan、pool/lock 和主机遥测只能判定为 `statistical_variance_insufficient_evidence`（low）；Auth/Security 将 invalid-login 与 invalid-webhook 聚合为单一 gating trend，判定 `metric_tag_aggregation_bug`（high）。下一步为 **P7-V2-R3B-PRR-REPRO**：先进行 evaluator/harness 诊断性修复并执行 Recovery4，之后才可决定 Runtime 修复；不得修改阈值或重跑至通过。Soak、Demo、最终 Gate 未执行；未访问生产资源、未调用真实 Provider/Douyin、未自动上架、未创建 Tag，仍 **非 Production Ready**。

**Stage update**: 2026-07-15 — **Phase P7-V2-R3B-PORT-R2 Incomplete（safe stop）**：双侧端口审计确认 Windows 与 WSL2 均有 `8080` LISTEN，未停止无法由自动审计完整确认的原端口进程；专用端口选择 `127.0.0.1:18080` 通过，未影响无关服务。P7-V2 端口配置、host guard、进程身份、启动/停止与 comparability 已开始参数化，端口配置审计通过；但三轮短启动/停止探针因启动脚本返回失败、且遗留清理验证未满足而 `0/3`，未生成新的 Runtime Freeze、Baseline、Current 或冻结 Artifact。未访问生产资源、未调用真实 Provider/Douyin、未自动上架、未创建 Tag，仍 **非 Production Ready**。下一步：修复 restart probe 的启动结果与精确停止验证，重新执行三轮探针后以新的 recovery3 Run ID 重新冻结和执行 Baseline → Current。

**Stage update**: 2026-07-15 — **Phase P7-V2-R3B-REBASELINE2 Incomplete（truthful stop）**：修复后的 Harness fixture、k6 v0.57.0 discovery、host guard 和 preflight 均通过，已生成新的独立 Baseline/Current Run ID 并冻结运行时、load、metric、policy、dataset、SLO 与凭据矩阵指纹。Formal Baseline 启动前发现端口 `8080` 仍被未知进程占用，环境启动以 exit `1` 停止；未执行 Dataset、k6、Artifact Freeze、Registry 更新、Current、Comparability 或 Regression V2。仅删除本次明确创建的隔离数据库，未停止未知端口进程；未访问生产资源、未调用真实 Provider/Douyin、未自动上架、未创建 Tag。下一步：确认端口 owner 后使用新的 Baseline/Current Run ID 从完整链路重新开始。
**Stage update**: 2026-07-15 — **Phase P7-V2-R3B-CI-RG Incomplete（truthful stop）**：完成 Current 进程身份与证据链根因审计，根因分类为 `process_identity_insufficient`：旧 harness 仅比较 PID，未证明旧进程停止、端口 Owner、进程 start ticks、二进制 Hash 或 Instance Nonce。已实现 Linux/WSL Process Identity、Clean Start / Restart / PID reuse / stale PID 语义、Worker/Redis/Mock 拓扑证据、Current Independence 组合判定、冻结 Artifact Manifest Resolver 和 Current Registry；相关 fixture 与语法检查通过。Recovery Baseline `p7v2-baseline-r3b-recovery-20260714-1719` 的原始 Artifact SHA-256 仍验证通过且未修改；但现有 `runtimeSourceTreeHash` 把 `scripts/p7-v2-*.mjs` 纳入指纹，修复 Harness 后不能与该不可变 Baseline 严格相等，因此 Reuse Decision 为 **`rebaseline_required`**。按门禁未执行 Current、Freeze、Comparability、Regression V2、Soak、Demo 或最终 Gate；未访问生产资源、未调用真实 Provider/Douyin、未创建 Tag、仍 **非 Production Ready**。下一步：先用修复后的 Harness 生成新的 immutable Baseline，再执行 Current → Freeze → Comparability → Regression V2。
**Stage update**: 2026-07-14 — **Phase P7-V2-R3B-REBASELINE Incomplete**：新的 Formal Baseline `p7v2-baseline-r3b-recovery-20260714-1719` 在隔离 Medium Dataset 上完成，原始 k6 Artifact 已立即冻结到受控目录并完成完整 SHA-256 校验（`c373a484b15737b8dbc479340ea35488f69de8968895586ec1378a26e0a1e709`）。Independent Current `p7v2-current-r3b-recovery-20260714-1750` 已创建新隔离数据库并完成 Medium Dataset 重建，但 restart identity probe 返回 `apiProcessChanged=false`，因此 Current 在 k6 启动前被阻止；未生成或冻结 Current Artifact，未执行 Regression V2。**Phase P7-V2-R3B Execution Blocked** · **Phase P7-V2 Incomplete** · **Phase P7 Closure Verification Incomplete** · Tag deferred · **非 Production Ready** · Final Production Acceptance Deferred。
**Stage update**: 2026-07-14 — **Phase P7-V2-R3B-FIX Completed（Harness Closure Passed）**：修复 Soak 连续稳态/Cooldown 证据 Schema、Current 隔离重启/数据库重建证据、唯一 Current/Baseline 负载脚本映射、唯一逐场景 Regression Engine、Registry-only Baseline Resolver、深度 P1-P7/P7-V2 Gate、Stability/Race 阶段状态、Demo Manifest 接线及 R3B 命令。Gate Fixture Tests 通过；Dry Gate 按预期因未执行链路失败；未启动 Current / Regression / Soak / Demo，未访问生产资源。审计同时确认冻结 Baseline 的原始 k6 artifact 已缺失，且本次对测试/测量语义的修复使严格可比性不能证明，因此 **Frozen Baseline No Longer Comparable · Rebaseline Required Before R3B Execution**。**Phase P7-V2 Incomplete** · **Phase P7 Closure Verification Incomplete** · Tag deferred · **非 Production Ready** · Final Production Acceptance Deferred。
**Stage update**: 2026-07-14 — **Phase P7-V2-R3A Completed（scoped closure）**：纠正历史 Formal Baseline `p7v2-baseline-20260714181000`：提交的解析指标 `completedRequests=0`，因此 **Invalid for Regression / Preserved / Superseded**，原始历史证据未删除、未覆盖。审计 R3 Candidate `p7v2-r3-baseline-20260714133131`：原始 k6 summary 存在、`requests=29,475`、完整场景覆盖、SHA-256 已记录，但无法证明与同时期 runtime tree 的逐文件 provenance，故保留为 `provenance_incomplete`，不事后伪造 freeze。经逐项精确授权后清理 7 个 `trademind_p7v2_` 隔离库，活动连接仅针对批准库终止；前缀残留 0、性能 API/k6 进程残留 0、保留基础 Redis/PostgreSQL 本地服务。新 Formal Baseline `p7v2-baseline-r3a-20260714225500` 使用 k6 v0.57.0 与 10 VU 标准 profile，`completedRequests=29,475`、完整 9 场景、classified errors=0、thresholds/absolute SLO 通过；原始 artifact SHA-256 `1c324a90…f05954bf9e`、source/config/dataset/profile/SLO/route fingerprints 已冻结并注册。Comparability Precondition **passed**，Current execution 仅获准留待 **P7-V2-R3B**。验证：R3A harness syntax、`pnpm check:dev`、`pnpm check:ui-copy --strict`、`pnpm build:admin`、`pnpm build:collector`、`go fmt ./...`、`go mod verify`、`go test ./...`、`go build ./cmd/server/... ./cmd/p7load ./cmd/p7verify`、`git diff --check` 均通过。**Phase P7-V2 Incomplete** · **Phase P7 Closure Verification Incomplete** · Current / Regression / Soak / Demo / Final Gates Pending P7-V2-R3B · Tag deferred · **非 Production Ready** · Final Production Acceptance Deferred。
**Stage update**: 2026-07-14 — **Phase P7-V2-R2 完成（scoped closure）**：修复 Performance Bootstrap / auth 不稳定，打通 **auth probe → route probe → bootstrap 幂等 → auth stability（3/3）→ diagnostic load → formal baseline**。Formal Baseline `p7v2-baseline-20260714181000` **passed**（10 VU、20m profile、k6 exit 0、`unexpected401=0`）；Diagnostic `p7v2-diagnostic-20260714180000` passed。根因修复：性能模式 bootstrap 密码同步、`APP_ENV=performance` 跳过 `.env` 密码覆盖、harness 使用性能默认凭据、server 启停与 `runtime.env`、webhook HMAC（`printf` + k6 secret/data 顺序）、k6 import 路径。保留失败 baseline 证据：`p7v2-baseline-20260714143530`、`p7v2-baseline-quick`、`p7v2-baseline-20260714180000`。证据：`docs/P7_V2_R2_*`、`docs/p7-v2-r2-closure-report.json`。**未执行**（按 R2 范围）：Current / Regression / Soak / Demo / Final Gates。策略：**Phase P7-V2-R2 Completed (scoped)** · **Formal Baseline Passed** · **Phase P7-V2 Closure Verification Partial** · **Current/Regression/Soak/Demo/Final Gates Pending** · **Real Production Performance/Capacity/Peak Verification Deferred** · **非 Production Ready** · **Tag deferred**。
**Stage update**: 2026-07-14 — **Phase P7-V2 性能验收 Harness 已落地，但 Closure Verification 仍 Incomplete**。新增 `scripts/p7-v2-*` 全套入口（preflight、host guard、WSL 隔离环境、dataset、smoke/baseline/current/soak load、regression、race 复用、demo、P1-P7 gate、final closure）、`tests/load/p7v2-*.js` 与 `tests/load/lib/*`；`backend/cmd/p7load` 扩展 `trademind_p7v2_` 前缀；WSL2 安装 Redis 6、`pnpm` 脚本 `p7-v2:*` / `check:p7-v2`。已验证：Load Host Guard passed；WSL PostgreSQL 14 + Redis PONG；P7-V2 Medium 数据集 `actualRows=1,900,150`、`failedRows=0`、幂等复跑通过；Runtime Cleanup `trademind_p7v2_%` 残留 0；`go fmt` / `go mod verify` / `go test ./...` 通过；复用 P7-C4 Race `dataRaces=0`。阻塞：**k6 无法从 GitHub 下载/安装**（网络 SSL/超时），正式 Load / Baseline / Current / Regression / 30 分钟 Soak 均为 blocked；`demo:auto-acceptance` 两轮未执行；`scripts/p7-v2-final-closure-gate.mjs` **failed=11**。策略：**Phase P7-V2 Incomplete** · **Phase P7 Closure Verification Incomplete** · **Load Test Environment Blocked (k6)** · **P7-C4 Completed** · **Real Production Performance/Capacity/Peak Verification Deferred** · **非 Production Ready** · **Tag deferred**。在 WSL2 本地 PostgreSQL（Unix socket `/var/run/postgresql`，`APP_ENV=performance`、`PERFORMANCE_TEST_MODE=true`）确认非生产环境后，精确删除遗留库 `trademind_p7c4_p7c4_20260714042442`（连接数 0，终止 0）；`trademind_p7c4_%` 前缀查询 **0 行**，`remainingDatabasesWithPrefix=0`，无临时进程/端口残留。增强 `scripts/p7-c4-stop-runtime-env.mjs`（默认只检查报告、精确库名删除、未知残留只报不删）与 `scripts/p7-c4-final-closure-gate.mjs`（前缀残留、证据新鲜度、live 查询校验）；新增 `docs/P7_C4_R_CLEANUP_REPORT.md` / `docs/p7-c4-r-cleanup-report.json`。保留既有 P7-C4 运行证据（Medium 190 万行、Pagination / Query Plan / N+1 / Provider / Permission / Linux Race 均 passed，`dataRaces=0`）。`scripts/p7-c4-final-closure-gate.mjs`、`scripts/p7-c3-final-closure-gate.mjs`、`scripts/p7-c2-final-closure-gate.mjs`、`scripts/p7-c-capability-closure-check.mjs` 均为 **failed=0**；`go mod verify`、`go test ./...`、`go build ./cmd/server/... ./cmd/p7load ./cmd/p7verify`、`pnpm check:dev`、`pnpm check:ui-copy --strict`、`pnpm build:admin`、`pnpm build:collector`、`git diff --check` 通过。策略：**Phase P7-C4 Completed** · **Ready for Phase P7-V2** · **Phase P7 Closure Verification Incomplete** · **Load/Soak/Baseline/Regression/Demo Acceptance Pending P7-V2** · **Real Production Performance/Capacity/Peak Verification Deferred** · **非 Production Ready** · **Tag deferred**。
**Stage update**: 2026-07-13 — **Phase P7-C2 能力状态归一化、Medium 运行验证与 Linux Race 收口完成部分证据，但 Closure Verification 仍 Incomplete**。本轮在 WSL2 Ubuntu 22.04 隔离环境安装并使用 PostgreSQL 14，生成 `docs/p7-c2-runtime-environment.json` / stop 报告且清理 `trademind_p7c2_%` 临时库；Medium Dataset Resume Drill 真实执行 `plannedRows=1,900,150`，中断后保留 `rowsBeforeInterruption=3,150`，续跑插入 `1,897,000` 行，最终 `actualRows=1,900,150`、`duplicateRows=0`、幂等复跑 `insertedRows=0`、fingerprint 稳定；Linux Race 在 WSL2 实跑 `go mod verify`、`go test ./...`、`go build ./cmd/server/... ./cmd/p7load` 与 11/11 race package matrix，`dataRaces=0`、`deadlocks=0`。能力归一化 resolver 将 P7 mandatory partial 从 33 收敛到 11，但 `scripts/p7-c2-final-closure-gate.mjs` 仍为 `failed=17 / passed=17`，`scripts/p7-c-capability-closure-check.mjs` 仍为 `failed=4 / passed=5`。剩余阻塞：6 个业务 keyset cursor pagination runtime/wiring 未通过、Query Plan 与 N+1 runtime 因分页证据阻塞、provider concurrency limit / adaptive slowdown / permission cache invalidation 仍缺代码证据。策略：**Phase P7-C2 Incomplete** · **Phase P7 Closure Verification Incomplete** · **Medium Dataset Resume Drill Passed** · **Linux Race Verification Passed** · **Capability Normalization Partially Closed** · **Load/Soak/P8 not started** · **Real Production Performance/Capacity/Peak Verification Deferred** · **非 Production Ready** · **Tag deferred**。
**Stage update**: 2026-07-13 — **Phase P7-C 能力收口前置补齐进行中（Closure Verification Incomplete）**。本轮真实读取 `docs/p7-v-capability-completeness-audit.json`，确认 P7-V 仍为 `implemented=24 / partial=33 / missing=0`；修复 Cursor 防篡改基础，`backend/internal/pkg/pagination` 从纯 Base64 JSON 改为 HMAC 签名 Cursor，production 要求 `PAGINATION_CURSOR_SIGNING_KEY`；新增最小共享 `backend/internal/pkg/cache`，覆盖 TTL、最大 entries、失效、negative cache、singleflight 与租户安全 key 基础，解决 P7-V race matrix 指向不存在 cache 包的真实代码缺口；`backend/cmd/p7load` 增加 `--fail-after-batches` / `--stop-after-rows` 受控中断参数用于 resume drill。新增 P7-C resume、pagination、query-plan、N+1、Linux race 与 closure gate 脚本及报告入口，当前 `scripts/p7-c-capability-closure-check.mjs` 结果为 `failed=6 / passed=3`：mandatoryPartial 仍为 33，Dataset Resume/Pagination/Query Plan/N+1 需要隔离 Medium PostgreSQL 真实执行，Linux Race 需要 WSL2/Linux 执行；Race package mapping 已完整映射 11/11，Cache Decision 为 `cache_required_implemented`。策略：**Phase P7-C Incomplete** · **Cache Capability Decision Closed at code level** · **P7 Runtime Pre-Verification Pending** · **Linux Race Verification Blocked on current Windows host** · **Load/Soak Pending P7-V2** · **Real Production Performance/Capacity/Peak Verification Deferred** · **非 Production Ready** · **Tag deferred**。
**Stage update**: 2026-07-13 — **Phase P7-V Medium 数据集真实写入完成，但 Closure Verification 仍 Incomplete**。本轮将 `backend/cmd/p7load` 从 plan-only 记录器改为受控 PostgreSQL 批量写入器，支持 `--execute`、幂等续跑、runId 作用域清理、真实行数核验和 fingerprint；新增 `scripts/p7-v-start-performance-env.mjs` / stop、能力审计、load、soak、Linux race 与 final closure gate。当前在临时隔离 PostgreSQL 环境真实写入 Medium 数据集：`plannedRows=1,900,150`、`insertedRows=1,900,150`、`actualRows=1,900,150`、`failedRows=0`，同 runId 幂等复跑通过且 fingerprint 不变；Medium 写入暴露并修复 `product_sk_uid` / `external_sk_uid` GORM 字段标签迁移问题。当前阻塞：能力审计仍有 33 个 partial；k6 不可用导致 Load blocked；Soak 未执行 30 分钟；Regression baseline/current 缺失；Linux Race 在 WSL2 实际执行且 `dataRaces=0`，但 `internal/pkg/cache` 包不存在导致 P7-V race coverage failed；P7-V final gate `failed=20`。策略：**P7-V Incomplete** · **Large Dataset Validation Partially Passed** · **Real Production Performance/Capacity/Peak Verification Deferred** · **Tag deferred** · **非 Production Ready**。
**Stage update**: 2026-07-13 — **Phase P7 性能、容量、分页、限流与证据门闸基础启动（Closure Verification Incomplete）**。新增 P7 配置与生产守卫（性能测试模式、数据集生成、分页、DB Pool、Worker、限流、缓存、导出、pprof）、DB 连接池环境变量接入、`pkg/pagination` 深 offset 保护与 cursor scope 基础、`pkg/ratelimit` 本地 token bucket 与 Gin 中间件、P7 性能/容量表模型与 PostgreSQL 候选索引迁移、受控 `p7load` / `scripts/p7-generate-dataset.mjs` 数据集计划入口、`scripts/p7-performance-capacity-check.mjs` 与 `scripts/p7-performance-regression-gate.mjs`。新增 P7 文档、Dashboard、Runbook 与 env/API/docs 索引。当前明确缺口：Medium 数据集未真实生成、Load Test 未执行、Soak Test 未执行、Linux Race 未执行、性能回归 baselines 未校准、Redis 分布式限流/Provider 限流/Worker 公平调度/管理端性能中心未完成。策略：**Production Capability Development In Progress** · **P7 Foundation In Progress** · **Phase P7 Closure Verification Incomplete** · **Real Production Performance/Capacity/Peak Verification Deferred** · **非 Production Ready** · **Tag deferred** · **Final Acceptance Deferred**。
**Stage update**: 2026-07-13 — **Phase P6-VR Linux Race 环境修复完成，Phase P6 Fully Closed（真实生产验证仍 Deferred）**。审计 `backend/go.mod` 确认仓库要求 `go 1.25.0` 且无 `toolchain` 指令；WSL2 Ubuntu 22.04.5 初始默认 `/usr/bin/go` 为 `go1.18.1`，P6-V Linux Race 为 `environment_blocked`。本轮安装官方 Go `go1.25.12` 到 `/usr/local/go`，校验 SHA-256 `234828b7a89e0e303d2556310ee549fbcf253d28de937bac3da13d6294262ac1`，配置 `/etc/profile.d/trademind-go-toolchain.sh`，确认 `GOOS=linux`、`GOARCH=amd64`、`CGO_ENABLED=1`、GCC 11.4、Node v22.22.1、pnpm 9.15.4。`scripts/p6-v-linux-race.mjs` 已补强 WSL2/Go/CGO/GCC/路径检测、真实退出码、stdout/stderr、timeout、环境阻塞与测试失败分类、Markdown/JSON 报告；`scripts/p6-v-final-closure-gate.mjs` 已补强 Race JSON 真字段校验。WSL2 内 `go mod verify`、`go test ./...`、`go build ./cmd/server/... ./cmd/p6drill`、9 个 P6 race 包与组合 race 矩阵全部通过，`dataRaces=0`、`deadlocks=0`；`pnpm check:p6` 为 `failed=0`，`scripts/p6-v-final-closure-gate.mjs` 为 `failed=0`，P1–P6-VR gates 失败数为 0，`git diff --check` 退出码 0（仅 LF/CRLF warning）。策略：**Phase P6 Fully Closed** · **Linux Race Verification Passed** · **Development Acceptance Passed** · **Isolated Restore Drill Passed** · **Restore Integrity Verification Passed** · **Isolated Release Rollback Drill Passed** · **Application Rollback Ready** · **Real Production Backup/Restore/PITR/Release/Telemetry/Douyin Credential Verification Deferred** · **Tag deferred** · **非 Production Ready** · **Final Acceptance Deferred**。
**Stage update**: 2026-07-13 — **Phase P6-V 隔离恢复与发布回滚真实演练通过，Linux Race 仍阻塞（Closure Verification Incomplete）**。新增 `backend/cmd/p6drill` 演练 CLI、`scripts/p6-v-isolated-restore-drill.mjs`、`scripts/p6-v-linux-race.mjs`、`scripts/p6-v-final-closure-gate.mjs` 等入口；恢复演练使用本机 PostgreSQL 17 `initdb/pg_ctl` 创建临时隔离集群和 run-id 数据库，真实执行 AutoMigrate、Demo/P6-V seed、`backup.Service`、AES-GCM 加密、SHA-256、Manifest 校验、`pg_restore --list`、`restore.Service` 到全新空库、恢复后摘要与 audit chain 校验；6 个负向安全拒绝项（未验证备份、checksum、manifest、密文、production target、非空目标）均通过。发布回滚演练在同一隔离集群内通过 `release.Service` 执行 preflight、发布前备份、状态机、应用回滚，确认不自动恢复数据库、不执行破坏性 down migration。演练发现并修复：全新空库迁移中 Douyin 10.2 索引检查顺序问题；后台审计日志 hash chain 未写链与 timestamp 精度导致恢复后 mismatch。当前 `scripts/p6-v-final-closure-gate.mjs` 为 **failed=1 / passed=14**，唯一阻塞：当前 WSL2 Go 为 **1.18.1**，不兼容仓库 `go 1.25.0`，Linux race 报告为 `environment_blocked`。策略：**Isolated Restore Drill Passed** · **Restore Integrity Verification Passed** · **Isolated Release Rollback Drill Passed** · **Application Rollback Ready** · **Linux Race Verification Pending** · **Phase P6 Closure Verification Incomplete** · **Real Production Backup/Restore/PITR/Release Verification Deferred** · **非 Production Ready** · **Tag deferred**。
**Stage update**: 2026-07-13 — **Phase P6 备份、恢复、发布、回滚与灾备代码级基础完成（Closure Verification Incomplete）**。新增 `backup` / `restore` / `release` / `disasterrecovery` 后端模块，`pkg/backupruntime` AES-GCM 分块加密、SHA-256、`pg_dump` / `pg_restore` 安全 argv 构造、PITR 目标时间与 WAL 连续性检查，`pkg/artifact` Release Manifest；新增 P6 配置与生产保护、AutoMigrate 表、P6 RBAC 权限、`/api/v1/ops/backups|restores|releases|dr` API、管理端 `/ops/backups|restores|releases|disaster-recovery` 页面、P6 指标/告警/Dashboard/Runbook/文档与 `scripts/p6-backup-release-dr-check.mjs`。当前已验证：P6 后端核心包测试、`go test ./...` 三连、`go build ./cmd/server/...`、P6 高风险包 `-count=10`、`pnpm check:dev`、`pnpm check:ui-copy --strict`、`pnpm build:admin`、`pnpm build:collector`、`go mod tidy`、`go mod verify`、`git diff --check`、P1-P6 静态 gates 与两轮 `pnpm demo:auto-acceptance` 均通过或按外部环境阻塞分类收敛；两轮 demo 均为 `passed_with_blocked`，`failed=0`、`codeFailed=0`、`nonAiFailed=0`。当前明确缺口：隔离 PostgreSQL 恢复演练未执行、隔离发布回滚演练未执行、Linux Race 未执行。策略：**Backup Foundation Ready** · **Encrypted Backup Foundation Ready** · **Restore Safety Gate Ready** · **PITR Foundation Ready** · **Application Rollback Boundary Ready** · **Disaster Recovery Foundation Ready** · **Phase P6 Closure Verification Incomplete** · **Real Production Backup/Restore/PITR/Release Verification Deferred** · **非 Production Ready** · **Tag deferred**。

**Stage update**: 2026-07-13 — **Phase P5-V 标准 OTLP/HTTP JSON 代码级闭环通过**。`backend/internal/pkg/tracing` 已将 P5.2 custom HTTP span JSON 替换为标准 OTLP/HTTP JSON TraceService Export 结构（`resourceSpans -> resource -> scopeSpans -> scope -> spans`），保留 `/v1/traces`、`application/json`、父子 Span、错误状态、typed attributes、敏感字段过滤、受控 retry、队列/批大小/timeout 上限与 shutdown flush；新增 `valid_otlp_trace.json` golden fixture、Mock Collector 严格解析测试与 `scripts/p5-v-final-observability-gate.mjs`。当前已验证：`go test ./...`、`go build ./cmd/server/...`、`go test ./internal/pkg/tracing/... ./internal/config/... ./internal/modules/observabilitymod/...`、高风险包 `-count=10`、WSL2 Linux `go test -race` 观测矩阵、`pnpm check:dev`、`pnpm check:ui-copy --strict`、`pnpm build:admin`、`pnpm build:collector`、两轮 `pnpm demo:auto-acceptance`（均 `failed=0`、`codeFailed=0`、`nonAiFailed=0`）与 P1-P5-V 静态扫描均已通过或按外部 provider 阻塞/告警分类收敛。策略：**P5-V Code-Level Observability Gate Passed** · **Standard OTLP/HTTP Export Ready** · **Mock Collector Protocol Verification Passed** · **Real Environment Telemetry Verification Deferred** · **External Alert Channels Deferred** · **Production SLO Validation Deferred** · **非 Production Ready** · **Tag deferred** · **Final Production Acceptance Deferred**。

**Stage update**: 2026-07-12 — **Phase P5.2 九类真实业务埋点接线完成（Closure Verification Incomplete）**。HTTP Client、Webhook、Order Sync、Inventory、AI Text、AI Image、File Scan、Security、Auth 九类业务路径已接入共享 `metrics.Catalog`，新增对应 `observability.go` / `observability_test.go`、`scripts/p5-2-business-instrumentation-check.mjs`、`scripts/p5-2-business-instrumentation-smoke.mjs` 与 P5.2 文档族。`scripts/p5-1-observability-closure-check.mjs`、`scripts/p5-2-business-instrumentation-check.mjs`、P5.2 smoke、后端 `go test ./...` 与 `go build ./cmd/server/...` 已通过。当前明确缺口：span exporter 仍为 custom HTTP JSON exporter 而非标准 OTLP/HTTP、Linux Race 未在 Linux/WSL2 执行、demo:auto-acceptance 未执行、真实 telemetry backend / 外部 alert channel / 生产 SLO 验证 Deferred。策略：**P5.2 Code-Level Instrumentation Passed** · **Phase P5 Closure Verification Incomplete** · **Real Environment Telemetry Verification Deferred** · **非 Production Ready** · **Tag deferred** · **Final Acceptance Deferred**。

**Stage update**: 2026-07-12 — **Phase P5.1 可观测性执行闭环增强进行中（Closure Verification Incomplete）**。新增 DB runtime collector 与 `InstrumentedDB` 查询/事务封装、轻量 OTLP HTTP span exporter（规避 genproto ambiguous import，Mock Collector 单测通过）、telemetry export success/failure/drop 指标、Alert Evaluator / Delivery Worker（`alert_evaluation_runs`、`alert_deliveries`）、SLO Evaluator（Snapshot / Error Budget / Burn Rate / `insufficient_data`）、`alerts-and-slo` Dashboard、`scripts/p5-1-observability-closure-check.mjs` 与 P5.1 文档族。聚焦单测通过：`pkg/metrics`、`pkg/tracing`、`pkg/observability`、`modules/alerting`、`modules/observabilitymod`。当前明确缺口：HTTP Client、Webhook、Order Sync、Inventory、AI Text、AI Image、File Scan、Security、Auth 等业务模块真实埋点尚未全量接线；Linux Race、完整 `go test ./...`、Admin/Collector build、demo:auto-acceptance 与真实 Telemetry / 外部 Alert Channel / 生产 SLO 验证仍未完成。策略：**P5.1 Incomplete** · **Phase P5 Closure Verification Incomplete** · **Database Observability Code Ready** · **OTLP Mock Export Code Ready** · **Alert Evaluation and Internal Delivery Code Ready** · **SLO Evaluation Code Ready** · **Business Instrumentation Incomplete** · **Real Environment Telemetry Verification Deferred** · **非 Production Ready** · **Tag deferred** · **Final Acceptance Deferred**。

**Stage update**: 2026-07-12 — **Phase P5 可观测性生产化代码完成**。统一结构化日志（`pkg/logging`）、Prometheus 指标（`pkg/metrics`）、OpenTelemetry 追踪（`pkg/tracing`）、可观测性门面（`pkg/observability`）、HTTP/Provider/Task/Webhook/AI/Security 指标目录、告警事件模型与默认规则（`modules/alerting`）、`/internal/metrics` 内部保护、运维可观测性中心 UI（`/ops/observability`）、SLO 基础表、`migrate_p5`、15 份 Runbook、Dashboard JSON、`scripts/p5-observability-check.mjs` 与 P5 文档族。OTLP HTTP Exporter 因 genproto 依赖冲突标记 **telemetry_export_environment_blocked**；真实监控平台验证 **Deferred**。策略：**Observability Foundation Ready** · **Structured Logging Ready** · **Metrics Collection Ready** · **Distributed Tracing Ready** · **Alerting Foundation Ready** · **SLI and SLO Foundation Ready** · **Operations Dashboard Ready** · **Real Environment Telemetry Verification Deferred** · **Production Capability Development In Progress** · **非 Production Ready** · **Tag deferred** · **Final Acceptance Deferred**。

**Stage update**: 2026-07-12 — **Phase P4-R Demo 回归稳定性收口完成（自动化 Run 2 / Run 3 均 passed_with_blocked）**。新增 `scripts/lib/clean-test-env.mjs` 与 `scripts/go-test-isolated.mjs`，`demo:auto-acceptance` 的 `go test` 改为隔离白名单环境并输出 `artifacts/demo-acceptance/go-test.log`；Demo Data / Permission seed 增加 `p4-r-v1` 版本、生产禁跑、结构化 JSON 退出语义与验证脚本，重复 seed 已验证不重复创建核心 Demo 商品 / 用户；AI 图片 trial 增加超时分类、`lastCompletedStage` 与安全证据字段，Run 2 / Run 3 均将图片超时归类为 warning、AI 文案真实 Provider 归类为 external provider blocked；新增 `scripts/p4-r-demo-regression-closure-check.mjs` 和 P4-R 审计/根因/版本化/分类文档。策略：**Phase P4 Closure Verification Incomplete** · **Run 2 passed_with_blocked** · **Run 3 passed_with_blocked** · **Real Environment Security Verification Deferred** · **Real Credential Verification Deferred** · **非 Production Ready** · **Tag deferred**。

**Stage update**: 2026-07-12 — **Phase P4-V 安全关闭验证门完成（Phase P4 Fully Closed）**。Secret Rotation 聚合 `AllReencryptTargets`（`settings_encrypted` + `shop_auth_tokens`）接入 `CountSecretReferencesByKeyID` / `VerifyRotation` / `ProcessReencryptBatch`；legacy 密文纳入重加密路径；租户 SQL scope 补强 inventory / ordersync / productpublish / customerchat / taskcenter；IDOR **55** 例、Shop Scope **21** 例回归通过；WSL2 `go test -race` 全矩阵 **0 data races**；`scripts/p4-v-security-closure-gate.mjs` failed=0；7 份 `P4_V_*` 文档。策略：**Phase P4 Fully Closed** · **Tenant Isolation Enforced** · **Secret Key Rotation Ready** · **Linux Race Verification Passed** · **Real Environment Security Verification Deferred** · **Real Credential Verification Deferred** · **Production Capability Development In Progress** · **非 Production Ready** · **Tag deferred** · **Final Acceptance Deferred**。

**Stage update**: 2026-07-11 — **Phase P4.2 全量租户 Worker 与安全 Worker 收口文档完成**。`tasktenant` 包（`BeginWorker` / `RequireTaskTenant` / `ResolveShopTenant`）接入 collect / order_sync / customer_message_sync / product_publish / inventory_sync / webhook / file_security_scan；新增 `security_secret_reencrypt` 与 `file_security_scan` Worker（`worker.TypeSecuritySecretReencrypt` / `TypeFileSecurityScan`）；`secret_targets.go` 覆盖 `settings_encrypted` + `shop_auth_tokens`；`migrate_p4_2.go` 补全 sync/batch/export/collect/AI 批次 `tenant_id` 与索引；`tenantquery` + `taskcenter` 租户 scope；安全中心 UI（运行概览/认证与会话/租户隔离/密钥轮换/文件安全/审计完整性）；IDOR 自动化 **22** 例、店铺 scope **5** 例（closure 目标 40+/20+ 待扩充）；race **deferred_on_windows**；11 份 `P4_2_*` 文档 + `scripts/p4-2-security-final-closure-check.mjs`。策略：**Tenant Worker Context Implemented** · **Security Workers Registered** · **IDOR/Shop Scope Automation Partial** · **Linux Race Verification Deferred** · **Real Environment Security Verification Deferred** · **非 Production Ready** · **Tag deferred** · **Final Acceptance Deferred**。

**Stage update**: 2026-07-11 — **Phase P4 安全、权限、租户隔离与敏感数据生产化代码完成**。实现 Refresh Token 哈希存储与轮换、Token Family 重用检测、Session 撤销、登录限流与账户锁定、JWT `kid` 与过渡密钥、TenantContext、AuthorizationService、PII 脱敏、enc:v2 密钥环、审计 Hash Chain、上传安全校验、安全响应头/CSRF、configstatus P4 分组、`scripts/p4-security-check.mjs` 与 16 份 P4 文档。策略：**Security Foundation Implemented** · **Tenant Isolation Enforced** · **Sensitive Data Protection Implemented** · **Real Environment Security Verification Deferred** · **Production Capability Development In Progress** · **非 Production Ready** · **Tag deferred** · **Final Acceptance Deferred**。

**Stage update**: 2026-07-11 — **Phase P3.1 抖店收口代码完成（Phase P3 Fully Closed）**。订单 Webhook `OrderEventHandler` 注入并接入 `UpsertPlatformOrder`；Webhook 与轮询共用统一 Upsert；`platform_revision` / `platform_updated_at` 乱序保护；`ContractCapabilityGate`（IM/品牌/签名 v1）；AI 文案/图片 apply reconciliation；Token `token_version` DB 持久化；`migrate_p31_douyin`；configstatus P3.1 项；`scripts/p3-1-douyin-closure-check.mjs` 通过。策略：**Phase P3 Fully Closed** · **Production Capability Development In Progress** · **Douyin Production Adapter Implemented** · **Contract-Gated Capabilities Explicitly Isolated** · **Real Credential Verification Deferred** · **非 Production Ready** · **Tag deferred** · **抖店 Release Candidate** · **Final Acceptance Deferred**。WSL2 `-race` 待 Linux CI 执行（Windows 原生不支持 race detector）。

**Stage update**: 2026-07-11 — **Phase P3 抖店 Production Adapter 代码实现完成**。实现抖店 Provider 增强（facade/http_transport/errors/token_lock/order_detail/inventory_query/customer/webhook_sign/webhook_events/health/brand）、Webhook 模块（douyin_verifier/douyin_handler）、OAuth state DB 模型（DouyinOAuthState）、图片幂等缓存（DouyinImageAsset）、订单同步游标（DouyinSyncCursor）、P2-DEBT-001 AI apply 幂等、configstatus P3 指标、数据库迁移（migrate_p3_douyin）、单元测试全部通过、13 份架构文档。策略：**抖店 Production Adapter Implemented** · **Real Credential Verification Deferred** · **只创建平台草稿 / 不自动上架** · **代码实现完成不等于真实 E2E** · **非 Production Ready**。

**Stage update**: 2026-07-11 — **Phase P2 Fully Closed（生产能力可靠性收口）**。AI apply/undo 幂等、Webhook HTTP 接收地基、六大生产 Worker 统一 tasklease、WSL2 `-race` 通过（无 data race）、P2.1 三 warning 清零、P2.2 静态扫描通过。策略：**Phase P2 Fully Closed** · **Production Capability Development In Progress** · **AI Result Application Idempotency Ready** · **Webhook Receiver Foundation Ready** · **All Production Workers Lease-Protected** · **Infrastructure Foundation Ready** · **MVP Demo Ready** · **Tag deferred** · **非 Production Ready** · **抖店 Release Candidate** · **Final Acceptance Deferred**（AI Provider Key 可能仍为 environment_blocked）。

**Stage update**: 2026-07-11 — **Phase P2.2 AI 文案/图片 apply+undo 幂等完成**。`keys.go` 扩展 `ai-text-apply/undo`、`ai-image-apply/undo`；`aiproducttext`/`aiproductimage` 接入 `idempotency.Service`；目标版本冲突码；生成 Worker `WHERE status=running` 写回守卫；`set_main` undo 恢复 `previousBestMainId`；并发单测通过。策略：**Production Capability Development In Progress** · **非 Production Ready**。

**Stage update**: 2026-07-11 — **Phase P2.2 Webhook HTTP 接收地基完成**。公开 `POST /api/v1/webhooks/:platform/:eventType`（无 JWT）；签名/时间戳/体限制；幂等持久化后快速 ACK；DB 轮询异步 noop 处理；`WEBHOOK_*` 配置。策略：**Production Capability Development In Progress** · **非 Production Ready**。

**Stage update**: 2026-07-11 — **Phase P2.2 tasklease 扩展至采集 / 图片 / 客服同步 Worker**。`TryClaimPendingOrRetrying`（pending 或 retrying+`next_retry_at IS NULL`）；collect / imagetask / customersync 接入 `execution_id`+心跳续约+`finish*Task` 守卫写回；productpublish 终态写回加固；`migrate_p2_2` 索引；taskcenter `applyLeaseMeta` 覆盖上述类型；stale worker 单测。策略：**Phase P2.2 Completed** · **Core Reliability Foundation Ready** · **非 Production Ready** · **Final Acceptance Deferred**。

**Stage update**: 2026-07-10 — **Phase P2.1 领域幂等与任务心跳租约完成**。统一 `idempotency.Service` 接入订单同步/导入、库存扣减/推送、刊登、客服外发、AI 文案/图片批次、Webhook；`tasklease` 包（`heartbeat_at` / `execution_id` / `lock_version`）接入订单同步、库存同步、刊登 Worker；静态扫描 `scripts/p2-1-domain-idempotency-check.mjs`；文档 `P2_1_*`、`DOMAIN_IDEMPOTENCY_INTEGRATION`、`TASK_LEASE_AND_HEARTBEAT_DESIGN`、`STALE_WORKER_PROTECTION`、`CONCURRENT_WRITE_SAFETY`。策略：**Phase P2.1 Completed** · **Core Reliability Foundation Ready** · **非 Production Ready** · **Final Acceptance Deferred**（AI Key / 抖店 E2E 等仍可能阻塞）。

**Stage update**: 2026-07-10 — **Phase P2 核心可靠性基础完成；P1 已收口**。统一幂等（`idempotency_records`）、任务租约/心跳/重试/死信、订单 upsert 唯一键、库存台账与 `business_event_key`、客服 `clientMessageId`、刊登批次/子任务幂等、Provider `httpclient`+熔断+429、PostgreSQL 迁移 advisory lock、staging/production CORS fail-fast、`STORAGE_PROVIDER` 生产门禁。设计文档：`IDEMPOTENCY_DESIGN.md`、`TASK_RELIABILITY_DESIGN.md`、`ORDER_SYNC_RELIABILITY.md`、`INVENTORY_CONSISTENCY_DESIGN.md`、`CUSTOMER_MESSAGE_IDEMPOTENCY.md`、`PUBLISH_IDEMPOTENCY_DESIGN.md`、`PROVIDER_RESILIENCE_DESIGN.md`、`CIRCUIT_BREAKER_AND_RATE_LIMIT.md`、`MULTI_INSTANCE_SAFETY.md`、`CORS_PRODUCTION_GUIDE.md`、`MIGRATION_LOCK_DESIGN.md`。策略：**Phase P2 Completed** · **Core Reliability Foundation Ready** · **P1 Closed** · **Production Capability Development In Progress** · **Infrastructure Foundation Ready** · **MVP Demo Ready** · **非 Production Ready** · **Tag deferred** · **抖店 Release Candidate**。

**Stage update**: 2026-07-10 — **Phase P1 生产配置、Storage 与环境基础设施完成**。多环境配置体系（`APP_ENV` profiles + `.env.*.example`）；production fail-fast（JWT/MasterKey/危险功能/API 公网 URL）；`/health/live` + `/health/ready`；Storage `ValidatePublicBase` + 公网测试别名路由；配置状态中心增强（环境/生产安全/Storage 边界）；`deploy/` Nginx/systemd/脚本；P1 扫描 `scripts/p1-production-config-check.mjs`。策略：**Production Capability Development In Progress** · **Infrastructure Foundation Ready** · **MVP Demo Ready** · **Tag deferred** · **非 Production Ready** · **抖店 Release Candidate** · **Final Acceptance Deferred**（P10）；F9 为历史 Demo 基线；不允许灰度。

**Stage update**: 2026-07-10 — **Phase H1.5.1 Post-F9 Enhancement 真实浏览器签收 + AI 图片基线确认完成**。Chrome 13/13 核心 URL 后退/前进/刷新签收 **passed**；Edge 7 项抽查 **passed**；1366×768（11 张）/ 1024×768（8 张）真实 PNG 归档；修复 ProTable 首请求清空 URL（订单/异常/失败任务/草稿/客服）与 AI 工作台 compare-before-write；AI 图片基线 **stable_range_14_to_15_of_16**（本轮 14/16 `passed_with_warning`）。报告 [`H1_5_LIVE_BROWSER_ACCEPTANCE.md`](H1_5_LIVE_BROWSER_ACCEPTANCE.md)、[`H1_5_AI_IMAGE_BASELINE_CONFIRMATION.md`](H1_5_AI_IMAGE_BASELINE_CONFIRMATION.md)。策略不变：**Tag deferred**、**Post-F9 Enhancement**、**MVP Demo Ready**、**非 Production Ready**、**抖店 Release Candidate**。

**Stage update**: 2026-07-10 — **Phase H1.5 Post-F9 Enhancement 次级列表 URL 状态 + 浏览器签收完成**。接入刊登批次/任务、采集任务、订单同步、客服消息同步、AI 文案/图片批次列表与详情 URL 状态；扩展 `source` allowlist；Chrome 后退/前进/刷新/深链签收 **passed_with_warning**；1366/1024 响应式 **passed** / **passed_with_warning**；报告 [`H1_5_SECONDARY_URL_BROWSER_CHECK.md`](H1_5_SECONDARY_URL_BROWSER_CHECK.md) / [`h1-5-secondary-url-browser-check.json`](h1-5-secondary-url-browser-check.json)。策略不变：**Tag deferred**、**Post-F9 Enhancement**、**MVP Demo Ready**、**非 Production Ready**、**抖店 Release Candidate**；不进入真实预发、抖店真实 E2E、生产灰度。

**Stage update**: 2026-07-09 — **Phase H1.4 Post-F9 Enhancement 订单/异常 URL 状态补漏 + keyword UX 安全增强完成**。订单列表 URL 保持 `status` / `fulfillmentStatus` / 创建时间范围（`start`/`end`）；订单异常 URL 保持 `severity` / 创建时间范围；keyword 最大 80 字符、敏感信息轻提示、清空同步 URL；浏览器后退/前进与 1366/1024 点检 **passed_with_warning**；报告 [`H1_4_URL_KEYWORD_RESPONSIVE_CHECK.md`](H1_4_URL_KEYWORD_RESPONSIVE_CHECK.md) / [`h1-4-url-keyword-responsive-check.json`](h1-4-url-keyword-responsive-check.json)。策略不变：**Tag deferred**、**Post-F9 Enhancement**、**MVP Demo Ready**、**非 Production Ready**、**抖店 Release Candidate**；不进入真实预发、抖店真实 E2E、生产灰度。

**Stage update**: 2026-07-09 — **Phase H1.3 Post-F9 Enhancement AI 图片 warning 收敛 + 抖店 E2E 前置提示完成**。结构化 AI 图片 warning 码（可解释/可定位/可恢复）；批次详情 overview；失败任务 `ai_image_*` 分类；配置状态中心增强（通义万相 Key、Storage `public_base`）；抖店/Storage 前置 Banner；文档 `AI_IMAGE_WARNING_RECOVERY_GUIDE.md` / `DOUYIN_E2E_PRECHECK_GUIDE.md` / `STORAGE_PUBLIC_URL_GUIDE.md`。策略不变：**Tag deferred**、**Post-F9 Enhancement**、**MVP Demo Ready**、**非 Production Ready**、**抖店 Release Candidate**；不进入真实预发、抖店真实 E2E、生产灰度。

**Stage update**: 2026-07-09 — **Phase H1.2.1 Post-F9 Enhancement URL 状态浏览器点检完成**。对 H1.1 + H1.2 已接入页面做浏览器/API 点检；修复 Dashboard `productSource` 与导航 `source` 冲突、Dashboard 挂载丢参、三页 reset Drawer 未关、商品草稿 source 筛选未写 URL 等 P0/P1 项；输出 [`H1_2_URL_STATE_BROWSER_CHECK.md`](H1_2_URL_STATE_BROWSER_CHECK.md) / [`h1-2-url-state-browser-check.json`](h1-2-url-state-browser-check.json)。策略不变：**Tag deferred**、**Post-F9 Enhancement**、**MVP Demo Ready**、**非 Production Ready**、**抖店 Release Candidate**；不进入真实预发、抖店真实 E2E、生产灰度。

**Stage update**: 2026-07-09 — **Phase H1.2 Post-F9 Enhancement 第二批页面 URL 状态保持完成**。在 H1.1 基础上扩展 `urlState` allowlist，接入 `/orders/list`、`/orders/exceptions`、`/product/drafts`、`/inventory`、`/inventory/alerts`、`/inventory/sync-tasks`、`/customer/hub`、`/customer/conversations` 筛选 / 分页 / Drawer / 深链恢复；Dashboard `source=dashboard` 与失败任务中心 `source=taskcenter` 出站链接保留；兼容 `jumpOrder`、`orderId`、`productSkuId`、`skuId`、`batchId`、`suggestionId` 等旧深链。策略不变：**Tag deferred**、**Post-F9 Enhancement**、**MVP Demo Ready**、**非 Production Ready**、**抖店 Release Candidate**；不进入真实预发、抖店真实 E2E、生产灰度。设计见 [`WORKBENCH_URL_STATE_DESIGN.md`](WORKBENCH_URL_STATE_DESIGN.md)。

**Stage update**: 2026-07-07 — **Phase H1.1 Post-F9 Enhancement 启动并完成首批工作台 URL 状态保持**。

**Stage update**: 2026-07-07 — **Phase F9 最终总体验收完成**。复跑 `pnpm demo:auto-acceptance`（**passed**，AI 图片 14/16 warning）；F9.1 16 步主链路浏览器/API 走查 **passed_with_warning**；RBAC **passed**；1366/1024 响应式 **passed_with_manual_required**；F9.2 预发 **blocked_by_environment**；F9.3 Storage/AI **passed_with_warning**；F9.4 抖店 E2E **blocked_by_real_credentials**；F9.5 回滚文档就绪、灰度 **未允许**。P0/P1 **0**。总报告 [`F9_FINAL_ACCEPTANCE_REPORT.md`](F9_FINAL_ACCEPTANCE_REPORT.md) / [`f9-final-acceptance.json`](f9-final-acceptance.json)。**状态**：**Phase F9 Passed** · **Tag deferred** · **MVP Demo Ready** · **非 Production Ready** · **抖店 Release Candidate** · **不允许灰度**。

**Stage update**: 2026-06-30 — **Phase F8.1 冻结基线复跑与 F9 准入确认完成**。重启 backend（`backend/tmp/server.exe`）；`/health` database=ok、redis=ok、workers running；dev-only `POST /api/v1/dev/demo-seed/full-project-edge-cases` **200**（4 样本）；复跑 Demo/权限 seed；`pnpm demo:auto-acceptance`（Phase **F8.1-Auto**）**passed**（0 failed）。修复 smoke 路径、seed 脚本、readonly `POST /products` 403、phase102 重复 Demo 订单清理。报告 [`DEMO_AUTO_ACCEPTANCE_FULL_PROJECT_REPORT.md`](DEMO_AUTO_ACCEPTANCE_FULL_PROJECT_REPORT.md)。P0/P1 **0 open**。F9 准入：**`f8_1_passed_with_warning_ready_for_f9`**（抖店凭证 / Storage 公网 `environment_required`）。**状态**：**Phase F8.1 Passed with Warning** · **Function Freeze Confirmed** · **Ready for Phase F9** · **MVP Demo Ready** · **非 Production Ready** · **Tag deferred**。

**Stage update**: 2026-06-30 — **Phase F8 功能冻结与 P0/P1 清零完成**。P0/P1 代码层面 **0 open**（P0-02/P0-03 降级为 F9 `environment_required`）；新增 dev-only `POST /api/v1/dev/demo-seed/full-project-edge-cases`；商品详情 `putProductPlatformPublishConfig` 接入 `confirmPlatformPublishConfigSave`；采集流程目标店铺提示（Hub/Tasks/空状态/成功提示）；文档：`FUNCTION_FREEZE_P0_P1_AUDIT.md`、`FUNCTION_FREEZE_RULES.md`、`F9_FINAL_ACCEPTANCE_PRECHECK.md`、`POST_FREEZE_BACKLOG.md`；`docs/api.md` 同步 `/shops` 与 dev seed；`demo-auto-acceptance` 升级 **Phase F8-Auto**。`go test ./...`、`go build ./cmd/server/...`、`pnpm build:admin`、静态扫描通过。**状态**：**Phase F8 Completed** · **Function Freeze Ready** · **MVP Demo Ready** · **非 Production Ready** · **抖店 Release Candidate** · **Tag deferred**。**未**打 tag、**未**真实抖店 E2E、**未**最终人工总体验收（留 **Phase F9**）。

**Stage update**: 2026-06-30 — **Phase F7 全项目 Demo 数据升级完成**。**Phase F2–F7 已全部交付**；**当前阶段 Phase F8（功能冻结）**；**F9 最终总体验收待启动**。扩展 `scripts/seed-demo-data.ps1`：20 商品 slot + 订单 / 库存 / 客服 / Dashboard KPI 聚合样本；`scripts/seed-demo-permissions.ps1`（demo_admin / demo_operator / demo_readonly）；输出 `docs/demo-dataset.*.json` 与 `docs/demo-dataset.full-project.json`；新增 smoke：`demo-dashboard-smoke`、`demo-rbac-smoke`、`demo-order-inventory-customer-smoke`、`demo-empty-state-scan`、`demo-sensitive-confirm-scan`；`demo-auto-acceptance.ps1` 升级为 **Phase F7-Auto**；指南 [`DEMO_SEEDING_GUIDE.md`](DEMO_SEEDING_GUIDE.md)、[`DEMO_AUTO_ACCEPTANCE_GUIDE.md`](DEMO_AUTO_ACCEPTANCE_GUIDE.md)。F7 全局状态文案复扫 **passed**（`check-ui-copy --strict` + `global-status-copywriting-scan.json`）。`go test ./...`、`go build ./cmd/server/...`、`pnpm build:admin` 通过。**状态**：**Full Project Functionality In Progress** · **MVP Demo Ready** · **非 Production Ready** · **抖店 Release Candidate** · **Tag deferred**。**未**打 tag、**未**真实抖店 E2E、**未**最终人工总体验收（留 **Phase F9**）。

**Stage update**: 2026-06-30 — **Phase F6 总 Dashboard 与全局体验 / 权限收口完成**。运营总览 `/dashboard/product-operations` 扩展为全链路入口（10 KPI 卡片 + 统一待办）；新增 API `GET /dashboard/overview|todos|health`；Dashboard 聚合应用 RBAC 店铺 scope；菜单级权限 `menuAccess.ts` + `menuDataRender`；商品 / 刊登 `adminperm` 店铺 scope（`ApplyProductScope` / `ApplyStoreScope`）；失败任务批量重试等接入 `confirmSensitiveAction`；全局状态文案收口 `COMMON_STATUS_LABEL`；设计文档 `DASHBOARD_*` / `GLOBAL_*` / `FULL_PROJECT_DEMO_DATASET.md`。`go test ./...`、`go build ./cmd/server/...`、`pnpm build:admin` 通过。**仍为 MVP Demo Ready / 非 Production Ready**；**抖店 Release Candidate**；**Tag deferred**。未进入最终人工测试、真实预发、抖店 E2E、生产灰度。

**Stage update**: 2026-06-29 — **Phase F5 权限、RBAC、配置状态中心与审计完善完成**。完整 RBAC（admin/operator/readonly）+ `user_store_permissions`；`adminperm` 权限矩阵与店铺隔离（订单/客服/失败任务/操作日志）；用户管理 `/settings/users` + API `/api/v1/admin/users`；配置状态中心 `/settings/config-status`；Profile 扩展 permissions/storePermissions；设置写操作与配置中心 admin 保护；审计日志扩展 shopId/adminRole；前端 `permission.ts` / `usePermission` / `PermissionGuard` / `sensitiveConfirm`；Demo 权限种子 `scripts/seed-demo-permissions.ps1` + `docs/demo-dataset.permissions.json`；设计文档 `RBAC_*` / `CONFIG_STATUS_*` / `OPERATION_AUDIT_*`。`go test ./internal/pkg/adminperm/...`、`go build ./cmd/server/...`、`pnpm build:admin` 通过。**仍为 MVP Demo Ready / 非 Production Ready**；**抖店 Release Candidate**；**Tag deferred**。未进入最终人工测试、真实预发、抖店 E2E、生产灰度。

**Stage update**: 2026-06-29 — **Phase F4 客服中心与 AI 回复建议完善完成**。

**Stage update**: 2026-06-29 — **Phase F3 库存中心与库存同步完善完成**。新增库存中心 `/inventory` + `GET /api/v1/inventory`；库存扣减记录 `/inventory/deductions`；统一 SKU 未绑定/歧义阻断文案；`inventory_sync_enabled` 引导横幅；订单详情库存 Tab 深链；失败任务/异常工作台库存联动；只读 RBAC（`CanWriteInventory`）；Demo 种子 Phase F3（`docs/demo-dataset.inventory.json`）；设计文档 `INVENTORY_*`。`go test ./...`、`go build ./cmd/server/...`、`pnpm build:admin` 通过。**仍为 MVP Demo Ready / 非 Production Ready**；**抖店 Release Candidate**；**Tag deferred**。未进入最终人工测试、真实预发、抖店 E2E、生产灰度。下一阶段建议：**Phase F4 客服中心完善**。

**Stage update**: 2026-06-29 — **Phase F2 订单中心与订单异常工作台完善完成**。

**Stage update**: 2026-06-29 — **Phase F1 全项目功能缺口审计与后续开发路线规划完成**。新增 [`FULL_PROJECT_FUNCTION_MAP.md`](FULL_PROJECT_FUNCTION_MAP.md)（34 模块完成度）、[`FULL_PROJECT_MVP_MAIN_FLOW.md`](FULL_PROJECT_MVP_MAIN_FLOW.md)（16 步主链路）、[`FULL_PROJECT_DEVELOPMENT_PLAN.md`](FULL_PROJECT_DEVELOPMENT_PLAN.md)（F2–F9 阶段）、[`FULL_PROJECT_MVP_GAP_AUDIT.md`](FULL_PROJECT_MVP_GAP_AUDIT.md)（P0–P3 缺口）。结论：**Full Project Development Planning Completed**；状态仍为 **`MVP Demo Ready`**（**非 Production Ready**）。**Tag deferred**。**抖店仍 Release Candidate**。`go test ./...`、`go build ./cmd/server/...`、`pnpm build:admin` 通过。**未**进入最终人工测试、真实预发、抖店真实 E2E、生产灰度。下一阶段建议：**Phase F2 订单中心与订单异常工作台完善**。

**Stage update**: 2026-06-27 — **Phase R1.3-Codex 模拟人工测试与体验问题扫描完成**。结论 **`codex_simulation_passed_with_warning`**（仍为 **`MVP Demo Ready`**，非 Production Ready）。新增 [`CODEX_MANUAL_SIMULATION_REPORT.md`](CODEX_MANUAL_SIMULATION_REPORT.md) 与 [`codex-manual-simulation.json`](codex-manual-simulation.json)。修复 P1/低风险体验问题：运营看板快捷入口不再指向旧版 `/ai/batches`；旧版 `/ai/batches?id=` 可定位批次详情；商品草稿页旧版批量 AI 文案标识为旧版兼容；`TechnicalDetails` 支持 `style` 并保持默认折叠。未打 `v0.1.0-demo` tag；未进入抖店真实 E2E / 生产灰度。多分辨率真实截图、浏览器后退/刷新状态仍需人工复核。

**Stage update**: 2026-06-27 — **Phase R1.2-Auto 可自动化验收项补齐完成**。新增总控脚本 `scripts/demo-auto-acceptance.ps1` / `.sh`，串联 go test、build、git diff --check、路由 smoke、Demo 数据校验、AI 文案/图片试跑、批量刊登、工作台 perf、中文文案扫描、权限安全扫描、文档一致性检查；输出 [`docs/DEMO_AUTO_ACCEPTANCE_REPORT.md`](DEMO_AUTO_ACCEPTANCE_REPORT.md) 与 [`docs/demo-auto-acceptance.json`](demo-auto-acceptance.json) 及 `*.auto.json` / `*.auto.md` 分项报告。自动化结论 **passed**（AI 图片 14/16 `passed_with_warning`）。状态仍为 **`MVP Demo Ready`**（**非 Production Ready**）。**Tag deferred**。**抖店仍 Release Candidate**；未打 tag、未进入真实 E2E / 生产灰度。

**Stage update**: 2026-06-27 — **Phase R1.2 真实预发部署与 Demo Tag 确认（部分完成）**。状态仍为 **`MVP Demo Ready`**（**非 Production Ready**）。本轮复跑：`go test ./...`、`go build -o tmp/server.exe`、`pnpm build:admin`、抖店/刊登/工作台回归通过；`/health` database+redis ok，9 worker running；路由 smoke → `docs/demo-route-smoke.preprod.json` passed=true；Demo 数据 → `docs/demo-dataset.preprod.json` 20 slot + 7 samples；12 步 Demo 浏览器复验 + 1366/1024 点检。**阻塞**：仓库无真实预发 SSH/HTTPS 域名；本机 **Docker 未安装** 无法 `docker-compose.full.yml` Nginx 模拟；Storage 公网 / 预发备份回滚 / 真实 HTTPS **待人工**。Git tag **Tag deferred**。**抖店仍 Release Candidate**；未进入真实 E2E / 生产灰度。

**Stage update**: 2026-06-27 — **Phase R1.1 MVP Demo 预发部署与人工走查完成**。状态仍为 **`MVP Demo Ready`**（非 Production Ready）。本地 dev 等价环境：`go test ./...`、`go build -o tmp/server.exe`、`pnpm build:admin` 通过；`/health` database+redis ok，9 worker running；路由 smoke 8 路由无 404（`docs/demo-route-smoke.json` 2026-06-27T05:42:58Z）；Demo 数据复跑 20 slot + 7 task samples；12 步 Demo 走查 11 通过 / 图片应用 1 警告（沿用试跑结果）；1366/1024 分辨率浏览器验收；失败任务中心正确路由 `/ops/task-center/failures`。`.env` 缺 10 个可选键（后端默认）；Nginx/HTTPS / Storage 公网 / 真实预发 tag **待人工**。Git tag **Tag deferred**。**抖店仍 Release Candidate**；未进入真实 E2E / 生产灰度。

**Stage update**: 2026-06-27 — **Phase R1 MVP Demo Release 收口完成**。状态 **`MVP Demo Ready`**（非 Production Ready）。新增脚本 `seed-demo-data`、`demo-route-smoke`、`ai-operation-workbench-perf`；Demo 数据 20 类商品 + 任务样本（`docs/DEMO_DATASET.md`）；路由 smoke 8 路由通过（`docs/demo-route-smoke.json`）；工作台 757 待办 summary/todos <100ms（`docs/AI_OPERATION_WORKBENCH_PERF_REPORT.md`）；批量刊登 20×2/50×2/100×3 编排耗时记录（`docs/PUBLISH_BATCH_PERF_REPORT.md`）；AI 文案试跑 16/16、图片 14/16 `passed_with_warning`；文案审计 + Collect 批次 partial_success 修复；权限安全检查文档；演示脚本与部署前检查清单。`go test ./...`、`pnpm build:admin`、抖店回归通过。**抖店仍 Release Candidate**；真实 E2E 未进入本阶段。

**Stage update**: 2026-06-27 — **AI 商品运营体验 Phase A3.3 AI 商品运营工作台已完成**。新模块 `aiopsworkbench`；API `/api/v1/ai/operation-workbench/summary|todos|todos/:id|todos/refresh`；前端 **AI 工具 → 商品运营工作台**（`/ai/operation-workbench`）。聚合 AI 文案/图片待复核与冲突、发布检查 failed/warning（复用 `productcheck`）、刊登批次 failed/partial_success、taskcenter 失败（去重）；支持筛选、分页、详情抽屉与跳转；**不**自动应用 AI、**不**自动上架、**不**调外部平台 API。`go test ./...`、`pnpm build:admin`、抖店回归通过。设计见 [`AI_OPERATION_WORKBENCH_DESIGN.md`](AI_OPERATION_WORKBENCH_DESIGN.md)。**抖店仍 Release Candidate**。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A3.2.1 批量 AI 图片真实 Provider 试跑与视觉验收完成（`passed_with_warning`）**。路由 smoke（`scripts/ai-image-route-smoke.ps1`）12 路由无 404；真实试跑 16 子项 10 成功（`scripts/ai-image-trial-run.ps1`）；应用/撤销/safedownload 验收（`scripts/ai-image-apply-undo-verify.ps1`）。修复 P1：① 运行后端需含 `aiproductimage` 路由的新二进制；② dashscope 白底图走 `replace_background` 而非硬编码 remove.bg；③ 质量评分写回 score 后刷新快照避免误报冲突。Provider `dashscope_image` 已配置；建议补 API Key 后复跑白底图 5 张。**建议可进入 A3.3 评审**。验收见 [`BATCH_AI_IMAGE_UX_ACCEPTANCE.md`](BATCH_AI_IMAGE_UX_ACCEPTANCE.md)。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A3.2 批量 AI 图片处理、复核、应用与撤销（代码交付）已完成**。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A3.1.2 批量文案真实试跑与路由部署验收已完成**。路由 smoke（`scripts/ai-text-route-smoke.ps1`）通过；真实 Qwen Provider 试跑 16 子项全部 `pending_review`（`scripts/ai-text-trial-run.ps1`）；修复 P1：异步 generation 使用 `detachedGinContext` 避免 HTTP context 取消导致卡 pending；`retry-failed` 同时重试 pending/running/failed。`go test` / `pnpm build:admin` / 抖店回归通过。**未做** A3.2 批量图片。验收见 [`BATCH_AI_TEXT_UX_ACCEPTANCE.md`](BATCH_AI_TEXT_UX_ACCEPTANCE.md)。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A3.1.1 批量文案运营可用收口已完成**。失败任务中心聚合 `ai_product_text_items`（`taskType=ai_text`）；深链 `/product/ai-text-batches/:id?itemId=` 高亮并打开复核；旧版 `/ai/batches` 隐藏菜单并加旧版提示，主入口 `/ai/text-batches`；冲突提示中文化；质量 warning 补强（含 `desc_unclear_structure`）；`go test` / `pnpm build:admin` / 抖店回归通过。真实 AI 小样本试跑在 A3.1.2 完成。验收见 [`BATCH_AI_TEXT_UX_ACCEPTANCE.md`](BATCH_AI_TEXT_UX_ACCEPTANCE.md)。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A3.1 批量 AI 标题/描述生成、复核、应用与撤销已完成**。新模块 `aiproducttext`；表 `ai_product_text_batches` / `ai_product_text_items`；API `/api/v1/products/ai-text/batches*`；商品列表「批量 AI 优化」四步向导；复核工作台 `/product/ai-text-batches/:id`；生成默认 `pending_review`，应用复用 `applyAIContent` 冲突保护；批量撤销；质量 warning 中文展示。**未做**批量图片、自动上架、平台独立标题策略。**抖店仍 Release Candidate**。设计见 [`BATCH_AI_TEXT_OPERATION_DESIGN.md`](BATCH_AI_TEXT_OPERATION_DESIGN.md)。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A2.2 批量刊登统一配置与覆盖配置 UI 收口已完成**。批量向导第 3 步「统一刊登配置」与第 4 步「单独配置」改为生产级表单（`PublishConfigEditor` + `OverrideConfigTabs`）；支持价格 / 图片 / 库存 / 包裹 / 备注结构化 `commonConfig` 与四层 `overrides`（商品 / 平台 / 店铺 / 商品目标）；第 5 步可查看生效配置与配置提醒；`localStorage` 向导草稿持久化；后端 `validateBatchPublishConfig` + `PUBLISH_CONFIG_INVALID` 中文错误；深度合并 `effectiveConfig` / `configSources`；**已移除**批量刊登相关 `window.prompt`。**未做**自动上架、新真实平台 API、批次队列化、Phase A3 批量 AI。**抖店仍 Release Candidate**；真实 E2E 待凭证。设计见 [`MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md) §Phase A2.2。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A2.1 批量刊登验收与生产安全收口已完成**。显式 PostgreSQL migration（[`PUBLISH_BATCH_MIGRATION.md`](PUBLISH_BATCH_MIGRATION.md)）；批量上限 env：`PUBLISH_BATCH_MAX_PRODUCTS=100`、`PUBLISH_BATCH_MAX_TARGETS=20`、`PUBLISH_BATCH_MAX_TASKS=300`（前后端校验，超限中文提示）；任务级 dedup 纳入 config hash；批次 `idempotency_key` 部分唯一索引（先查重复）；retry-failed 并发 claim；批次 `CreatedBy` 越权校验；集成测试 10 场景；性能脚本 `scripts/publish-batch-perf.ps1`；批次详情 UX / 失败任务联动收口。**执行策略**：create-drafts 仍同步 orchestration，单批 ≤300 子任务；抖店子任务仍 Redis worker；`local_draft_only` 不调外部 API。**未做**自动上架、新真实平台 API、批次队列化。**抖店仍 Release Candidate**；真实 E2E 待凭证。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A2 多商品批量创建刊登草稿已完成**。商品草稿列表支持多选 →「批量创建刊登草稿」；5 步向导（确认商品 / 选择平台店铺 / 统一配置 / 单独覆盖 / 检查并创建）；新增 `POST/GET /api/v1/product-publish/batch-targets*` 与 `GET/POST /api/v1/product-publish/batches*`；`product_publish_batches` 扩展 `batch_type=multi_product`、`product_count`、`task_count`、`idempotency_key`；每个商品 × 每个目标独立子任务，支持 `partial_success`、只重试失败项、取消等待项；抖店复用 create-draft；`local_draft_only` 不调外部 API；失败任务中心可跳转批次详情。设计见 [`MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md)。**未做**自动直接上架与新增真实平台 OpenAPI；抖店仍为 Release Candidate。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A1.2 刊登中文化 + 多平台刊登中心已完成**。面向运营人员的错误码 / 状态 / 字段统一中文映射（后端 `opslabels` + 前端 `productOperationLabels` / `publishLabels` / `platformLabels`）；商品详情「刊登」Tab 升级为三层结构（刊登目标 / 统一配置预留 / 各平台单独配置）；新增 `GET/POST /api/v1/products/:id/publish-targets*` 支持单商品多平台多店铺预检查与批量创建刊登草稿；抖店继续复用现有 create-draft 链路；TikTok / Shopee / Lazada / Amazon 等 `local_draft_only` 仅生成本地 publication 与任务快照；新增 `product_publish_batches` 表。设计见 [`MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md)。**未做**多商品批量发布与自动直接上架；抖店仍为 Release Candidate。

**Stage update**: 2026-06-19 — **AI 商品运营体验 Phase A1.1 真实样本验收通过，Ready for Phase A2**。完成 20 个真实/补丁商品样本矩阵（1688、拼多多、淘宝/天猫、custom、manual）；修复 P1：发布检查动作映射（`DOUYIN_SHOP_NOT_AUTHORIZED`、`collect.*`、`CATEGORY_REQUIRED`）、深链锚点 `publish-check`/`publish-config`、平台 Select 中文、多平台中心窄屏 overflow；列表 1000 条性能验证（pageSize=50 约 5–11ms，无 N+1）；新增验收脚本 `scripts/a1-acceptance-run.ps1`、`scripts/a1-prepare-samples.ps1`、`scripts/seed-product-list-perf.ps1` 与 `TestListAttachOperationProgressUsesFixedBatchQueries`。全量 `go test ./...`、`pnpm build:admin`、抖店回归通过。记录见 [`AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md`](AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md)。样本 #19 抖店真实 create-draft 仍为 `blocked_by_real_credentials`（不阻塞 A2）。

**Stage closure**: 2026-06-17 — **AI 商品运营体验 Phase A1 商品草稿全链路体验收口完成**。新增商品运营进度只读模型与 `GET /api/v1/products/:id/operation-progress`，商品草稿列表返回轻量 `operationProgress` 摘要并支持按当前步骤筛选；商品详情顶部展示完成度、当前步骤、阻断/建议数量和下一步入口，可直接打开基础信息、图片、规格、发布检查或刊登 Tab。发布前检查继续复用既有 readiness 结果，不复制前端判断；AI 标题/描述应用改为原始内容、AI 建议、准备应用内容对比，应用时校验 `expectedUpdatedAt` / 任务快照，写入 `product_ai_content_applications` 快照并支持安全撤销，避免静默覆盖后续人工修改。阶段未修改抖店 Provider 或抖店接口字段，抖店状态仍为 **Release Candidate**；未新增售后、财务、多仓、WMS、复杂 BI 或自动直接上架能力。

**Stage closure**: 2026-06-13 — **抖店 Phase 10.4 可观测性与发布收口完成**。不引入 Prometheus；复用 **taskcenter 告警**、**operationlog**、**operationdashboard**、**`GET /health`** 队列块作为抖店生产监控面。新增 E2E 脚本（`scripts/douyin-e2e-preflight|readonly|write|report` 的 `.sh`/`.ps1`，无凭证 exit `3` + `blocked_by_real_credentials`）；CI **`backend-race`** job（`CGO_ENABLED=1`，`-race` 覆盖 `douyinshop`/`ordersync`/`inventory`/`productpublish`）。文档：[`DOUYIN_PRODUCTION_AUDIT.md`](DOUYIN_PRODUCTION_AUDIT.md) §5.3、[`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md)、[`DOUYIN_ROLLBACK_DRILL_REPORT.md`](DOUYIN_ROLLBACK_DRILL_REPORT.md)（`environment_simulation_only`）、灰度 Runbook 更新。**发布状态仍为 Release Candidate**；真实 E2E 仍为 `blocked_by_real_credentials`。下一步：有凭证环境全链路 E2E + 48–72h 灰度观察 + 生产回滚实跑。

**Stage closure**: 2026-06-13 — **抖店 Phase 10.3 运行可靠性与安全加固完成**。

**Stage closure**: 2026-06-13 — **抖店 Phase 10.2 契约与一致性加固完成**。新增抖店 OpenAPI 脱敏 Fixture（`backend/internal/providers/platform/douyinshop/testdata/`）与契约测试（`contract_test.go`）；订单幂等 Postgres 唯一索引（`migrate_douyin_phase102.go`）；订单同步 partial_success **仅重试失败页**（`ordersync/checkpoint.go`、`SyncOrdersRequest.RetryPages`）；`hasMore` 且达 maxPages 时任务状态为 `partial_success`；商品草稿创建前 **`product.detail` 按 `out_product_id` 回查**避免超时重复创建；库存同步 **stockVersion + pending/running dedup**；E2E 报告模板 [`docs/DOUYIN_E2E_REPORT_TEMPLATE.md`](DOUYIN_E2E_REPORT_TEMPLATE.md)。**发布状态仍为 Release Candidate**；真实凭证 E2E 与重复数验收仍为 `blocked_by_real_credentials`。下一步：**Phase 10.3** 统一重试/限流/超时、stale 回收、紧急停用。

**Stage closure**: 2026-06-13 — **抖店 Phase 10.1 生产审计与预检完成**。新增生产预检 API（`POST/GET /api/v1/platform/douyin/production-preflight`）、Storage 公网访问验证（`POST /api/v1/storage/test-public-access`）、审计文档 [`docs/DOUYIN_PRODUCTION_AUDIT.md`](DOUYIN_PRODUCTION_AUDIT.md)；管理端：平台开放配置 → 抖店「生产预检」、存储设置「测试公网访问」。**发布状态仍为 Release Candidate**；真实凭证 E2E、`blocked_by_real_credentials` 项待人工验收。下一步：**Phase 10.2** 契约 Fixture、幂等、订单断点恢复。

**Stage closure**: 2026-06-13 — **管理端 Admin 文案与 UI 规范阶段性完成**：全站约 44 个业务页统一 **`TmPageContainer`**；新增 **`copywriting.ts` / `layoutTokens.ts` / `errorMessages.ts`** 与 **`components/ui`**（`SectionCard`、`TechnicalDetails`、`TaskJsonBlock` 等）；任务详情抽屉、发布检查、库存同步说明、店铺授权高级字段、采集/Prompt JSON 编辑等场景默认折叠技术信息；状态/平台/任务类型统一 **`commonStatusLabel` / `platformLabel` / 各模块 label 函数**；全局扫尾英文表头（platform、SKU、RequestID、URL 等）；规范写入 **`docs/ai-workflow.md` § Admin 文案与 UI 规范**。验证：**`pnpm build:admin` 通过**。后续新页面须按该节与 `PublishTasks` 抽屉模式实现。

**Stage closure**: 2026-06-07 — **抖店接入 Phase 1–9.2 已完成；主链路代码闭环完成，进入 MVP Demo Release 收口**。抖店 / Douyin Shop（`douyin_shop`）已支持：**平台配置**、**店铺 OAuth 授权**、**token 刷新**、**类目/属性同步**、**商品字段映射**、**图片上传（素材中心）**、**平台商品草稿创建（`product.addV2`，非直接上架）**、**订单同步（`order.searchList`）**、**SKU 绑定校准（`product.detail`）**、**SKU 手动绑定兜底**、**库存同步（`sku.syncStock`）**。验收文档：[`docs/DOUYIN_E2E_CHECKLIST.md`](DOUYIN_E2E_CHECKLIST.md)；演示清单：根目录 [`DEMO_CHECKLIST.md`](../DEMO_CHECKLIST.md)「抖店完整演示流程」。**剩余事项**：真实抖店凭证 E2E 验收；`product.addV2` / `product.detail` / `order.searchList` / `sku.syncStock` 线上字段校准；Storage `public_base` 公网可访问验证。**明确不在当前 MVP 范围**：售后/退款、财务结算、多仓 WMS、自动补货、直接上架 `publish_online`。建议 Release tag：`v0.8.0-douyin-mvp-demo`（E2E 通过后）。

**Stage closure**: 2026-06-07 — **Douyin Shop Phase 9.2 SKU manual binding fallback is implemented**. Building on Phase 9.1 auto-calibration, Phase 9.2 adds human fallback for `ambiguous` / `unmatched` publication SKUs: view platform SKU candidates (`platformSkus` cached on `product_publications.raw_data`), **manual bind** (`POST /api/v1/product-publication-skus/:id/douyin/bind-sku`), **unbind** (`POST .../unbind-sku`), and **recheck** (reuse sync-sku-bindings). Manual bind sets `bindStatus=bound`, `bindConfidence=100`, `bindMessage=手动绑定`, writes `external_sku_id`; validates conflict (same platform SKU on another local spec). Inventory sync now gates on all SKUs bind-ready via `inventorySyncReady` / `DOUYIN_SKU_BINDING_REQUIRED` / `DOUYIN_SKU_BINDING_CONFLICT`. New error codes: `DOUYIN_SKU_MANUAL_BIND_FAILED`, `DOUYIN_SKU_MANUAL_UNBIND_FAILED`, `DOUYIN_PLATFORM_SKU_ID_MISSING`, `DOUYIN_SKU_BINDING_REQUIRED`. Operation logs: `douyin.sku.binding.manual_bind/unbind/recheck/conflict`. Admin: Product Detail → Listing **抖店 SKU 绑定管理** table with manual bind / unbind / candidates drawer; Inventory tab blocks sync until bound. **Douyin main path enters full end-to-end acceptance** (see `DEMO_CHECKLIST.md`). Next: run acceptance with real Douyin credentials and Release hardening.

**Stage closure**: 2026-06-07 — **Douyin Shop Phase 9.1 SKU binding calibration is implemented**. After `product.addV2` draft creation, `product_publication_skus.external_sku_id` may be empty; Phase 9.1 adds official-doc-checked **`product.detail`** (`show_draft=true`) via `douyinshop.Client.GetProductDetail`, local↔platform SKU matching (attrs exact → spec name+price → similar=ambiguous, no low-confidence bind), and persistence of `bindStatus` / `bindConfidence` / `bindMessage` / `lastSyncedAt` on `product_publication_skus` plus `skuBindingSyncedAt` on `product_publications`. New APIs: **`GET /api/v1/product-publications/:id/douyin/sku-bindings`**, **`POST /api/v1/product-publications/:id/douyin/sync-sku-bindings`**. Inventory sync blocks unbound / ambiguous Douyin SKUs with **`DOUYIN_SKU_NOT_BOUND`** / **`DOUYIN_SKU_BINDING_AMBIGUOUS`**; new detail/binding error codes include **`DOUYIN_PRODUCT_DETAIL_FAILED`**, **`DOUYIN_PRODUCT_NOT_FOUND`**, **`DOUYIN_PRODUCT_DETAIL_PERMISSION_DENIED`**, **`DOUYIN_SKU_BINDING_SYNC_FAILED`**, **`DOUYIN_SKU_BINDING_UNMATCHED`**, **`DOUYIN_SKU_BINDING_AMBIGUOUS`**. Operation logs: **`douyin.product.detail.sync.start/success/failed`**, **`douyin.sku.binding.matched/unmatched/ambiguous`**. Admin: Product Detail → Listing adds「校准抖店 SKU 绑定」and inventory tab shows bind status; unbound SKUs cannot sync stock until calibration. Next: full Douyin end-to-end acceptance and optional direct online listing.

**Stage closure**: 2026-06-07 — **Douyin Shop Phase 9 inventory sync MVP is implemented**. Authorized `douyin_shop` stores with completed product/SKU binding can manually sync local stock to Douyin via existing inventory orchestration (`internal/modules/inventory`). Provider method **`SyncInventory`** in `douyinshop/inventory.go` calls official-doc-checked OpenAPI **`sku.syncStock`** (`product_id`, `sku_id`, `stock_num`, `incremental=false`) through Phase 3 `douyinshop.Client` with token auto-refresh, platform error mapping, and sanitized logs/raw. Reuses APIs **`POST /api/v1/product-publication-skus/:id/sync-inventory`**, **`POST /api/v1/products/:id/sync-inventory`**, **`GET /api/v1/inventory-sync/tasks*`**, **`POST /api/v1/inventory-sync/tasks/:id/retry`**, and batch APIs under **`/api/v1/inventory-sync/batches*`**. Sync is gated by **`inventory_sync_enabled`** in platform open config (default off). Pre-checks block unauthorized shops, missing `externalProductId` / `externalSkuId`, invalid stock, and disabled platform switch; missing platform SKU ID does not guess — surfaces **`DOUYIN_SKU_NOT_BOUND`**. Failures enter failure task center with codes such as **`DOUYIN_INVENTORY_SYNC_FAILED`**, **`DOUYIN_INVENTORY_PERMISSION_DENIED`**, **`DOUYIN_INVENTORY_RATE_LIMITED`**, **`DOUYIN_PRODUCT_NOT_BOUND`**, **`DOUYIN_SKU_NOT_BOUND`**, **`DOUYIN_STOCK_INVALID`**. New operation logs: **`douyin.inventory.sync.start/success/failed/retry`**, **`douyin.inventory.sku.failed`**. Admin: Product Detail → Inventory tab and Inventory Alerts page support Douyin sync when capability is `beta`. Scope remains tight: no multi-warehouse, no auto-replenish, no scheduled auto sync by default. Next phase: Douyin listing online / SKU calibration improvements or next platform capability.

**Stage closure**: 2026-06-07 — **Douyin Shop Phase 8.1 order sync pagination closure is implemented**. Phase 8 manual sync now auto-paginates `order.searchList` per task (`page` from 0, `size` from task `limit`), default **max 5 pages or 500 orders** per run. Platform open config adds **`order_sync_max_pages`** (default 5); task body may pass **`maxPages`**. Per-page failures record **`page` + error** in `order_sync_tasks.output.pageErrors`; mixed page success/failure yields **`partial_success`**. Output summary adds **`totalFetched` / `totalPages` / `successPages` / `failedPages` / `nextCursor` / `nextPage` / `createdOrders` / `updatedOrders` / `matchedItems` / `unmatchedItems` / `deductedStockItems`**. Scheduled polling remains off; no after-sale/refund; no Douyin inventory API. Next phase: Douyin inventory sync MVP.

**Stage closure**: 2026-06-07 — **Douyin Shop Phase 8 order sync MVP is implemented**. Authorized `douyin_shop` stores can manually sync orders via existing APIs **`POST /api/v1/shops/:id/sync-orders`**, **`GET /api/v1/order-sync/tasks*`**, **`POST /api/v1/order-sync/tasks/:id/retry`**. Provider method **`SyncOrders`** in `douyinshop/order.go` calls official-doc-checked OpenAPI **`order.searchList`** through Phase 3 `douyinshop.Client` with token auto-refresh, pagination (`page` from 0), time range (`create_time_start` / `create_time_end`), platform error mapping, and sanitized logs/raw. Orders upsert into `orders` / `order_items` / `order_shipments`; **`MatchOrderItemsForOrder`** matches `product_publication_skus.platformSkuId` and related rules; unmatched/ambiguous lines surface in the order exception workbench; local inventory deduct reuses **`DeductInventoryForOrder`** when policy allows. **`order_sync_enabled`** in platform open config gates sync (default off). Failures enter failure task center with codes such as **`DOUYIN_ORDER_LIST_FAILED`**, **`DOUYIN_ORDER_SYNC_FAILED`**, **`DOUYIN_ORDER_PARSE_FAILED`**, **`DOUYIN_ORDER_PERMISSION_DENIED`**, **`DOUYIN_ORDER_RATE_LIMITED`**. New operation logs: **`douyin.order.sync.start/success/failed/retry`**. Scope remains tight: no after-sale/refund sync, no Douyin inventory API, no scheduled polling by default. Next phase: Douyin inventory sync MVP.

**Stage closure**: 2026-06-07 — **Douyin Shop Phase 7 product draft creation is implemented**. Product Detail → Listing now supports creating a Douyin platform product draft from the saved mapping and uploaded material-center images. Backend calls the official-doc-checked OpenAPI method `product.addV2` through the existing Phase 3 `douyinshop.Client` with `commit=false` and `start_sale_type=1` (save as platform draft, not direct online listing). New APIs: `POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft`, `GET /api/v1/products/:id/platform-configs/douyin_shop/publish-tasks`, plus reused `GET/POST /api/v1/product-publish/tasks/*` (including `cancel`). Publish tasks persist `mappingSnapshot`, `platformPayload`, `platformResult`, `platformProductId`, `requestId`, `retryable`, and `platformRawError` (sanitized). Successful runs write `product_publications` (`publishStatus=draft_created`) and `product_publication_skus`. Pre-publish readiness checks block unauthorized shops, missing category/attributes, unuploaded main images, invalid SKU price/stock, and missing mapping. Failures enter the failure task center with codes such as `DOUYIN_CREATE_PRODUCT_FAILED`. New operation logs: `douyin.product.draft.create.start/success/failed`, `douyin.product.payload.build`, `douyin.product.publish_task.cancel`. Scope remains tight: no order sync, no inventory sync, no default direct online listing. Next phase: Douyin order sync MVP.

**Stage closure**: 2026-06-07 — **Douyin Shop Phase 6 image upload is implemented**. Product Detail → Listing now shows Douyin image upload status for main/detail images and supports uploading all images, force re-uploading all images, and retrying a single image. Backend APIs under `/api/v1/products/:id/platform-configs/douyin_shop/images/*` read the saved Douyin listing draft, sync external images into the current Storage Provider first, validate jpg/png/webp bytes with size and dimension checks, block private-network image URLs, call the official-doc-checked Douyin material center method `supplyCenter.material.batchUploadImageSync` through the existing Phase 3 `douyinshop.Client`, and persist `platformImageId` / `platformImageUrl` / upload status / failure reasons back to `product_platform_publish_configs.mapped_images`. Readiness checks now fail when main images are missing, not uploaded, or failed, and warn for detail-image partial failures. New operation logs: `douyin.image.upload.start`, `douyin.image.upload.success`, `douyin.image.upload.failed`, `douyin.image.upload.retry`, `douyin.image.storage.sync`, `douyin.image.processed`. Scope remains tight: no Douyin product creation, no order sync, no inventory sync. Next phase: create a Douyin product draft from the saved mapping and platform image IDs.

**Stage closure**: 2026-06-07 — **Douyin Shop Phase 5 product field mapping and listing preview is implemented**. Phase 5 maps internal product drafts to a saved `douyin_shop` listing draft preview without calling Douyin product creation or image upload APIs. `product_platform_publish_configs` now stores mapped title, description, images, SKUs, price, stock, warnings/errors, and `lastMappedAt`; new APIs cover build/read/save/validate mapping under `/api/v1/products/:id/platform-configs/douyin_shop/*`. Product Detail → Listing can generate a Douyin listing draft, preview title, description, main/detail images, image sync status, category attributes, SKU specs, price and stock, then save manual adjustments and validate before publishing. Readiness checks now include saved Douyin mapping errors/warnings. New operation logs: `douyin.mapping.build`, `douyin.mapping.save`, `douyin.mapping.validate`, `douyin.mapping.failed`. Scope remains tight: no Douyin product create API, no Douyin image upload, no order sync, no inventory sync, and no failure-task-center writes for mapping validation. Next phase: Douyin image upload / image service sync.

**Stage closure**: 2026-06-07 — **Douyin Shop Phase 4 category / attribute sync is implemented**. Phase 4 adds official-doc-checked Douyin OpenAPI methods `shop.getShopCategory` (`/shop/getShopCategory`, recursive from `cid=0`) and `product.getCatePropertyV2` (`/product/getCatePropertyV2`, `category_leaf_id`) through the existing `douyinshop` Client, reusing signed requests, token auto-refresh, safe error conversion, and secret-free logs. New cache tables: `platform_categories` and `platform_category_attributes`; new product listing prep table: `product_platform_publish_configs`. New APIs: `GET /api/v1/platform/douyin/categories`, `POST /api/v1/platform/douyin/categories/sync`, `GET /api/v1/platform/douyin/categories/:categoryId/attributes`, `POST /api/v1/platform/douyin/categories/:categoryId/attributes/sync`, `GET /api/v1/platform/douyin/categories/stats`, `GET/PUT /api/v1/products/:id/platform-configs/:platform`. Admin now supports category sync from Settings → Platform Open Config → Douyin Shop, local category search / leaf-only selection, attribute refresh and required attribute filling in Product Detail → Listing, and Douyin-specific readiness checks. Scope remains tight: no Douyin product publishing, image upload, order sync, or inventory sync. Next phase: product field mapping and Douyin image upload.

**Stage closure**: 2026-06-07 — Douyin Shop Phase 1 / Phase 2 / Phase 3 are closed for the previous milestone. Phase 1 completed `douyin_shop` Provider registration and platform open-app settings, including App Key, App Secret, Service ID, callback URL, environment, timeout, real-API switch, encrypted sensitive settings, and masked frontend display. Phase 2 completed the Douyin Shop OAuth authorization loop, including authorize start, callback, Redis state CSRF protection, code-to-token exchange, encrypted access/refresh token storage, token expiry persistence, refresh, revoke, and shop authorization status display. Phase 3 completed the Douyin Shop OpenAPI Client, HMAC-SHA256 signing layer, unified request/response/error handling, token auto-refresh, safe logs, real connection test, and shop-info calibration through the official token refresh response. Live smoke test was not completed because no real Douyin Shop app credentials / authorized shop token are available in the local environment. Phase 3 boundary at that time: no product publishing, order sync, inventory sync, image upload, or category/attribute implementation.

**Latest update**: 2026-06-06 — **Douyin Shop Phase 3 OpenAPI Client, signing layer, and shop-info calibration are implemented**. `backend/internal/providers/platform/douyinshop` now has a reusable client structure (`client.go`, `sign.go`, `request.go`, `response.go`, `errors.go`, `token.go`, `shop.go`) plus reserved files for category/image/product/order/inventory. The client centralizes common parameters, HMAC-SHA256 signing, `param_json` POST bodies, response parsing, platform error mapping, safe request logs, token auto-refresh, and per-shop refresh locking. Douyin shop connection test now performs a real platform-side token refresh and uses the official token refresh response to calibrate `shops` and `shop_auth_tokens`; failures keep the authorization record and mark `need_check`, `expired`, or `invalid`. New API: `POST /api/v1/shops/:id/oauth/douyin/sync-shop-info`. New/expanded provider error codes: `DOUYIN_API_ERROR`, `DOUYIN_AUTH_EXPIRED`, `DOUYIN_TOKEN_REFRESH_FAILED`, `DOUYIN_PERMISSION_DENIED`, `DOUYIN_RATE_LIMITED`, `DOUYIN_REQUEST_TIMEOUT`, `DOUYIN_RESPONSE_PARSE_FAILED`, `DOUYIN_SHOP_INFO_FAILED`, `UNKNOWN_DOUYIN_ERROR`. Scope remains tight: no product publishing, order sync, inventory sync, category implementation, image upload, or product draft creation in Phase 3. Next phase: category / attribute sync.

> **用途**：记录仓库当前真实进度，供后续会话（含 Cursor）快速对齐上下文，避免重复造轮子、偏离架构或漏掉已做决策。  
> **维护规则**：每完成一个**阶段**、一个**独立模块**，或一次**较大的代码修改**后，须同步更新本文件（含日期与变更摘要）。

**最新补充**：2026-06-06 — **抖店 / Douyin Shop Phase 2 OAuth 店铺授权闭环已落地**：在 Phase 1 平台配置基础上，新增抖店 OAuth 发起、公开回调、Redis state 防 CSRF（10 分钟，绑定管理员与 `platform=douyin_shop`）、`code` 换 `access_token` / `refresh_token`、token 加密保存、授权过期时间保存、店铺记录创建 / 更新、刷新授权、解除授权和店铺连接测试。新增接口：**`GET /api/v1/shops/oauth/douyin/start`**、**`GET /api/v1/shops/oauth/douyin/callback`**、**`GET /api/v1/shops/:id/oauth/douyin/authorize-url`**、**`POST /api/v1/shops/:id/oauth/douyin/refresh`**、**`POST /api/v1/shops/:id/oauth/douyin/revoke`**、**`POST /api/v1/shops/:id/oauth/douyin/test`**。操作日志新增 **`douyin.auth.start` / `douyin.auth.callback` / `douyin.auth.success` / `douyin.auth.failed` / `douyin.auth.refresh` / `douyin.auth.revoke` / `douyin.shop.info.sync` / `douyin.shop.connection.test`**，均不落 App Secret、access token、refresh token 或平台敏感 raw。管理端 **设置 → 平台开放配置** 支持连接抖店店铺，**店铺管理** 支持抖店重新授权、刷新授权、解除授权和测试连接。Phase 3 进入 **抖店 API Client + 签名层 / 店铺信息校准 / 类目属性**。**边界**：本阶段不做商品发布、订单同步、库存同步，不铺 TikTok / Shopee / AliExpress，不让前端直连抖店 API。

**此前补充**：2026-06-06 — **真实平台接入路线调整为抖店优先，Phase 1 平台配置已落地**：第一个生产级真实平台闭环不再优先从 TikTok Shop 展开，改为优先接入 **抖店 / Douyin Shop**（内部平台标识统一为 **`douyin_shop`**，不要与跨境 **TikTok Shop** 混用）。本次完成 Phase 1：新增 **`internal/providers/platform/douyinshop`**，注册 **`douyin_shop`** Provider；`GET /api/v1/platform/providers` 返回抖店；`GET`/`PUT /api/v1/platform/settings/douyin_shop` 支持 **App Key / App Secret / 回调地址 / 环境 / 超时 / 真实接口与订单、库存、商品草稿开关**；App Secret 走 settings 加密存储、前端脱敏；新增 **`POST /api/v1/platform/settings/:platform/test-connection`**，抖店 Phase 1 只做配置完整性校验，不猜测调用真实接口。下一阶段继续按顺序做 **OAuth 授权 → 店铺信息 → 类目属性 → 图片上传 → 商品草稿创建 → 订单同步 → 库存同步**。**边界**：不要多平台并行接入、不要自动直接上架、不要绕过平台审核、不要扩展复杂售后退款 / 财务结算 / 多仓 WMS / 自动补货；直接上架能力如后续开放必须二次人工确认。

**最新补充**：2026-06-06 — **采集能力阶段性验收完成，进入发布刊登生产级闭环**：1688 采集器 **已可用**，拼多多采集器 **已可用**，淘宝/天猫采集器 **已可用（含批量）**，自定义链接采集器 **基础可用**，速卖通采集器 **测试中 / beta**，SHEIN/Temu **规划中**。本阶段不继续新增采集器，优先完成「商品草稿 → 定价 → 图片同步 → 发布前检查 → 刊登任务 → 失败中心 / 操作日志」闭环。本次补齐商品详情统一草稿视图字段（`mainImages` / `descriptionImages` / `attributes` / `skuGroups` / `collectWarnings` / `publishStatus` / `raw` 等）、定价规则（成本来源、固定 / 百分比 / 倍率加价、运费、佣金、汇率、利润与尾数保护）、发布检查三态结果（`passed` / `warning` / `failed`，兼容旧 `ready` / `warning` / `blocked`）、刊登任务快照字段（`targetPlatform` / `targetStoreId` / `publishMode` / `checkResult` / `platformPayload` / `platformResult` / `errorCode` 等）和失败任务中心刊登错误码分类。**边界**：真实平台 API 仍按各 Platform Provider 能力执行；未真实接入的平台只生成 / 保存平台刊登草稿与任务快照，不扩展重型 ERP。

**最新补充**：2026-06-06 — **淘宝/天猫采集器生产可用收口（已可用 + 批量采集）**：采集中心状态 **`available`（已可用）**；**单品采集**与 **批量采集** 均已开放（`batchSupported=true`）。批量默认 **每批最多 20 条**、**并发 1**（最大 2）、**间隔 3500–6000ms**、**重试 2 次**；走 Redis 队列逐条执行，支持 **`partial_success`**；批量开始前校验采集服务、登录态与安全验证；无效淘宝/天猫链接自动跳过；遇 **LOGIN_REQUIRED / VERIFY_REQUIRED** 可暂停本批剩余任务（可配置）；失败子任务进入 **失败任务中心**；设置页新增 **淘宝/天猫批量配置**（开关、每批上限、并发、间隔、重试、登录/验证暂停）；操作日志 **`collect.taobao_tmall.batch.*`**。商品草稿 **`source=taobao_tmall`**、发布前检查与外链图片同步不变。**边界**：SKU / 库存 / 详情图仍建议发布前人工复核；不绕过验证码。

**此前**：2026-06-06 — **淘宝/天猫专用采集器生产级完善（测试中）**：专用 Provider **`taobao_tmall`**（非 custom）；支持 `item.taobao.com` / `detail.tmall.com` / `detail.tmall.hk` / `world.taobao.com` / `chaoshi.tmall.com` / `ju.taobao.com`；非标准淘宝生态链接返回 **`UNSUPPORTED_TAOBAO_URL`**；独立登录浏览器 Profile **`taobao_tmall`**；单品采集含标题清洗、价格区间、主图/详情图、参数、SKU 点击采集（可配置）、质量评分；登录/验证/下架/访问受限检测；设置页新增滚动等待、详情图等待、SKU 点击配置；失败任务中心与发布前检查（含外链图片提示）；商品草稿支持 **同步图片到平台存储**；验收表见 [`docs/collector-taobao-tmall-test-links.md`](collector-taobao-tmall-test-links.md)。**批量采集暂未开放**；库存与部分 SKU 需人工复核。

**此前**：2026-06-05 — **淘宝/天猫采集图片与 SKU 修复**：修正主图 URL 尾部 `_.jpg` 导致 404、预览空白；过滤 `s.gif` 占位图；新增页面 JSON 解析 `skuBase/skuCore` 补全规格；DOM 规格组展开为多条 SKU（不再仅默认一条）。

**此前**：2026-06-05 — **淘宝/天猫采集 HTTP 超时对齐**：修复后端 `COLLECTOR_TIMEOUT_SECONDS=60` 短于 Playwright 实际耗时（典型 ~100s）导致 `context deadline exceeded`；淘宝/天猫 Worker 按 `gotoTimeoutMs + 90s` 自动放宽单次 collector HTTP 超时（上限 300s）；默认 `COLLECTOR_TIMEOUT_SECONDS` 调整为 **120**。

**此前**：2026-06-05 — **淘宝/天猫专用采集器 Beta**：新增 Provider **`taobao_tmall`**（`source=taobao_tmall`），支持 `item.taobao.com` / `detail.tmall.com` / `detail.tmall.hk` / `world.taobao.com`；独立浏览器 Profile **`taobao_tmall`**（与 1688 / 拼多多 / custom 隔离）；单品采集（标题、价格、主图、详情图、参数、SKU 尽力识别）；登录检测 **`logged_in` / `login_required` / `verify_required` / `unknown`**；错误码含 **`LOGIN_REQUIRED` / `VERIFY_REQUIRED` / `ITEM_NOT_FOUND` / `PAGE_LOAD_TIMEOUT` / `MAIN_IMAGES_EMPTY` / `PRICE_NOT_FOUND` / `SKU_INCOMPLETE` / `DETAIL_IMAGES_INCOMPLETE`**；采集中心状态 **测试中（beta）**，**批量采集暂未开放**；设置页 **淘宝/天猫专属配置**；失败任务中心支持打开采集浏览器与重试；商品草稿页来源提示与发布前检查。**边界**：不保存密码、不绕过验证码、SKU/库存需人工复核。

**此前**：2026-06-05 — **跨 AI 工具工作流优化**：新增 `docs/ai-workflow.md`，为 Codex、Cursor、Claude Code、Copilot、Continue、Windsurf、Trae 等 AI 工具提供通用 vibe coding 流程；明确最小上下文包、任务分流、token 节约策略、标准执行流程、多工具协作入口和自我成长机制。同步 `AGENTS.md`、`docs/ai-coding-rules.md`、`docs/module-map.md`、`docs/task-checklist.md`、`docs/README.md`、`docs/cursor-rules-usage.md`、`.cursor/rules/13-ai-workflow.mdc`、`.cursor/rules/README.md`、`.cursorrules`、README / README.en、CONTRIBUTING 与 PR 模板，让长期规则留在仓库可审计文档中。

**此前**：2026-06-05 — **图片文字翻译人工可编辑兜底**：`translate_image_text` 在 OCR + 翻译 + Mask 擦除 + 背景修复 + 规则排版重绘 + 二次 OCR 质检之外，新增任务级人工编辑闭环；后端提供 `GET /image/tasks/:id/translate-edit-state` / `POST /image/tasks/:id/manual-render`（AI 路由同名），可读取原图、已擦除底图、结果图、可编辑文字块、排版框、擦除框与样式，人工修改文案/位置/字号/颜色/是否清原文后，使用程序擦除与规则重绘重新生成结果并上传 Storage Provider；任务输出记录 `manualEdit` 审计信息，状态回写 `success_with_review`，管理端「AI 图片任务」详情新增「人工编辑译图」弹窗。该兜底不使用“AI 直接生成翻译图”作为主链路。

**最后更新**：2026-06-05 — **图片文字翻译渲染校验与旧底清理修复**：二次 OCR 校验目标关键词覆盖 `fixedShortTranslation`、`badgeTranslation`、`compactTranslation`、`standardTranslation` 等实际绘制文案，避免 `Cool Black` / `Universal Stand` 已写入却被低估命中率；目标英文已命中、未溢出、未遮挡商品且商用分≥60 时，单个疑似原文残留、单字 OCR 碎片或局部补丁痕迹不再直接触发 `failed_render_validation`，改为 `success_with_review` 并保留复核提示；`pure_text_replace` 对 badge/pill 等标签块会先清理原胶囊/标签底和旧中文，再用黑色译文绘制，避免英文叠在旧中文和多余背景上。

**此前**：2026-05-30 — **pure_text_replace 坐标与擦除修复**：OCR 框落在左上角时改为右上角商品文案堆叠修复；支持归一化坐标缩放；白字标题保留 `#ffffff` 绘制；浅色胶囊按 `pill` 只擦黑字；擦除重试增至 3 次；质量分 68 且无硬错误时标记 `success_with_review` 而非渲染失败。

**此前**：2026-05-30 — **图片文字翻译 pure_text_replace 生产模式**：新增默认渲染模式 `pure_text_replace`（先擦除原字再绘制译文，禁止白底/气泡/标签补片）；title 仅重绘文字；badge/pill 保留原胶囊背景、只替换内部白字；固定短文案映射（炫酷黑→Cool Black、雪花白→Snow White 等）与 capsule 放不下时自动降级 Universal Stand/For Phones；质量校验原文残留/额外背景层/重叠则 failed 或自动重试；输出 original/erased/final 调试图；管理端展示「纯文字替换」与对应失败提示。

**此前**：2026-05-30 — **图片文字翻译 mask 空失败修复**：`text_pixel_mask` 去字 mask 在 dilate 后易因稀疏笔画合并成超大块（>45% 区域）被误判为空；新增 `dilateMaskWithinLimits` 自适应 dilate（必要时 dilate=0）、`cleanBorderBackgroundColor` 排除边框文字污染；修复 `[TRANSLATE_RENDER_FAILED] imagerender: text pixel mask empty`。

**此前**：2026-05-30 — **图片文字翻译 text_pixel_mask 生产级去字与质量分级**：固定流程 OCR → 文字像素 mask → 按 `blockClass` 分类去字（title/normal Telea inpaint radius=2；badge/pill `pillTextErase` 中位色填充白字、禁止整胶囊 inpaint）→ 短英文重绘 → 二次 OCR；mask 连通域过滤 + 单块≤2%/总计≤6%；质量分≥85 success、75–84 `success_with_review`（可下载、保存商品前提示）、65–74 同任务自动重试一次（maskDilate+1/更短文案/二次 inpaint）、<65 或中文残留/溢出/遮挡主体 failed；输出 `debugOriginalUrl`/`debugMaskUrl`/`debugErasedUrl`/`debugFinalUrl` 调试图；验收「雪花白 / 折叠伸缩版/通用手机」→ `Snow White` + `Universal Phone Stand`/`Universal Stand`。

**此前**：2026-05-30 — **图片文字翻译安全坐标映射与失败清理**：新增安全坐标映射器（normalized / crop 优先，禁止 ocrBox 硬映射）；小误差自动映射（aspectRatioDiff≤0.03、scale 0.5~2.0）、中等误差降级为 group 整体擦除重排（aspectRatioDiff≤0.08、scale 0.3~3.0）、大误差才 failed；完整输出 original/ocr/render/crop/scale/aspectRatioDiff/mappingMode/finalMappedBox 诊断；render 失败或 low_quality 时删除 temp/preview/output 文件并清空 DB resultUrl/output，重试前 supersede 同 sourceId+taskType+targetLanguage 旧结果为 obsolete；错误文案 `[TRANSLATE_RENDER_FAILED] OCR 图与渲染图尺寸差异超过安全映射阈值…`；单测覆盖同比例缩放、crop 局部 OCR、中等降级、大误差失败、失败清理与重试 supersede。

**此前**：2026-05-30 — **图片文字翻译 product_relayout 生产级落地**：默认布局改为 `product_relayout`（强制 `removeOriginalText` / `compactTranslation` / `allowTextOverflow=false`）；OCR block 按 proximity 合并为 `top_right_badge` 等 group 统一擦除重排；OCR 坐标映射支持 `cropOffset` + scale 并输出诊断日志；擦除 padding 按 title/badge/普通分级（16/14/8px），中文残留自动 1.5x mask 重试最多 2 次，纯色/渐变背景优先 `background_sample`；翻译输出 literal/compact/short/badge 四档并按 box 自动选型；渲染前 `measureText` 适配，溢出则任务 failed；二次 OCR 校验原文残留/溢出/重叠/遮挡商品主体，问题结果标记 `low_quality` 或 `need_manual_review`；前端低质量不再显示「成功」，禁用保存/设主图/设详情图，保留重试与下载预览。验收样例「炫酷黑 / 折叠伸缩版/通用手机」→ `Cool Black` + `Foldable Universal Stand` / `Universal Phone Stand`。

**此前**：2026-05-29 — **图片文字翻译生产级排版质量收紧**：OCR block 先分类为 `title` / `subtitle` / `badge` / `color_badge` / `small_caption`，只有满足黑底白字、小高度和胶囊区域特征的块才按 badge 渲染；`erase_bbox` 与 `layout_bbox` 继续分离，badge 宽高硬限制为原框 `1.35x / 1.25x`，超限时优先使用 `compact_translation`、缩字号或降级普通文本；绘制前新增布局模拟（溢出、碰撞、疑似遮挡商品主体、版面失衡），绘制后把异常 badge、背景补丁、原文残留等统一打入 `low_quality`；任务详情输出 block 分类、精简文案、bbox 数量、badge/异常 badge、背景补丁分、重叠分和 `finalQualityStatus`，管理端对低质量结果禁用保存到商品图片 / 设主图 / 设详情图。

**此前**：2026-05-29 — **图片文字翻译右上角标题与擦除修复**：修复商品图右上角标题（如「炫酷黑」）因包含“黑”被误判为黑底徽章的问题，单行大标题现在会进入 `main_title` 分组；徽章识别改为依赖暗色背景或斜杠标签，不再把普通颜色标题直接画成黑底胶囊；标题/普通文案擦除 padding 从过小的 2px 调整为 6–8px；二次加强擦除此前没有把 `opencv_inpaint` 结果写回画布，本次修复后才会真正生效，减少旧中文残留和英文叠旧文案。

**此前**：2026-05-29 — **图片文字翻译擦除校验不再误杀任务**：擦除后 OCR 改为按「原文字块仍识别数量」判定（需达到半数以上才算未擦干净），擦除不足时自动加强 `opencv_inpaint` 二次擦除后仍继续绘制译文；任务不再因擦除校验直接 `failed`，改为产出结果并标记 `low_quality` / `erase_failed`；全部策略失败时增加 `opencv_inpaint` 兜底渲染。

**此前**：2026-05-29 — **图片文字翻译 OCR 坐标映射与渲染质量收口**：修复 OCR 坐标与渲染图尺寸不一致时的统一缩放（`scaleX` / `scaleY`，任务详情展示 OCR/渲染尺寸与是否缩放）；外部 OCR 优先使用与渲染相同的 base64 原图，避免 URL 与本地解码尺寸不一致；渲染流程改为先擦除、二次 OCR 校验原文后再绘制译文；`erase_bbox` 与 `layout_bbox` 继续分离，标签圆角改为胶囊（修复 `BorderRadius:999` 导致的大黑圆块）；「牛奶白」识别为左下角 `bottom_badge`；`text_overflow` / `source_text_may_remain` / `badge_shape_abnormal` 等触发 `low_quality` 而非「成功（有警告）」；低质量结果不推荐保存/设主图/设详情图（需二次确认）；任务详情补充擦除/渲染诊断字段。

**此前**：2026-05-29 — **图片文字翻译严格 OCR 模式**：图片文字翻译改为严格 OCR 模式，已取消默认 OCR 降级逻辑；使用 `translate_image_text` 必须先在「设置 → 图片 AI 设置」选择 OCR Provider 并通过真实 OCR 调用测试，AI 视觉 OCR 作为独立 Provider，不再作为隐藏兜底；OCR 未配置、配置不完整、调用失败或未识别到文字时任务直接失败并提示用户完成 OCR 配置，不再出现“备用识别方式完成任务”的生产文案；任务详情展示配置 OCR 与实际 OCR，二者应保持一致；新建图片文字翻译入口会实时检测 OCR 可用状态，未通过时禁用提交并引导去配置 OCR。

**此前**：2026-05-29 — **图片文字翻译 OCR 执行一致性修复**：区分图片服务“配置检查”和 OCR“真实调用测试”，`POST /api/v1/settings/test-ocr` 对 AI 视觉、阿里云、腾讯云和 PaddleOCR 均执行真实 OCR 调用，只有返回 blocks 才提示“当前 OCR 服务可用”；图片文字翻译任务输出新增 `configuredOcrProvider`、`actualOcrProvider`、`ocrFallbackUsed`、`ocrFallbackReason`、`ocrErrorCode`、`ocrBlocksCount`、`ocrAverageConfidence`，阿里云 / 腾讯云失败会分类为 `SECRET_MISSING`、`AUTH_FAILED`、`SERVICE_NOT_OPEN`、`PERMISSION_DENIED`、`IMAGE_URL_NOT_ACCESSIBLE`、`TIMEOUT`、`EMPTY_BLOCKS` 等；任务详情页新增 OCR 配置与执行、渲染检查信息；检测到原文残留、背景补丁或擦除区域过大时标记 `low_quality`，并增加扩大擦除框与切换 `opencv_inpaint` / `background_sample` 的重试策略。

**此前**：2026-05-29 — **腾讯云 OCR Provider 正式接入**：OCR 配置新增腾讯云 OCR Provider，设置页下拉只显示生产可用的 `ai_vision`、`paddleocr`、`aliyun`、`tencent`，不再展示百度 OCR；已支持 AI 视觉 OCR、本地 PaddleOCR、阿里云 OCR 与腾讯云 OCR 真实调用；腾讯云 OCR 支持 `GeneralBasicOCR` 与可选 `GeneralFastOCR`，读取 Endpoint、Region、SecretId、SecretKey、接口类型、超时、最低置信度和失败降级设置；腾讯云 `TextDetections` 已转换为统一 OCR blocks，低置信度文字会被过滤并记录数量；`POST /api/v1/settings/test-ocr` 支持腾讯云真实测试并返回文字数量与平均置信度；图片文字翻译任务可使用腾讯云 OCR，失败且开启降级时自动切换到 AI 视觉 OCR，任务输出记录 OCR 服务、接口类型、是否降级、识别数量、平均置信度、过滤数量和错误信息。百度 OCR 暂不显示，后续完整实现后再上线。

**此前**：2026-05-29 — **图片文字翻译生产级擦除与校验增强**：`translate_image_text` 明确拆分 `erase_bbox` 与 `layout_bbox`，擦除使用 OCR 原始文字紧框，排版使用 group/template 布局框，避免为了放下译文而擦大块背景；默认擦除从 `background_sample` 切换为 `precise_mask` 精细擦字，`background_sample` 仅作为显式/兜底策略，质量不过时按 `tighter_erase_bbox`、`compact_font_layout` 与 `blur_fill` / `opencv_inpaint` 自动重试；输出 `eraseAreaRatio`、`patchAreaRatio`、`flatFillRatio`、`largePatchDetected`、`retryStrategies` 等诊断指标，强补丁/大擦除会拉低 `renderQuality` 并标记 `success_with_warnings` 或 `low_quality`；任务详情页补充擦除面积、补丁面积、背景破坏、自动重试策略等信息，并支持对低质量结果重新套用商品图模板。固定验收样例仍为手机支架图目标英文 `Metal Base` / `Foldable Stand` / `Phone/Tablet` / `Midnight Black`，主标题禁止浅色补丁，标签保持黑底胶囊。

**此前**：2026-05-29 — **图片文字翻译商用渲染整改**：`translate_image_text` 增加文字区域分组能力（`main_title` / `badge` / `bottom_badge` 等），手机支架样例会将「金属底座 / 折叠支架」合并为主标题组，并将「手机 / 平板」「暗夜黑」识别为标签组；新增 `layoutTemplate`（`auto` / `title_badge` / `preserve_original` / `ecommerce_clean`），当前手机支架图自动走 `title_badge` 商品图模板；样式继承增强，主标题保留黑色粗体层级，黑底标签重绘圆角胶囊背景；文字擦除改为按 group 控制 padding 并支持 warning 触发 `blur_fill` / `opencv_inpaint` 重试，降低突兀白底块；新增 `renderQuality` 评分（文字写入、原文清理、排版、风格一致性、可读性、商品保护、商用可用性），不合格结果标记 `success_with_warnings` 并输出 `background_patch_visible` / `layout_not_natural` / `source_text_may_remain` 等 warning；管理端详情页展示渲染质量、问题提示和多种重试入口，低分结果保存/设主图需确认。固定验收样例：手机支架图目标英文 `Metal Base` / `Foldable Stand` / `Phone/Tablet` / `Midnight Black`，要求商用评分 `commercialUsabilityScore >= 75` 才作为纯成功。

**此前**：2026-05-29 — **图片文字翻译生产级链路与 OCR Provider 接入**：新增 `OCRProvider` 抽象层（支持 PaddleOCR 与 AI 视觉兜底）；PaddleOCR 支持本地服务调用（`POST /predict/ocr_system`）；系统设置新增 `ocr_provider`、`ocr_service_url`、`ocr_timeout_seconds` 等配置；`ai_inpaint` 设为增强项，未配置时无缝降级程序擦除并增加 warning；完善 `translate_image_text` 批量并发控制（默认并发 1、独立成功/失败、支持重试）；批量任务支持状态汇总（`success`, `partial_success`, `failed`）；任务详情增加 OCR 服务类型及降级提示（AI 视觉/本地 OCR/备用降级）。后续增强预留：百度/阿里云/腾讯云 OCR Provider 接入、ComfyUI/通义万相 ai_inpaint 擦除支持。

**此前**：2026-05-29 — **图片文字翻译生产级链路**：`translate_image_text` 升级为 **OCR + 翻译 + 确定性排版渲染**；默认 `renderMode=hybrid`（程序擦除原文字 + 程序绘制译文），`ai_edit` 保留为实验模式；新增 `internal/pkg/imagerender`（字体/擦除/绘制/编码）；支持 `eraseMode` auto/background_sample/blur_fill/opencv_inpaint；输出 **二次校验**（hash 对比 + 结果 OCR），未改图则 **failed**（`IMAGE_NOT_CHANGED` / `IMAGE_TEXT_NOT_APPLIED`）；结果图仍走 **Storage Provider**、**不覆盖原图**；Docker 镜像安装 **Noto CJK** 字体；前端详情展示渲染/校验摘要与重试入口。

**此前**：2026-05-29 — **图片文字翻译自动排版增强**：`translate_image_text` 新增 **自动排版** 能力 — 支持翻译文字 **自动换行**、**自动调整字号**、文字区域 **轻微扩展**（≤30%）、过长文案 **自动精简**（`shortTranslatedText` + AI/规则）；`options` 增加 `autoLayout` / `autoWrap` / `autoFontSize` / `allowTextBoxExpand` / `allowTextSimplify` / `minFontSize` / `maxFontSize` / `lineHeightRatio` / `maxLines` / `layoutMode`（自动适配 / 尽量保持原图 / 优先清晰可读）；任务输出 **`quality.layout`** 摘要（换行/缩字号/精简/溢出计数与 warning 码）；前端 **「图片文字翻译」弹窗** 增加排版方式与处理选项；AI 图片任务详情 **翻译结果摘要** 展示排版统计与小白化警告；排版失败时 **`success_with_warnings`** 不阻断；结果图仍上传当前 **Storage Provider**、**不覆盖原图**。

**此前**：2026-05-29 — **AI 图片任务新增「图片文字翻译」**：新增任务类型 **`translate_image_text`**（图片文字翻译）；支持 **中文 → 英文**、**英文 → 中文**、**自动识别源语言**；流水线为 AI OCR 识别 → 文本翻译 → 图片编辑（OpenAI / 通义万相 / ComfyUI）；翻译结果上传至当前 **Storage Provider**（`products/{productId}/ai/translate_image_text/{yyyy}/{mm}/{uuid}.webp`）；可自动回写 **`product_images`**（不覆盖原图）；商品详情 **图片管理** 每张图增加 **「AI 翻译图片文字」** 入口；AI 图片任务页增加模板与翻译结果摘要展示；**第一版仅支持单图**，批量后续增强。

**此前**：2026-05-23 — **登录 / 注册页 UI 现代化**：`/user/login` 重构为响应式 SaaS 双栏布局（桌面 `grid: 1.1fr / minmax(420px,520px)`，平板/移动端单列）；左侧品牌区更新 slogan、简介与 6 项能力标签，背景装饰改为低透明度绝对定位卡片（`opacity 0.1–0.14`，不遮挡正文）；右侧登录卡片 `max-width 460px`、圆角阴影、Tab/输入框/渐变主按钮统一；`<1024px` 隐藏左侧宣传区并显示顶部 Logo；`<768px` 全宽卡片 + 16px 边距；**未改动**登录/注册接口与权限逻辑。

**此前**：2026-05-23 — **看板 SQL 容错（ai_description / MAX 扫描）**：`operationdashboard` 改用 `information_schema` 检测 `products.ai_description` 是否存在（缺列时回退 `description` 长度口径）；`MAX(updated_at)` 改用 `sql.NullTime` 避免无失败任务时 Scan 报错；启动迁移增加 `migrateLegacyProductTextColumns` 补全 `products` AI 文本列。

**此前**：2026-05-23 — **工作台看板验收修复**：修复 `operationdashboard` 聚合 SQL（`product_images`/`product_skus` 无 `deleted_at`、兼容缺失 `ai_description` 列）；前端 KPI 并入概览区（10 项，0 也展示）、待办/漏斗/异常/快捷入口始终渲染（接口空时用本地兜底结构）、最近动态空态增加引导按钮。

**此前**：2026-05-23 — **工作台 / 商品运营看板体验增强**：基于现有 `operationdashboard` 模块增强 `GET /api/v1/dashboard/product-operations`（新增 `funnel`、`exceptions`、紧凑 KPI 别名、12 项快捷入口、最近 10 条运营动态）；前端 `/dashboard/product-operations` 重构为五区布局（欢迎区、今日待办、运营概览、异常提醒、快捷入口）+ AI 运营进度漏斗 + 最近动态；支持时间/平台/店铺/来源筛选、60 秒自动刷新（页面隐藏时暂停）、loading skeleton / 空态 / 错误态；修复失败任务中心等深链为 `/ops/task-center/failures`；商品草稿 / 刊登 / AI / 图片 / 采集 / 库存同步列表页补齐 `?status=` 等 URL 筛选。**保持只读聚合**，不调用平台 API、不自动创建任务；完整 ERP 增强仍后置。

**此前**：2026-05-23 — **AI 图片任务「新建任务」小白化**：新增复用组件 **`CreateImageTaskModal`**（默认 9 类任务、商品图/上传/URL 选图、处理结果自动入库/设主图/设详情图、高级设置折叠 **商品图片 ID / 外部链接 / JSON 参数**）；**`/ai/image-tasks`** 与 **商品详情 → 图片管理** 共用弹窗；每张商品图旁 **AI 去水印 / 去 Logo / 去背景 / 营销图 / 评分 / 设为最佳主图** 一键预填；后端 **`autoSetDetail`** 支持任务成功后自动写入详情图。

**此前**：2026-05-23 — **AI 图片任务生产可用版收口**：新增 **`/api/v1/ai/image/*`** 标准路由（tasks / task-items save-to-product / set-as-main / score）；**`POST /products/:id/images/select-best-main`**（`score_only` / `recommend` / `auto_set`）；**`ai_image_task_items.product_id`**；发布检查增加主图评分/水印/Logo/二维码/详情图缺失告警；商品详情图片管理补全快捷 AI 操作与设主图/详情图。

**此前**：2026-05-21 — **AI 图片任务生产化 + 存储打通**：`image_tasks` 扩展为 AI 图片任务主表（含 prompt/options/result_count 等）；新增 **`ai_image_task_items`** 子项表；**`product_images`** 增加 source/source_task_id/original_image_id/storage_key/score/is_best_main 及 marketing/ai_generated 类型；支持 **14+ task_type**（去水印/去 Logo/综合清理/营销图/评分/自动选主图等）；Worker 结果 **统一下载并上传至当前 Storage Provider**（key：`products/{productId}/ai/{taskType}/{yyyy}/{mm}/{uuid}.webp`）；**POST /image/tasks/:id/apply** 一键回写商品图库；商品详情页与 AI 图片任务页增加模板入口与批量操作。

**此前**：2026-05-21 — **拼多多商品详情页采集提示优化**：合并重复蓝色/黄色提示为统一组件 **`ProductCollectQualityAlert`**；蓝色仅保留一句来源说明 + 字段识别小标签；黄色/红色仅展示基于商品实际数据的 **error / warning**（去重、最多 5 条可展开）；**info** 级 fallback 文案不再刷屏；**发布前检查**逻辑不变；**1688 / custom** 详情页不变。

**此前**：2026-05-21 — **拼多多图片采集完整性优化**：pifa 详情页新增 **缩略图轮播读取**（右箭头最多 5 次）、**点击缩略图获取大图**（最多 12 张）、**详情图分段滚动懒加载**（10 段 + 商品介绍 Tab）；主图来源含当前大图 / 缩略图 / 点击切换大图 / 页面脚本 gallery / SKU 兜底；**主图少于 3 张自动补充兜底**（`main_images_fallback_used` / `main_images_maybe_incomplete` warning，不阻断草稿）；**`raw.imageDebug`** 增加 `mainAreaCandidates` / `thumbnailClickedImages` 等摘要；单个与批量采集 **共用 `wholesale-detail` 解析链**；商品草稿页补 **主图不完整 / 详情图缺失 / 兜底提示**。**1688 / custom 不变**。

**此前**：2026-05-21 — **拼多多采集器生产化（available）**：Provider **`status=available`**、**`batchSupported=true`**；优先 **`pifa.pinduoduo.com/goods/detail`**；移动端商品页 / 批发首页 / 登录 / 微信 / App 引导返回明确错误码（**`UNSUPPORTED_PINDUODUO_URL` / `WECHAT_AUTH_REQUIRED` / `APP_REDIRECT` / `LOGIN_REQUIRED`**）；采集阶段 **部分成功**（缺主图/标题等 warning + 草稿），**发布前检查** 拦截无主图/无效价/SKU；**批量采集** 默认并发 1、间隔 4–9s、可配置重试；设置页 **`collect_pinduoduo_*`** 接入超时/访问检测/批量节流；失败任务中心与 **操作日志**（`collect.pinduoduo.*`）；README 小白教程。**边界**：不保存密码、不破解验证码、不绕过风控、不前端直连拼多多。**1688 / 速卖通 / custom 不变**。

**此前**：2026-05-21 — **拼多多主图兜底与采集容错（beta）**：修复图片分类过严导致 **`no_main_images` 采集失败**；放宽主图区/尺寸过滤（未知尺寸保留、主图区不因 URL 软关键词误杀）；**五级主图兜底**（画廊 → 缩略图 → SKU 图 → 详情首图 → 页面商品图池）；**`raw.imageDebug`** 记录各阶段候选数量；**采集阶段**主图缺失不再失败，生成草稿并 **`no_main_images` / 兜底 warning**，**`partial_success`**；**发布前检查**仍要求主图；失败任务中心 **图片缺失** 归类与拼多多提示文案；商品草稿页补图/兜底提示。

**此前**：2026-05-21 — **拼多多采集器图片分类精细化（beta）**：按 **DOM 区域** 分轨提取（左侧轮播 → **mainImages**、商品介绍区 → **descriptionImages**、规格行 → **sku.imageUrl**）；**SKU 图不再入主图**；增强 **srcset / data-* / background-image** 与高清去重；过滤店铺/客服/二维码等；**`raw.imageSummary`**；图片管理页 **主图/详情图/规格图** 分组排序与 beta 提示。**仍为 beta**；**批量仍关闭**；**1688 / custom 不变**。

**此前**：2026-05-21 — **拼多多 pifa 详情页解析精细化（beta）**：修复标题误拼 **「分享商品」** 等按钮文案（`cleanProductTitle` / `title_maybe_contaminated`）；**`mainDescription`** 从商品介绍/参数区提取（缺失 **`description_missing`**，入库 **`products.description`**）；**主图 / 详情图** 按 **`imageSource`** 分类并过滤店铺图/图标（**`imageFilterSummary`**）；SKU 名称剥离 **「仅剩 N 件」**；**`raw.quality`** 统计；商品草稿页 **字段级 beta 提示**。**仍为 beta**；**批量采集仍关闭**；**1688 / custom 不变**。

**此前**：2026-05-21 — **拼多多 pifa 批发详情页解析增强（beta）**：**`pifa.pinduoduo.com/goods/detail`** 专用 DOM 解析（`wholesale_detail`）；修复商品标题误抓 **`document.title` / 平台标题**；支持 **价格区间**（`priceMin` / `priceMax` / `priceRange`，`currency=CNY`）；解析 **SKU 行**（名称、行价、**仅剩 N 件** 库存）；**主图 / 详情图** 分区（左侧画廊 vs 商品介绍懒加载）；**商品参数** 尽力解析；采集质量校验与 **`partial_success`**；商品草稿页 beta / SKU 识别提示；后端 **有 SKU 不再生成默认规格**；**`BuildImportSKU`** 支持 `name` / `attrs`。**拼多多采集器仍为 beta**；**批量采集仍暂未开放**；**1688 / custom 不变**。

**此前**：2026-05-20 — **拼多多「重新检测」登录态修复**：新增 **`POST …/pinduoduo/check-login`**；检测优先级为传入 URL → 最近失败任务 → 设置项 **`collect_pinduoduo_auth_check_url`** → 仅 pifa 首页；首页可访问返回 **`homepage_only`**（不判已登录）；商品详情页需识别标题/价格/主图才返回 **`ok`**；返回 `profileKey`/`checkedUrl`/`finalUrl`/`urlType`/`evidence` 安全摘要；设置页增加检测用商品链接输入框；open-login 与 check-login 统一 **`pinduoduo` Profile**。

**此前**：2026-05-20 — **拼多多登录打开页策略**：不再默认打开 `mobile.yangkeduo.com` App 首页；优先使用失败任务/采集弹窗**原始商品或 pifa 链接**；新增 **`app_redirect`**；无上下文时回退 **pifa.pinduoduo.com**。

**此前**：2026-05-20 — **拼多多登录浏览器体验修复**：跳转 **`open.weixin.qq.com`** 时状态为 **`wechat_auth_required`（需要微信扫码授权）**，不再仅凭 Cookie 误判「已登录」；登录窗口 **1280×900**；重新检测访问拼多多首页/批发页并根据最终 URL 判断；设置页与失败任务中心补充微信扫码提示。

**此前**：2026-05-20 — **拼多多采集器专属登录态（对齐 1688 流程）**：设置页 `/settings/collector?provider=pinduoduo` 增加 **拼多多登录状态 / 重新检测 / 打开拼多多采集浏览器登录**；独立持久化 Profile **`pinduoduo`**（不复用 1688 / custom）；API **`GET/POST /api/collector/providers/pinduoduo/auth-status|open-login-browser`**；采集任务支持 **`useBrowserProfile` + `profileKey`**，**`pifa.pinduoduo.com` 批发页**自动建议登录 Profile；采集中心弹窗展示登录态与链接类型；失败任务中心 **`LOGIN_REQUIRED`** 提供 **打开拼多多采集浏览器登录 / 重试 / 打开原始失败页面**。**批量采集仍暂未开放**。**安全边界**：不保存账号密码、不破解验证码、不绕过风控、不把 Profile 内容返回前端、不前端直连拼多多。

**此前**：2026-05-20 — **拼多多采集器 URL 类型与登录态优化**：**`pifa.pinduoduo.com`** 识别为 **批发页**（`wholesale_detail`）；需登录时 **`LOGIN_REQUIRED`**；失败任务中心归类 **`login_required`（需要登录）**；采集弹窗/任务页 **实时链接提示**；失败详情展示 **链接类型 / 页面访问状态 / 建议操作**。

**此前**：2026-05-20 — **自定义链接采集器状态调整为基础可用（beta）**：采集中心卡片状态由「测试中」改为「基础可用」；更新卡片说明、能力标签（含商品价格，不含 SKU/库存）与批量采集提示；无规则时「开始采集」引导创建规则或 AI 生成；Modal 增加基础信息采集说明。**当前支持**：单链接采集、采集规则、AI 生成规则、规则测试、通用访问状态检测、登录 Profile、生成商品草稿。**边界**：SKU / 库存 / 动态价格 / 图片质量不保证完整；批量采集暂未开放。**建议**：已支持平台优先专用采集器，其他网站使用自定义链接采集器。

**此前**：2026-05-20 — **AI 生成采集规则质量增强**：Prompt 要求覆盖用户勾选字段（含 missingGeneratedFields / warnings）；禁止过宽 selector（h1/img 等）；生成后自动测试并返回 **qualityGate**（评分≥60 才允许保存启用）；前端展示识别评分、字段命中与「保存为草稿 / 保存并启用」分流；后端启用规则校验 mainImages；采集规则编辑页对仅 title 规则提示补充。

**此前**：2026-05-20 — **自定义链接采集器采集质量增强**：Collector 增加标题可信度检测、价格/币种归一化、懒加载主图/详情图提取与图片过滤；规则测试返回 **qualityScore** 与字段缺失提示；AI 生成规则 Prompt 强化（禁止全局 img/过宽标题 selector）；入库前纠正 currency 误写价格；管理端规则测试与商品草稿页增加自定义采集质量提示。**自定义采集器适合基础字段**；**SKU / 库存 / 动态价格不保证完整**；京东等复杂平台后续建议专用 Provider。

**此前**：2026-05-20 — **管理端全站文案小白化（二期）**：AI/图片/订单规格匹配/库存设置/店铺授权/后台监控/商品详情等页面统一中文表述；菜单「AI 技能模板」「规格匹配」「后台任务监控」；技术 JSON/错误码默认折叠或移至高级区。

---

## 当前产品路线

**当前阶段不是「完整 ERP 扩展阶段」。** 开发只收口以下两条主线：

1. **第一优先级**：**AI 商品运营工具**
2. **第二优先级**：**多平台跨境 ERP**（MVP）
3. **后续迭代**：**完整 ERP 增强**，**暂时不做**

**完整 ERP 增强**包括多仓、采购、售后退款、财务、预测补货、复杂 BI、WMS/OMS 等能力，**全部后置**到后续版本。

**当前开发重点：**

- **抖店整链路真实环境 E2E 验收**（见 [`DOUYIN_E2E_CHECKLIST.md`](DOUYIN_E2E_CHECKLIST.md)）
- **MVP Demo Release 收口**（文案/空状态/构建检查；不新增功能）
- **SKU 候选推荐**（已落地；订单异常工作台联调）
- **AI 商品运营工具体验打磨**

---

### 一、AI 商品运营工具当前目标

优先保证以下链路**可用、可演示、可小规模试用**：

采集商品 → 商品草稿 → **发布价格配置 / 定价规则** → **AI 标题优化** → **AI 描述生成** → **AI 图片处理** → **批量 AI 商品运营** → **商品发布前检查** → **商品刊登** → **库存预警** → **商品运营看板**

---

### 二、多平台跨境 ERP 当前目标

优先保证以下链路**可用、可演示、可小规模试用**：

店铺授权 → **平台订单同步** → **SKU 自动匹配** → **未匹配 SKU 进入异常工作台** → **SKU 候选推荐** → **人工绑定 SKU** → **订单扣减库存** → **平台库存同步** → **客服消息拉取** → **AI 客服建议** → **人工确认发送** → **失败任务中心 / 告警兜底**

---

### 三、完整 ERP 增强全部后置

以下能力**暂时不进入当前开发范围**，只作为后续版本迭代：

- 多仓库存
- 采购入库
- 供应商管理
- 库存预测
- 自动补货
- 财务结算
- 售后 / 退款
- 退货入库
- 平台售后单同步
- 平台批量库存 Feed 深度优化
- 飞书 / 企业微信真实通知
- AI 智能归因深化
- 复杂 BI 报表
- 完整 WMS / OMS / 财务系统

---

### 四、当前下一步

**SKU 候选推荐**已按主线交付（见 §3.2「内部订单」与变更记录 **2026-05-18**）。**做完该条目的收口与联调后，不要继续新增完整 ERP 高级模块。**

接下来进入：

1. **多平台跨境 ERP MVP 验收检查**
2. **AI 商品运营工具体验打磨**
3. **错误提示 / 空状态 / 引导文案优化**
4. **演示版本准备**

（与下文 **§8 下一步开发计划** 对齐。）

---

### 五、文档口径（必读）

在后续迭代与 Cursor 会话中，默认遵守：

- **当前阶段**只聚焦 **AI 商品运营工具** 与 **多平台跨境 ERP MVP** 两条主线。
- **完整 ERP 增强**一律视为**后续迭代**，不纳入当前排期。
- **工程收口**：优先把已落地能力做到**可演示、可小规模试用**，而非横向扩张重型 ERP 模块。

---

## 1. 当前阶段

| 维度 | 状态 |
|------|------|
| **全项目阶段** | **F1–F9 ✅**（2026-07-07）· **Phase H1.2 Post-F9 Enhancement** |
| **发布状态** | **MVP Demo Ready** · **Tag deferred** · **非 Production Ready** · 抖店 **Release Candidate** · 灰度 **不允许** |
| **路线图阶段** | **第 5 阶段（采集）**保持；**第 6 阶段（AI 图片）**保持（见 §3.2）；**AI 客服 MVP**：手工 / **平台拉取** 会话 + **AI 建议**；**仅人工**可 **发送到平台**（**不自动外发**） |
| **当前阶段定位（2026-05-19）** | **非**完整 ERP 扩展阶段；**只收口**：**① AI 商品运营工具**、**② 多平台跨境 ERP MVP**。**完整 ERP 增强**（多仓、采购、售后财务、WMS/OMS 等）**后置**，见上文 **《当前产品路线》** |
| **优先级** | **1）第一优先级：AI 商品运营工具** → **2）第二优先级：多平台跨境 ERP** → **3）后续迭代：完整 ERP 增强（暂时不做）**；供应链深能力**刻意不在当前排期** |
| **MVP 闭环** | 登录 → 配置 AI → 采集/草稿 → **AI 标题/描述** → **可选：手工录入或拉取平台客服消息** → 关联内部手工订单（可选）→ AI 生成建议（含订单/物流快照）→ **人工采纳（仅入库）或人工确认后外发到平台**（依赖有效大模型与平台客服权限） |
| **产物形态** | Monorepo 可构建；本地需 **PostgreSQL**；AI 调用走 **后端 Gateway**，前端 **不直连** 第三方模型 |

---

## 2. 阶段目标（第 1 阶段 — 项目地基）

本阶段（v0.1.0）的验收方向（与规则一致）：

- 项目可启动；**管理端可登录**（至少管理员）
- **统一 API 返回**、**系统设置可读可写**（`settings` 表 + 敏感字段加密）
- **本地存储**与**上传 API**（`POST /api/v1/files/upload`）已落地；`public_base` 与 **`GET /static/*`** 对齐；docker-compose 支撑 **PostgreSQL + Redis** 不变
- 后台 **系统设置页** 与配置后端连通；**不**在前端直连第三方 AI

> 说明：后端与管理端已具备 **操作日志**、**本地文件上传与列表**、**settings CRUD**、**test-ai / test-storage**；管理端 **登录 JWT、Bearer、401、access** 与各设置页、**操作日志 / 文件管理** 页已绑定 API。

---

## 3. 已完成事项

### 3.1 仓库与工程

- **Monorepo（pnpm）**：`pnpm-workspace.yaml`，根脚本 `dev:admin` / `build:admin` / `dev:collector` 等；**禁止使用 npm workspaces 与 package-lock 混用**（以 `pnpm-lock.yaml` 为准）。
- **Docker Compose**：本地 **PostgreSQL 16 + Redis 7**（根目录 `docker-compose.yml`）。
- **环境变量模板**：根目录 `.env.example`（含 backend / Redis / collector 等）。

### 3.1a 变更记录（2026-05-17）

1. **订单库存扣减基座**：… 2. **`order.Handler`/`ordersync`** 链式调用。3. **SKU 自动匹配（2026-05-17）**：**`order_item_sku_matches`**、**`MatchOrderItemsForOrder`**、管理端 **`/orders/sku-matches`** 与设置扩展。4. **库存预警基础（2026-05-17）**：**`product_skus` 阈值列**、**`GET /api/v1/inventory/alerts`**、**`PUT …/stock-settings`**、**`settings.inventory`** 预警默认项、管理端 **`/inventory/alerts`** 与草稿 **「库存」Tab**；**预警只读、不自动改库存/不回写平台**；本节仅作文档对齐。

### 3.2 后端（`backend/`）

- **Go + Gin** 可启动；**统一响应** `internal/pkg/response`（`code/message/data/traceId`）。
- **中间件**：RequestID（UUID）、**Recovery**（JSON 错误体，不泄露 panic 细节）、访问日志（slog）；**JWT Bearer** `middleware.BearerAuth`（受保护路由）。
- **配置**：`internal/config` 从环境变量加载（DB、Redis、**JWT**、`APP_MASTER_KEY`、**`UPLOAD_MAX_MB`（默认 10）**、**ADMIN_BOOTSTRAP_*** 等）；**生产环境**强制非默认 `JWT_SECRET`。
- **日志**：`internal/logger`（development 文本 / production JSON）。
- **数据库**：GORM，默认 **PostgreSQL**（`DB_DRIVER` 默认 `postgres`；未设置 `DB_PORT` 时默认 **5432**，MySQL 为 **3306**）；启动时 **Ping**；失败则进程退出。
- **迁移**：启动时 `database.AutoMigrate`：在既有 **`admin_users`/`settings`/`operation_logs`/`files`、商品与 SKU、`ai_operation_batches`、图片任务、订单与同步任务、店铺与授权、采集、worker、`ai_*`、客服相关表** 基础上，**`customer_conversations`** …；**库存** …；**`order_exception_marks`**（**订单异常工作台**：按 **`exception_type`+`source_type`+`source_id`+`mark_type`** 唯一标记 **`handled`/`ignored`**，**仅影响列表视图**，不改 **`orders`/`order_inventory_effects`/`inventory_sync_tasks`** 等业务终态）；**`task_failure_marks`**、**`task_alerts`**（失败中心 **ignored/handled 标记** + **站内告警去重**；**不篡改**业务任务状态）、**`task_alert_notifications`**（**告警外部通知投递审计**）；**`product_skus`** 增 **`warning_stock` / `safety_stock`**（及可选 **`stock_status` / `last_stock_checked_at`**，与 **`product.CalculateSKUStockStatus`** 口径一致）、**`cost_price` / `compare_at_price` / `min_publish_price`**（定价规则，2026-05-20）；**`order_item_sku_matches`**（**平台订单行 SKU 匹配审计**，与 **`order_items.product_id` / `product_sku_id`** 联动）；刊登 **`product_publish_tasks`/`product_publications`/`product_publication_skus`**；启动 seed：**`aiprompt.EnsureDefaults`**、**`settings.EnsureImageDefaults`**、**`settings.EnsureAIBatchDefaults`**、**`settings.EnsureInventoryDefaults`**、**`settings.EnsureTaskcenterDefaults`**（**幂等键含 `taskcenter.alert_detail_public_base`、出站/扫描策略等；初装默认多为空/false**）、**`settings.EnsureAlertNotifyDefaults`**（**含 `webhook_timeout_seconds`；敏感 URL/Secret 加密**）、**`settings.EnsurePricingDefaults`**（**`settings.pricing`** 默认定价规则）。
- **Redis**：`internal/rdb`（go-redis），连接失败 **仅告警**，服务继续（健康检查体现 `redis: skipped/degraded`）。
- **健康检查**：`GET /health`、`GET /api/v1/health`（含 DB/Redis 检查；**`data.collectQueue`** …；**`data.imageQueue`** …；**`data.orderSyncQueue`** …；**`data.customerMessageSyncQueue`** …；**`data.productPublishQueue`**（Redis 深度、`workerRunning`/`concurrency`）；**`data.inventorySyncQueue`**；**`data.workers`**：**`heartbeatEnabled`/`reaperEnabled`、按 `running`/`stale`（心跳推导）汇总、`byType`（`collect`/`image`/`order_sync`/`customer_message_sync`/`product_publish`/`inventory_sync`/`task_alert_scan`）**；Worker 查询失败时 **`workers.degraded`** 且整体可 **`degraded`**；**不对 Collector 做 HTTP 探测**以免拖慢健康接口）。队列开启且 Redis **Ping 正常但 LLEN 不可得**时整体 **`status` 可标记 `degraded`**。
- **ID 约定**：管理员等域表主键 **UUID**（`internal/pkg/model` + `internal/pkg/id`；GORM `char(36)`）；`settings` 表为 **`BIGINT` 自增**（与规则文档一致）。
- **认证**：`admin_users` 模型；`POST /api/v1/auth/login`（bcrypt 口令、**JWT HS256**）；`GET /api/v1/auth/profile`、`POST /api/v1/auth/logout`（无状态，客户端弃 token）。
- **JWT 上下文**：`BearerAuth` 写入 `ctxkey.AdminID` 与 **`ctxkey.AdminUsername`**（供审计与业务使用）。
- **操作日志**：`operation_logs` 表；模块 `internal/modules/operationlog`；**`GET /api/v1/operation-logs`**（分页；**action / username / resource / start / end（RFC3339）** 筛选）。写入场景：**登录成功/失败**、**logout**、**settings 批量保存成功/失败**、**platform.settings.update**（Partner 应用配置；**不落明文密钥**）、**test-ai / test-storage 成功/失败**（消息不落敏感配置明文）；**采集**：…；**商品**：…；**AI 标题**：…；**AI 描述**：…；**内部订单**：**`order.create` / `order.update` / `order.delete`（软删）**、**`order.item.{create|update|delete}`**、**`order.shipment.{create|update|delete}`**、**`order.sku_match.{auto|manual_bind|unmatched|ambiguous|rebuild}`**（摘要 **orderId / orderItemId / productSkuId**，**不落 token**）；**订单同步任务**：**`order.sync.create` / `order.sync.running` / `order.sync.success` / `order.sync.failed` / `order.sync.retry`**（**不落 token / 不全量 raw_data**）；**订单异常工作台**：**`order.exception.handle` / `order.exception.ignore` / `order.exception.unmark` / `order.exception.bind_sku` / `order.exception.retry_deduct` / `order.exception.retry_inventory_sync`**（摘要 **orderId / orderItemId / productSkuId / exceptionType / sourceId**，**不落客户正文 / token / 平台大包响应**）；**店铺 OAuth**：**`shop.oauth.tiktok.success|failed`、`shop.oauth.shopee.success|failed`、`shop.oauth.lazada.success|failed`、`shop.oauth.amazon.success|failed`**（不落 token）；**客服（MVP + 平台同步）**：**`customer.conversation.create` / `customer.conversation.update` / `customer.conversation.close`（含软删会话）**、**`customer.conversation.link_order`**、**`customer.message.create`**、**`customer.conversation.replied`**、**`customer.reply_generate.success` / `customer.reply_generate.failed`**、**`customer.reply_suggestion.edit` / `accept` / `discard`**（**不落客户正文全量**；可含 **conversationId / suggestionId / platform / 长度摘要**）；**`customer.platform_message.send.success` / `customer.platform_message.send.failed`**（**不落 token / 不全文记录客户消息**）；**库存预警（只读策略外仅记阈值/同步人工触发）**：**`inventory.stock_alert.update`**（**productSkuId / warningStock / safetyStock** 摘要）、**`inventory.stock_alert.batch_update`**（**matched/updated/筛选条件**摘要，**SKU ID 前缀截断**）、**`inventory.alert.sync_inventory`**（**`fromInventoryAlert` 成功**摘要，**不写 token / 不全量平台响应**）；**图片任务**：…
- **存储 Provider**：`internal/providers/storage` 接口 **Put / GetURL / Delete / Get**（调用方 **`Close` Get 返回体**）；**本地** `providers/storage/local`；**S3 兼容** `providers/storage/s3store`（**AWS SDK v2**）；**腾讯云 COS** **`providers/storage/cos`**（**`cos-go-sdk-v5`**）；**阿里云 OSS** **`providers/storage/oss`**（**`aliyun-oss-go-sdk`**）；**工厂** **`settings.storage.kind`** → **`local` / `s3` / `r2` / `minio` / `cos` / `oss`**（按 **`PlainByGroup`** 解密：**`s3_*`、`cos_*`、`oss_*`** 密钥）；**对象键** **`keypath.NormalizeSafe`** 防 **`..` 与非法路径段**；**不写密钥到日志**。**`GET /static/*filepath`** 仍仅 **`kind=local`**。
- **文件**：`files` 表；**`POST /api/v1/files/upload`**（`multipart` 字段名 **`file`**；**jpg/jpeg/png/webp/gif**；**objectKey = 日期目录/UUID.ext**；**`storage_kind` 落库**，**云端 `public_url`** 取自 **`GetURL`**）；**本地行为不变**。**`GET /api/v1/files`**（分页、`contentType`）；**`DELETE /api/v1/files/:id`**（**先做 Provider.Delete（按 `storage_kind`，缺省回落当前 settings.kind）**，成功后再删 DB，避免「库删了对象残留」）。
- **配置中心**：`settings` 模型与 `GET/PUT /api/v1/settings`；`item_value` 在 `is_encrypted=true` 时 **AES-GCM**（`APP_MASTER_KEY`）存储；列表接口 **脱敏**（`****` 规则）；PUT 若密文占位含 `****` 则 **不覆盖**原密钥，可更新 remark / value_type 等。
- **连通性测试**：`POST /api/v1/settings/test-ai`（…）；`POST /api/v1/settings/test-storage`（**`local`** 目录可写；**`s3`/`r2`/`minio`** **HeadBucket + 短时 context**；**`cos`** **COS Bucket HEAD**；**`oss`** **OSS `ListObjects` MaxKeys=1**；**均无真实 PUT 测试对象**）；**`POST /api/v1/settings/test-platform-tiktok`**（校验 **`platform_tiktok`** JSON 必填项，`RuntimeFromMergedMap`，**不真实请求 TikTok**）；**`POST /api/v1/settings/test-email`**（SMTP 试发；读 **`PlainMailSettings`**：`mail` 覆盖 legacy **`email`**）；**集成注册与总览**：**`GET /api/v1/settings/integration-schemas`**（静态 **`IntegrationConfigSchema`/`FieldSchema`** 说明各 **`settings.*` 分组职责**）；**`GET /api/v1/settings/integrations/overview`**（AI / image / storage / mail / 各 **`platform_*`** 应用配置是否齐全 / **`collect_rules` 计数**）；**`GET`/`PUT /api/v1/platform/settings/:platform`**：响应体含 **`schema` 与 `values`**；**Open Platform 应用级**按 **Provider `AppConfigSchema`** 写入；**`planned`** 允许**部分保存**（不完整不阻塞 PUT）；**`beta`/`available`** 仍做 TikTok **`RuntimeFromMergedMap`** 等完整性校验；**`sensitive`** **AES-GCM**、**masked `****`** 语义与 legacy settings 一致。**`storage`** 优先 **`s3_*`**（兼容遗留 **`endpoint`/`bucket`/`access_key`/`secret_key`/`region`**）。
- **默认管理员**：库中无管理员时，按 **`ADMIN_BOOTSTRAP_EMAIL` 与/或 `ADMIN_BOOTSTRAP_PHONE`**（至少填一项）及 `ADMIN_BOOTSTRAP_PASSWORD`（**非 production** 空密码 Fallback `changeme` 并告警；**production** 无用户则必须配置密码）插入一条记录；**不支持用「用户名」登录**，仅邮箱或手机号 + 密码；内部 `username` 列为随机 ID，由实现自行分配。
- **商品草稿**：模块 `internal/modules/product`；模型含 **`tenant_id`、`created_by`、JSONB `raw_data`** 及 **`product_images` / `product_skus`**（SKU **`attrs`、`raw_data` JSONB**，**前端不可改 raw_data**；**`product_skus.stock` 为本地当前库存**；**`warning_stock` / `safety_stock`** 为预警线与安全线，**新建 SKU 默认**可来自 **`settings.inventory`** 的 **`default_warning_stock` / `default_safety_stock`**）。**列表**：**`GET/POST /api/v1/products`**；**详情**：**`GET/PUT/DELETE /api/v1/products/:id`**；**`PUT` 可编辑**：`title`、`originalTitle`、`aiTitle`、`description`、`aiDescription`、`currency`、`status`；**一并支持 JSON snake_case**（如 `original_title`、`ai_title`）；**不写** `id` / `created_by` / `created_at`；**不通过 PUT 修改** `source` / `source_url` / `raw_data`（采集来源与归一快照只读）。**`status`** 枚举校验：`draft` / `ai_processing` / `ready` / `published` / **`archived`**。删除仍为 **软删除**（`products.deleted_at`）。**子资源**：**`POST/PUT/DELETE /api/v1/products/:id/skus`、`:skuId`**；**`PUT /api/v1/products/:id/skus/:skuId/stock-settings`**（**仅** **`warning_stock`/`safety_stock`**；**`inventory.stock_alert.update`**；**不改 `stock`、不写 `inventory_change_logs`、不触发库存同步队列**）；**`POST/POST(reorder)/PUT/DELETE /api/v1/products/:id/images`、`:imageId`、`/images/reorder`**；图片 **`image_type`** 支持 **`main` / `detail` / `sku`**（并接受旧值 **`description` 归入 detail**）；**可按 `files.id`（`fileId`）关联本地上传**；**删除 `product_images` 仅断开关联**，默认 **不删除 `files`** 存储对象。**采集入库**：新草稿详情图默认写入 **`detail`**（历史中可能仍有 **`description`** 行）。
- **AI Provider**：`internal/providers/ai` — **`ChatRequest` / `ChatResponse`**、**`Gateway`**（只读 **`settings.ai`**：`provider`（仅 **`openai_compatible` / `openai`** 首版落地）、`base_url`、`api_key` AES-GCM 解密、`model`、`temperature`、`max_tokens`、`timeout_sec`）；**业务仅调 Gateway**；**`openai_compatible/`** 实现 HTTP **`/chat/completions`**，**Context 超时** + **http.Client Timeout**；日志与响应 **不落 api_key**
- **Prompt 模板**：模块 `internal/modules/aiprompt`；表 **`ai_prompts`**；**`EnsureDefaults`** 插入默认 **`product_title_optimize`**（变量含 **`{{title}}` `{{category}}`…**）、**`product_description_generate`**（变量 **`{{title}}`… `{{tone}}`**，JSON 输出 **description / highlights / …**）与 **`customer_reply_generate`**（**scene `customer_service`**；变量 **`{{customerMessage}}` `{{conversationHistory}}`**、遗留 **`{{productInfo}}`**、**结构化 `{{orderInfo}}`、`{{orderItems}}`、`{{shipmentInfo}}`**、**`{{customerProfile}}`**、 **`{{language}}` `{{tone}}` `{{platform}}` `{{shopPolicy}}`**；内置 System **禁止捏造物流/退款事实**并强调 **UNKNOWN**；JSON 输出 **reply / intent / sentiment / riskLevel / notes**）；API：**`GET/POST /api/v1/ai/prompts`**、**`GET/PUT/DELETE .../:id`**、**`POST .../:id/enable|disable`**
- **AI 调用记录**：模块 `internal/modules/aitask`；表 **`ai_tasks`**（可选 **`product_id`**、**`conversation_id`** / 客服关联；`task_type` / `provider` / `model` / `prompt_code` / status **`pending|running|success|failed`** 等）；**标题优化**（**`title_optimize`**）、**描述生成**（**`product_description_generate`**）、**客服建议**（**`customer_reply_generate`**）各写一条；**`raw_response`** 仅存提供商返回 JSON 裁剪字段，**不含密钥**。**只读查询**：**`GET /api/v1/ai/tasks`**（分页；筛选 **taskType / status / provider / model / promptCode / productId / conversationId / start|end（RFC3339）**；列表 **不返回** `input`/`output`/`raw_response`）；**`GET /api/v1/ai/tasks/:id`**（详情含 **input/output/rawResponse**，响应前对 JSON 内 **api_key 等敏感键** 做 **`[REDACTED]`** 脱敏）；均 **JWT**、统一 **envelope**
- **统一店铺（平台授权基座，MVP）**：模块 **`internal/modules/shop`**；表 **`shops`**、**`shop_auth_tokens`**（敏感值 **AES-GCM**，API **脱敏 + masked 不覆盖**）。**`internal/providers/platform`**：**`Registry`** + **`manual` / `mock` / `planned`（各带 `AppConfigSchema` 预置）**。**TikTok Shop**、**Shopee**、**Lazada**、**Amazon SP-API** 在 **`api/router`** 分别 **`tiktok` / `shopee` / `lazada` / `amazon`** 绑定 **`BindShops` + `RegisterProvider()`**，状态 **`beta`**，能力 **`CapOrderSync`/`CapShopInfo`/`CapCustomerMessage`/`CapProductPublish`/`CapInventorySync`**（**`GET /api/v1/platform/providers`** 返回 **`capabilityStatus`**：**`inventory_sync`**：**mock=`available`**、**TikTok/Shopee/Lazada/Amazon=`beta`**、**manual=`disabled`**（详见库存同步条）；**`product_publish`**：**mock=`available`**、**TikTok/Shopee/Lazada/Amazon=`beta`**、**manual=`disabled`**；**`customer_message`**：**`mock=available`**，**`tiktok`/`shopee`/`lazada`/`amazon` 真实 **`PullMessages`/`SendMessage` 已接（`amazon` = SP-API **Messaging API** `beta`，见 §3.2 客服同步条）**，**`manual=disabled`**；用户在平台侧申请的客服/刊登权限反映在 **`shop_auth_tokens.scopes`**，**代码不写死 scope**）。**应用级**：**`GET /api/v1/platform/providers`** 返回 **`appConfigSchema`/`settingsGroupKey`**；**`GET`/`PUT /api/v1/platform/settings/:platform`** 写入 **`platform_tiktok` / `platform_shopee` / `platform_lazada` / `platform_amazon`** 等分组，其中 **`tiktok.beta`/`shopee.beta`/`lazada.beta`/`amazon.beta`** **`PUT` 前**做 **`RuntimeFromMergedMap`** 完整性校验，**`planned`** 仍允许部分保存；**敏感项加密**，**`platform.settings.update`** 审计；**access/refresh token** 仅落 **`shop_auth_tokens`**。**店铺级**：Drawer **可选**覆盖 **App Key/Secret（TikTok/Lazada）或 Partner（Shopee）/ redirect**。**OAuth（Redis `state`，10 分钟）**：**`/shops/:id/oauth/tiktok/*`**、**`/shops/:id/oauth/shopee/*`**（Shopee callback 需 **`shopId`**）、**`/shops/:id/oauth/lazada/*`**、**`/shops/:id/oauth/amazon/*`**；操作日志 **`shop.oauth.tiktok.*` / `shop.oauth.shopee.*` / `shop.oauth.lazada.*` / `shop.oauth.amazon.*`**。**`ResolveRuntime`（各平台包）** 合并 **settings 小写键 + 店铺覆盖**；缺省报错 **`platform config incomplete …`**（各平台固定文案）。**`shop.*` 不写密钥**。与订单：**`shopSummary`**。
- **平台订单同步（框架）**：模块 **`internal/modules/ordersync`**；表 **`order_sync_tasks`**（UUID、`shop_id`、`platform`、`task_type=order_sync`、状态、**JSONB `input`/`output`**，**不落 token**）；**Redis `ORDER_SYNC_QUEUE_*`** + **Worker**（**`ORDER_SYNC_QUEUE_ENABLED=false`** 时同步执行）；**编排**：校验 **`CapOrderSync`**／店铺 **`active`**；**`tiktok`、`shopee`、`lazada`、`amazon`（均为 beta）** 走 **`SyncOrders`**；**`planned`/`manual`** **501／不支持**；**`mock`** 模拟。**API**：**`POST /api/v1/shops/:id/sync-orders`**、**`GET /api/v1/order-sync/tasks*`**。**`order.UpsertSyncedOrders`** 入库 **`orders`/`order_items`/`order_shipments`**；随后 **`order.MatchOrderItemsForOrder`** 做 **本地 SKU 精确匹配**（**不在 Provider 内**），摘要写入 **`order_sync_tasks.output.skuMatch`**；**匹配失败不将同步任务标为 failed**；**扣库**仅在策略允许（含 **`auto_deduct_platform_orders` + `auto_deduct_after_sku_match`**）时 **`DeductInventoryForOrder`**；**`/health` `orderSyncQueue`**。**管理端**：**`/orders/sync-tasks`**、**`/shops` 同步**；**`/orders`** **platform/externalOrderId**、Drawer **「SKU 匹配」**、**`/orders/sku-matches`**；前端不直连平台。
- **平台客服消息同步（框架 + TikTok / Shopee / Lazada / Amazon 真实 API beta）**：模块 **`internal/modules/customersync`**；表 **`customer_message_sync_tasks`**（租约 + **`input`/`output` JSONB**，**不落 token**）；**`internal/providers/platform.CustomerMessageProvider`**（**`PullMessages`/`SendMessage`**；**mock** 可拉取/发送仿真消息；**TikTok** **`providers/platform/tiktok`**、**Shopee** **`providers/platform/shopee`**、**Lazada** **`providers/platform/lazada`**（**`/im/*` IM API**，字段映射 **随 Partner 文档演进时需校准**）真实接入；**Amazon** **`providers/platform/amazon`**：**Messaging API v1**（**`/messaging/v1/orders/{amazonOrderId}`**、**`/attributes`**、按需 **`POST .../messages/{template}`**），**Pull** = **Orders 分页 + 每单 Messaging actions（HAL）**（**非买家会话正文**，同步写入 **模板可用性摘要**）；**Send** = **优先可用 `{text}` 模板**（见 **`amazon/customer_messages.go`**）；**manual** 不支持）；**`customerchat.SyncPlatformCustomerMessages`** 幂等 upsert **`customer_conversations`/`customer_messages`**（裁剪 **`raw_data`**）；**`POST /api/v1/shops/:id/sync-customer-messages`**、**`GET /api/v1/customer/message-sync/tasks`**、**`GET .../:id`**、**`POST .../:id/retry`**；Redis **`CUSTOMER_MESSAGE_SYNC_*`** + Worker **`customer_message_sync`**（队列关闭时可同步执行）；**`/health` `customerMessageSyncQueue`**；**TaskReaper** 回收 **`customer_message_sync_tasks`** 超时 **`running`**；**`POST /api/v1/customer/conversations/:id/send-platform-message`**：**仅人工**触发外发，成功后 **`source=platform`** 写 **`customer_messages`**，**`accept` 建议仍仅内部记录**；权限不足时 **`platform customer message permission denied or not configured`**（HTTP **发送**接口 **403** 附中文说明，含 **Amazon Messaging / Buyer-Seller Messaging** 提示）。
- **库存同步基础框架 + TikTok / Shopee / Lazada / Amazon 真实写入（beta）**：模块 **`internal/modules/inventory`**；迁移 **`inventory_change_logs`**、**`inventory_sync_tasks`**。**库存预警列表（只读 + 人工同步入口）**：**`GET /api/v1/inventory/alerts`**（**keyword / productId / productSkuId / platform / shopId / alertType / stockStatus / onlyPublished / pageSize / `includeNormal`**；默认仅 **存在 `alertTypes`** 的 SKU；比对 **`product_skus.stock`** 与 **`product_publication_skus.stock`**（镜像快照）及 **`settings.inventory.platform_stock_mismatch_threshold`**；**最近一条** **`inventory_sync_tasks`** **`failed`** → **`inventory_sync_failed`**，**error 裁剪**、**不返回原始大包**）；**`POST /api/v1/product-publication-skus/:id/sync-inventory`** 可带 **`fromInventoryAlert`**，**Worker 成功后**记 **`inventory.alert.sync_inventory`**。**`InventorySyncProvider.SyncInventory`**：**mock** 模拟成功 · **manual** 显式不支持 · **TikTok** **`inventory_sync`**=`beta`（**`providers/platform/tiktok/inventory_sync*.go`**，**`warehouse_id`** 优先 **`tasks.input.options`**，否则 **`settings.platform_publish_tiktok`**；缺 **`warehouse_id`** → **`platform inventory config incomplete: missing warehouse_id`**）· **Shopee** **`inventory_sync`**=`beta`（**`providers/platform/shopee/inventory_sync*.go`**，**`POST /api/v2/product/update_stock`**；无变体 listing 的 **`external_sku_id`** 与 **`external_product_id`** 同为 **`item_id`**；多规格传 **`model_id`**；仓库/位置：**`options.warehouse_id`/`location_id`** 或 **`settings.platform_publish_shopee.warehouse_id`** → **`seller_stock`**；否则 **`normal_stock`**；平台返回 **failure_list** 中与仓/位置相关提示 → **`missing warehouse_id`** 摘要；权限类 → **`ErrPlatformInventorySyncPermissionDenied`**）· **Lazada** **`inventory_sync`**=`beta`（**`providers/platform/lazada/inventory_sync*.go`**：**`POST /product/price_quantity/update`**，`payload` **JSON**，**`/product/item/get`** 辅助 **`SellerSku`**；可选仓 **`WarehouseCode`**：**`platform_publish_lazada.warehouse_id` / tasks `options`**；权限类 **`ErrPlatformInventorySyncPermissionDenied`**；配置/仓提示与 **TikTok/Shopee** 同 **`missing warehouse_id`** / **`mapping incomplete`** 文案）· **Amazon** **`inventory_sync`**=`beta`（**`providers/platform/amazon/inventory_sync*.go`**：SP-API **Listings Items** **`PATCH …/items/{sellerId}/{sku}`**，**`productType` + patches `/attributes/fulfillment_availability`**；**`marketplace_id` / `fulfillment_channel` / `product_type`** 来自 **`platform_publish_amazon`** 与 **`options` 覆盖**，**`settings.platform_amazon.marketplace_id`** 兜底 **`marketplace_id`**；缺 **`marketplace_id`/`fulfillment_channel`/`product_type`** → **`platform inventory config incomplete: missing …`**；**Seller SKU** = **`product_publication_skus.external_sku_id`**（可辅以 **`sku_code`**）；**`external_product_id`/ASIN 可空**；权限类 **`ErrPlatformInventorySyncPermissionDenied`**）。**`GET /api/v1/platform/providers`** 含 **`capabilityStatus.inventory_sync`**。**队列：** **Redis** **`INVENTORY_SYNC_*`**；Worker **`inventory_sync`**；**`/health`** **`inventorySyncQueue`**；**`/workers/monitor`** **`leasedTasks.inventorySync`**；**TaskReaper** 回收 **`inventory_sync_tasks`**。**JWT**：**`POST /api/v1/products/:id/skus/:skuId/adjust-stock`**（**`inventory.stock.adjust`**；默认更新 **`product_skus`**，可选 **`sync=true`** 建批量任务）；**`POST /api/v1/product-publication-skus/:id/sync-inventory`**、**`POST /api/v1/products/:id/sync-inventory`**；**`GET /api/v1/inventory-sync/tasks`**（**`batchId` 筛选**、单条 **retry**）；**`POST|GET /api/v1/inventory-sync/batches`**、**`GET …/batches/:id`**、**`GET …/batches/:id/tasks`**、**`POST …/batches/:id/retry-failed`**、**`POST …/batches/retry-failed-tasks`**（≤100）；**批量创建** **`confirmAll`/`force`**、默认 **禁止空条件全量**、上限 **`inventory_sync_batch_max_size`**（默认 **500**）；**`inventory_sync_batches`**（**`batch_no`/`source`/`status`/计数/`input`/`output` 摘要**）；**`inventory_sync_tasks`** 可选 **`batch_id`/`batch_no`**；Worker **任务完成幂等聚合批次统计**（终态 **`success`/`partial_success`/`failed`**）；**平台限流**：**`inventory_sync_platform_rate_limit_*`**（初版 **defer/requeue**，精确延迟队列仍遗留在 §7）；操作日志 **`inventory.sync_batch.{create|success|partial_success|failed|retry_failed}`**（摘要）；**`GET /api/v1/products/:id/publication-skus`**；**`GET /api/v1/products/:id/skus/:skuId/inventory-logs`**、**`GET /api/v1/inventory/logs`**；**`inventory.sync.*`**（单任务；**不写 token**）。成功回填 **`product_publication_skus.stock`** 并记 **`inventory_change_logs`**（含 **`sync_success`** 等）。管理端：**`/inventory/alerts`**（**勾选 SKU → `inventory_alert` 批次**；展开行仍可 **单条 sync-inventory**）、**`/inventory/sync-batches`**、**`/inventory/sync-tasks`**（**失败批量重试**）、草稿 **「库存」Tab**（**刊登映射多选 → `product_detail` 批次**；单条 Modal 不变）、**`admin/src/services/inventory.ts`**；**.env.example** 含 **`INVENTORY_SYNC_*`**。
- **商品刊登真实 API（TikTok / Shopee / Lazada / Amazon beta）**：**`ProductPublishProvider.PublishProduct`** 统一消费 **`PlatformProductDraft`**，**productpublish** 模块只处理统一模型、任务状态与落库。**TikTok / Shopee / Lazada** 维持既有 **`beta`** 行为不变；**Amazon** **`providers/platform/amazon`** 新增 **`product_publish*.go`**，复用 **`ResolveRuntime` / LWA token refresh / SigV4 `doSPAPI`**，走 SP-API **Listings Items** **`PUT /listings/2021-08-01/items/{sellerId}/{sku}`**；配置复用 **`settings.platform_amazon`** 与 **`settings.platform_publish_amazon`**（Marketplace / Product Type / Brand / Manufacturer / 手工 **`amazon_attributes`** 等），授权复用 **`shop_auth_tokens`**（不写死 scope / token / marketplace / brand / SKU / ASIN）。成功或提交成功后 **product_publish Worker** 写 **`product_publications`** 与 **`product_publication_skus`**；Amazon 未立即返回 ASIN 时保持 **`publishing`**，不错误标记最终 **`published`**。公网图片校验不通过时返回 **`amazon product image must be publicly accessible`**；权限类统一 **`platform product publish permission denied or not configured`**。
- **内部手工订单**：模块 **`internal/modules/order`**；表 **`orders`**（订单号唯一、枚举 **status/paymentStatus/fulfillmentStatus**、可选 **shopId / externalOrderId / rawData**、可选 **`remark`**、金额与时标、软删）、**`order_items`**（可关联 **`products`/`product_skus`** 的快照行）、**`order_shipments`**（承运商、追踪号、`status`、`trackingUrl` 等）；与库存联动：**幂等 **`order_inventory_effects`****。**库存**：**`POST /api/v1/orders/:id/deduct-inventory`**、**`POST …/restore-inventory`**、**`GET …/inventory-effects`**。**SKU 匹配**：**`GET …/sku-matches`**、**`POST …/match-skus`**、**`GET /api/v1/order-item-sku-matches`**、**`POST /api/v1/order-items/:itemId/bind-sku`**（**幂等扣库仍走 `DeductInventoryForOrder`**）；**SKU 候选推荐（只读辅助）**：**`GET /api/v1/order-items/:itemId/sku-candidates`**、**`POST /api/v1/orders/:id/sku-candidates/batch`**（**`internal/modules/skucandidate`**，不写库、不入队库存同步、不调平台）；详情含 **`inventorySummary`**；手动创建 **`deductInventory` / `syncInventory`**。**`ordersync`**：`UpsertSyncedOrders` 后 **`MatchOrderItemsForOrder`**，再按策略 **`DeductInventoryForOrder(PlatformAuto)`**（**不影响**平台拉单事务成功态）。**订单 CRUD**：**`GET/POST /api/v1/orders`**（列表筛选 **`orderNo` / `customerName` / `platform` / …、可选 `shopId`**）、**`GET/PUT/DELETE /api/v1/orders/:id`**（**`shopId` 可空**；**`PUT` 解绑**透传 **`shopId` 空串** 等约定）；嵌套：**`POST/PUT/DELETE …/:id/items(/:itemId)`**、**`POST/PUT/DELETE …/:id/shipments(/:shipmentId)`**；详情 **Preload 行与子表**。**`ConversationSummary`** + **`BuildAIContext`**：供会话详情与 **`generate-reply`** 拼装 **`orderInfo` / `orderItems` / `shipmentInfo`**；同步写入的订单与手工订单共用 **`orders`**，**客服订单上下文链路不变**。**订单异常工作台（orderexception）**：聚合 **`GET /api/v1/orders/exceptions`**（**标记不影响原始订单/任务**）；写路径 **`bind-sku`/重试** 复用 **`BindOrderItemSKU`**、**`DeductInventoryForOrder`**、**`inventory.RetryInventorySyncTask`**。
- **AI 客服（MVP + 人工平台外发）**：模块 **`internal/modules/customerchat`**；注入 **`Orders *order.Service`**、**`Shops *shop.Service`**（**`shopSummary` 批量摘要**）；表 **`customer_conversations`**（可空 **`order_id`**、**`shop_id`**、**`external_conversation_id`**、**`raw_data`**）、**`customer_messages`**（**`message_type`/`external_message_id`/`raw_data`**）、**`customer_reply_suggestions`**。**列表**：**`GET /customer/conversations`** 支持 **`shopId`** 筛选；**`GET …/:id`** 带出 **`orderSummary`/`shopSummary`**。**`POST …/ai/generate-reply`**：**AI 不调用外发**。**`POST …/reply-suggestions/:id/accept`**：**仅内部采纳**。**`POST …/send-platform-message`**：**人工确认**后经 **Provider `SendMessage`** 外发。**管理端**：`/customer/conversations`（**拉取平台消息**）、`/customer/conversations/:id`（**店铺/外部会话 ID**、**采纳为内部回复** / **发送到平台** 二次确认）、**`/customer/message-sync-tasks`**；**`services/customer.ts`** 封装同步与外发 API。
- **AI 图片任务**：模块 **`internal/modules/imagetask`**；表 **`image_tasks`**（**`task_type`**：`remove_background` / …；**`status`**：**`pending` / `running` / `retrying` / `success` / `failed` / `cancelled`**；**`retry_count` / `max_retries` / `next_retry_at` / `retry_enqueued_at`**；**JSONB `input` / `output`**；**`source_image_id`**：**`files.id` / `product_images.id`**；**源解析**：**`source_resolver.go`** — 优先 **`storage_kind` + `object_key` → Provider `Get`**；失败则 **`public_url`/`origin_url`：`httppublic.IsPublicHTTPURL` → remove.bg `image_url`**；**`/static/...` 或 loopback `/static/...` → 本地 `object_key`**（见 §7 风险）；**`source_image_url`**：公网则 `image_url`，否则静态映射 / **`files.public_url` 精确匹配**再 `Get`；**`result_file_id` / `result_url`**；**不落库源图二进制**）。**Image Provider**：**`internal/providers/image`**：**`noop`**；**`removebg`**（**`image_file` 优先，其次 `image_url`**；**`internal/pkg/httppublic`**）；**`openaiimage`**；**`comfyui`**（**`POST /prompt`、`/history`、`/view`、`/upload/image`**；**日志不打 API Key / 完整 workflow**）；**`factory.NewForTask`：`noop` | `removebg` | `openai_image` | `comfyui`**，读 **`settings.image`**（密钥 **解密不写日志**，**不回退 `settings.ai.api_key`**）。**`remove_background`**：**强制 `provider=removebg`**。**`generate_scene` + `openai_image`**：**`prepareGenerateSceneHints` → `assembled_prompt`**；**+ `comfyui`**：同 **hints** + **模板变量**。**`generate_scene`**：**`openai_image` / `comfyui` 可无源图**。**`replace_background`**：**`openai_image`**（**后端 `resolveRemoveBGSource` → multipart `/images/edits`**；**`prepareReplaceBackgroundHints` → `assembled_prompt`**）；**`comfyui`**（须 **workflow + output 节点**；**`image` 节点** 用于上行源图）。**`IMAGE_QUEUE_ENABLED` + Redis**、**Worker**（认领 **`pending`** 或已调度入队的 **`retrying`**，条件 **`next_retry_at IS NULL`**）、**503 回滚**、**`GET /api/v1/image/tasks/monitor`**（**`retry` / `recentRetrying` / `recentFailures`**）、**人工 retry**；**`IMAGE_AUTO_RETRY_ENABLED`**（**.env 默认 true**；**`IMAGE_MAX_RETRIES` / `IMAGE_RETRY_*_DELAY`**）与 **`StartImageRetryScheduler`**（约 **5s**、到期 **CAS** **`LPUSH`** **`image:tasks`**）；可重试错误 **`IsRetryableImageTaskError`**（**5xx** / **429** / 超时 / 网络类；**缺 Key、workflow/JSON、源图不可读且非公网、`not implemented` 等**不重试）；操作日志 **`image.task.create` / `retry` / `success` / `failed` / `auto_retry_scheduled` / `auto_retry_enqueued` / `retry_exhausted`**（**不写密钥与完整 workflow**）；**Comfy 成功 `output`**：**`promptId`/`workflow`（空）** 等；**执行超时** 对 **`comfyui`** 不低于 **`comfyui_max_poll_seconds` + `comfyui_timeout_sec`**，再与 **`IMAGE_TASK_TIMEOUT_SECONDS`** 取 cap。**管理端**：**`/settings/image`**（**ComfyUI 大文本 workflow**）、**`/ai/image-tasks`（可选 `sourceImageId`；`replace_background` + `openai_image` 文案提示后端代传）**、**商品详情图片 Tab**（**`replace_background`：`openai_image` / `comfyui`**）。**其它**：noop **resize/enhance**；removebg **仅 remove_background**；openai **`generate_scene` + `replace_background`**。
- **商品 AI 标题**：**`POST /api/v1/products/:id/ai/optimize-title`**（body：`language` / `platform` / `maxLength`；**不自动改 `title`**）；**`POST /api/v1/products/:id/apply-ai-title`**（`aiTitle` + `taskId`，校验任务归属，**仅更新 `products.ai_title`**）；操作日志：**`ai.title_optimize.success` / `ai.title_optimize.failed` / `ai.title.apply`**（消息 **不含密钥与完整 Prompt**）
- **商品 AI 描述**：**`POST /api/v1/products/:id/ai/generate-description`**（`language` / `platform` / `tone`，默认 en / TikTok Shop / professional；**Preload `images`+`skus`**；**不自动改 `products.description`**）；**`POST /api/v1/products/:id/apply-ai-description`**（`aiDescription` + `taskId`，**仅更新 `products.ai_description`**）；**`GET /api/v1/products/:id/ai/tasks`**（详情页最近任务，列表 **省略大体量 JSON 列**，含 **`title_optimize`** 与 **`product_description_generate`**）；操作日志：**`ai.description_generate.success` / `ai.description_generate.failed` / `ai.description.apply`**（同上）
- **批量 AI 商品运营**：模块 **`internal/modules/aioperationbatch`**；表 **`ai_operation_batches`**（**批次号可读**、**计数列**、`input/output` **仅摘要**）；**文本批量** 复用 **`product.OptimizeTitleWithBatch` / `GenerateDescriptionWithBatch`** → **`ai_tasks`**（可选 **`save_ai_field`** 写入 **`ai_title`/`ai_description`**）；**图片批量** 仅 **`POST` 创建 `image_tasks`**（**主图 `product_images.id` 作源**），**Worker 与消费者不变**；**`POST …/retry-failed`**（文本 **再调 AI Gateway**，图片 **`RetryEnqueue`**）；**`apply-results`** 仅从 **成功任务的 `output` JSON** 写入 **`products.ai_*`**；**并行** **`ai_batch_concurrency`** + **`c.Copy()`**；**`batchId`** 可查 **`GET /api/v1/ai/tasks`、`/api/v1/image/tasks`**；详见 **头部「最后更新」** 与方法列表。
- **发布价格配置 / 商品定价（pricing，2026-05-20）**：模块 **`internal/modules/pricing`**；**`CalculatePublishPrice`**（成本价优先、**percent/fixed/none** 加价、最低利润率/最低价保护、尾数 **none/integer/.9/.99/.95**、可选 **`exchangeRate`** 手填）；**`settings.pricing`** + **`EnsurePricingDefaults`**（全局默认 + **TikTok/Shopee/Lazada/Amazon** 加价覆盖、**`batch_max_size`** 默认 **500**）；**`product_skus`** 增 **`cost_price` / `compare_at_price` / `min_publish_price`**；采集入库 **`BuildImportSKU`** 将归一 **`price`** 写入 **`cost_price`** 并初始化 **`price`**；**JWT**：**`POST /api/v1/pricing/calculate`**（试算）、**`POST /api/v1/products/:id/pricing/apply`**（**`confirm=false` 预览 / `confirm=true` 写 `price`**）、**`POST /api/v1/products/pricing/batch-apply`**（**`productIds` 或 `filters`**，空条件须 **`confirmAll`**）；操作日志 **`pricing.product.apply` / `pricing.batch_apply` / `settings.pricing.update`**（摘要 **skuCount/platform/markup**，**不落 SKU 全表**）；**边界**：**仅改本地 `product_skus.price`**，**不**创建 **`product_publish_tasks`**、**不**调平台 API、**不做**财务结算。
- **商品发布前检查（productcheck）**：模块 **`internal/modules/productcheck`**；**只读**本地 **`products` / `product_skus` / `product_images`** 与 **`settings` / `shops` / `shop_auth_tokens` 解密摘要**；**不下载图片**、**不请求平台 HTTP**、**不写商品**、**不入 `product_publish_tasks`**；**`GET /api/v1/products/:id/readiness`**；**`POST /api/v1/products/readiness/batch`**（≤100）；**`POST /api/v1/products/:id/publish`** 创建任务前 **`CheckProductReadiness`**，**error 阻断**（**`BlockedError` → HTTP 400**，**`message`**=`product readiness check failed`，**`data`**=结果）；**定价增强**：**`pricing.price_missing` / `pricing.price_invalid` / `pricing.price_below_cost` / `pricing.price_below_min_publish_price` / `pricing.compare_at_below_price`**；**`productpublish.Service.Readiness`** 注入；操作日志 **`product.readiness.check` / `product.readiness.batch_check`**（**不落 token / secret / 全量 settings**）。
- **多实例 Worker 与任务租约（MVP）**：模块 **`internal/modules/worker`**（**`worker_instances`**：`worker_id`/`worker_type`（**`collect`/`image`/`order_sync`/`customer_message_sync`/`product_publish`/`inventory_sync`/**`task_alert_scan`**）/`status`（`running`/`stale`/`stopped`）/心跳；**`WORKER_HEARTBEAT_*` / `WORKER_STALE_*` / `WORKER_REAPER_*` / `WORKER_LEGACY_RUNNING_TIMEOUT_SECONDS`**）；**`internal/modules/taskreaper`** 定时回收 **`locked_until`** 到期 **`running`**（采集走现有自动重试/失败；图片走 **`ErrWorkerLeaseExpired`** 与退避；订单同步 **直接 failed**+**`order.sync.lease_expired`**；**客服消息同步任务**/**`product_publish_tasks`/`inventory_sync_tasks`** 同订单同步策略回收租约）；**legacy**：**`running` 且 `locked_by` 空**且 **`updated_at` 过旧**。**JWT**：**`GET /api/v1/workers/monitor`**（**`leasedTasks.customerMessageSync`/`productPublish`/`inventorySync`**）。管理端：**`/workers/monitor`**。操作日志：**`worker.instance.start`/`stop`**；采集事件 **`worker.lease.acquired`/`expired`/`recovered`**（及既有 **`task.*`**）；图片 **`image.task.lease_expired`**。
- **统一失败任务中心（taskcenter）**：模块 **`internal/modules/taskcenter`**；表 **`task_failure_marks`**（跨模块 **`ignored` / `handled`** 视图标记，**不写入**业务任务状态）、**`task_alerts`**（站内告警：**`task_type` + `source_id` + `failure_category`** 去重；**不改变**业务任务状态）。聚合 **collect / image / order_sync / customer_message_sync / product_publish / inventory_sync**，归一 **`failed` / `retrying` / `stale` / `lease_expired`** 等；**规则归因** **`internal/modules/taskcenter/failureclassifier`**：**`failureCategory`** / **`severity`** / **`classificationReason`（裁剪）** / **`matchedRule`** / **`suggestedAction`**；列表与详情回填 **`alertStatus`**、**`relatedAlertId`**；**query** 支持 **`failureCategory` · `severity`**；**JWT**：**`GET /api/v1/task-center/failures` · `GET /summary` · `GET /failures/:taskType/:id` · `POST …/retry` · `POST /failures/batch-retry` · ignore/handle/unmark · `batch-ignore` / `batch-handle` · `POST …/generate-alert`**；**告警**：**`GET /task-center/alerts`**、**`POST /task-center/alerts/scan`**（**手动扫描**，默认 **不在 failures 列表读路径写库**）、**`POST …/alerts/:id/handle`、`ignore`、`DELETE …/mark`**；（**不写 token · 不大包 JSON · 不落客服正文**）；**retry** **分发**到原有 Service；成功后 **清除 marks**；**出站**：**`task_alert_notifications`** 表、**`internal/modules/taskcenter/notify`**（**SMTP 仅 `settings.mail`/`email`**、**Webhook `X-TradeMind-Signature`**、**飞书/企微 planned**）；**策略与渠道全部读 `settings.taskcenter` + `settings.alert_notify`**（**无 .env 业务项**；**`alert_detail_public_base`** 拼详情 URL；**`webhook_timeout_seconds`** 控 HTTP 超时）；**扫描/手工生成告警后 `NotifyGeneratedAlerts`**（**open**、**严重等级**、**渠道去重 / `alert_count` 重复策略**）；API **`GET /task-center/alert-notifications`**、**`POST /alerts/:id/notify`**；**`task_alert_scan`** Worker（**`TASK_ALERT_SCAN_ENABLED`** + **`enable_alert_scan_worker`**，**进程内定时**，**非 Redis list**）；操作日志 **`task_center.*`** 与 **`task_center.alert.scan` / `.generated` / `.handle` / `.ignore` / `.unmark` / `.notify.*` / `.scan_worker.*`**（仅存 **摘要**）；**`settings.EnsureTaskcenterDefaults`**、**`settings.EnsureAlertNotifyDefaults`**（分组 **`taskcenter`/`alert_notify`**，初装默认 **空/false** 由管理员在 **系统设置 / 告警通知配置** 填写）。初版 **`id::text`** 关联；列表 **`total`/`summary`** 仍受 **按类抓取上限** 约束。**全文索引**、**失败趋势大屏**、**通知自动重试**、**飞书/企微真实发送**、**AI 归因深化** — 见 §7。
- **商品运营看板（operationdashboard）**：模块 **`internal/modules/operationdashboard`**；**JWT**：**`GET /api/v1/dashboard/product-operations`**（**只读**；**不调平台**、**不创建任务**、**不写库**、**不写 operation_logs**；**`summary`/`todos`/`funnel`/`exceptions`/`quickLinks`/`recent`**）；复用 **`inventory.ListInventoryAlerts`**、**`taskcenter.Summary`**、**`orderexception.DashboardSummary`**；**`GET /api/v1/products`** 支持 **`missingAiTitle`/`missingAiDescription`/`readiness=blocked`/`publishable`/`status`** 深链筛选；管理端 **`/dashboard/product-operations`**（五区工作台布局、筛选、60s 自动刷新）、**`src/services/dashboard.ts`**。
- **采集任务与批次**：模块 `internal/modules/collect`。表与 **`collect_task_events`**、`GET …/tasks/:id/events`、`COLLECT_*` Worker/队列、`GET …/monitor` **与历史一致**。**1688 批量稳定性（2026-05-20）**：**`COLLECT_BATCH_CONCURRENCY_1688`（默认 1）**、**`COLLECT_BATCH_DELAY_*_1688`（默认 1500–5000ms 随机）**、**`COLLECT_BATCH_RETRY_ON_BLOCKED/TIMEOUT`**、**`COLLECT_BATCH_MAX_RETRIES_1688`（默认 2）**；**settings.collector** 同键可覆盖；Worker 对 **`batch_id` 非空 + source=1688** 任务：**`batch.delay.applied` 事件**、**Redis 并发门闸**、**批量专用退避**；**`GET /collect/batches/:id`** 返回 **`retryingCount`/`blockedCount`/`timeoutCount`/`parseFailedCount`/`errorSummary`**；任务 DTO 含 **`collectorErrorCode`/`retryable`/`failureHint`/`sameUrlSucceededElsewhere`**。**自定义规则**：模块 **`internal/modules/collectrule`**，表 **`collect_rules`**（声明式 **`rule` JSONB**、域名 / 可选 **`match_pattern`**、`priority`、`enabled/disabled` 软删）；**JWT**：**`GET/POST/PUT/DELETE /api/v1/collect/rules`**、**`POST …/:id/enable|disable|test`**（测试调用 Collector，**不写** `collect_tasks` / `products`）；创建/更新 **`rule`** **≤64KB**、selector **长度校验**；操作日志 **`collect.rule.*`**。**任务**：**`collect_tasks.request_options`**（JSONB）保存 **`ruleId`/`ruleName`/域名/`rule` 快照**供 Worker 下发 Collector。**Provider 驱动契约**：**`GET /api/v1/collect/providers`**（JWT，优先 **`Collector` `GET /v1/providers`**，失败用 **内置兜底**）；**`POST …/collect/tasks`** **`provider.status`** 允许 **`available` 或 `beta`**（`planned`/`disabled` 拒单）；**`source=custom`**：必选 **`ruleId`**（UUID）或 **按 URL 域名 + `priority` 自动匹配启用规则**；**`POST …/collect/batches`** **仍仅** **`provider.status===available` 且 `batchSupported`**（**custom `batchSupported=false`**）；**`source`** **大小写不敏感**；**URL** 仅 **`http`/`https`**。**Collector 即时失败码**：**`INVALID_REQUEST`/`INVALID_URL`/…** → **不进行自动退避重试**；**`COLLECT_FAILED`/`NAVIGATION_FAILED`** 等仍按 **`COLLECT_*` Retry**。
- **Collector HTTP 客户端**：`collector_client.go`：**`POST /v1/collect`** body 支持可选 **`options`**（**custom** 传 **`rule`/域名等**）；**`FetchProviders`** 不变；422/`ok:false` → **`CollectorRejectedError`**；成功 **`raw_result`** 写 **`NormalizedProduct` JSON**。
- **分层**：业务 Orchestration 在 **collect.Service**，采集解析仍在 **Node Collector**；Go **不写死** 1688 解析逻辑。

### 3.3 管理端（`admin/`）

- **@umijs/max**（脚本使用 **`max`**，禁止用 **`umi`** 跑 Max 配置，否则配置键会报错）。
- **登录与鉴权**：`/user/login` 调用 `POST /api/v1/auth/login`；**JWT** 存入 `localStorage`（`AUTH_TOKEN_KEY`）；**`request` 拦截器**自动附加 `Authorization: Bearer`；**HTTP 401**（除登录请求外）清 token 并 **整页跳转**登录页带 `redirect`；**`access.canAdmin`** 控制侧栏与业务路由；**`getInitialState`** 用 token 拉取 `/api/v1/auth/profile`。
- **布局**：右上角展示当前管理员与**退出**（`POST /auth/logout` + 清 token + 更新 initialState）。
- **Settings 与各分组页面**：`GET/PUT /api/v1/settings`，**`group_key`**：**`system`、`ai`、`storage`、`collector`、`security`、`image`、`inventory`、`pricing`、`mail`（**推荐**；加载时 **合并 legacy `email`**）**；Open Platform 推荐 **`/platform/settings/:platform`**；敏感项 **脱敏**。**`/settings/integrations`（第三方集成总览）**；**`/settings/inventory`**（**订单扣库存 + SKU 匹配策略**：**`auto_match_order_skus` / `auto_deduct_after_sku_match` / `auto_sync_inventory_after_order_deduct`（与旧 **`auto_sync_platform_inventory_after_deduct`** 读取合并、保存可双写）/ `allow_manual_sku_bind_after_deduct`** 等；**库存预警默认**：**`default_warning_stock` / `default_safety_stock` / `enable_inventory_alerts` / `alert_out_of_stock` / `alert_platform_stock_mismatch` / `platform_stock_mismatch_threshold`** — **保存不回写已有 SKU 阈值**，提示 **逐 SKU 或后续批量**；**批量同步**：**`inventory_sync_batch_max_size`**、**`inventory_sync_platform_rate_limit_enabled`**、**各平台 `inventory_sync_platform_rate_limit_per_minute_*`**）；**`/settings/pricing`**（**默认加价/尾数/平台覆盖**、**`batch_max_size`**；应用后**仅改本地 SKU `price`**）；**`/inventory/alerts`**、**`/inventory/sync-batches`**、**`/inventory/logs`** / **`/inventory/effects`**（审计列表）；**`/settings/platforms`**：**`GET /api/v1/platform/providers`** + **`GET …/platform/settings/:platform`（含 `schema`）** 动态表单 → **`PUT …`**；**`planned`** 平台提示可先保存配置。**AI / 图片 AI / 存储** 顶栏 **Alert** 强调 **自备密钥、前端不直连第三方**。**`test-email`**。**`image` / 存储** 详见 §3.2 / 上文存储段。
- **存储页保存策略**：按当前 `kind` 仅提交相关键（**local / s3compat / cos / oss** 各一套字段）。
- **操作日志页**：**`ProTable`** → **`GET /api/v1/operation-logs`**；只读、可筛选。
- **文件管理页**：**`ProTable`** → **`GET /api/v1/files`**；图片预览；删除 **`DELETE /api/v1/files/:id`**。
- **运维与失败聚合**：侧栏 **`运维`** — **`layouts/OpsGroupLayout`**：**`/workers/monitor`**（**`task_alert_scan`** 心跳实例）、**`/ops/task-center/failures`**（**分类列**：`failureCategory` / **`severity` Tag** / `suggestedAction` / `alertStatus`；**筛选** `failureCategory`·`severity`；**Drawer** 归因与 **生成告警 / 跳转告警**）、**`/ops/task-center/alerts`**（**ProTable**：**`notificationStatus`**、**通知记录 Drawer**、**手动通知（按 `taskcenter.notification_channels`）**、扫描 **`scanTaskAlerts`**、`handle`/`ignore`/取消标记、链回失败详情）；**`services/taskCenter.ts`** **`queryAlertNotifications` / `notifyTaskAlert`** 等。**设置 → 系统设置**（**`/settings/system`**）：**站内告警策略** + **告警扫描 Worker** 开关/间隔；**设置 → 告警通知配置** **`/settings/alert-notify`**（**`taskcenter` 出站 + `alert_notify` 通道**，**非 SMTP**）。**工作台** · **商品运营看板** **`/dashboard/product-operations`**（**`layouts/DashboardGroupLayout`**、**`services/dashboard.ts`**；**`/dashboard` 重定向**）、**`/workers/monitor`** 顶栏 **快照** → 失败中心。
- **开发代理**：`.umirc.ts` 将 **`/static`** 代理到后端，便于 **`public_base=/static`** 时预览。
- **商品草稿**：路由 **`/product/drafts`**，`ProTable` → **`GET /api/v1/products`**（**勾选批量发布检查 Drawer** → **`POST /products/readiness/batch`**；**「批量设置发布价」** → **`POST /products/pricing/batch-apply`**；**勾选批量 AI Drawer** → **`POST /api/v1/ai/batches/product-text` / `POST /api/v1/ai/batches/product-images`**，**`/ai/batches`** 批量详情 **重试失败 / 应用 AI 文案**）；**`/product/drafts/:id`** **Tabs**（基础、AI 标题/描述、**图片管理**（上传、`createProductImage`、**AI 图片任务**：**resize/noop**、**remove_background**、**replace_background（`openai_image` / `comfyui`）**、**generate_scene（openai_image / comfyui）**、Prompt/背景/style、**可无源场景图**、异步提示 + **`/ai/image-tasks`**、reorder）、**SKU 表**（**`costPrice`/`price` 列**、**「应用定价规则」** → **`PricingApplyModal`** / **`POST …/pricing/apply`**）、**库存**（**`warningStock`/`safetyStock`/`stockStatus`** 列；**设置预警线** Modal → **`PUT …/stock-settings`**；**低库存 / 缺货 / 平台不一致 Tag**；**`adjust-stock`**、**`inventory_sync`** **能力**；**TikTok Shop / Shopee / Lazada / Amazon** **`beta`** 可走真实 **`SyncInventory`**；Tooltip、**`sync-inventory`** Modal 引导 **平台开放/刊登配置** 与 **Partner / Open Platform 库存权限**；`/inventory/alerts`（批量同步批次）、`/inventory/sync-batches`、`/inventory/sync-tasks`（**`batchId` 筛选 / 失败批量重试**）；**`?tab=inventory`** 深链（库存 Tab **刊登映射勾选批量同步**）；**`services/inventory.ts`**、`services/products.ts` **stock-settings**）、**发布检查**（**Tab**、**`?tab=readiness`**、**`services/productReadiness.ts`**）、**刊登**（**已授权且 `capabilityStatus.product_publish` 为 `available`（mock）或 `beta`（TikTok Shop / Shopee / Lazada / Amazon）** 店铺、**投递前 **`getProductReadiness`**、**`POST /products/:id/publish`**（**失败时展示检查明细**）、缺配置引导 **设置 → 平台刊登配置 → Amazon**、本商品 **`GET /products/:id/publications` 快照**）、最近 AI 任务）；分组 **`/inventory/alerts`** **`/inventory/sync-batches`** **`/inventory/sync-tasks`**；全局 **`/product/publish-tasks`**（**`services/productPublish.ts`**）；**`/settings/platform-publish`**（**`GET/PUT /platform/publish-settings/:platform`**，与 **`/settings/platforms`** 应用配置分拆）；**`/ai/prompts`**、**`/ai/tasks`**、**`/ai/image-tasks`**、**`/ai/batches`**（约 **4s** 轮询、**`document.visibilityState` 隐藏时暂停**；**新建任务可选 `sourceImageId`**）。**`products.ts` / `platformPublish.ts` / `imageTasks.ts` / `inventory.ts`**、**`aiBatches.ts`** 封装 API。
- **采集**：侧栏分组 **采集**：**`/collect`** **采集中心**（**`available` / `beta`** 可申请单链接；**批量**仍受 **`batchSupported`**；**custom** 状态 **`beta`** 展示 **「基础可用」**（非「测试中」）；**custom** 批量 Tooltip **「已支持的平台请优先使用专用采集器；自定义链接批量采集暂未开放」**；其余 beta Tooltip **「测试阶段暂未开放批量」**；卡片 **「采集服务配置」/ planned「查看配置」** → **`/settings/collector?provider=*`**）；**`/settings/collector`** **Provider Segmented**（**1688 / 速卖通 / 自定义 / planned 预留**）：**通用**（**`main_service_url` / `collector_http_addr` / `goto_timeout_ms` / `headless`**）+ **1688 登录态与批量键**（**`collect_batch_*_1688`** 等，**仅 1688 区**）+ **custom**（**Profile / 规则入口 / `collect_custom_*`**，**不展示 1688 登录**；状态 **基础可用**）+ **aliexpress beta 说明 / `collect_aliexpress_*` 预留** + **planned 空状态**；**`/collect/rules`**（**采集规则**：列表 / CRUD / 启用停用 / **测试预览**）；**`/collect/tasks`**（**`source=custom`** 时 **规则下拉**（可选，未选则域名自动匹配提示）；**`ruleId`** 提交）；**`/collect/batches`**（仅 **`batchSupported` 且 status=`available`**）、**`/collect/monitor`**；**`services/collectRules.ts`** + **`collectProviders`** + **`collectTasks`** / **`collectBatches`** / **`collectMonitor`**。
- **内部订单**：路由分组 **`/orders`**（**`OrderGroupLayout`**）：**`/orders`** **订单列表** — **ProTable + Drawer**：头表表单（含可选 **`shopId`**）、**`platform` 筛选**、**外部单号列**、Tabs **明细行 / 发货 / 库存 / SKU 匹配**（扣减、回滚、影响表、**失败行 Alert → `/orders/exceptions?orderId=`**、**自动匹配 / 人工绑定 / 扣库+同步**、`/product-skus/search`、**未匹配/多候选 → 去工作台**）、**`/orders` 支持 `jumpOrder`** 快捷打开 Drawer；Modal **新建手工订单**：**扣库存 / 推平台队列**开关（跟 **`settings.inventory`**）；**`/orders/sync-tasks`** **同步任务表**；**`/orders/sku-matches`** **全局匹配审计**（**`queryOrderSkuMatches`**）；**`/orders/exceptions`** **订单异常工作台**（**`services/orderExceptions.ts`** · **`services/skuCandidates.ts`**：只读 **SKU** 候选、绑定抽屉候选区与「查看候选」Modal；深链 **`?orderId=`/`?exceptionType=`**）。**`/orders/sync-tasks`** **同步任务表**（**`services/orderSync.ts`** · **`services/orderInventory.ts`**）。
- **店铺管理（授权基座）**：路由 **`/shops`**；**`services/shops.ts`** + **`platformOpen.ts`**：**TikTok / Shopee / Lazada / Amazon OAuth**；默认 **不写每店开放平台密钥**，引导 **「设置 → 平台开放配置」**；**`formatPlatformPartnerErr`** 映射不完整 **`platform_*`** 配置；**创建店铺 / 授权 Drawer** 前校验 **`GET /api/v1/platform/settings/:platform`** 必填（**`****` 视同已配置密钥**）；生成授权链接在无配置时 **warning**。**同步订单**、**`/orders/sync-tasks`**。**`SHOP_*` / `PLATFORM_PROVIDER_STATUS`**（**`beta`** 前端显示 **测试中**）。
- **客服（AI + 平台同步）**：路由 **`/customer/conversations`**（**ProTable**：**`shopId`** 筛选、**拉取平台消息**）、**`/customer/conversations/:id`**（**店铺 / 外部会话 ID**、**回复区**：可 **仅手写**；**采纳为内部回复** **与** **发送到平台**（二次确认）；**AI 不自动外发**）、**`/customer/message-sync-tasks`**；**`/shops`** **拉取客服消息** / **客服同步记录**（依 **`capabilityStatus.customer_message`**）。**`services/customer.ts`**：**`syncCustomerMessages`**、任务查询/重试、**`sendPlatformMessage`**。
- **常量**：`src/constants/status.ts`（商品状态、**采集任务 / 批次**状态枚举、**订单与支付 / 履约 / 发货**、**订单同步任务**、**店铺与平台 Provider**）。

### 3.4 采集服务（`collector/`）

- **Playwright + TypeScript**，独立进程，**不直连主业务库**。
- **`CollectorProvider` 接口**（含 **`meta`**：名称、**`status`**、**`batchSupported`**、**`urlPatterns`**、**`features`**）+ **有序注册表**。**1688**：`available`，**结构化解析不变**；**batchMode** 时进程内 **`with1688BatchGate`** 串行兜底；**风控页** → **`PAGE_BLOCKED_OR_VERIFY_REQUIRED`**；**`TIMEOUT`/`PARSE_FAILED`/`fieldMissing`** 摘要。**AliExpress**：**`beta`**，**真实解析**，**`batchSupported=false`**。**拼多多**：**`pinduoduo`** **`available`**，**`collector/src/providers/sourcePinduoduo`**（**`wholesale_detail`（pifa 专用 DOM）** 为主；**`goods_detail` 移动端** → **`UNSUPPORTED_PINDUODUO_URL`**；**`WECHAT_AUTH_REQUIRED`/`APP_REDIRECT`/`LOGIN_REQUIRED`**；**pifa** 解析标题/价格区间/SKU/主图详情图/参数；**`partial_success` + warnings**；**Profile**（1280 桌面）；**`batchSupported=true`** + **`collect_pinduoduo_batch_*` 设置**；失败中心分类 + **`collect.pinduoduo.*` 操作日志**）。**自定义**：**`custom`** **`beta`**，**`collector/src/providers/sourceCustom`**（后端 **`options.rule`** + **域名校验**；CSS Selector；**JSON-LD / OG / Meta** fallback；**`raw.stateDigest`**；无 JS 注入）；**`batchSupported=false`**。**占位**：**`taobao` / `shein_temu`**，`collect` **`PROVIDER_NOT_IMPLEMENTED`**。**统一错误码**： **`COLLECT_FAILED`、`PAGE_BLOCKED_OR_VERIFY_REQUIRED`、`PROVIDER_*`、`INVALID_*` 等**，`runCollectTask` **前缀映射**。
- **任务编排**：`runCollectTask`（唯一 HTTP 编排入口）。
- **HTTP**：`GET /health`（契约不变）；**`GET /v1/providers`**（注册表 **`listProviderPublicMetas()`**）；`POST /v1/collect`（body：**`source`** + **`url`** + 可选 **`options`**；**custom** 必填后端下发的 **`options.rule`** / **`domain`** 等）。
- **浏览器**：`BrowserManager` 单例 Chromium，`withPage` 保证关闭 page/context。
- **与 Go 对接**：主 API **HTTP 同步调用**上述 **`POST /v1/collect`**；**NormalizedProduct JSON 契约未变**，`BuildImportSKU` 支持 **`properties` / `name` / `attrs`**；**pinduoduo** 有 SKU 时不合成默认规格；采集解析仅在 Collector。
- **1688 已知坑与防复发**：**[`docs/collector-1688-pitfalls.md`](collector-1688-pitfalls.md)**（`page.evaluate` / `unitWeight` 误价 / SKU 维度噪声 / mm 价表 / 回归命令）。
- **本地调试**：**`pnpm collect:test -- --url "https://detail.1688.com/offer/..."`**（仍为默认 **`source=1688`**）；或 **`pnpm collect:test -- --source aliexpress --url "https://www.aliexpress.com/item/100500....html"`**；环境变量 **`COLLECT_TEST_URL` / `COLLECT_TEST_SOURCE`**；根 **`pnpm collect:test`** **透传到** **`@trademind/collector`**。
### 3.5 文档

- **本文件**：`docs/PROGRESS.md`（进度与决策单一事实来源之一，与 `README` 互补）。
- **开源文档体系**：`README.md` 为中文 GitHub 首页，`README.en.md` 为结构一致的英文首页；明确 **Apache-2.0**、原项目地址、赞助与贡献入口；新增 `LICENSE`、`CONTRIBUTING.md`、Issue / PR 模板、`docs/development.md`、`docs/docker-deployment.md`、`docs/architecture.md`、`docs/provider.md`、`docs/roadmap.md`、`docs/sponsor.md` 与 `.github/FUNDING.yml`；README 预留 **合作商展示 / 贡献榜 / 赞助榜**。

---

## 4. 未完成事项（相对「地基」验收以外的路线图）

> 「未完成」聚焦 AI / 云存储 / 采集结构化深化及异步编排：地基阶段的条目已全部勾选。

### 4.1 后端

- [x] **认证**：`POST /api/v1/auth/login`、**JWT**、管理员模型、`profile` / `logout`
- [x] **Settings 业务**：`settings` 表与 `GET/PUT /api/v1/settings`、**AES-GCM（APP_MASTER_KEY）**、脱敏与 masked 更新语义
- [x] **迁移**：启动时 GORM **AutoMigrate**（地基表 + **商品 / 采集** + **`ai_prompts` / `ai_tasks` / `image_tasks` / 客服三表 / 刊登三表 / 库存两表**）
- [x] **操作日志**：表 + **`GET /api/v1/operation-logs`**；登录 / logout / settings / test-ai / test-storage / **采集关键节点 / 商品 CRUD / AI 标题与描述 / 图片任务 / 客服 MVP（`customer.*`）**
- [x] **对象存储与文件 API**：Storage **factory**（**`local` + `s3`/`r2`/`minio`（S3 兼容）+ `cos` + `oss`（独立 SDK）**）**Put / GetURL / Delete / Get**；**`POST /api/v1/files/upload`**（**`storage_kind` 入库**）；**`GET/DELETE /api/v1/files`**（删 **云端对象先于 DB**，按 **`files.storage_kind`**）；**`/static`** 仅 **本地** 只读
- [x] **settings 连通性测试**：`test-ai`、`test-storage`（**local·S3-compat·COS·OSS**；见 §3.2）
- [x] **商品草稿 API**：§3.2 **商品草稿**；**AI 标题与 AI 描述** 生成/应用 / 任务列表（见上）
- [x] **采集任务 API + Collector Client**：§3.2 **采集任务** / **Collector HTTP 客户端**
- [x] **AI 文本（标题+描述 + 客服建议）**：§3.2 **AI Provider / Prompt / ai_tasks**（含 **`customer_reply_generate`**、**`conversation_id`**、**`GET /ai/tasks?conversationId=`**）/ **商品 AI 标题与 AI 描述** / **AI 客服 MVP**（§3.2）
- [x] **AI 图片任务**：**remove.bg `image_file` + `image_url`** + **OpenAI Image `generate_scene` + `replace_background`（multipart `/images/edits`）** + **ComfyUI**（**`generate_scene` / 基础 `replace_background`**）+ **Redis Worker** + **自动退避重试**（详见 §3.2）

### 4.2 管理端

- [x] 登录页与 **access 模型**（@umijs/max access）；**Bearer** 请求拦截与 **401** 处理
- [x] **系统 / AI / 存储 / 采集服务 / 安全 / 图片 AI** 设置页与 **settings API**；**test-ai / test-storage**（**local · S3-compat · COS · OSS**）；**存储页 COS/OSS 独立表单 + `s3_*` + 本地**；**上传测试** **`/files/upload`**；**操作日志**与 **文件管理**（**ProTable**）
- [x] **商品草稿 / 采集任务（含批量采集 `/collect/batches`）**：分页列表 API、筛选、**单链接**与**批量**表单；失败 **重试** / 批次 **重试失败**
- [x] **Prompt 模板页（`/ai/prompts`）**；**商品详情编辑页（`/product/drafts/:id`）**：**Tabs**（基础表单、保留 **AI 标题/描述** 弹窗、**图片 ModalForm/Reorder** 与 **AI 图片任务入口**、**SKU `EditableProTable`**、最近 AI 任务）；**`/ai/tasks` 全局 AI 任务记录页**（**`conversationId` 列**，可见 **`customer_reply_generate`**）；**`/ai/image-tasks` 图片任务页**；**`/ai/batches` AI 批次页**（**列表 / 详情 / 子任务 / 重试 / 文案应用 `ai_title`/`ai_description`**）；**`/settings/image` 图片 AI 设置**（**`/settings/ai`** 含 **批量 AI 开关与上限并发**）；**`/customer/conversations` + `/customer/conversations/:id`** **客服工作台**

### 4.3 采集服务

- [x] 1688 **结构化解析落地（首版）**：主图 **`mainImages`**、详情 **`descriptionImages`**、**`attributes`**、**`skus`**（含 **`properties`/价格/库存/可选图**）；**降级不抛解析异常**（仅非法 URL、导航失败、非 offer 跳转、验证码页且全无结构化字段时 **`INVALID_URL`** 失败）。
- [ ] **反爬与稳定性深化**（人机验证绕过、SKU 多维长期可用、异步详情 iframe 全覆盖等）。
- [x] 与 **Go 任务编排**对接：**HTTP 异步队列**（Redis list + Worker **`POST /v1/collect`**），由 Go 写 **`collect_tasks`** 与 **`products`**（Collector **不写主库**）

### 4.4 跨模块

- [x] **Go ↔ Collector**：HTTP **`POST /v1/collect`**（Worker，`NormalizedProduct` 不变）；**`GET /v1/providers`** 元数据；422/`ok:false` → 任务 **`failed`/退避策略**（见 §3.2）。
- [x] e2e（本地）：提交合法 **1688 详情链接** → **结构化解析** → **草稿入库**（**主图/SKU 等完整性仍受站点与风控影响**）

---

## 5. 当前项目结构说明（高频路径）

```text
trademind-ai/
├── backend/                 # Go Gin 主 API
│   ├── cmd/server/          # 入口 main
│   └── internal/
│       ├── api/             # 路由
│       ├── config/          # 环境配置
│       ├── database/        # GORM
│       ├── rdb/             # Redis 客户端
│       ├── middleware/      # RequestID / Recovery / AccessLog
│       ├── pkg/             # response, id, ctxkey, model
│       ├── providers/       # **`storage`**（local / s3 / cos / oss 等）、**`ai`、`image`、`platform`**（**`tiktok`/`shopee` beta `OrderSyncProvider`**）
│       └── modules/         # auth、admin、settings、**operationlog**、**files**、**product**、**order**、**ordersync**、**collect**、**collectrule**、**aiprompt**、**aitask**、**imagetask**、**customerchat**、**taskcenter**
├── admin/                   # Ant Design Pro（Umi Max）
│   ├── .umirc.ts            # 含 proxy `/api` 与 **`/static`** → 8080
│   ├── config/routes.ts
│   └── src/
│       ├── pages/           # … **Collect/Hub**、**Collect/Rules**、**Collect/Tasks**、**Collect/Batches**、**Collect/Monitor** …
│       ├── services/        # … **`collectProviders`**、**`collectTasks`**、**`collectBatches`**、**`collectMonitor`**、**`imageTasks`** …
│       └── constants/       # 状态枚举
├── collector/               # Node 采集（Playwright）
│   └── src/
│       ├── browser/         # BrowserManager
│       ├── providers/       # `registry` + **source1688** + **sourcePinduoduo** + **sourceAliExpress** + **sourceCustom** + **stub/placeholders**（**meta**、`/v1/providers`）
│       ├── tasks/           # runCollectTask
│       ├── http/            # HTTP 服务
│       └── types/           # NormalizedProduct
├── docs/                    # 架构与进度文档
├── data/uploads/            # 本地存储目录（默认）
├── docker-compose.yml
├── pnpm-workspace.yaml
└── .env.example
```

---

## 6. 已确认技术决策（勿随意推翻）

| 领域 | 决策 |
|------|------|
| Monorepo 包管理 | **pnpm** workspace；根目录脚本统一入口 |
| 管理端 CLI | **@umijs/max** 使用 **`max` 命令**（dev/build/setup） |
| API 形态 | 后端 JSON **`{ code, message, data, traceId? }`**；`code===0` 成功 |
| 主键 | **UUID**（应用内生成；DB `char(36)`）；**`settings` 表主键为 BIGINT 自增**（与 SQL 草案一致） |
| 认证 / 系统配置 | **JWT（HS256）** + `Authorization: Bearer`；**settings** 敏感值 **AES-GCM（APP_MASTER_KEY）**，接口侧 **脱敏** |
| 采集 | **独立 Node 服务**；统一输出 **NormalizedProduct**；**必须保留 `raw`** |
| 安全 | 第三方密钥、Token **不进前端明文**；日志 **不打全量密钥** |
| 架构 | 平台/采集/AI/存储 **走 Provider 抽象**；TikTok 专有 HTTP **仅在 `providers/platform/tiktok`** |
| 主数据库 | **PostgreSQL** 为开发与 `docker-compose` 默认；仍支持 **`DB_DRIVER=mysql`** |
| 文件存储（MVP） | **上传到后端**；**object_key / public_url**；**factory**：**`local` / `s3` / `r2` / `minio`**（AWS SDK **S3 兼容**）+ **`cos`（COS SDK）+ `oss`（OSS SDK）**；**非公网可读 Bucket** 依赖 **`*_public_base`（CDN/静态站）或后续按需签名 URL** |
| AI 文本（扩展） | **标题/描述/客服建议** 均走 **`AI Gateway`** 与 **`ai_tasks`**；**`settings.ai.provider`**：**`openai` / `openai_compatible` / `deepseek` / `qwen`**（**`compatclient`** Chat Completions）；**`test-ai`** 经 Gateway；**`customer_reply_generate`** **不写入/不返回第三方 API Key**；**采纳建议**仅写入 **`customer_messages`（`role=agent`, `source=manual`）**，**不调用任何平台外发 API** |
| AI 图片 | **`internal/providers/image`**：**`noop`** + **`removebg`**（**`image_file` / `image_url`**）+ **`openaiimage`**（**`/images/generations` + `/images/edits` multipart `image[]`**）+ **`comfyui`**（HTTP REST，**结果统一 PNG**）；**factory**：**`noop` \| `removebg` \| `openai_image` \| `comfyui`**；**异步队列 + 自动退避重试**（**`IMAGE_AUTO_RETRY_*`**）；**源解析**：**`imagetask/source_resolver`** + **Storage `Get`**（**`local` / `s3` / `r2` / `minio` / `cos` / `oss`**，`NewFromPlainForStoredKind`）+ **`httppublic.IsPublicHTTPURL`**；**OpenAI Image** 密钥 **`settings.image.openai_image_*`（不回退 `settings.ai.api_key`）**；**ComfyUI `replace_background`**：**可配置 workflow 基础链路** |

## 7. 当前遗留问题 / 风险

1. **401 处理**：采用**整页跳转**登录以清空 initialState；后续可改为无刷新同步 `setInitialState`。
2. **`s3_presign_enabled` 入库 URL**：启用预签名时 **`files.url`** 为**短时有效链接**，过期后预览/外链失效；生产推荐配置稳定 **`s3_public_base`**（或后续做按需重签）。
3. **COS / OSS 外链可读性**：**`files.public_url`** 取自 **`GetURL`**；若 Bucket **非公共读**，缺省 **`*_public_base`（CDN/自定义域名绑定）时外链可能无法在浏览器匿名访问**；（与 **S3 预签名**类似，后续可增强 **COS/OSS 按需签名**。）
4. **静态访问**：生产环境需自行用 **反代 / CDN** 暴露 **`/static`** 或改写 **`public_base`**（**仅本地 `kind`**）；开发依赖 admin **`/static` 代理**或直连后端端口。
5. **1688 采集** 已升级为 **结构化首版**：多数商品页可从 DOM + JSON 抽到 **主图/详情图/属性/SKU**；**站点改版、登录/验证码/风控会导致字段缺失**，详情图若在 **跨域 iframe / 异步接口** 仍可能不完整；非生产 SLA。
6. **多实例 Worker / 编排观测**：**`collect_task_events`** 与 **任务租约 + `worker_instances` 心跳 + Reaper + `/health` `workers` + `/api/v1/workers/monitor` + 管理端 `/workers/monitor`** 已落地；极端网络下指标仍可加强。
7. **忘记密码未完成**：已在登录页占位，尚未实现后端逻辑。
8. **手机号注册/短信未完成**：注册仍仅限 **邮箱 + 验证码**；**登录**已支持邮箱或手机号（规范化数字，兼容 +86）；短信注册/找回未做。
9. **更多邮件服务商未完成**：当前仅完成了 SMTP 方式对接发送，尚未提供 Mailgun 等其它供应商实现。
10. **验证码风控可继续增强**：目前已做简单的时间窗与数量限制。
11. **历史管理员数据**：早期仅在内部 `username` 列有意义、未填 **`email` / `phone`** 的账号将无法按新规则登录；需在库里补齐邮箱或手机号，或清空表后重新 bootstrap。
12. **端口对齐**：**`COLLECTOR_BASE_URL`**（Go）必须与 **`COLLECTOR_HTTP_ADDR`**（Collector）监听端口一致（模板默认 **3100**）；`.env.example` 已备注。
13. **Admin 与 Backend / Collector**：本地需 **Go :8080 + Collector + Postgres**；admin dev 代理 **`/api`** → `8080`。
14. **Collector** 首次需 `pnpm install:collector:browsers`（Chromium）。
15. **MySQL 可选驱动**：当前 JSON 字段迁移以 **PostgreSQL `JSONB`** 为主路径；若使用 **MySQL**，需自行核对 GORM 对 `JSON`/`JSONB` 标签的兼容性（默认开发仍为 Postgres）。
16. ~~**`settings.ai.provider` 仅 openai_compatible**~~ → **（2026-05-19）** 已支持 **`openai` / `openai_compatible` / `deepseek` / `qwen`**；DeepSeek / Qwen 第一版为 **Chat Completions**（共享 **`internal/providers/ai/compatclient`**）；**多模态 / Embedding / Rerank / 多 Provider 配置表** 仍后置。
17. **AI 图片（边界）**：**ComfyUI `replace_background`** 依赖用户 **API 工作流**（**非通用 guarantee**）；**OpenAI `images/edits`** 受 **模型/额度/合规** 与 **源图格式** 约束。
18. **公网 URL 启发式**：**`httppublic.IsPublicHTTPURL`** 按 **scheme/host 字面** 排除 **loopback / RFC1918 / 链路本地** 等；**普通域名默认视为公网**（**不做 DNS**）。若 **`public_base` / 签名 URL 主机名为内网域名但字面非上述范围**，可能被误判为「公网」而走 **`image_url`**（remove.bg 仍不可拉取）；此时应依赖 **`source_image_id` + `Get`** 路径。
19. **AI 客服与内部订单上下文**：工作台已支持 **绑定内部订单**（含 **同步入库的 `orders`**）。**已落地** **平台客服消息同步**（**`customersync`**、**人工外发**、**mock** 全链路可测）；**TikTok**、**Shopee**、**Lazada** **`PullMessages`/`SendMessage` 已真实接入（`beta`**；**TikTok** 依赖 **`api_version`** 与 Customer Service 路径；**Shopee** 依赖 **`api/v2/sellerchat/*`**；**Lazada** 依赖 **`/im/session/list`** / **`/im/message/list`** / **`/im/message/send`**（IM **`template_id`** / **`content` JSON** 映射见 **`lazada/customer_message_mapping.go`**，需随文档校准）；**Amazon**：**SP-API Messaging API** **`PullMessages`/`SendMessage`**（**`beta`**；**不提供买家会话正文**，见 **`amazon/customer_messages.go`**）。**`GET /platform/providers`**：**TikTok / Shopee / Lazada / Amazon `customer_message`** 为 **`beta`**。仍无各平台 **实时消息 WebSocket 推送**。
20. **多 AI Provider 并存（未完成）**：当前仍为 **单一 `settings.ai` 默认配置**；**DeepSeek / Qwen 已独立 Provider 名**，但尚无 **`settings.ai_providers` 多配置表** 与按任务选模。
21. **1688 采集边界 / 反爬稳定性**：虽已 **DOM + script JSON 解析**，仍存在 **SKU 组合不全**、详情图异步、**`/offer` URL 误判**、**人机验证 / 风控**等边界；需在真实流量下持续补强选择与稳定性。**已踩坑清单**见 **`docs/collector-1688-pitfalls.md`**（`page.evaluate` 注入、`unitWeight` 误价、SKU 维度/mm 价表）。
22. **AliExpress（Collector `beta`）边界**：受 **人机验证 / 风控 / 多语言 PDP / 区域价与币种格式**影响，**SKU 映射对部分模板仍不完整**；详情若 **异步 / iframe**，**`descriptionImages` 可为空**，**候选见 `raw.detailImageCandidates`**。**批量链路未开放**（`batchSupported=false`）。
23. **拼多多（Collector `pinduoduo` `available`）边界**：**单链接 + 批量**已开放（批量默认 **并发 1**、**4–9s 随机间隔**）。优先 **`pifa.pinduoduo.com/goods/detail`**；**移动端商品页**返回 **`UNSUPPORTED_PINDUODUO_URL`**。支持 **标题 / 价格区间 / 主图 / 详情图 / 参数 / SKU 行价 / 库存（尽力）**；采集阶段可 **部分成功 + warning**，**发布前检查** 拦截无主图/无效价；**登录 / 微信 / 验证 / App 引导** 有明确错误码。**淘宝/天猫（`taobao_tmall`）** 为 **`beta`（测试中，单品采集）**；**SHEIN·Temu** 仍为 **`planned`**。**自定义链接**：Collector **`custom` 为 `beta`**，管理端展示 **「基础可用」**；**单链接采集**、**采集规则**、**AI 生成规则**、**规则测试**、**通用访问状态检测**、**登录 Profile**、**商品草稿**已支持；**边界**：**SKU / 库存 / 动态价格 / 图片质量**不保证完整；**批量采集暂未开放**；**建议**已支持平台优先专用采集器。**高级可视化规则编辑器**、**更强 SSRF（内网等）** 仍未完成。
24. **`ai_tasks` / AI 描述**：标题与描述生成均 **`running → success|failed`**；描述任务依赖模型输出 **合法 JSON**；失败写入 **`ai_tasks`** 与操作日志。
25. **TikTok Shop、Shopee、Lazada、Amazon（beta）订单同步**：均已接 **`providers/platform/*/SyncOrders`** → **`order.UpsertSyncedOrders`** / **`order_sync_tasks`**；**Lazada** 依赖正确的 **`api_base_url`**（各站点 **`/rest`**）与时间窗；**Amazon** 依赖 **LWA**、**IAM SigV4**（运行时凭证）与 **Orders v0** 分页（**NextToken**）；**429** 等限流错误需 **人工 retry**；生产需持续压测与策略调优。
26. **Amazon 商品刊登（beta）边界**：已接 **`providers/platform/amazon.PublishProduct`**（SP-API **Listings Items** `PUT /listings/2021-08-01/items/{sellerId}/{sku}`），复用 **`settings.platform_amazon`**、**`settings.platform_publish_amazon`**、**`shop_auth_tokens`**、**`product_publish_tasks`** 与 **product_publish Worker**，刊登结果写入 **`product_publications` / `product_publication_skus`**。**Product Type Definitions / Listings Items attributes** 仍需生产实测；首版依赖用户在 **`platform_publish_amazon.amazon_attributes`** 或任务 **options** 手工补齐平台必填字段，不伪造 Brand / Manufacturer / Browse Node / ASIN。
27. **Amazon Listings Items 库存 PATCH（beta）边界**：**`PATCH …/items/{sellerId}/{sku}`** 依赖有效 **`product_type`**、**`fulfillment_channel_code`** 与 **`marketplace_id`**（**刊登配置 / options**）；**部分账户 / 商品类型 / 多渠道库存** 行为需 **生产实测**；**429 / 5xx / 超时** 文案可含 **`retryable`**。**Feeds 批量库存** 未做。
28. **订单 SKU 匹配深化（相似度入库 / 映射导入仍为增强项）**：自动匹配仍为 **确定性 external_id / sku_code / 本地 sku_code**；**SKU 候选服务**已为人工提供「标题 token + 规格属性 + 历史绑定 + 刊登映射」排序提示，但 **不自动写回订单行**；**多仓库存** 未做（**库存预警基础版** 已于 **2026-05-17** 落地，见 **§3.2** **库存同步** 条）。
29. **Amazon FBA / 财务 / 结算未完成**。
30. **Amazon 真实客服消息 API**：已接入 **`providers/platform/amazon`**（**Messaging API `beta`**）：**Orders + `getMessagingActionsForOrder` / `GetAttributes` / 模板 `POST`**；**买家会话正文 SP-API 不可读**，同步内容为 **模板摘要 + 免责声明**；**生产权限 / 模板可用性 / 限流**需持续实测。**订单同步不受影响**。**TikTok / Shopee / Lazada** 仍为 **`beta`**（IM 路径）。另：**Buyer-Seller Messaging** **权限 / 模板可用性 / 合规场景**需 **生产实测**（当前 **`PullMessages`** 仅为 **Orders + 模板能力摘要**，**非买家会话正文**）。**Messaging / Orders**：**客户端节流**、**`429`** 与 **HAL / JSON** 边界需持续优化。
31. **TikTok 商品刊登（beta）边界**：依赖 **`api_version`** 与 Partner **类目必填属性**（**`product_attributes` / SKU `sales_attributes`** 需用户在 **`attrs` JSON** 或 **`options`** 中提供）；**字段映射随 TikTok 文档升级需校准**；**多 SKU** 无 **`sales_attributes`** 时会拒绝刊登。
32. **TikTok 库存同步（beta）边界**：**`POST …/products/{product_id}/inventory/update`** 与 **Partner 文档**需随版本校准；**多仓 / `location_id`** 等扩展未强制；**429 / 5xx / 超时** 错误文案带 **`retryable`** 提示由运营 **人工 retry**。
33. **Shopee 库存同步（beta）边界**：**`v2.product.update_stock`** 与 **`stock_list`** / **`failure_list`** 字段随 **Open Platform** 演进需校准；无变体与多规格 **`model_id`** 依赖 **`product_publication_skus`** 映射；**按仓库存（`seller_stock`）** 依赖用户在 **`platform_publish_shopee`** 或任务 **options** 提供 **`location_id`**；**429 / 5xx / 超时** 走 **`retryable`** 提示与 **人工 retry**。
34. **Lazada 商品刊登（beta）边界**：依赖 **`settings.platform_publish_lazada`**（**`default_category_id` / 包裹重量·长宽高 / `delivery_option` / `default_brand_id`（写入 `brand`）** 等）与 **`attrs` + `options.lazada_attributes`** 填报类目必填属性；**`/image/migrate`**（公网图）+ **`/image/upload`**（Storage **Get**）；**`payload` JSON** **`Request.Product`** 随 **Open Platform** 演进需校准；**`publish_as_draft`** 仅落摘要，**不强改平台字段**。
35. **Shopee 商品刊登（beta）边界**：依赖 **`settings.platform_publish_shopee`**（**`default_category_id` / `logistic_channel_id` / `default_weight`** 等）与 **`shop_auth_tokens`**；类目属性由 **`shopee_attribute_list`**（**`attrs` / `options`**）手工提供；多 SKU 默认单维度 **`Variation`**，多维规格用 **`options.shopee_tier_variation`**；**`media_space/upload_image` · `add_item` · `init_tier_variation`** 字段映射随文档升级需校准。
36. **Lazada 库存同步（beta）边界**：**`/product/price_quantity/update`** 与 **`/product/item/get`**、`payload` **JSON Schema**（**camelCase/PascalCase**）随 **Open Platform** 演进需校准；**`SellerSku`** 取自 **`sku_code`** 或由 **item get** 反查；按仓 **`WarehouseQty`** 依赖 **`platform_publish_lazada`** 或 **`options`** 的 **`warehouse_id`（WarehouseCode）**；**429 / 5xx / 超时** 文案带 **`retryable`**，由 **人工 retry**。
37. **（已落地，见 §3.2 / 头部日期）批量设置库存预警线**：**`batch-preview`/`batch-update`** 与 **`/inventory/alerts`**、草稿 **「库存」Tab** UI；**不改** **`stock`**、**不写** **`inventory_change_logs`**、**不创建** **`inventory_sync_tasks`**。**仍不做**：按 **Excel/导入模板** 批量（仅筛选 + 多选）。
38. **自动补货建议（未完成）**：MVP **不做**；仅 **提醒**，无采购/补货编排。
39. **多仓库存 / WMS（未完成）**：单仓 **本地 `stock` + 平台镜像** 模型；**多仓**、库区、调拨未做。
40. **采购入库（未完成）**：无 **ASN/采购单/入库单** 与库存流水类型的 **采购入账** 链路。
41. **库存预测（未完成）**：无 **销量预测 / 安全库存自动计算**。
42. **库存预警列表查询（MySQL 边界）**：**`GET /api/v1/inventory/alerts`** 在 **`DB_DRIVER=postgres`** 下用 **`DISTINCT ON (publication_sku_id)`** 取 **每刊登 SKU 最近一条** **`inventory_sync_tasks`**；**`mysql`** 若未单独适配需 **等价子查询** 或 **应用层去重**（仓库默认路径仍为 **PostgreSQL**，见 **`.cursor/rules/11-local-dev-postgres.mdc`**）。
43. **统一失败任务中心 — 全文搜索（未完成）**：初版 **`keyword`** 仍为 **粗筛**（各表 LIKE / 单列）；仍无 **跨类型全文索引** 与 **统一搜索语法**。
44. **统一失败任务中心 — AI 智能归因（未完成）**：首版已实现 **规则 `failure_category` / `severity` / `suggestedAction`**（**`failureclassifier`**）；未接 **LLM 根因聚类**（**不写全量原始 error** 约束仍适用）。
45. **统一失败任务中心 — 失败趋势图与归因大盘（未完成）**：虽已支持 **`failureCategory`/`severity`** 筛选与 **`task_alerts`**，尚无 **按时间维度的趋势图**，**`summary` / 大盘聚合**仍为 **抓取窗口近似**.
46. **告警 — 外部通知自动重试（未完成）**：**`task_alert_notifications`** 按渠道**单次**投递，无 **退避重试队列**。
47. **告警 — 飞书真实发送（未完成）**：配置可保存；出站仍为 **`planned` / `skipped`**（**不向飞书生产 POST**）。
48. **告警 — 企业微信真实发送（未完成）**：同上 **planned**。
49. **告警 — 趋势图 / 统计大屏（未完成）**：无 **按时间维度的告警漏斗 / 报表**。
50. **告警 — AI 智能归因（未完成）**：仍为 **`failureclassifier` 规则**；未接 **LLM 聚类**。
51. **批量库存同步 — 平台真实批量 Feed/API（未完成）**：当前 Worker 仍为 **逐 SKU `inventory_sync_tasks` → Provider**；未接 **Amazon Feeds** 等原生批量库存通路。
52. **批量库存同步 — 延迟队列与精确限流（未完成）**：**`settings.inventory` 每分钟阈值**已落地；实现为 **简化 defer/requeue**；尚无 **Redis ZSET 延迟队列** / **令牌桶级**全局协同。
53. ~~**批量 AI 商品运营（未完成）**~~ → **（2026-05-18，MVP）** 已交付 **`ai_operation_batches`** + **`/api/v1/ai/batches/*`**；**`/ai/batches`** 与草稿 **批量 AI**；详见 **§3.2「批量 AI 商品运营」** 与正文 **头部最后更新**。仍无 **跨模块营销编排**。
54. ~~**商品运营看板 / 待办中心（未完成）**~~ → **（2026-05-18）** 已交付 **`/api/v1/dashboard/product-operations`** + 管理端 **`/dashboard/product-operations`**（**只读、无平台 HTTP、无任务副作用**）；**PROGRESS** 头部·§3·§8·变更记录
55. ~~**订单异常工作台（未完成）**~~ → **（2026-05-18）** 已交付 **`order_exception_marks`** + **`internal/modules/orderexception`** + **`GET|POST|DELETE /api/v1/orders/exceptions*`** + 管理端 **`/orders/exceptions`** + 看板 **`summary`/待办** + 订单 Drawer **库存/SKU 匹配** 联动；**标记不改变业务任务状态**；详见 **§3.2 / 头部最后更新**
56. ~~**SKU 候选推荐（未完成）**~~ → **（2026-05-18）** 已交付 **`internal/modules/skucandidate`** + **`GET|POST …/sku-candidates(*)`** + 管理端异常抽屉 / 订单 SKU 匹配 Tab；**只读、不自动绑定、不自动扣库、不自动库存同步**；详见 **头部最后更新** 与 **§3.2 内部订单**。
57. ~~**标题 / 属性相似度 SKU 推荐（未完成）**~~ → **（2026-05-18）** 首版已并入 **SKU 候选服务**（**Jaccard + 属性键值**），仍以 **人工确认** 为唯一落库路径；**不标题模糊自动绑定**。
58. **售后 / 退款异常工作台（未完成）**：当前 **`orderexception`** 聚焦 **SKU / 库存 / 同步**；退换货单独迭代。
59. **多仓库存（未完成）**：单 **`product_skus.stock`** 模型未扩展 **多仓**。
60. **租户级 settings 生效缺口（R102 P2 遗留，产品决策；R104 部分收口）**：口径已定——**面向业务/租户体验的设置租户化，部署/基础设施级设置保留平台级**。R104 已落地 **ai**（整组回退：租户配置自有 API Key 则整组用租户配置，否则整组回退平台默认）与 **collector**（逐 key 合并：租户覆盖单项、其余继承平台默认），经 `internal/pkg/tenantsettings` 统一解析。**剩余待租户化组**（按同口径单列迭代）：`image`（AI 图片 provider 凭据/参数，26 处）、`inventory`（库存策略，9 处）、`pricing` 残留 4 处（selection/procurement 已按租户读）、`taskcenter`+`alert_notify`（告警通知，3 处）、`sourcing`（1 处）。**保留平台级**：`storage`（部署存储桶/本地根路径）、`mail/email`（SMTP）、`system`、`platform_*`（开放平台应用凭据；店铺级凭据已按租户存 `shop_auth_tokens`）、`/ops/*` 备份容灾。

## 8. 下一步开发计划（建议顺序）

**全项目阶段（2026-07-07）**：**F1–F9 ✅** · **Phase F9 Passed** · **Phase H1.1 Post-F9 Enhancement** · **Tag deferred**。策略见 [`FULL_PROJECT_DEVELOPMENT_PLAN.md`](FULL_PROJECT_DEVELOPMENT_PLAN.md)。

**Post-F9 当前排期**

1. **H1.0 发布状态收口**：README / 计划 / 冻结规则 / changelog 与 Tag deferred 策略一致。
2. **H1.1 工作台状态保持**：优先覆盖 Dashboard、AI 工作台、失败任务中心。
3. **P2 评审**：tag deferred 期间逐条判断 P2 是否升为 P1，P3 继续 deferred。

**生产决策前仍不做**：真实生产灰度、Production Ready 标记、自动直接上架、无凭证伪造抖店 E2E 通过。

与文件顶部 **《当前产品路线》§四** 对齐；**不新增完整 ERP 高级模块**。

**体验收口（F8 内 P1，原排期 1–4 部分并入）**

1. **多平台跨境 ERP MVP 回归**：订单/库存/客服 Demo 样本 + smoke 脚本联动 **`taskcenter`**。
2. **AI 商品运营工具体验**：批量 AI、草稿全链路、关键路径提示。
3. **错误提示 / 空状态 / 引导文案**：`demo-empty-state-scan` 持续通过。
4. **演示包**：16 步主链路无凭证可走查（抖店步骤 mock / `local_draft_only`）。

**后续迭代（非当前「完整 ERP」排期；按需穿插，不阻断 1–4）**

- **Amazon Product Type Definitions / 刊登字段配置增强**（**PTD**、`amazon_attributes` 模板化，与 **Listings Items** 实测对齐）
- **Amazon SP-API / 各平台** 限流与部分失败策略在 **`taskcenter` / Worker** 上持续调优（**不引入过重编排**）
- **Collector** 反爬与稳定性增强（**自定义规则**与 **AliExpress beta**）
- **通知重试队列**（**Webhook/邮件** 出站退避；飞书/企微真实发送仍属 **§7 暂缓** / 《当前产品路线》**§三** 后置项）
- **售后 / 退款基础能力**：属 **完整 ERP 增强**方向，**不纳入**当前双主线 MVP 收口；若单列迭代须单独立项

（**完整 ERP 增强**清单见文件顶部 **《当前产品路线》§三**；规则性阶段划分仍可参考 **`.cursor/rules/09-dev-workflow.mdc`**。）

---

## 9. Cursor 后续开发注意事项

1. **必读**：`docs/PROGRESS.md`（本文件）、`.cursorrules`、`.cursor/rules/*` 中与本任务相关的规则。
2. **开工前**：对照「**已完成 / 未完成**」，确认是否已有接口或占位，**避免重复实现**。
3. **改架构前**：核对「**已确认技术决策**」；若需变更，在本文件与相关架构文档中**写明原因与日期**。
4. **收工后**：若完成一整块功能或一次较大重构，**必须**更新本文件：勾选进度、补充遗留问题、调整「下一步」。
5. **前端**：继续 **`services/` 统一请求**；表格 **`ProTable`**、表单 **`ProForm`**；敏感字段 **脱敏**。
6. **后端**：Handler 薄、Service 编排、**外部调用带超时**；**采集 / 图片 / 订单同步 / 客服消息同步 / 商品刊登 / 库存同步**均为 **Redis BRPOP + DB 租约 CAS + 续约 + Reaper**（队列关闭 **`ORDER_SYNC_*` / `CUSTOMER_MESSAGE_SYNC_*` / `PRODUCT_PUBLISH_*` / `INVENTORY_SYNC_*`** 时对应任务可在请求内 **`inline`** 同步执行）（**可观测指标/告警**仍可加强）。
7. **采集**：新业务逻辑放在 **`collector` 对应 Provider**，**不要**塞进 Go 核心业务层。
8. **本地数据库**：遵守 **`.cursor/rules/11-local-dev-postgres.mdc`**，默认 **PostgreSQL**；勿默认生成 MySQL 专用迁移/compose。

---

## 变更记录（简短）

| 日期 | 说明 |
|------|------|
| 2026-08-06 | **R130 线1：R122 遗留性能 P2 聚合下推**：财务对账/利润报表由应用层全量拉取聚合改为 SQL GROUP BY 下推——finance `computeOrderFinance` 回款/费用/采购实付/订单行改按 (order, currency)/(order, product, sku) 分组求和（`chunkOrderIDs` 1000/批），参考成本按 (product, sku) 去重对解析（口径不变：多币种折算、租户本位币、estimated/actual 毛利、缺实付行数、结算状态逐项与治理前 SQL 对照一致）；reports `ProfitReport` 三维度改为 orders/order_items 分组聚合 + 窗口函数 first-seen（order 维度仅装载前 500 行明细，排序补 `id DESC` 决定性 tiebreak）；`expensePartsByGroup` 分组下推。API contract 不变。PERF seed（1 万订单/2 万行）p50：profit order 366→130ms、shop 303→156ms、product 353→253ms、对账 428→378ms、对账报表 542→448ms；治理前后 JSON/CSV 数值逐项一致（仅未匹配商品标签的既有平票非确定性）|
| 2026-08-05 | **R124 线2 P2 收口**：①响应式断点口径固化——<768px 移动模式仅底部导航（`:has(.tm-mobile-tabbar)` 隐藏 ProLayout 抽屉汉堡，二者互斥），≥768px 平板/桌面仅侧栏；移动「我的」页补经营报表/异常工作台/告警中心入口（汉堡隐藏后的补偿入口）；新增 `round124-p2.spec.ts`（375/767/768/769/1440 五档互斥断言）。②采购单详情「来源销售订单」由 UUID 前缀改为真实订单号：后端详情接口同租户回填 items `salesOrderNo`（gorm:"-" 展示字段，向后兼容），前端概览+明细行展示订单号并保留跳转（缺号回退短 ID），Go/E2E 回归补齐；`docs/api.md` 同步。③R119–R122 报表/工作台空/加载/错误态巡检：财务回款/对账/报表、买家消息、选品数据面板/对比、异常工作台均已具备 Skeleton/EmptyState/错误 Alert+重试，无需修改 |
| 2026-08-05 | **R122 线1 性能收口 v2**：新增 `cmd/seedperf` + `internal/modules/perfseed` 万级压测种子（`PERF-` 前缀，1 万订单/5 千采购/3 万库存流水/2 万自动化日志/1.2 万回款/2 万费用，独立于 DEMO- seed，幂等、clean/verify 零残留、production 拒绝；`pnpm seed:perf(:clean/:verify)`）；慢查询治理：`/orders/exceptions` 聚合排序 O(n²) 插入排序改 `sort.SliceStable`（万级异常行 p50 ~2373ms → ~278ms）、`migrate_round122` 补三个复合索引（orders 租户+支付状态+时间、order_automation_logs / inventory_change_logs 租户+时间，日志深分页 p500 ~31ms → ~7ms）；自动化规则引擎新增集成压测 `automation_engine_load_test.go`（10k order_created：首轮 ~74 events/s、重放幂等 ~404 events/s、dedup 零重复，默认 500 条快跑、`PERF_AUTOMATION_ORDERS` 放大）；前端万级数据下核心列表/报表页加载 1.2–2.2s、分页正常，Lighthouse 登录页无回退。财务对账/报表与利润报表全量装载聚合列为 P2 |
| 2026-08-05 | **R114 审单规则**：`order_review_rules`/`order_review_hits` AutoMigrate；订单 `reviewStatus`；创建动线跑规则引擎（首个命中定动作、全部命中落快照）；审单工作台 `/orders/review`（单个/批量放行、拒绝入取消动线、命中原因可见）；待审/挂起后端强制阻断采购单与发货；管理页 `/settings/order-review-rules`（增删改/排序/启停/dry-run）；demoseed R114 规则与命中样本 clean/verify 零残留；contracts 66 端点、permmatrix +8、后端/前端/E2E 测试补齐；`docs/api.md`、`docs/module-map.md` 同步 |
| 2026-08-04 | **R107 P2 收口（R106 复检后续）**：前端 `AUTH_STATE_UNAVAILABLE` 专门处理（不登出、「服务暂时不可用，正在自动重试」提示 + 指数退避自动重试，恢复后无感续用，与 401 重登守卫区分；覆盖 umi 拦截器 / fetch 守卫 / 启动期 profile 三链路）；UX v5 P2 收口（AI 未配置统一 banner 落地新手入门/采集/AI 工作台、新手入门第 1 步按设置权限降级、`GET /healthz` 别名）；v11 P2 复核（生产写闸门补 `real_credentials_forbidden`/`unsupported_adapter_mode` 中文文案，告警取消标记租户隔离回归通过） |
| 2026-08-04 | **R106 季度生产复检（部署/升级/语义抽查）**：从零 production+Caddy 部署（全新 VM）275 秒、HTTPS 登录累计 <7 分钟，launch-checklist 14 项验证全过；旧版本（R101/#214）→ 新版本升级演练通过（存量 collect_rules 归 tenant 0、task_alerts 按来源租户回填、租户 settings 归属正确、业务无回退、迁移幂等）；语义抽查通过（tenant0 写闸门、legacy token 拒绝、ORIGIN_NOT_ALLOWED、业务租户 /ops 403、备份 verify/download SHA-256 一致、恢复演练）。修复 P1：secure_session 下 `ValidateSessionAccess` 将数据库瞬断误报为 `AUTH_SESSION_REVOKED`（前端强制登出），改为 30s last-known-good 快照 + `AUTH_STATE_UNAVAILABLE` fail-closed（与 R103 legacy 路径同口径，补 Go 回归）；文档纠偏：恢复命令 `docker compose exec -T` stdin 损坏二进制流改 `docker exec -i`、`--pre-upgrade-check` 非 root 需 `BACKUP_DIR` 覆盖、env.md 补 fail-closed 语义 |
| 2026-08-04 | **R103 安全 P2 收口（#216 后续）**：`ai_prompts` 读收紧为平台租户专属（矩阵 7 条 forbid/platformAdmin，前端 `/ai/prompts` 与 `/ops/备份/恢复/发布/容灾` 入 `PLATFORM_ADMIN_ROUTES`）；`EnsureAccountActive`/`EnsureTenantActive` 由 fail-open 改为 **30s last-known-good 缓存 + fail-closed**（新增 `AUTH_STATE_UNAVAILABLE`，补 Go 回归）；注册验证码 SMTP 反枚举与 #215 配置引导融合（未配置 SMTP 统一 503 引导、发送失败对未注册地址返回与成功一致的 200）；upgrade-guide 增补 R102 采集规则/profile `tenant_id` 回填指引 + 预检 SQL 与业务租户 `/ops/*` 替代口径；租户级 settings 生效缺口列为遗留 §7-60 |
| 2026-08-04 | **UX 视觉复核 v4（R76 后三批新页面）**：物流（物流商/批量发货弹窗/打印拣货单）、迁移导入向导（上传→映射→校验→结果/历史）、多币种（汇率设置/报表折算）三视口全走查 + v3 闭环回归无回退；修复 P1×2：报表折算合计卡「含未折算币种」改「不含未折算币种」（与后端未折算不计入口径一致）、打印单「平台」接入 `platformLabel` 中文映射；记录 P1-3：demo 全量环境运营任务中心开箱 403（`operationtask` handler 拒绝 tenant 0 而 demo 落 tenant 0，需后端收口）；报告归档 `docs/ux-review/UX_REVIEW_V4_REPORT.md` |
| 2026-08-04 | **R94 QA 修复（P1）毛利估算与报表汇率口径统一**：订单成本/毛利估算（`/procurement/cost-estimates`）非 CNY 订单折算改为优先使用报表手工汇率表（`report_currency`，CNY→币种 = rate(CNY→本位币)/rate(币种→本位币)），与销售报表同一口径；报表表无该币种时回退原 `settings.pricing.exchangeRate` 单一汇率（行为兼容）。此前两处汇率来源独立，`default_exchange_rate=1` 时 USD 订单毛利被按 1:1 折算严重失真。补 Go 回归测试（优先级 + 回退）。另修复报表币种设置页「添加币种汇率」首次点击偶发不加行：加载完成时 `setFieldsValue` 会重置 rates 覆盖提前新增的行，改为加载期间禁用添加/删除按钮（初始 loading=true），补 Playwright 回归 `round94-report-currency-add-row.spec.ts` |
| 2026-08-03 | **R76 P2×4 收口**：选品候选详情状态/审核列改共享 `StatusTag` 中文映射（`COMMON_STATUS_LABEL` 补 `scored`）；草稿列表「来源」列 `collect` 中文化（`PRODUCT_SOURCE_LABEL` 补 `collect: 采集`）；demo seed 客服会话 Bob/Carol 样本改挂 operator/readonly 已授权的 DEMO 手工渠道店（Alice 保持抖店），两角色客服页开箱非空、幂等/clean 零残留；修复刊登「子任务」tab 与批次详情数据源不一致根因——`product_publish_tasks` 创建时未写 `tenant_id`（恒为 0）而 `ListTasks` 按租户过滤（`ADMIN_BOOTSTRAP_TENANT_ID=1` 下列表恒空）、批次详情按 `batch_id` 查询不受影响；三处任务创建补租户 + `migrate_round76` 按 products 回填存量，补 Go 回归测试 |
| 2026-08-03 | **R74 发布/刊登链路非空态回归**：全栈 docker compose + demo seed 实测 readiness/刊登草稿/运营任务降级执行/发布总览/三角色 RBAC/响应式，硬指标（console error/panic/5xx/42703）全零；修复 P1：`/product/publish-tasks` 刊登批次表挂载时误清 URL `tab` 参数导致「子任务」Tab 切换/刷新后回跳批次视图（两个 ProTable 的 URL 同步按 activeTab 守卫），子任务列表空状态改用独立 `publishTaskRecords` 文案；新增 `round74-publish-tasks-tab.spec.ts` 回归；P2 遗留：demo seed 无平台刊登能力预设致 publications/批次创建非空态不可达、缺 ≥2 同时待审任务致批量驳回不可测 |
| 2026-08-02 | **R60 发布/运营任务链路复查**：全栈 docker compose + demo seed 实测发布链路（含 #98/#99 回归、三角色 RBAC、375/768/1440 响应式），未发现 P0/P1；修复 P2：执行尝试记录改为回显实际生效 adapterMode（`publicAdapterModeForPort`）、readonly 运营总览隐藏写向快捷入口；根 workspace 固定 react/react-dom 18.2.0 使 `pnpm test:frontend` 恢复通过；已知 operator 运营任务读扩权待后续（任务缺 shop 维度） |
| 2026-08-02 | **1688 采集链路首次真实实测（Round 51）**：修复采集任务创建未写入 `tenant_id` 导致 Worker 全部拒绝（`任务缺少租户上下文`）的 P0 缺陷（单条 + 批量）；collector 新增 `COLLECTOR_PROXY_SERVER/_USERNAME/_PASSWORD/_BYPASS` 代理配置项（仅配置，不内置代理）；UA 默认按 bundled Chromium 主版本自动生成；风控/验证早期拦截路径补失败快照与 `[1688-collect]` 调试日志；compose 挂载 `./data/snapshots`。实测 6 条真实 1688 链接 0/6 成功、100% 风控跳转登录/验证页，失败上报（`PAGE_BLOCKED_OR_VERIFY_REQUIRED`/「页面需要验证」）准确；报告见 Round 51 实测报告 |
| 2026-08-02 | **R60 发布/运营任务链路复查**：全栈 docker compose + demo seed 实测发布链路（含 #98/#99 回归、三角色 RBAC、375/768/1440 响应式），未发现 P0/P1；修复 P2：执行尝试记录改为回显实际生效 adapterMode（`publicAdapterModeForPort`）、readonly 运营总览隐藏写向快捷入口；根 workspace 固定 react/react-dom 18.2.0 使 `pnpm test:frontend` 恢复通过；已知 operator 运营任务读扩权待后续（任务缺 shop 维度） |
| 2026-07-11 | **Phase P4.2 全量租户 Worker 与安全 Worker 收口**：`tasktenant` 接入 7 类生产 Worker；`security_secret_reencrypt` + `file_security_scan`；`migrate_p4_2`；`secret_targets`；安全中心 UI 九区块；11 份 `P4_2_*` 文档；`scripts/p4-2-security-final-closure-check.mjs`；IDOR 22 / shop scope 5 自动化；race deferred_on_windows |
| 2026-07-11 | **P2.2 文档与扫描收口**：AI apply/undo、Webhook HTTP/签名、Worker 租约矩阵、race 占位报告；`p2-2-reliability-closure-check.mjs`；更新 IDEMPOTENCY / TASK_RELIABILITY / P2.1 矩阵 / CHANGELOG / README |
| 2026-07-11 | **P2.2 AI text/image apply+undo 幂等**：`AITextApply/Undo`、`AIImageApply/Undo` key；apply/undo Acquire/Complete；版本冲突码；生成写回 `status=running` 守卫；set_main undo 恢复 previousBestMain；并发测试通过 |
| 2026-07-11 | **P2.2 Webhook HTTP 接收地基**：公开 `POST /api/v1/webhooks/:platform/:eventType`；签名验签抽象 + `internal-test` HMAC；时钟偏差/体限制；幂等 Ingest → `queued`；DB 轮询 Worker；配置 `WEBHOOK_*`；`go test ./internal/modules/webhook/...` 通过 |
| 2026-07-11 | **Phase P2.2 tasklease 扩展**：`TryClaimPendingOrRetrying`；collect / imagetask / customersync Worker 租约写回守卫；productpublish `finishProductPublishTask`；`migrate_p2_2`；taskcenter lease meta；stale worker 单测通过 |
| 2026-07-10 | **Phase P2.1 领域幂等与任务心跳租约**：关键写路径接入 `idempotency.Service`；`tasklease` + P2.1 迁移；扫描脚本与接入/并发安全文档 |
| 2026-06-27 | **Phase R1.2 真实预发部署与 Demo Tag 确认（部分）**：构建/smoke/Demo 数据/浏览器点检通过；Docker 不可用 + 无预发 SSH，HTTPS/Storage/备份/tag 仍 pending |
| 2026-06-27 | **Phase R1.1 MVP Demo 预发部署与人工走查**：本地 dev 等价环境 `go test`/`build`/`pnpm build:admin` 通过；路由 smoke + Demo 数据复跑；12 步 Demo 走查与 1366/1024 分辨率验收；`DEPLOYMENT_PRECHECK` 备份记录；Git tag **Tag deferred**（待真实预发 Nginx/HTTPS） |
| 2026-06-13 | **管理端文案与 UI 规范收口**：`TmPageContainer` 全站替换；`copywriting` / `layoutTokens` / `errorMessages` + `components/ui`；任务/刊登/库存/店铺/采集/AI 页技术信息折叠；`docs/ai-workflow.md` 补充典型模式；全局英文表头扫尾；`pnpm build:admin` 通过 |
| 2026-06-07 | **抖店 MVP Demo Release 收口**：新增 [`docs/DOUYIN_E2E_CHECKLIST.md`](DOUYIN_E2E_CHECKLIST.md)（18 步 E2E + 安全 + 风险 + tag 准备）；`DEMO_CHECKLIST.md` 增加「抖店完整演示流程」；`PROGRESS` 标记 Phase 1–9.2 主链路完成；管理端补抖店前置/订单同步/库存同步/失败中心/异常工作台小白提示；抖店平台配置描述更新；建议 tag **`v0.8.0-douyin-mvp-demo`** |
| 2026-06-06 | **淘宝/天猫采集器生产可用收口**：状态 **beta → available（已可用）**；开放 **批量采集**（默认每批 20 条、并发 1、间隔 3500–6000ms、重试 2）；批量前置登录/验证检查；无效链接跳过；**partial_success**；登录/验证失败可暂停批次；设置页 **collect_taobao_tmall_batch_***；操作日志 **collect.taobao_tmall.batch.***；README / DEMO_CHECKLIST 同步 |
| 2026-05-29 | **修复 AI 设置保存清空其他服务商密钥**：保存时仅 PUT 当前 provider 连接字段 + 全局参数；隐藏字段保留切换状态；后端对已存在加密项忽略空字符串提交 |
| 2026-05-29 | **AI 文本设置按服务商独立密钥**：`settings.ai` 新增 **`{provider}_api_key` / `{provider}_base_url` / `{provider}_model`**（openai、openai_compatible、deepseek、qwen）；管理端切换卡片自动带出对应配置；启动时 **`EnsureAIProviderDefaults`** 将 legacy **`api_key/base_url/model`** 迁移至当前 provider；Gateway 读取 provider 专属字段（legacy **`api_key`** 仍作回退） |
| 2026-05-23 | **AI 图片清理任务走真实 Provider**：去水印/去 Logo/去二维码/综合清理等不再走 **noop 占位演示**；默认读取 **`settings.image.provider`**（或 **`image_task_default_provider`**）；**`dashscope_image`** 接入万相 2.7 **图像编辑** API；结果仍 **`persistProviderResult` 入库**；未配置 Key 返回「未配置通义万相 API Key」；**`score_image`** 走 **AI 视觉模型**（`ai_vision`），**`select_best_main`** 逻辑保留 |
| 2026-05-23 | **通义万相默认模型更新**：默认模型 **`wan2.7-image-pro`**、默认尺寸 **`2K`**；**`dashscope_image`** 客户端切换至万相 2.7 **`multimodal-generation`** API；设置页 placeholder 同步 |
| 2026-05-21 | **拼多多图片分类精细化**：区域分轨主图/详情图；SKU 图仅 **sku.imageUrl**；修复详情图误入主图；**imageSummary**；管理端图片类型排序与提示 |
| 2026-05-21 | **拼多多 pifa 解析精细化**：标题清洗（去「分享商品」等）；**mainDescription** 提取与入库；图片 **source** 分类与 **imageFilterSummary**；SKU 名/库存拆分；草稿页字段级提示 |
| 2026-05-21 | **拼多多 pifa 批发详情页解析增强**：**`wholesale-detail*.ts`** 专用解析；标题/价格区间/SKU 行价库存/主图详情图/商品参数；平台标题过滤与质量 **`partial_success`**；草稿页 beta/SKU 提示；**`normalizePinduoduoImport`** **`priceMin/Max`**；**1688/custom 不变**；**批量仍关闭** |
| 2026-05-20 | **拼多多采集器 URL 类型与登录态**：**pifa.pinduoduo.com** → **`wholesale_detail`**；**`login_required`** 失败归类（非 unknown）；**`PinduoduoCollectModal`** + 任务页链接提示；**Profile** 可选；失败详情 **链接类型/访问状态**；Collector **`url-type.ts`**；不影响 **1688 / custom** |
| 2026-05-20 | **拼多多专用采集器 beta**：Collector **`sourcePinduoduo`**（**`source=pinduoduo`**，**`beta`**，**单链接**；**`batchSupported=false`**）；字段 **标题/价格/主图/详情图/参数/规格尽力**；**`LOGIN_REQUIRED`/`PAGE_BLOCKED_OR_VERIFY_REQUIRED`/`PARSE_FAILED_*`/`PRODUCT_NOT_FOUND`**；Go **`normalizePinduoduoImport`** + **`raw.extractProvider=pinduoduo`**；管理端采集中心 **测试中**、**`/settings/collector?provider=pinduoduo`**、草稿 **beta 提示**；自定义链接 **拼多多 URL 引导专用采集器**；不影响 **1688 / AliExpress / custom** |
| 2026-05-20 | **自定义链接采集器状态为基础可用（beta）**：采集中心 / 采集设置 **「基础可用」**（`status=beta`）；卡片文案与能力标签；无规则引导；Modal 说明；**PROGRESS** 边界与建议 |
| 2026-05-20 | **管理端全站文案小白化（二期）**：设置（AI/图片/库存/平台/采集）、AI 任务与技能模板、图片任务、订单规格匹配与异常、店铺授权、后台任务监控、商品详情/定价等；侧栏菜单与 `userFriendly`/`taskCenter` 文案；技术 JSON 默认折叠 |
| 2026-05-20 | **管理端采集文案小白化**：采集规则页顶栏说明、AI 弹窗/自定义采集/采集设置/登录状态页统一中文表述；错误码改用户可读说明并默认折叠「技术信息」；`collectErrors`/`collectAccess` 映射；Collector 测试建议与后端 `failureHint` 去技术词；README 采集章节术语说明 |
| 2026-05-20 | **AI 一键生成自定义采集规则**：新增 Collector **`POST /v1/custom/analyze-page`**（`PageStructureDigest`，每类候选 ≤20，不含完整 HTML）；后端模块 **`collectruleai`** + **`POST /api/v1/collect/rules/ai-generate`**（可选 **`ai-generate-and-save`**）；Prompt **`collect_rule_generate`**；采集规则页 **AI 生成规则** Modal；自定义链接采集器无规则 / 无匹配规则时引导 AI 生成；生成后自动 **`custom-rule-test`**；**1688 / AliExpress available/beta** 拦截提示专用采集器；**planned** 平台允许但提示 SKU/库存不保证；settings.collector **`collect_rule_ai_*`**；操作日志 **`collect.rule.ai_generate`** / **`ai_generate_failed`** / **`ai_save`**（不记完整 URL 参数 / 摘要 / Prompt）；安全：前端不直连 AI/目标站、AI 不访问 URL、规则 JSON 后端校验禁 script/eval、不保存账号密码、不绕过验证码。当前路线不变：**AI 商品运营工具** 优先 → **多平台跨境 ERP MVP** → 完整 ERP 增强暂不做 |
| 2026-05-20 | **采集服务设置页 Provider 维度**：**`/settings/collector?provider=1688|aliexpress|pinduoduo|taobao|shein_temu|custom`**；采集中心卡片 **「采集服务配置」** 带 provider 跳转（planned 显示 **「查看配置」**）；页面 **通用采集服务配置**（影响所有采集器）与 **Provider 专属配置** 分离；**1688** 保留登录态与 **`collect_batch_*_1688`**；**custom** 展示 Profile / 规则测试入口与 **`collect_custom_*`**（不展示 1688 登录）；**aliexpress beta** 说明与 **`collect_aliexpress_*` 预留**；**planned** 空状态；**settings.collector 保存逻辑不变**；不影响 1688 单条/批量与自定义链接采集 |
| 2026-05-20 | **自定义链接采集器用户文案与平台冲突校验**：采集中心卡片改为用户友好说明，移除 Profile/登录检测等技术描述；自定义 Modal 增加使用说明；输入 **1688 / 速卖通** 等已支持平台链接时前端禁用提交并引导专用采集器；**`POST /api/v1/collect/tasks`（`source=custom`）** 后端域名冲突校验 **`CUSTOM_COLLECT_PROVIDER_CONFLICT`（`recommendedProvider` + `message`）**；批量采集仍禁用且提示改为「先单链接验证规则」；通用访问状态检测仍保留在规则测试与任务详情 |
| 2026-05-20 | **自定义链接采集浏览器 Profile**：**`collect_browser_profiles`**；API **`/api/v1/collect/browser-profiles*`**；Collector **`POST /v1/browser-profiles/:key/open-login|check`**；**`useBrowserProfile`+`profileKey`** 贯穿规则测试与 **`collect_tasks.request_options`**；管理端 **Profile 列表/登录引导**；安全：不存密码、Cookie 仅 Collector 本地、不前端直连 |
| 2026-05-20 | **自定义链接通用访问状态检测**：Collector **`accessStatus`**（public/login_required/verify_required/…）+ **`POST /v1/collect/custom-rule-test`**；**`POST …/collect/rules/:id/test`** 返回 **accessStatus/finalUrl/extractedFields/missingFields/warnings/errorCode/suggestion**；Hub Modal **「测试访问与规则」**；任务 **`LOGIN_REQUIRED`/`PAGE_BLOCKED_OR_VERIFY_REQUIRED`/`PARSE_FAILED_*`** 中文解释；**非 1688 固定平台登录检测**（1688 仍 **`with1688Page`**）；预留 **`custom_*_browser_profile`**；**不**验证码破解、**不**自动绕过登录、**不**保存账号密码；**`docs/custom-collect-rules.md`** |
| 2026-05-20 | **自定义链接采集器（单链接）**：**`collect_rules`** CRUD/测试/域名匹配；规则 JSON 支持 **selector+type** 与 **selectors+attr**（Go **`NormalizeRuleJSON`** + Collector **`normalizeCustomRuleDecl`**）；**`source=custom`** 任务 **`request_options` 规则快照** → Worker → Collector **`sourceCustom`**（CSS + JSON-LD/OG fallback，**不写 DOM 到 Go**）；管理端 **采集中心 Modal**、**采集规则测试预览**、错误码 **`CUSTOM_*`/`PARSE_FAILED_*`**；**`batchSupported=false`**；边界：不用户 JS、不存完整 HTML、不破解验证码；1688 **`*.1688.com` 复用登录 Profile**；文档 **`docs/custom-collect-rules.md`** |
| 2026-05-20 | **发布价格配置 / 商品定价规则**：**`settings.pricing`**、**`internal/modules/pricing`**、**`product_skus.cost_price/compare_at_price/min_publish_price`**；API **`POST /pricing/calculate`**、**`POST /products/:id/pricing/apply`**、**`POST /products/pricing/batch-apply`**；管理端 **`/settings/pricing`**、草稿详情 **「应用定价规则」**、列表 **「批量设置发布价」**；**Product Readiness** 定价检查；**仅更新本地 SKU `price`**，不刊登、不调平台、不做财务 |
| 2026-05-20 | **1688 采集防复发文档**：新增 **`docs/collector-1688-pitfalls.md`**；修复 **`page.evaluate` `__name`**（禁止 toString 注入）、**`unitWeight` 误作价格**、**SKU 维度噪声**与 **`1.2mm` 价表**未解析；失败中心 **`collector_evaluate_script`** |
| 2026-05-20 | **1688 解析增强**：多路主图/价格 JSON+DOM 提取、页面滚动懒加载、mainImages 兜底、partial_success + completeness/extractDebug、失败 HTML/截图快照、失败中心 **missing_images/missing_price** 分类 |
| 2026-05-20 | **1688 采集浏览器登录态**：Collector **`launchPersistentContext`**（`collector/data/browser-profiles/1688`）；**`GET/POST /api/collector/providers/1688/*`**；管理端采集设置 **登录态卡片**；失败中心 **`collector_platform_login`**；Docker **Profile volume** |
| 2026-05-20 | **1688 批量采集稳定性**：Go **`batch_policy`/`batch_throttle`/`batch_stats`**；**`COLLECT_BATCH_*_1688`** + **settings.collector**；批量随机 delay + Redis 并发门闸；批量可重试风控/超时/导航失败；批次 **errorSummary**；Collector **风控增强 + `TIMEOUT`/`PARSE_FAILED`**；admin **批次/任务失败提示** |
| 2026-05-19 | **图片 AI 小白友好配置 + 国内 Provider**：**`/settings/image`** 场景卡片 → Provider 选择 → 按 Provider 动态字段；**`GET /api/v1/image/providers`**；**`POST /api/v1/settings/test-image`**（`config_only`/`live`）；**`settings.image`** 扩展通义万相/火山方舟/硅基流动/混元预留密钥字段；后端 **`dashscope_image`**（万相文生图）、**`volcengine_image`/`siliconflow_image`**（OpenAI 兼容 generations）、**`hunyuan_image` planned**；**`image_tasks` 创建**校验能力矩阵；商品详情/图片任务页按任务过滤 Provider；仍经 **Worker**，前端不直连；**README** 同步。 |
| 2026-05-19 | **1688 采集增强（二）**：解析 **`window.context.result.data`**（gallery/skuMap/属性/默认价）；修正 SKU 键 **`颜色值>尺码值`** → `颜色`+`尺码` 维度；从 **`specAttrs`/`canBookCount`** 与 DOM 尺码表补 **价/库存**；详情图仅保留 **ibank** 商品图；过滤 **imgextra/cms/tps** 图标。 |
| 2026-05-19 | **1688 采集增强**：Collector 主图 **DOM 优先**、过滤服务承诺图标（`isLikelyJunkImage` / 祖先区域跳过）、JSON 仅取 offer 图字段；SKU 增加 **DOM 规格表 + `window.__INIT_DATA` 等全局 JSON** 兜底；管理端草稿 **「采集属性」** 表展示 `raw_data.attributes`。 |
| 2026-05-19 | **开源治理与 AI 关联配置补齐**：新增 **`.github/CODEOWNERS`**、**`.github/dependabot.yml`**、**`.github/labeler.yml`**、**`.github/workflows/labeler.yml`**、**`.github/workflows/docker.yml`**、**`CHANGELOG.md`**；Go / Node CI 增加 **`workflow_dispatch`**；新增 **`docs/module-map.md`**、**`docs/env.md`**、**`docs/api.md`**、**`docs/provider-template.md`**、**`docs/task-checklist.md`**，并同步 **AGENTS / Cursor rule / docs index / README / CONTRIBUTING / PR 模板**，用于约束环境变量、API、Provider、Docker、CI 与文档联动。 |
| 2026-05-19 | **通用 AI Agent 入口**：新增根目录 **`AGENTS.md`**，作为 Cursor 以外 AI 编辑器 / Agent 的通用协作入口，集中说明必读文档、技术栈、开发规则、文档同步要求、检查命令与禁止事项；README / README.en / docs index 增加入口。 |
| 2026-05-19 | **Cursor rules 轻整理**：新增 **`.cursor/rules/README.md`**，按全局规则与领域规则列出每个 `.mdc` 的用途、适用范围和新增规则 checklist；`docs/README.md` 增加 Cursor rules 索引入口。 |
| 2026-05-19 | **文档中心整理**：新增 **`docs/README.md`**，统一收口开发、部署、架构、Provider、路线图、分支规则、AI 编程规则、赞助、安全、行为准则等入口；README / README.en 文档导航改为分组式入口，首页更清爽，后续新增文档优先维护 docs index。 |
| 2026-06-30 | **Phase F8.1 冻结基线复跑**：backend 重启；edge-case seed 200；demo:auto-acceptance **passed**；smoke 路径与 seed/RBAC 修复；F9 准入 **`f8_1_passed_with_warning_ready_for_f9`** |
| 2026-06-30 | **Phase F7 全项目 Demo 数据升级**：扩展 `seed-demo-data` / `seed-demo-permissions`；订单 / 库存 / 客服 / Dashboard KPI 样本与 `demo-dataset.full-project.json`；F7 smoke 脚本；`demo-auto-acceptance` Phase F7-Auto；[`DEMO_SEEDING_GUIDE.md`](DEMO_SEEDING_GUIDE.md)、[`DEMO_AUTO_ACCEPTANCE_GUIDE.md`](DEMO_AUTO_ACCEPTANCE_GUIDE.md)；F7 文案复扫 passed。**F2–F7 完成**；**F8 冻结进行中**；**非 Production Ready**；**Tag deferred** |
| 2026-06-27 | **AI 商品运营工作台 Phase A3.3**：**`internal/modules/aiopsworkbench`**、**`/api/v1/ai/operation-workbench/*`**、**`/ai/operation-workbench`**；聚合 AI 文案/图片待复核、发布检查、刊登批次异常、taskcenter 失败；设计见 **`AI_OPERATION_WORKBENCH_DESIGN.md`** |
| 2026-05-19 | **AI 编程规则与文档同步要求**：新增 **`docs/ai-coding-rules.md`** 与 Cursor 持久规则 **`.cursor/rules/12-ai-coding-doc-sync.mdc`**，明确代码、配置、环境变量、Docker、CI、API、Provider、页面、任务、数据库与安全变更必须同步相关文档；同步 **README / README.en / CONTRIBUTING / PR 模板**。 |
| 2026-05-19 | **CI 与分支策略文档**：新增 **`.github/workflows/node.yml`**，对 **admin** 执行 `pnpm build:admin`、对 **collector** 执行 `pnpm build:collector`（push / PR 到 `main`、`dev`）；新增 **`docs/branching.md`**，固化 `main` / `dev` / `feat/*` / `fix/*` / `release/*` 分支策略与 PR 合并规则；更新 **README / README.en / CONTRIBUTING / PR 模板** 导航与检查项。 |
| 2026-05-19 | **开源社区治理文件补齐**：新增 **`.github/ISSUE_TEMPLATE/config.yml`**（关闭空 Issue，提供文档 / 安全 / 赞助入口）、**`SECURITY.md`**、**`CODE_OF_CONDUCT.md`**、**`NOTICE`**，并同步 README / README.en 文档导航。 |
| 2026-05-19 | **README 中英文拆分与展示区补齐**：中文首页保留在 **`README.md`**，英文首页独立为 **`README.en.md`**，两者结构一致并互相跳转；标题改为居中；项目介绍改为当前已支持能力表达；新增 **合作商展示 / 贡献榜 / 赞助榜** 预留区；README 命令仅保留根 `package.json` 实际脚本与现有 Docker Compose 命令。 |
| 2026-05-19 | **赞助入口补充**：新增 **`docs/sponsor.md`**（微信 / 支付宝二维码赞助说明，图片位于 **`docs/assets/`**），新增 **`.github/FUNDING.yml`** 指向赞助说明页，README 中英文赞助章节与文档导航同步更新。 |
| 2026-05-19 | **GitHub 开源文档体系建设**：重构根 **`README.md`** 为中英文双语开源首页（Banner、Badges、导航、功能表、启动方式、架构、Roadmap、开源使用规范、Sponsor、License）；新增 **Apache-2.0 `LICENSE`**、**`CONTRIBUTING.md`**、**Issue / PR 模板**、基础 docs（development / docker-deployment / architecture / provider / roadmap）；原项目地址明确为 **`https://github.com/lien0219/trademind-ai`**。 |
| 2026-05-19 | **Docker 部署启动**：根 **`docker-compose.full.yml`**（独立 **`name: trademind-full`**；postgres 16 + redis 7 + **backend / admin / collector** 分容器；**backend** `COLLECTOR_BASE_URL=http://collector:3001`、持久卷 **`trademind_full_*`**；**admin** **`Dockerfile`** + **`nginx.conf`** 代理 **`/api` `/static`** → backend，SPA **`try_files`**）；**`backend/Dockerfile`**（Go 多阶段）；**`collector/Dockerfile`**（**`mcr.microsoft.com/playwright`** + pnpm build）；**`.env.docker.example`**；README **「启动方式」**：方式一 **pnpm dev**（沿用 `scripts/`）、方式二 **Docker 部署启动** + FAQ；**不移除** `docker-compose.yml` / 不写密钥入镜像 / 默认 **`down` 不 `-v`** / admin **不直连第三方**。 |
| 2026-05-19 | **开源一键本地开发启动**：根 **`pnpm dev`**（`scripts/dev-all.ts`）拉起 **PostgreSQL+Redis** 与 **backend/admin/collector** 并行；**`pnpm check:dev` / `dev:infra` / `dev:backend` / `dev:stop` / `dev:reset`**；**.env 仅在缺失时从 `.env.example` 复制**；README **「启动方式」方式一** 与 **分开启动**；根 **devDependencies：`tsx`/`execa`/`picocolors`** |
| 2026-05-19 | **产品路线固化**：文件顶部新增 **《当前产品路线》**（双主线优先级、§一§二目标链路、§三完整 ERP 后置清单、§四下一步、§五文档口径）；**§1** 补充「当前阶段定位」；**§8** 与验收/体验/演示收口对齐；**最后更新** 日期调整 |
| 2026-05-23 | **工作台 / 商品运营看板体验增强**：`operationdashboard` 增强 **`funnel`/`exceptions`/紧凑 KPI/12 快捷入口/最近动态**；前端五区布局 + 筛选 + 自动刷新 + 深链；列表页 **`?status=`** URL 筛选；**只读聚合**不变 |
| 2026-05-18 | **商品运营看板 / 待办中心**：**`internal/modules/operationdashboard`**、**`GET /api/v1/dashboard/product-operations`**（只读）；**`/dashboard/product-operations`**、**`services/dashboard.ts`**；**`GET /products`** 深链筛选；**PROGRESS** §1·§3·§7·§8 |
| 2026-05-18 | **商品发布前检查（productcheck）**：**`GET/POST …/readiness(/batch)`**、**刊登前强制检查**、管理端 **发布检查 Tab / 刊登联动 / 草稿列表批量**；**PROGRESS** §1·§3·§7·§8 |**`POST /api/v1/inventory/stock-settings/batch-preview`** · **`batch-update`**；**`settings.inventory.inventory_stock_settings_batch_max_size`**；**`inventory.stock_alert.batch_update`**；管理端 **`/inventory/alerts`**、草稿 **「库存」Tab**、**设置 → 库存/订单**；**不改 stock / 不写 inventory_change_logs / 不创建 inventory_sync_tasks`**；**PROGRESS** §1·§3·§7·§8 |
| 2026-05-17 | **批量库存同步批次**：**`inventory_sync_batches`** + **`inventory_sync_tasks.batch_id`/`batch_no`**；**`POST|GET /api/v1/inventory-sync/batches`** 及详情/任务/重试 API；Worker **幂等聚合批次** + **`inventory_rate` 初版限流**；**`settings.inventory`** 批量上限与各平台每分钟阈值；管理端 **`/inventory/sync-batches`**、预警页 **勾选批量同步**、草稿库存 Tab **刊登映射批量同步**、同步任务页 **失败批量重试 + `batchId` 筛选**；**`inventory.sync_batch.*`** 操作日志；**PROGRESS** §3/§7/§8 |
| 2026-05-17 | **告警通知配置 settings 化（收紧）**：通知业务项 **不写入 .env**（移除示例中的 **详情前缀 / Webhook 超时**）；新增 **`taskcenter.alert_detail_public_base`**、**`alert_notify.webhook_timeout_seconds`**；**EnsureTaskcenterDefaults / EnsureAlertNotifyDefaults** 初装 **空/false**；站内告警 **去掉代码内嵌阈值默认**；**`/settings/alert-notify`** 页正名为 **告警通知配置**；**`PROGRESS`** 与 **`integration_schema`** 对齐 |
| 2026-05-17 | **告警外部通知 + `task_alert_scan` Worker**：**`task_alert_notifications`** **AutoMigrate**；**`settings.alert_notify`/`EnsureAlertNotifyDefaults`**（**Webhook/飞书/企微 URL·Secret 加密**）；**扩展 `taskcenter`**（**`enable_external_notifications`** 等）；**`internal/modules/taskcenter/notify`**（**mail+SMTP**、**Webhook+HMAC**、**feishu/wecom planned**）；**扫描/生成后 `NotifyGeneratedAlerts`**（**去重** / **`alert_count` 重复规则**）；API **`GET /task-center/alert-notifications`**、**`POST /alerts/:id/notify`**；**`task_alert_scan`** **`worker_instances`**；**`/health` `byType`**；**`settings.alert_notify.update`** 等操作日志；管理端 **`/settings/alert-notify`**、**`/task-center/alerts`** **通知列·Drawer**；**.env.example** **`TASK_ALERT_SCAN_*`（部署门闸）**；**PROGRESS** 头部·§7·§8·变更记录 |
| 2026-05-17 | **任务失败原因分类 + 站内告警基座**：**`failureclassifier`**（**`failure_category` / severity / suggestedAction**，**不改**任务状态）；**`UnifiedTaskDTO`** 扩展 **`failureCategory`、`severity`、`matchedRule`、`alertStatus`、`relatedAlertId`**；**`task_alerts`** 表 **`AutoMigrate`**；**`settings.taskcenter`** **`EnsureTaskcenterDefaults`**；**`GET …/failures` 筛选** + **`POST …/generate-alert`**；**`GET/POST …/alerts`、`scan`、`handle`、`ignore`、`DELETE …/mark`**；操作日志 **`task_center.alert.*`**；管理端 **`/task-center/alerts`**、失败中心列/抽屉、**`/settings/system` 告警策略**；**PROGRESS** §3.2§3.3§7§8 |
| 2026-05-17 | **统一失败任务中心（taskcenter）**：表 **`task_failure_marks`**；模块 **`internal/modules/taskcenter`**；**JWT** **`GET /api/v1/task-center/failures`**、`GET /summary`、`GET …/failures/:taskType/:id`、`POST …/retry`、`POST …/failures/batch-retry`、`POST …/ignore|handle`、`DELETE …/mark`、`batch-ignore`/`batch-handle`；类型 **collect · image · order_sync · customer_message_sync · product_publish · inventory_sync**；操作日志 **`task_center.*`**；管理端 **`/ops/task-center/failures`**、`services/taskCenter.ts`、运维分组与 **Dashboard / Worker** 快照；**PROGRESS** §3.2 §3.3 §7 §8 |
| 2026-05-17 | **库存预警基础能力**：**`product_skus.warning_stock` / `safety_stock`**（及可选 **`stock_status` / `last_stock_checked_at`**）；**`product.CalculateSKUStockStatus`**；**`settings.inventory`** 幂等 **`default_warning_stock` / `default_safety_stock` / `enable_inventory_alerts` / `alert_out_of_stock` / `alert_platform_stock_mismatch` / `platform_stock_mismatch_threshold`**；**`GET /api/v1/inventory/alerts`**（默认仅 **有 `alertTypes`**；**`includeNormal`**；**平台镜像 vs 本地**、**`inventory_sync_failed`** 摘要）；**`PUT /api/v1/products/:id/skus/:skuId/stock-settings`** + **`inventory.stock_alert.update`**；**`fromInventoryAlert` 同步成功** → **`inventory.alert.sync_inventory`**；管理端 **`/inventory/alerts`**、**设置 → 库存/订单**、草稿 **「库存」Tab**；**PROGRESS** §3/§7/§8 |
| 2026-05-17 | **平台订单 SKU 自动匹配基座**：**`order_item_sku_matches`**、**`MatchOrderItemToSKU` / `MatchOrderItemsForOrder`**、**ordersync 编排**、REST **`GET …/sku-matches`、`POST …/match-skus`、`GET /order-item-sku-matches`、`POST /order-items/:id/bind-sku`、`GET /product-skus/search`**、**`order.sku_match.*`**、**`settings.inventory`** 新键、管理端 **`/orders/sku-matches`** 与订单 Drawer **「SKU 匹配」**；**PROGRESS** §3/§7/§8 |
| 2026-05-17 | **Amazon 真实库存同步 API（beta）**：**`amazon/inventory_sync*.go`**（**`SyncInventory`**，Listings Items **`PATCH`** **`fulfillment_availability`**）、**`shop.AmazonShopsBridge.AmazonPublishSettings`**（**`platform_publish_amazon`**）、**`platform.InventorySyncImplementationStatus(amazon)=beta`**；**`inventory` Worker** 允许 **`amazon`** 任务 **`external_product_id` 为空**；管理端草稿 **「库存」** Amazon **beta** 文案与错误提示；**PROGRESS** §1/§3/§7/§8 |
| 2026-05-17 | **Amazon 商品刊登真实 API（beta）**：**`amazon/product_publish*.go`**（**`PublishProduct`**、SP-API **Listings Items** `PUT /listings/2021-08-01/items/{sellerId}/{sku}`、**`amazon_attributes`** 手工 attributes、公开图片 URL 校验）、**`platform.ProductPublishImplementationStatus(amazon)=beta`**；复用 **`settings.platform_amazon` + `settings.platform_publish_amazon` + `shop_auth_tokens`**；product_publish Worker 对 **`publishing`** 状态不再误标 **`published`**；管理端草稿刊登说明加入 **Amazon beta**；**PROGRESS** §1/§3/§7/§8 |
| 2026-05-17 | **Lazada 真实库存同步 API（beta）**：**`lazada/inventory_sync*.go`**（**`SyncInventory`**，**`/product/price_quantity/update`**、**`/product/item/get`**），**`platform.InventorySyncImplementationStatus(lazada)=beta`**；**`LazadaShopsBridge.LazadaPublishSettings`** + **`publish_config_presets`** **Lazada 可选 `warehouse_id`**；管理端 **草稿「库存」`/inventory/sync-tasks`/错误提示**；**PROGRESS** §1/§3/§7/§8 |
| 2026-05-17 | **Shopee 真实库存同步 API（beta）**：**`shopee/inventory_sync*.go`**（**`SyncInventory`**、**`POST /api/v2/product/update_stock`**）、**`platform.InventorySyncImplementationStatus(shopee)=beta`**、**`shop.ShopeeShopsBridge.ShopeePublishSettings`**（**`platform_publish_shopee`**）；管理端草稿 **「库存」Tab** / 同步 Modal **Shopee** 说明与错误提示；**PROGRESS** §1/§3/§7/§8/变更记录 |
| 2026-05-17 | **TikTok Shop 真实库存同步 API（beta）**：**`tiktok/inventory_sync*.go`**（**`SyncInventory`**、**`POST /product/{ver}/products/{id}/inventory/update`**）、**`platform.InventorySyncImplementationStatus(tiktok)=beta`**、**`ErrPlatformInventorySyncPermissionDenied`**、**`TikTokPublishSettings` bridge**（**`platform_publish_tiktok`**）；管理端草稿 **「库存」Tab** 文案与 **TikTok** 同步 Modal 引导；**PROGRESS** §1/§3/§7/§8/变更记录 |
| 2026-05-17 | **库存同步基础框架**：表 **`inventory_change_logs`/`inventory_sync_tasks`**、**`InventorySyncProvider.SyncInventory`**（**mock=`available`**；**TikTok/Shopee/Lazada/Amazon `inventory_sync` rollout** 见上/下相邻变更记录）、Redis **`INVENTORY_SYNC_*`**、Worker **`inventory_sync`**、**`/health` `inventorySyncQueue`**、JWT **adjust-stock / sync-inventory / inventory-sync tasks / inventory logs**；admin **`/inventory/sync-tasks`** 与草稿「库存」Tab；**PROGRESS** §3/§7/§8 |
| 2026-05-17 | **Lazada 商品刊登真实 API（beta）**：**`lazada/product_publish*.go`**（**`PublishProduct`**、**`/product/create` `payload`**、**`/image/migrate` + `/image/upload`**）、**`platform.ProductPublishImplementationStatus(lazada)=beta`**、**`platformlazada.BindPublishImages`**、**`options.lazada_attributes`**；**`productpublish` handler 403** 文案含 **Lazada Open Platform**；管理端 **草稿刊登** 说明 **Lazada beta**；**PROGRESS** §1/§3/§7/§8 |
| 2026-05-17 | **Shopee 商品刊登真实 API（beta）**：**`shopee/product_publish*.go`**（**`PublishProduct`**、**Media Space `upload_image`**、**`add_item`**、多 SKU **`init_tier_variation`**）、**`postShopMultipartImage`**、**`platform.ProductPublishImplementationStatus(shopee)=beta`**、**`platformshopee.BindPublishImages`**（复用 **`tikTokListingImageFetcher`**）、**`merge_publish`/`platform_publish_shopee`** 必填 **`default_weight`**、刊登 **403** 文案泛化；**PROGRESS** §1/§3/§7/§8 |
| 2026-05-17 | **TikTok 商品刊登真实 API（beta）**：**`tiktok/product_publish*.go`**（**`PublishProduct`**、上图、映射 **`PlatformProductDraft`** → **`POST /product/{ver}/products`**）、**`platform.ProductPublishImplementationStatus(tiktok)=beta`**、**`IsProductPublishRunnable`**、**`BindPublishImages`**（**`api/tiktok_listing_images.go`**）、**`product_publication_skus` price/stock/raw_data**、**`ErrPlatformProductPublishPermissionDenied`**、管理端刊登 Tab **`beta`**；**PROGRESS** §1/§3/§7/§8 |
| 2026-05-17 | **Amazon 真实客服消息 API（Messaging API，`beta`）**：**`amazon/customer_messages.go`** + **`customer_message_*`**（Orders + Messaging HAL；模板 **`POST`**；**`CustomerMessageImplementationStatus(amazon)=beta`**；**`send-platform-message` 403 `%w` 权限哨兵**；管理端 **`/shops`**、**权限文案**；**PROGRESS** §1/§3/§7/§8 |
| 2026-05-17 | **Lazada 真实客服消息 API（beta）**：**`lazada/customer_messages.go`** + **`customer_message_*`**（**`/im/session/list`** / **`/im/message/list`** / **`/im/message/send`**；**`signedGET`/`signedPOSTForm`**；**`CustomerMessageImplementationStatus(lazada)=beta`**；**`platform/customer_message.go`**；管理端 **`/shops`** **权限文案含 Lazada**；**PROGRESS** §3/§7/§8 对齐 |
| 2026-05-17 | **Shopee 真实客服消息 API（beta）**：**`shopee/customer_messages.go`** + **`customer_message_*`**（**`/api/v2/sellerchat/*`**, **`getShopWithStatus`** GET 签名、**`PullMessages`/`SendMessage`**；**`CustomerMessageImplementationStatus(shopee)=beta`**；**`platform/customer_message.go`**；**`send-platform-message`** **403** 泛化文案；**`/shops` 权限错误提示**；**PROGRESS** 全文对齐 |
| 2026-05-17 | **TikTok 真实客服消息 API（beta）**：**`tiktok/customer_message_*.go`**（**`PullMessages`/`SendMessage`**，**`customer_service/{api_version}/…`** + **`signedPOSTJSONStatus`**）；**`CustomerMessageImplementationStatus(tiktok)=beta`**；**`/shops`** **客服拉取**放行 **`beta`**；**`send-platform-message`** **403** 中文提示；**PROGRESS** §1/§3/§7/§8 |
| 2026-05-17 | **平台客服消息同步框架（admin 与文档收尾）**：**CustomerMessageProvider**、**`customersync`**、**`CUSTOMER_MESSAGE_SYNC_*`**；管理端 **`/customer/message-sync-tasks`**、会话 **拉取平台消息**、详情 **发送到平台**、**`/shops`** **拉取客服消息**；**`.env.example`**；**PROGRESS** §3/§7/§8/变更记录 |
| 2026-05-17 | **Amazon SP-API OAuth + 订单同步（beta）**：**`providers/platform/amazon`**（**`config`/`oauth`/`sign`/`orders_api`/`mapping`/`provider`**）；**`GET/POST /api/v1/shops/:id/oauth/amazon/*`**；**`platform_amazon`** schema 增 **`lwa_token_url`/`redirect_uri`** 等；**bootstrap** 移除 Amazon **`planned`**；**`shop.AmazonShopsBridge`**；**`ordersync`** 与 **Worker 租约**不变；管理端 **`/shops`**；**PROGRESS** §3/§7/§8 |
| 2026-05-17 | **多实例 Worker heartbeat / 任务租约**：`worker_instances`、`taskreaper`、三任务表租约列、`GET /api/v1/workers/monitor`、管理端 `/workers/monitor`、`/health` `workers`、`WORKER_*` / `COLLECT_TASK_TIMEOUT_SECONDS`、采集 **`worker.lease.*`** 事件 |
| 2026-05-17 | **Lazada OAuth + 订单同步（beta）**：**`providers/platform/lazada`**（`Sign`、`ResolveRuntime`、OAuth token、`/seller/get`、`/orders/get`…、`mapping`）；**`/shops/*/oauth/lazada/*`**、**Redis state**；**bootstrap 移除 Lazada planned**；**`/platform/settings/lazada`** 强校验；管理端 **`/shops`**；PROGRESS §3/§7/§8 |
| 2026-05-16 | **Shopee OAuth + 订单同步（beta）**：**`providers/platform/shopee`**（签名、OAuth、`TestConnection`、`SyncOrders`、**`mapping.go`→`PlatformOrder`**）；**`/shops/*/oauth/shopee/*`**、**Redis state**；**bootstrap 移除 Shopee planned**，**`/platform/settings/shopee`** 强校验；管理端 **`/shops`** Shopee 授权 Drawer；PROGRESS §3/§7/§8 |
| 2026-05-16 | **第三方配置中心整理**：**`integration-schemas` / `integrations/overview`**、**`PlainMailSettings`（mail+email）**、**`test-email` 路由**、**`/settings/integrations`**、**平台 `planned` 部分保存**、**platform settings 响应 `schema`**、**店铺创建/授权前校验 Partner 配置**、**AI temperature/max_tokens** 与多页 **Alert** |
| 2026-05-16 | **Open Platform 应用配置 Schema 化（多平台）**：**`Provider.AppConfigSchema()`**；**`GET /api/v1/platform/providers`** 带 **`appConfigSchema`/`settingsGroupKey`**；**`GET`/`PUT /api/v1/platform/settings/:platform`** + **`platform.settings.update`**；管理端 **`/settings/platforms`** Tabs 动态表单；**TikTok** 缺 Partner 配置 **`platform config incomplete…`**；README 多平台门户表 |
| 2026-05-16 | **平台开放配置（`platform_tiktok`）+ TikTok ResolveRuntime 去环境默认**：**`/settings/platforms`**、**test-platform-tiktok**、`app_secret` 加密、店铺 OAuth/token 分层、README/PROGRESS/.env.example；**tiktok/oauth callback** 仅写 token |
| 2026-05-16 | **TikTok Shop OAuth + 订单同步（beta）**：**`tiktok`** Provider（**TestConnection`、`SyncOrders`、token 刷新）、**`/shops/:id/oauth/tiktok/*`**、`platform_tiktok` + **`TIKTOK_*`**、管理端 **`/shops`**；沿用 **`ordersync`/`UpsertSyncedOrders`**；**.env.example** 补 TikTok；PROGRESS §1/§3/§§7–8 |
| 2026-05-16 | **平台订单同步基础框架**：**`order_sync_tasks`** + **`internal/modules/ordersync`**（**`ORDER_SYNC_*`** Redis **Worker** / **`ORDER_SYNC_QUEUE_ENABLED=false`** 同步执行）；**`OrderSyncProvider`/`PlatformOrder`/`mock`/`planned`/`manual`**；API **`POST /api/v1/shops/:id/sync-orders`**、**`/api/v1/order-sync/tasks*`**；**`order.UpsertSyncedOrders`**；**`/health` `orderSyncQueue`**；管理端 **`/orders/sync-tasks`**、**`/shops` 同步入口**；**`orders.remark`**；PROGRESS 对齐 |
| 2026-05-16 | **自定义链接采集（beta）**：**`collect_rules`** + **`collect_tasks.request_options`**；**`/api/v1/collect/rules*`** CRUD / enable / disable / **test**（Collector **`options`**）；**Collector `sourceCustom`**（selector + JSON-LD / OG / meta，`raw.stateDigest`）；**`/collect/rules`**、Tasks **custom `ruleId`**；Hub **custom 批量提示**；**PROGRESS** 对齐 |
| 2026-05-16 | **AliExpress 速卖通真实解析（Collector `beta`）**：**`collector/src/providers/sourceAliExpress`**（脚本 JSON + DOM；标题 / 主图≤10 / 详情图≤30 / 属性 / SKU **`properties`**；**`INVALID_URL`/人机 `PAGE_BLOCKED_OR_VERIFY`/无标题 `COLLECT_FAILED`**）；移除 stub 占位；**`pnpm collect:test -- --source aliexpress`** + **`COLLECT_TEST_SOURCE`**；**Go **`ValidateSourceForCollect`** 放行 **`beta` 单笔**；**/collect Hub+Tasks **`beta`** 可跑**、**批量**仍 **可用性=available&&batchSupported**（**beta Tooltip**）；**兜底列表 `providers.go`** **`aliexpress`→`beta`**；**PROGRESS** 对齐 |
| 2026-05-16 | **采集 Provider 通用化**：**Collector **`GET /v1/providers`** + 占位 Provider **`pdd`|`taobao`|`aliexpress`|`shein_temu`|`custom`**；Go **`GET /api/v1/collect/providers`**、创建任务 **`available`/batchSupported**、**scheme** 前缀校验与非 **Collector** 报错脱敏兜底；1688 **`PAGE_BLOCKED_OR_VERIFY_REQUIRED`**；管理端 **`/collect` Hub**、`collectProviders.ts`；**PROGRESS §3–§8** 对齐 |
| 2026-05-16 | **内部手工订单 + 客服关联**：**`internal/modules/order`**（**`orders`/`order_items`/`order_shipments`** + JWT CRUD + **`order.*`** 日志）；**`customer_conversations.order_id`**、详情 **`orderSummary`**、**`customer.conversation.link_order`**；**`customer_reply_generate`** 增补 **`{{orderInfo}}`** 等 + **`migrateCustomerReplyGenerateOrderContext`**；生成回复 **载入订单快照与风险调高**；管理端 **`/orders`**、工作台 **选单/解绑与 Alert**；**PROGRESS** 对齐 |
| 2026-05-16 | **AI 客服 MVP**：**`internal/modules/customerchat`** + **三表** + **Prompt `customer_reply_generate`** + **REST API** + **`ai_tasks.conversation_id` / `task_type=customer_reply_generate`** + **管理端 `/customer/*`** + **操作日志 `customer.*`**；**PROGRESS** 全篇对齐 |
| 2026-05-16 | **腾讯云 COS / 阿里云 OSS 独立 Storage**：**`providers/storage/cos`、`oss`** + **factory**（**`kind`** + **`files.storage_kind`**）；**test-storage**：**COS HeadBucket** / **OSS ListObjects(max1)**；**管理端存储页** **COS·OSS**；**`keypath.NormalizeSafe`**；**.env.example / README**；**go.mod SDK** |
| 2026-05-16 | **OpenAI Image `replace_background`**：`openaiimage.ReplaceBackground`（**multipart `image[]` → `/images/edits`**）；**`imagetask` 编排 + `SaveProcessed` + 命名 `openai-replace-bg-*`**；**`output.taskType`**；**retry 分类**；**admin `/ai/image-tasks` + 商品详情**；**PROGRESS** §1/§3/§4/§6/§7/§8 |
| 2026-05-16 | **remove.bg 非公网源图**：Storage **`Get`**（**local / s3 / r2 / minio / cos / oss**）；**`imagetask/source_resolver`** + **`httppublic`**；remove.bg **`image_file`/`image_url`**；重试分类 **`source image is not readable…`** 等 **不可重试**；admin **`/ai/image-tasks`** **`sourceImageId`** + 文案；**PROGRESS** §1/§3/§4/§6/§7/§8 |
| 2026-05-16 | **图片任务自动退避重试**：**`image_tasks`** 重试字段与 **`retrying`**、**`IsRetryableImageTaskError`**、**`retry_scheduler.go`**（**`IMAGE_QUEUE_ENABLED` + `IMAGE_AUTO_RETRY_ENABLED`**）、**Worker** 认领规则、**monitor** **retry** 块、**`image.task.auto_retry_*` / `retry_exhausted`**；**`.env.example` / `config`** **`IMAGE_AUTO_RETRY` 默认 true**；管理端 **`/ai/image-tasks`**；**PROGRESS** §3/§4/§6/§7/§8 |
| 2026-05-16 | **ComfyUI Image Provider**：**`providers/image/comfyui`**（prompt/history/view/upload、变量替换、`generate_scene` + 基础 **`replace_background`**）；**`settings.image` / EnsureImageDefaults** 补全 **`comfyui_*`**；**`factory` / Worker / `files.SaveProcessed`**；**管理端** `/settings/image`、`/ai/image-tasks`、**商品详情**；**`golang.org/x/image/webp`**；PROGRESS 全篇对齐 |
| 2026-05-16 | **OpenAI Image Provider**：**`providers/image/openaiimage` HTTP Client + `factory` 适配 **`openai_image`**；**`settings.image`** 增补 **`openai_image_*`** 默认种子；**`generate_scene`**（**assembled_prompt**、可无源、`output.model`）；**`/settings/image`、`/ai/image-tasks`、商品详情图片 Tab** 联动；PROGRESS §1–§8 对齐 |
| 2026-05-16 | **图片任务异步化**：**`image:tasks`** + **`imagetask/worker`**（**`IMAGE_QUEUE_*`** / **`IMAGE_TASK_TIMEOUT_SECONDS`**）；入队 **`pending`** / **retry** / **503 队列不可用** / **`IMAGE_QUEUE_ENABLED=false`** 同步、`/image/tasks/monitor`、`/health` **`imageQueue`**；admin **轮询** |
| 2026-05-16 | **remove.bg**：**`providers/image/removebg` Client** + **`factory.NewForTask`**（noop/removebg）；**`settings.image`** **`removebg_base_url`** 种子；**`files.SaveProcessed`**；**imagetask** 持久化 **`result_file_id`/`result_url`/output**；admin **`/settings/image`**、**`/ai/image-tasks`**、商品详情 **Provider 可选 removebg**；**PROGRESS** §1/§3/§6/§7/§8 同步 |
| 2026-05-16 | **云存储 S3-compatible**：后端 **`internal/providers/storage/s3store`**（**AWS SDK v2**）、**factory**（`local`/`s3`/`r2`/`minio`，**COS/OSS 当时未接独立 SDK**，见本条记录之后续「COS/OSS」行）、**`files/upload|delete`** 与 **`test-storage` HeadBucket**；删除按 **`storage_kind`**；admin **存储设置 `s3_*`**；**`.env.example` / README** 存储说明；**go.mod** 引入 AWS SDK；**PROGRESS** 全篇对齐 |
| 2026-05-16 | **GitHub Actions Go CI**：`.github/workflows/go.yml`（`main` 上 **push / pull_request**；`backend/` 内 **`gofmt -l` / `go vet` / `go test` / `go build`**；缺失 **`backend/`** 或 **`backend/go.mod`** 时显式失败；**`go-version-file: backend/go.mod`**）；**`go fmt`** 整理部分后端源文件以满足格式检查；**README** 增加「**CI / 自动检查**」 |
| 2026-05-19 | **AI Provider（DeepSeek / Qwen）**：**`internal/providers/ai/compatclient`** 抽取 Chat Completions；**`deepseek` / `qwen` / `openai`** 独立包 + **`factory`/`gateway`**；**`test-ai`** 返回 **provider/model/latencyMs** 与中文错误；管理端 **AI 设置** 四档 Provider 与示例 **base_url/model**；**README** / **`docs/provider.md`** 配置说明 |
| 2026-05-16 | **AI 图片任务预留**：**`image_tasks`**、**`internal/providers/image` + `noop`**、**`POST|GET /api/v1/image/tasks`、详情、`retry`**、**`settings.EnsureImageDefaults`（`image` 分组）**、操作日志 **`image.task.*`**、管理端 **`/ai/image-tasks`**、**`/settings/image`**、商品详情 **图片 Tab 入口**；**PROGRESS** §1/§3/§6/§7/§8 同步 |
| 2026-05-16 | **`collect_task_events` + Timeline API + Admin Drawer**：新增表（**§3.2**）、节点写入、`GET /api/v1/collect/tasks/:id/events`（**JWT**、**ASC**、默认 **pageSize=50**）；**`CollectTaskEventDrawer`**（任务/批次/监控）；rollback 连带删事件；**§7 遗留（heartbeat/AI图/多云/Collector）§8 下一步** 重排 |
| 2026-05-16 | **采集队列可观测性**：**`GET /api/v1/collect/monitor`**（JWT；**`LLEN`**、任务/批次 **`GROUP BY status`**、**`recentFailures`**、**`oldestPendingSeconds`**、**Worker**、**Collector `/health` 短超时**）；**`/health` / `/api/v1/health`** **`collectQueue`**（无 Collector 探测）；**`ConfigureWorkerMonitor` + `SetCollectWorkersRunning`**；管理端 **`/collect/monitor`**（**5s**、**visibility** 暂停、失败任务 **Drawer**）；**`/collect/batches?batchId=`**、**`/collect/tasks?batchId=`** 深链；**§7 遗留 / §8 下一步** 按监控收尾后重排 |
| 2026-05-16 | **批量采集**：**`collect_batches`** + **`collect_tasks.batch_id`**；**`POST /api/v1/collect/batches`**（**URL 裁剪/去重、默认最多 50 条 `COLLECT_BATCH_MAX_URLS`、先入队失败后整批回滚**）；**批次列表 / 详情 / 子任务** API；任务列表 **`batchId`** 筛选；**Worker 与各阶段状态变更后以 `GROUP BY status` 重算批次**，**不设并发 +-1**；管理端 **`/collect/batches`**（**5s 轮询**、抽屉内任务列表 + **批次快照刷新**）；操作日志 **`collect.batch.create`** / **`collect.batch.retry_failed`**；**.env.example** 补 **`COLLECT_BATCH_MAX_URLS`**；**§7/§8** 对齐下一步与遗留 |
| 2026-05-17 | **商品刊登基座**：**`internal/modules/productpublish`** + **`PRODUCT_PUBLISH_QUEUE_*`** + **`main`** Worker / **taskreaper** / **`workers` byType **`product_publish`**；**`/platform/publish-settings`**；管理端 **`/settings/platform-publish`**、**`/product/publish-tasks`**、草稿 **「刊登」Tab**（**`publishConfigSchema`** 对齐 **`shops.ts`**） |
| 2026-05-16 | **管理员登录**：仅 **邮箱或手机号 + 密码**（不再接受用户名）；首启账号通过 **`ADMIN_BOOTSTRAP_EMAIL` / `ADMIN_BOOTSTRAP_PHONE`**（至少一项）配置；`admin_users.username` 为内部不透明 ID；`docs/PROGRESS`、`.env.example` 同步 |
| 2026-05-16 | **邮箱注册与通知**：UI 增加登录注册 Tab 切换与设计稿对齐；管理端增加 **Email 邮箱设置** 页并可测试连接（`test-email`）；后端实现 **Email Provider（SMTP）** 与 settings 写入，密码 AES-GCM；扩展 `admin_users` 邮箱与 **`account`** 登录链路；验证码限流与 TTL；注册入库并自动登录 |
| 2026-05-16 | **管理端 UI**：Ant Design 主题与 **mix 布局**（顶栏+侧栏）、登录分区样式、工作台快捷入口；各页去掉冗余说明与 Alert；与 PROGRESS 同步 |
| 2026-05-16 | **采集任务异步化**：Redis **`collect:tasks`**（`COLLECT_QUEUE_*`）；**Worker** 消费、`RunCollectJob`、**`operationlog.WriteBackground`**；`POST /collect/tasks` **非阻塞**；**retry 再入队**；**503** `Redis queue unavailable`；**main** 优雅关闭；管理端 **轮询** 与文案；**`ImportDraftWithContext`**；PROGRESS 同步 |
| 2026-05-16 | **1688 Collector 结构化解析**：`collector/src/providers/source1688/` 分拆 **parser/selectors/utils**；抽取 **标题/主图(≤10)/详情图(≤30)/attributes/skus**（`properties` 兼容 Go **`BuildImportSKU`**），**SKU 粒度 `raw`**；**顶层 `raw`** 结构化（候选图/属性/SKU、`pageMeta`、`extractedAt`、snippet 摘要，**不含整 HTML**）；**`pnpm collect:test`**；验证码且零字段时 **`INVALID_URL`**；PROGRESS §4.3/遗留/下一步更新 |
| 2026-05-15 | **商品详情编辑增强**：后端 **`PUT /products/:id`**（camelCase/snake_case、**status 枚举**、不写 source/raw）；**SKU / images / reorder API**；**操作日志 `product.sku.*` `product.image.*`**；前端 **`DraftDetail`** **Tabs + 图片 ModalForm + 可编辑 SKU**；采集入库详情图 **`detail`**；**PROGRESS** 同步遗留与下一步 |
| 2026-05-15 | 初版：记录地基进度、admin/collector/backend 基线与决策 |
| 2026-05-15 | **本地开发规则**：新增 **`.cursor/rules/11-local-dev-postgres.mdc`**（alwaysApply），同步 `.cursorrules` / `00` / `01` / `08` / `09` 中数据库表述为 **PostgreSQL 默认** |
| 2026-05-15 | **默认数据库改为 PostgreSQL**（compose、`.env.example`、`DB_DRIVER` 默认）；MySQL 仍可选 |
| 2026-05-15 | **管理端**：登录页（`/user/login`）、JWT 存储与 **Bearer** 拦截、**401** 回登录、**access**；系统/AI/存储/采集/安全设置接 **`GET/PUT /api/v1/settings`**；**test-ai / test-storage** 按钮；**后端**新增两测试接口与 **PlainByGroup** 解密探测（OpenAI 兼容最小 chat 请求；本地目录读写校验） |
| 2026-05-15 | **操作日志**：`operation_logs` + **`GET /api/v1/operation-logs`**；登录/失败、logout、改 settings、test-ai、test-storage 落库；**JWT** 写入 **username** 上下文；管理端 **操作日志 ProTable** |
| 2026-05-20 | **自定义链接采集器状态为基础可用（beta）**：采集中心 / 采集设置状态文案 **「基础可用」**（`status=beta`，区别于速卖通「测试中」与 1688「已可用」）；卡片说明与能力标签（含 **商品价格**，不含 SKU/库存）；无规则时「开始采集」提示创建规则或 AI 生成；Modal 增加基础信息采集说明；Collector/Go fallback **`features` 含 `price`**；**PROGRESS** 对齐边界与建议 |
| 2026-05-20 | **自定义链接采集器质量增强**：标题可信度检测与疑似错抓提示；价格/币种归一化（`price`/`currency` 分离，`raw.priceText`）；主图懒加载多属性/srcset/JD 大图归一与过滤；详情图滚动加载；attributes **pairs/row/text_all**；规则测试 **qualityScore**；AI Prompt 迁移强化；入库纠正 currency 误写并合成默认 SKU 价格；管理端规则测试与草稿页提示；**SKU/库存/动态价格不保证**；京东等建议专用 Provider |
| 2026-05-15 | **存储与文件**：**Storage Put/GetURL/Delete**、**local Provider**、**`files` 表**、**`/api/v1/files/upload|list|delete`**、**`GET /static/*`**；**`UPLOAD_MAX_MB`**；管理端 **文件管理**、**存储页上传测试**；admin 代理 **`/static`**；**`.env.example`** 补充上传配置 |
| 2026-05-15 | **商品草稿 + 采集闭环**：`products` / `product_images` / `product_skus`、`collect_tasks`（JSONB）；商品 CRUD 与采集 API；**Go Collector HTTP 客户端**（`COLLECTOR_BASE_URL`、`COLLECTOR_TIMEOUT_SECONDS`）；归一化结果入库与操作日志；管理端 **商品列表/详情**、**采集表单 + 任务表 + 重试**；`.env.example` 补充 Collector 编排变量 |
| 2026-05-15 | **AI 文本（第 3 阶段主线）**：`providers/ai` Gateway + **openai_compatible**；**`ai_prompts`/`ai_tasks`**、默认 **product_title_optimize**；商品 **optimize-title / apply-ai-title / ai/tasks** API；管理端 **`/ai/prompts`** 与详情页 **AI 标题**；操作日志 **ai.title_*** |
| 2026-05-15 | **AI 描述**：默认 **`product_description_generate`**；**`POST .../ai/generate-description`**、**`POST .../apply-ai-description`**；**`ai_tasks.task_type=product_description_generate`**；商品详情 **AI 描述** 区块；操作日志 **`ai.description_generate.*` / `ai.description.apply`**；**PROGRESS** 同步遗留与下一步 |
| 2026-05-15 | **全局 AI 任务**：**`GET /api/v1/ai/tasks`**（分页筛选，列表无大体量 JSON）、**`GET /api/v1/ai/tasks/:id`**（详情 **input/output/rawResponse** + 敏感键脱敏）；管理端 **`/ai/tasks`**、**`services/aiTasks.ts`**；**PROGRESS** 更新下一步与遗留对齐 |
| 2026-05-29 | **AI 图片任务配置统一**：AI 图片任务配置逻辑统一走设置页；新增 OCR 配置区（支持 PaddleOCR、AI 视觉 OCR 等）；新增局部擦除配置区（支持 ComfyUI 等）；图片文字翻译读取用户配置；未配置时提供友好提示和降级策略；简化任务弹窗，折叠高级选项。 |
| 2026-05-29 | **图片 AI OCR 配置优化**：OCR 配置统一留在图片 AI 设置；图片文字翻译读取用户 OCR 配置；支持 PaddleOCR 与 AI 视觉 OCR 兜底，百度/阿里云/腾讯云 OCR 预留；新增 OCR 测试连接，校验连通、blocks 与 bbox。 |
| 2026-05-29 | **图片文字翻译生产级排版质量收紧**：新增 OCR block 分类、`compact_translation`、badge 宽高硬限制、绘制前布局模拟与绘制后质量字段；异常 badge、文本重叠、背景补丁、原文残留和版面失衡统一标记 `low_quality`；管理端低质量结果不再推荐保存或设为主图/详情图。 |
| 2026-07-20 | **Phase P7-V2-R3B Dedicated Benchmark Host Contract tooling**：关闭已消耗的 Host Isolation V3 不完整矩阵为 `invalid_incomplete`（C2/B2 不得补跑，不能用于 Formal Plan）；新增 Dedicated Benchmark Host Contract V1 preflight、host gate、validation gate 与 15 项 fixture。当前 Windows/共享工作区不执行 dedicated-host B-C-C-B 矩阵、Formal Plan、Runtime Freeze、Soak、Demo、Tag 或生产就绪声明；P7-V2 仍 Incomplete。 |
| 2026-07-14 | **Phase P7-V2-R2 scoped closure**：修复 Performance Bootstrap / auth 不稳定；auth/route probe、bootstrap 幂等、auth stability 3/3、diagnostic load、formal baseline（`p7v2-baseline-20260714181000`）均 passed；保留失败 baseline `p7v2-baseline-20260714143530` / `p7v2-baseline-quick` / `p7v2-baseline-20260714180000`；证据 `docs/P7_V2_R2_*`；Current/Regression/Soak/Demo/Final Gates 未执行。 |
| 2026-07-14 | **Phase P7-C4-R 隔离环境清理与最终关闭**：精确删除遗留库 `trademind_p7c4_p7c4_20260714042442`，`trademind_p7c4_%` 前缀 **0 残留**；增强 stop 脚本与 P7-C4 Gate（前缀校验、证据新鲜度、live 查询，Gate 不自动删库）；P7-C4/C3/C2/C Gate **failed=0**；**Phase P7-C4 Completed** · **Ready for P7-V2** · P7 Closure 仍 Incomplete。 |
| 2026-07-14 | **Phase P7-C4 能力收口**：Task Center 九源 SQL keyset + 有界多路归并 + Signed Merge Cursor；修复 `failureRowFilter` OR 优先级导致 keyset 失效、Go driver 毫秒截断（`::timestamptz` 字符串比较）；隔离 Medium PostgreSQL 六类 Pagination / Query Plan / N+1 Runtime 通过；Provider 并发+自适应降速、Permission Cache 全写路径失效、WSL2 增量 Race 通过；`mandatoryPartial=0` / `mandatoryMissing=0`；P7-C4/C3/C2/C Gate 通过；Load/Soak/Baseline 仍 **pending P7-V2**；**非 Production Ready**。 |
| 2026-07-15 | **Phase P7-V2-R3B-PRR-A read-only diagnostic closure**：Recovery3 artifact integrity 和 Comparability V2 保持 passed；保存 Regression V2 失败证据。三项 p95 均完成原始指标审计：Task List、Webhook Ingestion 为 `statistical_variance_insufficient_evidence`（low），Auth/Security 为 `metric_tag_aggregation_bug`（high）。九个 p99 均为 `summary_stat_missing` 被 parser 默认成零。下一阶段为 **P7-V2-R3B-PRR-REPRO**，必须诊断性 Recovery4；Regression V2 仍 failed，Soak/Demo/Final Gates 未执行。 |
| 2026-07-19 | **Phase P7-V2-R3B Formal Host Isolation V2 harness repair checkpoint**：基于 `af9d6fe8ee20e2cf122d4a38b8f9d750782a0d42` 的隔离 worktree 新增 `formalHostIsolationVersion=2` 合同、生命周期对称哈希、dataset barrier、deterministic warmup/cooldown、host quiet window、PG-2 dedicated PostgreSQL isolation contract、background/evidence IO gates、Comparability V5 / runtime freeze binding；旧 B-C-C-B 矩阵仅用于子根因归类，结论为 `A6_multiple_harness_isolation_factors`（secondary: A2/A3/A4/A5, confidence=medium）。Host Isolation Final Gate passed；post-repair B-C-C-B validation matrix、new formal plan/runtime freeze/formal pair/soak/demo/final close 尚未执行，P7-V2 仍 Incomplete，非 Production Ready。 |
| 2026-07-20 | **Phase P7-V2-R3B Host Isolation V3 bounded repair checkpoint pending validation**：冻结失败 V2 validation matrix（`p7v2-diag-host-isolation-validation-20260719061648`，`validForFormalPlan=false`、`runIdsConsumed=true`），新增 C1/C2 current-self variance audit，失败 metrics 为 `Webhook Ingestion p99` 与 `Auth Invalid Login p99`；binary/input/branch mix 均一致，primary root cause 归类为 `V3_E_quiet_window_not_predictive_of_measurement_stability`（secondary: `V3_H_insufficient_evidence_from_completed_validation_matrix`，confidence=medium）。V3 合同升级为 `formalHostIsolationVersion=3`，新增 predictive host stability barrier、PG pid/port/dataDir/WAL dir/cluster identity distinct gate、actual lifecycle sequence gate 和 V3 repair final gate；业务运行时、阈值、VUs、stages、duration、dataset、input sequence、branch mix 均未改变。Fresh V3 B-C-C-B validation matrix、validation gate、cleanup evidence checkpoint 尚未完成；不得创建 formal plan/runtime freeze/formal pair，P7-V2 仍 Incomplete，非 Production Ready。 |
| 2026-07-20 | **Phase P7-V2-R3B Host Isolation V3 validation stopped / execution blocked**: Fresh fixed-order matrix `p7v2-diag-host-isolation-v3-validation-20260720054828` was executed from repair checkpoint `7fb5d481196e799de4268af2e0f32fd6d1178078` in order B1 -> C1 -> C2 -> B2 with no fifth run. B1 and C1 completed; C2 stopped before measurement with `dataset post-build barrier failed`; matrix status is `invalid_incomplete`, runCount=2, `validForFormalPlan=false`, and validation final gate failed=8 for incomplete run count/slots, incomplete PG distinctness, and `validForFormalPlan`. B1/C1 binary, input, and branch-mix bindings passed; predictive host stability and quiet-window readiness passed; dedicated PostgreSQL pid/port/dataDir/WAL dir/cluster identity were distinct for completed runs. Cleanup summary confirms current V3 dedicated resources at diagnosticDatabaseCount=0, diagnosticConnectionCount=0, listener18080Count=0, validationPostgresListenerCount=0, and relatedProcessCount=0. Stop per upper bound: **Phase P7-V2-R3B Execution Blocked / blocked_on_benchmark_environment_repeatability / require dedicated benchmark host**. No V4/V5, formal plan, runtime freeze, formal pair, soak/demo/final race, tag, push, or release; P7-V2 remains Incomplete and not Production Ready. |
**Stage update**: 2026-07-20 — **Phase P8 Batch 3 State Machine / Draft Version / Approval Services Completed / P8 In Progress**：完成 `P8-201 Task State Machine`、`P8-202 Draft Version Service`、`P8-203 Approval Service`。在 `backend/internal/modules/operationtask` 新增 `TaskStateMachine`、`TaskTransitionService`、`DraftVersionService`、`ApprovalService` 与 Canonical JSON SHA-256 payload hash v1；状态迁移、草稿版本创建/编辑、审批/拒绝均保持 tenant、revision、idempotency、event append 和事务回滚保护。新增 [`P8_TASK_BATCH_3_STATE_DRAFT_APPROVAL_SERVICES.md`](P8_TASK_BATCH_3_STATE_DRAFT_APPROVAL_SERVICES.md)、[`p8-task-batch-3-state-draft-approval-services.json`](p8-task-batch-3-state-draft-approval-services.json)、`scripts/p8-task-batch-3-final-gate.mjs` 与 `tests/gates/p8/task-batch-3.mjs`。P8 Batch 1 Completed: `P8-101`、`P8-102`、`P8-106`；P8 Batch 2 Implementation Completed: `P8-103`、`P8-104`、`P8-105`；P8 Batch 3 Implementation Completed: `P8-201`、`P8-202`、`P8-203`。Batch 3 Source Status: `workingBranch=dev`、`committed=false`、`checkpointStatus=not_created_by_owner_instruction`、`workingTreeDirty=true`。本批未实现执行编排、重试服务、执行幂等服务、API、Admin UI、平台适配器或真实平台写入。下一批建议：`P8-204 Execution Orchestrator`、`P8-205 Retry / Failure Service`、`P8-206 Idempotency Protection`。状态：**P7 Conditionally Closed** · **P8 In Progress** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **P7 deferred performance preserved** · **P10 production boundary preserved**。
**Stage update**: 2026-07-21 — **Phase P8 Batch 4 Execution / Retry / Idempotency Services Completed / P8 In Progress**：完成 `P8-204 Execution Orchestrator`、`P8-205 Retry and Failure Service`、`P8-206 Idempotency Protection`。在 `backend/internal/modules/operationtask` 新增 `ExecutionOrchestrator`、`DraftExecutionPort`、`ExecutionFailureClassifier`、`ManualRetryService` 与执行幂等协调；执行流程拆分为 Prepare 事务、事务外 Port 调用、Finalize 事务，严格绑定最新已批准草稿版本和 payload hash。新增 [`P8_TASK_BATCH_4_EXECUTION_RETRY_IDEMPOTENCY_SERVICES.md`](P8_TASK_BATCH_4_EXECUTION_RETRY_IDEMPOTENCY_SERVICES.md)、[`p8-task-batch-4-execution-retry-idempotency-services.json`](p8-task-batch-4-execution-retry-idempotency-services.json)、`scripts/p8-task-batch-4-final-gate.mjs` 与 `tests/gates/p8/task-batch-4.mjs`。P8 Batch 1 Completed: `P8-101`、`P8-102`、`P8-106`；P8 Batch 2 Implementation Completed: `P8-103`、`P8-104`、`P8-105`；P8 Batch 3 Implementation Completed: `P8-201`、`P8-202`、`P8-203`；P8 Batch 4 Implementation Completed: `P8-204`、`P8-205`、`P8-206`。Source State: `branch=dev`、`committed=false`、`checkpointStatus=not_created_by_owner_instruction`。本批未新增 API、Admin UI、消息队列 Worker、定时任务、后台自动重试、真实平台 Adapter、真实平台写入、自动发布或自动上架。下一批建议范围：`P8-301 Platform Draft Interface`、`P8-302 Local Draft Adapter`、`P8-303 Douyin Mock/Sandbox Adapter`、`P8-304 Unsupported Platform Guard`、`P8-305 Automatic Publish Guard`；本轮未启动下一批。状态：**P7 Conditionally Closed** · **P8 In Progress** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **P7 deferred performance preserved** · **P10 production boundary preserved**。
**Stage update**: 2026-07-30 — **商品-货源档案 + 采购协同（人工下单过渡版）模块交付（本地分支，未推远端）**：新增后端模块 `sourcing`（suppliers / product_sources / product_source_skus / source_price_history / source_switch_events，主/备货源、SKU 映射、历史进价、断货自动切换、涨价建议、锁定不切换）与 `procurement`（purchase_orders / purchase_order_items / purchase_order_events / purchase_logistics，销售订单按主货源聚合生成采购清单、CSV 导出、人工回填 1688 订单号/运单号、状态机 draft→pending_confirm→placing→placed→paid→shipped→delivered + failed/cancelled）；新增 Provider 抽象 `providers/sourceinfo`（Mock 报价）与 `providers/trade`（Mock1688，人工下单过渡；官方 API 暂不可用）；管理端新增 `/sourcing/suppliers`、`/sourcing/product-sources`、`/procurement/orders` 页面与 services；docs/api.md、docs/provider.md 已同步；后端单测覆盖绑定去重、主源切换、锁定、历史进价、切换规则、聚合幂等、状态机与 CSV 导出。
**Stage update**: 2026-07-26 — **Phase P8 Batch 8 Admin Operation Task Center Completed / P8 In Progress**：完成 `P8-601 Task List`、`P8-602 Task Detail`、`P8-603 Draft Preview / Edit`、`P8-604 Approval Actions`、`P8-605 Execution / Retry Actions`、`P8-606 Audit Timeline`。Admin 新增 `/ops/task-center/operation-tasks` 与详情页，复用 Batch 7 `/api/v1/operation-tasks` API、现有 request envelope、菜单权限、URL state、TmPageContainer/TmProTable/SectionCard/TaskJsonBlock；按钮状态来自后端 `allowedActions`，写请求带 `Idempotency-Key`，冲突刷新最新数据，审计 metadata 白名单与敏感键脱敏。新增 [`P8_TASK_BATCH_8_ADMIN_OPERATION_TASK_CENTER.md`](P8_TASK_BATCH_8_ADMIN_OPERATION_TASK_CENTER.md)、[`p8-task-batch-8-admin-operation-task-center.json`](p8-task-batch-8-admin-operation-task-center.json)、`scripts/p8-task-batch-8-final-gate.mjs` 与 `tests/gates/p8/task-batch-8.mjs`。Source State: `branch=dev`、`committed=false`、`checkpointStatus=not_created_by_owner_instruction`。本批未新增真实 Douyin API/OAuth、真实凭证、真实平台写入、自动发布、自动上架、后台自动重试、定时执行、生产队列 Worker、生产灰度、Tag、Release 或 Production Ready。下一批建议范围：`P8-701 Integration Fixtures`、`P8-702 API/Admin E2E Fixtures`、`P8-703 Platform Boundary Gate`、`P8-704 P8 Final Gate`、`P8-705 Closure Evidence`。状态：**P7 Conditionally Closed** · **P8 In Progress** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **P7 deferred performance preserved** · **P10 production boundary preserved**。
**Stage update**: 2026-07-28 — **Phase P9 Canonical Scope Resolved / Owner Scope Decision Approved / Full Implementation Plan Ready / Batch 1 Scope Ready**：从当前 `dev` 分支与真实 `HEAD` 重新完成 P9 发现，归一为 **Douyin Shop inventory sync MVP with SKU binding calibration and manual binding fallback**。仓库内 P9 参考被分为 current / historical / completed / conflicting 四类，当前无冲突，历史 Phase 9 继续保留为实现证据而不直接复用为当前任务名。新增 [`P9_OWNER_SCOPE_DECISION.md`](P9_OWNER_SCOPE_DECISION.md) / [`p9-owner-scope-decision.json`](p9-owner-scope-decision.json)、[`P9_SCOPE_DISCOVERY.md`](P9_SCOPE_DISCOVERY.md) / [`p9-scope-discovery.json`](p9-scope-discovery.json)、[`P9_EXECUTION_PLAN.md`](P9_EXECUTION_PLAN.md) / [`p9-execution-plan.json`](p9-execution-plan.json)、[`P9_TASK_BATCH_1_SCOPE.md`](P9_TASK_BATCH_1_SCOPE.md) / [`p9-task-batch-1-scope.json`](p9-task-batch-1-scope.json)、`scripts/p9-entry-gate.mjs`、`tests/gates/p9/entry.mjs`、`scripts/p9-plan-final-gate.mjs`、`tests/gates/p9/plan.mjs`、`scripts/p9-task-batch-1-scope-gate.mjs`、`tests/gates/p9/task-batch-1-scope.mjs`。状态：**P9 Scope Resolved** · **P9 Owner Scope Decision Approved** · **P9 Execution Plan Ready** · **P9 Batch 1 Scope Ready** · **P9 Implementation Not Started** · **Production Ready: No** · **realCredentialsEnabled=false** · **realPlatformWriteEnabled=false** · **automaticPublishEnabled=false** · **automaticListingEnabled=false** · **P10 boundary preserved**。
**Stage update**: 2026-07-29 — **Phase P9 Batch 4 Permissions / Audit / Safety Completed / P9 In Progress**：完成 `P9-801 Inventory Sync RBAC`、`P9-802 Inventory Sync Audit Events`、`P9-803 Inventory Metadata Redaction`、`P9-804 Provider and Production Boundary Guard`。库存同步、人工重跑和人工绑定确认/拒绝接入现有 `adminperm` RBAC、可信 Actor、租户隔离和默认拒绝；同步生命周期、权限拒绝、生产能力阻断、人工绑定确认/拒绝复用 `operationlog` 做事务性安全审计；Provider Metadata、同步错误、Cursor、审计 Metadata 和人工绑定评论接入 `safefields` 脱敏/allowlist；真实凭证、真实网络、真实平台读写、真实库存读写、库存修改、自动执行和自动重试继续显式阻断。新增 [`P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY.md`](P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY.md)、[`p9-task-batch-4-permissions-audit-safety.json`](p9-task-batch-4-permissions-audit-safety.json)、[`P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY_GATE.md`](P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY_GATE.md)、[`p9-task-batch-4-permissions-audit-safety-gate.json`](p9-task-batch-4-permissions-audit-safety-gate.json)、`scripts/p9-task-batch-4-permissions-audit-safety-gate.mjs` 与 `tests/gates/p9/task-batch-4-permissions-audit-safety.mjs`。验证：`go test ./backend/internal/pkg/adminperm ./backend/internal/modules/inventorysyncp9 ./backend/internal/config -count=1` passed；`go test -race ./backend/internal/modules/inventorysyncp9 -count=1` passed；P9 Batch 1-4 fixture/gate passed；`TEST_DATABASE_URL=not_set`，PostgreSQL 集成继续 `not_run` 并 defer 到 `P9_Final_Development_Closure`，`p9FinalClosureBlocker=true`。本批未实现 API、Admin UI、后台 Worker、Cron/Ticker、Queue Consumer、自动重试 Worker、真实 Douyin Provider、OAuth、真实凭证、真实网络或库存修改。下一批建议范围：`P9-901`～`P9-905 Backend APIs`，当前 `notStarted`。状态：**P9 In Progress** · **P9 Batch 4 Completed Locally** · **P9 Final Development Closure Blocked Pending PostgreSQL Verification** · **Production Ready: No** · **P10 boundary preserved**。
**Stage update**: 2026-07-29 — **Phase P9 Batch 5 Backend APIs Completed Locally / PostgreSQL Deferred**：完成 `P9-901` 至 `P9-905`。新增受保护的 fixture/mock-only inventory sync API：run、snapshot、SKU binding/calibration、manual binding、history/audit；统一复用认证/租户/RBAC、严格 JSON body、Idempotency-Key、Request ID、签名 keyset cursor 与安全 DTO。受控 recalibration 生成不可变新版本；无 Admin UI、真实 Douyin/OAuth/凭证/网络、库存 mutation、worker/cron/ticker/queue/自动 retry。新增 [`P9_TASK_BATCH_5_BACKEND_APIS.md`](P9_TASK_BATCH_5_BACKEND_APIS.md)、JSON、gate script 与 fixture。`TEST_DATABASE_URL` 未设置，PostgreSQL integration `not_run`，保留 P9 final closure blocker；当前 `dev` 未提交、未暂存、非 Production Ready，P10 boundary 保持。
## P9 PostgreSQL Integration Baseline（2026-07-30）

```text
P8 — Development Complete
P9 — In Progress
P9 Product Batch 1 — Completed
P9 Product Batch 2 — Completed
P9 Product Batch 3 — Completed
P9 Product Batch 4 — Completed
P9 Product Batch 5 — Completed
P9 PostgreSQL Integration Baseline — Passed
P9 Product Batch 6 — Ready to Start (Not Started)
productionReady=false
Tag deferred
Final Production Acceptance Deferred to P10
```

权威运行证据：`artifacts/p9-postgres-runtime.json`（run `p9pg-20260730074632-3b1bbb38`）。PostgreSQL 专项测试 fail-closed，不回退 SQLite；本轮未实现 Admin UI、真实 Provider、OAuth、后台 Worker 或库存写入。

### 变更记录（2026-07-30）AI 比价选品引擎（线B）

- 新增 `backend/internal/modules/selection`：`selection_tasks` / `selection_candidates` / `selection_source_matches` / `selection_evaluations` 四表 + Redis 队列（`selection:tasks`）异步 Worker（复用 tasklease 租约/心跳/重试）+ REST API（见 docs/api.md「AI 比价选品引擎 API」）。
- 新增 Provider 抽象：`providers/marketprice`（海外在售价，mock）、`providers/sourcematch`（1688 同款：mock + collector 爬虫兜底 + open1688 官方 API 空壳）、`providers/fx`（汇率）、`providers/logistics`（物流线性报价）。
- 利润模型 `selection/profit.go`：汇率×物流×佣金×退货率×采购价 → 落地成本/预期利润/利润率，参数走 settings `selection` 分组 + 每任务 params 覆盖；完整单元测试。
- LLM 打分走既有 `providers/ai.Gateway` + `ai_prompts`（code `selection_scoring`），AI 不可用时规则兜底评分。
- Admin 新增 `/selection/tasks` 选品任务页与 `/selection/tasks/:id` 可上架清单页（排序展示、人工审核、一键转商品草稿进入既有刊登链路）。
- 边界：未改动货源档案/采购协同（source/procurement）模块；1688 官方 API 保持空壳。

### 变更记录（2026-07-29）迭代第 1 轮：采购批量回填 + 待办导向首页驾驶舱

- 采购批量回填：新增 `POST /procurement/orders/batch-mark-placed`（按采购单 ID 批量回填 1688 订单号）与 `POST /procurement/orders/batch-logistics`（按 1688 外部订单号匹配采购单批量回填运单号，placed 状态自动先 mark-paid），单批 ≤200 行、逐行独立处理、部分成功不回滚、重复行/未知单号/非法 ID 逐行报错，附 service 层单测（`procurement/batch_test.go`）。
- Admin 采购页新增「批量回填 1688 订单号 / 快递单号」粘贴式弹窗：多行文本解析（空格/逗号/Tab 分隔）、采购单 ID 支持完整 UUID 或唯一前缀（对当前「下单中」采购单解析）、客户端格式校验 + 服务端逐行结果表，解析逻辑独立为 `batchParse.ts` 并附单测；采购列表页支持 `?status=` URL 初始筛选（配合首页待办跳转）。
- 待办导向首页：dashboard 新增选品/货源/采购统计（选品待审核、选品失败、货源涨价/断货、采购待确认/待下单/待付款/待回填运单），统一待办流与首页「今日待办」新增对应卡片；首页待办改为只展示有数量的待办并按 P0/P1 + 数量排序，全空时给出下一步引导空状态。
- 同步 `docs/api.md`、`admin/src/services/{procurement,dashboard}.ts`、`admin/src/constants/dashboardDefaults.ts`。

### 变更记录（2026-07-29）迭代第 2 轮：采购单参考价闭环

- 生成采购单时 SKU 映射缺参考价自动回退到最近一条历史进价（`source_price_history`）；仍缺价时采购单照常生成，但以 `warnings`（`price.missing`）逐行提示，避免 0.00 金额误导。
- 新增 `PUT /procurement/orders/:id/items/:itemId/price`：draft / pending_confirm 状态下补填/修改明细参考价并重算 `totalAmount`，记录操作日志；附 service 层单测（历史价回退、缺价告警、改价重算与状态限制）。
- Admin 采购单详情：明细「参考价」缺失时显示「缺参考价」标记，可编辑状态下行内「填价」直接补填；详情顶部聚合缺价行提示；生成采购清单弹窗展示缺价 warnings。
- 同步 `docs/api.md`、`admin/src/services/procurement.ts`。

### 变更记录（2026-08-01）迭代第 13 轮：订单成本/毛利估算

- 新增 `GET /api/v1/procurement/cost-estimates/:id`（id 为销售订单）：按主货源 SKU 映射参考价（缺价回退最近历史进价，与生成采购单同口径）逐行估算 CNY 采购成本；订单币种为 CNY（汇率恒 1）或已配置 `settings.pricing.exchangeRate`（CNY→订单币种）时折算 `estimatedCost` 并计算 `grossProfit`/`marginPercent`（任一行缺价时不计算毛利）；问题行以 `issueCode`（`sku.unmatched`/`source.missing`/`mapping.missing`/`price.missing`）返回。procurement.Service 新增 `SettingsReader` 依赖（接口解耦），附 sqlite 单测（含汇率折算、缺价、CNY 订单三条路径）。
- Admin 订单详情「订单概览」新增「成本 / 毛利估算」卡：销售额、预估采购成本（CNY + 折算）、预估毛利（正绿负红）、毛利率；缺价行与未配置汇率分别以 Alert 提示。
- 顺带闭环第 12 轮 P3：库存手工扣减/回滚成功 toast 不再直出后端摘要原文 `ok`，改为结构化中文（成功/幂等跳过 + 原因）。

### 变更记录（2026-08-01）迭代第 47 轮：商品草稿 / 货源档案 / 设置页移动端走查修复

- 五档视口（375/768/1024/1280/1440）走查商品草稿列表、供应商管理、商品货源档案（含 SKU 映射抽屉）与设置全部子页（含用户与权限），按 P0-P2 分级修复。
- 商品草稿列表 375px（P0）：商品图（fixed left）+ 操作（fixed right）双固定列把标题列压到约 20px 不可读；商品图/来源/运营进度/创建时间加 `responsive: ['md']` 小屏折叠，固定列小屏取消 fixed（沿用第 46 轮订单列表模式），小屏保留标题/状态/操作。
- 全站页头标题窄屏截断（P1）：`TmPageContainer` 标题与描述同行导致 375px 下标题省略成「商…」「用户与…」；`global.less` 新增 <768px 规则让 heading-left 换行、标题/副标题整行正常换行展示，桌面端不变。
- SKU 映射抽屉（P1）：固定 `width={720}` 在 375px 下表格内输入框被压到不可用；改 `width="min(720px, 100vw)"` + 内表 `scroll={{ x: 640 }}` 并给货源规格 ID 列固定宽度，小屏横向滚动可正常录入。
- 商品货源档案（P1）：商品选择器固定 360px 超出小屏内容区，改 `min(360px, calc(100vw - 48px))`；切换审计表加 `scroll={{ x: 900 }}` 消除表头挤压；历史进价弹窗宽度改 `min(640px, calc(100vw - 32px))`。
- 供应商管理 / 用户与权限（P1）：次要列（外部ID/评分/备注；邮箱/手机/授权店铺/最近操作）加 `responsive: ['md']` 小屏折叠，小屏无需横向滚动即可触达操作列。
- 纯前端改动，无 API / 权限 / 状态机变更；修复后五档视口回归 95 项检查根节点均无横向溢出，1440px 桌面端无回归。

### 变更记录（2026-08-01）迭代第 46 轮：首页新手入门引导卡 + 订单列表 375px 移动端优化

- 首页驾驶舱顶部新增「新手入门」引导卡（第 45 轮 UX 走查 Top4）：按业务闭环列 6 步（配置 AI 与存储 → 采集商品 → 完善草稿与货源 → 导入订单 → 生成采购单 → 发货），每步直达对应页面；用既有列表/设置 API（settings、collect/tasks、products、suppliers、orders、procurement/orders）计数自动标记已完成步骤（不新增后端接口），带完成进度条；可关闭，关闭状态经 `localStorage`（`tm_dashboard_onboarding_dismissed_v1`）持久化。
- 订单列表 375px 优化（第 45 轮 UX 走查 Top5）：次要列（外部单号/平台/店铺/客户/支付/商品数/规格匹配/库存扣减/同步/异常/预估毛利/物流/三个时间列）加 `responsive: ['md']` 在 <768px 折叠，小屏默认仅保留订单号/订单状态/金额/操作四列（复现时 20 列 scrollWidth 2382px vs 视口 295px，修复后无横向滚动）；操作列小屏取消 fixed 并收窄，完整信息进订单详情查看；桌面端列集与交互不变（沿用第 26 轮异常工作台 `Grid.useBreakpoint` + 表格 key 重挂载模式）。
- 纯前端改动，无 API / 权限 / 状态机变更。

### 变更记录（2026-08-02）迭代第 52 轮：库存中心 UX 走查与修复

- 库存流水可追溯性：`GET /api/v1/inventory/logs` 行内补 `productId/productSkuId/productTitle/skuCode/skuName/refOrderNo`（service 层批量 enrich，无 N+1，不改状态机/权限）；前端流水页新增「商品」（直达商品库存 Tab）与「商品规格」列，关联订单显示订单号并直达订单详情。
- 流水页体验：变更类型筛选改中文 select、原因列中文化（`INVENTORY_CHANGE_REASON`）、接入共享空态（`inventoryLogs` 文案）、宽表横向滚动；库存中心行操作新增「流水」直达链接；扣减记录页补横向滚动。
- readonly 口径：库存预警（调整库存/预警线/批量设置预警线/批量同步库存/同步库存）、同步任务（重试/批量重试）、同步批次（重试失败）写入口按 `canWriteInventory` 隐藏/禁用。
- 新增 `admin/e2e/specs/inventory-center.spec.ts`（17 用例：列表可读性、流水可追溯、空态、五档视口、手工调整写请求拦截 0/1/防重复、readonly 口径）；修正 e2e mock 用户 `permissions: ['*']` 为 `[]`（`hasPermission` 不识别通配符导致写入口在 E2E 中不可见）。
- 已知遗留：`/inventory` 菜单重复 key（PR #94 迁移 `/inventory/center` 根治）、antd `destroyOnClose` 告警（PR #91），console guard 临时放行并注明合并后删除。

### 变更记录（2026-08-02）迭代第 16 轮：全站列表页 params 覆盖问题审计修复

- 按第 15 轮根因对全站 ProTable 列表页做同类问题审计：失败任务（TaskCenter/Failures）、商品草稿（Product/Drafts）、客服会话（Customer/Conversations）三页仍在 `params` 中透传 URL 筛选值，存在与订单列表相同的「分页点击/表单提交被旧值冲掉」风险。统一改为「URL query 为唯一筛选来源」模式：移除 `params`、新增 `onSubmit` 写回 URL、urlState 变化 effect 触发 reload、`request` 一律读 urlState/legacy URL 值。
- `ALLOWED_QUERY_KEYS` 补 `operationStep`（商品草稿运营进度筛选）与 `customerName`（客服会话买家筛选）；客服会话布尔筛选（待回复/有 AI 建议/发送失败/有关联订单）URL 采用 `1`/`0`，兼容既有 `replyStatus`/`aiSuggestionStatus`/`sendStatus` 深链。
- 修复真实测试发现的既有后端 P1：`GET /api/v1/task-center/failures` 对 tenant>0 用户必报 400（PostgreSQL 42703）。根因是统一的 `tenant_id = ?` 过滤被套到了自身没有 `tenant_id` 列的失败源表上。新增 `applyTenantListFilterVia`：`image_tasks` 经 `products` 限定租户（`product_id IS NULL` 的工具级任务保留可见）、`ai_product_text_items`/`ai_product_image_items` 经各自 batches 表、`customer_failure_events` 经 `shops` 表；附 sqlite DryRun SQL 回归单测。

### 变更记录（2026-08-02）迭代第 22 轮：首页「订单待采购」待办卡 + 订单列表采购覆盖筛选

- `GET /api/v1/orders` 新增可选 `hasPurchase` 三态过滤（`1`/`0`，缺省不过滤），按「任一明细行被未取消/未失败采购单覆盖」判定（与生成采购单防重同一口径的 EXISTS 子查询），并纳入游标分页 scope 指纹。
- 首页/统一待办新增「订单待采购」卡（`order_await_procurement`）：已付款未发货且无采购覆盖的订单数，直达 `/orders/list?payStatus=paid&hasPurchase=0`，补齐每日「出单 → 采购」漏斗的入口缺口。
- 订单列表新增「采购覆盖」查询项（已生成/未生成采购单），URL query 单一来源模式，`ORDER_QUERY_KEYS` 补 `hasPurchase`（沿用第15/18轮 allowlist 经验）。

### 变更记录（2026-08-02）迭代第 34 轮：订单批量标记送达 + 首页「订单在途待送达」待办卡

- 订单列表批量操作条新增「批量标记送达（N）」：仅统计所选 `status=shipped` 订单，逐单复用既有 `PUT /orders/:id`（`status=delivered` + `deliveredAt`），闭环销售订单生命周期最后一步（导入 → 付款 → 采购 → 发货 → 送达）。
- 首页/统一待办新增「订单在途待送达」卡（`order_in_transit`）：已发货订单数，直达 `/orders/list?status=shipped`；前端 `DEFAULT_TODOS` 同步补 key（沿用第15/18/22轮 allowlist 经验）。

### 变更记录（2026-08-02）迭代第 36 轮：前端静态资源 contenthash + 缓存策略（部署后免硬刷新）

- Admin 构建开启 `hash: true`：`umi.js`/`umi.css` 等产物文件名带 contenthash，部署新版本后浏览器自动加载新资源，消除「重建部署后浏览器缓存旧 CSS/JS 需硬刷新」的问题（第 35 轮 E2E 备注项）。
- `admin/nginx.conf`：`index.html` 设 `Cache-Control: no-cache`（始终拿到最新入口），带 hash 的 `.js/.css` 设一年 immutable 长缓存；`/static/` 前缀改为 `^~` 避免被 js/css 正则 location 抢占（保留本地优先、后端回退）。

### 变更记录（2026-08-02）迭代第 35 轮：订单列表批量操作条移动端可触达（换行修复）

- 订单列表 ProTable 批量操作条（tableAlertOptionRender）`Space` 增加 `wrap`，并在 `global.less` 为 `.ant-pro-table-alert-info` 增加 `flex-wrap: wrap`，修复 375px 窄屏下操作条 nowrap 被 `ant-layout-content overflow:hidden` 裁剪、右侧批量按钮（生成采购单/标记已付款/导出发货清单/标记送达）不可见且无法滚动触达的问题（第 34 轮 E2E P3 观察项）。

### 变更记录（2026-08-02）迭代第 33 轮：订单批量导出发货清单 CSV

- 新增 `GET /api/v1/orders/shipping-list/export.csv?ids=`：合并导出所选销售订单（去重后 ≤50 个）的发货清单（订单号/客户名/电话/商品/SKU/数量/币种/金额），「快递单号(回填)」「承运商(回填)」列留空供线下打单后粘贴回「批量发货」；租户隔离，任一 id 不在租户内返回 404，UTF-8 BOM 兼容 Excel（复用采购清单导出模式，附单测）。
- 订单列表批量操作条新增「批量导出发货清单（N）」，计数与「批量生成采购单」同口径（所选 paid 且非终态订单），导出不清空选择；闭环「待发货订单 → 导出清单打快递单 → 批量发货回填」动线。

### 变更记录（2026-08-02）迭代第 32 轮：订单详情「取消订单」入口 + 订单列表终态行禁选

- 订单详情右上角新增「取消订单」（可写角色且非终态订单，Popconfirm 确认），复用既有 `PUT /orders/:id` 的 `status=cancelled` 更新路径；已扣减库存按既有 `AutoRestoreCancelledOrders` 策略自动回滚，取消后订单自动移出待收款/待采购/待发货待办口径。
- 订单列表行选择排除终态订单（`cancelled`/`refunded`/`closed` 不可勾选），批量标记付款/批量生成采购单的派生集合同步排除终态，防止对已取消订单误操作。

### 变更记录（2026-08-02）迭代第 31 轮：订单列表批量标记已付款 + 首页「订单待收款确认」待办卡

- 订单列表 `unpaid` 状态行可勾选，批量操作条新增「批量标记已付款（N）」，逐单调用既有 `PUT /orders/:id`（与详情页「标记已付款」同一 API），成功/失败逐单汇总；「批量生成采购单（N）」改为只统计并提交所选中已付款订单。
- 首页/统一待办新增「订单待收款确认」卡（`order_await_payment`）：未付款且未取消/退款/关闭的销售订单数，直达 `/orders/list?payStatus=unpaid`；`DEFAULT_TODOS` 同步补 key。
- 订单侧生命周期批量入口闭环：批量导入 → 批量标记已付款 → 批量生成采购单 → 批量发货。

### 变更记录（2026-08-02）迭代第 30 轮：采购单批量标记签收 + 首页「采购单待签收」待办卡

- 采购单列表 `shipped` 状态可勾选，批量操作条新增「批量标记签收（N）」，逐单调用既有 `mark-delivered` API（沿用第11轮签收自动入库与幂等语义），按状态分组计数、逐单汇总失败原因。
- 首页/统一待办新增「采购单待签收」卡（`procurement_await_receipt`）：已发货采购单数，直达 `/procurement/orders?status=shipped`；`DEFAULT_TODOS` 同步补 key（沿用第15/18/22轮 allowlist 经验）。
- 采购批量生命周期入口闭环：提交 → 确认 → 导出 → 标记付款 → 标记签收（自动入库）。

### 变更记录（2026-08-02）迭代第 29 轮：首页经营概览统计

- 新增 `GET /api/v1/orders/stats/sales`：按创建时间统计今日/近7日/近30日订单数、已付款数、已发货数与分币种已付款销售额（租户内、软删订单不计入）；附单测。
- 首页驾驶舱在 KPI 区下方新增「经营概览」卡：三个时间窗口的订单/付款/发货计数与销售额，直达订单列表，回答运营者「今天卖了多少」。
- 同步 `docs/api.md`。

### 变更记录（2026-08-02）迭代第 28 轮：订单详情页删除订单入口

- 订单详情页右上角新增「删除订单」（仅可写角色，Popconfirm 确认），复用既有 `DELETE /api/v1/orders/:id` 软删除，删除后返回订单列表；与订单列表抽屉里的既有删除入口语义一致，闭环「错误导入/测试订单无法从详情页作废」的缺口（第 27 轮 E2E 反馈项）。
- 纯前端改动，无后端 / API 变更。

### 变更记录（2026-08-02）迭代第 27 轮：销售订单批量发货（粘贴订单号+快递单号）

- 新增 `POST /api/v1/orders/shipments/batch`：`{items:[{orderNo, trackingNo, carrier?}]}`（≤200 条），按订单号在租户内匹配已付款销售订单并新增 `shipped` 物流（复用既有 AppendShipment，订单自动流转为已发货）；未付款、已取消/关闭/退款、未找到、多重匹配、重复订单号逐行失败并给出中文原因；附单测。
- 订单列表工具栏新增「批量发货」：粘贴 `订单号 快递单号 [承运商]` 多行文本，行级格式校验 + 逐行结果表，闭环「打完快递单后逐单进详情加物流」的重复操作。
- 同步 `docs/api.md`。

### 变更记录（2026-08-02）迭代第 26 轮：采购单列表批量标记付款

- 采购单列表批量操作条新增「批量标记付款（N）」：已下单（placed）状态可勾选并批量调用既有 `POST /api/v1/procurement/orders/:id/mark-paid`，逐单汇总成功/失败，闭环「1688 付款后逐单点标记付款」。
- 批量提交/确认/标记付款统一抽为 `BATCH_ACTIONS` 配置（文案/空选提示/单条 API），行为不变。
- 纯前端改动，无后端 / API 变更。

### 变更记录（2026-08-02）迭代第 25 轮：采购单批量导出合并采购清单 CSV

- 新增 `GET /api/v1/procurement/purchase-lists/export.csv?ids=`：逗号分隔采购单 UUID（去重后 ≤50），逐单合并明细行为一份采购清单 CSV（「采购单号」列区分来源），复用单单导出的表头/行渲染（`writePORows`），任一 id 不存在返回 404；附单测。
- 采购单列表批量操作条新增「批量导出清单（N）」；可勾选状态扩展为草稿/待确认/下单中(人工)，覆盖「确认后去 1688 逐单导出」场景，多张采购单一次导出一份合并清单。
- 同步 `docs/api.md`。

### 变更记录（2026-08-02）迭代第 24 轮：采购单列表批量提交/批量确认 + 生成结果提示统一

- 采购单列表新增行选择（仅草稿/待确认状态可勾选，只读角色不显示）与「批量提交」「批量确认」操作，按状态分组循环调用既有 `POST /api/v1/procurement/orders/:id/submit|confirm`，逐单汇总成功/失败并弹窗列出失败原因，减少待办卡「采购单待确认」后的逐单点击。
- 采购单列表「从销售订单生成采购单」弹窗的结果提示改用共享组件 `GenerateResultAlerts`，line.covered 已覆盖提示与缺参考进价分组与订单列表/详情统一（修正该页遗留的分组不一致）。
- 状态筛选变化同步写回 URL query（replace），深链/刷新与筛选保持一致。
- 纯前端改动，无后端 / API 变更。

### 变更记录（2026-08-02）迭代第 23 轮：订单列表批量生成采购单

- 订单列表新增行选择（仅已付款订单可勾选，只读角色不显示）与「批量生成采购单」批量操作，复用既有 `POST /api/v1/procurement/orders/generate`（本就支持多 orderIds 且带跨请求防重），选中多单一键生成后自动清空选择并刷新列表。
- 生成结果的 blockers / 已覆盖（line.covered）/ 缺参考进价三组提示抽取为共享组件 `admin/src/components/procurement/GenerateResultAlerts`，订单列表与订单详情共用，消除既有重复。
- 纯前端改动，无后端 / API 变更。

### 变更记录（2026-08-02）迭代第 21 轮：采购单详情反向直达销售订单 + 生成结果弹窗分组

- 采购单详情补「采购 → 出单」反向直达：概览新增「来源销售订单」（去重短 ID 链接直达订单详情），采购明细每行新增「来源订单」列；纯前端复用既有 `purchase_order_items.salesOrderId`，无后端变更。
- 订单详情「生成采购单结果」弹窗按 warning code 分组：`line.covered`（已有采购单覆盖，info 样式）与缺参考进价（warning 样式）分开展示，标题不再混用（第20轮 E2E 反馈项闭环）；纯防重结果不再额外弹「没有可进入采购清单的明细行」toast。

### 变更记录（2026-08-02）迭代第 20 轮：订单详情采购协同直达

- 订单详情补「出单 → 采购」直达闭环（此前需跳到采购协同页在下拉里逐个找订单）：已付款订单右上角新增「生成采购单」按钮（复用 `POST /procurement/orders/generate`，blockers/warnings 以弹窗展示）；概览新增「关联采购单」卡片，展示该订单聚合出的采购单（状态 / 供应商 / 金额 / 1688 订单号，链接直达采购单详情）。
- `GET /procurement/orders` 新增可选 `salesOrderId` 查询参数（经 `purchase_order_items.sales_order_id` 子查询过滤，非法 UUID 返回 400），附单测；既有参数与返回结构不变。
- `POST /procurement/orders/generate` 增加跨请求防重：订单明细行已被未取消/未失败采购单覆盖时跳过并返回 `line.covered` warning，取消原采购单后可重新生成（E2E 发现重复点击会生成重复采购单，修复并附单测）。

### 变更记录（2026-08-02）迭代第 19 轮：订单详情商品明细行内增删改

- 订单详情「商品明细」Tab 补齐明细行编辑闭环（此前仅列表页抽屉可编辑，详情页只读，手工建单后无法从详情页补明细）：可写角色新增「新增明细」按钮与每行「编辑 / 删除」（Popconfirm 确认），弹窗表单数量/单价变更时自动按 数量 × 单价 重算小计（可手工覆盖），保存后刷新详情、规格匹配与成本估算。复用既有 `POST/PUT/DELETE /orders/:id/items` API，无后端与协议变更。
- 订单列表「新建手工订单」弹窗补充提示：商品明细在创建后进入订单详情「商品明细」Tab 添加，成本/毛利估算依赖明细行。

### 变更记录（2026-08-02）迭代第 18 轮：负毛利订单自动拦截 + Admin 静态资源 404 修复

- 订单异常工作台新增聚合异常类型 `negative_margin`（利润为负）：已付款、未发货且未取消/退款/关闭的销售订单，按主货源参考价成本估算（复用 `procurement.Service.EstimateOrderCostBatch`，与订单成本卡/列表毛利列同一口径）预估毛利为负时，以 `sourceType=order` 进入工作台，`orderexception.Service` 通过新增 `Cost *procurement.Service` 依赖复用估算逻辑（router 注入），单次列表最多扫描最近更新的 200 个候选订单。缺参考价/未配汇率的订单不误报（毛利不可算即不判定）。
- 支撑：`summary.negativeMargin` 统计、handled/ignored 标记与详情兼容（`resolveOrderPointers`/`GetOrderExceptionDetail` 支持 `order` source）；Dashboard `summary.negativeMarginOrderCount` + 待办卡/统一待办 `order_negative_margin`（P0，直达 `/orders/exceptions?exceptionType=negative_margin`）；前端异常页新增类型标签、统计卡与「去订单复核」动作；`docs/api.md` 已同步；新增 sqlite 单测覆盖亏损产生/盈利不产生/已履约不产生/忽略隐藏。
- 修复 Admin 构建产物 `/static/*`（如 logo 哈希资源）404：`admin/nginx.conf` 的 `/static/` 改为 `try_files` 先取本地前端构建产物，未命中再回退代理 backend 上传静态文件。

### 变更记录（2026-08-02）迭代第 17 轮：Admin TypeScript 全量错误清零

- 修复 Admin `tsc --noEmit` 全部 24 个既有类型错误并把 `tests/quality/baselines/admin-typescript.json` 基线 ratchet 到 0（CI「Admin TypeScript baseline ratchet」此后任何新增类型错误直接红灯）。均为类型层修正，无运行时行为变化。
- 主要修正：`postJSON`/`putJSON` 泛型默认 body 参数、`getWithParams` params 支持 boolean（订单 hasException）；`AppMessageBridge` 改为逐方法显式补丁（消除 union-of-signatures 不可调用问题）；`typings.d.ts` 补 `*.png` 模块声明；`TranslateLayoutSummary` 补 `renderMode`/`eraseMode`、`TranslateTaskOutput` 补 `resultUnavailable`、去除 imageTasks 重复字面量属性；`OrderSkuMatchListRow` 补 `createdAt`/`updatedAt`；Collect 规则编辑 request 新建分支补全表单字段；`attrsToJSON` 返回类型补 null。

### 变更记录（2026-08-01）迭代第 15 轮：订单列表分页/查询交互修复

- 修复订单列表 UI 点击分页器与「查询」按钮不生效的既有问题（第 14 轮测试发现）：此前 `params` 中透传的 URL 筛选值会覆盖表单/分页的新值。现改为与异常工作台一致的「URL query 为唯一筛选来源」模式：`onSubmit` 把表单值写回 URL、urlState 变化 effect 触发 reload、`request` 一律从 urlState 读筛选；`hasException` 补入 URL keys（深链可用）。

### 变更记录（2026-08-01）迭代第 14 轮：订单列表预估毛利列

- 新增 `POST /api/v1/procurement/cost-estimates/batch`：批量（≤50）返回订单成本/毛利汇总（复用单订单估算口径），不存在的订单省略；附单测（去重 + 缺失订单跳过）。
- Admin 订单列表新增「预估毛利」列：正毛利绿色 / 负毛利红色（含毛利率），缺参考进价显示「缺价」标记、未配置汇率显示「未配汇率」标记（tooltip 说明配置入口）；估算请求异步进行，失败不阻塞列表加载。
- 同步 `docs/api.md`、`admin/src/services/procurement.ts`。

### 变更记录（2026-08-01）迭代第 12 轮：订单详情补库存手工操作入口

- Admin 订单详情「库存影响」Tab 新增「手工扣库存」「手工回滚库存」按钮（Popconfirm 确认，`canWriteOrders` 权限内可见），分别调既有 `POST /orders/:id/deduct-inventory` / `POST /orders/:id/restore-inventory`（`syncInventory=false`，回滚 reason=`manual_ui`），成功后刷新详情与影响流水。此前该入口位于旧列表页死代码 Drawer 中（第 8 轮详情页化遗留），restore 链路 UI 不可达。后端 API 无改动。
- 后端 P1 修复：扣库幂等键 `inventory-deduct:{orderId}:{itemId}:{skuId}` 首扣成功后永久 succeeded，导致「扣库→回滚→再扣库」被 `INVENTORY_DEDUCT_KEY_CONFLICT`（400）拒绝。现以行级 restore 流水计数作为轮次（round），再扣库使用 `…:roundN` 幂等键；effect 行仅保留每 effect_type 最新状态（唯一索引不变），完整历史留在 `inventory_change_logs`（restore 流水补 `business_event_key=inventory-restore:…:roundN`）。同轮重复扣减/重复回滚仍幂等跳过。新增回归测试 `TestDeductRestoreDeductCycle`（扣→重复扣→回滚→重复回滚→再扣→再回滚全周期）。API 路由与 payload 无变化。

### 变更记录（2026-08-01）迭代第 11 轮：采购签收自动入库

- 新增：采购单「标记签收」（shipped → delivered）现同事务将每条采购明细数量加回本地 SKU 库存，并写入 `inventory_change_logs`（`change_type=purchase_inbound`，含 before/after/delta 与采购单号 remark），通过 `business_event_key` 每行幂等，重复入库自动跳过。此前签收只改物流状态，采购入库后本地库存不会增加，库存只减不增无法反映真实可售量。缺 SKU/数量≤0/SKU 不存在的行跳过并记录原因，不阻塞签收。签收事件 payload 记录逐行入库结果，操作日志记录累计入库数量。
- Admin：库存变动日志类型映射新增「采购签收入库」；采购单详情签收成功提示注明已入库。附 sqlite 回归测试（入库 + 幂等重放）。
- 测试反馈修复①（P2）：库存流水页「变更类型」列此前直出原始 change_type key，新增 `INVENTORY_CHANGE_TYPE` 中文映射并接入渲染。
- 测试反馈修复②（既有 P1）：`inventory/order_mirror.go` 的 `orderLineMirror.ProductSKUID` 缺 column 标签，GORM 默认命名找 `product_sk_uid` 映射不到（与第 7 轮 `external_sk_uid` 同类），订单扣库全部被 `missing_product_sku_id` 静默跳过。补 `gorm:"column:product_sku_id"` 并附列名映射回归测试。

### 变更记录（2026-08-01）迭代第 10 轮：规格匹配列表零 UUID 行修复

- 修复：导入时未勾「自动匹配」的订单行没有 `order_item_sku_matches` 记录，`GET /orders/:id/sku-matches` 对这类行返回零值匹配行（`orderItemId` 为零 UUID），前端「绑定 SKU」抽屉以零 UUID 调候选接口 404「候选加载失败」，且该行无展开箭头。现后端对无匹配记录的行回填真实 `orderItemId`/`orderId`/平台与行内编码，状态标为 `unmatched`（原因「尚未执行自动匹配」），无需先「自动匹配整单」即可直接展开候选/绑定；前端对零 UUID 行防御性禁用候选加载与绑定入口。附 sqlite 回归测试。

### 变更记录（2026-08-01）迭代第 9 轮：修复 product_skus 硬删除表被按 deleted_at 过滤（42703）

- `product_skus` 为硬删除表（`HardDeleteBase`，无 `deleted_at` 列），但 SKU 候选推荐（`skucandidate`）、规格搜索（`product/sku_search`）、订单库存扣减/回补（`inventory/order_inventory`）、异常工作台（`orderexception`）多处查询按 `deleted_at IS NULL` 过滤，PostgreSQL 上整条查询以 42703 失败：候选推荐 publication/本地编码/历史手工绑定通道全部失效，库存扣减遇到该查询即报错。已全部移除对 `product_skus.deleted_at` 的引用（软删除表 `products`/`product_publications`/`orders` 的过滤保留）。
- 附 sqlite 回归测试：`skucandidate.SuggestForOrderItem` publication 通道产出候选（修复前该查询直接报错）。

### 变更记录（2026-08-01）迭代第 8 轮：销售订单发货闭环

- 修复（P0）：`POST /orders/:id/shipments` 路由未注册，Admin 订单详情「新增物流」一直 404；现已注册。
- 新增物流写入自动流转：物流状态 `shipped`/`in_transit` → 订单 `shipped`/`fulfilled`（缺省补 `shippedAt`），`delivered` → 订单 `delivered`（缺省补 `deliveredAt`）；仅前进不回退（按订单生命周期 rank），已取消/退款/关闭订单不受影响。附 sqlite 单测（发货/签收流转、pending 不回退）。
- 首页待办新增 `order_await_shipment`「订单待发货」（已付款未发货订单数），直达 `/orders/list?payStatus=paid&fulfillmentStatus=unfulfilled`。
- 同步 `docs/api.md`、`admin/src/constants/dashboardDefaults.ts`。

### 变更记录（2026-08-01）迭代第 7 轮：销售订单批量导入 + 手工订单租户修复

- 新增 `POST /orders/import`：批量创建手工销售订单（≤200 张/批），订单号已存在或批内重复自动跳过（`skipped_duplicate`），单张失败不影响其余，可选创建后自动按 SKU 编码匹配本地规格；Admin 订单列表工具栏新增「批量导入订单」粘贴弹窗（逐行校验 + 同订单号合并明细 + 逐单结果表）。店铺 API 未接入前的订单来源过渡方案。
- 修复：手工订单创建（`POST /orders`）此前不写 `tenant_id`，租户 >0 的管理员创建后立即查不到该订单（`record not found`）；现从请求上下文写入当前租户。附 service 层单测（创建/去重/单行失败不中断/租户可见性）。
- 同步 `docs/api.md`、`admin/src/services/orders.ts`。

### 变更记录（2026-08-01）迭代第 6 轮：刷新提示结构化中文化

- `POST /products/:id/sources/refresh` 的 `alerts` 由英文内部字符串（含货源 UUID）改为结构化对象（`code` / `sourceId` / `supplierName` / `reason` / `thresholdPercent`），货源档案页「切换规则提示」按 code 渲染中文文案并显示供应商名称（第 5 轮 E2E 反馈项）。
- 同步 `docs/api.md`、`admin/src/services/sourcing.ts`。

### 变更记录（2026-08-01）迭代第 5 轮：涨价/断货预警一键动作闭环

- `source_switch_events` 新增 `status` 字段（suggested 事件：open / adopted / ignored），同一商品同一「原货源→备选货源→原因」的待处理建议去重，避免每次刷新报价重复刷屏。
- 新增 `POST /source-switch-events/:id/adopt`（采纳建议：主供应商切换到建议备选货源并标记 adopted，写操作日志，非待处理返回 409）与 `POST /source-switch-events/:id/ignore`；附 sqlite 单测（去重、采纳切主、重复处理拒绝）。
- 新增 `GET /product-source-alerts` 预警货源总览（涨价预警/断货货源 + 商品标题 + 供应商 + 待处理建议数）；货源档案页未选商品时展示该总览，一键「查看档案」直达，首页「货源涨价预警/断货」待办卡落地页不再是空态。
- 货源档案「切换审计」表：suggested 事件展示处理状态，待处理建议提供「采纳建议」（Popconfirm）/「忽略」一键动作；货源列显示供应商名称替代 UUID 截断。
- 同步 `docs/api.md`、`admin/src/services/sourcing.ts`。

### 变更记录（2026-08-01）迭代第 4 轮：SKU 映射删除入口

- 新增 `DELETE /api/v1/product-source-skus/:id`（软删除单条本地↔外部 SKU 映射，写操作日志，附 sqlite 单测）；此前 UI 只能清空 `external_sku_id`，行仍计为有效映射，导致采购受阻判定与实际不符（E2E 测试反馈项）。
- 货源档案 SKU 映射弹窗每行新增「删除映射」操作（Popconfirm 确认），删除后行回到未映射状态并刷新档案。
- 同步 `docs/api.md`、`admin/src/services/sourcing.ts`。

### 变更记录（2026-07-31）迭代第 3 轮：问题订单自动拦截（采购受阻异常）

- 订单异常工作台新增聚合异常类型 `procurement_blocked`：已付款、未发货且未取消/退款/关闭的订单行，已绑定本地 SKU 但缺可用主货源或主货源缺 SKU 映射、且未被任何未取消/未失败采购单行覆盖时，自动进入工作台（`sourceType=order_item`，严重程度 high），错误信息区分「缺主货源 / 缺 SKU 映射」并给出建议动作；附 sqlite service 层单测（缺货源、缺映射、补映射后消失、被采购单覆盖/取消、未付款不拦截、标记已处理）。
- 行内新增 `sourcingUrl` 跳转货源档案；`/sourcing/product-sources` 支持 `?productId=` 直达；异常页新增「采购受阻」筛选、统计卡与「去货源档案」操作。
- Dashboard：`summary.procurementBlockedOrderItems` + 统一待办 `procurement_blocked`（P0）；顺带修复待办 `order_sku_unmatched` 链接 query 参数名（`type` → `exceptionType`）导致筛选不生效的问题。
- 同步 `docs/api.md`、`admin/src/services/{dashboard,orderExceptions}.ts`、`admin/src/constants/dashboardDefaults.ts`。

### 变更记录（2026-07-31）Docker 全栈构建修复与 legacy 登录租户修复

- 修复 `docker-compose.full.yml` 全栈构建失败：admin/collector 镜像缺少 `scripts/patch-pro-field-antd-select.mjs` 导致 `pnpm install` postinstall 报错（Dockerfile 增加 `COPY scripts`、`.dockerignore` 放行该脚本）；admin 基础镜像 node:26 不再内置 corepack，改用 `npm install -g pnpm` 安装。
- 修复 legacy_local_storage 登录模式 JWT 恒定 `tenant_id=0` 的问题：`LegacyMintToken` 改为携带管理员真实租户，选品等按租户隔离的模块在生产（无 dev 租户 fallback）下不再静默卡 pending，附单测。

### 变更记录（2026-07-29）生产部署收口与运营手册

- 新增 `ADMIN_BOOTSTRAP_TENANT_ID`：首次创建管理员时指定租户（示例文件默认 1），解决选品 worker 因 tenant_id=0 静默拒绝任务的问题（staging/production 禁用 dev 租户 fallback，故必须在种子阶段落租户）；同步 `.env.example` / `.env.docker.example` / `.env.production.example` / `docs/env.md` / `docs/docker-deployment.md`，附单测。
- 新增 `docs/operations-manual.md`：日常运营操作手册（选品→上架→出单→采购→发货，1688 人工下单过渡模式），并登记到 `docs/README.md`。

### 变更记录（2026-08-02）迭代第 37 轮：只读角色后端写防线（P0）+ 新建用户继承租户（P1）

- P0：`order`/`procurement` 路由改为在写端点统一挂 `requireWrite()` 守卫（只读账号 403「当前账号为只读权限，无法执行此操作」）。此前仅 Import/库存影响/SKU 匹配少数 handler 调用 `denyWrite`，只读账号可直接调 API 创建/修改/删除订单与操作采购单，前端隐藏是唯一防线；附路由级回归单测。
- P1：设置→用户 新建用户此前恒为 `tenant_id=0`，与创建者（如 tenant 1）跨租户导致新账号登录后看不到数据；改为继承当前请求租户。
- 订单列表工具栏（新建订单/批量导入/批量发货）对只读角色整体隐藏。

### 变更记录（2026-08-02）迭代第 38 轮：只读写防线扩展到货源/异常/定价模块

- 新增 `adminperm.RequireWritable(db)` 通用路由级只读守卫中间件；`sourcing`（供应商/货源/SKU 映射/建议采纳忽略/刷新）、`orderexception`（处理/忽略/绑定 SKU/重试）与 `pricing` apply 写端点统一挂守卫（只读账号 403），读端点不受影响；附 sourcing 路由级回归单测。
- 前端：供应商管理、商品货源档案、订单异常工作台的写入口（新增/编辑/删除、绑定货源、刷新、锁定开关、设为主供应商、SKU 映射、采纳/忽略建议、已处理/忽略/取消标记、重试）对只读角色隐藏或禁用，导航与详情等读操作保留。

### 变更记录（2026-07-29）迭代第 39 轮：异常工作台租户/店铺范围过滤（scope 口径统一）

- 订单异常工作台此前聚合查询无任何租户/店铺范围限制：只读账号（无店铺授权）订单列表为空但异常工作台可见全租户数据，且可跨租户读取异常行。现列表/汇总/详情统一应用范围：所有 collector（SKU 未匹配/歧义、库存扣减失败、库存同步失败、订单同步部分失败、采购受阻、负毛利）按当前租户过滤；非 admin 结果再按授权店铺过滤（无店铺授权返回空，口径与订单列表 `ApplyStoreScope` 一致，无店铺归属的行仅 admin 可见）；详情按 sourceId 越权访问返回 404。附 sqlite 范围回归单测（跨租户隔离、店铺授权过滤、无授权为空、越权详情 404）。
- Dashboard 汇总/统一待办中的异常计数同步走相同 scope（`operationdashboard.Scope` 新增 `TenantID`），非 admin 首页异常待办不再泄露范围外数据。

### 变更记录（2026-07-29）迭代第 40 轮：异常工作台批量标记已处理/忽略/取消标记

- 订单异常工作台列表新增行选择（仅可写角色显示），批量操作条提供「批量已处理（N）」「批量忽略（N）」「批量取消标记（N）」：前两者只统计所选未标记行，取消标记只统计所选已标记行；逐行复用既有 handle/ignore/mark 删除 API，逐行汇总成功/失败数并展示首个错误。批量已处理支持一次填备注应用到全部所选行。纯前端改动，无 API/权限/状态机变化。
- 移动端收口：<768px 视口下操作列不再 `fixed: 'right'`（避免固定列盖住选择列导致 checkbox 不可点）。注意 pro-table 的 columnsMap 会固化首帧列 `fixed`，因此宽屏判断用 `useState(() => window.innerWidth >= 768)` 惰性初始化保证移动端首帧即不固定，并以 `key={wide|narrow}` 在跨断点时重挂载表格（选择为父级受控状态，不丢已选）；rowSelection 开启 `preserveSelectedRowKeys` 支持跨视图/分页保留已选行。

### 变更记录（2026-07-29）迭代第 41 轮：异常工作台「全部」视图

- 「视图状态」筛选新增「全部」：`GET /orders/exceptions` 支持 `all=true` 同屏返回未处理/已处理/已忽略行（`summary` 口径不变，仍只统计未处理），未标记与已标记行可同屏混合勾选批量操作，不再需要跨视图切换。附 filterAggRows 视图口径单测。

### 变更记录（2026-07-29）迭代第 42 轮：批量操作条常驻不位移

- 订单列表 / 异常工作台的 ProTable rowSelection 增加 `alwaysShowAlert: true`，采购单列表批量 Alert 改为可写角色常驻显示（导出按钮 0 选中时 disabled）：批量操作条不再在勾选首行时突然出现，消除表格行整体下移导致连续勾选误点的问题（第 41 轮测试反馈项）。纯前端样式/交互改动。

### 变更记录（2026-07-29）迭代第 43 轮：采购协同租户/店铺范围过滤

- 采购协同全部读写接口统一 tenant/store scope（r38/r42 测试遗留的读口径不一致项）：列表按 `purchase_orders.tenant_id` + 授权店铺（经明细行来源销售订单 `shop_id`）过滤；详情/导出/单号回填等 `:id` 路由挂 `scopePO` 守卫，范围外一律 404 不泄露存在性；成本估算（单/批）按来源销售订单 scope 判定；批量回填单号/运单号逐行跳过范围外记录；生成采购单校验来源订单在范围内并把订单租户写入 `purchase_orders.tenant_id`（含存量数据 backfill 迁移）。附 sqlite 范围回归单测（scope_test.go）。API URL/method/payload/状态机不变。

### 变更记录（2026-07-29）迭代第 44 轮：批量导入订单支持店铺归属

- 「批量导入订单」弹窗新增「关联店铺（可选，应用到本次导入的全部订单）」下拉，选中后整批订单写入 `shopId`（复用既有 `CreateBody.shopId` 与后端店铺可见性校验，API 不变）；补齐订单店铺归属入口，使店铺授权（store scope）的正向过滤在手工/导入订单上可用（此前导入订单 shop_id 恒为 NULL，非 admin 授权店铺账号看不到任何导入订单）。纯前端改动。
### 变更记录（2026-08-01）迭代第 45 轮：经营报表页（按日趋势）

- 新增 `GET /api/v1/orders/stats/daily?days=30`（默认 30，最大 90）：按本地日历日聚合订单数/已付款数/分币种已付款销售额；口径与 `stats/sales` 一致（当前租户、软删除订单不计入），店铺 scope 与订单列表一致（非 admin 按授权店铺过滤，admin 全量）；空缺日期补零返回，附单测（多币种/软删/跨租户/窗口外/默认与上限 clamp）。
- 管理端订单组新增「经营报表」页（`/orders/reports`，readonly 可见的只读报表）：近 30 天合计卡（订单数/已付款/分币种销售额）+「每日订单数/已付款数」折线图 +「每日销售额（按币种堆叠）」柱状图，含加载骨架/空数据引导/错误重试，375px 无横向溢出；图表引入 `@ant-design/plots@2.6.8`（AntD 官方图表库，稳定版）。
- 同步 `docs/api.md`。

### 变更记录（2026-08-01）迭代第 45 轮：用户管理删除入口 + 店铺授权体验收口

- 设置→用户新增「删除用户」入口（Popconfirm 确认，admin 角色可用，不能删除自己）：后端新增 `DELETE /api/v1/admin/users/:id` 软删除端点（`deleted_at` 保留数据，参考订单软删除口径），事务内同时撤销全部店铺授权并递增 `token_version` 使既有会话失效；路由级挂 `adminperm.RequireWritable` 只读守卫，handler 内 `user.manage` 权限校验（仅 admin）；附 sqlite HTTP 单测（软删除落库、撤销授权、不能删自己 400、readonly/operator 403、重复删除 404）。解决测试账号只能禁用不能删除、fixture 越积越多的问题。
- 修复「店铺权限」保存首次点确定不生效：原实现在二次确认弹窗 onOk 里调用 `permForm.submit()`，而弹窗 `destroyOnClose` 下 Form 首次打开前未挂载，`setFieldsValue`/`submit` 可能落空。改为 Modal `forceRender` 保证 Form 常驻挂载、打开前 `resetFields` 后回填；外层确定先 `validateFields`（校验错误内联展示），二次确认 onOk 改为 async 直接提交（Modal.confirm 呈现 loading，失败 message.error 且弹窗不关闭）。
- 授权店铺列显示店铺名而非 UUID：后端 `loadStorePerms` 店铺名查询改 `Unscoped`（软删除店铺也能回显名称）；前端缺名时兜底「未知店铺」。
- 同步 `docs/api.md`（用户与权限管理表）、`admin/src/services/adminUsers.ts`。API 既有端点语义/状态机/readonly 403 文案不变。

### 变更记录（2026-08-02）迭代第 59 轮：设置中心系统性安全走查与修复

- 设置中心全量真实走查（docker compose 全栈，15 个 /settings 子页 + /shops/manage）：脱敏（sk-****abcd）、DB 密文、日志无完整密钥、连接测试、持久化、375/768/1440 响应式均复核通过；本轮修复以下问题。
- P1 租户隔离（QA 回归发现）：`/api/v1/admin/users` 列表与 Get/Update/SetStorePermissions/Delete 未按租户过滤，tenant-2 用户对 tenant-1 admin 可见且可删。修复：`adminuser.Service` 全部操作按 `ctxkey.TenantID` 过滤，跨租户 ID 统一 404「用户不存在」不泄露存在性，店铺权限分配仅限同租户店铺；附 admin/operator/readonly 三角色单测（`tenant_scope_test.go`）。
- P0 readonly 写入口缺守卫：`DELETE /shops/:id`、`PUT /shops/:id/auth`、`POST /shops(/… test/oauth 写操作)`、`PUT /platform/(publish-)settings/:platform`、`POST /settings/test-image|test-ocr` 此前对 readonly 放行。修复：新增 `adminperm.RequirePermissionMW/RequireWriteMW` 路由中间件，平台设置写走 `settings.manage`、店铺写走 `store.operate`，readonly 一律 403；附路由守卫单测（`router_readonly_test.go`）。Shops 页面同步按 `store.operate` 隐藏全部写控件。
- P1 连接测试失败提示英文直出：test-ai / test-ocr / 存储公网测试等非 2xx 时 toast 显示 axios 原文。修复：`admin/src/services/request.ts` 统一从后端 Envelope 提取中文 message 抛 `ApiRequestError`。
- P2 采集设置「页面打开超时」对越界值静默钳制：改为显式 1000~300000 范围中文校验文案。
- 工程修复：admin `react-dom` 声明 19.2.8 与 react 18.2.0 不一致导致前端单测挂掉；对齐 18.2.0 并把 overrides 同步进 `pnpm-workspace.yaml`（新版 pnpm 只读该文件），`pnpm test:frontend` 恢复全绿。

### 变更记录（2026-08-02）生产部署本地演练收口（R45 部署包复核）

- 在开发 VM 上按生产口径完整演练 `docker-compose.prod.yml`（APP_ENV=production + Caddy 内部 CA + /etc/hosts 假域名）：从零 env → 构建 → 启动 → bootstrap 管理员 → HTTPS 登录 → 核心页面抽查 < 15 分钟，验证恢复演练生产禁用、错误统一 envelope 不泄露堆栈、静态资源 immutable 缓存。
- 修复演练发现缺陷：① backend 镜像缺 `pg_dump`/`pg_restore`/`psql` 导致 `/api/v1/ops/backups` 生产必失败，Dockerfile 增装 PGDG postgresql-client-16；② GORM record-not-found 按错误打完整 SQL（含账号入参）入生产日志，database.go 改用 `IgnoreRecordNotFoundError: true`；③ `.env.prod.example` 澄清 `BACKUP_SCHEDULE` 仅为元数据、每日自动备份须配宿主机 crontab。
- 新增 `docs/production-launch-checklist.md`：资源清单（服务器/域名/DNS/防火墙）、逐步上线命令、回滚方案、上线后验证清单。

- 修复第 45 轮 UX 走查 P1：JWT 过期后提交表单遇 401 直接 `window.location.assign` 跳登录页，弹窗内未保存内容全部丢失。新增前端会话守卫（`admin/src/utils/sessionGuard.ts`）：
  - 401 处理链路改为 umi `responseInterceptors` 内先静默续期（复用后端既有 `POST /api/v1/auth/refresh`，secure_session 走 HttpOnly cookie、legacy 走响应体 refreshToken），成功后原样重放原请求，调用方无感知；续期不可用/失败时在当前页弹「登录已过期」重新登录弹窗（`SessionExpiredModal`，挂载于 innerProvider），重登成功后同样重放请求，页面与表单状态不丢失；用户选择「去登录页」才清凭证跳转。
  - 临期静默续期：登录/注册开始保存 `expiresAt`（及 legacy 模式的 refreshToken），请求拦截器在 token 剩余有效期 < 5 分钟时先 single-flight 续期再发请求；续期失败有 60s 冷却，legacy 无 refreshToken 时直接跳过续期走重登弹窗。
  - 兜底：重放后仍 401（如 token_version 已失效）才回退整页跳登录页。API URL/method/payload/权限/状态机均未改，后端零改动。附 sessionGuard 单测（保存/清理、阈值判断、single-flight、冷却、重登协调）。实测覆盖手工建单弹窗、订单详情编辑、设置页保存三场景过期后内容不丢失。

### 变更记录（2026-08-01）迭代第 46 轮：发货与库存扣减口径 UI 传达（第 45 轮 UX 走查 P1）

- 根因结论：**非 bug，是既有「手工扣库」策略使然**。发货（单笔新增物流 / 批量发货）只推进订单生命周期（`advanceOrderOnShipment`），从不调用 `DeductInventoryForOrder`；库存扣减由手工触发（第 12 轮 `POST /orders/:id/deduct-inventory`）或库存策略（如建单自动扣减）触发，取消订单按策略自动回滚（第 32 轮）。本机全栈复现确认：未扣减订单直接发货成功且 `order_inventory_effects` 始终为空。问题在于 UI 未传达该口径，导致「未扣减 + 可发货」看似自相矛盾。
- 处理（不引入「发货强制扣库存」语义，不改状态机/权限/既有字段语义）：
  - 订单详情「新增物流」弹窗：当本单尚无成功扣减记录时提示「本单尚未扣减库存……可到库存影响 Tab 手工扣减」（编辑既有物流不提示）。
  - 「库存影响」Tab 顶部新增口径说明 Alert：发货不会自动扣减库存，扣减由手工/策略触发，取消订单自动回滚。
  - 批量发货：结果成功行按订单是否已有成功扣减新增标记「未扣库存」；`POST /orders/shipments/batch` 成功行新增可选返回字段 `inventoryDeducted`（仅新增字段，见 docs/api.md），弹窗说明文案同步补充口径。附 handler 层单测（TestAnnotateBatchShipmentInventory）。

### 变更记录（2026-08-01）第 46 轮会话守卫全栈验证与后端修复

- 全栈（docker-compose.full.yml）真实环境验证 PR #73 四个未测分支均通过：①secure_session 模式 401 后经 HttpOnly cookie 静默续期并重放；②legacy 模式响应体 refreshToken 续期+轮换保存；③token 剩余 <5 分钟请求前 single-flight 提前续期；④并发多请求 401 仅发一次 refresh 并全部重放。legacy「登录已过期」弹窗+重登+重放回归不回退。
- 修复后端两处缺陷（均补回归测试）：secure_session 登录先分配会话 ID 再签发访问令牌，修复 access token `session_id` 为全零导致会话吊销失效；legacy 模式 `/auth/refresh` 优先使用请求体 refreshToken，避免从 secure_session 切回后残留 HttpOnly cookie 触发复用检测使续期持续 401。
- 已知边界：legacy 登录本身不签发 refreshToken（响应无该字段），故 legacy 纯自然场景下 401 直接走重登弹窗（前端按设计降级）；②的续期链路以数据库构造有效 refreshToken 验证。secure_session + Docker 需设置 `ADMIN_PUBLIC_URL`（CSRF Origin 校验），`.env.docker.example` 已补注释说明。

### 变更记录（2026-08-02）迭代第 48 轮：采集任务处理超时终态 + 等待时长/失败原因反馈（UX P1-6）

- 修复采集任务在无外网/采集器异常时长期停留「处理中」无反馈的问题。后端新增采集任务处理超时终态机制：任务在 `pending`/`running`/`retrying` 停留超过 settings `collector.collect_task_processing_timeout_seconds`（默认 600 秒，最小 30 秒，后台「采集设置 → 通用采集设置 → 任务处理超时」可配）时，由 task reaper 周期扫描自动置为 `failed`，`errorMessage` 标注「任务超时」，事件时间线记录 `task.processing_timeout`，操作日志记 `collect.task.processing_timeout`，批次计数同步 reconcile。
- 任务模型/DTO 新增向后兼容字段 `queuedAt`（最近一次入队/重试入队时间；旧数据为空时以 `createdAt` 计），创建、批量创建、手动重试、批次重试失败任务时写入/重置，保证重试后超时重新计时。
- 采集中心任务列表：处理中（排队/处理中/重试中）行在状态下方显示「已等待 X 秒/分钟/小时」；失败原因列在错误码/提示缺失时回退展示 `errorMessage`（超时原因可见）；既有「重试」按钮对超时失败任务可用。附后端单测（超时置失败/未超时与终态不动/重试重新计时/旧数据回退 createdAt）。

### 变更记录（2026-08-02）迭代第 49 轮：草稿批量导出上架清单（选品→优化→导出→人工上架闭环）

- 新增 `GET /api/v1/products/listing-list/export.csv?ids=`（复用采购清单/发货清单批量导出模式）：逗号分隔商品 UUID，去重后 ≤50 个，每草稿一行，列含 商品ID/标题/副标题(AI标题)/描述/类目(取发布配置 categoryPath)/价格(区间)/币种/主图URL/规格列表/来源/来源链接/状态，UTF-8 BOM 供 Excel 直开；租户+店铺 scope 与草稿列表一致（ApplyTenantScope + ApplyProductScope），任一 id 不在 scope 内整单 404；导出属读操作，readonly 可用（与采购/发货导出后端口径一致）。附单测（表头/BOM/类目/规格列表、跨租户 404、店铺 scope 允许/拒绝、handler 去重/上限/非法 id/未知 id）。
- 草稿列表批量操作条新增「批量导出上架清单（N）」按钮（复用既有行选择），前端限 50 个/次与后端对齐；运营者优化完标题/描述/价格后可一键导出去 Temu/Shopee 手工上架，不再逐字段复制。

### 变更记录（2026-08-02）迭代第 51 轮：一键演示种子数据（seed / clean 幂等闭环）

- 新增 `backend/cmd/seeddemo`（`pnpm seed:demo:full` / `seed:demo:full:clean` / `seed:demo:full:verify`）：向指定租户（`-tenant`，默认 0）直接写库生成贯穿全链路的演示数据——店铺 ×2、商品草稿 ×5（采集/手动来源、AI 优化前后文案、主图、SKU ×10 含低库存告警样本）、供应商 + 货源档案 + 货源 SKU 映射 + 价格历史、销售订单覆盖 pending/paid/shipped/delivered/cancelled（含物流记录与 SKU 匹配/未匹配样本）、采购单覆盖 draft→delivered 全链 9 状态（不含作废，#85 未合并）、库存变动流水（订单扣减/采购入库/手动盘点）、库存同步批次与任务（成功+失败）、订单同步 partial_success、异常工作台 handled 标记。
- 所有数据带 `DEMO-` 前缀；seed 幂等（先清后建，重复执行计数一致）；clean 只删 DEMO- 前缀数据并级联子表，verify 复核零残留；采购单状态链逐步经 `procurement.CanTransition` 校验并写 `purchase_order_events`，订单生命周期经 `order.ValidateOrderStateTransition` 校验，不产生非法状态；`APP_ENV=production` 拒绝执行；不改任何 API/权限。种子实现复用 `internal/modules/demoseed` 模块（`FullDemoSeeder`），附单测（状态链合法性/前缀/生产环境守卫）。
- 同步 `docs/development.md`、`README.md`、`README.en.md`、`package.json`。

### 变更记录（2026-08-02）迭代第 52 轮：权限矩阵契约测试 + 路由级只读守卫收口

- 新增权限矩阵契约测试套件 `backend/internal/securitytests/permmatrix/`：从生产路由器（`api.Register`）实际挂载的全部 486 条路由建立矩阵登记表 `matrix.json`，逐路由 × {admin, operator(有限店铺), readonly(无店铺), 跨租户 admin} 断言授权预期（allow=通过守卫 / forbid=403），并断言匿名 401、跨租户店铺隔离、operator 店铺 scope、readonly 空列表。**新增端点未在 matrix.json 登记预期时 `TestRouteRegistryComplete` 失败**（含陈旧条目检测），`PERM_MATRIX_GENERATE=1` 可打印草稿条目供安全评审。运行与维护方式见 `docs/permission-matrix.md`。
- P0 修复①：`POST /settings/test-image`、`POST /settings/test-ocr` 缺 `settings.manage` 权限检查（同组其它 settings/test-* 均有），readonly/operator 可触发外部连接测试；已补 `adminperm.RequireWrite(PermSettingsManage)`，回归见 `TestReadonlyWriteGuardRegression`。
- P0 修复②：新增路由级只读守卫 `adminperm.ReadonlyWriteGuard` 挂在 `/api/v1`（authed 组）与 `/api/collector`：全部写方法路由 readonly 一律 403（fail-closed，新写端点默认被守卫），显式允许清单仅保留自助 session 管理与纯计算类 POST（calculate/check/preview/validate/estimate）。修复前约 190 条写端点无路由级只读守卫、对 readonly 返回 400/404 或放行。
- 同步 `docs/api.md`（权限矩阵契约一节）、新增 `docs/permission-matrix.md`。
### 变更记录（2026-08-02）迭代第 53 轮：操作日志页 URL 深链 + 审计可见性验收

- 现状盘点结论：审计能力后端已完备（`operation_logs` 模型含租户/店铺/角色/hash 链；写入覆盖登录、设置修改、采购状态流转/作废、任务重试、库存变动、店铺授权等；`GET /api/v1/operation-logs` 只读端点已含 `operationlog.view` 权限 + 租户/店铺 scope；管理端已有统一 `/system/operation-logs` 页）。本轮补齐 UX 缺口：
  - 操作日志页筛选（操作/用户/资源/时间范围）与分页写入 URL 深链（`page`/`pageSize`/`action`/`username`/`resource`/`start`/`end`），刷新/直链可恢复；`urlState` 允许清单新增 `action`/`username`/`resource`。
  - 新增「对象」列（`resourceId`，可复制），满足「时间/操作人/模块/动作/对象/摘要」验收口径；敏感字段不展示（后端本就不落 Secret/Token，IP 仅哈希）。
  - 新增 Admin E2E `operation-logs.spec.ts`（列表渲染、筛选深链、直链恢复、375px 无溢出、无写请求/致命 console）与 mock。
  - 修复全局 `SessionExpiredModal` 的 antd `destroyOnClose` 弃用告警（antd 5.29 升级后导致全部 E2E console 守卫失败），改为 `destroyOnHidden`。
- docs/api.md 同步 `GET /api/v1/operation-logs` 契约说明（权限/scope/筛选/深链）。

### 变更记录（2026-08-02）迭代第 51 轮：一键演示种子数据（seed / clean 幂等闭环）
### 变更记录（2026-08-02）迭代第 55 轮：经营报表导出 CSV + 移动端收口
### 变更记录（2026-08-02）迭代第 59 轮：商品草稿模块系统性真实走查收口

- 新增 `backend/cmd/seeddemo`（`pnpm seed:demo:full` / `seed:demo:full:clean` / `seed:demo:full:verify`）：向指定租户（`-tenant`，默认 0）直接写库生成贯穿全链路的演示数据——店铺 ×2、商品草稿 ×5（采集/手动来源、AI 优化前后文案、主图、SKU ×10 含低库存告警样本）、供应商 + 货源档案 + 货源 SKU 映射 + 价格历史、销售订单覆盖 pending/paid/shipped/delivered/cancelled（含物流记录与 SKU 匹配/未匹配样本）、采购单覆盖 draft→delivered 全链 9 状态（不含作废，#85 未合并）、库存变动流水（订单扣减/采购入库/手动盘点）、库存同步批次与任务（成功+失败）、订单同步 partial_success、异常工作台 handled 标记。
- 所有数据带 `DEMO-` 前缀；seed 幂等（先清后建，重复执行计数一致）；clean 只删 DEMO- 前缀数据并级联子表，verify 复核零残留；采购单状态链逐步经 `procurement.CanTransition` 校验并写 `purchase_order_events`，订单生命周期经 `order.ValidateOrderStateTransition` 校验，不产生非法状态；`APP_ENV=production` 拒绝执行；不改任何 API/权限。种子实现复用 `internal/modules/demoseed` 模块（`FullDemoSeeder`），附单测（状态链合法性/前缀/生产环境守卫）。
- 同步 `docs/development.md`、`README.md`、`README.en.md`、`package.json`。

### 变更记录（2026-08-02）迭代第 56 轮：本地模式备份→下载→校验→恢复演练最小真实闭环

- 备份下载通道：新增 `GET /api/v1/ops/backups/:id/download`（`backup.download` 权限，仅 admin；readonly/operator 403，不存在/越权 404），仅允许下载校验通过的 completed 备份，下载前重验 SHA-256，流式返回，成功/失败均写操作日志（action=`backup.download`）；管理端备份列表新增「下载」按钮。
- 备份校验修复：未启用加密（local 模式）时加密检查按「未启用（跳过）」处理，不再恒 failed；校验结果新增 `details.checks` 结构化检查项（校验和 / manifest / 加密 / pg_restore 结构），管理端弹窗中文结构化展示。
- 恢复演练真实化（本地/开发限定）：`POST /ops/restores/:id/verify` 与 `POST /ops/dr/drills` 真实执行备份文件完整性（SHA-256）与 `pg_restore --list` 结构校验两项检查，替换原六项硬编码 passed 桩；其余检查项（迁移版本/租户隔离/RBAC/审计链/对象清单/密钥密文、RPO/RTO/应用切换）在 `details.checks`/`reportJson.checks` 中明确标注 `not_implemented`；`APP_ENV=production` 下两接口直接拒绝，恢复安全门（隔离目标、`trademind_p6v_restore_` 前缀、二次确认、高风险确认）保持不变。附 backup/restore 服务层单测；同步 `docs/api.md` 与 `docs/docker-deployment.md` 生产备份 SOP 章节。

### 变更记录（2026-08-02）迭代第 55 轮：经营报表导出 CSV + 移动端收口

- 后端新增只读导出端点 `GET /api/v1/orders/stats/daily/export.csv?days=30`：复用 `stats/daily` 数据与租户/店铺 scope，UTF-8 BOM，列为「日期/订单数/已付款数/已发货数」+ 窗口内每币种一列「已付款销售额(币种)」（字典序），空日期补 0；`stats/daily` 同步新增 `shippedCount` 字段（口径与 `stats/sales` 已发货一致）。附字节级单测（BOM/表头/数据行/默认天数）。
- 报表页新增「导出 CSV」按钮与近 7/30/90 天切换（Segmented），数据请求与导出共用同一 days；导出带 loading 防重复、成功/失败提示；readonly 可用。五档视口（1440/1280/1024/768/375）无根节点横向溢出，新增 `admin/e2e/specs/orders-reports.spec.ts`（GET-only mock）。
- 依赖修复：admin `react-dom` 由 19.2.8 回对齐 `react` 18.2.0（含 `@types/react-dom`），pnpm overrides 迁移至 `pnpm-workspace.yaml`（pnpm 10+ 不再读取 package.json `pnpm` 字段）；`SessionExpiredModal` 的 `destroyOnClose` 改 `destroyOnHidden`（antd 5.29 弃用警告导致 E2E console guard 全量失败）。

### 变更记录（2026-08-02）R57 选品中心 P2×7 走查修复

- 选品任务列表：新增状态筛选（复用后端既有 `status` 查询参数）；处理中/待处理任务自动轮询刷新（4s，无活跃任务或页面隐藏时停止）；失败/部分失败状态 Tag 悬浮显示失败原因；readonly 隐藏「新建选品任务」「重试」写入口（仅前端对齐既有权限模型，后端 403 守卫不变）。
- 选品详情：失败/部分失败任务页顶 Alert 显示任务级失败原因；失败候选状态列直接展示失败原因文本；处理中任务自动轮询并提示；readonly 隐藏「通过/拒绝/转草稿」；任务信息 Descriptions 改响应式列数；全局 PageHeader 标题/副标题窄屏允许换行，修复 375px 头部溢出/挤压。
- 错误透传：新增 `extractApiErrorMessage`（`admin/src/services/request.ts`），选品写操作 catch 优先展示后端 envelope 结构化中文 message（如 readonly 403「当前账号为只读权限，无法执行此操作」）。
- AI 设置：新增「清空当前服务商配置」入口（Popconfirm 确认）；settings `PUT /api/v1/settings` item 新增可选 `clear` 字段（为 true 时强制清空已存值，含加密字段，绕过「空加密值保留旧密钥」语义；不新增端点，遵循既有 settings 模式），附 sqlite 单测（保留语义回归 + clear 清空 + clear 不落敏感值）。
- E2E：新增 `selection-r57-p2.spec.ts`（状态筛选、失败原因展示、readonly 隐藏、403 中文透传、375px 无横向溢出）；ConsoleGuard 支持单测试级预期输出白名单（故意 mock 的 4xx 等）。

### 变更记录（2026-08-02）第 58 轮：采集模块写守卫与 tenant scope 审计（R54 遗留 P1）

- 修复 readonly 用户可创建采集任务的 P1：collect 全部写端点（`POST /collect/tasks`、`/collect/tasks/:id/retry`、`/collect/batches`、`/collect/batches/:id/retry-failed`、各 `open-login-browser`）路由级补挂 `adminperm.RequireWritable`，readonly 直调返回 403 且数据库零变化；`check-login`/`auth-status` 视为登录态诊断读，不拦截。
- 读端点补 tenant scope（与订单/选品口径一致）：任务/批次列表按 `tenant_id` 过滤，`GET /collect/tasks/:id`、`/collect/tasks/:id/events`、`/collect/batches/:id`、`/collect/batches/:id/tasks` 及手动重试的对象查询均走 `adminperm.ApplyTenantScope`，跨租户访问返回 404 不泄露存在性；任务/批次创建时落 `tenant_id`（与 integration/round55-preview 上 #88/#93 的 tenant_id 修复方向一致，PR → main 单独说明关系）。
- 前端采集页对齐 readonly 模式：采集任务/批量采集页隐藏创建表单与重试入口，采集中心禁用单条/批量采集按钮（提示只读文案）；`/collect/rules`、`/collect/browser-profiles`、`/collect/monitor`、`/settings/collector` 原本已由 SETTINGS_MANAGE 菜单权限隔离。
- 回归单测：三角色路由守卫（readonly 全写端点 403、admin/operator 不误伤、读端点不拦截）+ tenant scope（同租户 200、跨租户 404、列表不泄露、跨租户重试零变化）。同步 `docs/api.md` 采集节权限口径说明。

### 变更记录（2026-08-02）迭代第 58 轮：Ops P2 修复（备份校验状态门 / 安全门原因透传 / 库存口径 / demo 采购单清理）

- 备份管理：`manual_review`（及其他非 completed）状态的「校验备份」按钮禁用，Tooltip 中文说明需先在环境启用 `BACKUP_ENABLED` 并通过人工审查后重新创建备份才能校验；不改后端校验行为。附 helper 单测。
- 恢复验证：安全门拒绝（`RESTORE_TARGET_FORBIDDEN` / `RESTORE_APP_ENV_FORBIDDEN` / `RESTORE_TARGET_NOT_ISOLATED` / 前缀 / 二次确认 / 备份未校验 / 目标库非空等 14 个结构化错误码）在 `errorMessages.ts` 补齐中文映射，创建失败 toast 透出具体原因；恢复记录表新增「失败原因」列展示 `errorSummary`。不弱化任何安全门。附映射单测。
- 批量发货 / 订单详情：结果区补「未扣库存属手工扣库存策略的预期行为」口径 Alert 与 Tag Tooltip（Tag 文案改为「未扣库存（预期）」）；订单详情「物流」Tab 说明补同一口径。沿用 R46 口径传达模式，不改库存扣减逻辑与接口。
- demo seed clean：`seed:demo:full:clean` 清理采购单从仅按 `idempotency_key LIKE 'DEMO-%'` 扩展为并集：`external_order_id`/`supplier_name` 带 DEMO- 前缀、挂在 DEMO- 供应商名下、或采购行关联 DEMO- 销售订单（覆盖测试期 UI 建的采购单）；verify 同步扩展。仅清 DEMO 关联数据，不动真实采购单。附 `collectDemoPurchaseOrderIDs` 单测（真实采购单不被匹配）。

### 变更记录（2026-08-02）迭代第 59 轮：商品草稿模块系统性真实走查收口

- 全栈真实走查（docker compose + demo seed）商品草稿列表/详情/AI 优化/归档删除/采集回链 + 三角色权限 + 375/768/1440 响应式，发现并修复：
  - **P0（后端越权风险）**：商品写接口除 `POST /products` 外均无路由级只读守卫（PUT/DELETE 商品、SKU/图片 CRUD、平台配置、抖音 mapping/图片、AI 优化/应用/撤销、sync-images），readonly 此前仅靠店铺可见性 scope「碰巧」404。现全部补 `denyWrite`（AI 应用/撤销走 `ai_text.apply` 守卫），附 `readonly_guard_test.go` 全路由 403 回归测试；docs/api.md 商品节同步说明。
  - **P1**：前端统一 request 错误规范化——HTTP 非 2xx 时还原后端 envelope `message`（如 AI 未配置时「请配置 base_url」），不再裸显 axios「Request failed with status code 400」；附 request 单测。
  - **P1**：商品草稿列表 readonly 隐藏选中工具栏批量写入口（批量 AI 优化/AI 图片处理/创建刊登草稿/设置发布价；只读可用的批量发布检查与批量导出上架清单保留），顶部工具栏与 main 口径一致（隐藏「新建草稿」，保留「更多」）；详情页 readonly 隐藏「标记为可用/归档/删除」头部操作；复用 main 的 readonly E2E。
  - **P2**：归档增加二次确认 Popconfirm；新建草稿失败给出 message.error；新建 SKU 保存后延迟拉取真实行修复「新行只有删除按钮」；`operationStep` 筛选加入口径说明 Alert（筛选=该步骤未完成，行内 Tag=当前所处步骤）；批量发布检查抽屉在当前平台无授权店铺时给出空态引导并禁用店铺选择。
  - **修复 main 上 test:frontend 挂掉**：dependabot 将 admin `react-dom` 升到 19.2.8（react 仍 18.2.0）导致 3 个组件测试套件崩溃；回退 `react-dom`/`@types/react-dom` 到 18 系，`pnpm-workspace.yaml` 增加 overrides（pnpm 10 读取位置，与 package.json `pnpm.overrides` 保持一致），root devDependencies 固定 react/react-dom 供 @testing-library/react peer 解析。
- 遗留：「批量导出上架清单」已由第 49 轮在 main 实现（本分支合入）；operator demo 账号无授权店铺，operator 正常店铺 scope 场景未覆盖；商品计数口径（列表 total 与看板数）建议后续统一。

### 变更记录（2026-08-02）第 61 轮：运营任务店铺 scope（P2）+ execute 校验失败 4xx（P3）

- **P2（越权）**：`operation_tasks` 新增可空 `shop_id`（索引 `idx_operation_tasks_tenant_shop_updated`），运营任务按订单/采购/异常口径纳入店铺 scope：admin 不受限；operator/readonly 仅见授权店铺任务，无授权店铺列表为空；`shop_id IS NULL`（租户级）仅 admin 可见；越权/跨租户直读（含 drafts/approvals/attempts/events 子资源与全部写路径）统一 404 不泄露存在性。创建接受可选 `shopId`（admin 可省略=租户级；非 admin 必须绑定授权店铺，缺失 400、越权 404）。存量 backfill 随迁移执行：`source_reference` 命中同租户店铺 id → 归属该店铺；命中商品 id 且发布关联唯一店铺 → 归属该店铺；推导不出保持租户级。
- **P3（错误码）**：execute/retry 适配器 payload 校验失败由 HTTP 500/50000 改为 HTTP 400、业务码 40001、`errorCode=execution_validation_failed`（适配器 permission/state/idempotency 类分别映射 403/409）；失败 attempt 创建与 finalize 行为不变。
- 回归：`api_scope_test.go`（admin/operator/readonly/跨租户四口径读写 + backfill + execute 4xx HTTP 层断言）+ 权限矩阵套件新增 `TestOperationTaskStoreScope`；docs/api.md 新增运营任务节、docs/permission-matrix.md、docs/P8_OPERATION_TASK_API.md 同步；前端 `operationTasks.ts` 类型补 `shopId`（无 UI 行为变更）。基于 integration/round55-preview（收敛 #79–#119）。

### 变更记录（2026-08-02）第 63 轮：PlatformTag/StatusTag 语义组件 + 首页信息设计打磨（R62 视觉走查 P1）

- 新增共享 `PlatformTag`（`admin/src/components/ui/PlatformTag.tsx`）：平台内部枚举 → 中文名 + 品牌色 Tag（douyin_shop→抖店/volcano 等），整体不换行，空值兜底 `—`，未知枚举保留原值；复用 `constants/platformLabels.ts` 集中映射。
- 扩展共享 `StatusTag` 集中映射（matched/partial/completed/manual_review/verified/deferred/ready/not_ready/ready_with_warning/active/revoked 等），`copywriting.ts` 补对应中文文案；替换全站裸枚举直出处（订单/异常/SKU 匹配/详情、库存告警与同步、选品任务与详情、货源供应商、任务失败中心、客服、商品草稿/刊登覆写、设置用户/安全、采购、运维备份/恢复/灾备），仅改展示层，不改 API/权限/数据口径。
- 首页 `Dashboard/ProductOperations`：漏斗转化率超 100% 时封顶展示 100% 并以「超额 +N%」另行标注（Tooltip 保留真实转化值），进度条宽度封顶；待办卡改用 antd token（间距/警示色）与 tabular-nums 数字排版。
- 测试：新增 `PlatformTag` 单测、扩展 `StatusTag` 单测；新增 E2E `round63-semantic-visual.spec.ts`（漏斗超额标注、375 无溢出、供应商/任务失败中心语义 Tag）。

### 变更记录（2026-08-02）第 64 轮：报表图表规范化（R62 视觉走查 Top5 第 5 项收口）

- 新增 `admin/src/constants/chartTokens.ts`：图表系列色取自 AntD 主题 seed（首色 = `colorPrimary`）、涨跌语义色（沿用订单预估毛利 #3f8600/#cf1322）、`formatCount`（千分位）、`formatAmount`（千分位+两位小数+币种前缀）、`tabularNumsStyle`。
- `/orders/reports`：Line/Column 图表统一 `scale.color.range` 配色 token，y 轴/tooltip 千分位与金额格式，legend 置顶，合计卡 tabular-nums；堆叠销售额图 tooltip 取原始 `amount` 字段避免展示堆叠累计值；空态/骨架/375px autoFit 保持。
- 首页经营概览统计卡与报表页口径一致：计数 `formatCount` + tabular-nums，销售额 `formatAmount`（如 `USD 1,234.50`）。
- 订单列表 cost-estimates 批量接口 `data:null` 空值防御（`out?.items ?? {}`），不白屏；预估毛利涨跌色改引用 chartTokens。不改任何 API/权限/数据口径。
- 测试：新增 `chartTokens` 单测（9 用例）与 E2E `round64-charts-visual.spec.ts`（千分位合计+双图渲染、空数据引导、375px 无溢出、cost-estimates `data:null` 回归）。

### 变更记录（2026-08-02）第 65 轮：R62 视觉走查 P2 批量收口（仅 antd token/配置，无 API/权限/口径变更）

- 首页经营概览窗口标签映射补 `last7d`/`last30d` 别名，任何后端 key 口径均展示中文（今日/近 7 日/近 30 日），不再英文直出。
- P2-1：今日待办卡仅首个待办用 primary 按钮，其余降级 default，消除满屏蓝主按钮。
- P2-2/P2-12：订单异常工作台 8 张裸 Statistic 卡改共享 `MetricCard`（>0 时 danger/warning 语义色），栅格改 xs12/md6 均衡 2 或 4 列，消除残行。
- P2-3：AI 工作台优先级 Tag 仅 P0 用红色（P1 orange、P2 gold、P3 default），消除红墙。
- P2-7：商品草稿列表无封面占位由灰字「无图」改图形图标占位（保留 aria-label）。
- P2-9：登录页删除 3 张空白幽灵装饰卡（decor-card）及关联动画。
- P2-11：客服会话「买家」列加 ellipsis，脱敏名不再折行。
- chartTokens.heightCompact 落地：抽共享 `useWideScreen` hook（TmProTable 同步复用），报表页 <768px 图表用紧凑高度 220。
- 测试：新增 `aiOperationWorkbench` 优先级色彩单测与 E2E `round65-visual-p2.spec.ts`（窗口标签中文、待办按钮层级、异常统计卡语义色+375 无溢出、报表紧凑高度、无图占位、登录页无幽灵卡）。

### 变更记录（2026-08-02）第 68 轮：合并前大回归 v4 P2 收口（P2-1/2/3/5 + ENV-1，跳过 P2-4 由并行分支处理）

- P2-1：新增 `admin/src/constants/imageFallback.ts` 统一破图占位（内联 SVG），商品草稿列表/选品明细/刊登批次/草稿抖店预览与规格图的 antd Image 补 `fallback`，DEMO 失效图 URL 不再裸破图。
- P2-2：任务中心 mapper 用户可见标题（订单同步/客服消息同步/刊登/库存同步）改用 `opslabels.PlatformLabel`，AI 工作台待办描述随之显示「抖店」；DTO `platform` 字段保持原始编码，不改 API 口径。补 mapper 标题回归测试。
- P2-3：订单异常工作台严重程度列改用既有 `SEV_LABEL` 中文映射（筛选枚举与技术详情不变）。
- P2-5：`AppMessageBridge` 扩展 patch antd 静态 `Modal.*` 到 App context（消除静态方法 context warning）；订单列表/详情的明细与物流 Modal 由 `destroyOnHidden` 改 `forceRender`（打开前 `resetFields/setFieldsValue` 不再触发 useForm 未连接 warning，openXxxModal 每次打开仍重置字段）。
- ENV-1：Docker 镜像已内置 postgresql-client-16；`docs/docker-deployment.md` 补宿主机部署 pg_dump 主版本 ≥16 要求与自检说明；backup service 执行前 `exec.LookPath` 预检，缺失时给出友好错误（含 POSTGRES_PG_DUMP_PATH 提示），备份校验口径不变。

### 变更记录（2026-08-03）第 70 轮：客服会话子资源鉴权收口（R69 QA）

- 修复 R69：AI 客服越权/跨租户会话的子接口（`GET /customer/conversations/:id/messages`、`GET /customer/conversations/:id/ai-suggestions` 等）此前返回 200 空数据。现全部会话子资源读写路径（messages 读写、ai-suggestions 读写、mark-replied、ai/generate-reply、send-platform-message、reply-suggestions/ai-suggestions 建议操作）先按父会话 tenant+店铺 scope 校验（`customerchat` 新增共享 `findScopedConversation` / `findScopedSuggestion`），越权/跨租户统一 404，不泄露存在性；正常授权路径与 DTO 不变。
- 客服消息同步同口径：`POST /shops/:id/sync-customer-messages` 校验店铺 tenant+店铺 scope；`customer/message-sync/tasks` 列表按租户过滤（带 shopId 时叠加店铺 scope），`tasks/:id`、`tasks/:id/retry` 越权 404；新建同步任务写入店铺 `tenant_id`（此前恒为 0）。
- 全站复扫收口同类缺口：`GET /products/:id/skus/:skuId/inventory-logs`、`GET /products/:id/publication-skus`（inventory）、`GET /products/:id/ai/tasks`（product）补父商品 tenant scope，越权 404。
- 回归单测：customerchat 子资源三角色（同租户 admin / 跨租户 admin / operator 店铺授权与否）+ 越权写无副作用；customersync 任务 scope；inventory / product 跨租户 404。
- 复扫发现、本轮未收口（越权目前仍可按裸 ID 读，需后续按同口径收口）：`GET /products/:id/sources`、`GET /product-source-skus/:id/price-history`（sourcing）、`GET /image/tasks/:id/items`（imagetask）、`GET /ai-operation-batches/:id/tasks`（aioperationbatch）、`GET /products/:id/publications`、`GET /product-publications/:id/douyin/sku-bindings`（productpublish）；`ordersync` 的 `POST /shops/:id/sync-orders` 未做店铺 scope（GET/retry 已有 tenant scope）。→ 已于第 71 轮全部收口（见下）。

### 变更记录（2026-08-03）第 71 轮：业务子资源鉴权收口（R70 复扫清单）

- R70 复扫清单逐项收口，口径与订单/采购/运营任务/R70 客服一致（子资源先校验父资源 tenant+店铺归属，越权/跨租户统一 404，不泄露存在性；正常授权路径行为与 DTO 不变；操作日志 `ApplyStoreScopeOrNull` 的 tenant 级例外未复制到业务数据）：
  - sourcing：`GET /products/:id/sources`、`GET /product-source-skus/:id/price-history` 补父商品 tenant scope（价格历史沿 source SKU → product source → product 链校验；货源行 `tenant_id` 现网恒为 0，故以父商品为准）。
  - imagetask：`GET /image/tasks/:id/items`、`DELETE /image/tasks/:id/items/:itemId` 补父任务关联商品 tenant scope（`ImageTask` 无租户列；无商品关联的存量任务无租户归属，保持与任务详情一致的可见性）。
  - aioperationbatch：`GET /ai/batches/:id`、`/:id/tasks`、`retry-failed`、`apply-results` 统一走新 `GetScoped`，按批次创建人（`created_by` → `admin_users.tenant_id`）校验租户（批次无租户列；无创建人的存量批次按租户 0 归属）。
  - productpublish：`GET /products/:id/publications` 补父商品 tenant scope 并按店铺 scope 过滤发布行；`GET /product-publications/:id/douyin/sku-bindings` 及 sync/手工绑定/解绑写路径统一走 `loadDouyinPublicationScoped`（父商品 tenant + `EnsureStoreVisible` 店铺 scope）。发布行无租户列，按父商品归属校验。
  - ordersync：`POST /shops/:id/sync-orders` 补店铺 tenant+店铺 scope（与 GET/retry 口径一致），新建任务写入店铺 `tenant_id`（此前恒为 0）。
- 回归单测：五个模块各补三角色（同租户 admin / 跨租户 admin / operator 店铺授权与否）+ 跨租户 404 + 越权写无副作用的 `subresource_scope_test.go` / `task_scope_test.go` 同款用例。
- 无「不宜收口」项；`GET /ai/batches`（列表）无租户过滤属遗留（批次表无租户列，需建模后收口），已记录为后续复扫项。→ 已于第 72 轮收口（见下）。

### 变更记录（2026-08-03）第 72 轮：AI 批次租户建模（R71 遗留收口）

- `ai_operation_batches` 新增 `tenant_id` 列（默认 0、`json:"-"` 不进 DTO；索引 `idx_ai_op_batches_tenant_created`）；创建批次写入当前租户。
- 存量 backfill（`database.migrateRound72AIBatchTenant`，随迁移自动执行）：按 `created_by` → `admin_users.tenant_id` 推导；推导不出（无创建人/创建人已删）保持租户 0（legacy 单租户桶，不放大可见性）。
- `GET /ai/batches` 列表接入 `adminperm.ApplyTenantScope`（此前无租户过滤）；`ensureBatchVisible` 改按 `tenant_id` 列校验，未 backfill 的 tenant-0 且有创建人的行回退按创建人租户（与 R71 `GetScoped` 口径一致）；详情/子资源跨租户仍统一 404。批次无店铺维度，各角色同租户口径一致。
- 回归单测：`TestAIOperationBatchTenantColumnScope`（三角色列表过滤 + 跨租户 404 + 缺租户上下文报错 + backfill/回退口径）；docs/api.md「AI 批次租户口径（round72）」与 docs/permission-matrix.md「round72」已登记。

### 变更记录（2026-08-03）第 81 轮：平台租户管理最小闭环（R80 开租缺口收口）

- 新增 `platformtenant` 模块与 `tenants` 表：`GET/POST /api/v1/platform/tenants`，仅平台管理员（最保守判定：`tenant_id = 0` 且 role=admin）可列出/创建租户；创建时事务一次建好租户 + 初始管理员（bcrypt，落新租户 admin），开租写操作日志 `tenant.create`（不含密码）。
- 迁移含 `tenants` id 序列与 `admin_users.tenant_id` 存量对齐（`syncTenantsIDSequence`），避免与 SQL 手工造的租户冲突；`/auth/register`、「新建用户继承创建者租户」口径不变；demo seed 不动。
- `/auth/profile` 增补 `tenantId`；Admin 设置中心新增「平台租户」页（列表 + 创建弹窗），菜单/路由/页面三层按平台管理员判定隐藏（readonly/operator/非 tenant0 admin 不可见不可用）。
- 权限矩阵登记两条新路由（四 persona 均 forbid，统一 403）；正反向行为由 `platformtenant/api_test.go` 与前端 `permission.test.ts` 覆盖；docs/api.md、docs/permission-matrix.md 已同步。
- 遗留：租户目前无停用/改名/删除能力（最小闭环，计费/自助开租不做）；权限矩阵 harness 尚无 tenant0 平台管理员 persona（正向路径由模块测试覆盖）。

### 变更记录（2026-08-03）第 77 轮：UX 复核 v3 Top5 展示层收口（P1×2 由并行分支处理）

- 时间列短格式统一：`formatTime` 新增 `formatDateTimeShort`（MM-DD HH:mm）与共享组件 `DateTimeText`（短格式 + Tooltip 完整时间）；订单/商品草稿/运营任务/失败中心/AI 工作台/客服会话列表时间列接入并收窄列宽（详情页保留完整 `formatDateTime`）。
- 状态映射巡检（v3 P2-2/P2-4）：运营任务中心「最新执行状态」由误用任务状态映射改为 `OperationAttemptStatusTag`（failed/succeeded 中文语义 Tag）；`copyableText` 对 `-`/`—` 占位值不再渲染复制图标；客服会话「平台」列改用 `PlatformTag`（v3 P2-3 草稿来源 collect 已在 R76 收口）。
- 移动端首页待办收纳（v3 P2-6）：<768px 视口今日待办默认只展示前 5 条 + 「查看全部 N 项待办 / 收起」切换，缩短 375 首屏长度；宽屏行为不变。
- 报表 x 轴抽稀（v3 P2-5）：`chartTokens` 新增 `chartAxisXTickCount`（宽屏 10 / 紧凑 6）与 `formatDateTickShort`（YYYY-MM-DD → MM-DD），经营报表双图接入，移动端不再出现 30 标签竖排墙。
- 销售额舍入口径统一（v3 P2-1）：报表合计卡销售额由 antd Statistic `precision`（截断）改为与首页经营概览相同的 `formatAmount`（四舍五入），消除 171.40 vs 171.39 展示差；后端数据与 DTO 不变。
- UX 复核 v3 报告归档至 `docs/ux-review/UX_REVIEW_V3_REPORT.md`（响应 v3 流程建议：走查报告入仓可追溯）。

### 变更记录（2026-08-04）第 95 轮：安全审计复跑（R95）店铺授权/订单号/导入 跨租户收口

- 店铺授权面按租户+店铺 scope 收口（`shop/service.go` 新增 `findScopedShop`/`ensureShopScoped`）：`PUT /shops/:id/auth`、`POST /shops/:id/test-connection` 与各平台 OAuth（amazon/lazada/shopee/tiktok/douyin 的 authorize-url、callback、refresh、revoke、test、sync）此前按裸 ID 查店铺，跨租户可覆写他租户平台凭证；现统一 404（`PUT /auth` 的 record not found 也由 400 改 404，与既有越权口径一致）。worker 路径（nil gin context）不变。
- 订单号唯一键由全局改按租户（`idx_orders_tenant_order_no`，migrate_round95）：全局唯一索引让 duplicate key 成为他租户订单号的存在性探针，并可被跨租户抢占号段导致迁移导入失败；手工导入与迁移导入的重复判定同步补租户过滤。
- 迁移导入任务列表/详情补店铺 scope（`ApplyStoreScope` + `EnsureStoreVisible`），错误行 CSV 与商品/订单/采购/日销导出统一经 `pkg/csvsafe` 中和公式注入（`=`/`+`/`-`/`@`/Tab/CR 前缀加 `'`，纯数值不误伤）。
- 租户清退补 `import_job_rows`（按 `import_jobs.id` 级联）与残留校验表，双租户实测清退报告 total=0。
- 前端构建链 `pnpm.overrides` 补 `@babel/core ^7.29.7`、`@babel/runtime ^7.29.7`、`path-to-regexp@1 1.9.0` / `@8 8.4.0`（同大版本补丁）：pnpm audit 55→49，其余需跨大版本（vite/vitest/axios/immer/esbuild，均构建期依赖）列入 P2；govulncheck 0 命中。
- 回归单测：order `TestOrderSubresourceCrossTenant404`、`TestOrderNoUniquePerTenantNotGlobally`、`TestManualImportDuplicateIsTenantScoped`，shop `TestShopAuthRoutesAreTenantScoped`，`pkg/csvsafe` 单测；权限矩阵契约全量复跑通过。

### 变更记录（2026-08-03）第 78 轮：安全审计复跑（R73）跨租户越权收口 + Go 补丁工具链

- sourcing 写/列路径改按请求租户收口（`scope.go`：supplier/source/sourceSKU/switchEvent/product 可见性校验；service 方法改收 `*gin.Context`）：跨租户 supplier 改删、source 改删/设主/刷新、SKU 映射保存删除、切换建议采纳/忽略统一 404；`GET /suppliers`、`/product-source-alerts`、`/product-sources/orphans`、`/source-switch-events` 补租户过滤（此前返回全量）；新建 supplier/source/SKU/价格历史写入 `tenant_id`。
- imagetask 详情/写路径按关联商品租户收口（`scope.go`：`EnsureTaskVisible` / `EnsureProductVisible` / `EnsureTaskItemVisible`）：`GET /image/tasks/:id`、`/ai/image/tasks/:id`、`translate-edit-state`、`retry`、`manual-render`、`apply`、item `save-to-product` / `set-as-main`、任务创建与图片评分的商品参数统一跨租户 404；全局任务列表按本租户商品过滤（无商品关联的存量任务保持可见）。
- productpublish：`loadProductForPublish` / `loadProductsForBatch` 补 `tenant_id`，`publish-targets` / `check` / 批量检查与建草稿跨租户统一 404（此前 200 泄露发布可行性与店铺清单）；发布目标店铺清单与目标校验按租户过滤（跨租户店铺一律 blocked 且不回显店铺名）；`CancelTask` 由裸 ID 查询改租户内查询（此前可取消他租户 pending/running 发布任务）。
- Go 工具链升到 1.25.12（`backend/go.mod`、`go.work`）：govulncheck 由 27 条标准库符号命中降为 0。
- 前端构建链 `pnpm.overrides` 补 `vite ^4.5.14`、`postcss ^8.5.18`（同大版本补丁，dependabot major 封禁范围外）；vitest / umi 系高危均为构建期依赖且补丁需跨大版本，按封禁保留并列入清单。
- 回归单测：sourcing `TestSourcingWritesScopedByTenant`、imagetask `TestImageTaskDetailAndApplyScopedByTenant`、productpublish `TestPublishTargetsScopedByTenant` / `TestCancelTaskScopedByTenant`；权限矩阵契约全量复跑通过（新增路由无漏登记）。

### 变更记录（2026-08-03）第 81 轮：安全审计遗留 P2 收口（刊登批次租户建模 + 越权 404 统一 + 前端 401 竞态）

- `product_publish_batches` 补租户建模（口径同 round72 `ai_operation_batches`）：`tenant_id` 列 + `idx_publish_batches_tenant_created` 索引 + 按 `created_by` backfill（`migrateRound81PublishBatchTenant`，推导不出保持租户 0）；创建（单/多商品 create-drafts）写入当前租户；列表 `ApplyTenantScope` 过滤，详情/retry-failed/cancel-pending/`retryFailedOnly` 回放按 `tenant_id` 校验（tenant-0 行回退按创建人租户），跨租户 404，DTO 不变。
- 发布任务越权口径统一：tasks `:id/retry|cancel|recover*` 与批次 retry/cancel 对跨租户/不存在对象由 400 统一为 404（不泄露存在性）；`recover` 增加租户归属前置校验；同租户业务校验 400、同租户非创建者 403 口径不变。
- 前端 401 竞态收口：`sessionGuard` 的 `requireRelogin` single-flight 扩展到「弹窗未注册」窗口（硬刷新首屏并发 401 等待 `SessionExpiredModal` 注册后共享同一次重登引导，超时兜底 false）；`redirectToLoginPage` 并发去重只跳转一次。
- 回归：后端 `TestPublishBatchScopedByTenant` / `TestFailedTargetsFromBatchScopedByTenant` / `TestPublishMutationEndpointsCrossTenant404`；前端 sessionGuard 新增硬刷新并发 401 用例；docs/api.md、permission-matrix.md「round81」、PUBLISH_BATCH_MIGRATION.md 已登记。Docker 双租户实测（PostgreSQL）：批次详情/重试/取消与任务 retry/cancel/recover 跨租户全部 404，列表互不可见。

### 变更记录（2026-08-03）第 82 轮：平台租户治理（停用/启用/改名）

- 租户生命周期（不做删除）：`tenants` 表新增 `status`（active/disabled）；新增 `PUT /platform/tenants/:id`（改名）、`POST /platform/tenants/:id/disable|enable`；tenant 0 不可停用/改名（400）、不存在 404、重名 400；全部操作写操作日志（`tenant.rename` / `tenant.disable` / `tenant.enable`）。
- 停用强制：登录（legacy/secure）、refresh 轮换、`ValidateSessionAccess`（每次 Bearer 请求）三处统一检查租户状态，租户停用返回 401 `AUTH_TENANT_DISABLED`（前端中文提示「租户已被停用」），已有会话下次请求即失效；无 `tenants` 行的 legacy 租户与 tenant 0 恒为 active（fail-open，避免误锁全站）。
- 权限矩阵：harness 新增可选 persona `platformAdmin`（tenant0 admin），平台租户 5 条路由全部登记（platformAdmin allow、四常规角色 forbid 403）；模块证据 `auth/tenant_state_test.go`、`platformtenant/api_test.go`。
- Admin 前端：平台租户页新增状态列与改名/停用/启用入口（Modal 二次确认，停用为危险操作文案），仅平台管理员可见；E2E `round82-tenant-govern.spec.ts`（写请求 mock + 取消不发请求 + 非平台角色 403）。
- R81 遗留 UX：操作日志「路径」「说明」列补数值列宽（220/240），按 TmProTable 列宽口径参与横向滚动估算，默认视口不再被挤出。
- docs/api.md、docs/permission-matrix.md 已同步。

### 变更记录（2026-08-03）第 83 轮：双租户全链路隔离实测 + 仪表盘/库存聚合租户收口（P1）

- 双租户实测（docker compose 全栈 + seed:demo:full + 平台租户页正规开租租户 B）发现：运营仪表盘聚合数值跨租户泄露（商品/客服/刊登/库存等计数含他租户数据）；代码走查同时发现 `GET /inventory/alerts` 与 `POST /inventory/stock-settings/batch-preview|batch-update` 无租户过滤（后者可跨租户批量改库存阈值）。
- 修复：`operationdashboard.Scope` 新增 `applyTenantColumn` / `applyTenantViaProduct` / `applyTenantViaShop`，Summary/Exceptions/Recent 全部聚合查询按可信租户限定；库存 `buildSKUAlertBaseTX` 支持可选 `TenantID`，三个库存端点 handler 注入当前租户。详见 docs/permission-matrix.md「round83」。
- 回归：`operationdashboard` / `inventory` 新增 dry-run 租户谓词单测；`go test ./...` 全量通过。

### 变更记录（2026-08-03）第 83 轮：草稿创建脏数据收口 + 引导管理员租户口径 + React warning 清理（R82 遗留 P2）

- P2 修复：`POST /api/v1/products` 手工新建草稿此前对 operator 先落库后做可见性校验，校验失败返回 400 但残留孤儿行。现 `product.Service.Create` 在写入前做 principal 校验：新草稿无店铺关联，仅 admin 可见/可建，非 admin 统一 403 且不落库；权限矩阵 operator 由 allow 改 forbid（见 docs/permission-matrix.md「round83」）。前端草稿列表「新建草稿」按钮改为仅 admin 显示。
- 引导管理员租户口径：设计意图确认为「引导账号 = tenant 0 平台管理员」（平台租户治理入口）。代码默认本就是 0，本轮把 `.env.example` / `.env.docker.example` 的 `ADMIN_BOOTSTRAP_TENANT_ID` 由 1 收口为 0，并在 docs/env.md 说明（显式 >0 配置仍兼容遗留单租户部署；仅 admin_users 为空首次创建生效，不迁移存量）。
- React console warning 清理（R82 报告 5 条）：Settings/Users 店铺权限弹窗 `Form.List` 行内 `{...field}` 展开重复传 `key`（4 条 duplicate key warning）改为解构 `{ key, name, ...restField }`；「新建用户」弹窗 `destroyOnHidden` 导致 `useForm` 未连接 warning，改为 `forceRender` + 取消时 `resetFields`。
- 回归：`product/create_scope_test.go`（operator/readonly 403 + 0 残留行、admin 正常创建）；权限矩阵契约复跑；go fmt/vet/build/test、pnpm test:frontend/build:admin/test:contracts；Docker 实测两条动线（operator 建草稿 403 无残留、bootstrap 账号落 tenant 0 可开租）。
### 变更记录（2026-08-03）第 84 轮：AI 工作台/失败中心/采集链路季度回归 + P1 修复

- R84 季度回归（docker compose 全栈 + seed:demo:full，#182/#183 本地叠加；租户 B 正规开租补测）：AI 优化草稿降级提示、AI 批量任务/批次/子任务、失败中心筛选与 canRetry、选品全链路、客服建议人工确认动线、采集创建/失败终态/R72 假草稿/R73 规则模板直填、三角色 scope、375/1440 视口、硬指标（console error/panic/5xx/42703=0）通过。
- P1 修复：失败中心客服失败（`customer_failure`）此前统一任务读取与重试均不识别该类型（重试恒 400 unknown task type）。`taskcenter` 补 `unifiedOne`/`RetryFailure`/`sourceTableForType` 分支，重试 = 对原会话重新生成 AI 建议（人工确认后才可发送，无自动外发）；`Retryable` 口径收紧为仅 `customer_reply_generate_failed` 可重试，发送失败/权限/未授权类不再显示可重试。
- P1 修复：选品任务创建此前对 tenant 0（平台管理员上下文）静默落库，业务 worker 拒绝 tenant<=0 导致任务永远 pending。现创建入口校验租户，非正租户返回 400 `TENANT_CONTEXT_MISSING` 且不落悬挂任务（与采集入口既有闸门口径一致）。
- P2 修复：`confirmSensitiveAction` 的 `onOk` 统一 catch 错误并 `message.error` 弹出中文提示（重试等确认类操作失败不再静默）。
- 回归测试：`taskcenter/customer_failure_retry_test.go`（分类 retryable 口径 + RetryFailure 识别 customer_failure）、`selection/create_tenant_gate_test.go`（tenant 0 → 400 零残留、正租户不受闸门影响）。
- 遗留 P2 清单：无效 URL 采集失败原因英文直出（collector 1688 provider 校验错误未中文映射）、readonly 写按钮部分仅接口 403 未做 UI disabled、readonly 时间线空状态缺说明、批次详情子任务失败原因展示较弱；采集超时终态未自然观察（真实失败 ~22s 即达终态）。

### 变更记录（2026-08-03）第 85 轮：生产部署演练复跑 + 生产 tenant 0 平台管理员 403 修复（P0）

- 生产演练复跑（APP_ENV=production + docker-compose.prod.yml + Caddy 内部 CA 假域名，从零部署约 4 分钟）发现 P0：`secure_session` 生产模式下，引导平台管理员（tenant 0）任何带 token 请求被 JWT 中间件的 `ResolveRequestTenantID` 以 `PRODUCTION_TENANT_FALLBACK_FORBIDDEN` 拒绝（403），平台租户治理（开租/停用/改名）在生产完全不可用；#181 口径此前仅在 development/test 验证过。
- 修复：`middleware.BearerAuthWithDB` 对 tenant 0 claim 增加 DB 复核（`admin_users` 行属 tenant 0 且 active 才放行，authSource 标记 `platform_tenant_token`）；未知/停用账号维持 403，业务租户（>0）路径不变，业务侧 `RequireTenantID` 仍拒绝 tenant 0。回归单测 `middleware/jwt_platform_tenant_test.go`。
- 文档收口：`.env.prod.example` `ADMIN_BOOTSTRAP_TENANT_ID` 由 1 改为 0（与 .env.example/.env.docker.example、docs/env.md 口径一致）；production-launch-checklist 增补开租/会话治理/备份校验下载验证项与 2026-08-03 复跑结论。
- 演练验证通过：开租→新租户登录、secure_session 续期/登出失效、备份创建→verify（checksum/pg_restore_list/manifest/encryption 全 passed）→下载 SHA-256 一致、生产禁 restore、日志无敏感信息、登录页 FCP≈330ms、懒加载 chunk 全部 200。

### 变更记录（2026-08-03）第 84 轮：设置中心/用户与店铺授权深度回归 + P1/P2 修复

- R83 全栈回归（docker compose + seed:demo:full，#179/#180 本地叠加）：用户管理全动线、平台租户治理、设置子页、readonly/operator 口径、三视口通过；发现 P1「被删用户旧会话下次请求触发未处理 Promise 拒绝整页红屏遮罩」与 P2「店铺授权弹窗重复 key 告警」「缺少改密码入口」。
- P1 修复：`app.tsx` 会话守卫在跳登录页兜底路径改为悬挂原请求 Promise（不再向页面抛出无人消费的 401 错误），旧会话失效体验为静默跳转登录页。
- P2 修复：店铺授权 `Form.List` 不再向 `Form.Item` 透传 `key`（消除 React 重复 key/AntD 告警；与 main 上 #181 同源修复已合并取 `restField` 写法）。
- 新增用户改密码：`POST /admin/users/:id/reset-password`（≥6 位、bcrypt、`token_version+1` + 吊销全部 secure 会话/refresh token 使旧会话失效且不可 refresh 复活、操作日志 `user.password.reset`、权限矩阵已登记 admin-only）；Admin 用户管理页新增「改密码」入口；E2E `r83-users-reset-password.spec.ts` + 后端 `reset_password_test.go`。
- docs/api.md 已同步；权限矩阵契约全量复跑通过。

### 变更记录（2026-08-04）第 97 轮：报表本位币/手工汇率表按租户隔离（R95 审计 P2 收口）

- `PUT/GET /settings/report-currency` 由固定读写 tenant 0 平台级设置改为当前租户（`adminperm.TenantIDFromGin`，缺租户上下文 403）：每个租户配置自己的本位币与手工汇率表，互不影响；`fxrate.ManualProvider` 移除「租户无配置回退 tenant 0」语义，未配置租户回默认口径（本位币 CNY、空汇率表、外币列 `unconvertedCurrencies`）。报表/首页经营概览/CSV 导出/毛利估算读取本就按当前租户（或订单租户）传入，随 Provider 收口自动隔离。
- 存量迁移 `migrateRound97ReportCurrencyTenant`：启动迁移把既有 tenant 0 `report_currency` 配置复制到所有尚无该分组配置的现存租户（保持既有租户折算口径不变），tenant 0 自身保留（仍是合法的单租户/demo 租户）；新租户默认未配置。幂等（NOT EXISTS 判重）。
- 毛利估算兜底 `settings.pricing.default_exchange_rate` 的 tenant 0 回退保留：pricing 分组是平台级刊登定价默认值（settings 页写 tenant 0），非本轮报表汇率面；报表手工汇率表优先级更高且已按租户隔离。
- 回归测试：settings `TestReportCurrencyIsTenantScoped` / `TestReportCurrencyRequiresTenantContext`（双租户 PUT/GET 互不影响、无 tenant 0 写入、缺租户上下文 403）、fxrate `TestManualProviderTenantIsolation`（未配置租户不继承他租户/tenant 0 汇率）、Postgres 集成 `TestRound97ReportCurrencyBackfill`（迁移复制/不覆盖已配置租户/幂等）；order stats 测试种子改按租户写入。docs/api.md 已同步。
- R95（#206）P2 清单其余项：前端跨大版本依赖升级（vite/vitest/axios/immer/esbuild 等，均构建期依赖）仍按 dependabot major 封禁保留不动，理由与 R78/R95 一致（跨大版本升级风险大于构建期依赖的实际暴露面）。

### 变更记录（2026-08-04）第 94 轮：seed clean/verify 覆盖迁移导入产物

- `seed:demo:full:clean` / `verify`（含 `-prefix` 自定义前缀）扩展覆盖迁移导入产物：`import_jobs` + `import_job_rows` 按「文件名/批次标识带前缀，或导入到前缀（DEMO-）店铺」识别清理；由导入创建的草稿/订单沿用既有前缀口径（标题/SKU 编码/订单号带前缀）删除，真实导入历史不受影响。QA 不再需要手工 SQL 清零。回归测试 `TestCleanupRemovesMigrationImportArtifacts` / `TestCleanupCustomPrefixCoversMigrationImports`；docs/development.md、docs/migration-guide.md 同步。#201（报表多币种）未合并，demo seed USD 订单+汇率样本核对随该 PR 合并后另行处理。

### 变更记录（2026-08-04）第 93 轮：seed 与文档 Demo 账号口径统一

- `pnpm seed:demo:full`（Go seeddemo）新增幂等保证三个 Demo RBAC 账号（demo_admin / demo_operator / demo_readonly @trademind.local）存在且密码与文档一致；密码漂移时重置回文档值并递增 `token_version` 使旧会话失效。跨平台（不再依赖 PowerShell 的 seed-demo-permissions.ps1 才能拿到三角色账号），仅限非 production（seeder guard）。回归测试 `TestSeedEnsuresDemoAccounts`；docs/development.md、docs/DEMO_SEEDING_GUIDE.md 同步。

### 变更记录（2026-08-03）第 84 轮补充：会话吊销收口（P3）

- 删除用户（`DELETE /admin/users/:id`）同步吊销该用户全部 `auth_sessions`/refresh token（`user_deleted`），旧会话立即不可 refresh 续期；回归测试 `TestDeleteUserRevokesUserSessions`。
- `/auth/refresh` 增加 `token_version` 校验兜底：`auth_sessions` 新增 `token_version` 列（登录时快照），refresh 时与 `admin_users.token_version` 比对，不匹配即 401 并吊销会话（`token_version_mismatch`），口径与访问令牌 `ValidateSessionAccess` 一致；存量会话（列值 0）跳过校验不强制下线；正常续期与 secure_session 模式不受影响。回归测试见 `session_service_test.go`。
- 失效类操作统一口径梳理（删用户/改密码/改角色/改状态/店铺授权变更/租户停用）：均已递增 `token_version` 或直接校验，refresh 与访问令牌双链路兜底；详见 docs/P4_AUTH_SESSION_SECURITY.md「失效类操作统一口径」。

### 变更记录（2026-08-03）第 89 轮：平台租户清退删除（测试租户留存收口）

- 新增平台租户清退能力：`POST /platform/tenants/:id/purge`（提交后台清退任务）+ `GET /platform/tenants/:id/purge`（任务状态/报告）。仅 tenant 0 平台管理员可用；前置条件已停用；tenant 0 永不可清退；请求体 `confirmName` 必须与租户名完全一致；同租户进行中任务去重。
- 后台任务（`tenant_purge_tasks` 表，pending/running/succeeded/failed）在单事务内级联清理：先按父对象 ID 删除无 `tenant_id` 的关联子表（SKU/图片/订单项/客服消息/发布明细等），再经 information_schema 动态发现全部带 `tenant_id` 的表做 FK 感知（savepoint 重试）清扫，最后硬删 `tenants` 行；清理期间经 `operationtask.WithImmutableGuardsDisabled` 临时提升（Postgres replica session role）以删除 append-only 审计行。
- 清退后逐表计数校验零残留并将报告（`tables`/`total`/`verifiedAt`）持久化到任务行；残留非零任务判失败。业务操作日志随租户删除；平台侧审计（`tenant.purge.start|done|failed`）与任务记录保留在 tenant 0。
- 前端平台租户页对已停用租户展示「清退删除」入口，双重安全门（输入租户名 + 二次确认弹窗）；权限矩阵登记两条新路由（platformAdmin allow、四常规角色 forbid）；docs/api.md、docs/permission-matrix.md 已同步。
- 验证：模块回归 `purge_api_test.go` 全绿；Docker + PostgreSQL 实测开租→seeddemo 造数→停用→清退→86 表零残留，tenant 0/未停用/错误名称 400、不存在 404、tenant B admin 与 tenant0 operator/readonly 403。

### 变更记录（2026-08-03）第 87 轮：R86 生产演练 P2 收口（legacy token 收紧 + 统一 404 + 租户列表 createdAt + LE 签发注意事项）

- `secure_session` 模式（staging/production 强制）不再接受无 session 绑定的 legacy JWT，统一 401 + `AUTH_SESSION_BINDING_REQUIRED`，前端会话守卫引导重新登录；`legacy_local_storage`（开发/遗留部署）行为不变，迁移说明见 docs/env.md。回归测试 `jwt_session_binding_test.go`。
- 未知路由统一 JSON 404 envelope（`40401` + 中文口径），替换 Gin 裸文本 `404 page not found`（`api.RegisterNoRoute`）；前端未匹配路由已有统一 404 页（`admin/src/pages/404.tsx`，引导返回工作台）。
- 平台租户列表：隐式平台租户 0 的 `createdAt` 取最早平台管理员创建时间，不再空展示；回归测试 `list_created_at_test.go`。
- docs/production-launch-checklist.md 补「Let's Encrypt 真实域名签发注意事项」（DNS/端口/CDN 前置、staging CA 联调、速率限制、排查顺序、自动续期），仅文档不改代码。

### 变更记录（2026-08-03）第 85 轮：seed clean 自定义前缀 + collector 不可达设置页优雅降级（R84 P2 / R83 P3 收口）

- seeddemo 自定义前缀：`cmd/seeddemo` 新增 `-prefix`（默认 `DEMO-`），仅 clean/verify 生效（seed 仍只写 DEMO-，传自定义前缀直接报错）；前缀白名单校验（字母数字加连字符、以 `-` 结尾、禁 SQL LIKE 通配符）；`FullDemoSeeder.Prefix` 贯穿 Cleanup/VerifyClean 的全部 LIKE 条件，legacy 客服会话兜底（`F8 Demo%`/`Demo %` tenant-0 孤儿）与 DEMO 设置预设 remark 清理仅在默认 DEMO- 前缀下执行，避免自定义前缀误删；production 拒绝口径不变（cmd 层 + guard 双重）。回归单测 `fulldemo_round85_test.go`：前缀校验、QA- 清理幂等零残留、DEMO-/普通数据不受影响、seed/production 守卫。
- collector 不可达优雅降级：collector 代理传输层错误（连接拒绝/超时/DNS）由裸 502+50000 收口为 502 + 新业务码 `CodeCollectorUnreachable=50302` + 中文引导文案（`failCollectorProxy`，collector 业务拒绝仍保留原 message）；前端 `request.ts` 新增 `isCollectorUnreachableError`，设置中心采集页对 1688/拼多多/淘宝天猫登录态检测与打开采集浏览器失败识别该码，页面顶部渲染「采集服务未启动或不可达」warning Alert（含启动指引 + 重新检测按钮），不再弹裸错误 toast，采集参数表单不受影响可正常查看保存。回归：后端 `collector_unreachable_test.go`（unreachable 走 50302 引导、业务拒绝不被掩盖）、前端 request.test.ts 补 2 用例。
- R84 报告其他杂项：R84 报告（第 84 轮变更记录）所列 P1/P2 已随 #182/#185 合入 main，本轮除上述两条外无其他待收口小杂项。

### 变更记录（2026-08-03）第 86 轮：R85 AI/采集季度复查 P2 UX 收口

- 采集失败原因中文化：`collectErrors.ts` 新增 `UNSUPPORTED_URL` 标签/建议映射，并识别 collector 原始英文消息 `url is not supported by source "1688"` 与 `INVALID_URL:*`（不改错误码契约）；补单测。
- readonly 写按钮 UI 收口：失败中心（行内重试/生成告警/更多、批量重试/忽略/已处理、抽屉登录浏览器/恢复/重试/生成告警）、AI 批次（重试失败/应用结果）、AI 图片任务（新建/快捷模板/重试/保存到商品/设为主图等）、AI 文案批量复核（重试/批量应用/撤销/复核弹窗写按钮）按既有口径隐藏；E2E `round86-ux-p2.spec.ts`。
- readonly 时间线空态：采集任务事件抽屉与运营任务审计时间线补中文空态说明。
- AI 批次子任务失败原因可读化：`mapAiTaskErrorText`/`AiTaskErrorText`（AI 错误码中文映射 + 悬浮查看完整原始原因），应用于 AI 批次详情/子任务表与文案复核详情（失败原因列 + 弹窗失败原因块，完整原始错误可展开复制）。

### 变更记录（2026-08-03）第 88 轮：恢复 operator 手工建草稿（带店铺归属）

- `POST /api/v1/products` operator 由 forbid 改回 allow（round83 收紧的产品口径补全）：请求体新增可选 `shopId`，operator 必填且须属其授权范围（不传 400、越权 404 不泄露存在性、仅 view 授权 403 40303），校验全部发生在写入前，被拒绝零落库；创建成功在同一事务写入 `product_platform_publish_configs` 关联，草稿按既有可见性口径对创建者可见。admin 保持现口径（`shopId` 可选）；readonly 仍路由级 403。
- 前端新建草稿弹窗增加「归属店铺」选择（operator 必填含中文引导、下拉由后端店铺 scope 只列授权店铺；admin 可选）。
- 权限矩阵 operator=allow 已登记并复跑契约测试；回归测试 `product/create_scope_test.go` 全场景重写（含零脏数据断言）；E2E 补 operator 建草稿动线。docs/api.md、docs/permission-matrix.md 已同步。

### 变更记录（2026-08-03）第 92 轮：迁移通道（店小秘/马帮 商品与历史订单导入向导）

- 新增 `migrationimport` 模块（`import_jobs`/`import_job_rows` 表）：`POST /imports/parse|validate|commit`、`GET /imports`、`GET /imports/:id`、`GET /imports/:id/errors.csv`。CSV（UTF-8/GBK）与 XLSX（标准库 zip+xml 解析，无新依赖）上传，单批 ≤1000 行、文件 ≤10MB；表头别名自动猜列（中英文）+ 来源格式识别（店小秘/马帮/自定义）+ 手工映射；逐行校验（必填缺失/批内重复/非法值）不入库。
- 商品导入：按商品名称聚合创建**草稿**（行=SKU，复用 product service 的店铺归属与 operator scope 校验；已存在 SKU 编码按重复跳过）。订单导入：按订单号聚合创建（platform=migration，来源状态映射内部枚举，收件人信息入 rawData/备注；已存在订单号重复跳过）；`POST /orders` CreateBody 新增可选 `remark`/`rawData`。幂等：同租户同 kind 同文件 sha256 只提交一次，重传返回原批次结果（replayed）。
- 导入历史批次化：每批记录总数/成功/失败/重复与错误行明细，错误行报告 CSV 下载（UTF-8 BOM）。
- 前端：新设置页 `/settings/migration`（上传→列映射→校验报告→导入结果四步向导 + 导入历史 Tab），商品草稿页「迁移导入」入口、订单批量粘贴弹窗升级为可跳转文件导入向导；readonly 只读提示。
- 权限矩阵登记 6 条路由（readonly 写 403）；契约测试补 6 端点；E2E `round92-migration.spec.ts` + 合成样例文件（店小秘/马帮风格，无真实数据）；docs/api.md 与 `docs/migration-guide.md`（含公开格式假设说明）同步。回归：`migrationimport` parse/service 测试全绿。

### 变更记录（2026-08-03）第 91 轮：打单发货物流闭环（物流商 / 运单 / 轨迹 Provider 预留 / 拣货单打印）

- 物流商管理：新增 `carriers` 表与 `carrier` 模块（租户隔离，`GET/POST/PUT/DELETE /api/v1/carriers`），按租户幂等预置国内常用快递（顺丰/京东/中通/圆通/申通/韵达/邮政EMS/极兔/德邦/其他，含轨迹 URL 模板），支持自定义新增与启停；预置不可删除只可停用。前端新增设置页 `/settings/carriers`。
- 运单模型升级：`order_shipments` 补 `carrier_id`/`carrier_code`/`tracking_url`（保留 `carrier` 名称快照，legacy 自由文本兼容）；传 `carrierCode` 时关联已启用物流商并按其规则宽松校验运单号（顺丰/京东/EMS 专用、其余通用 6~40 位）、自动补轨迹 URL。发货弹窗（列表/详情）升级为物流商选择（AutoComplete，自由文本兼容）。
- 批量粘贴发货升级：第三列物流商（代码/名称/前缀）+ `defaultCarrierCode` 默认物流商下拉，旧两列格式兼容（沿用「其他快递」）。
- 轨迹 Provider 预留：`providers/tracking`（`TrackingProvider` 接口 + `manual` provider，不接真实 API），`POST /orders/:id/shipments/:shipmentId/refresh-tracking` 端点返回 manual 口径；手工编辑物流状态推动订单在途→送达既有流转不变。
- 拣货/发货单打印：`GET /api/v1/orders/print/sheets?ids=`（≤50 单，店铺 scope）+ 前端打印页 `/orders/print`（订单+收件人+SKU 明细+物流商+运单号+贴单区，浏览器打印，非电子面单），订单列表勾选后「打印拣货单」入口。
- 权限矩阵登记 6 条新路由（readonly 写 403、operator 店铺 scope），docs/api.md / module-map / permission-matrix / provider.md 同步；demo seed 补物流商预置与顺丰运单样本；契约测试补 7 端点。回归：carrier `service_test.go`、order `carrier_shipment_test.go`。

### 变更记录（2026-08-04）第 96 轮：tenant 0 运营任务口径修复 + UX v4 P2 收口

- **tenant 0 语义理清**：读接口不再被 R85 #185「tenant 0 误建业务数据闸门」拦截（operationtask 各层 `tenantID <= 0` 改为 `< 0`，读仍严格按 `tenant_id` 隔离）；写接口保留生产闸门，新增 `Handler.AllowTenantZeroWrites`（router 按 `EnableDemoSeed && !IsProduction` 注入），demo/dev 全量环境 tenant 0 演示租户可完整使用运营任务中心，production tenant 0 写入仍 403 且零落库。回归测试 `tenant0_gate_test.go` 覆盖两种口径 + 三角色 + 跨租户隔离。
- **UX v4 P2 批次**：迁移向导「标题/宝贝标题/商品名/产品名」列名别名扩充（商品+订单）；导入结果页按成功/部分成功/失败视觉分层（部分成功明确失败行未入库可下载修正）；导入历史空态引导（跳导入向导按钮）；批量发货「手工扣库存」长说明改为单行可展开；打印页小屏提示建议桌面端打印；物流商轨迹 URL 模板列悬浮完整显示；demoseed 空汇率表时幂等补 USD 手工汇率（已配置不覆盖），报表演示不再出现未折算提示。v3 遗留 P2-7：订单列表批量工具栏移除 `alwaysShowAlert` 占位，选中行才出现。

### 变更记录（2026-08-04）第 93 轮：报表合规（多币种本位币折算）

- 新增 `providers/fxrate`（报表折算汇率表 Provider 抽象 + `ManualProvider`，不接实时汇率 API）：汇率语义「1 单位原币 = rate 本位币」，`math/big.Rat` decimal 精度、输出两位小数半入舍出。
- 新增 settings 分组 `report_currency`（provider/base_currency/rates，默认 manual/CNY/空表）与端点 `GET/PUT /api/v1/settings/report-currency`（`settings.manage`，readonly 403，操作日志 `settings.report_currency.update`；校验币种代码、正十进制汇率、去重、≤50 条），启动时幂等种默认值。
- 报表口径统一：`stats/sales`、`stats/daily` 返回 `baseCurrency`、每窗口/每日 `paidAmountBase` 与 `unconvertedCurrencies`（缺汇率币种不静默计入合计），原币桶补 `baseAmount`；CSV 导出每币种补「折算金额(币种→本位币)」列（无汇率留空）+「已付款销售额合计(本位币)/未折算币种」列。首页经营概览与经营报表共用同一口径。
- 前端：新设置页 `/settings/report-currency`（本位币 + 手工汇率表）；经营报表页折算合计/每日折算图/原币明细/未折算警示（含设置页跳转）；首页销售窗口卡展示折算合计与未折算标记。异常/待办统计不受影响。
- 权限矩阵登记 2 条新路由；demo seed 补 2 条 USD 已付款订单样本；docs/api.md、docs/provider.md 同步；后端补 fxrate/report_currency/stats 折算与 CSV 测试，前端补 service 单测与 E2E `round93-report-currency.spec.ts`。

### 变更记录（2026-08-04）第 99 轮：打印路由别名 + 升级演练 P2 清单收口

- **打印路由**：新增 `/orders/print-sheets` 别名路由（重定向到 `/orders/print` 并保留 `ids` 查询参数），不再被 `/orders/:id` 捕获显示「未找到订单」；E2E `round99-print-route.spec.ts` 覆盖深链/重定向/刷新。全站路由排查未发现其他静态路由被动态路由遮蔽（React Router v6 静态段优先），顺带修复草稿页死链 `/store/list` → `/shops/manage`。
- **升级演练 P2 收口**：① 手工建单撞同租户重复订单号时返回业务文案「订单号「X」已存在，请更换订单号」（不再透出裸 SQLSTATE 23505），回归测试覆盖，并修复新建手工订单弹窗提交失败无可见提示的问题（onFinish 捕获并 message.error）；② 升级期间全站不可用缓解：Caddy 对 admin 不可达返回「系统升级维护中」维护页、nginx 对 backend 不可用的 `/api` 返回统一 JSON 503（蓝绿/迁移解耦超出单机 compose 范围，见 upgrade-guide 陷阱 2）；③ 迁移吞错点（round72/76/81/97、p4_2、migrate.go）改为 WARN 日志 `database migrate step skipped`，迁移语义不变；④ `deploy-prod.sh --pre-upgrade-check`：全量备份 + R95 重复订单号预检一键完成（不部署）。

### 变更记录（2026-08-04）升级演练收口：R95 迁移预检 + 升级 SOP

- 生产升级路径演练（旧版本 #204 → 含 #206/#208 新版本，存量多租户/订单/运单/导入任务/汇率配置）暴露：同租户重复订单号会让升级在 GORM AutoMigrate 建 `idx_orders_tenant_order_no` 时以裸 `SQLSTATE 23505` 中断且全站不可用。新增 `preflightOrderNoTenantUnique`（AutoMigrate 最先执行）：检测到同租户重复订单号时输出可操作报错（列出 tenant_id/order_no 组合并指向清理指引），数据零改动；集成回归 `order_no_preflight_test.go`。
- 新增 `docs/upgrade-guide.md`：带数据迁移的版本升级 SOP（备份、预检 SQL、迁移中断处置、回滚路径与已知陷阱——含跨租户重复订单号出现后不可回滚到 R95 之前版本的陷阱）；docs/README.md、production-deployment.md 挂链接。

### 变更记录（2026-08-04）第 98 轮：PlatformTag migration 映射 + ProTable render 口径修复 + antd dev 警告清零

- `PlatformTag` / `platformLabels` 补 `migration`（迁移导入）平台映射，订单/打印页不再显示裸英文 `migration`。
- ProTable `render` 统一口径修复：批量发货 / 订单导入 / 采购批量回填 / 迁移向导 / 安全设置 / 运营任务中心等多处 `render` 参数误用（dom 与 entity 混用）修正，展示值与实体字段一致。
- antd dev 警告清零：ProTable 缺失/不稳定 `rowKey`、`Spin` 嵌套用法整治；E2E `round98-p2.spec.ts` + `PlatformTag` 单测回归。

### 变更记录（2026-08-04）第 101 轮：R100 回归 P2×2 收口（订单异常徽标口径 + 采购承运商口径）

- **P2-1 订单列表「异常」徽标口径对齐**：徽标计数原为原始聚合（未匹配+歧义+扣减失败），不感知 `order_exception_marks`，已处理/已忽略后仍显示 >0，点进异常工作台却为空。`countOpenExceptionRows` 改为镜像工作台默认打开行口径（sku_unmatched 仅非手工平台、含 skipped/缺 SKU，sku_ambiguous，扣减失败逐条计数，排除 handled/ignored 标记）；首页待办计数走 `ListOrderExceptions` 本就感知标记，无需改动。回归 `list_open_exception_test.go`。
- **P2-2 采购回填承运商**：当前 main 上单笔/批量/UI 回填「中通」均正常落库与展示（未复现丢失）；根因面为采购侧承运商为自由文本、与发货侧物流商档案口径不一致。采购详情回填弹窗改用共享 `CarrierSelect`（启用中的物流商档案，与发货侧同口径），并修复 `FillLogistics` 未落 `tenant_id` 的隐患；回归 `TestFillLogisticsPersistsCarrierAndTenant`。

### 变更记录（2026-08-04）第 100 轮：R99 季度复查 P2 收口 + 文档一致性巡检

- **R91–R99 里程碑索引**：R91 物流闭环 → R92 迁移导入 → R93 报表多币种折算 → R94 第二平台预研 + 毛利汇率口径统一 → R95 安全审计复跑 + 迁移预检/升级 SOP → R96 tenant0 运营任务口径 + UX v4 P2 → R97 汇率租户隔离 → R98 前端展示口径/警告清零 → R99 打印路由别名 + 升级演练 P2。各轮明细见上方对应变更记录。
- demo seed 商品主图由外链占位域名（`img.demo.trademind.local`，离线环境产生 `ERR_NAME_NOT_RESOLVED` 网络噪音）改为内联 SVG data URI 占位图（`demoImageDataURI`，中性底色 + DEMO-n 标签）；`imageFallback` 破图占位组件口径不变（仍用于真实失效外链）。
- 宿主机直跑 seed 的 `DB_HOST` 覆盖说明补入 `docs/env.md` 与 `docs/DEMO_SEEDING_GUIDE.md`。
- 文档一致性巡检：`docs/module-map.md` 补 R89/92/93/95 关联行（迁移导入、报表多币种、租户治理/清退、版本升级/迁移预检）；`docs/production-launch-checklist.md` 回滚章节补「带存量数据升级走 upgrade-guide + --pre-upgrade-check」指引；api.md/permission-matrix.md 抽查 R91–R99 条目（carriers、print/sheets、refresh-tracking、imports、report-currency、tenants purge）与实际路由一致，无需纠偏。

### 变更记录（2026-08-04）第 101 轮：季度复查 P0——自助注册租户隔离

- **P0 修复**：`POST /api/v1/auth/register` 此前把新注册用户落在 `tenant_id=0`（平台租户）且 `role=admin`，注册即获得 tenant0 全部数据视图（跨租户数据泄露）。现在注册时在事务内为每个新账号创建独立 tenants 行并绑定（`createRegistrationUser`），查询层隔离机制无需改动；回归测试 `register_test.go` 覆盖「非 0 租户、落库一致、两次注册不同租户」。
- 本轮季度复查其余项（发布/刊登降级、运营任务批量批准驳回、采集失败链路、选品全链路、三角色权限、响应式硬指标、clean 零残留）复查通过，P2 观察项见 QA 报告（PR 评论）。

### 变更记录（2026-08-04）第 102 轮：注册/租户生命周期安全复验——tenant 0 平台数据隔离 + legacy token 吊销

- **#214 复验通过**：Docker 双租户实测，自助注册为账号新建独立租户（落 `tenant_id=5`，非 0），租户/账号 1:1；平台租户管理、店铺跨租户授权、租户停用/清退口径保持正确。
- **P0 legacy token 吊销失效**：`legacy_local_storage` 模式下 access token 不校验账号状态与 `token_version`，改密码 / 停用 / 删除账号后旧 token 仍可用（此前仅 secure_session 模式生效）。新增 `auth.EnsureAccountActive`，JWT 中间件在租户解析后逐请求校验账号存在 / 未软删 / 状态 active / `token_version` 未被提升；同时修复 `LegacyMintToken` 硬编码 `token_version=1`（否则任何被提升过版本的账号登录后每个请求都会 401）。
- **P1 tenant 0 平台数据泄露面收口**：`GET /settings` 不再返回 tenant 0 平台配置（平台默认值仅服务端内部消费），`PUT /settings` 忽略请求体 `tenantId`、跨租户写 403；`/ops/*`（备份 / 恢复 / 发布 / 容灾）与 `ai_prompts` 写操作收紧为平台租户专属；AI 运营工作台、客服仪表盘聚合与 `product-skus/search` 补可信租户过滤；新增 `ProductRouteTenantGuard` 统一校验 `/products/:id` 子资源租户归属（跨租户 404）。
- **P1 采集规则 / 浏览器 profile 缺租户维度**：`collect_rules`、`collect_browser_profiles` 新增 `tenant_id`（AutoMigrate，默认 0，索引），列表 / 详情 / 增删改 / 启停 / 规则解析 / profile 注入全部按可信租户限定，跨租户 404。
- **P1 注册接口枚举防护**：`POST /auth/send-email-code` 对已注册与未注册邮箱返回一致的 `200 {ok:true}`（已注册不下发验证码，写 `status=skipped` 操作日志）并消耗同样限流额度；叠加单 IP 每小时 20 次限流以钝化批量注册。
- 回归证据：`permmatrix` 新增 `tenant_zero_test.go`（平台专属运维路由 / 提示词写 / settings tenant 0 读写 / 采集规则与 profile 跨租户 / 商品子资源守卫）、matrix.json 登记 23 条新口径；`middleware/jwt_account_state_test.go`（停用 / 缺失 / `token_version` 提升三类旧 token 401）、`auth/jwt_access_test.go` 补 token_version 携带断言。测试库全量 `go test ./...` + 权限矩阵契约通过（Actions CI 不作依据）。

### 变更记录（2026-08-04）第 103 轮：合并前集成大回归（#213–#216）P1 收口

- **P1 反枚举补口**：无 SMTP 配置时 `POST /auth/send-email-code`（scene=register）对已注册邮箱返回 200、未注册返回 503，仍可枚举。现将邮件配置检查（`checkEmailSettings`）前移到注册状态查询之前，无 SMTP 时对所有邮箱统一 503/50301 中文引导；SMTP 齐全时保持 #216 反枚举口径（均 200）。回归测试 `email_code_settings_test.go`。
- **P1 平台专属 /ops 路由前端对齐**：`/ops/backups|restores|releases|disaster-recovery` 加入 `PLATFORM_ADMIN_ROUTES`，业务租户菜单隐藏、直达显示统一语义页，与后端 `opsPlatform` 收口一致；前端权限单测补断言。
- `docs/env.md` 补充 secure_session 模式必须配置 `ADMIN_PUBLIC_URL`（否则写请求 403 `ORIGIN_NOT_ALLOWED`）。
- 集成回归结论：#213–#217 叠加后全量门禁（go/contracts/frontend/build/ui-copy/E2E 160 通过）与 docker 全栈动态回归（R57 主链路、双租户三角色、legacy/secure_session 生命周期、采集规则租户迁移、清退零残留、硬指标全零）通过；375px 视口受浏览器最小宽度限制以 500px 替代，demo seed 暂无采集规则/浏览器 profile 样例数据（P2 观察项）。

### 变更记录（2026-08-04）UX v5 全站走查 P1 修复：全局库存/告警跨租户泄露收口 + 无权限设置读 403 噪音清零

- **P1 跨租户数据泄露收口（后端）**：`GET /inventory/logs`、`GET /inventory/effects` 此前无租户过滤，新注册业务租户可看到 tenant 0 演示数据的库存流水（实测泄露）。现在流水按 `tenant_id` 限定、effects 经 `orders.tenant_id` 子查询限定；`GET /task-center/alerts` 同步按 `tenant_id` 过滤，告警 handle / ignore / unmark / notify 单条操作改为租户内查找（跨租户 404），杜绝按 ID 越权操作。回归测试：`inventory/global_feeds_tenant_scope_test.go`、`taskcenter/alerts_tenant_scope_test.go`。
- **P1 无权限设置读取 403 噪音清零（前端）**：operator / readonly 打开工作台、订单列表、选品任务、任务中心告警页时不再调用 `GET /settings`、`GET /settings/integrations/overview`（无 `settings.manage` 时跳过，页面正常降级），浏览器控制台不再出现 403 报错；`/settings/report-currency` 登记进 `ROUTE_PERMISSIONS`（需 `settings.manage`），operator / readonly 菜单不再出现该死路入口。
- **P1 采购单空态缺下一步指引**：`/procurement/orders` 空态此前仅"暂无数据"，与其他模块空态指引不一致；接入统一 `useListEmptyLocale`（新增 `purchaseOrders` 文案：先在订单列表标记已付款再一键生成采购单，含权限提示与"前往订单列表"按钮）。
- 已知后续观察项（P2）：告警扫描（`ScanAndGenerateTaskAlerts`）目前以平台视角运行、告警行 `tenant_id` 恒为 0，业务租户暂看不到自身任务失败聚合出的告警（失败明细在任务中心仍可见）；如需业务租户级告警需在 Upsert 链路补租户来源。

### 变更记录（2026-08-04）第 104 轮：settings 租户化第一批——AI 配置 + 采集配置（R102 P2-2 收口开工）

- 新增 `internal/pkg/tenantsettings`：按可信租户上下文（`security.TenantContext`）解析生效 settings，两种回退口径——**ai 整组回退**（租户配置任一自有 `api_key`/`*_api_key` 则整组以租户配置为准，杜绝租户模型跑平台 Key 的混流；未配置则整组回退平台默认）、**collector 逐 key 合并**（行为参数：租户覆盖单项，空值视为未设置继承平台默认）。
- JWT 中间件将 `TenantContext` 同步挂到 `c.Request.Context()`，服务层/Provider 层仅凭 `context.Context` 即可取可信租户；worker 链路沿用 `tasktenant.BuildWorkerContext` 既有注入。
- 替换读点：`ai` 组 14 处（AI Gateway Chat/TestConnection、aiproducttext、aioperationbatch、product ai_title、customerchat、collectruleai、settings test-ai/integration-overview、configstatus AI 项）与 `collector` 组 17 处（collect 批策略/超时/profile/auth 检查、collectruleai）改经 `tenantsettings` 解析；`PlainByGroup` 本身语义不变，平台级组照旧显式读 tenant 0。
- 无存量迁移：租户未配置时整组/逐 key 回退平台默认，行为与租户化前逐字节一致（tenant 0 demo 开箱不回退）；AES-GCM 加密与脱敏口径不变（租户行同样按 `is_encrypted` 加密、`List` 脱敏）。
- 测试：`tenantsettings` 单测（回退矩阵/合并/错误传播）、settings 双租户隔离+加密 DB 测试、AI Gateway 租户整组原子性测试、JWT 请求上下文注入回归测试。
- 剩余组清单与归类见 §7 遗留 60 条目（image/inventory/pricing 残留/告警通知/sourcing 待租户化；storage/mail/system/platform_* 保留平台级）。

### 变更记录（2026-08-04）第 105 轮：settings 租户化第二批——inventory/pricing/sourcing/alert_notify + 告警租户来源闭环（R104 续、#221 P2-1 收口）

- **inventory / pricing / sourcing 逐 key 合并租户化**：`tenantsettings` 新增 `InventoryPlain` / `PricingPlain` / `SourcingPlain`（行为参数组，租户覆盖单项、空值继承平台默认，口径同 collector）。替换读点：inventory 组 8 处（订单扣减策略、库存告警策略、批量任务上限、SKU 默认预警/安全库存 ×3、订单 SKU 匹配、configstatus 库存项）、pricing 组 4 处（定价规则/批量上限、productcheck 保护线、douyin 映射）、sourcing 组 1 处（换源规则）。**保留平台级**：库存同步平台限流（`inventory_sync_platform_rate_limit_*`，Redis 按平台节流保护共享平台 API 配额，租户不得覆盖，代码内注明）。
- **alert_notify 整组回退租户化**：`AlertNotifyPlainForTenant`——租户配置过任一 alert_notify 值则整组用租户配置（收件人/webhook 密钥不与平台混流），否则整组回退平台默认。通知发送（`NotifyGeneratedAlerts`）按**告警行归属租户**解析通知配置；SMTP 发信服务器（mail/email 组）仍为平台级基础设施。
- **#221 P2-1 告警租户来源闭环**：`UpsertAlertForFailure` 经 `resolveSourceTenant`（按 source 表直查 tenant_id，image/ai_text/ai_image/customer_failure 经商品/批次/店铺关联）落 `tenant_id`，bump 时自愈历史 tenant-0 行；新增 `migrateRound105AlertTenant` 回填历史告警（source 行已删的留在 tenant 0 桶，幂等）；`GET /task-center/alert-notifications` 补租户过滤（经告警归属，此前泄露其他租户通知目标/错误详情）；`POST /failures/:taskType/:id/generate-alert` 跨租户来源统一 404。
- **settings 写侧前端对齐**：`PUT /settings` 自 #216 起忽略/校验 `tenantId`（显式传别租户 403），但 Inventory/AlertNotify/Pricing/AI 设置页仍硬编码 `tenantId: 0`，业务租户保存必 403；改为省略 `tenantId`（后端落调用方租户），`toPutItems` 默认不再传 0。
- 存量迁移口径：settings 组本身无需数据迁移（未配置回退平台默认，tenant 0 demo 行为逐字节不变）；告警数据迁移见上。
- 测试：`tenantsettings` 合并/整组回退单测、taskcenter `alert_tenant_source_test.go`（Upsert 落租户+自愈、通知审计租户隔离）、PG 集成 `alert_tenant_backfill_test.go`（回填/孤儿/幂等）。
- 剩余待租户化（下一批）：`image` 组（26 处）、`taskcenter` 组（告警扫描/通知触发策略，扫描 worker 仍平台视角运行）、Storage/Email/System 设置页 `tenantId: 0` 硬编码（对应组保留平台级，暂不动）。

### 变更记录（2026-08-04）第 106 轮：settings 租户化第三批收尾——image 整组回退 + 告警扫描/通知触发策略租户化

- **image 整组回退租户化**：`tenantsettings.ImagePlain`——image 组混存多家图片 Provider 凭据（`*_api_key`）与其配套参数（base_url/model/size/workflow/超时），逐 key 合并会让租户参数跑平台 Key（或反之），故与 ai 组同口径整组回退：租户配置任一自有凭据（任一 `*_api_key`，或无 Key Provider ComfyUI 的 `comfyui_base_url`）则整组以租户配置为准；未配置则整组回退平台（tenant 0）默认。替换读点 26 处：providers/image factory ×6、imagetask ×10（含 test-image、翻译渲染/视觉链路）、aiproductimage ×5、aioperationbatch ×1、configstatus ×3、settings integration-overview ×1。
- **告警扫描/通知触发策略（taskcenter 组）逐 key 合并租户化**：`TaskCenterPlainForTenant`（行为策略组：告警生成开关/最低等级/分类开关/重复失败阈值、外发通知开关/等级/渠道/触发条件，租户覆盖单项、空值继承平台默认）。`UpsertAlertForFailure` 先解析来源租户再按该租户策略判定是否生成告警；`NotifyGeneratedAlerts` 按告警行归属租户逐租户解析 taskcenter 触发策略与 alert_notify 通知配置（均带批内缓存）。扫描 worker（`ScanAndGenerateTaskAlerts`）继续全局扫描（平台视角），告警按来源租户落桶，租户在告警列表看到自身聚合告警，tenant 0 视图为平台桶。**保留平台级**：扫描 worker 运行键 `enable_alert_scan_worker` / `alert_scan_interval_seconds`（单进程 worker 基础设施，仅平台管理员可配）。
- **前端对齐**：系统设置页（system + taskcenter）站点信息与「后台定时扫描」仅平台管理员可见可存；业务租户 admin 保存告警策略落自己租户（页面此前已省略 `tenantId`）。
- 存量迁移口径：image / taskcenter 组均无需数据迁移——租户未配置时回退/继承平台默认，tenant 0 demo 行为逐字节不变；越权口径沿用 #216（`PUT /settings` 一律写调用方租户、显式传别租户 403，`GET /settings` 仅返回本租户行）。
- 收尾核对：全仓 `PlainByGroup(ctx, 0, ...)` 残留仅剩明确平台级组——storage、mail/email、system、platform_*（平台应用凭据与发布配置 schema 组）、库存同步平台限流、taskcenter 扫描 worker 运行键；清单入 PR 描述。
- 测试：`tenantsettings` 单测（image 整组回退凭据判定含 ComfyUI、taskcenter 逐 key 合并/平台视图）。

### 变更记录（2026-08-04）第 108 轮：R107 遗留三小项收口（多账号切换误弹 / tenant0 演示重试 / 停库快照 500）

- **多账号切换旧 token 误弹「登录已过期」收敛**：切换账号动线中，旧账号 token 的在飞请求返回 401 时本地凭证已是新会话，此前仍走静默续期→重登弹窗。前端两条 401 链路（umi responseInterceptors `handleUnauthorizedAndRetry`、`fetchWithSessionGuard`）新增 stale 凭证判定（`sessionGuard.isStaleAuthHeader`/`hasNoCredentials`）：发送时所用 Authorization 与当前 localStorage token 不一致 → 直接用当前凭证重放一次，不弹重登；凭证已清（登出动线）→ 静默跳登录页不弹窗。当前 token 的真实过期 401 行为不变（续期→重登守卫）。单测 + `round108-auth-p2` E2E 覆盖。
- **tenant0 演示刊登任务重试不再被 worker 静默拒绝**：选择「修 worker」而非仅提示——tenant0 demo 口径已在 API 侧存在（`EnableDemoSeed && !production` 允许演示能力），worker 侧同口径补齐更一致，且演示任务本身是 local_draft_only 本地草稿链路、不触真实平台。`productpublish.Service` 新增 `AllowTenantZeroTasks`（router 按 `EnableDemoSeed && !IsProduction` 注入），worker 对 tenant0 任务在允许时以显式 tenant0 worker 上下文处理；不允许（生产等）时不再静默 continue 留任务永久 pending，而是 `failTaskTenantGate` 将任务落 `failed/task_tenant_missing` 并带中文可见错误信息，UI 可见可判。通用 `tasktenant.RequireTaskTenant` 拒绝 tenant0 的全局口径不变。
- **停库快照过期首请求 500 统一为 AUTH_STATE_UNAVAILABLE/503**：认证经新鲜快照放行（bridged）但业务查询失败时，此前返回普通 500。auth 侧新增 detailed 校验 API（`ValidateSessionAccessDetailed`/`EnsureAccountActiveDetailed`/`EnsureTenantActiveDetailed`）上报 bridged 状态，JWT 中间件置 `ctxkey.AuthStateBridged`，response 层对 bridged 请求的 5xx 统一改写为 503 + `AUTH_STATE_UNAVAILABLE`（业务码 50301）；非 bridged 500 与非 5xx 不受影响。前端 `authStateRetry` 同口径识别 401/503 两种形态走指数退避重试。回归测试：response 改写范围、bridged 上报、worker 租户闸门落败。

### 变更记录（2026-08-04）第 107 轮：R106 复检 P2 收口——前端 AUTH_STATE_UNAVAILABLE 专门处理 + UX v5 / v11 P2

- **前端 `AUTH_STATE_UNAVAILABLE` 专门处理**（`admin/src/utils/authStateRetry.ts`）：后端 fail-closed 数据库瞬断（#224）返回 401 + `AUTH_STATE_UNAVAILABLE`，与会话失效（`AUTH_SESSION_REVOKED`/token 过期）语义不同。前端不清凭证、不弹重登、不跳登录页，提示「服务暂时不可用，正在自动重试…」（15s 去重）并按 1s/2s/4s/8s 指数退避自动重放，恢复后原样返回响应无感续用；重试中转为普通 401 立即交回重登守卫，耗尽仍不可用按普通失败提示（`errorMessages` 补 `AUTH_STATE_UNAVAILABLE` 中文映射）。覆盖三条链路：umi responseInterceptors（新分支先于 401 重登守卫）、`fetchWithSessionGuard`（CSV 导出/备份下载）、启动期 `getInitialState` profile（`fetchProfileWithTokenDetailed` 标记 `authStateUnavailable`，硬刷新遇瞬断不清凭证）。`errorConfig.errorHandler` 对该错误码直接 throw，绝不兜底跳登录页。单测 `authStateRetry.test.ts` 11 例。
- **UX v5 P2 收口**：
  - P2-2 AI 未配置统一提示：新增共享 `AiConfigBanner`（与新手入门同口径探测，仅设置管理权限账号探测避免 403 噪音），落地新手入门卡、采集中心（/collect/hub）、AI 商品运营工作台。
  - P2-3 新手入门第 1 步权限：无 `settings.manage` 账号不再跳 `/settings/ai`（403 页），改为禁用态 +「需设置管理权限，请联系管理员完成 AI 配置」。
  - P2-4 `/healthz` 别名：后端补 `GET /healthz`（与 `/health` 同 handler），docs/api.md 同步。
  - P2-1 告警租户归属：#222/#223 已收口（`UpsertAlertForFailure` 按来源租户落桶），本轮复核无回退，不重复修改。
  - P2-5 demo seed 采集规则/浏览器 profile 样例：不在本轮修——demoseed 扩表涉及清退零残留不变量与 collect_rules/browser_profiles 样例设计，属独立 seed 迭代，非前端 P2 最小 diff 范围。
  - P2-6 v4 遗留清单（导入标题别名/部分成功分层/打印页桌面提示/承运商 URL tooltip 等）：R96 已收口（见第 96 轮变更记录），代码抽查在位（`migrationimport/fields.go` 别名、`PrintSheets.tsx` 小屏提示），维持不再重复修。
- **大回归 v11 P2 复核**：
  - 生产写闸门文案：`AutomaticPublishGuard`/`CredentialAbsenceGuard`/adapter 模式三道闸门的错误码中，`production_capability_forbidden` 已有中文文案，补齐 `real_credentials_forbidden`、`unsupported_adapter_mode` 中文映射（`constants/operationTasks.ts`），闸门触发时 UI 语义明确；seed 场景 tenant_id=0 重试被 worker 拒绝属 seed 局限（`order_sync_worker_tenant_missing` 仅日志，不产生真实外发），维持观察。
  - 告警「取消标记」：`TestUnmarkAlertRouteIsTenantScoped` 回归通过（跨租户 unmark 404、本租户正常），无回退。

### 变更记录（2026-08-05）第 114 轮：大回归 v14 + UX 视觉复核 v6（qa-engineer / user-experience-officer）

- **大回归 v14**（main `e08e55b9` + PR #239）：全量门禁除 admin/e2e 外全绿（check:dev / ui-copy strict / 前端 273 单测 / collector 18 / contracts 10 / build ×2 / go vet+gofmt+test / backend:integration / db / redis）；Docker 全栈 + `seed:demo:full` 实测 R57 主链路与 R109–R113 新功能动线（违禁词/话术模板/深度报表/AI 规避/面单+发货规则/多仓闭环/移动模式）三角色 × 三视口通过；双租户正规开租→隔离验证→清退、`seed:demo:full:clean` + verify 零残留；PR #239（375px 移动指标卡截断修复）验证通过。
- **P1 修复两条**：① 运营任务批量审批弹窗 `useForm` 未连接 Form 的 console.error（admin/e2e 全量此前 2 条失败根因）——Modal `destroyOnHidden` 改 `forceRender`，round63-optask-batch 12/12 通过；② 登录/注册页错误提示丢弃后端中文指引（未配 SMTP 时「获取验证码」只显「发送失败」，50301 指引文案不可见）——`getAuthErrorMessage` 改用共享 `httpErrorCopy`，补 envelope 中文透出单测。
- **UX 视觉复核 v6**：报告归档 `docs/ux-review/UX_REVIEW_V6_REPORT.md`；无 P0；P2 清单三条（默认仓无「设为默认」入口的产品口径说明、seeddemo verify 软删除残留口径、权限空态与 404 文案分离）。#237「默认仓唯一」为后端部分唯一索引兜底（Go 回归通过），UI 层无切换入口属轻量多仓产品口径，待产品确认（P2-1）。

### 变更记录（2026-08-05）第 115 轮：R115 P2 收口（fullstack-engineer）

- **默认仓切换**（UX v6 P2-1，产品已确认）：新增 `POST /api/v1/inventory/warehouses/:id/set-default`——事务内行锁原子切换旧默认→新默认（与 #237 部分唯一索引兼容：先清旧默认再置新默认）；因默认仓库存为推导口径，切换时先将旧默认仓推导库存物化为 `warehouse_stocks` 行、清空新默认仓持久行，各仓库存数量不变；已停用仓库拒绝设为默认（400），readonly 403（`requireInventoryWrite`），admin/operator 可操作；默认仓不可删除/停用口径随标志迁移。仓库管理页新增「设为默认仓」确认动线。回归：`TestSetDefaultWarehouse`（服务层：幂等/停用拒绝/跨租户/唯一默认/库存分布）、`TestSetDefaultWarehouseRoles`（HTTP 三角色）、`round115-warehouse-default` E2E（写拦截 + 切换后标签/只读隐藏）。
- **seed verify 软删除口径**（UX v6 P2-2）：`VerifyClean` 各表检查拆分 live（`deleted_at IS NULL`，计入残留）与 soft-deleted（单独上报 `softDeleted` 字段，不计残留）；`seeddemo verify` 仅对 live 残留失败，软删除历史残留以 stderr note 提示。回归 `TestVerifyCleanIgnoresSoftDeletedResidue`。
- **权限空态与 404 语义分离**（UX v6 P2-3）：`RouteAccessGuard` 无权限改为「暂无访问权限」+ 联系管理员开通权限/授权店铺引导（403 Result），`404.tsx` 改为「页面不存在」纯 404 语义，不再共用「页面不存在，或当前账号无权访问」混合文案；单测覆盖无权限/真实 404/有权限/未登录四态，相关 E2E 文案同步。
- **顺带（#240 已合入 main）**：移动首页快捷入口新增「审单工作台」（`/orders/review`，按 `ORDER_VIEW` 权限显隐；`menuAccess` 补该路由权限映射），round113-mobile-h5 E2E 断言补充。

### 变更记录（2026-08-05）第 117 轮：R116 审计 P2×8 批次收口（fullstack-engineer）

逐条处理 R116 审计 P2 / 观察清单 8 项（详见 `docs/permission-matrix.md` round117）：

- **部分修（1 依赖通告）**：`pnpm.overrides` / `pnpm-workspace.yaml` 补 `isomorphic-fetch>node-fetch → 2.6.7`（作用域覆盖，仅替换 dva → isomorphic-fetch 传递的 1.7.3；浏览器包不含 node-fetch，@umijs/test 的 node-fetch 3 不受影响；覆盖唯一一条运行时 high「secure headers 转发」通告）。`react-router 6.30.2` 覆盖已实测并放弃：6.30.x 引入 `@remix-run/router` 新增 1 high + 3 moderate 通告（其 7.x 才修复），净变差，维持 6.3.0 并登记已知风险。`build:admin`、`test:frontend`、`test:contracts` 全绿。
- **已修（2 只读试算口径）**：`shipping-rules/recommend`、`orders/shipping-recommendations` 入只读白名单，矩阵同步 `readonly: allow`。
- **已修（4 映射列上界）**：`import_mapping_presets` 保存时校验列数与列索引 `0 ≤ idx < MaxMappingColumns(200)`，回归 `TestMappingPresetColumnBounds`。
- **已修（8 通用契约测试）**：permmatrix 新增 `TestOrderByIDWriteStoreScope`（按 id 取单再写路径全覆盖 + 完整性检查），并修复其暴露的 4 处数据级缺口（`DELETE /orders/:id` 跨租户可删单、`match-skus` / `deduct-inventory` / `restore-inventory` 裸 id 执行、`bind-sku` 与 sku-candidates 订单行无租户/店铺条件）。
- **已知风险登记（不修 / 观察）**：
  - （1 余项）`react-router 6.3.0` 两条 moderate（外部/反斜杠跳转）：6.x 线内升级实测净变差（见上），彻底修复需 RR 7（umi 4 不兼容，大版本冒进，不做）；`vite` / `esbuild` / `send` / `hono` / `@hono/node-server` 均为构建或本地 dev server 依赖（经 `@utoo/pack` / `@umijs/test` 传递），不进产物、不对外网监听，接受；`elliptic` 无修复版本（patched `<0.0.0`），构建期传递依赖，接受。跟进条件：umi 官方升级内部固定版本或发布兼容 patch 时复评。
  - （3 打单页眉页脚）当前仍为 JSX 纯文本渲染，无 XSS 面；若未来改 HTML 渲染必须引入净化白名单（登记为变更前置条件）。
  - （5 check-batch 日志灌水）每请求一条操作日志，受全局限流（20 r/s burst 40）约束，日志表可清理，低风险接受；如放量再加端点级频控。
  - （6 迁移预览租户级口径）库存/仓库为租户级资源，`migration-preview` 与现有库存口径一致，不按店铺收敛，维持现状。
  - （7 `order_review_hits`）仍未在任何列表接口暴露，无缺口；后续若开放查询须补店铺 scope（与 F-1 同类，登记为开放前置条件）。

### 变更记录（2026-08-05）第 116 轮：R116 安全审计依赖补丁（security-auditor）

- 前端构建链 `pnpm.overrides` / `pnpm-workspace.yaml` 补 `axios 0.33.0`、`immer 9.0.21`（两者均为 `@umijs/plugins` 传递依赖，仓库未直接声明）：`pnpm audit --prod` critical 1→0、high 14→3、moderate 21→9、low 5→4；`axios` 覆盖 SSRF / 代理绕过 / 原型污染 / 头注入 / 凭据泄露等 9 条通告，`immer` 覆盖唯一一条 critical 原型污染。`pnpm build:admin`、`pnpm test:frontend`（41 套 283 用例）、`pnpm test:contracts` 全绿。
- 未处理并列入 P2（均为构建期或框架内部固定版本，补丁需跨大版本或与 umi 4 绑定）：`react-router 6.3.0`（umi 4 内部固定）、`node-fetch 1.7.3`（dva → isomorphic-fetch 传递）、`vite` / `esbuild`（devDependencies）、`send`、`elliptic`、`hono` / `@hono/node-server`。
- `govulncheck ./...`：0 命中（另有 1 条仅存在于 require 图、代码未调用）。

### 变更记录（2026-08-06）第 129 轮线2：UX 视觉复核 v8（user-experience-officer / ui-designer）

- **UX 视觉复核 v8**：报告归档 `docs/ux-review/UX_REVIEW_V8_REPORT.md`。基线 main `16f1ec52`（R124–R128 已合并批次）+ 本地叠加唯一未合并 PR #272；Docker 全栈 + `seed:demo:full`，375/767/768/769/1440 五档，admin+operator 实走 + tenant2 隔离抽查。v7 历史 P2 四项全部收口无回退（#264 深链重定向、日志行高 ellipsis、买家消息保存渲染、选品摘要折行）；R126 新动作参数表单（应用方式/分仓策略）、执行日志真实触发（DEMO-AT-1004 三动作成功）与失败重试、买家消息、财务对账三页、选品数据面、375 底部导航全动线与 768 断点互斥边界全部通过；#272 第二租户隔离与自动化日志中文化验证通过（可合并，需先解 `docs/PROGRESS.md` 冲突）。硬指标全零（console error/pageerror/根节点溢出/403·500 噪音）。**无 P0/P1**；P2×2 登记（T2 缺执行日志 seed 演示样本、操作日志操作类型列英文技术 key），本轮纯走查归档无代码变更。

### 变更记录（2026-08-05）第 123 轮：验收前最终全站大回归 v17（qa-engineer）

- **大回归 v17**：基于 main（#254–#260 已合并）本地叠加 #261 perf/round122（合并无冲突）。全量门禁通过：go build/vet/gofmt/test（97 包）+ 集成（integration/redis，TEST_DATABASE_URL/TEST_REDIS_URL）、contracts 15、frontend 46 文件 315 用例、collector 18、build:admin/collector、`check:ui-copy --strict`；全量 admin/e2e 280 passed / 3 skipped（webServer 冷构建超 120s 超时，改为预启 `max preview :8001` 复用后全绿，非用例失败）。Docker 全栈（当前分支镜像）+ `seed:demo:full` 实走 R57 主链路、R119 自动化（正样本 DEMO-AT-1004 标记已付款→生成采购单成功并跳转、负样本 DEMO-AT-1002 SKU 未匹配安全阻断、DEMO-AT-1003 跳过、?tab=automation 深链）、R119 买家消息闭环、R120 数据面板/趋势/对比 CSV、R121 回款登记→CSV 导入→差异工作台→实算毛利→对账报表、R122 收口面 + #261 索引后订单/日志/报表/选品页加载无回退且 SQL 数值对照 4 处全一致、审单/多仓/订单导入/375 底部导航叠加面；双租户（Redis 注入验证码正规开租、数据隔离、越权 not found）三角色三视口硬指标全零；clean+verify `zero DEMO- residual rows`。
- **零 P0/P1 产品缺陷**。上报的「DEMO-AT-1001 未被阻断」核对 seed 源码（`fulldemo_round119.go`）确认 1001 本就是自动确认付款成功样本、阻断负样本是 1002，属走查 skill 文档口径漂移而非缺陷——本轮修正 `demo-fullstack-walkthrough` SKILL（样本映射 + demo_operator/readonly 实际密码 DemoOperator123!/DemoReadonly123!）。P2×1 遗留：768 视口同时出现侧栏汉堡与移动端底部导航（无溢出，断点意图待确认）。
- **合并结论：#261 可合并**（叠加后全部门禁与实走通过，无其他待合并批次，直接合入 main 即可）。

### 变更记录（2026-08-05）第 121 轮：大回归 v16 + UX 视觉复核 v7（qa-engineer / user-experience-officer）

- **大回归 v16**：基于 main（R119 #254/#255 已合并）本地叠加 #256/#257（冲突仅 `docs/PROGRESS.md` 双 Round 120 条目）。全量门禁通过：go build/vet/fmt/test、contracts 14、frontend 45 文件 305 用例、collector 18、build:admin/collector、`check:ui-copy --strict`；CI=1 admin/e2e 268 passed / 3 skipped。Docker 全栈 + `seed:demo:full` 实测 R57 主链路、R119 自动化规则（含确认支付触发、日志/重试、时间线 tab/深链、审单/多仓/导入叠加面）、R119 买家消息闭环、R120 选品面板/走势/对比 CSV；双租户三角色三视口硬指标全零；clean+verify 零残留。唯一未覆盖：自动生成采购单正向样本（demo seed 无本地 SKU 匹配订单，负向安全拦截已验证）。
- **P1 修复两条（V6 P1-1 回归的双重根因）**：① 登录/注册两个 `<Form>` 同位三元切换被 React 复用实例，antd Form 始终绑定初始 `loginForm`，注册「获取验证码」请求体缺 `email` → 400 → 「发送失败」——两 Form 加独立 `key` 强制重挂载，补 `round120-register-send-code` E2E；② `admin/nginx.conf` `error_page 502 503 504` 把后端业务 503（50301 SMTP 指引、AUTH_STATE_UNAVAILABLE 契约）吞成「系统升级维护中」——仅拦 502/504，503 透传。
- **UX 视觉复核 v7**：报告归档 `docs/ux-review/UX_REVIEW_V7_REPORT.md`；无 P0；P2 清单四条（自动化日志小屏行高、买家消息保存瞬时渲染、选品摘要 768 竖排折行、`/purchase/orders` 深链 404）。结论：**#257 可合并；#256 功能验证通过但需先解 `docs/PROGRESS.md` 冲突再合并**。

### 变更记录（2026-08-05）第 121 轮：R121 线1 财务对账——回款/费用记账/实算毛利/对账报表（fullstack-engineer）

- **回款记录**：新模块 `backend/internal/modules/finance`（`finance_payment_records` / `finance_order_expenses` / `finance_shop_monthly_expenses`，金额 decimal(18,4)）。按订单登记平台回款（金额/币种/手续费/回款日期/渠道），手工录入 + CSV 批量导入（数据搬家向导新增 `kind=payment`：模板/自动映射/校验/幂等重放/重复跳过/错误行报告/全量导出）；回款与订单应收自动对账标记 未回款/少款/多款/已结清（容差 0.01）。
- **费用记账**：订单级费用（平台佣金/推广费/运费/其他，settings `finance.expense_types` 可增配）+ 店铺级月度费用（店铺×YYYY-MM×类型）；采购实付价 `PUT /procurement/orders/:id/items/:itemId/actual-price`（`actual_price` 区别于参考价 `expected_price`）。
- **实算毛利与对账**：实算毛利=回款（扣手续费）−采购实付（销售订单绑定采购项）−费用，与估算毛利并列（订单详情财务面板 + 工作台 + 报表）；差异较大（≥10% 且 ≥ 本位币 10）列入对账差异工作台；多币种沿用 `report_currency` 手工汇率本位币折算（无汇率缺省不伪造）。对账报表按店铺×月份汇总回款率/费用构成/实算 vs 估算差异；工作台与报表均支持 CSV 导出（UTF-8 BOM + csvsafe）。
- **前端**：`/orders/finance-payments`（回款+店铺月度费用，登记/删除/CSV 导入入口）、`/orders/finance-reconciliation`（汇总统计+状态筛选+导出）、`/orders/finance-report`（店铺×月份+导出）、订单详情「财务」面板；服务层集中 `admin/src/services/finance.ts`，全中文文案。
- **权限与租户**：全部端点登记权限矩阵（readonly 写 403、operator 店铺 scope、跨租户/越权 404）；顺带补齐 main 上缺登记的 buyer-messages×2 前缀 20 条与 `GET /imports/progress`（TestRouteRegistryComplete 此前在设 TEST_DATABASE_URL 时失败）。
- **demo seed**：fulldemo 新增财务样本（已结清/少款/多款回款、订单费用、店铺月度费用、采购实付价绑定销售订单）；clean/verify 覆盖三张 finance 表零残留（`TestFullDemoSeedFinanceSamples`）。
- **测试**：后端 finance 单测（CRUD/scope/对账状态/毛利/报表/CSV）、payment 导入/导出回归（含租户隔离与店铺 scope）、demoseed 回归、契约注册 109 端点 + finance payload/query 断言、前端 services 单测、`round121-finance` Playwright E2E 8 条（列表/登记回款写拦截 payload/删除/店铺费用写拦截/readonly/工作台/报表/五档视口无根溢出）。

### 变更记录（2026-08-05）第 122 轮线2：UX v7 P2×4 + #256 遗留 P2×2 收口批次（fullstack-engineer）

- **UX v7 P2×4 收口**：① 自动化执行日志「结果/原因」列固定 260px + `ellipsis` + Tooltip，并修表格列宽总和超 `scroll.x` 导致弹性列被压缩为 0 宽的问题（`scroll.x` 1100→1400）；② 买家消息草稿保存改为先 `load()` 刷新列表再关弹层，消除瞬时旧/新内容混排；③ 选品任务摘要「目标平台/国家」加 `nowrap`，768 不再逐字竖排；④ 新增 `/purchase` 与 `/purchase/orders` 路由别名重定向至 `/procurement/orders`。低成本建议四条全部落地（与 P2 清单重合 + 正向 demo seed）。
- **#256 遗留 P2×2 收口**：① 无商品行订单触发「自动生成采购单」定性为**跳过**而非可重试失败——`order.empty` 是确定性前置缺失，重试不可能改变结果，可重试失败会造成 3 次无效重试与误导性的「重试」操作；引擎新增 `order.AutomationSkip` 哨兵错误类型，router hook 将 `order.empty` blocker 映射为跳过并保留其余 blocker 的可重试失败语义，回归 `TestAutomationSkipOutcomeRecordsSkippedWithoutRetry`；② 规则引用已删除模板维持两态设计（已启用可停用不可再启用、编辑强制重选模板），补「已失效」红色标签让状态可视化（不需 hover），回归 E2E `round122-p2`。
- **demo seed 正向采购样本**：`seedRound119OrderAutomation` 新增 `DEMO-AT-1004`（审单通过 + 未付款 + 商品行已匹配本地 SKU/主货源/SKU 映射），后台「标记已付款」即真实触发自动生成采购单成功动线；clean/verify 零残留（复用 DEMO- 前缀清理路径），回归断言补入 `fulldemo_round119_test.go`。

### 变更记录（2026-08-05）第 123 轮线1：验收包整备——ACCEPTANCE_R123 + DEMO_SCRIPT + 文档核对（technical-writer / product-manager）

- **验收对照表**：新增 `docs/acceptance/ACCEPTANCE_R123.md`——按业务闭环（采集/选品→优化→草稿→货源→刊登→订单→审单→采购→入库→发货→签收→消息→财务对账→报表→横切治理）逐环节列能力点/实现轮次与 PR/验证证据/状态；外部凭证依赖单独列表并注明降级路径；汇总 R118 竞品复评矩阵结论（超越 3 / 达到 10 / 落后 3，落后项全部归因外部凭证）。基线 main `02b6b086`（#260 已合并）；#261 性能收口未合并，相关项标注待合并。
- **演示动线脚本**：新增 `docs/acceptance/DEMO_SCRIPT.md`——基于 `seed:demo:full` 的约 30 分钟 23 步完整演示（三账号三角色 + 375px 移动模式），Docker 全栈逐步实跑全部 23 步验证可照做（全程录屏）：步骤 11 自动化正/负样本真实触发通过（DEMO-AT-1004 自动生成采购单成功、DEMO-AT-1001 安全阻断留痕）；步骤 20/21 文案口径按实际表现修正（「暂无访问权限」/ readonly 写入口隐藏而非禁用）。
- **文档核对**：README/部署/升级/运维/env 逐条命令实跑核对——Docker 全栈启动、health、seed 全套（seed/clean/verify 零残留）、`check:dev`、升级指南预检 SQL、pg_dump 备份均验证通过；README 补充宿主机直跑 Go 种子需 `DB_HOST=127.0.0.1` 的说明（实跑发现缺省 `DB_HOST=postgres` 时报解析错误）。
- **门禁**：`pnpm check:ui-copy --strict` 通过；本轮仅文档变更，未触及代码门禁。

### 变更记录（2026-08-05）第 126 轮线1：R125 审计 P2×4 收口（fullstack-engineer）

- **依赖漏洞（P2-1）**：overrides 补 `hono 4.12.34`（moderate 修复）与 `send 0.19.1`（low 修复，构建链传递依赖）；`react-router` 6.x 线内升级实测净变差（6.30.x 引入 GHSA-337j / GHSA-jjmj 等新通告且 6.x 无修复版本），维持 6.3.0 并登记。仍需跨 major 才能修的项（交老板决策，不擅自升级）：`vite`（2 条 high 仅 6.4.3+ 修复，4→6 双跨大版本且被 umi 4 构建链绑定）、`vitest`（critical，0.34→3.x，#82 明确忽略其 major）、`esbuild`（0.18→0.25，0.x 语义等同 major）、`@hono/node-server`（1.x→2.0.5，`@utoo/pack` peer 限定 1.x）、`react-router`（彻底修复需 7.18+，umi 4 不兼容）、`elliptic`（无修复版本）。以上均为构建/dev server/测试链传递依赖，不进产物、不对外网监听。`pnpm audit` 全量 23（6 low/12 mod/4 high/1 crit）→ 21（5/11/4/1）；`govulncheck ./...` 前后均 0 命中。
- **财务 CSV 导入控制字符/格式校验（P2-2）**：`migrationimport` 解析层新增统一单元格校验（CSV 与 XLSX 共用）：拒绝表头/数据单元格中除 `\t` 外的一切控制字符（C0/C1/DEL，含 ANSI 转义、NUL、引号内嵌换行），并在解析阶段拒绝超过 `MaxMappingColumns`（200）列的超宽表头；错误信息带行列定位。恶意样本单测覆盖（ANSI/NUL/VT/BS/C1/表头/引号内换行/XLSX/超宽表头 + 合法样本回归）。
- **执行日志表冗余 shop_id（P2-3，评估后即修，低成本）**：`order_automation_logs` 新增 `shop_id` 快照列（写入时取订单店铺；迁移 `UPDATE ... FROM orders` 回填存量），店铺 scope 由订单子查询改为直接按日志 `shop_id` 过滤（无店铺订单日志维持仅 admin 可见的原口径）；API/前端类型/文档同步。
- **Hono 测试工具链（P2-4）**：`hono` 经 override 升至 4.12.34（patch 内修复）；`@hono/node-server` 需 1.x→2.0.5 跨大版本（`@utoo/pack` 传递依赖，peer 限定 ^1.19），登记为 umi 升级时复评。

### 变更记录（2026-08-06）第 131 轮线1：对账/毛利 CSV 全量导出收口（fullstack-engineer）

- **对账 CSV 全量导出**：`finance.ExportReconciliationCSV` 不再复用页面 500 行截断结果——服务内拆出 `reconciliation(c, r, status, maxRows)`，页面沿用 `maxReconRows=500` + `truncated` 标记，CSV 传 `maxRows=0` 携带全部匹配行（排序/口径/列与页面一致）。数据源 `scopedOrdersInRange` 去掉隐性 `Limit(5000)`，改为 `created_at DESC, id DESC` keyset 分页（每批 1000）加载，单条 SQL 不再物化无界结果；CSV 注入防御（csvsafe）与多币种/本位币折算口径不变。
- **全站 CSV 导出隐性行上限扫描**：同类问题命中报表毛利导出——`reports.ExportProfitCSV` 原复用页面 `profitMaxRows=500` 截断，同口径改为全量（`profitReport(..., maxRows=0)` + 订单维度 keyset 分批加载）。其余导出登记为有意上限：商品刊登导出 50（显式勾选批量上限，超限 400）、数据搬家导出 `MaxExportRows=50000`（防御性上限，已分批实现且有注释说明）、订单发货/采购导出为显式勾选 ID 集合、日报导出按天聚合，均无隐性报表级截断。
- **PERF 实测（万级 seed，10000 订单/6999 已付款）**：对账导出 `?days=120` 全量 6999 数据行 + 表头，耗时 0.56s；毛利订单维度导出 6999 行耗时 0.42s；行数=DB 计数（6999），随机抽样 3 单应收/已回款与 DB 逐值一致；页面接口维持 500 行 + `truncated=true` 且汇总仍覆盖全量（orderCount=6999）。`seedperf clean` + `verify` 零 PERF- 残留。
- **测试**：新增 `TestReconciliationCSVFullExport` / `TestProfitCSVExportFullRows`（1005 单：页面 500 截断、CSV 全量无重复、跨 keyset 批次边界）。门禁：go 全套（52 包）、contracts 15、frontend、build:admin、`round121-finance` E2E 8 条通过（首跑 2 条冷启动超时，复跑全绿）。

### 变更记录（2026-08-06）第 135 轮线1：R134 复评收口 + 订单自动打标签（fullstack-engineer）

- **R134 复评收口①（DEMO-AT-1004 SKU 提示口径）**：订单详情「库存影响」阻断提示按实际规格匹配状态区分文案——`unmatched/unbound` 提示「SKU 未绑定」、`ambiguous` 提示「匹配歧义」（`inventoryBindBlockHint`），不再同时展示两种互斥提示；与列表/前置描述口径一致。
- **R134 复评收口②（demo 采集规则样本）**：`seed:demo:full` 新增采集规则样本（开箱可见非空态），`clean`/`verify` 覆盖零残留，幂等可重复执行（单测覆盖）。
- **订单标签（round135 新功能）**：新表 `order_tags`（租户级名称/颜色，租户内重名 400）+ `order_tag_links`（`(order_id, tag_id)` 唯一，来源 manual/automation）。API：标签 CRUD（`/api/v1/order-tags`）、订单打标/去标（`/orders/:id/tags`）、批量打标/去标（`/orders/batch-tags`，≤200 单，返回 applied/removed 幂等计数）；订单列表/详情返回 `tags`，列表支持 `?tagId=` 过滤（进 keyset 指纹）。自动化规则新增 `add_tag` 动作（`tagIds` 校验当前租户存在性；沿用条件引擎/幂等/执行日志/时间线/dry-run/审单闸门/tenant+shop scope 口径）。Admin：设置→订单标签管理页、订单列表标签列 + 按标签筛选（URL query 唯一来源，`tagId` 进 ALLOWED_QUERY_KEYS）、批量打标签、详情手工打标/去标、自动化规则表单配置标签；readonly 只读（写入口隐藏/禁用）。demo seed 补 3 个标签样本 + 订单打标 + `add_tag` 自动规则与成功日志。
- **E2E**：新增 `round135-order-tags.spec.ts` 13 条——标签管理增删改、readonly 禁用、列表标签列/筛选 URL 写回与深链刷新、批量打标、详情手工打标/去标（含 readonly 无写入口）、自动化规则 add_tag 配置、五档视口无根节点横向溢出；全部非 GET 写请求显式拦截声明。

### 变更记录（2026-08-06）第 142 轮线1：买家自动消息生效范围 + 768 客服直达 + demo 失败样例标注（fullstack-engineer）

- **规则生效范围（R141 观察项收口）**：`buyer_message_rules` 新增 `effective_from`（空=回溯存量）；创建/停用→重新启用规则默认写入当前时间，只对生效后的订单节点事件生成草稿（事件时间口径：paid/shipped/delivered 用对应时间戳 COALESCE created_at；logistics_exception 用异常 shipment 更新时间；refunded 用订单 updated_at），不回溯存量订单。新增可选 `backfill` 开关（默认关）：开启时清空 `effective_from` 回溯全部存量；新增只读预估端点 `GET /customer/buyer-message-rules/backfill-estimate`（node 必填 + platforms/shopIds 过滤，口径与生成查询一致）。Admin 规则弹窗新增「回溯存量订单」开关（默认关，开启提交时先调预估并弹确认展示将生成数量），规则表新增「生效范围」列（仅新订单/回溯存量）。草稿仍仅站内生成、绝不自动外发。
- **P2-1（768px 客服直达）**：移动「我的」页新增「客服中心」入口；E2E 断言 768px 侧栏含客服菜单可直达 `/customer/hub`、无底部导航、无根节点横向溢出（与 round124 断点口径一致）。
- **P2-2（demo 发送失败样例标注）**：F8 种子的客服发送失败会话/消息/失败事件统一带「演示样例·非真实故障」标注（含未授权店铺失败事件），避免误判为真实故障；`clean`/`verify` 的 F8 会话清理由仅 tenant-0 孤儿扩展为任意租户 `F8 Demo%`（默认 DEMO- 前缀时），Docker 实测 seed→clean→verify 后 F8 会话/消息/失败事件零残留。
- **测试**：后端新增 `buyermsg_scope_test.go`（默认不回溯、事件时间口径、回溯预估与生成一致 + 幂等 + 租户隔离、停用重启重置生效时间、预估路由鉴权/跨租户/非法节点）；contracts +1 端点；前端服务单测补 backfill/预估；新增 `round142-msg-scope.spec.ts` E2E 4 条（开关默认关、预估确认、768 侧栏直达、375 我的入口）。
- **门禁**：go 全套 + vet + gofmt、contracts 15（端点 110）、frontend 339、build:admin、ui-copy strict、E2E（round142 4 条 + round119/round124 回归 29 条）全绿；Docker 实测默认规则 generate=0、开启回溯 generate=28（与预估一致、二次扫描 0）。

### 变更记录（2026-08-06）第 138 轮线1：对象存储备份上传（fullstack-engineer）

- **备份对象存储 Provider**：新增 `backend/internal/providers/backupstore`（S3 兼容：AWS S3 / MinIO / 阿里 OSS S3 兼容端点，官方 aws-sdk-go-v2），接口 `Upload/Download/List/Delete/Target`；`Target` 只输出脱敏目标（bucket/prefix/endpoint host），AK/SK 不落日志、不进 API 响应与错误信息。
- **上传状态与重试**：`backup_jobs` 新增 `upload_status/upload_target/upload_attempts/uploaded_at/upload_error`，`backup_artifacts` 新增 `object_key`；备份完成自动上传，`BACKUP_UPLOAD_MAX_ATTEMPTS` 有界重试，失败不影响备份任务本身（uploadStatus=failed），新端点 `POST /ops/backups/:id/upload`（backup.create）支持手动重试，已登记权限矩阵与 tenant-zero 测试。
- **保留策略与恢复取回**：上传成功后按 `BACKUP_OBJECT_RETENTION_COUNT` 保留最近 N 份（0=不清理；retention hold 备份不清理）；download/校验在本地文件缺失时自动从对象存储取回并校验 SHA-256。
- **降级路径**：`BACKUP_S3_*` 全部留空 = 未配置，备份仅保存本地路径、uploadStatus=skipped，不阻塞部署；半配置（缺 SK/缺桶等）启动即报 CONFIG_INVALID。
- **Ops 页**：备份页新增「上传状态/上传目标」列与「重试上传」按钮；readonly 创建/校验/下载/保留/重试上传全部禁用。
- **文档**：docs/env.md、production-launch-checklist（crontab 改为建议 + 对象存储上传为持久化路径）、upgrade-guide、api.md、provider.md、.env.example、.env.docker.example 同步。

### 变更记录（2026-08-06）第 137 轮线1：UX v9 P2-3 收口——报表 CSV「未折算」显式口径（fullstack-engineer）

- **P2-3 收口**：非 CNY 币种无手工汇率时，报表 CSV 折算/本位币列由留空统一为显式「未折算」占位（与页面渲染口径一致，仍不伪造折算、不补 0）：利润报表 CSV（折算收入列 + 本位币成本/费用/毛利列）、经营报表逐日 CSV（折算金额列，空日期行同口径）、对账报表/差异工作台 CSV（本位币列）。区分口径：店铺月度费用「无登记」仍留空（缺记录 ≠ 未折算）；无费用/成本行的 0 合计保持 `0.00`。UI 无需改动（已为「未折算」）。
- **测试**：改 `TestExportDailyStatsCSV`/`TestExportDailyStatsCSVConvertedColumns` 断言为「未折算」；`TestProfitCSVExport` 补无汇率行断言；新增 `TestFinanceCSVUnconvertedExplicit`（report/reconciliation CSV 双导出列级断言，含「0.00 不误标」「店铺月费留空」负口径）。`docs/api.md` 四处导出说明同步。
- **杂项巡检（R132–R136 合入面）**：`check:ui-copy --strict` 通过、无 console.log 残留、订单标签/自动化规则页空态含引导文案；无低成本待修项，无新增登记项。
- **门禁**：go 全套 + vet + gofmt、contracts 15、frontend 327、build:admin、ui-copy strict、E2E（orders-reports / round110-deep-reports / round121-finance 共 22 条）全绿。基于 #284（未合并）本地叠加。

### 变更记录（2026-08-06）第 144 轮线1：MCP 只读入口（fullstack-engineer）

- **MCP 只读 server 入口**：官方 Go SDK（Streamable HTTP，stateless）挂载 `POST /api/mcp`，暴露 4 个只读工具（订单查询/库存查询/经营摘要/异常待办），全部强制租户 scope，输出脱敏，无任何写操作。
- **租户级只读 API token**：`mcp_api_tokens`（SHA-256 哈希存储、明文仅创建返回一次、脱敏展示、吊销幂等、操作日志、每 token 限流），设置页新增「MCP 只读接入」；权限矩阵登记 4 条。
- 详见 `docs/progress/R144.md` 与 `docs/mcp.md`。

### 变更记录（2026-08-06）第 146 轮线1：MCP 安全加固——R145 P2 收口（fullstack-engineer）

- **token 可选过期**：`mcp_api_tokens.expires_at`（可空，默认不过期保持兼容），创建时可选 `expiresInDays`（1-730），到期鉴权即 401（fail closed 与吊销同口径）；管理页有效期选择 + 过期/即将过期（≤7 天）提示。
- **工具调用逐次审计**：新模块 `mcpaudit`（`mcp_tool_call_logs`），MCP `tools/call` 每次落一条（tenant/token 脱敏/工具名/时间/成败/耗时，不落查询参数与结果内容）；`GET /api/v1/mcp/audit-logs`（四角色）+ 管理页审计卡片。
- **限流多副本口径**：`pkg/ratelimit.RedisLimiter`（Lua 令牌桶，复用队列 Redis，无新依赖/变量），Redis 可用时 MCP 三层限流共享额度，不可用降级进程内（不 fail-open，已文档化）。
- 契约 114→115、权限矩阵、`docs/mcp.md`/`api.md`/`env.md`/`permission-matrix.md` 同步。详见 `docs/progress/R146.md`。基于 #295 分支叠加。

### 变更记录（2026-08-06）第 145 轮线1：实时经营大屏（fullstack-engineer）

- **经营大屏页面 `/dashboard/screen`**：深色大屏主题（可切浅色）、今日订单/销售额/毛利 KPI、待办五类计数、订单状态流转漏斗（近 7 天）、近 24h 逐小时趋势、异常/低库存告警滚动；15/30/60s 可配轮询 + 全屏投屏；1920 主视口，1440/1280/1024/768 优雅降级。
- **后端 `GET /api/v1/dashboard/screen` 单次聚合**：漏斗/趋势/待办均为分组 SQL 下推（无 N+1），销售额/毛利复用 `/reports/profit` #276 聚合口径；tenant/shop scope 与 dashboard 其余端点一致（空店铺授权 fail closed）；权限矩阵登记四 persona allow。
- 详见 `docs/progress/R145.md`。

### 变更记录（2026-08-06）第 145 轮线2：MCP 只读入口安全交叉审查（security-engineer）

- **审查范围**：R144 线1 MCP 只读入口（`POST /api/mcp` + 租户级只读 token）合入前交叉审查，双租户 Docker 全栈实测（token 生命周期/越权/写路径枚举/注入面/限流/输出脱敏/readonly 管理面/日志泄露）。
- **P1 修复**：① 鉴权强制 `scope=readonly`（此前只校验哈希与吊销位，非 readonly scope 的 token 行也能通行）；② 限流补每租户聚合桶（此前仅每 token 桶，同租户多 token 可线性放大额度）与每 IP 鉴权失败预算（此前无效 token 请求不受限，可无限触发 token 哈希查库）；③ 每租户活跃 token 上限 20（限流桶随 token 数无界增长）。
- **P2 收口**：429 envelope 由 `code=40001` 改为新增的 `CodeTooManyRequests=42901`；契约登记补 `POST /api/mcp` 与 token 响应字段/禁止字段（plaintext、tokenHash）。
- **未发现**：跨租户数据泄露、写路径可达、token 明文入库/入日志/入 API 响应、SQL/路径注入、tenant 0 平台数据经租户 token 泄露，均为零发现。
- **遗留（P2）**：token 无过期字段（仅显式吊销）；限流为进程内本地桶，多副本部署时额度按副本数放大（与 P7 Redis 限流收口项同源）；MCP 工具调用无逐次审计日志（仅 `lastUsedAt` 每分钟节流更新）。
- **测试**：新增 `mcpserver/hardening_test.go`（无效 token 限流 + 合法流量不被失败预算牵连 + 租户桶封顶多 token 放大）、`mcptoken/hardening_test.go`（非 readonly scope 拒绝、活跃 token 上限与吊销释放槽位）；contracts 端点 114。

### 变更记录（2026-08-06）第 147 轮线1：杂项收口——裸枚举中文化 + MCP token demo seed + R146 QA 复核（fullstack-engineer）

- **裸枚举中文化收口**：采购单详情支付状态/支付渠道、订单异常处理状态、客服消息 role/source/type 直出英文枚举改为既有语义映射口径（未知值兜底原值）；映射统一沉淀 `admin/src/constants/status.ts` 并补单测。
- **MCP token demo seed**：seed 新增 `DEMO-MCP 演示只读 token`（仅落哈希+脱敏元数据，明文即弃）+ `mcp_token_create` 审计样本（幂等）；Cleanup/VerifyClean 覆盖 `mcp_api_tokens` 零残留。
- **R146 QA 复核**：零数据租户空态、长租户名/大数值截断 Docker 实测复核。
- 详见 `docs/progress/R147.md`。

### 变更记录（2026-08-07）第 152 轮线2：买家消息多语言模板（fullstack-engineer）

- **模板多语言变体**：`customer_reply_templates` 新增 `default_language`（缺省 `zh-CN`，既有 `content` 即默认正文，历史零迁移）；新表 `customer_reply_template_variants`（tenant+template+language 唯一）；语言表可扩展（15 语种）；模板 API 增 `defaultLanguage`/`variants`（事务内全量替换）。
- **草稿语言口径**：生成时按 收货地国家→店铺语言→平台→回退默认语言 推断，草稿 DTO 增 `language`/`langSource`（`order_country`/`shop_language`/`platform`/`fallback`/`no_variant`/`manual`）；新端点 `POST /buyer-messages/drafts/:id/regenerate` 按所选语言重新生成（仅 pending，只改草稿绝不外发，readonly 403）。
- **UI**：模板页语言变体维护、工作台语言列+回退标注+切换语言重新生成，全中文管理界面。demo seed 补英/西/葡变体与 US→en / BR→pt 正样本、无国家 fallback 负样本（clean/verify 覆盖）。
- 详见 `docs/progress/R152.md`。

### 变更记录（2026-08-06）第 148 轮线2：安全审计季度复跑（security-engineer）

- **审计范围**：基于 main（#289–#300 已全部合入）复跑季度安全审计——MCP 入口（R145 修复零回退 + R146 过期/逐次审计/Redis 限流边界）、实时经营大屏新 API scope 与聚合注入面、备份定时器与恢复开关越权/配置注入、买家消息回溯开关越权，叠加常规越权/跨租户契约、readonly 403、tenant 0 闸门、CSV/XSS、密钥脱敏与日志 grep、seed 生产拒绝、govulncheck、pnpm audit，以及 R139 四项 S3/备份加固零回退核对与双租户 Docker 全栈实测。
- **P1 修复（本轮）**：权限矩阵 registry 漂移——`buyer-message-rules/backfill-estimate`（两个挂载点）未登记，且矩阵中 `POST /api/mcp` 因测试 harness 未启用 `MCPEnabled` 被判 stale，`TestRouteRegistryComplete` 失败（安全契约套件红）。修复：补登记两条只读条目 + harness 与生产 router 对齐启用 MCP。
- **零发现项**：跨租户数据泄露、MCP 写路径可达、token 明文入库/入日志/入响应、SQL/参数注入、平台租户接口越权、R139 修复项回退，均无。
- **P2 清单**：MCP 审计写失败仅告警不阻断；大屏 today 销售/毛利口径忽略 shopId/platform 筛选；大屏非法 shopId 静默降级不报 400；MCP token 上限 count→insert 竞态及其回归测试缺失；前端工具链依赖告警 13 条（2 high，均为构建/开发期）。
- 详见 `docs/SECURITY_AUDIT_R148.md`。


### 变更记录（2026-08-06）第 149 轮线1：R148 安全审计 P2 批次收口（fullstack-engineer）

- **P2-1 MCP 审计写失败收口**：`mcpserver.auditMiddleware` 由 best-effort 改为 fail-closed——审计行写入失败时扣留成功结果并拒绝该次调用（工具只读、可安全重试），同时 `slog.Error` 留可见告警；取舍：审计完整性优先于可用性。
- **P2-2 MCP token 上限竞态**：`mcptoken.Create` 的 count→insert 改为事务内检查 + 进程内 per-tenant 互斥 + PostgreSQL `pg_advisory_xact_lock`（跨副本），并新增 SQLite/PostgreSQL 并发回归测试（还原旧实现时测试失败，确认能捕捉竞态）。
- **P2 大屏口径**：`/dashboard/screen` today 销售/毛利改走 `reports.ProfitReportFiltered`，与订单数一致地应用 shopId/platform 过滤（过滤只收窄、不放宽租户/店铺授权）；非法 `shopId` 由静默降级改为 HTTP 400（`CodeBadRequest`），适用于全部 dashboard 端点。
- **P2 权限矩阵 CI 漂移预警**：`project-tests.yml` PostgreSQL 集成 job 新增 `pnpm test:permmatrix` 步骤（APP_ENV=test + TEST_DATABASE_URL），消除套件在 CI 静默 skip。
- **登记（本轮不改）**：operator 是否可管理 MCP token 收紧为 admin-only 属产品决策，待老板拍板。
- 前端工具链依赖告警（P2-3/审计编号）不在本轮范围。



### 变更记录（2026-08-07）第 153 轮线1：R152 两新功能交叉 QA + 安全审查（security-engineer / qa-engineer）

- **范围**：最新 `main` 上按 #308 → #309 顺序本地叠加，做开放 API 攻击面（purpose 越权、跨租户、限流绕过、泄密扫描、规范一致性、审计口径、注入）与多语言模板（regenerate 越权、语言回退、XSS/变量注入、seed 零残留）专项，含双租户三角色 Docker 实测。
- **P1 修复**：① 被禁用/清退租户的 MCP / 开放 API token 仍可读数据（token 鉴权补租户状态校验）；② 伪造 `X-Forwarded-For` 绕过每 IP 鉴权失败预算（新增 `TRUSTED_PROXIES`，默认不信任任何代理）；③ 开放 API 审计 fail-open 与 MCP（#303）fail-closed 口径不一致（改为审计落库后才返回结果）。
- **P2 清单**：both token 双入口双份额度；401/429 不写审计；分页非法参数静默归一化与日期 400 口径不一；operator 未授权店铺 regenerate 返回 404 与 readonly 403 口径不一；token 上限 count→insert 竞态（#303 已修未合）。
- 详见 `docs/progress/R153.md`。

### 变更记录（2026-08-07）第 154 轮线1：R153 安全审查 P2 批次收口（fullstack-engineer）

- **修复**：① 401/429 入口级拒绝写审计行（`mcp:auth`/`openapi:auth`，状态 `auth_failed`/`rate_limited`，未认证来源记租户 0，按来源每分钟至多一条防审计表放大，管理页筛选同步）；② 开放 API `page`/`pageSize` 非法值从静默归一化改为 400（与日期口径一致）。
- **登记不改**：both token 双入口额度合并需产品决策；operator 未授权店铺 regenerate 404 为店铺可见性口径（改 403 会泄露资源存在性）。
- **跳过（#303 已覆盖）**：token 上限 count→insert 竞态、MCP 审计 fail-closed，已随 #303 合入 main 闭合。
- **文档收口**：#311 三个行为变更补入 `docs/mcp.md`、`docs/open-api.md`、`docs/upgrade-guide.md`（R152/R153/R154 版本要点行）。
- 详见 `docs/progress/R154.md`。


### 变更记录（2026-08-07）第 155 轮线1：v25 P2×4 收口 + 合并期杂项（fullstack-engineer）

- **v25 P2×4 逐项处置**：① `GET /api/health` 404 登记不改（全仓无该路径声明，规范健康路径 `/health`、`/healthz`、`/api/v1/health` 文档已正确）；② #307 审计卡轻刷新时序补完整验证——单元级确定性时序 2 用例（新行入库后刷新、迟到错误响应不覆盖）在 main 失败、叠加 #307 通过，随 #307 以 PR #314 合入，Docker 全栈实测复核通过；③ 登录 body 字段 `account` 口径收口进 `docs/api.md`；④ #311 承接 P2 复核仍存在，随 #312/#303 闭合，登记不重复实现。
- **合并期预案**：#308 `mcptoken.Create` 增 `purpose` 参数与 #303/#311 测试旧签名调用的语义冲突——均未合并，登记提醒（合入时调用处补 `""`）。
- 详见 `docs/progress/R155.md`。
### 变更记录（2026-08-07）第 158 轮线2：验收包补 R153–R157 增量（fullstack-engineer）

- **验收包增量**：`docs/acceptance/ACCEPTANCE_R123.md` 新增 §一/16「R153–R157 增量能力」六行（安全加固三项行为变更 #311 ⏳、入口级审计+分页 400 #312 ⏳、v25 P2 收口+生产配置面 #313/#315/#316 ✅、MCP 错误码 -32603 等 #317 ⏳、大屏折算+自定义指标 #318 ⏳、R157 集成预演+交叉 QA）；§一/15 合入状态收口（#303–#309 已全部合入，七个 ⏳ 转 ✅）；§三/§五 同步（§五/11 建议合并顺序 #312→#317→#318，#311 可随 #312 关闭）。
- **演示脚本**：`docs/acceptance/DEMO_SCRIPT.md` 大屏折算/自定义指标演示点并入第 1b 步（约 1 分钟 → 约 1.5 分钟，压缩第 2 步），保持 ~30 分钟；常见坑与构建前置同步更新。
- **Docker 全栈三角色实跑**（main 合入 v26 集成预演分支构建，录屏证据外置不入库）：新演示点与抽查动线全部符合，两处失实即修（「已折算：X」与未折算行互斥、卡片配置无「恢复默认」按钮），并登记进 demo SKILL 常见坑。
- 详见 `docs/progress/R158-line2.md`。

### 变更记录（2026-08-07）第 159 轮线2：生产升级演练季度复跑（devops-engineer）

- **升级演练**：R149 时点基线（`7f5645c1`，双业务租户 2 万订单/存量 MCP token/存量话术模板等）→ 最新 main（`6a64eb39` 与演练中合入 #312 后的 `32a9aaea` 两个时点）全流程通过：R152 `mcp_api_tokens.purpose`/`customer_reply_template_variants`/drafts 语言列落地，业务指纹逐项 0 差异（仅 `order_automation_logs.shop_id` 回填为预期变化）；从零部署 165s、升级部署 464s/246s（<15 分钟目标）。
- **升级后实测**：purpose 三口径（mcp/openapi/both 互斥与放行）、开放 API 限流/租户隔离/审计/分页 400、多语言模板变体、`OPENAPI_ENABLED=false` 404、`TRUSTED_PROXIES` 配置与留空两口径 XFF 实测均与文档一致；`--pre-upgrade-check` 备份+预检、备份→恢复→幂等重跑闭环通过。
- **文档核对**：无 P0/P1；P2 三条登记（#317 -w 报错口径已于演练后合入、待下轮实测、#318 大屏折算/汇率 seed 演练时未合入、演练后已合入待下轮补验、恢复窗口回退增量数据口径）。证据外置不入库；演练记录已补 `docs/upgrade-guide.md` §五。
- 详见 `docs/progress/R159-line2.md`。

### 变更记录（2026-08-07）第 156 轮线2：经营大屏汇率折算与自定义指标（fullstack-engineer）

- **多币种折算显式口径**：大屏今日销售额/毛利沿既有租户 `report_currency` 手工汇率折算本位币（复用 /reports/profit 口径），新增 `today.unconvertedRevenue`（无汇率币种原币金额显式列出、不计入合计）与 `today.convertedCurrencies`；前端销售额/毛利卡补折算口径角标 tooltip 与「未折算（不计入合计）」原币金额展示。
- **租户级自定义大屏卡片**：新端点 `GET/PUT /api/v1/dashboard/screen/config`（卡片池 8 张：订单/销售额/毛利/告警 KPI + 待办/漏斗/趋势/告警列表；顺序+开关，默认保持现状；GET 四角色可读、PUT 需 `settings.manage`，readonly/operator 403；配置沿 tenant scope 存 settings `dashboard_screen.cards`，记操作日志）；`/dashboard/screen` 响应带 `cards`，禁用卡片跳过对应聚合；前端按配置分段渲染 + 配置弹窗（开关/排序）。
- **配套**：demo seed 补今日多币种大屏样本 `DEMO-FX-USD-0001`（可折算）/`DEMO-FX-EUR-0001`（无汇率未折算），clean/verify 覆盖；权限矩阵登记两条新端点；契约 117→119；新增后端/前端单测与 `round156-dashboard-screen-config.spec.ts` E2E。
- 详见 `docs/progress/R156-line2.md`。

### 变更记录（2026-08-07）第 156 轮线1：R155 登记 P2 + 合并期杂项收口（fullstack-engineer）

- **MCP 审计 fail-closed 错误码**：拒绝调用由普通 error（wire 上 JSON-RPC `code:0`，非规范值）改为 `-32603 internal error`（`jsonrpc.CodeInternalError`），补 code 断言回归测试；开放 API 侧同场景本就是 HTTP 500 + envelope `50000`，口径一致无改动。
- **`--pre-upgrade-check` 备份目录**：维持默认 `/var/backups`（root 部署面向），非 root 下 mkdir 失败与目录不可写（新增 `-w` 检查）都启动即清晰报错并提示 `BACKUP_DIR=<可写目录>` 覆盖，不再误报「pg_dump 备份失败」。
- **demo clean 覆盖面核实**：`seed:demo:full:clean` 只清 `DEMO-` 前缀 token，`e2e-refresh-verify` 等测试遗留不覆盖（实测确认）；不扩大删除面，SKILL 常见坑登记「测试收尾自行吊销」。
- **合并期预案落地**：#308 purpose 签名冲突按 R155 §3 预案修复（叠加分支内调用处补 `""`）；permmatrix harness 补 `OpenAPIEnabled`（cherry-pick #313）。
- 详见 `docs/progress/R156.md`。

### 变更记录（2026-08-07）第 159 轮线1：安全审计季度复跑（security-auditor）

- **P1 修复**：仅 `view` 授权的店铺可被写入（同租户内店铺授权粒度越权写）——R149–R158 新增/改造的订单与买家消息草稿写路径只校验店铺可见性，未校验可操作性；按 R125 口径收口（view-only 403 / 不可见 404 / admin 与 operate·manage 不变 / 被拒零落库），`adminperm` 新增 `OperableStoreIDs`、`EnsureStoreOperable`、`ApplyStoreOperateScope`，覆盖草稿五路由与订单创建·更新迁店·删除·行项·发货单·打标·物流刷新·自动化重试·库存扣减回滚·SKU 匹配与 bind-sku·打单标记，附先失败后通过的回归测试。
- **文档/契约修正**：开放 API `severity` 枚举由 `error/warning` 更正为实际 `low/medium/high/critical`（jsonschema + `docs/open-api.md`）；`docs/mcp.md` 明确入口级 401/429 留痕为 best effort、fail-closed 仅作用于 `tools/call`。
- **复验无回退**：开放 API purpose 双向隔离/跨租户 404/脱敏/XFF 无绕过/逐次审计 fail-closed/`OPENAPI_ENABLED=false` 运行时；MCP R145·R148 修复项与 `-32603`、租户禁用即失效；多语言模板注入面与授权；大屏折算与卡片配置 scope/readonly/参数校验；权限矩阵 644 条 route 无漂移；govulncheck 0 可达；seed 生产拒绝。
- **P2 清单**：非法入参静默降级（`severity`/`lowStockOnly`）、前端构建工具链依赖 13 条（2 high）、view-only 403 业务码 40301 与 40303 不统一、入口级拒绝审计 best effort、矩阵 harness 缺 view-only persona。
- 报告归档 `docs/SECURITY_AUDIT_R159.md`，详见 `docs/progress/R159.md`。

### 变更记录（2026-08-07）第 160 轮线1：R159 审计 P2 收口（fullstack-engineer）

- **P2-1 view-only persona**：权限矩阵 harness 新增可选 persona `viewOnlyOperator`（店铺仅 `view` 授权），新增契约测试覆盖全部订单/订单行写路由与买家消息草稿写路径（403 + 40303 + 零落库，带路由完整性检查防 #322 同类漂移）。
- **P2-2 非法入参显式 400**：共享只读查询层新增枚举校验（orders `status`/`paymentStatus`、exceptions `exceptionType`/`severity`），Open API `lowStockOnly` 改严格布尔；非法值 400/40001（沿 #303/#312 口径），Open API 与 MCP 同层收敛；OpenAPI 规范补齐枚举并修正 `severity` 漂移。
- **P2-3 业务码统一**：店铺级「可见但仅 view 授权」403 统一 40303（order/customerchat/finance 各写 handler），40301 保留全局/租户级 forbidden、40302 保留权限位拒绝；前端与契约无引用无需改动。
- **P2-4 依赖告警**：13 条构建链告警逐项评估，0 条可不跨 major 净收敛（react-router minor 覆盖实测反增告警已回退），登记 `docs/DEPENDENCY_ADVISORIES_R160.md`。
- 详见 `docs/progress/R160.md`。

### 变更记录（2026-08-07）第 161 轮线1：竞品复评 v8（product-researcher + qa-engineer）

- **最新 main（b2d20535）Docker 全栈实测复评 R151 16 项矩阵：零回退，矩阵升位为超越 4 / 达到 12 / 落后 0**——v7 建议 1（开放 API #308）与建议 2（消息多语言）已合入并实测坐实（开放 API 五端点端到端 + MCP 复测 + 多语言草稿/回退/400 白名单 + 安全边界抽验 + UI 走查录屏），第 12 项客服管理升位、新增第 20 增项「开放 API/可编程集成」评超越；v7 建议 3（#318）未合入不计入。
- **竞品 2026 复查**（店小秘经营看板、马帮 TikTok 双赛道认证、AutoDS Claude MCP 写操作等）：无新结构性缺口；MCP 写白名单决策紧迫性上升。
- **下一阶段建议（按杠杆）**：①合入积压收口（#322 P1 安全优先→#323→#318→#321）②MCP 写白名单设计稿（决策项）③维护期节奏（复评每 12 轮或触发式）。
- **合并期更新（本 PR 合并 main 时点，260bf123）**：#318/#321/#322/#323 已全部合入 main（积压收口建议①闭合），R162 MCP 写白名单设计稿（#326，建议②）亦已归档；#318 相关项（第 11 项报表/财务）升位待下轮 Docker 实测复核。
- 报告：`docs/COMPETITIVE_BENCHMARK_R161.md`；详见 `docs/progress/R161.md`。

### 变更记录（2026-08-07）第 162 轮线2：全站视觉/UX 复核 v10（user-experience-officer / ui-designer）

- **走查**：距 v9 25 轮的全站复核。Docker 全栈 + `seed:demo:full` 三角色实测录屏；headless 硬指标矩阵 3 角色 × 5 精确视口（1920/1440/1024/768/375）× 29 路由全零（溢出/NaN/Invalid Date/console/pageerror/403·500 噪音）；v9「精确 375/1920 未达」覆盖限制收口；v9 遗留无回退；R137–R161 新面（备份 Ops、MCP token、大屏、开放 API 入口、多语言模板、币种设置、标签/自动化、财务对账）全走查。
- **P1 修复**：`/settings/report-currency` dirty 时路由跳转静默丢弃修改 → 新增共享 `useUnsavedChangesGuard`（history.block + beforeunload）并接入。
- **P2 顺手修**：备份确认弹窗英文按钮、备份/恢复创建时间 raw ISO、大屏趋势 tooltip raw ISO、操作日志新动作英文 key（补 `dashboard` 资源 + 18 动作中文映射）。
- **P2 遗留**：MCP 页文档入口不可点击（待产品定文档挂载位置）、v9 P2-3 财务 CSV 未折算占位口径待产品确认。
- 报告归档 `docs/ux-review/UX_REVIEW_V10_REPORT.md`，详见 `docs/progress/R162-line2.md`。
