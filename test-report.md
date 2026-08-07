# PR #317 fix/round156-misc 测试报告

环境：本机 docker-compose.full.yml 全栈（Admin http://127.0.0.1:8000，后端 :8080，MCP/OPENAPI enabled），登录 demo_admin@trademind.local。

## 结果总览

| # | 用例 | 结果 |
|---|------|------|
| T1 | UI 创建 MCP token（e2e-r156-mcp，MCP + 开放 API）并在审计日志中看到调用记录 | ✅ 通过 |
| T2a | 审计表重命名后 MCP tools/call 返回 JSON-RPC error code **-32603**（非 0） | ✅ 通过 |
| T2b | 同场景开放 API /api/open/v1/orders 返回 HTTP **500 / code 50000 / data:null** | ✅ 通过 |
| T2c | 表改回后两接口恢复正常（MCP result / 开放 API 200 code 0） | ✅ 通过 |
| T3a | 非 root、BACKUP_DIR=/var/backups：预检立即清晰报错并提示 BACKUP_DIR 覆盖 | ✅ 通过 |
| T3b | 目录存在但不可写（chmod 555）：第二分支「不可写」报错 | ✅ 通过 |
| T4 | Regression：demo 订单列表 + 订单详情正常 | ✅ 通过（需先重新 seed，见备注） |

备注：接手时 orders 表为空（demo 数据不在），执行 `DB_HOST=127.0.0.1 pnpm seed:demo:full` 后回归通过。收尾已在 UI 吊销 e2e-r156-mcp（clean 不覆盖非 DEMO- token，已按要求处理）。

## T1 UI 创建 token & 审计日志

![Token 创建成功，明文仅展示一次](https://app.devin.ai/attachments/fcdf8fd7-a465-421e-9090-caae02222753/ss_658584fd.png)

![审计日志出现 e2e-r156-mcp 的 orders_query / openapi:orders_list 成功记录](https://app.devin.ai/attachments/3bfc3748-36fa-407e-bb8d-0aa0948a59d2/ss_7999415f.png)

## T2 审计库故障 fail-closed（shell 证据）

```
$ docker exec ... psql -c 'ALTER TABLE mcp_tool_call_logs RENAME TO mcp_tool_call_logs_bak;'
--- MCP tools/call with broken audit:
{"jsonrpc":"2.0","id":3,"error":{"code":-32603,"message":"audit log unavailable, tool call rejected"}}
--- open API /orders with broken audit:
{"code":50000,"message":"audit log unavailable, request rejected","data":null,"traceId":"..."}
HTTP_STATUS=500

$ ... RENAME back ...
--- MCP after restore:
{"jsonrpc":"2.0","id":4,"result":{..."structuredContent":{"items":[],"total":0}}}
--- openapi after restore:
{"code":0,"message":"ok","data":{"list":[],"total":0},...}  HTTP_STATUS=200
```

被拒调用不产生审计行（fail-closed 语义一致）；恢复后调用正常且审计行可见。

## T3 deploy-prod.sh --pre-upgrade-check（shell 证据，临时 harness /tmp/dp + docker shim 绕过前置项）

```
$ BACKUP_DIR=/var/backups bash scripts/deploy-prod.sh --pre-upgrade-check   # 非 root
[deploy][失败] 无法创建备份目录 /var/backups（当前用户 ubuntu）：请以 root 运行，或用 BACKUP_DIR=<可写目录> 覆盖  (exit 1)

$ BACKUP_DIR=/tmp/dp/ro (chmod 555) ...
[deploy][失败] 备份目录 /tmp/dp/ro 不可写（当前用户 ubuntu）：请以 root 运行，或用 BACKUP_DIR=<可写目录> 覆盖  (exit 1)
```

## T4 Regression：demo golden path

| 🟢 订单列表（DEMO- 订单） | 🟢 订单详情 DEMO-FIN-2110 |
|---|---|
| ![订单列表](https://app.devin.ai/attachments/2253e260-fdfd-4718-990e-7e3c6cdcd830/ss_e74860b0.png) | ![订单详情](https://app.devin.ai/attachments/c2e871da-14c9-4841-a666-87283a792cff/ss_9f218c1f.png) |

## 收尾

![e2e-r156-mcp 已吊销](https://app.devin.ai/attachments/1e8e2f3d-d49d-4c69-a4ca-49c51860b683/ss_10984b61.png)

审计表已确认恢复原名 `mcp_tool_call_logs`；/tmp/dp 为一次性临时目录，未改动仓库代码。
