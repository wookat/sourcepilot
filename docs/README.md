# TradeMind 文档中心

TradeMind 是一个聚焦 `AI 商品运营工具` 与 `多平台跨境 ERP MVP` 的开源平台。仓库首页负责展示项目定位与产品预览，`docs/` 负责承载使用、部署、开发、测试与安全细节。

如果你是第一次进入这个项目，建议先按角色找到入口，而不是从头顺着文件名阅读。

## 按角色开始

| 你现在想做什么 | 建议先看 |
| --- | --- |
| 我想快速了解项目能做什么 | [../README.md](../README.md) · [roadmap.md](roadmap.md) · [FULL_PROJECT_FUNCTION_MAP.md](FULL_PROJECT_FUNCTION_MAP.md) |
| 我想本地跑起来或用 Docker 试用 | [development.md](development.md) · [docker-deployment.md](docker-deployment.md) · [env.md](env.md) |
| 我想部署到云服务器 + 公网域名 + HTTPS | [production-deployment.md](production-deployment.md) |
| 我想按业务流程使用产品做运营 | [operations-manual.md](operations-manual.md) · [DEMO_SEEDING_GUIDE.md](DEMO_SEEDING_GUIDE.md) |
| 我想接 API、改功能、扩 Provider | [architecture.md](architecture.md) · [api.md](api.md) · [provider.md](provider.md) |
| 我想参与协作或用 AI 工具开发 | [../AGENTS.md](../AGENTS.md) · [ai-workflow.md](ai-workflow.md) · [module-map.md](module-map.md) |
| 我想了解当前进度与遗留问题 | [PROGRESS.md](PROGRESS.md) |

## 文档分层

| 层级 | 作用 |
| --- | --- |
| `README.md` / `README.en.md` | 对外首页：项目定位、能力概览、界面预览、快速开始。 |
| `docs/README.md` | 文档导航首页：按 使用 / 部署 / 开发 / 测试 / 安全 分类索引。 |
| `docs/*.md` | 详细规则与实现说明。 |
| `.cursor/rules/` 与 `AGENTS.md` | AI 协作规则入口，约束工程实践与文档同步。 |

## 使用（运营与演示）

| 文档 | 内容 |
| --- | --- |
| [operations-manual.md](operations-manual.md) | 日常运营操作手册（选品 → 上架 → 采购 → 发货，1688 人工下单过渡模式） |
| [DEMO_SEEDING_GUIDE.md](DEMO_SEEDING_GUIDE.md) | 演示数据一键 seed 与验证 |
| [DEMO_DATASET.md](DEMO_DATASET.md) / [FULL_PROJECT_DEMO_DATASET.md](FULL_PROJECT_DEMO_DATASET.md) | 演示数据集说明与覆盖范围 |
| [DEMO_SCRIPT.md](DEMO_SCRIPT.md) | 产品演示脚本 |
| [AI_IMAGE_WARNING_RECOVERY_GUIDE.md](AI_IMAGE_WARNING_RECOVERY_GUIDE.md) | AI 图片任务告警与恢复操作 |
| [custom-collect-rules.md](custom-collect-rules.md) | 自定义链接采集规则 JSON、API 与错误码 |

## 部署与运维

| 文档 | 内容 |
| --- | --- |
| [docker-deployment.md](docker-deployment.md) | `docker-compose.full.yml` 完整部署、端口、日志、数据卷 |
| [production-deployment.md](production-deployment.md) | 生产部署 SOP：Caddy HTTPS、一键部署、升级/回滚、备份恢复 |
| [env.md](env.md) | 环境变量清单、敏感配置、安全规则与同步要求 |
| [ENVIRONMENT_PROFILE_GUIDE.md](ENVIRONMENT_PROFILE_GUIDE.md) | 环境 profile 与配置组合 |
| [DEPLOYMENT_PRECHECK.md](DEPLOYMENT_PRECHECK.md) | 部署前检查清单 |
| [PRODUCTION_DOMAIN_HTTPS_GUIDE.md](PRODUCTION_DOMAIN_HTTPS_GUIDE.md) / [NGINX_PRODUCTION_GUIDE.md](NGINX_PRODUCTION_GUIDE.md) / [CORS_PRODUCTION_GUIDE.md](CORS_PRODUCTION_GUIDE.md) | 域名、HTTPS、Nginx 与 CORS 生产配置 |
| [SYSTEMD_DEPLOYMENT_GUIDE.md](SYSTEMD_DEPLOYMENT_GUIDE.md) | systemd 裸机部署 |
| [DOUYIN_PRODUCTION_RUNBOOK.md](DOUYIN_PRODUCTION_RUNBOOK.md) / [DOUYIN_ROLLBACK_RUNBOOK.md](DOUYIN_ROLLBACK_RUNBOOK.md) | 抖店上线与回滚 Runbook |
| [P6_BACKUP_ARCHITECTURE.md](P6_BACKUP_ARCHITECTURE.md) / [P6_RESTORE_ARCHITECTURE.md](P6_RESTORE_ARCHITECTURE.md) / [P6_RELEASE_ARCHITECTURE.md](P6_RELEASE_ARCHITECTURE.md) / [P6_DISASTER_RECOVERY_PLAN.md](P6_DISASTER_RECOVERY_PLAN.md) | 备份、恢复、发布与灾备设计及 Runbook 入口 |
| [SLO.md](SLO.md) | 服务目标与观测口径 |

## 开发与架构

| 文档 | 内容 |
| --- | --- |
| [development.md](development.md) | 本地开发环境、`pnpm dev`、分服务启动、调试与故障排查 |
| [architecture.md](architecture.md) | Go backend、React admin、Node collector、PostgreSQL、Redis 的整体关系 |
| [api.md](api.md) | `/api/v1` API 契约、统一返回、鉴权与前后端同步要求 |
| [migration-guide.md](migration-guide.md) | 店小秘 / 马帮迁移导入指南：向导流程、字段别名、状态映射与格式假设 |
| [provider.md](provider.md) / [provider-template.md](provider-template.md) | Provider 抽象、扩展建议与新增 Provider 模板 |
| [module-map.md](module-map.md) | 模块关联索引，避免代码、配置、文档、CI 漏同步 |
| [roadmap.md](roadmap.md) | 版本路线图与阶段目标 |
| [FULL_PROJECT_FUNCTION_MAP.md](FULL_PROJECT_FUNCTION_MAP.md) / [FULL_PROJECT_MVP_MAIN_FLOW.md](FULL_PROJECT_MVP_MAIN_FLOW.md) | 全项目功能地图与 MVP 主链路定义 |
| [collector-1688-pitfalls.md](collector-1688-pitfalls.md) / [collector-taobao-tmall-test-links.md](collector-taobao-tmall-test-links.md) | 采集器已知问题、防复发约束与测试链接 |
| 模块设计（`*_DESIGN.md`） | 各业务模块设计文档：工作台（[DASHBOARD_OVERVIEW_DESIGN.md](DASHBOARD_OVERVIEW_DESIGN.md)）、订单（[ORDER_CENTER_DESIGN.md](ORDER_CENTER_DESIGN.md)、[ORDER_EXCEPTION_WORKBENCH_DESIGN.md](ORDER_EXCEPTION_WORKBENCH_DESIGN.md)）、库存（[INVENTORY_CENTER_DESIGN.md](INVENTORY_CENTER_DESIGN.md)、[INVENTORY_CONSISTENCY_DESIGN.md](INVENTORY_CONSISTENCY_DESIGN.md)）、刊登（[MULTI_PLATFORM_PUBLISHING_DESIGN.md](MULTI_PLATFORM_PUBLISHING_DESIGN.md)、[PUBLISH_IDEMPOTENCY_DESIGN.md](PUBLISH_IDEMPOTENCY_DESIGN.md)）、客服（[CUSTOMER_SERVICE_CENTER_DESIGN.md](CUSTOMER_SERVICE_CENTER_DESIGN.md)、[CUSTOMER_AI_REPLY_SUGGESTION_DESIGN.md](CUSTOMER_AI_REPLY_SUGGESTION_DESIGN.md)）、批量 AI（[BATCH_AI_TEXT_OPERATION_DESIGN.md](BATCH_AI_TEXT_OPERATION_DESIGN.md)、[BATCH_AI_IMAGE_OPERATION_DESIGN.md](BATCH_AI_IMAGE_OPERATION_DESIGN.md)）、AI 工作台（[AI_OPERATION_WORKBENCH_DESIGN.md](AI_OPERATION_WORKBENCH_DESIGN.md)）等 |
| 可靠性设计 | [IDEMPOTENCY_DESIGN.md](IDEMPOTENCY_DESIGN.md) · [TASK_RELIABILITY_DESIGN.md](TASK_RELIABILITY_DESIGN.md) · [TASK_LEASE_AND_HEARTBEAT_DESIGN.md](TASK_LEASE_AND_HEARTBEAT_DESIGN.md) · [CONCURRENT_WRITE_SAFETY.md](CONCURRENT_WRITE_SAFETY.md) · [MULTI_INSTANCE_SAFETY.md](MULTI_INSTANCE_SAFETY.md) · [STALE_WORKER_PROTECTION.md](STALE_WORKER_PROTECTION.md) · [CIRCUIT_BREAKER_AND_RATE_LIMIT.md](CIRCUIT_BREAKER_AND_RATE_LIMIT.md) · [PROVIDER_RESILIENCE_DESIGN.md](PROVIDER_RESILIENCE_DESIGN.md) · [MIGRATION_LOCK_DESIGN.md](MIGRATION_LOCK_DESIGN.md) |
| 抖店对接（`DOUYIN_*.md`） | OAuth 与 Token 生命周期、订单/库存/客服/Webhook 适配器、商品草稿映射与幂等、错误分类、E2E 清单等 |
| [PROGRESS.md](PROGRESS.md) | 当前进度、已完成事项、遗留问题 |

## 协作与工程规则

| 文档 | 内容 |
| --- | --- |
| [branching.md](branching.md) | `main` / `dev` / `feat/*` / `fix/*` / `release/*` 分支策略与 PR 规则 |
| [ai-workflow.md](ai-workflow.md) | 跨 AI 工具通用工作流、提示词优化、上下文预算与经验沉淀 |
| [ai-coding-rules.md](ai-coding-rules.md) | AI 编程规则、配置文件与文档同步要求 |
| [task-checklist.md](task-checklist.md) | 按任务类型收尾自查：Go、Admin、Collector、环境变量、API、Provider、Docker、CI |
| [ui-copywriting.md](ui-copywriting.md) | 用户可见文案中文化、术语表与 `pnpm check:ui-copy` |
| [cursor-rules-usage.md](cursor-rules-usage.md) / [../.cursor/rules/README.md](../.cursor/rules/README.md) | Cursor rules 使用说明与索引 |
| [../AGENTS.md](../AGENTS.md) / [../CONTRIBUTING.md](../CONTRIBUTING.md) | 通用 AI Agent 协作入口与贡献指南 |
| [github-repo-presentation.md](github-repo-presentation.md) / [open-source-presentation-checklist.md](open-source-presentation-checklist.md) | GitHub 仓库首页与开源展示自检清单 |

## 测试与阶段报告

自动化测试规范以 `.agents/skills/` 为唯一来源（[project-testing](../.agents/skills/project-testing/SKILL.md)、[backend-testing](../.agents/skills/backend-testing/SKILL.md)、[frontend-unit-testing](../.agents/skills/frontend-unit-testing/SKILL.md)、[admin-e2e-testing](../.agents/skills/admin-e2e-testing/SKILL.md)、[api-contract-testing](../.agents/skills/api-contract-testing/SKILL.md)）。

历史迭代产生的阶段验证、审计与验收报告按文件名前缀归档，阶段进度以 [PROGRESS.md](PROGRESS.md) 为准：

| 前缀 / 文档族 | 内容 |
| --- | --- |
| [TEST_DATABASE_ISOLATION.md](TEST_DATABASE_ISOLATION.md) / [GO_TEST_STABILITY_REPORT.md](GO_TEST_STABILITY_REPORT.md) | 测试库隔离与 Go 测试稳定性 |
| `DEMO_AUTO_ACCEPTANCE_*` / [DEMO_AUTO_ACCEPTANCE_GUIDE.md](DEMO_AUTO_ACCEPTANCE_GUIDE.md) | Demo 自动验收指南与运行报告 |
| `F9_*` | F9 阶段验收报告（E2E、RBAC、回滚灰度、响应式等） |
| `P1_*` – `P3_*` | 生产配置、可靠性闭环、抖店适配器与多店铺 Webhook 验证 |
| `P4_*` | 安全与多租户：RBAC、租户隔离、IDOR、密钥轮换、PII 脱敏、上传下载安全等 |
| `P5_*` | 可观测性：业务埋点、OTLP 导出、SLO 评估与告警 |
| `P6_*` | 备份、恢复、发布、回滚与灾备演练 |
| `P7_*` | 性能与容量：负载/浸泡测试、基线冻结、主机隔离与可重复性诊断 |
| `P8_*` / `P9_*` | 运营任务中心与 PostgreSQL 集成各批次范围、执行计划与关闭门评审 |
| `H1_*` / [WORKBENCH_URL_STATE_DESIGN.md](WORKBENCH_URL_STATE_DESIGN.md) | 工作台 URL 状态保持设计与浏览器验收 |
| `*_UX_ACCEPTANCE.md` / `*_UX_AUDIT.md` / `*_PERF_REPORT.md` | 各模块 UX 验收、体验审计与性能报告 |
| [FULL_PROJECT_MVP_GAP_AUDIT.md](FULL_PROJECT_MVP_GAP_AUDIT.md) / [FULL_PROJECT_DEVELOPMENT_PLAN.md](FULL_PROJECT_DEVELOPMENT_PLAN.md) | MVP 缺口审计与后续开发计划 |
| [FUNCTION_FREEZE_RULES.md](FUNCTION_FREEZE_RULES.md) / [POST_FREEZE_BACKLOG.md](POST_FREEZE_BACKLOG.md) / [POST_F9_ENHANCEMENT_PLAN.md](POST_F9_ENHANCEMENT_PLAN.md) | 功能冻结规则与冻结后 backlog |

## 安全

| 文档 | 内容 |
| --- | --- |
| [../SECURITY.md](../SECURITY.md) | 安全漏洞披露与部署安全建议 |
| [SECRET_MANAGEMENT_GUIDE.md](SECRET_MANAGEMENT_GUIDE.md) | 密钥管理：加密存储、轮换与脱敏 |
| [RBAC_PERMISSION_DESIGN.md](RBAC_PERMISSION_DESIGN.md) / [STORE_PERMISSION_DESIGN.md](STORE_PERMISSION_DESIGN.md) / [GLOBAL_NAV_PERMISSION_DESIGN.md](GLOBAL_NAV_PERMISSION_DESIGN.md) | 权限矩阵、店铺归属与导航权限设计 |
| [OPERATION_AUDIT_DESIGN.md](OPERATION_AUDIT_DESIGN.md) | 操作日志与审计设计 |
| [SECURITY_PERMISSION_CHECKLIST.md](SECURITY_PERMISSION_CHECKLIST.md) / [SECURITY_RELEASE_CHECK.md](SECURITY_RELEASE_CHECK.md) | 安全权限检查清单与发布前安全检查 |
| [WEBHOOK_SIGNATURE_AND_REPLAY_PROTECTION.md](WEBHOOK_SIGNATURE_AND_REPLAY_PROTECTION.md) / [WEBHOOK_HTTP_RECEIVER_DESIGN.md](WEBHOOK_HTTP_RECEIVER_DESIGN.md) | Webhook 签名、防重放与接收器设计 |
| [P4_SSRF_SECURITY.md](P4_SSRF_SECURITY.md) / [P4_UPLOAD_DOWNLOAD_SECURITY.md](P4_UPLOAD_DOWNLOAD_SECURITY.md) / [P4_AUTH_SESSION_SECURITY.md](P4_AUTH_SESSION_SECURITY.md) | SSRF、文件上传下载与认证会话安全 |
| [STORAGE_PUBLIC_URL_GUIDE.md](STORAGE_PUBLIC_URL_GUIDE.md) / [STORAGE_PUBLIC_CHECK_GUIDE.md](STORAGE_PUBLIC_CHECK_GUIDE.md) | 存储公开访问配置与检查 |

## 社区与治理

| 文档 | 内容 |
| --- | --- |
| [sponsor.md](sponsor.md) | 微信 / 支付宝赞助入口与赞助榜 |
| [../CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) | 社区行为准则 |
| [../NOTICE](../NOTICE) | 第三方声明、商标和致谢 |
| [../LICENSE](../LICENSE) | Apache-2.0 开源协议 |
| [../CHANGELOG.md](../CHANGELOG.md) | 版本与重要变更记录 |

## 仓库关键文件说明

| 路径 | 作用 |
| --- | --- |
| `.github/CODEOWNERS` | 定义关键目录负责人，PR 改动匹配路径时请求维护者 review。 |
| `.github/dependabot.yml` | 自动检查 GitHub Actions、pnpm、Go modules、Docker 依赖更新。 |
| `.github/labeler.yml` | 按改动路径为 PR 自动打 `area:*`、`needs:*` 标签。 |
| `.github/workflows/` | Go / Node / Docker 配置检查 / PR Labeler 等 GitHub Actions。 |
| `.github/ISSUE_TEMPLATE/` | Bug、Feature、Documentation Issue 模板与入口配置。 |
| `.cursor/rules/` | Cursor / AI Agent 项目级持久规则。 |
| `.agents/skills/` | 代码质量、架构、测试、UI 设计等专项 Skill 规范。 |
| `AGENTS.md` | 通用 AI 编程工具协作入口，适用于 Cursor 以外的 Agent。 |
| `.env.example` | 本地开发环境变量模板。 |
| `.env.docker.example` | Docker 完整部署环境变量模板。 |
| `docker-compose.yml` | 本地开发基础设施：PostgreSQL + Redis。 |
| `docker-compose.full.yml` | 完整 Docker 部署编排：PostgreSQL + Redis + backend + admin + collector。 |
| `docker-compose.prod.yml` | 生产部署编排（配合 Caddy HTTPS）。 |

## 维护规则

新增或修改功能时，请同步检查：

- 启动命令变化：更新 `README.md`、`README.en.md`、`development.md`。
- Docker 行为变化：更新 `docker-deployment.md`、`.env.docker.example`。
- 环境变量变化：更新 `.env.example`、必要时更新 `.env.docker.example`，并同步 [env.md](env.md)。
- API / Provider / 队列 / 数据库变化：更新 [api.md](api.md)、[provider.md](provider.md)、[module-map.md](module-map.md) 或对应架构文档。
- 分支、CI、PR 流程变化：更新 `branching.md`、`CONTRIBUTING.md`、PR 模板。
- 较大模块或阶段性变更：更新 [PROGRESS.md](PROGRESS.md)。
- 新增文档：按 使用 / 部署 / 开发 / 测试 / 安全 归类，更新本索引对应分组。

详细规则见 [ai-coding-rules.md](ai-coding-rules.md)。
