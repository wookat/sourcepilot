# UX 视觉复核 v6 报告（重点：移动模式 #238 与 R109–R113 新页面）

- 复核日期：2026-08-05
- 复核角色：qa-engineer + user-experience-officer（真实卖家视角走查，Docker 全栈 demo 环境实测）
- 基线：main `e08e55b9`（R109–R113 全部已合并）+ 本地叠加唯一开放 PR #239（`0882ffbf`，移动首页指标卡 375px 截断修复）
- 环境：`docker compose -f docker-compose.full.yml up -d --build` + `DB_HOST=127.0.0.1 pnpm seed:demo:full`
- 视口：375 / 768 / 1440 三档；三角色（admin / operator / readonly）；双租户（demo tenant0 + 正规注册新租户，用后清退）
- 硬指标：走查页面 console error = 0、pageerror = 0、根节点横向溢出 = 0、403/500 噪音 = 0；`seed:demo:full:clean` + verify 零残留

## 一、走查范围

### 新页面/新动线（本轮重点）

| 批次 | 页面/动线 | 路由 |
| --- | --- | --- |
| R113 移动模式 | 移动首页（指标卡+关键待办） | /m/home |
| R113 移动模式 | 底部导航（首页/待办/我的） | /m/* |
| R113 移动模式 | 关键待办触屏动线 | /m/todos 等 |
| R109 违禁词 | 违禁词库设置 + 草稿合规命中提示 | /settings/banned-words、草稿详情 |
| R109 话术模板 | 话术模板 CRUD/分组/会话插入 | /customer/reply-templates |
| R110 深度报表 | 报表新维度切换（chartTokens） | /orders/reports |
| R110 AI 规避 | AI 优化违禁词规避提示 | 草稿 AI 动线 |
| R111 面单+发货规则 | 发货规则推荐物流商 + 面单模板 | /orders/list、/settings/shipping-rules |
| R112 多仓 | 仓库管理/签收选仓/按仓扣减/调拨/报表按仓 | /inventory/warehouses 等 |

### 老页面抽查（回归）

Dashboard、订单列表、经营报表、商品草稿、刊登任务、任务中心、库存中心、客服会话、登录/注册页。

## 二、真实卖家动线评价

### 1. 移动模式（#238/#239）

- /m/home 指标卡改纵向明细行后，375px 下销售额/毛利金额完整可读（#239 修复验证通过，核心验收点）；底部导航三 tab 触达清晰，触屏按钮高度符合拇指操作。
- 关键待办动线（待审核任务/失败任务/库存告警入口）从首页卡片直达，路径短；「我的」页提供桌面版工作台回跳，动线闭环。
- 未发现横向溢出或截断；PWA manifest 生效。

### 2. R109–R113 桌面新页面

- 违禁词（#229）：词库增删顺畅，草稿合规命中提示位置醒目；话术模板（#228）分组过滤与会话插入动线自然。
- 深度报表（#231）：维度切换出图使用 chartTokens 统一配色；时间格式 YYYY-MM-DD HH:mm 一致；空态文案符合统一口径。
- 多仓闭环（#236）：签收选仓→按仓扣减→调拨→报表按仓筛选全程可跑通；仓库管理页「默认仓承接存量库存」说明与迁移预检 Descriptions 信息设计清楚。
- 无障碍抽查：图标按钮带中文 aria-label（如仓库排序上移/下移）、Tag 语义色、表单错误关联正常。

### 3. 开箱注册动线（新租户）

- 未配置 SMTP 时点「获取验证码」此前仅提示「发送失败」，形成开箱死路（P1-1，本轮已修复：后端可操作指引文案现可直达用户）。
- 注册开租后隔离正确：demo 数据完全不可见，越权路由走统一「无法访问该页面」空页。

## 三、问题清单

### P1（本轮直接修复，随本报告 PR）

| 编号 | 页面 | 问题 | 修复 |
| --- | --- | --- | --- |
| P1-1 | 登录/注册页 | 未配置 SMTP 时「获取验证码」仅提示「发送失败」，后端返回的中文指引（50301「邮件服务未配置…」）被前端丢弃，开箱注册死路 | 登录页错误提示改用共享 `httpErrorCopy`（中文 envelope message 原样透出、错误码走 ERROR_MAP 映射），补单测 |
| P1-2 | 运营任务批量审批弹窗 | Modal `destroyOnHidden` 下 `openBatchModal` 先 `resetFields` 再挂载 Form，触发 `useForm is not connected to any Form element` console.error，admin/e2e 全量 2 条失败（round63-optask-batch） | Modal 改 `forceRender`（Form 常驻挂载），`resetFields` 时机不变；round63 spec 12/12 通过 |

### P2（列清单，不在本轮修复）

| 编号 | 页面 | 问题 | 建议 |
| --- | --- | --- | --- |
| P2-1 | /inventory/warehouses | 「默认仓」为保留编码 `default` 的固定仓，UI 无「设为默认」切换入口，#237「默认仓唯一」索引兜底在 UI 层不可端到端触发；「不可删除/停用」文案未说明能否切换 | 若产品上确不支持切换默认仓（轻量多仓口径），在页头补一句说明；若支持则需补 UI 入口 |
| P2-2 | seeddemo verify | 软删除（`deleted_at` 非空）历史行仍被 verify 计为残留，易误报（本轮清理了一条历史 `QA-` 软删除话术模板） | verify 统计排除软删除行，或输出中区分「软删除残留」 |
| P2-3 | 新租户越权路由 | 权限外路由与不存在路由共用同一空态「页面不存在，或当前账号无权访问」，难判断是权限还是地址错误 | 权限不足与 404 分开文案 |

> 未发现 P0。chartTokens、语义 Tag、时间格式、空态文案、无障碍抽查均符合 UX_REVIEW_V4 标准。

## 四、Top 改进建议

1. 默认仓语义显性化（P2-1）：仓库管理页头一句话说明默认仓定位与是否可切换，消除「为什么没有设为默认按钮」的困惑。
2. verify 语义与软删除口径（P2-2）：避免回归时误判残留。
3. 权限空态与 404 分离（P2-3）：新租户/低权限角色的可理解性。

## 五、回归结论

- 全量门禁（check:dev / check:ui-copy --strict / test:frontend / test:collector / test:contracts / build:admin / build:collector / go vet / gofmt / go test / backend:integration / db / redis）全部通过。
- admin/e2e Playwright 全量此前 214 passed / 2 failed（P1-2 所致），P1-2 修复后 round63 相关 spec 12/12 通过。
- **PR #239 功能验证通过，可合并**；多仓/移动模式/R109–R111 新功能动线全部通过；#237 属后端索引兜底（Go 回归测试通过），UI 层无切换入口属产品口径（P2-1）。
