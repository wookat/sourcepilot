# R164 线2：客服/AI 工作流季度复查（qa-engineer）

日期：2026-08-07。距 R141 线2 复查 22 轮，期间合入 R142 #291（消息规则仅新事件+回溯开关）、R152 #309（多语言模板）、R159/R160 view-only 修复等。本轮在 Docker 全栈（`docker-compose.full.yml` + `seed:demo:full`）实测客服/AI 全链路、权限口径与三角色三视口 UI 走查。录屏/测试报告外置不入库；Actions CI 不作依据（结论基于本地 go test / 实测）。

## 1. API 层全链路实测（38 项 + 双租户 16 项）

- 话术模板：CRUD、多语言变体维护（PUT variants 全量替换）、非法语言码返回可选值清单——通过。
- AI 建议：`generate-reply` 未配置 base_url 时返回 400 可读中文提示（无 5xx，降级路径可接受）；建议列表/编辑/采纳/拒绝口径正常。
- 消息节点规则：仅新事件默认（`effectiveFrom` 生效时间戳）、回溯开关、`backfill-estimate` 数量预估幂等、节点必填 400——通过。回溯正例：delivered 节点 estimate=1 → generate created=1 → 再次 generate created=0（幂等）。注：估算按「订单×节点已存在草稿则排除」的口径去重，shipped 节点因 seed 已有草稿估算为 0 属正确幂等行为，非 bug。
- 买家消息草稿：变量填充（缺失变量保留占位并计入 missingVars）、编辑重算、语言切换重生成（en 变体）、mark-sent 幂等、batch-mark-sent（updated+skipped）、非 pending 不可忽略——通过。
- 人工发送闸门：生成/回溯/批量标记均只落草稿；外发仅 `send-platform-message` 显式人工路径（校验店铺授权/Provider/幂等键），代码与 UI 文案均明示「不会向买家发送」——通过。
- 双租户隔离：临时第二租户 16 项断言全过（列表零可见、跨租户读写一律 404、外租户 templateId 拒绝、batch 无副作用）。
- AI key 脱敏：settings 响应无 `sk-` 明文——通过。

## 2. 发现并修复（P1）：view-only 店铺授权可写客服会话

- 现象：店铺 `view` 授权的 operator（R160 persona 口径）对客服会话写接口全部放行——可 `mark-replied` 落 agent 消息、可触发 AI 建议、可编辑/删除会话、可走 `send-platform-message` 入口（仅被业务校验拦下）。买家消息草稿写路径 R159 已收口，会话族漏收。
- 修复：`customerchat` 新增 `findScopedConversationForWrite` / `findScopedSuggestionForWrite`（`EnsureStoreOperable`，view-only → `ErrStoreNotOperable`），覆盖编辑/删除会话、创建绑定店铺会话、添加消息、mark-replied、`ai/generate-reply`、建议编辑/采纳/丢弃/apply/reject、`send-platform-message`；handler 统一映射 403 业务码 40303「店铺无操作权限」（与订单写路由、买家消息草稿一致）；会话详情 `canWrite` 对 view-only 返回 `false`。
- 回归测试：`permmatrix` 新增 `TestViewOnlyPersonaConversationWriteScope`（13 条写探针 403+40303、零落库、读保持可用、detail canWrite=false）。
- 实测：重建 backend 容器后 UI + API 双重验证 403 生效、无 5xx/白屏、DB 零新增。

## 3. UI 三角色三视口走查

- admin/operator/readonly + r164 临时 view-only 账号走查客服工作台/话术模板/消息节点规则/买家消息草稿四页面；operator 仅见授权手工店会话；readonly 详情页只读 Alert + 写按钮 disabled。
- 1920/768/375 三视口四页面根节点无横向溢出（`scrollWidth<=clientWidth` 实测）；375 移动端底部导航正常、表格内部滚动。
- 不存在会话 id 显示友好错误非白屏；全程 console 无 error / React / AntD 告警；中文文案未见异常。

## 4. P2 清单（本轮不改行为）

1. regenerate 缺变体口径文档漂移：`docs/api.md` 原写「无变体时回退默认语言并标 no_variant」，实际实现（R152 起）人工指定语言缺变体返回 400 提示先维护；`no_variant` 回退仅在自动生成的语言推断路径。本轮已按实现修正文档（人工切换报错引导维护更符合工作台交互，前端已按 error 提示处理）。
2. view-only 授权前端未呈现只读口径：会话详情页无只读 Alert、写按钮可点（仅后端 403 兜底）。后端本轮已让 detail `canWrite=false`，前端可复用 readonly 禁用逻辑跟进。
3. readonly/view-only 会话列表仍显示「新建会话/拉取平台消息」入口（点击被 403 拦截）。
4. `POST /shops/:id/sync-customer-messages` 与 sync 任务 retry 对 view-only 授权放行（仅 `EnsureStoreVisible`）：与 `sync-orders` 现行口径一致（同步视为数据导入非业务写），是否收口到 operate 建议下轮统一定口径。

## 5. 清理与残留核对

- 测试期间创建的临时话术模板/规则/草稿/第二租户账号/r164-viewonly 账号均已清理；`seed:demo:full:clean` + `verify` 零 DEMO- 残留（见验收输出）。
