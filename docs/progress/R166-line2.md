# R166 线2：view-only 前端体验与后端权限一致性审计

- 轮次：R166 线2（UX / 权限一致性）
- 角色：user-experience-officer
- 基线：`main` 叠加 PR #330 → #332（依赖这两个 PR 先行合入；本分支含叠加时的合并解决与一处测试口径对齐）
- 验证方式：Docker 全栈（`docker-compose.full.yml`）+ `seed:demo:full` 实测，Actions CI 不作依据；全程录屏留证（录屏不入库）

## 背景

R165 修复了六处 view-only 后端越权（审单决定、异常标记族、店铺删除、店铺授权/OAuth 写、店铺同步/重试、刊登目标店），后端统一收口为：view-only 店铺写操作 → 403/40303「店铺无操作权限」，不可见店铺 → 404「记录不存在或已被删除」；审单批量端点当时按 #332 语义保持 200 信封、按行返回失败（**R167 定案更新**：批量中含 view-only 店铺订单时整批 403/40303 拒绝，见 `docs/progress/R167.md`）。本轮验证前端体验是否与该口径一致。

## 实测方法

- demo 账号：admin / operator / readonly（`seed:demo:full`）。
- 为构造「view-only 店铺」人格，在测试库内给 demo_operator 追加两条 `permission_scope=view` 的店铺授权（抖店、TikTok 演示店），其原有手工渠道店为 `operate`。该数据仅存在于本地测试库，不入库、不入 seed。
- 覆盖：六个写操作面（operator 对 view-only 店铺）、readonly 三个面抽验、operator/admin 正向操作回归、视口（最大化 1920 / ~768 / ~375）响应式、空态/错误态文案、控制台错误。

## 结论

六个面全部 PASS，无 P0 / P1 缺陷：

| 面 | 结果 |
|---|---|
| 审单放行/拒绝（view-only 店订单） | 中文提示含「店铺无操作权限」，订单状态不变 |
| 订单异常标记/取消标记 | view-only 被拒（中文提示）；operate 店正向标记成功 |
| 删除店铺 | 403/40303 中文 Toast，店铺仍在 |
| 刷新授权/解除授权/同步店铺信息 | 403/40303 中文 Toast，授权状态不变 |
| 同步订单/客服消息/同步任务重试 | 中文提示，无越权变化 |
| 刊登目标店选 view-only 店 | create-drafts 403/40303，无刊登任务生成 |

- readonly：审单批量按钮 disabled + tooltip「只读账号不可操作」，店铺/异常页写操作隐藏。
- admin/operator 正向操作（审单放行、异常标记）无回退。
- 响应式：/orders/review、/shops/manage 各视口无根级横向溢出，375 显示底部导航。
- 控制台全程无 error / unhandled rejection；全站错误文案兜底（`admin/src/utils/httpErrorCopy.ts`）保证无裸 40303 JSON / 英文 axios 原文直出。

## P2 清单（未修复，列入后续轮次）

1. 店铺管理「删除店铺」确认弹窗正文中文但按钮为英文 `Cancel`/`OK`（缺 okText/cancelText），语言一致性问题。
2. 审单工作台放行/拒绝按钮仅按全局 readonly 禁用；operator 对 view-only 店铺订单按钮可点，靠点击后 403 中文提示兜底（符合验收口径，但建议按店铺 scope 预禁用；需要工作台行数据带 shopId + profile storePermissions 口径打通）。

## 未覆盖项

- 真实 OAuth 外部 redirect：demo 环境缺抖店/TikTok 平台应用凭据，无法生成有效授权链接；UI 前置中文提示「请先完成「平台接入设置」中的必填项。」合规。需真实凭据方可覆盖。

## 本分支代码变更

- `backend/internal/securitytests/permmatrix/r165_store_write_scope_test.go`：叠加合并解决——审单批量端点当时在 #332 语义下保持 200 信封、按行失败（「店铺无操作权限」）且不改 review_status，测试断言与之对齐（**R167 已按整批 403/40303 定案重新对齐**，本分支采用 v29 口径的测试版本）。
- `docs/progress/R166-line2.md`：本报告。
