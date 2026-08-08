# R173 线2：客服/AI 工作流季度复查（qa-engineer）

日期：2026-08-08。距 R164 线2 复查 9 轮，期间客服线经历 #330 view-only 会话族修复合入、R152 多语言模板持续运行、40303 文案统一（#337 + #346 收口）。本轮在 Docker 全栈（`docker-compose.full.yml` + `seed:demo:full`）实测客服/AI 全链路、权限口径与三角色三视口 UI 走查。测试栈 = `origin/main`（`7bf6137e`）+ PR #346 分支叠加（40303 文案统一漏网四处，盘点时未合并；PROGRESS 清稿性冲突按双条目保留解决）。录屏/证据外置不入库；Actions CI 不作依据（结论基于本地实测）。

## 1. API 层全链路实测（约 50 项断言）

- **话术模板**：CRUD、多语言变体全量替换（PUT variants en+es 替换验证）、非法语言码 400 返回可选值清单——通过。
- **AI 建议降级路径**：未配置 AI base_url 时 `generate-reply` 返回 400 可读中文配置类提示（无 5xx、无伪成功）；建议编辑（`editedReply`）/采纳（`finalReply`）/apply/reject 口径正常——通过。
- **消息节点规则**：节点必填 400；仅新事件默认（`effectiveFrom`）——paid 节点存量 30 单 estimate=30、新建规则 generate created=0（存量正确排除）；`backfill=true` 后 generate created=30（与预估一致）、再次 generate created=0（幂等）；delivered 节点回溯正例 estimate=1 → created=1 → 0——通过。
- **草稿链路**：变量填充（订单号/物流单号/买家昵称实填，无裸占位）、编辑重算 missingVars（注入 `{unknownVar}` 正确登记）、语言切换 regenerate（en 变体 `langSource=manual`）、缺变体人工指定语言 400 引导先维护、mark-sent 幂等、非 pending 不可忽略 400、batch-mark-sent updated+skipped——通过。
- **人工发送闸门**：generate/backfill/batch-mark-sent 均只落草稿零外发；`send-platform-message` 为唯一显式人工外发路径，demo 环境（无平台外部会话 id/凭证）业务校验拒绝、无伪成功、零落库——通过。
- **AI key 脱敏**：settings 响应无 `sk-` 明文——通过。

## 2. 权限面复查（#330 零回退）

- **view-only 会话族（R164 #330 修复面）**：给 demo_operator 临时追加抖店 `view` 授权（仅本地库、实测后删除），14 条写探针（编辑/删除会话、创建绑定店铺会话、添加消息、mark-replied、generate-reply、ai-suggestions 别名、send-platform-message、建议编辑/accept/discard/apply/reject、绑定店铺新建会话）全部 **403 + 40303 +「店铺无操作权限」统一文案**；读保持 200、detail `canWrite=false`；零落库。**零回退。**
- **readonly**：写 403/40301、drafts 列表 `canWrite=false`、读正常。
- **双租户隔离**：tenant2 admin 读/写 tenant1 会话一律 404（不泄露存在性）、会话列表零 tenant1 数据、引用 tenant1 templateId 建规则被拒、batch-mark-sent 打 tenant1 草稿 updated=0 零副作用。
- **40303 文案统一（#346 修复面叠加实测）**：四处漏网模块 view-only 实弹全部返回统一「店铺无操作权限」——orderexception `order/:id/handle`、finance `POST /finance/payments`（view-only 店订单）、productpublish `create-drafts`（view-only 目标店）、migrationimport `imports/validate`（kind=product + view-only shopId）。

## 3. UI 三角色三视口走查（录屏留证）

- admin/operator/readonly × 1920/768/375，客服会话工作台/买家消息（双 tab）/话术模板三页面：四页面 12/12 视口组合根节点 `scrollWidth <= clientWidth`；375 底部导航正常、表格内滚。
- operator：view-only 抖店会话详情**已显示只读 Alert + 写按钮 disabled**（R164 P2-2「前端未呈现只读口径」已不复现，现状改善）；operate 手工店会话写入口正常。40303 toast 实测为统一中文文案，无裸 JSON/英文 axios 直出。
- readonly：列表页「新建会话/拉取平台消息」入口不渲染（R164 P2-3 亦已改善）；详情页只读 Alert + 全部写按钮 disabled。
- 全程 console 无 error / React / AntD fatal warning。

## 4. P0/P1

无。本轮无需修复代码。

## 5. P2 清单（登记不阻塞）

1. `send-platform-message` 业务校验报错英文直出：`conversation has no platform external id`（建议中文化，与全站中文口径一致）；同族英文参数校验文案 `reply is required`/`finalReply is required`/`editedReply is required` 同批处理。
2. 参数校验先于店铺 scope 判定的模式在 migrationimport 依旧存在（kind/mapping 必填校验先于 `resolveShop`，view-only 缺参得 400 而非 403）——与 R168 P2-2 同模式，无越权与存在性泄露，探针需带合法 body；`kind=payment` 不需要店铺（`kindNeedsShop` 仅 product/order），实测用 product 面验证。
3. demo seed 的 `DEMO-SO-DELIVERED-0004.delivered_at` 为未来时间戳（+18h）：新建「仅新事件」delivered 规则会立即命中该单（事件时间 >= 生效时间，语义正确），但演示「仅新事件不回溯」时该样本有误导性，建议 seed 改为过去时间戳或文档注记。

## 6. 清理与残留核对

- 临时 view 授权行、r173 模板/规则/草稿均已清理；`seed:demo:full:clean` + `verify` 输出 `zero DEMO- residual rows`；`buyer_message_drafts`、`%r173%` 残留、`user_store_permissions` 全部为 0。
