# R187 线2：性能与加载体验审计季度复跑（performance-engineer + qa-engineer）

日期：2026-08-08。距 R122 首次大数据量压测已 60+ 轮，期间合入 MCP 写链（R179–R183）、审计列表与治理 UI（R183–R185）、大屏折算/自定义指标、多语言模板、开放 API 等大批新面。本轮为季度复跑：同量级 seed 压测 + 基线对比 + MCP/开放 API/前端首包三个新面首次纳入基准。

## 【结论】

**全部 PASS，无 P0/P1，无需代码修复**：双租户 2 万订单量级下核心列表页 p50 全部 <40ms、报表/对账全面优于 R130 修后基线（快 21–60%）、异常聚合与 R122 修后基线持平；MCP 读工具分页 p50 ≤16ms、写链 dry-run→execute 端到端 p50 18ms、11 万行审计列表深分页/筛选 p50 ≤20ms；开放 API 限流路径开销可忽略（429 拒绝 p50 1ms）；前端首包 gzip 320.6kB，较 R79 瘦身基线 +5.8%（108 轮功能增量下可接受），路由级懒加载无回退（177 个 async chunk，图表库不在首包）。EXPLAIN 复核未发现导致页面不可用级的慢查询与 N+1；P2 登记 4 项（见文末）。

## 【口径与基线】

- 基线代码：main（`cce97efa`），本地构建 `APP_ENV=development`，Docker PostgreSQL 16 + Redis（docker compose）。
- 数据量（双租户各一套，业务数据总量约 2 倍于 R122 单租户口径）：每租户 10,000 订单（20,000 订单行）/ 5,000 采购单 / 30,000 库存流水 / 20,000 自动化日志 / 2,000 商品（4,000 SKU）/ 12,000 回款 / 20,000 订单费用 / 5,000 选品候选；`mcp_tool_call_logs` 合计 110,000 行（含双租户、10 种工具、双 mode、多状态分布）。
- seed 方式：`backend/cmd/seedperf`（R122 引入）灌租户 2；租户 1 由租户 2 PERF 数据经确定性 UUID 重映射 SQL 克隆（绕开 seedperf 清理按前缀全局删除的限制，见 P2-4），业务键唯一性（order_no/dedup_key/business_event_key/external_shop_id）按租户闭合处理。
- 测量：本机 HTTP 直压（15 次/端点取 p50/p95，MCP/开放 API 10 次且限频 4 rps 以避开 5 rps 令牌限流），JSON/脚本证据外置附件不入库；Actions CI 不作依据。
- 历史基线：R122（异常页修后 p50 ≈278ms@万级、日志深分页修后 ≈7ms）、R130（报表/对账修后：利润订单 130ms / 店铺 156ms / 商品 253ms、对账 378ms、对账报表 448ms）、R79（首包 gzip 386kB→303kB、大屏 p50 0.44s@万级）。

## 【1. 核心列表页 / 大屏 / 报表延迟基准（租户 1，p50/p95 ms）】

| 端点 | p50 | p95 | 历史基线 | 判定 |
|---|---|---|---|---|
| 订单列表 p1 | 9 | 17 | R122 页面级 1.2–2.2s | PASS |
| 订单列表 p500（深分页） | 14 | 18 | — | PASS |
| 订单列表 paymentStatus 筛选 | 8 | 10 | — | PASS |
| 订单列表 keyword 搜索 | 24 | 25 | — | PASS |
| 采购列表 p1 / p250 | 3 / 4 | 4 / 5 | — | PASS |
| 库存中心 p1 | 33 | 38 | — | PASS |
| 库存流水 p1 / p1500（3 万行深分页） | 13 / 31 | 19 / 37 | R122 修后 ≈7ms@1.5万 | PASS（量级翻倍，线性合理） |
| 自动化日志 p1 | 6 | 7 | — | PASS |
| 异常工作台 p1 | 237 | 276 | R122 修后 ≈278ms | PASS（持平，无回退） |
| 经营大屏 overview | 684 | 766 | R79 修后 ≈440ms@万级 | PASS（数据量约 2 倍，无劣化性回退，登记 P2-1） |
| 经营大屏 screen | 323 | 345 | — | PASS |
| 利润报表 order / shop / product | 99 / 100 / 158 | 113 / 109 / 170 | R130 修后 130 / 156 / 253 | PASS（全面更优） |
| 财务对账列表 | 251 | 293 | R130 修后 378 | PASS |
| 财务对账报表 | 299 | 335 | R130 修后 448 | PASS |

租户 2 抽查（订单 p1/p500、大屏 overview/screen）与租户 1 同量级一致，双租户隔离下无跨租户放大效应。

### 慢查询 / N+1 / 缺索引复核（pg_stat_statements + EXPLAIN ANALYZE）

- 全程压测 GORM 慢 SQL 日志（阈值 500ms）**零命中**。
- Top SQL 为异常聚合的 order_item_sku_matches 关联扫描（mean 54ms/次、40k 行），被 overview/screen/异常页共享调用，为大屏/异常页延迟主因；属 O(全量匹配行) 线性扫描，当前量级可接受（登记 P2-1）。
- 报表/财务链路均为批量 `IN (...)` 聚合查询（R130 改造后形态），**未发现 N+1 回退**。
- `mcp_tool_call_logs` 深分页 EXPLAIN：`Index Scan Backward (created_at)` + `Filter: tenant_id`，11 万行 OFFSET 5 万实测 27ms，但过滤丢弃 49,655 行——缺 `(tenant_id, created_at)` 复合索引，随跨租户总量线性劣化（登记 P2-2）。
- 自动化日志深分页（OFFSET>10,000）返回 400 `pagination_offset_too_deep` 并提示 cursor 分页——R122 后加入的设计护栏，行为符合预期，非缺陷。

## 【2. MCP 面基准】

读工具（`POST /api/mcp` tools/call，pageSize=100，10 次 p50/p95 ms）：

| 工具/参数 | p50 | p95 |
|---|---|---|
| orders_query p1 | 11 | 14 |
| orders_query p100（深分页，第 10,000 行处） | 15 | 21 |
| orders_query paymentStatus 筛选 | 11 | 14 |
| inventory_query p1 | 14 | 18 |
| report_summary | 217 | 251 |
| exceptions_pending | 199 | 229 |

写链端到端（orders_add_tag / orders_remove_tag 交替 8 轮，均 execute 成功且审计落库）：dry_run p50 ≈7ms、execute p50 ≈9ms、**dry-run→execute 端到端 p50 18ms / p95 21ms**——三层闸门 + 确认 token 校验开销可忽略。

审计列表（11 万行，`GET /api/v1/mcp/audit-logs`）：p1 4ms、p2500（OFFSET 5 万）15ms、tool 筛选 7ms、status=error 筛选 20ms——**PASS**（深分页缺复合索引登记 P2-2）。

行为发现（非缺陷，登记 P2-3）：租户写配额（200 次/天）按 `mcp_tool_call_logs` 当日 success+execute 行数计数，压测合成审计行会吃掉真实配额；本轮通过把合成行 created_at 移出当日窗口解决。

## 【3. 开放 API 限流路径 + 前端首包】

开放 API（`/api/open/v1/*`，token 认证 + 5 rps 限流路径，10 次 p50/p95 ms）：

| 端点 | p50 | p95 |
|---|---|---|
| orders p1 / p100 | 7 / 9 | 10 / 10 |
| inventory p1 | 11 | 20 |
| exceptions | 335 | 363 |
| reports/summary | 319 | 426 |

同查询内部端点对照（session 认证订单列表 pageSize=100）p50 19ms → **限流+token 认证路径开销可忽略（甚至因响应更精简而更快）**；连发 30 次触发 429 共 20 次，**拒绝路径 p50 1ms / p95 2ms**，快速失败无放大。exceptions/reports 的 ~330ms 与内部异常聚合同根因（P2-1）。

前端（`pnpm build:admin` 生产构建，对比 R79 瘦身基线）：

- 首包 gzip：`umi.js` **320.6kB**（+ css 6.1kB + preload_helper 5.2kB，入口合计 332kB），较 R79 修后 303kB **+5.8%**（其间 108 轮合入 MCP 治理、多语言模板、大屏自定义指标等大批页面）——增幅温和，登记 P2-4 持续观察阈值 350kB。
- 懒加载无回退：177 个 async chunk，路由级代码分割完好；echarts/bizcharts/antv 均不在首包（最大图表 chunk 461kB gzip 为异步按需加载）。

## 【4. P0/P1 修复】

无 P0/P1。所有端点 p95 <800ms、无页面不可用级慢查询，无需代码修复（本 PR 仅文档归档）。

## 【P2 清单（登记不阻塞）】

1. **异常聚合线性扫描**：`order_item_sku_matches` 全量关联扫描（54ms/次@4 万行）被大屏 overview/screen、异常页、MCP exceptions_pending、开放 API exceptions 五处共享，随订单量线性增长；10 万订单量级预计 overview 将超 1.5s。建议：聚合结果短 TTL 缓存或物化待处理异常计数。
2. **`mcp_tool_call_logs` 缺 `(tenant_id, created_at)` 复合索引**：现单列索引深分页需回表过滤半数行（11 万行 27ms，随跨租户总量线性劣化）；status/tool 筛选同理可考虑带 tenant_id 前缀的复合索引。量级到 50 万行前建议补迁移。
3. **MCP 写配额计数口径**：租户日配额按审计表当日行数计数，测试/演示合成审计行会占用真实配额；建议 seed/演示数据约定 created_at 移出当日窗口，或配额计数排除标记行。
4. **首包持续增长**：320.6kB（R79 303kB → +5.8%），建议 CI 增加 bundle size 预算护栏（如 350kB 阈值告警），防止无感知回退。
（附带登记：seedperf 清理按 `PERF-` 前缀全局删除、不区分租户，双租户 seed 时后一次会清掉前一次的租户数据；本轮以 SQL 克隆绕开，建议 seedperf 清理条件加 tenant_id 闭合。）

## 【门禁】

无 Go/前端代码变更；`pnpm build:admin` 全绿（本轮构建即首包证据）；文档变更不触发后端门禁。

## 【证据（外置附件，不入库）】

`bench.py` / `mcp_bench.py` / `openapi_bench.py` / `clone_tenant.py` 压测与克隆脚本、`core-results.json` / `mcp-results.json` / `openapi-results.json` 原始延迟数据、pg_stat_statements Top SQL 与 EXPLAIN 输出——见会话附件。收尾已清理：PERF seed 双租户数据、11 万合成审计行、临时 MCP token（吊销）、`mcp.write_enabled` 设置还原。
