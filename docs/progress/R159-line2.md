# R159 线2：生产升级演练季度复跑（devops-engineer）

日期：2026-08-07。R149 演练（#304）后季度复跑：R155 时代基线（R149 时点 main `7f5645c1`）升级到最新 main。演练期间 #312 合入 main，目标版本从 `6a64eb39` 顺延为 `32a9aaea`（无新增表结构迁移，仅行为变更），两个版本均完成升级验证。证据（部署日志/指纹前后/功能实测输出）外置不入库。

## 1. 范围核对（与任务口径的差异）

- 任务口径中的「大屏卡片配置、汇率 seed」（#318）与「`--pre-upgrade-check` -w 新报错口径」（#317）截至演练时均未合入 main，登记为不在本轮迁移面，待合入后下轮补验。
- 实际迁移面（R152，#308/#309）：`mcp_api_tokens.purpose` 列、`customer_reply_template_variants` 新表、`buyer_message_drafts.language/lang_source` 列；R153/R154（#311/#312）无表结构迁移，仅行为变更（`TRUSTED_PROXIES`、入口级审计、分页 400、租户禁用 token 失效、审计 fail-closed）。

## 2. 部署与升级

- 从零部署（production compose + Caddy 内部 CA）：旧版本含全量镜像构建约 6 分钟内完成；最新 main 空库部署到登录实测 165s；含镜像构建的升级部署 464s（`6a64eb39`），#312 复跑 246s（`32a9aaea`）。均满足 <15 分钟目标。
- 生产 fail-fast 实测：`.env` 缺 `BACKUP_ENABLED=true` 时 backend 反复重启 `CONFIG_REQUIRED: BACKUP_ENABLED=true is required in production`，补 `.env.prod.example` 备份段后通过——生产 `.env` 必须包含备份配置段，`.env.prod.example` 与实际行为一致。

## 3. 基线与迁移验证

- 基线（`7f5645c1`）：双业务租户 2 万订单/4 万订单行/6 万库存流水/2 万自动化执行日志（半数 shop_id NULL）/4000 SKU/4000 回款 + 存量 MCP token 5 枚（无 purpose 列时代）+ 存量话术模板 + 跨租户重复订单号样本。
- `--pre-upgrade-check`：备份 + 同租户重复订单号预检 0 行通过；跨租户重复样本不误报。报错口径实测：`BACKUP_DIR` 不可创建 →「无法创建备份目录…（可用 BACKUP_DIR=路径 覆盖）」；目录存在但不可写 → 裸 `pg_dump 备份失败`（误导性，#317 -w 检查在途收口，本轮不重复修）。
- 升级后 AutoMigrate：purpose 列（存量 5 行回填 `mcp`）、变体新表、drafts 语言列全部落地；`order_automation_logs.shop_id` 回填补齐 1 万 NULL 行且与 orders 逐行 0 偏差；业务指纹（订单金额/订单行/库存流水/SKU 库存/回款/token hash/模板）逐项 0 差异（唯一变化为 shop_id 回填，预期内）。

## 4. 升级后功能实测

- purpose 各口径：存量 `mcp` token MCP initialize 可用、调 `/api/open/v1/*` 401；新建 `openapi` token 开放 API 可用、调 MCP 401；`both` 两侧可用；token 列表正确显示 purpose。
- 开放 API：orders/reports summary 数值与租户指纹一致；限流 RPS5/burst10 第 11 个请求起 429；租户隔离（tenant-b token 仅见本租户订单）；审计落 `openapi:orders_list`/`openapi:reports_summary` 行；R154 非法 `pageSize=0` 返回 400；`OPENAPI_ENABLED=false` 入口 404。
- 多语言模板：存量模板加 en/es 变体（PUT variants 全量替换）创建/落表/读取正常；非法语言码返回可选值清单。
- `TRUSTED_PROXIES`/XFF（#311/#312 行为 + #316 配置面）：配置 compose 网段（`172.18.0.0/16`）时 client_ip 为真实客户端且外部伪造 `X-Forwarded-For` 无效（Caddy 丢弃不可信 XFF）；留空时 client_ip 退化为 admin 容器 TCP peer（所有外部客户端共享预算）——与 `.env.prod.example`/`docs/production-deployment.md`/`docs/env.md` 口径逐条一致。注：`6a64eb39` 时点 backend 尚无该变量支持（#312 未合入），当时留空/配置行为相同；#312 合入后语义即与文档一致，故不构成失实。

## 5. 备份→恢复闭环

`stop backend` → `docker exec -i` + `pg_restore --clean --if-exists` 恢复升级前备份 → 指纹与升级前逐项一致（`customer_reply_template_variants` 新表残留符合既有「新表残留」口径）→ 重启 backend 迁移幂等重跑 → 指纹与升级后一致、purpose 列/回填重新落地。

## 6. 文档核对结论

- P0/P1：未发现（#316 配置面语义在 #312 合入后与实际行为一致；`docs/env.md` 已含 `TRUSTED_PROXIES` 条目；升级指南迁移点表 R152–R154 行已由 #312 补齐）。
- P2 清单：① `--pre-upgrade-check` 目录存在但不可写时报裸 `pg_dump 备份失败`（#317 在途）；② #318 未合入导致大屏折算/自定义指标/汇率 seed 不在本轮验证面（合入后下轮补验）；③ 恢复升级前备份会同时回退升级后新建的 token/变体等增量数据（演练环境预期行为，生产恢复窗口需按此口径知会）。
