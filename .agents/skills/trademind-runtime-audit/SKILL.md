---
name: trademind-runtime-audit
description: TradeMind 全栈本地运行走查的实操要点：登录 payload、角色账号行为、AI 未配置降级路径、响应式模拟与 fixture 清理
---

# TradeMind 运行时走查要点

## 登录 / API
- 登录 body 字段是 `account` 不是 `email`：`POST /api/v1/auth/login {"account":"...","password":"..."}`。
- AI 标题优化真实路由是 `POST /api/v1/products/:id/ai/optimize-title`（不是 `/ai/title`）；未配置 Provider 时后端返回 `{"code":40001,"message":"请配置 base_url"}`，UI 可能只显示裸 `Request failed with status code 400`。
- 权限语义：readonly POST 商品 → 403(40301)；readonly/operator 对 scope 外商品的 GET/PUT/DELETE → 404（scope 过滤优先，无存在性泄露）。

## 角色账号（demo seed）
- `demo_readonly@trademind.local` / `demo_operator@trademind.local` 均 `storePermissions=[]`，商品列表 API total=0，UI 显示带解释的空态；readonly 的 UI 写入口可能仍可见（历史缺陷，验证时注意）。

## 响应式验证
- Chrome 窗口最小宽约 500px，无法用 wmctrl 缩到 375。用 CDP `Emulation.setDeviceMetricsOverride`（remote-debugging 端口见 chrome 进程 cmdline `--remote-debugging-port`），websocket 连接需 `suppress_origin=True`。
- 恢复时 `clearDeviceMetricsOverride` 可能不生效，先 `setDeviceMetricsOverride {width:0,height:0,deviceScaleFactor:0}` 再 clear。
- 溢出判定：`document.documentElement.scrollWidth <= innerWidth + 4`。

## Fixture 清理
- 删除/归档只用自建商品（列表页「新建草稿」）；删除是软删除，删除后列表 total 应回落，但工作台商品草稿卡片计数可能仍含软删/归档（口径不一致）。
- 详情页归档无二次确认，删除有 Popconfirm——测试断言时区分。

## Devin Secrets Needed
- 无（demo 账号密码在 seed 文档/任务描述中）。
