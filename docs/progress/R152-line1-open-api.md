# Round 152 线1：对外开放 REST API（只读起步）

竞品复评 v7 路线①第一杠杆（店小秘/马帮均有开放 API 体系）。

## 内容

1. **开放 API 只读入口** `GET /api/open/v1/*`（`backend/internal/modules/openapi`）：订单列表/详情、库存查询、经营摘要、异常待办 5 个 GET 端点，仅注册 GET，无任何写操作。`OPENAPI_ENABLED`/`OPENAPI_RATE_RPS`/`OPENAPI_RATE_BURST` 配置。
2. **token 治理复用**：不另建 token 体系，`mcp_api_tokens` 新增 `purpose` 字段（`mcp` 默认/`openapi`/`both`）做入口选择器，`scope` 保持单一 `readonly` 轴；存量 token 视为 MCP 用途，不能访问新入口（攻击面不变宽）。哈希校验/过期/吊销/每租户 20 上限/三层限流（token/租户聚合/IP 鉴权失败预算，桶与 MCP 独立）/逐次审计（工具名 `openapi:<endpoint>`，不记参数与结果）全部沿 MCP 口径。
3. **共享只读查询层** `backend/internal/modules/readonlyquery`：MCP 4 工具与开放 API 5 端点共用同一查询实现与 DTO（防两面漂移），新增订单详情（按订单号，跨租户与不存在统一 404）。输出脱敏：客户名仅首字符，无内部 UUID/邮箱/电话/rawData/凭证字段。
4. **管理页**：token 创建可选用途（MCP/开放 API/两者），列表新增用途列（`McpTokens.tsx`、`mcpTokens.ts`）。
5. **契约**：OpenAPI 3 规范 `docs/openapi/open-api.v1.json`；Go 契约测试 `spec_test.go`（spec 路由 ↔ 实现路由、schema ↔ DTO 字段双向同步校验）；`tests/contracts/api-contracts.json` 登记 5 端点 + purpose 字段。权限矩阵登记 5 路由（`probe:false`）。demo seed 补开放 API 演示 token（`fulldemo_round147.go`）。
6. **文档**：新增 `docs/open-api.md`（curl 示例/错误码表/限流说明）；同步 `docs/api.md`、`docs/mcp.md`、`docs/env.md`、`docs/permission-matrix.md`、`docs/module-map.md`、`.env.example`、`.env.docker.example`。

## 验证

`gofmt`/`go vet`/`go test ./...`（openapi/mcptoken/mcpserver/demoseed/permmatrix）、`pnpm test:contracts`、`pnpm test:frontend`、`pnpm build:admin`；Docker 双租户实测（跨租户 401/404、readonly 语义、限流 429）证据不入库。
