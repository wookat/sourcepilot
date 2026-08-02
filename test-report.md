# TradeMind Round 59 商品草稿模块 — 修复后回归报告

> **本报告为修复后回归（Round 59 regression），覆盖此前 exploratory audit 的旧结果。**
> 环境：docker compose full 栈（backend/admin 已重建，含修复代码），Admin http://127.0.0.1:8000 ，Backend http://127.0.0.1:8080 。
> 方式：真实浏览器 UI 走查（admin + readonly 双角色）+ curl API 权限断言 + CDP 视口模拟，全程录屏加注解。

## 结论总览

| # | 回归项（对应原发现） | 结果 |
|---|---|---|
| R1 | AI 未配置降级文案（原 P1-1） | ✅ 通过 |
| R2 | 归档二次确认 + 删除回归（原 P2-1） | ✅ 通过 |
| R3 | 新建 SKU 后行操作完整（原 P2-5） | ✅ 通过 |
| R4 | operationStep 筛选说明 Alert（原 P2-3 缓解） | ✅ 通过 |
| R5 | 批量发布检查无授权店铺空态（原 P2-4） | ✅ 通过 |
| R6 | readonly UI 写入口隐藏 + 写 API 403（原 P1-2） | ✅ 通过 |
| R7 | 375/768/1440 响应式无根级横向溢出 | ✅ 通过 |

本轮回归未发现新的 P0/P1/P2 问题。

## R1 AI 未配置降级文案 — 通过

admin 登录 → `R1 demo multi SKU product` 详情 → AI 文案 Tab → 生成标题建议 → 提交。
错误提示显示后端 envelope 的可行动文案 **「请配置 base_url」**，不再是 `Request failed with status code 400`。

![AI degraded message shows 请配置 base_url](https://app.devin.ai/attachments/3946f27f-89f3-4d62-bd47-45ee972c5a5a/ss_b576a42d.png)

## R2 归档二次确认 + 删除回归 — 通过

一次性 fixture「R59 regression fixture」：
- 点「归档」出现 Popconfirm「确定归档该草稿？」（描述：归档后不能批量刊登，可在本页重新标记为可用）；取消保留原状态；确认后 toast「已归档」且状态变为 archived。
- 删除同样有 Popconfirm 二次确认；确认后列表恢复 26 条，fixture 消失。

| 🔴 归档 Popconfirm（确认前） | 🟢 确认后已归档 |
|---|---|
| ![archive popconfirm](https://app.devin.ai/attachments/a27c53c5-65cf-4a9f-a246-a7a829f0f9c3/ss_c79522d5.png) | ![archived](https://app.devin.ai/attachments/f62b7ac9-d6d5-45ff-bd9e-127ee028af3f/ss_6139fd09.png) |

| 🔴 删除 Popconfirm | 🟢 删除后列表 26 条、fixture 消失 |
|---|---|
| ![delete popconfirm](https://app.devin.ai/attachments/ac23b51d-fd1f-41cc-ae04-76fde87dd469/ss_aae47050.png) | ![list restored](https://app.devin.ai/attachments/59fd7fd4-4c39-46f4-87ad-2d37d952a854/ss_1c445205.png) |

## R3 新建 SKU 后行操作完整 — 通过

fixture SKU Tab 新建 `R59-REG / 1.00 / 1` 保存后，不刷新页面，新行立即同时显示「编辑」「删除」。

![new SKU row shows 编辑+删除 immediately](https://app.devin.ai/attachments/39370b2a-61db-446c-94a4-2fd83c52b326/ss_2d663c77.png)

## R4 operationStep 筛选说明 Alert — 通过

运营进度选「待生成描述」查询后，URL 同步 `?operationStep=description`，列表上方出现 info Alert：
「运营进度筛选展示「该步骤尚未完成」的商品；行内标签是每个商品当前所处步骤，可能早于所筛步骤。」重置后 Alert 消失。

| 🟢 筛选激活时显示说明 Alert | 🟢 重置后 Alert 消失 |
|---|---|
| ![alert shown](https://app.devin.ai/attachments/df7c1d7d-a560-4b39-8c5c-c78deb7b5735/ss_ca6da9b1.png) | ![alert gone](https://app.devin.ai/attachments/eb77ca75-4e0b-4f94-949f-1be2e79b495d/ss_c6d8e044.png) |

## R5 批量发布检查无授权店铺空态 — 通过

选中 1 行 → 批量发布检查抽屉：显示警告空态「当前平台（tiktok）没有已授权店铺」，附「店铺管理」链接与切换平台指引；店铺选择器为禁用状态。

| 选中行批量工具栏 | 🟢 无授权店铺警告空态 + 店铺选择禁用 |
|---|---|
| ![batch toolbar](https://app.devin.ai/attachments/607ed89c-72c0-4713-bba8-b614317cdd5f/ss_238f5824.png) | ![no shop empty state](https://app.devin.ai/attachments/bd34c8a2-a52f-462d-9da0-08f03c8492c1/ss_zoom_f0e968fd.png) |

## R6 readonly 写入口隐藏 + 写 API 403 — 通过

**UI（demo_readonly 登录 /product/drafts）**：「新建草稿」「更多」按钮均不渲染（DOM 验证 + 截图）。readonly 账号店铺 scope 为空，列表无数据，因此选中行批量工具栏与详情页按钮无法在 UI 触发——该限制如实记录；详情页 readonly 分支已由源码 readonly 判断控制，且写 API 已全部经由下方直连验证。

| 🟢 readonly 列表无写入口（对照 admin 有 新建草稿/更多） | readonly 空 scope 空态 |
|---|---|
| ![readonly list header](https://app.devin.ai/attachments/2ae298d8-688a-494f-aac1-affd8249967b/ss_zoom_2344d7de.png) | ![readonly empty](https://app.devin.ai/attachments/5f21e113-7b3a-4838-a852-84b576745c36/ss_927aca24.png) |

**API（readonly token 直连，商品 aae5148b-d70a-4430-81a5-e19780b52ec7）**：

| 请求 | HTTP | code | message |
|---|---|---|---|
| PUT /api/v1/products/:id | 403 | 40301 | 当前账号为只读权限，无法执行此操作 |
| POST /api/v1/products/:id/skus | 403 | 40301 | 当前账号为只读权限，无法执行此操作 |
| POST /api/v1/products/:id/ai/optimize-title | 403 | 40301 | 当前账号为只读权限，无法执行此操作 |
| DELETE /api/v1/products/:id | 403 | 40301 | 当前账号为只读权限，无法执行此操作 |

原来的 PUT/DELETE→404（scope-first）已改为路由级 403，文案精确匹配。

## R7 响应式快查 — 通过

CDP 模拟视口，`document.documentElement.scrollWidth <= innerWidth + 4`：

| 页面 | 375×812 | 768×900 | 1440×900 |
|---|---|---|---|
| 列表 /product/drafts | sw=375 ✅ | sw=753 ✅ | sw=1425 ✅ |
| 详情 /product/drafts/:id | sw=375 ✅ | sw=753 ✅ | sw=1425 ✅ |

![detail at 375 mobile layout](https://app.devin.ai/attachments/c118bd55-c4c2-475e-8983-fea8f2608a19/ss_83556d82.png)

## 诊断与清理

- **Console**：整轮回归无 error 级输出（结束时 console 为空）。
- **网络 4xx/5xx**：仅 AI 未配置的预期 400（R1 场景本身）；readonly 403 为直连 curl 验证，UI 内无意外 4xx/5xx。
- **Fixture 清理**：R59 regression fixture（含 R59-REG SKU）已通过 UI 归档→删除；API 复核商品总数 26、无标题含 "R59" 的残留；种子数据未改动。

## 备注 / 剩余风险

- readonly 账号店铺 scope 为空导致「选中行工具栏对 readonly 隐藏批量写按钮、保留批量发布检查」与「详情页头部按钮隐藏」无法在真实 UI 中直接演示（无可见数据）。该逻辑已由源码 readonly 分支 + 全部写 API 403 间接覆盖；如需 UI 级证据，可给 readonly 账号分配一个授权店铺后复验。

## 证据

- 录屏（含 test_start / assertion 注解）：`/home/ubuntu/screencasts/rec-ab9ca684-639b-4cc0-8372-eb77ae3f23fb/rec-ab9ca684-639b-4cc0-8372-eb77ae3f23fb-edited.mp4`
- 回归计划：`.devin-test-plan-round59-regression.md`
