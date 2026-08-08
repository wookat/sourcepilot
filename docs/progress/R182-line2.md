# R182 线2：全站大回归 v34（QA 报告）

- 日期：2026-08-08
- 角色：qa-engineer（company-os）
- 范围：最新 `main` + 全部 OPEN PR 按 ancestry 叠加后的集成分支全站回归 + Docker 全栈实测

## 一、PR 权威状态核实（GitHub API）

| PR | 状态 | 结论 |
| --- | --- | --- |
| #346–#359 | closed / merged | 已合入 main，跳过（与 R180 v33 结论一致：#360 相关系列在合入后 diff 为空并已关闭） |
| #360 | OPEN | MCP 写 W1（R179） |
| #361 | OPEN | MCP 写 W2（R180，基于 #360） |
| #362 | OPEN | R180 线2 回归报告（docs） |
| #363 | OPEN | MCP 写 W3 mark-paid + 安全审计（R181，基于 #361） |
| #364 | OPEN | R181 线2 验收包 / DEMO_SCRIPT（docs，基于 #361） |

## 二、集成方式与合并顺序终版

按 ancestry 依赖叠加（本轮集成分支即按此顺序 merge，全程仅 `docs/PROGRESS.md` 文书性冲突，无代码冲突；`git cherry` 验证全部 5 个 PR 提交均已包含）：

```
1. merge #360（W1 基础管道：scope/三层闸门/确认token/审计/限额）
2. merge #361（W2 三动作，依赖 #360）
3. merge #363（W3 mark-paid + 安全审计，依赖 #361）
4. merge #362（R180 线2 报告，docs）
5. merge #364（R181 线2 验收包，docs，依赖 #361）
```

说明：任务背景中提到的 #350/#357/#358 经权威核实均已 merged，无需叠加。

## 三、全套门禁（本地实跑，Actions CI 不作依据）

| 门禁 | 结果 |
| --- | --- |
| gofmt -l / go vet | 通过（无输出） |
| Go 全量 `go test ./...`（隔离 test 库/Redis DB15） | 通过，103 packages ok |
| securitytests + permmatrix（`go test -v ./internal/securitytests/...`） | 通过，113/113 |
| `pnpm check:dev` / `pnpm check:ui-copy --strict` | 通过 |
| `pnpm test:frontend` | 通过，372 tests / 57 files |
| `pnpm test:contracts` | 通过，17/17 |
| `pnpm test:collector` | 通过，18/18 |
| `pnpm build:admin` / `pnpm build:collector` / `go build ./...` | 通过 |
| `pnpm test:e2e` 全量 | 通过，364 passed / 3 skipped |

## 四、Docker 全栈实测（docker-compose.full.yml 重建镜像 + demo seed）

镜像基于集成分支重建（backend/admin/collector），`MCP_WRITE_ENABLED=true`，`pnpm seed:demo:full` 双租户种子。API 级场景矩阵脚本共 **48 项全部 PASS**（证据 `docker-scenarios.json`），另有三角色治理 UI Playwright 实测：

1. **R57 主链路**：订单列表/详情、DEMO-AT-1004 标记付款 → 自动化规则 `generate_procurement` 成功触发并落执行日志、经营统计正常。
2. **MCP 写全链五动作**：`orders_add_tag`（dry_run→确认token→execute applied=1→DB 落库 0→1→重放 alreadyExecuted 不重复变更）、`exceptions_mark`（handle 落 mark 行）、`procurement_mark_placed`（placing→placed）、`procurement_fill_logistics`（paid→shipped + 物流行）、`procurement_mark_paid`（见下）。
3. **mark-paid 三前提四路径**：① 未配置上限拒绝；② 金额 +0.01 / 币种不符拒绝（分级精确比较）；③ 前提齐备成功 placed→paid（dry_run 预览回显金额/两项上限/当日已用/明细行），确认 token 重放 alreadyExecuted；④ 超单笔上限拒绝、超日累计上限拒绝（64.50 已用 + 59.50 > 100），被拒目标状态不变。
4. **闸门逐层**：租户开关默认关时 dry_run 拒绝；readonly scope token 看不到写工具；write:ops token 恰好 6 个写工具；写 token 仅 admin 可建（operator/readonly 40301，非法 scope 40001）；非 admin 列表不可见写 token、吊销返回 404。
5. **审计落库**：`mcp_tool_call_logs` 含 dry_run/execute/拒绝行、confirmHash、paramsSummary；成功 execute 的 mark-paid 行 `amount` 落库。
6. **限额**：execute 响应回显 quotaRemainingToken/Tenant（30/小时、200/天递减）；超限拒绝逻辑由 securitytests（TestWriteQuotasFailClosed 等）覆盖。
7. **治理 UI 三角色**（Playwright 实测 `/settings/mcp-tokens`）：admin 可见「MCP 写白名单」卡片，operator/readonly 完全不可见（截图 ui-mcp-*.png）。
8. **settings 注册表/惰性收编**：敏感项写入即密（`is_encrypted=t`、GET 脱敏 `sk-****c123`）；预置明文行读取后惰性收编为密文且展示脱敏。
9. **view-only 40303**：临时 view 授权账号读 200 / 写 40303（用后硬删除）；readonly 写 40301；operator 对不可见店铺订单写 404（无存在性泄露）。
10. **跨租户 404**：t2 admin 读 t1 订单 404；t2 订单/店铺列表零 t1 数据。
11. **双租户零残留**：`seed:demo:full:clean` + `verify` 输出 `zero DEMO- residual rows`（全表 0）；QA 自建 token/settings/审计行已一并清除。

## 五、P0 / P1 / P2

- **P0：无**。
- **P1：无**（门禁与全栈场景无失败，无需修复）。
- **P2 清单（沿袭 + 本轮观察，不阻塞合并）**：
  1. （沿袭 SECURITY_AUDIT_R181）PostgreSQL READ COMMITTED 下日累计限额理论并发窗口，建议后续 advisory lock。
  2. （沿袭 SECURITY_AUDIT_R181）审计 `amount` 展示语义建议在治理 UI 标注为「登记金额」。
  3. MCP 端点限流（MCP_RATE_RPS=5）对连续脚本化调用会返回空体 429，客户端需重试；建议文档标注或返回 JSON-RPC 错误体。
  4. `seed:demo:full:clean` 保留 demo_* 登录账号（无业务数据残留，按既有设计）；如需完全清除需手工删除。

## 六、证据

证据不入库，作为会话附件提供：`/home/ubuntu/evidence/r182/`（gofmt/go-test/securitytests/test-frontend/contracts/collector/build/e2e 输出、docker-scenarios.json 48 项矩阵、ui-mcp-{admin,operator,readonly}.png、seed/clean/verify 输出）。

## 七、结论与建议

集成分支（#360→#361→#363→#362→#364 叠加）全部门禁与 Docker 全栈实测通过，无 P0/P1。建议按第二节顺序合并（PR 不由本会话 merge）。
