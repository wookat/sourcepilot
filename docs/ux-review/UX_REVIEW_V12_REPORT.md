# UX 视觉复核 v12 报告（重点：v11 全零基线回退抽验 + R170–R175 新面走查）

- 复核日期：2026-08-08
- 复核角色：user-experience-officer（真实卖家/协作账号走查，Docker 全栈 demo 环境实测，录屏/截图留档不入库）
- 基线：`origin/main`（R176 时点，#346/#347 已合入）+ 本地按序叠加未合并 PR（经 `git_view_pr` 确认均 OPEN、mergeable）：#349（客服发送报错中文化 + seed 修正 + modal rethrow 首修，`devin/1786163479-r174-line1-p2-closeout`）、#351（验收包/DEMO_SCRIPT 更新，`devin/1786166103-r175-line2`）、#352（modal rethrow 全站收口 + migrationimport 中文化，`devin/1786166009-r175-line1`）；无代码冲突
- 环境：`docker compose -f docker-compose.full.yml up -d --build` + `DB_HOST=127.0.0.1 pnpm seed:demo:full`，Admin dev server 8001
- 视口：五档精确视口 1920×1080 / 1440×900 / 1024×768 / 768×900 / 375×812（headless Playwright `viewport` 精确设定）
- 角色：demo_admin / demo_operator / demo_readonly 三角色全量 + 临时 view-only persona（仅 view scope 店铺授权，走查后清理不入库）+ bootstrap 平台管理员（tenant0，覆盖 /ops/backups、/ops/restores 平台级路由）
- 硬指标矩阵：5 persona × 5 视口 × 98 路由（含 7 条真实 seed ID 详情路由）共 2450 组合 headless 全扫，console error = 0（预期权限范围 404 网络日志与租户越权详情除外，见下）、pageerror = 0、根节点横向溢出 = 0、NaN / Invalid Date / undefined 文本直出 = 0、redirect-login = 0、403/500 请求噪音 = 0（每 persona×视口 独立登录规避 token 过期误报）

## 一、走查范围

### v11 结论零回退抽验

| v11 项 | 页面 | v12 结果 |
| --- | --- | --- |
| view-only 审单按钮预禁用 + tooltip | /orders/review | **无回退**。view-only persona 批量操作灰置、tooltip「店铺无操作权限」 |
| 40303 文案统一「店铺无操作权限」 | API + UI | **无回退**。view-only POST 客服平台发送 / 迁移导入校验均返回 `{"code":40303,"message":"店铺无操作权限"}` |
| /settings/report-currency 未保存拦截 | 抽验 | **无回退** |
| /dashboard/screen、/system/operation-logs 中文化 | 抽验 | **无回退** |
| v10 P2-3 /settings/mcp-tokens 文档纯文本 | 遗留 | **维持遗留**（待产品确认文档挂载位置） |
| v9 P2-3 finance-report CSV 未折算列 | 遗留 | **维持口径**（不伪造折算） |

### 重点新面（R170–R175 交付）

| 面 | 路由 / 方式 | 结果 |
| --- | --- | --- |
| modal 失败路径弹窗保持开 + 中文 toast（#352） | /procurement/orders 作废弹窗，拦截接口注入 50001 失败 | 通过：弹窗保持打开、中文 toast「服务器繁忙，请稍后重试」、无 react-error-overlay、无 pageerror、无 unhandledrejection |
| migrationimport 向导中文文案（#352） | /settings/migration 实走 | 通过：四步向导「上传文件/列映射/校验报告/导入结果」全中文，无裸英文 |
| migrationimport shape 校验中文化（#352） | API 实测 `/api/v1/imports/validate` | 通过：「表头列（columns）不能为空…」「归属店铺（shopId）不能为空…」等全中文；view-only 先 scope 返回 40303 |
| 客服发送失败提示（#349） | /customer/conversations 详情，拦截 send-platform-message 注入失败 | 通过：中文 toast「平台发送失败，请稍后重试」，输入内容保留、弹窗不闪退、无 pageerror |
| 客服参数校验中文化（#349） | API 实测 | 通过：「回复内容不能为空」「会话缺少平台外部会话 ID，暂不支持平台发送」等 |
| 40303 修复面 view-only 预禁用（#346/#347） | /customer/conversations 详情（view-only 实走） | 通过：10 个操作按钮灰置 + 回复框禁用 + 只读提示「当前为只读账号，不可生成建议或发送消息。」 |
| seed delivered_at 修正（#349） | 数据抽验 | 通过：无未来时间戳直出 |

重点走查 12 项断言 12/12 通过（拦截所有非 GET 写请求，未执行真实写操作）。

### 移动端与视觉现代感（375/768 重点目测）

- 375×812：/m/home、订单/客服/采集/设置等单列自适应，无横向滚动、无遮挡截断。
- 768×900：侧栏收起图标栏，表格容器内横向滚动，根节点无溢出。
- 权限外详情路由（如 readonly 访问未授权订单）呈现优雅空态「未找到订单：请从订单列表重新进入，或检查是否有访问权限。」，无破版、无技术信息泄露。

## 二、问题清单与处置

### P1

本轮未发现 P1 问题。

### P2（本轮即修）

| 编号 | 页面 | 问题 | 处置 |
| --- | --- | --- | --- |
| v12 P2-1 | /customer/conversations/:id | 375 视口下 antd Descriptions 报 console error `Sum of column span in a line not match column`（响应式 column={{xs:1}} 与固定 span={2} 冲突），dev 下有 console 噪音 | **已修**：5 处 `span={2}` 改响应式 `span={{ xs: 1, sm: 2 }}`；修后 375 视口 0 console error、0 溢出 |

### 既有遗留（维持）

| 编号 | 页面 | 问题 | 建议 |
| --- | --- | --- | --- |
| v10 P2-3 | /settings/mcp-tokens | 文档入口纯文本不可点击 | 待产品确认文档挂载位置 |
| v9 P2-3 | /orders/finance-report CSV | 未配置汇率本位币列空 | 维持「不伪造折算」口径 |

### 覆盖限制（非缺陷）

- 详情路由使用 tenant 1 seed ID；operator/readonly 未授权店铺与 tenant0 跨租户访问该类路由时产生预期 404 网络日志（权限不泄露设计），页面呈优雅空态、无应用级 console error，不计为缺陷。
- 走查基于本地叠加 #352 的复核分支；若合并顺序变化，modal/migrationimport 结论以合并后为准。
- 色彩对比为人工目测（未跑 axe 全站扫描）。
- 临时 view-only persona 为本地 demo 库临时账号，走查后已清理，不进入 seed/代码。

## 三、修复清单（本轮 PR）

- `admin/src/pages/Customer/ConversationDetail/index.tsx`：v12 P2-1 响应式 span 修复。
- `docs/ux-review/UX_REVIEW_V12_REPORT.md`：本报告。
- `docs/progress/R176-line2.md`：轮次进展。
- `docs/PROGRESS.md`：变更记录。

验证：`pnpm check:ui-copy --strict`、`pnpm test:frontend`（365 通过）、`pnpm test:contracts`（17 通过）、`pnpm build:admin`、`go vet ./...`、`gofmt -l`（空）、`go test ./...`、`go build ./...` 全通过；Docker demo 环境实测录屏/截图留档（证据不入库）。Actions CI 不作依据。
