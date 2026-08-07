# MCP 写操作白名单设计稿（R162 线1，决策项 · 纯方案，不实现）

- 轮次：R162 线1（product-researcher + security-auditor）
- 日期：2026-08-07
- 性质：**设计稿，仅供老板决策；本文档不改变任何代码行为，不实现任何写操作。**
- 背景：R161 竞品复评（`docs/COMPETITIVE_BENCHMARK_R161.md`）确认 AutoDS Claude MCP connector（2026-07）已含写操作（直接刊登/改价/批量更新），「对话式写操作」范式被市场验证，身位窗口开始收窄；R151/R161 均建议先出「写白名单设计稿」再决策。
- 前置阅读：`docs/mcp.md`（只读 MCP 入口）、`docs/open-api.md`（token purpose 体系）、`docs/permission-matrix.md`（授权契约测试与 ReadonlyWriteGuard）、`docs/SECURITY_AUDIT_R159.md`（分支 `fix/round159-security-audit`，view-only 越权修复口径）。

## 0. 一句话结论与立场

**建议采纳「P0 最小集」路线**：新增独立的写白名单 token scope（与现有 `readonly` 单轴兼容演进），首批只放行 5 个「站内低风险、可回滚/幂等、已有人工闸门」的写工具，全部强制 dry-run 先行 + 逐操作确认 + 逐次审计 + 限额，fail-closed 口径与现有 MCP 治理完全一致；**任何对外发送/真实平台 API 调用类操作进入永久「不做清单」**。如无异议，实现工作在老板批准后另开轮次执行（本轮不动代码）。

不建议采纳的方向（明确立场）：
- ❌ 不建议照搬 AutoDS「直接刊登到店铺」的写范围——那是真实平台外发动作，与我方「绝不自动外发、审单闸门、人工确认」既有原则冲突，且我方平台凭证尚未解锁（R161 凭证依赖项），无真实收益只有风险敞口。
- ❌ 不建议在现有 `readonly` scope 上加白名单旗标（如 token 加 `allowWrite` 布尔）——会让存量 token 的语义变得可疑，违反「存量攻击面不因新能力变宽」的 R152 取舍原则。
- ❌ 不建议做「自然语言直接执行」免确认模式——即使限额内也不做；LLM 参数幻觉是主要威胁模型（见 §3.1）。

## 1. 竞品调研：MCP/AI 写操作范围与安全模型

### 1.1 AutoDS（直接竞品，2026-07 上线 Claude MCP connector）

来源：autods.com/features/autods-claude-mcp、autods.com/blog/autods-introduces-claude-mcp-connector（2026-07-16）、autods.com/blog/autods-mcp-connector（2026-08-05）、help.autods.com 15505185（访问日期 2026-08-07）。

- **写操作范围**：导入/上传商品到 AutoDS 草稿、**直接发布（publish）商品到 eBay/Shopify/WooCommerce 店铺**、修改定价与 listing 内容、批量跨店更新（"Update pricing, listings, and more across your entire store with one message"）、修复商品问题（fix product issues）。
- **安全模型（公开资料可见部分）**：官方 connector 走 Claude directory 分发 + 账号级 OAuth 绑定；宣传口径强调便利性，**未见公开的逐操作确认、dry-run、限额或独立写 scope 机制**。写能力直接继承账号权限。
- **对我方的启示**：AutoDS 验证了需求存在（对话式运营是真实场景），但其安全模型是「账号全权限直通」，正是社区批评的反面教材；我方差异化恰在「自托管 + 安全边界」，白名单式写入是与其区隔的卖点而非模仿对象。

### 1.2 Shopify 生态（安全模型参考基准）

来源：shopify.dev/docs/agents/profiles/auth-and-rate-limiting、my.shop-mcp.app/docs/connect/shopify（ShopMCP）、github.com/Groupthink-dev/shopify-blade-mcp、weaverse.io AI Agent Write Access 指南（2026-06）（访问日期 2026-08-07）。

| 实践方 | 写安全机制 | 可借鉴点 |
| --- | --- | --- |
| Shopify 官方 Agent 分层 | 按 agent 身份强弱分三档（Token/Signed/Anonymous），身份越强能力越大、限流越宽；`complete_checkout` 等敏感工具仅最高档且需显式授予 | 「能力与身份绑定 + 敏感操作单独授予」= 我方按 scope 分轴 + 逐操作白名单 |
| ShopMCP | **三层闸门**：① Shopify 侧 `write_*` scope 最小授予；② 产品侧「Write tools」总开关默认关（over-scoped token 也写不了）；③ 每个写工具逐次向用户确认 | 三层结构直接映射到我方：token scope → 租户级写开关 → 逐操作确认 |
| shopify-blade-mcp（社区） | 逐操作 env 门控、破坏性操作需 `confirm=true` 参数、错误路径密钥擦除；点名批评社区 MCP「所有写全开、无操作级门控」 | `confirm` 显式参数 + 操作级门控是社区已收敛的最佳实践 |
| Weaverse 指南 | 「上限写进工具 schema 而不是 prompt」（"Prompts are suggestions. Schemas are physics."）；写 scope 必须是逐集成、可审计的显式决策 | 限额/枚举必须在 JSON Schema 与服务端双重强制，不依赖提示词 |

### 1.3 店小秘 / 马帮

R161 复查确认两者**均无 MCP/对话式入口**（AI 布局在内容生产侧）；马帮开放平台（open.mabangerp.com）有传统写 API（订单/库存/货件），采用开发者 key + 签名模型，无 agent 专属安全层。对本设计无直接安全模型输入，但说明「写 API + agent 入口」组合目前只有 AutoDS 一家，窗口仍在。

### 1.4 调研小结

行业收敛出的写操作安全共识（我方设计全部吸收）：**最小 scope 独立轴、默认关闭、逐操作确认、限额进 schema、逐次审计、破坏性操作显式 confirm、敏感动作永不下放**。AutoDS 的「账号直通」模式是获客卖点但安全负资产；我方应做「安全的写」而不是「快的写」。

## 2. 候选写操作清单（按业务闭环梳理，风险分级）

梳理口径：沿订单业务闭环「订单进入 → 审单 → 采购 → 回填 → 发货 → 售后/异常」，只考虑**已有 Admin API、已有权限/scope/审计口径**的站内动作（不发明新业务能力）。风险分级：

- **L1（低）**：可逆或纯标记类，不改资金/库存/对外状态；错误代价 = 撤销一次标记。
- **L2（中）**：推进状态机或影响下游生成，但站内可回滚（作废/取消）；错误代价 = 一次人工纠正。
- **L3（高）**：涉及资金口径、库存扣减、不可逆状态或批量放大；错误代价 = 对账/库存差异。
- **L4（禁止）**：对外发送、真实平台 API、租户/权限/配置管理——永久不进白名单。

| # | 候选操作 | 对应站内 API | 风险 | P0 建议 | 理由 |
| --- | --- | --- | --- | --- | --- |
| W1 | 订单打标签 / 去标签 | `POST /orders/:id/tags`、`DELETE /orders/:id/tags/:tagId` | L1 | ✅ 进 P0 | 幂等、完全可逆、零资金/库存影响；agent 分类整理订单是高频真实场景 |
| W2 | 异常待办标记已处理 / 忽略 | 异常工作台 handled/ignored 标记 | L1 | ✅ 进 P0 | 纯标记、可撤销；与只读工具 `exceptions_pending` 闭环（查→处理） |
| W3 | 采购单回填 1688 订单号（mark-placed） | `POST /procurement/orders/:id/mark-placed` | L2 | ✅ 进 P0 | 状态机单向推进但可作废重来；人工在 1688 下单后回填单号是最机械的操作 |
| W4 | 采购单回填运单号/承运商 | `POST /procurement/orders/:id/logistics` | L2 | ✅ 进 P0 | 同上；paid → shipped，录入错误可通过作废纠正 |
| W5 | 采购单标记付款（mark-paid） | `POST /procurement/orders/:id/mark-paid` | L2–L3 | ✅ 进 P0（带单笔金额上限） | 涉及资金口径但仅站内标记（不动真实资金）；限额 + 确认后风险可控；站内自动化规则已有 `confirm_payment` 带金额上限的先例 |
| W6 | 生成采购单（generate，草稿态） | `POST /procurement/orders/generate` | L2 | ⏸ P1 扩展 | 产出是 draft、有幂等 key、有 blockers 机制，本质安全；但涉及聚合逻辑，建议 P0 验证确认/审计链路后再放 |
| W7 | 销售订单物流回填（新增 shipment） | `POST /orders/:id/shipments` | L3 | ⏸ P1 扩展 | 推动订单发货流转、影响买家侧口径；且与批量发货（W12）边界易混，P0 不放 |
| W8 | 标记打单（print/mark） | `POST /orders/print/mark` | L1 | ⏸ P1 扩展 | 本身极低风险（不动状态机），但单独价值低，随 W7 一批评估 |
| W9 | 买家消息草稿标记已发送（mark-sent） | 买家消息 `mark-sent` 回执 | L2 | ⏸ P1 扩展 | 仅回执标记（发送动作永远在平台后台人工完成）；但语义上贴近外发边界，P0 先不碰，避免「agent 参与消息线」的观感风险 |
| W10 | 审单放行 / 拒绝（approve/reject） | 审单规则命中订单的人工裁决 | L3 | ❌ 不进白名单（本期） | 审单闸门是全系统安全边界的锚点（pending_review/held 阻断采购与发货）；让 agent 裁决等于让 LLM 拆闸门，需单独一轮专项论证才可考虑 |
| W11 | 扣减 / 回滚库存 | `POST /orders/:id/deduct-inventory` 等 | L3 | ❌ 不进白名单（本期） | 库存差异是 ERP 最贵的错误；且多仓/锁定逻辑复杂，参数幻觉代价高 |
| W12 | 批量操作（批量发货 / 批量回填 / 批量打标） | `/orders/shipments/batch` 等 | L3 | ❌ 不进白名单（本期） | 批量 = 错误放大器；逐单确认模式下批量无意义，等 P0 跑出审计数据再议 |
| W13 | AI 任务触发（标题优化 / 描述生成 / 图片任务） | `/ai/*` | L2 | ⏸ P2 择机 | 消耗 AI 配额、产出是站内草稿；风险低但与「写业务数据」性质不同（是触发任务），建议独立为 `tasks` 类 scope 再评估 |
| W14 | 商品草稿编辑 / 应用 AI 结果 | `PUT /products/:id`、`apply-ai-*` | L2 | ⏸ P2 择机 | AutoDS 主打场景（audit & fix listings）；但改动面大（标题/描述/SKU），需要字段级白名单设计，P2 单独出细稿 |
| W15 | 刊登 / 发布到店铺 | productpublish 系列 | **L4** | ❌ 永久不做（经 MCP） | 对外动作 + 凭证未解锁；即使凭证解锁，发布也必须走 Admin UI 人工确认动线 |
| W16 | 发送买家消息 / 客服回复外发 | — | **L4** | ❌ 永久不做 | 「绝不自动外发」是产品红线（docs/api.md §买家自动消息），MCP 不改变该红线 |
| W17 | token / 租户 / 用户 / 权限 / 设置管理 | `/settings/*`、mcptoken 等 | **L4** | ❌ 永久不做 | 权限自举风险（agent 给自己扩权）；治理面永不暴露给 agent |
| W18 | 删除类操作（订单/商品/文件删除） | `DELETE /*` | **L4** | ❌ 永久不做（本期） | 破坏性最高、收益最低；软删除也不放 |

**P0 最小集 = W1–W5（5 个写工具）**：全部满足「站内、有人工闸门先例、可逆或可作废、参数简单（id + 单号/金额，幻觉面小）」。

## 3. 安全模型设计

### 3.1 威胁模型（设计出发点）

1. **LLM 参数幻觉**：agent 把单号/金额/id 填错——最高频威胁，靠 dry-run + 逐操作确认 + schema 限额兜底。
2. **token 泄露**：写 token 被盗——靠独立 scope（只读 token 永不升格）、短有效期建议、限额、审计告警兜底。
3. **提示注入**：订单备注/商品文案中的注入文本诱导 agent 执行写操作——靠「确认必须由人完成」（out-of-band，见 §3.4）兜底，注入最多产生一个待确认提案。
4. **权限自举 / 横向移动**：靠 L4 永久禁区（治理面不暴露）+ 租户/店铺 scope 复用既有口径兜底。
5. **审计缺口**：靠 fail-closed（无审计即拒绝）延续 R146 口径兜底。

### 3.2 token scope：从单轴到双轴的兼容演进

现状：`scope` 单值 `readonly`（唯一权限轴），`purpose` 是入口选择器（mcp/openapi/both）。演进方案：

- `scope` 从单值扩展为**枚举集合语义**，新增 `write:<组>` 形式的细粒度值，首批仅一组：
  - `readonly`（不变，含义不变）；
  - `write:ops`（P0 组：W1–W5 五个工具的执行权，**不隐含读**——写 token 若需查询须同时带 `readonly`）。
- **兼容性口径（硬约束）**：
  - 存量 token 全部保持 `scope=readonly`，行为零变化；鉴权处对未知 scope 值继续按无效处理（现有 fail-closed 逻辑天然兼容）。
  - 写 scope **只能在创建 token 时授予**（与 purpose 同口径），不提供「升级」入口；要写权限就发新 token。
  - 写 token **强制有效期**（建议默认 30 天、上限 90 天，不允许永不过期——与只读 token 的「默认不过期」有意区分）。
  - 写 token 创建仅限 admin 角色（operator/readonly 不可创建），创建/吊销落操作日志（沿用现有）。
- **租户级写总开关**（借鉴 ShopMCP 第二层闸门）：`设置 → MCP 接入` 新增租户级「允许 MCP 写操作」开关，**默认关**；关闭时即使 token 带 `write:ops` 也一律 403 拒绝并审计留痕。环境级另有 `MCP_WRITE_ENABLED`（默认 `false`），部署方不打开则全平台无写面。

三层闸门叠加：`MCP_WRITE_ENABLED`（环境，默认关）→ 租户写开关（默认关）→ token `write:ops` scope（显式发放）。三者同时满足才有写面，任何一层关闭即刻全局止血，无需吊销 token。

### 3.3 dry-run 先行（强制，非可选）

每个写工具的 schema 含必填参数 `mode: "dry_run" | "execute"`，且服务端强制状态机：

1. `dry_run`：服务端做**全量校验 + 影响预览**（目标对象存在性/租户/店铺 scope/状态机合法性/限额余量），返回结构化预览（将改什么、从什么状态到什么状态、金额多少）+ 一次性 `confirmationToken`（绑定「租户+token+工具+参数哈希」，TTL 建议 5 分钟，单次使用）。dry_run 不落业务库、但落审计（status=`dry_run`）。
2. `execute`：必须携带有效 `confirmationToken`，服务端重放校验（参数哈希必须与 dry_run 时一致——防止确认后偷换参数），校验通过才执行。无 token / 过期 / 参数不一致 → 拒绝（fail-closed）。

这使「先看后做」成为协议层物理约束而不是 prompt 约定（吸收 Weaverse "schemas are physics" 原则），同时 confirmationToken 的参数哈希绑定天然提供幂等基础。

### 3.4 逐操作确认（人在环上）

- P0 采用 **dry-run → 人读预览 → execute** 两步模式：确认动作由使用 MCP 客户端的人完成（人在对话里看到预览后指示 agent 执行）。这与 MCP 生态的 tool-approval 交互（Claude 逐工具批准）叠加，形成双确认。
- 诚实口径：该模式下「人确认」发生在客户端侧，服务端无法证明是人而非 agent 自我确认。因此 P0 同时配置**限额**（§3.6）作为硬兜底，并在 Admin「MCP 接入」页提供**写操作流水视图**（谁的 token、什么时间、执行了什么、dry_run/execute 配对），让事后监督成本趋近于零。
- P1 可选增强（不进 P0）：高风险操作（W5 mark-paid 超阈值）改为「MCP 侧只能提案，Admin UI 收件箱人工点确认后才执行」的 out-of-band 模式——彻底免疫提示注入，但交互成本高，等 P0 审计数据说明必要性再上。

### 3.5 审计与幂等

- **审计**：复用 `mcp_tool_call_logs` 表与 fail-closed 口径（审计写失败 → 工具调用拒绝，`-32603`）。写工具审计在现有字段外**必须记录参数摘要与结果摘要**（与只读工具「不记参数」口径有意区分——写操作的参数就是证据；仍不记敏感内容，金额/单号/id 类字段白名单式记录）。dry_run 与 execute 各一条，关联同一 `confirmationToken` 哈希。
- **幂等**：三层——① `confirmationToken` 单次使用（同一确认不能执行两次）；② 底层 API 既有幂等语义沿用（打标已幂等、mark-placed/mark-paid 状态机重复执行会因状态不符拒绝、generate 有 idempotencyKey）；③ 工具层对「重复 execute 已成功的确认」返回明确的 `already_executed` 而非报错，方便 agent 重试不产生副作用。

### 3.6 限额（rate limit 之外的业务限额）

在现有三层限流（token/租户/IP）之外，写操作新增**业务限额**（服务端硬编码上限 + 租户可下调）：

| 限额 | 建议默认 | 说明 |
| --- | --- | --- |
| 每 token 每小时 execute 次数 | 30 | 对话式操作的自然节奏上限；超出 429 |
| 每租户每日 execute 次数 | 200 | 兜底放大攻击 |
| W5 mark-paid 单笔金额上限 | 租户配置，必填，无默认 | 未配置金额上限则 W5 在该租户不可用（fail-closed）；与自动化规则 `confirm_payment` 金额上限同款口径 |
| 单次操作对象数 | 1 | P0 无批量：每次 execute 只作用于单个订单/采购单/异常 |

### 3.7 租户 / 店铺 scope 与 fail-closed 口径

- 租户隔离：写 token 与租户绑定（同现状）；所有写工具强制 `ApplyTenantScope`，跨租户目标一律 **404**（不泄露存在性，与开放 API 口径一致）。
- 店铺 scope：写工具按目标对象的 `shop_id` 走 **可操作性** 校验（R159 P1-1 修复确立的 `EnsureStoreOperable` 口径，而非仅可见性）；MCP 写 token 视为其创建 admin 的委托，但**建议 P0 直接按「admin 创建、租户全店铺可操作」简化**，P1 若需要更细粒度再引入 token 绑定店铺清单。
- fail-closed 全清单：未知 scope→401；写总开关（环境/租户任一）关→403；审计不可写→拒绝执行；确认 token 缺失/过期/参数漂移→拒绝；W5 无金额上限配置→该工具不可用；权限矩阵登记——**每个新写路由必须进 `matrix.json` 且 readonly persona=forbid**，并新增「readonly token 调写工具必须全拒」的 MCP 侧契约测试（对应 permmatrix 思路在 MCP 层的镜像）。

### 3.8 与审单闸门 / 人工确认既有原则的关系

- **审单闸门优先级高于 MCP**：`pending_review` / `held` 订单的一切 MCP 写操作（打标除外——标签不影响状态机）一律拒绝，与站内自动化规则「pending_review/held 不自动化」同一口径；MCP 不提供放行/拒绝工具（W10 不进白名单）。
- **「绝不自动外发」原则完整延续且加强**：MCP 写白名单全部是站内状态标记/回填，**没有任何工具触达买家、平台或真实资金**；W9（mark-sent 回执）P0 也不放，确保「消息线零 MCP 触点」。该原则写入白名单治理规则：任何新工具进白名单前须通过「是否可能直接或间接导致对外发送/真实平台调用」一票否决审查。
- **与 ReadonlyWriteGuard 的关系**：MCP 写入口是新的独立面，不复用 `/api/v1` 的 admin session 鉴权；但其后端实现必须复用同一 service 层（不绕过 handler 层校验逻辑的等价物），确保权限/scope/状态机检查单点维护。

## 4. 分阶段落地建议与工作量估算

| 阶段 | 内容 | 工作量（Devin 轮次） |
| --- | --- | --- |
| **P0 最小集** | scope 双轴演进（`write:ops`）+ 环境/租户双开关 + dry-run/confirmationToken 状态机 + W1–W5 五工具 + 业务限额 + 写审计（含参数摘要）+ Admin token 页写 scope 创建/流水视图 + permmatrix/MCP 契约测试 + docs（mcp.md 写章节、env.md、permission-matrix.md） | 2 轮（后端 1.5 + Admin/文档/测试 0.5） |
| **P0 验收** | 安全审计专项（提示注入/确认绕过/限额绕过/审计 fail-closed 实测）+ Docker 全栈 E2E | 0.5 轮 |
| **P1 扩展** | W6–W9 四工具 + out-of-band 确认收件箱（如 P0 审计数据支持）+ token 绑定店铺清单（如有需求） | 1–1.5 轮（触发式：P0 上线后有真实使用数据再启动） |
| **P2 择机** | W13/W14（AI 任务触发、商品草稿字段级白名单——需单独细稿） | 单独立项，本稿不估 |

前置依赖：建议先合入 #322（view-only 越权 P1 修复）——写白名单的店铺 scope 口径依赖其确立的 `EnsureStoreOperable`；R161 建议 1 的合入顺序不变。

## 5. 不做清单（明确边界）

1. **永久不做（经 MCP）**：对外发送任何消息（W16）、真实平台 API 调用/刊登发布（W15）、token/租户/用户/权限/设置管理（W17）、删除类操作（W18）、支付/退款真实资金动作。
2. **本期不做**：审单放行/拒绝（W10）、库存扣减/回滚（W11）、一切批量写（W12）、写 scope 的「升级」入口、免确认执行模式、Webhook 触发写。
3. **不做的架构**：不为写操作另起第二套 token 体系（沿用 `mcp_api_tokens` 单表 + scope 演进）；不引入新的外部依赖（确认 token 用现有 Redis/DB 即可）。

## 6. 决策请求（供老板拍板）

| # | 决策点 | 建议 | 如无异议按此执行 |
| --- | --- | --- | --- |
| D1 | 是否立项 MCP 写白名单 | **采纳 P0 最小集（W1–W5）**，2.5 轮（含验收） | 批准后另开实现轮次；本轮只交设计稿 |
| D2 | 写范围是否对齐 AutoDS（含直接刊登） | **不采纳**——刊登/外发永久不进 MCP | — |
| D3 | 确认模式 | P0 用 dry-run+confirmationToken 双步；out-of-band 收件箱放 P1 触发式 | — |
| D4 | W5 mark-paid 是否进 P0 | 建议进（带租户金额上限、未配置即不可用）；如老板认为资金标记敏感，可降级 P0=W1–W4，不影响其余设计 | — |

## 附：证据索引

- 竞品来源（访问日期 2026-08-07）：autods.com（features/autods-claude-mcp、blog ×2）、help.autods.com（15505185）、shopify.dev/docs/agents/profiles/auth-and-rate-limiting、my.shop-mcp.app/docs/connect/shopify、github.com/Groupthink-dev/shopify-blade-mcp、weaverse.io（ai-agent-write-access-shopify-guardrails-mcp-hydrogen-2026）、open.mabangerp.com、dianxiaomi.com。
- 站内依据：`docs/mcp.md`、`docs/open-api.md`、`docs/permission-matrix.md`、`docs/api.md`（采购协同/发货/审单/自动化规则/订单标签/买家自动消息各节）、`backend/internal/modules/mcptoken/model.go`（scope/purpose 现状）、R159 审计报告（分支 `fix/round159-security-audit`）、`docs/COMPETITIVE_BENCHMARK_R161.md` §四-2/§六-2。
