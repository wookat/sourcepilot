# 模块关联索引

本文件用于帮助开发者和 AI Agent 判断“改一个点时还要检查哪些关联内容”。遇到不确定的改动，先查本表，再读取对应代码和文档。

## 通用原则

- 改代码时同步检查配置、示例、Docker、CI、文档和前端展示。
- 改公共契约时同步检查调用方，不只改定义处。
- 涉及敏感字段时同步检查加密、脱敏、日志和 `SECURITY.md`。
- 涉及较大模块或阶段性能力时同步更新 `docs/PROGRESS.md`。

## 关联检查表

| 改动类型 | 必须检查 / 同步 |
| --- | --- |
| 环境变量 | `.env.example`、`.env.docker.example`、`docker-compose.yml`、`docker-compose.full.yml`、`docs/env.md`、`docs/development.md`、`docs/docker-deployment.md` |
| 启动命令 / pnpm 脚本 | `package.json`、`README.md`、`README.en.md`、`docs/development.md`、`.github/workflows/*.yml` |
| Docker 部署 | `docker-compose.full.yml`、服务 Dockerfile、`.env.docker.example`、`docs/docker-deployment.md`、`.github/workflows/docker.yml` |
| 后端 API | `backend/internal/api`、对应 handler/service/dto、`docs/api.md`、`admin/src/services`、`admin/src/types`、相关页面 |
| 统一返回 / 错误码 | `backend/internal/pkg/response`、所有调用方、`docs/api.md`、前端错误处理 |
| 管理端页面 | `admin/config/routes.ts`、`admin/src/pages`、`admin/src/services`、`admin/src/types`、README 能力描述、相关 docs |
| 数据库模型 / 自动迁移 | `backend/internal/modules/**/model`、`backend/internal/database`、`docs/architecture.md`、`docs/PROGRESS.md` |
| 异步任务 / 队列 | 任务 model/service/worker、Redis 配置、健康检查、`.env.example`、`.env.docker.example`、`docs/env.md`、任务中心页面 |
| AI Provider | `backend/internal/providers`、AI settings、Prompt 模板、调用记录、`docs/provider.md`、`docs/provider-template.md` |
| Storage Provider | Provider 接口、文件上传 API、settings.storage、本地/对象存储文档、`docs/provider.md` |
| Image Provider | 图片任务、队列、settings.image、任务页面、`docs/provider.md` |
| Platform Provider | 店铺授权、Token 加密、平台配置、订单/库存/客服调用方、`docs/provider.md`、`SECURITY.md` |
| 多平台 / 批量刊登 | `backend/internal/modules/productpublish`、`docs/MULTI_PLATFORM_PUBLISHING_DESIGN.md`、`docs/PUBLISH_BATCH_MIGRATION.md`、`docs/api.md`（batch-targets / batches）、`admin/src/pages/Product/PublishBatch*`、`admin/src/pages/Product/PublishTasks`、`admin/src/constants/publishLabels.ts`、`admin/src/constants/publishLimits.ts`、`scripts/publish-batch-perf.ps1`、第二平台调研 `docs/platform-integration.md`（TikTok Shop / Shopee 开放平台 API 与开发者账号申请） |
| 违禁词合规检测（round109） | `backend/internal/modules/bannedwords`（model/presets/detector/service/handler/router）、`backend/internal/modules/productcheck/service.go`（readiness `compliance` 分组）、`backend/internal/api/router.go`、`backend/internal/database/migrate.go`、`backend/internal/modules/demoseed/fulldemo.go`、`backend/internal/securitytests/permmatrix/matrix.json`、`tests/contracts/api-contracts.json`、`admin/src/services/bannedWords.ts`、`admin/src/pages/Settings/BannedWords.tsx`、`admin/src/components/BannedWordsCheckPanel`、`admin/src/pages/Product/DraftDetail/index.tsx`（readiness 页合规面板）、`admin/src/pages/Product/Drafts/index.tsx`（批量检测抽屉）、`admin/config/routes.ts`、`admin/e2e/mocks/banned-words.ts`、`admin/e2e/specs/round109-banned-words.spec.ts`、`docs/api.md` |
| 物流商 / 运单 / 打单发货（round91） | `backend/internal/modules/carrier`、`backend/internal/modules/order`（shipment/batch/print/tracking）、`backend/internal/providers/tracking`（TrackingProvider，manual）、`backend/internal/database/migrate.go`、`backend/internal/securitytests/permmatrix/matrix.json`、`tests/contracts/api-contracts.json`、`admin/src/services/carriers.ts`、`admin/src/services/orders.ts`、`admin/src/components/CarrierSelect.tsx`、`admin/src/pages/Settings/Carriers.tsx`、`admin/src/pages/Orders/PrintSheets.tsx`、`admin/src/pages/Orders/BatchShipModal.tsx`、`docs/api.md`、`docs/permission-matrix.md`、demo seed（`backend/internal/modules/demoseed`） |
| 审单规则 / 审单工作台（round114） | `backend/internal/modules/order`（review_model/review_engine/review_rules/review_workbench/review_handler/router、`Order.reviewStatus`、创建动线跑规则、发货前审单校验）、`backend/internal/modules/procurement/service.go`（`review.blocked` blocker）、`backend/internal/database/migrate.go`、`backend/internal/securitytests/permmatrix/matrix.json`、`tests/contracts/api-contracts.json`、`admin/src/services/orderReview.ts`、`admin/src/pages/Settings/OrderReviewRules.tsx`、`admin/src/pages/Orders/Review`、`admin/src/pages/Orders/Exceptions`（固定拦截文案区分）、`admin/config/routes.ts`、`admin/e2e/mocks/order-review.ts`、`admin/e2e/specs/round114-order-review.spec.ts`、demo seed（`backend/internal/modules/demoseed/fulldemo_round114.go`）、`docs/api.md` |
| 财务对账（round121，回款/费用/实算毛利/对账报表） | `backend/internal/modules/finance`（model/service/handler/router/profit/report）、`backend/internal/modules/procurement`（采购项 `actual_price` 实付价）、`backend/internal/modules/migrationimport`（`kind=payment` 导入/导出）、`backend/internal/api/router.go`、`backend/internal/database/migrate.go`、`backend/internal/securitytests/permmatrix/matrix.json`、`tests/contracts/api-contracts.json`、`admin/src/services/finance.ts`、`admin/src/pages/Finance`（Payments/Reconciliation/Report）、`admin/src/pages/Orders/Detail/FinancePanel.tsx`、`admin/config/routes.ts`、`admin/e2e/mocks/finance.ts`、`admin/e2e/specs/round121-finance.spec.ts`、demo seed（`backend/internal/modules/demoseed/fulldemo_round121.go`）、`docs/api.md` |
| 迁移导入（round92，店小秘/马帮） | `backend/internal/modules/migrationimport`、`backend/internal/modules/order`（CreateBody `remark`/`rawData`）、`backend/internal/database/migrate.go`、`backend/internal/securitytests/permmatrix/matrix.json`、`tests/contracts/api-contracts.json`、`admin/src/pages/Settings/Migration`、`admin/src/services/imports.ts`、`docs/api.md`、`docs/permission-matrix.md`、`docs/migration-guide.md` |
| 报表多币种 / 本位币折算（round93/94/97） | `backend/internal/providers/fxrate`、settings 分组 `report_currency`（按租户隔离）、`backend/internal/modules/stats`、成本/毛利估算（procurement cost-estimates）、`admin/src/pages/Settings/ReportCurrency`、报表/首页折算展示、`docs/api.md`、`docs/permission-matrix.md`、`docs/provider.md` |
| 平台租户治理 / 清退（round82/89） | `backend/internal/modules/platformtenant`、登录/会话租户状态校验（auth/session）、`backend/internal/securitytests/permmatrix/matrix.json`、`admin/src/pages/Settings/PlatformTenants`、`docs/api.md`、`docs/permission-matrix.md` |
| 版本升级 / 迁移预检（round95/99） | `backend/internal/database/migrate.go`（AutoMigrate 预检，如 `preflightOrderNoTenantUnique`）、`scripts/deploy-prod.sh`（`--pre-upgrade-check`）、`docs/upgrade-guide.md`、`docs/production-launch-checklist.md`、`docs/production-deployment.md` |
| Collector Provider | `collector/`、采集任务 API、队列、raw 原始数据、`docs/provider.md`、**1688 改解析时必读 [`docs/collector-1688-pitfalls.md`](collector-1688-pitfalls.md)** |
| 安全 / 密钥 / Token | 加密、脱敏、日志、环境模板、`SECURITY.md`、相关 settings 文档 |
| CI / 分支 / PR 流程 | `.github/workflows`、`docs/branching.md`、`CONTRIBUTING.md`、`.github/PULL_REQUEST_TEMPLATE.md` |
| 开源治理 | `README.md`、`README.en.md`、`docs/README.md`、`CHANGELOG.md`、`.github/*` |
| AI 工作流 / Agent 规则 | `AGENTS.md`、`docs/ai-workflow.md`、`docs/ai-coding-rules.md`、`docs/README.md`、`docs/cursor-rules-usage.md`、`.cursorrules`、`.cursor/rules/README.md`、必要时新增或更新 `.cursor/rules/*.mdc`、`CONTRIBUTING.md`、PR 模板 |

## 前后端联动

后端 API 或 DTO 变化时，必须检查：

- `admin/src/services/**` 的请求路径、方法、参数、响应字段。
- `admin/src/types/**` 或页面内类型定义。
- ProTable / ProForm 字段名、枚举、状态文案和空状态。
- `docs/api.md` 的接口契约。

## 配置联动

新增配置时优先判断配置归属：

- 部署级固定配置：进入 `.env.example`，Docker 需要时同步 `.env.docker.example`。
- 可变业务配置：优先进入 settings 表和后台设置页。
- 敏感业务配置：存库时必须 AES-GCM 加密，前端展示必须脱敏。

## 文档联动

- README 只保留首页重点和入口。
- 细节放入 `docs/`，并在 `docs/README.md` 增加入口。
- 新增 AI 规则或关联说明时，同步 `AGENTS.md`、`docs/ai-workflow.md` 和 `.cursor/rules/README.md`。
- 重复出现的坑、质量门槛或工具协作经验，应写回对应 pitfalls、模块文档、`docs/PROGRESS.md` 或 AI 规则，避免只停留在单次对话。
