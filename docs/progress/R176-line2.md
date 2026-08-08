# R176 线2：全站视觉/UX 复核 v12（user-experience-officer）

- 日期：2026-08-08
- 基线：`origin/main` + 本地按序叠加未合并 PR #349/#351/#352（经权威 `git_view_pr` 核实均 OPEN、mergeable，无代码冲突）
- 范围：v11 全零基线回退抽验 + 5 persona × 5 视口 × 98 路由全扫 + R170–R175 新面重点走查；P1 即修 / P2 修复或列清单
- 报告：`docs/ux-review/UX_REVIEW_V12_REPORT.md`

## 一、执行摘要

1. **环境**：Docker 全栈（`docker-compose.full.yml`）+ `DB_HOST=127.0.0.1 pnpm seed:demo:full`，Admin dev server 8001；临时 view-only persona 走查后清理不入库。
2. **矩阵全扫**：5 persona（admin/operator/readonly/view-only/tenant0）× 5 视口（1920/1440/1024/768/375）× 98 路由（含 7 条真实 seed ID 详情路由）共 2450 组合，每 persona×视口 独立登录。结果：pageerror = 0、根横向溢出 = 0、NaN/Invalid Date/undefined = 0、redirect-login = 0、403/500 噪音 = 0；console error 仅两类——①未授权店铺/跨租户详情路由的预期 404 网络日志（权限不泄露设计，页面呈优雅中文空态，非缺陷）；②375 视口客服会话详情 antd Descriptions span 告警（v12 P2-1，本轮已修）。
3. **新面走查**（拦截全部非 GET 写请求，12 项断言 12/12 通过）：
   - modal 失败路径：采购单作废弹窗注入失败 → 弹窗保持打开 + 中文 toast + 无 error-overlay/pageerror；
   - migrationimport：向导四步全中文；API 实测 shape 校验中文（「表头列（columns）不能为空…」等），view-only 先 scope 返回 40303；
   - 客服发送失败：中文 toast「平台发送失败，请稍后重试」，输入保留；API 校验「回复内容不能为空」等全中文；
   - 40303 修复面：view-only 会话详情 10 个操作按钮预禁用 + 回复框禁用 + 只读提示；审单页 tooltip「店铺无操作权限」无回退。
4. **v11 遗留维持**：v10 P2-3（mcp-tokens 文档纯文本）、v9 P2-3（finance-report CSV 未折算列）维持口径，本轮不改。

## 二、问题与修复

| 级别 | 问题 | 处置 |
| --- | --- | --- |
| P1 | 无 | — |
| P2-1 | /customer/conversations/:id 375 视口 antd Descriptions `span={2}` 与响应式 `column={{xs:1}}` 冲突产生 console error | 已修：5 处改 `span={{ xs: 1, sm: 2 }}`（`admin/src/pages/Customer/ConversationDetail/index.tsx`），修后 375 视口 0 console error / 0 溢出 |

## 三、门禁

`pnpm check:ui-copy --strict`、`pnpm test:frontend`（365）、`pnpm test:contracts`（17）、`pnpm build:admin`、`go vet` / `gofmt -l` 空 / `go test ./...` / `go build ./...` 全通过。Actions CI 不作依据；录屏/截图证据留档不入库。

## 四、收尾

- 临时 view-only 账号与会话 external_conversation_id 临时 fixture 已还原/清理，数据库无残留。
- 审计脚本为临时工具不入库。
