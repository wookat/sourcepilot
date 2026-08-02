# Docker 部署说明

本文说明如何使用 Docker Compose 启动完整 TradeMind 项目。

> 生产环境（云服务器 + 公网域名 + 自动 HTTPS）请使用 `docker-compose.prod.yml`，见 [production-deployment.md](production-deployment.md)。

## 组成服务

`docker-compose.full.yml` 包含：

- PostgreSQL 16
- Redis 7
- backend：Go Gin API
- admin：React 管理端，使用 nginx 托管并代理 `/api`
- collector：Node.js + Playwright 采集服务

## 快速启动

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

Windows PowerShell：

```powershell
Copy-Item .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

## 默认访问地址

| 服务 | 地址 |
| --- | --- |
| Admin | `http://127.0.0.1:8000` |
| Backend Health | `http://127.0.0.1:8080/health` |
| Collector Health | `http://127.0.0.1:3001/health` |

## 端口配置

可在 `.env` 中覆盖以下端口：

```env
ADMIN_PUBLISH_PORT=8000
BACKEND_PUBLISH_PORT=8080
COLLECTOR_PUBLISH_PORT=3001
POSTGRES_PUBLISH_PORT=5432
REDIS_PUBLISH_PORT=6379
```

完整环境变量说明见 [env.md](env.md)。修改 Docker 变量时必须同步 `.env.docker.example`、`docker-compose.full.yml`、本文档和 `docs/env.md`。

backend 镜像内置 `postgresql-client-16`（与 compose 中 `postgres:16` 版本匹配），`BACKUP_ENABLED=true` 时备份/恢复演练可直接调用 `pg_dump` / `pg_restore`。

P5-V 可观测性默认使用 `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`。Docker 本地试用不配置真实 telemetry backend 时，`OTEL_EXPORTER_OTLP_ENDPOINT` 保持为空并视为 Deferred；不要把 Mock Collector 验证写成生产 collector 已上线。

P7 性能数据集与负载测试只能在隔离 `APP_ENV=performance` 环境执行；普通 Docker 试用与生产部署必须保持 `PERFORMANCE_TEST_MODE=false`、`ALLOW_PERFORMANCE_DATASET=false`，不得把隔离压测描述为真实生产容量验证。

## 生产备份 SOP（备份→下载→校验→恢复演练）

本节描述 Ops 备份链路的操作口径。本地/开发环境可完成最小真实闭环；生产环境保留更严格门槛。

### 1. 创建备份

- 前置：`BACKUP_ENABLED=true`，`BACKUP_MODE=local`（本地）或 `object_storage`/`hybrid`（生产强制）。
- 入口：Admin「运维 → 备份管理 → 创建备份」，或 `POST /api/v1/ops/backups`（需要 `backup.create` 权限）。
- 真实执行 `pg_dump`，生成 artifact 与带校验和的 manifest；`BACKUP_ENABLED=false` 时仅生成待复核记录。

### 2. 校验备份

- 入口：备份列表「校验备份」，或 `POST /api/v1/ops/backups/:id/verify`（`backup.verify` 权限）。
- 真实检查项：备份文件 SHA-256 校验和、manifest 完整性、`pg_restore --list` 结构校验。
- 加密检查：`BACKUP_ENCRYPTION_ENABLED=false`（本地 local 模式）时按「未启用（跳过）」处理，不判失败；生产强制启用加密，加密备份缺失密钥引用则判失败。
- 校验结果以结构化检查项（中文）返回并在页面弹窗展示。

### 3. 下载备份

- 入口：备份列表「下载」，或 `GET /api/v1/ops/backups/:id/download`。
- 权限：`backup.download`（settings.manage 级，仅 admin 角色具备）；readonly/operator 返回 403。
- 仅允许下载 `status=completed` 且校验（verification）已通过的备份；下载前再次校验 SHA-256，不匹配则拒绝。
- 备份不存在或越权访问返回 404，不暴露资源存在性与真实路径。
- 每次下载（含失败）写入操作日志（action=`backup.download`）。

### 4. 恢复演练（本地/开发限定）

- 入口：Admin「运维 → 恢复验证 / 灾备演练」，或 `POST /api/v1/ops/restores`、`POST /api/v1/ops/restores/:id/verify`、`POST /api/v1/ops/dr/drills`。
- 安全门：目标必须为隔离环境、目标库名以 `trademind_p6v_restore_` 开头、操作者二次确认与高风险确认；备份必须 completed 且校验通过。
- 前置条件：目标数据库必须**预先创建且为空**（例如 `docker compose -f docker-compose.full.yml exec postgres psql -U trademind -c 'CREATE DATABASE trademind_p6v_restore_drill1;'`），否则安全门以 `RESTORE_TARGET_CONNECT_FAILED` 拒绝。
- 恢复验证真实执行两项检查：备份文件完整性（SHA-256）、`pg_restore --list` 结构校验；其余检查项（迁移版本、租户隔离、RBAC、审计链、对象清单、密钥密文）明确标注「暂未实现」，不伪装通过。
- `APP_ENV=production` 下恢复验证与 DR 演练直接拒绝执行；生产恢复流程保持待接入。

### 失败处理与保留

- 校验/演练失败时以结构化检查项给出失败原因（已脱敏），修复后可重新触发。
- 需长期保留的备份可用「保留」（hold）标记，防止被保留策略清理；保留策略由 `BACKUP_RETENTION_*` 控制。

## 安全配置

生产环境或公网部署前必须修改：

- `JWT_SECRET`
- `APP_MASTER_KEY`
- `ADMIN_BOOTSTRAP_PASSWORD`
- `POSTGRES_PASSWORD`
- `DB_PASSWORD`
- 所有第三方平台、AI、存储、Webhook、邮箱等密钥

不要把真实密钥提交到仓库，也不要写入镜像。

## 常用命令

启动：

```bash
docker compose -f docker-compose.full.yml up -d --build
```

查看状态：

```bash
docker compose -f docker-compose.full.yml ps
```

查看日志：

```bash
docker compose -f docker-compose.full.yml logs -f backend
docker compose -f docker-compose.full.yml logs -f admin
docker compose -f docker-compose.full.yml logs -f collector
docker compose -f docker-compose.full.yml logs -f postgres
docker compose -f docker-compose.full.yml logs -f redis
```

停止并保留数据卷：

```bash
docker compose -f docker-compose.full.yml down
```

清空数据卷：

```bash
docker compose -f docker-compose.full.yml down -v
```

> `down -v` 会删除 PostgreSQL、Redis、上传目录等 Compose 管理的数据卷，请谨慎执行。

## 默认管理员

默认管理员由 `.env` 中的以下变量决定：

```env
ADMIN_BOOTSTRAP_EMAIL=admin@example.com
ADMIN_BOOTSTRAP_PASSWORD=admin123456
ADMIN_BOOTSTRAP_TENANT_ID=1
```

首次登录后请尽快修改密码。生产环境不要使用示例密码。

`ADMIN_BOOTSTRAP_TENANT_ID` 决定初始管理员所属租户：选品（selection）等按租户隔离的模块要求租户 ID > 0，否则 worker 会拒绝任务（任务停留在 pending）。仅在 admin_users 为空、首次创建管理员时生效；对已存在的历史管理员，需按 `P4_1_TENANT_DATA_MIGRATION.md` 的规则处理。

## 与本地开发 Compose 的区别

- `docker-compose.yml`：仅用于本地开发基础设施，包含 PostgreSQL + Redis。
- `docker-compose.full.yml`：用于完整 Docker 部署，包含 PostgreSQL + Redis + backend + admin + collector。

**1688 采集浏览器 Profile**：`docker-compose.full.yml` 为 collector 挂载 `./data/browser-profiles` 与 `./data/storage-states`，用于持久化 1688 登录 Cookie（含 Login Data、Cookies、History、Local Storage、Session Storage 等 Chromium 用户数据）。这些目录**必须持久化挂载、禁止提交 Git**（已在 `.gitignore` 忽略；本地 `collector/data/browser-profiles/` 同理）。容器内默认无图形界面，**首次登录建议在宿主机本地运行 collector（`COLLECTOR_HEADLESS=0`）完成 1688 登录**，Profile 目录可被 Docker 复用；或在已配置远程桌面的 Linux 服务器上打开登录浏览器。

两套 Compose 的服务、端口和数据卷应分开理解。

## 配置校验

CI 会执行轻量 Docker 配置检查：

```bash
docker compose -f docker-compose.full.yml config
```

本地修改 Dockerfile、Compose 或 `.env.docker.example` 后，建议先执行同样命令确认语法和变量引用正确。
