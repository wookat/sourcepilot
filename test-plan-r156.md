# PR #317 fix/round156-misc 测试计划

环境：docker-compose.full.yml 全栈已运行（Admin http://127.0.0.1:8000，后端 :8080，MCP/OPENAPI enabled）。登录 demo_admin@trademind.local / DemoAdmin123!。

## T1（UI，录屏）MCP 只读接入页可建 token 且审计日志可见
1. 登录 Admin，导航 /settings/mcp-tokens（侧栏「只读 API 接入」）。
2. 创建 token：名称 `e2e-r156-mcp`，用途「MCP + 开放 API」（both），记下明文 token。
   - PASS: 弹出/展示明文 token，列表出现该行。
3. shell 用该 token 调 `POST /api/mcp` tools/call（如 orders_list）→ 返回 result 非 error。
   - PASS: JSON-RPC 响应含 result，无 error。
4. 回 UI 刷新审计卡片。
   - PASS: 审计列表出现 tool 调用记录（token 名 e2e-r156-mcp，状态 success）。

## T2（shell）审计库故障 fail-closed：MCP 返回 -32603、开放 API 返回 500/50000
1. `ALTER TABLE mcp_tool_call_logs RENAME TO mcp_tool_call_logs_bak;`
2. 同一 token 再调 /api/mcp tools/call。
   - PASS: JSON-RPC error，`"code":-32603`，message 含 "audit log unavailable"；FAIL if code 0 或返回 result。
3. `curl -H "Authorization: Bearer <tok>" localhost:8080/api/open/v1/orders`。
   - PASS: HTTP 500，body `code:50000`，`data:null`（无业务数据）。
4. 表改回原名；重调两接口恢复正常（MCP 有 result；open API 200 code 0）。
   - PASS: 恢复正常，且 UI 审计日志可见 error/success 记录。

## T3（shell）deploy-prod.sh --pre-upgrade-check 备份目录检查
1. 非 root：`BACKUP_DIR=/var/backups bash scripts/deploy-prod.sh --pre-upgrade-check`（在有 .env 的目录或临时准备）。
   - PASS: 立即失败，错误信息含「无法创建备份目录 /var/backups」或「不可写」、当前用户名、以及「BACKUP_DIR=<可写目录> 覆盖」提示；不会跑到 pg_dump 报模糊错。
2. 对已存在但只读目录（chmod 555 临时目录）验证第二分支「不可写」报错。

## T4（UI，录屏，Regression）demo golden path 快速回归
- 订单列表 /orders 正常加载 DEMO- 订单；随便打开一个订单详情正常渲染。

## 收尾
- 在 UI 吊销/删除 `e2e-r156-mcp` token；确认表已恢复原名。
