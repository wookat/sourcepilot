# R184 线2：生产升级演练季度复跑（devops-engineer + qa-engineer）

- 日期：2026-08-08
- 角色：devops-engineer + qa-engineer
- 距上次升级演练：R178（6 轮，期间 MCP 写白名单 W1–W3、advisory lock、治理 UI、#367 审计修复合入）

## 口径

- **合并状态权威核实**（GitHub PR API）：#360/#361/#362/#363/#364/#365/#366/#367/#368 **全部已合入 main**（末次合并 #368 → `135f2c5e`）。无需叠加分支，本轮全程以 **main@135f2c5e** 为演练对象。
- 老栈基线取 `ab0c9b36`（#360 合入前的 main，即 MCP 写系列全部落地之前），在独立 Compose 项目中从零部署后灌入 R178 同构双租户基线数据，再原地升级到 main。
- 证据（部署日志、指纹文件、场景矩阵 JSON、三角色截图、恢复日志）作为会话附件提供，不入库；Actions CI 不作依据。

## 1. 从零部署计时（production compose + Caddy）

- 全新 checkout（main）+ 随机 `.env` + `MCP_WRITE_ENABLED=true`，`./scripts/deploy-prod.sh --no-pull`：**3m52s**（冷构建），退出码 0。
- postgres/redis/collector/backend/admin/caddy 六服务全部 healthy；Caddy HTTPS `/health-backend` 200（appEnv=production，DB/Redis ok）；bootstrap 管理员登录 200。
- 同栈原地重跑：**2.8s**，全部服务保持 healthy（与 R178 的 4m45s / 14.8s 同量级）。

## 2. 存量升级：R178 双租户基线 → 最新栈

- 老栈（`ab0c9b36`）独立项目部署（`COMPOSE_PROJECT_NAME=trademind-drill`，18.9s，热层缓存）；灌入 R178 同构基线：租户 2/3、3 店铺、20,000 订单（含 DUP-1..5 跨租户重复单号）、40,000 订单行、60,000 库存流水、19,982 自动化日志（每租户 4,996 条 shop_id NULL）、4,000 SKU、3,990 回款、24 采购单、5 个存量 MCP token（mcp/openapi/both 用途混布）、每租户 1 条加密 settings。
- `--pre-upgrade-check`：全量备份 + 同租户重复单号预检通过（跨租户 DUP 样本不受影响，符合 R95 索引口径）。
- checkout main → `deploy-prod.sh --no-pull`：**18.9s**，AutoMigrate 全部落地：
  - 新表 `mcp_write_confirmations`（confirm_hash 唯一索引、expires/consumed/executed 列齐全）；
  - `mcp_tool_call_logs` 新列 `mode / params_summary / result_summary / confirm_hash / amount`；
  - `mcp_api_tokens.scope`（readonly 默认）沿用。
- **数值指纹对比：业务表 0 差异**（orders / order_items / inventory_change_logs / product_skus / finance_payment_records / purchase_orders / mcp_api_tokens / shops / admin_users 的 count/sum/md5 完全一致）。仅两处确定性回填符合预期：
  - `order_automation_logs.shop_id` NULL 全部回填（回填后与订单表 shop_id 逐行比对 0 不一致、0 残留 NULL）；
  - 每租户新增 3 条 `report_currency` 默认 settings。

## 3. 升级后实测

- **MCP 写全链 25 项场景矩阵全部通过**：W1 打标 dry_run→确认 token→execute（DB 0→1）→重放 alreadyExecuted 不重复→参数漂移拒绝→无 token execute 拒绝；readonly scope 看不到写工具；租户开关关（租户 B）dry_run 拒绝；W2 exceptions_mark / mark-placed 状态机走通（DB 复核 mark_type=handled、placing→placed+外部单号）；W3 mark-paid 三前提四路径（金额差 0.01 拒绝、币种不符拒绝、精确一致 placed→paid、重放幂等、超单笔 150>100 拒绝、日累计 49+64.5>100 拒绝且目标单不变）；审计逐行落库（write 工具无 mode 空行——#367 修复无回退；execute 行带 amount）。
- **advisory lock 并发限额**：同一确认 token 6 并发 execute → 恰好 1 次 applied + 5 次 alreadyExecuted（DB 1 行）；8 并发不同订单 → 8 次成功且 quotaRemainingToken 严格唯一递减（无竞态重复扣减）；token 小时配额 30 打满后继续 dry_run 被「本小时写执行次数已达上限」拒绝。
- **治理 UI 三角色**（Playwright `/settings/mcp-tokens` 截图）：admin 可见「MCP 写白名单」卡片；operator/readonly 不可见（与 R182 一致）。
- **权限矩阵**：view-only 店铺写 40303、未授权店铺写 404（无存在性泄露）、readonly 写 40301、operator 管 settings 40305 / 管用户 40306、operator 建写 token 40301、非法 scope 40001、跨租户店铺/采购单一律 404；admin 正常写 0。

## 4. 备份 → 恢复 → 幂等重跑闭环

- 升级后再次 `--pre-upgrade-check` 全量备份；插入标记行后 `docker exec -i pg_restore --clean --if-exists` 恢复：退出码 0，标记行消失，**恢复后指纹与备份时点 0 差异**；backend 重启 AutoMigrate 幂等重跑无错误，health 200。
- `deploy-prod.sh --no-pull` 终态重跑：**2.9s**，六服务 healthy，指纹再次 0 差异。

## 缺陷与登记

- **P0/P1：无**。
- **P2（本轮新增登记）**：`docker-compose.prod.yml` 顶层硬编码 `name: trademind-prod`，同机第二个 checkout 直接部署会静默复用同名卷（本轮演练首次尝试即因此污染老栈数据库，重置后以 `COMPOSE_PROJECT_NAME` 覆盖解决——该覆盖有效）。建议在 `docs/production-deployment.md` 增加「同机多栈需显式 `COMPOSE_PROJECT_NAME`」警示。
- **口径说明**：存量 5 个 token 的明文未在演练中留存，升级后「存量 token 可用」以 `mcp_api_tokens`（含 token_hash）指纹 0 差异 + 同一校验路径新建 token 实测通过间接证明。
- 继承 P2（R183：非 PostgreSQL advisory lock 进程内、只读工具兜底审计时序、审计读接口权限位、pnpm audit 构建链 16 项）无变化。

## 收尾

- 演练全程在一次性隔离栈（trademind-prod / trademind-drill 两个独立 Compose 项目）内完成，结束后销毁，不触真实生产数据；`.env` 随机生成不入库。
