# R183 竞品对标复评报告（第十一次：店小秘 / 马帮 / AutoDS）

- 轮次：R183 线2（product-researcher + qa-engineer）
- 日期：2026-08-08
- 前十次评估：R90、R108、R118、R125、R134、R143、R151、R161、R170、R178（超4/达12/落0，报告见 `docs/COMPETITIVE_BENCHMARK_R178.md`）
- 背景：距 R178 v10 已 5 轮（R179–R182），期间 MCP 写白名单 W1–W3 全量产出（W1 订单打标、W2 异常标记+采购 mark-placed/物流回填、W3 采购 mark-paid），另有 R182 限额并发竞态闭合（advisory lock）。本轮回答：①W1–W3 的合入面到底是什么？②「AI 对话式写操作」矩阵位次能否从落后转持平/超越？③下一阶段是维护期节奏还是新杠杆？

## 〇、合入状态权威核实（#360–#366，2026-08-08 时点，来源 GitHub PR 状态）

| PR | 内容 | 状态 |
|----|------|------|
| #360 | R179 线1：MCP 写 W1（orders_add_tag/remove_tag + 三层闸门 + dry-run→确认 token→执行 + fail-closed 审计/限额） | **已合入 main**（merge commit `2fe041cd`） |
| #361 | R180 线1 | OPEN 未合入 |
| #362 | R180 线2 | OPEN 未合入 |
| #363 | R181 线1：W3 mark-paid | OPEN 未合入 |
| #364 | R181 线2：QA 验收包 | **已合入 main**（merge commit `a97c7b67`） |
| #365 | R182 线1：限额并发竞态闭合（advisory lock）+ W2 UI 收口 | OPEN 未合入 |
| #366 | R182 线2 | OPEN 未合入 |

**实测口径（诚实标注）**：本轮实测基线为**叠加分支** `origin/main + #365 分支`（#365 分支树内已传递包含 W2/W3 与 R181 W3 合流内容，本地分支 `r183-line2-test`，HEAD `5dc27fdc` 之上合入 `eff0dff0`）。**纯 main 时点只含 W1（#360）**；矩阵表中对「main 口径」与「叠加口径」分列标注，叠加口径能力以 #361/#363/#365 等 OPEN PR 合入为前提，不得视为 main 当前生产能力。

## 一、结论（一句话）

**16 项矩阵：超越 5 / 达到 11 / 落后 0——「AI 对话式写操作」按叠加口径从「落后」转为「超越」（写五动作+mark-paid 全链路实测通过、三层闸门/重放/参数漂移/跨租户/金额前提全部按设计拒绝、审计逐行落库），按纯 main 口径为「持平」（W1 已合入即具备对话式写 + 全套治理管道，写面窄于 AutoDS 但治理深度反超）**；竞品侧 2026-08 复查无新增结构性动作（AutoDS MCP 叙事沿 8-05 教程常态化，店小秘/马帮仍零 MCP）；维护期抽验零回退。

## 二、逐项矩阵复评（16 项 + 增项 4，相对 R178 变化）

图例：▲超越 ●达到 ◇凭证待解锁。实测环境：`docker-compose.full.yml` 全栈 5 容器 healthy + `pnpm seed:demo:full`。

| # | 能力项 | 本次实测（2026-08-08，叠加分支；差异处标注 main 口径） | 评级（相对 R178 变化） |
|---|--------|------------------------------------------------------|------------------------|
| 1 | 商品采集 | `/healthz` database/redis ok、collect worker 2 并发在位 | ●（持平） |
| 2 | 数据搬家/迁移 | 无合入面变化 | ●（持平） |
| 3 | 智能选品 | seed 5 任务五状态在位；外部大盘仍凭证依赖 | ●（持平） |
| 4 | AI 商品运营（文+图） | 无回退信号 | ▲（持平超越） |
| 5 | 刊登管理 | 无变化；美客多全托管属「明确不做」清单 | ◇（持平，凭证待解锁） |
| 6 | 订单管理 | 订单 API 200；MCP 写打标后订单标签正确变更（W1 实测） | ▲（持平超越） |
| 7 | 审单规则 | 无回退信号 | ●（持平） |
| 8 | 打单发货/物流 | API 200；电子面单凭证依赖不变 | ◇（持平，凭证待解锁） |
| 9 | 采购管理 | 采购单状态机经 MCP 写实测走通 placing→placed→paid→shipped（W2/W3，叠加口径） | ◇（持平，凭证待解锁；1688 直连仍卡资质） |
| 10 | 库存/多仓 | inventory_query MCP 只读 200 | ●（持平） |
| 11 | 报表/财务 | report_summary 只读 200；mark-paid 金额审计列落库（叠加口径） | ▲（持平超越） |
| 12 | 客服管理 | 「绝不自动外发」红线不变：消息发送不在 MCP 写面 | ▲（持平超越） |
| 13 | 违禁词合规 | 无变化 | ●（持平） |
| 14 | 移动端 | 无回退信号 | ●（持平） |
| 15 | 多租户与权限 | MCP 写跨租户目标统一「订单不存在」（404 语义实测）；operator 创建 write:ops token 403+40301；operator token 列表看不到写 token | ▲（持平超越） |
| 16 | 数据安全/自托管 | 三层闸门逐层独立拒绝实测（详见三-2）；fail-closed 审计（execute 与业务变更同事务）；确认 token 单次消费+参数哈希绑定 | ▲（持平超越） |
| 17* | 安全工程体系（增项） | W1–W3 治理管道全链路实测通过；R182 advisory lock 并发闭合（叠加口径） | ▲（持平超越） |
| 18* | 平台覆盖广度（增项） | 无变化 | 长期项（不计入主矩阵） |
| 19* | **AI 助手/对话式入口（增项，本轮重点）** | **叠加口径：写六工具全注册并实测（orders_add_tag / orders_remove_tag / exceptions_mark / procurement_mark_placed / procurement_fill_logistics / procurement_mark_paid），dry-run 预览+人话摘要+一次性确认 token→execute→重放 alreadyExecuted 幂等**；main 口径：W1 打标已合入，同一治理管道在位 | **▲（由「落后」转超越，叠加口径；main 口径持平）** |
| 20* | 开放 API/可编程集成（增项） | 无回退信号 | ▲（持平超越） |

**汇总：超越 5 / 达到 11 / 落后 0（第 19 增项计入主叙事后主矩阵第 6/9/11/12/15/16 项相关位次不变；「AI 对话式写操作」为本轮唯一位次变动项）。** 3 项「凭证待解锁」不变，单列不算落后。

## 三、重点：「AI 对话式写操作」我方 vs AutoDS Claude MCP（实测坐实）

### 1. 对比表

| 维度 | AutoDS Claude MCP（公开资料 2026-08-08） | 我方 MCP 写白名单（本轮实测） |
|------|------------------------------------------|------------------------------|
| 写面广度 | 建 listing、改价、批量变更、一键上架（跨 eBay/Shopify/WooCommerce） | 六动作窄白名单（打标 ×2、异常标记、采购三态）；**广度落后，刻意为之** |
| 执行前确认 | 公开资料未见 dry-run/预览机制 | 强制 dry_run→影响预览+人话摘要→一次性确认 token→execute（实测） |
| 防重放/幂等 | 未见公开说明 | 确认 token 单次消费；重放返回 `alreadyExecuted=true` 且零重复变更（实测） |
| 参数绑定 | 未见公开说明 | 确认 token 与租户+token+工具+参数哈希四元绑定，参数漂移拒绝（实测：换 tagName 被拒） |
| 开关治理 | 未见分层开关公开说明 | 三层闸门实测：env 关→写工具不注册（tools/list 仅 4 只读）；租户关→调用被拒；无 write:ops scope→看不到写工具 |
| 审计 | 未见公开说明 | fail-closed：execute 与业务变更同事务；`mcp_tool_call_logs` 逐行落库含 mode/params_summary/confirm_hash/amount（实测 DB 抽查） |
| 限额 | 未见公开说明 | 每 token 30 次/时、每租户 200 次/天；mark-paid 另有单笔/日累计金额上限，未配置=默认不可用（实测拒绝文案） |
| 金额型动作 | 改价直接执行 | mark-paid 强制金额/币种与采购单精确一致（64.49 vs 64.50 实测拒绝），不动真实资金 |
| 权限治理 UI | 未见公开说明 | 管理后台写 token 创建/吊销（operator 创建实测 403）、审计列表含 mode/金额列 |

**诚实口径结论**：写面广度我方仍窄于 AutoDS（六动作 vs 全店铺运营面）；但「有治理的对话式写」（确认、幂等、审计、限额、分层开关、金额前提）为我方独有公开可验证能力，AutoDS 公开资料无对应叙事。综合判：**叠加口径超越、main 口径持平**。

### 2. 实测记录摘要（叠加分支，Docker 全栈）

- 三层闸门：`MCP_WRITE_ENABLED=false` 重启后 tools/list 仅 4 只读工具；开启后租户开关仍关时 dry_run 返回「写操作未开启（租户级开关关闭）」；readonly token 的 tools/list 无任何写工具。
- W1 全链路：dry_run 返回 preview/summary/confirmationToken → execute `applied=1` → 同 token 重放 `alreadyExecuted=true`；参数漂移（remove_tag 换标签名）拒绝并提示重新 dry_run。
- W2：exceptions_mark handle 走通（mark=handled）；mark-placed placing→placed 回填外部单号（DB 复核）；fill_logistics paid→shipped + 物流记录落库（DB 复核 tracking_no/carrier）。
- W3 mark-paid 金额前提三连：上限未配置→拒绝；金额差一分（64.49）→拒绝；金额精确一致→dry_run 预览含供应商/明细行/两项上限/当日已用→execute 后 DB `status=paid, pay_status=paid`，审计行 `amount=64.50`。
- 治理：operator 创建 write:ops token `403+40301`；operator token 列表不含写 token；跨租户订单号统一「订单不存在」。
- 审计抽查：`mcp_tool_call_logs` dry_run/execute/error 逐行在位，params_summary 只含业务键。
- 无回退抽验：MCP 四只读工具正常（orders_query 客户名脱敏 `D**`）、/healthz 全绿、seed clean+verify 零 DEMO- 残留。

## 四、竞品 2026-08 动态复查（相对 R178 增量）

- AutoDS：无 R178 后新增公开动作；MCP 叙事沿 7-16 发布→8-05 多店铺教程→feature 页常驻（autods.com/features/autods-claude-mcp、autods.com/blog/autods-mcp-connector）。
- 店小秘：最新仍为 8-03 美客多全托管批量刊登（dianxiaomi.com/blog/article/608）；无 MCP/对话式/开发者 API 公开动作。
- 马帮：仍为 TikTok Shop 美区双赛道认证（mabangerp.com/article/main_contentDetails_1082.htm）与拉美奖项（960）；无 MCP 动作。
- 结论：**竞品侧本轮零增量**（R178 与本轮同日），无新结构性缺口。

## 五、结构性缺口与下一阶段路线建议（如无异议按此执行）

**总判断：R179–R182 把「对话式写」从决策推进到全量产出，唯一遗留是合入面偏差——W2/W3/R182 竞态闭合仍在 5 个 OPEN PR 上（#361/#362/#363/#365/#366）。**

| 序 | 建议 | 一句话理由 | 量级 |
|----|------|-----------|------|
| 1 | **PR 积压按序合入（#361→#362→#363→#365→#366）** | 叠加口径的「超越」只有合入 main 才算数；积压越久回归成本越高 | 0.5 轮 |
| 2 | **「有治理的对话式运营」对外叙事包**（安全白皮书式文档+demo 录屏，直接复用本报告三-1 对比表与实测证据） | 零代码高杠杆，竞品公开资料无治理叙事，窗口独占 | 0.5 轮 |
| 3 | **复评节奏回归常态**：每 12 轮或结构性触发（下次默认 R195 前后；竞品出现 MCP 治理/企业版审计叙事则提前） | 写白名单专项复评已由本轮完成，连续位次稳定 | 常态 |
| 4 | 写面扩展保持克制：W4+ 新动作仅在真实用户诉求出现后评审，不为追 AutoDS 广度而扩写面 | 「窄写面+深治理」是差异化本身 | 触发式 |

**凭证依赖项（单列，到位即插队）**：抖店/1688/电子面单/TikTok Shop/Shopee 凭证；外部选品大盘数据源。

## 附：证据索引

- 实测环境：叠加分支 `r183-line2-test`（origin/main `a97c7b67` + #365 分支 `eff0dff0`），`docker-compose.full.yml` 全栈 5 容器 healthy，`pnpm seed:demo:full`；实测后测试 token 全部吊销、临时 settings 复位、`seed:demo:full:clean` + verify 零 DEMO- 残留。
- 实测证据（curl 请求/响应、DB 抽查记录）作会话附件，不入库。
- PR 状态来源：GitHub PR #360–#366 权威状态（git_view_pr，2026-08-08）。
- 竞品资料（访问日期 2026-08-08）：autods.com/features/autods-claude-mcp、autods.com/blog/autods-mcp-connector（8-05）、autods.com/blog/autods-introduces-claude-mcp-connector（7-16）；dianxiaomi.com/blog/article/608（8-03）；mabangerp.com/article/main_contentDetails_1082.htm、main_contentDetails_960.htm。
- 前次报告：`docs/COMPETITIVE_BENCHMARK_R178.md`；MCP 写文档：`docs/mcp.md`。
- Actions CI 不作为本报告依据（按任务要求）。
