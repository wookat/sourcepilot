# 环境变量说明

本文件是 `.env.example` 与 `.env.docker.example` 的说明索引。新增、删除或重命名环境变量时，必须同步更新本文件，并检查 `docs/module-map.md` 中的关联项。

## 使用方式

本地开发：

```bash
cp .env.example .env
```

Windows PowerShell：

```powershell
Copy-Item .env.example .env
```

Docker 完整部署：

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

## 安全规则

- 不提交 `.env`。
- 生产环境必须替换 `JWT_SECRET`、`APP_MASTER_KEY`、`ADMIN_BOOTSTRAP_PASSWORD`、数据库密码。
- AI API Key、平台 Secret、Access Token、Refresh Token、Webhook Secret 不应写入环境模板，优先通过后台 settings 加密保存。
- settings 敏感项由服务端敏感 key 注册表（`settings.IsSensitiveKey`，来源：`IntegrationConfigDefinitions` 与各平台 App/Publish Config Schema 的 `Sensitive` 声明）强制加密落库与脱敏回显，不信任客户端 `isEncrypted` 标志；需要 `APP_MASTER_KEY` 已配置，否则保存敏感项返回 400 提示。
- 日志不得输出完整密钥、Token、Cookie 或密码。

## 后端基础配置

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `APP_ENV` | `development` | backend | 否 | 应用环境，生产建议设为 `production`。 |
| `APP_HTTP_ADDR` | `:8080` | backend | 否 | Go API 监听地址。 |
| `P7_V2_API_HOST` | `127.0.0.1` | P7-V2 harness | 否 | 仅 P7-V2 性能环境的 loopback API host；禁止公网或 WSL 非 loopback 地址。 |
| `P7_V2_API_PORT` | `8080` | P7-V2 harness | 否 | 仅 P7-V2 性能环境 API 端口；可迁移至 `18080`、`28080` 或 `38080`。 |
| `P7_BASE_URL` | 派生自前两项 | P7-V2 harness | 否 | 必须等于 `http://${P7_V2_API_HOST}:${P7_V2_API_PORT}`，供 k6 与探针统一使用。 |
| `P7_DIAGNOSTICS_*` | `P7_DIAGNOSTICS_ENABLED=false` | P7-V2 diagnostics | 否 | R3B local diagnostics switch and metadata (`P7_DIAGNOSTIC_RUN_ID` / `ROLE` / `DIR` / buffer / runtime + PG sample intervals). When explicitly enabled, writes bounded JSONL diagnostics to ignored `artifacts/p7-v2-diagnostics/` (or an external `/tmp/...` dir); diagnostic runs are non-formal and invalid for closure. |
| `APP_MASTER_KEY` | 空 / 64 位 hex | backend | 是 | AES-GCM 主密钥，用于 settings 敏感配置加密。 |
| `ADMIN_BOOTSTRAP_EMAIL` | 空 / `admin@example.com` | backend | 否 | 初始管理员邮箱。 |
| `ADMIN_BOOTSTRAP_PHONE` | 空 | backend | 否 | 初始管理员手机号。 |
| `ADMIN_BOOTSTRAP_PASSWORD` | 空 / 示例密码 | backend | 是 | 初始管理员密码，生产必须强密码。 |
| `ADMIN_BOOTSTRAP_TENANT_ID` | `0`（代码与示例文件一致） | backend | 否 | 初始管理员的租户 ID。默认 `0` = 平台租户：引导账号即平台管理员，负责平台租户治理（开租/停用/改名），业务租户通过「平台租户管理」创建。选品（selection）等按租户隔离的 worker 模块要求业务租户（>0）账号发起；如需把引导账号放入某个业务租户（遗留单租户部署），显式设置 >0 的值。仅在 admin_users 为空首次创建时生效，不迁移存量数据。 |
| `JWT_SECRET` | `change-me-in-production` | backend | 是 | JWT 签名密钥。 |
| `JWT_EXPIRE_HOURS` | `168` | backend | 否 | JWT 有效期小时数（仅 `legacy_local_storage` 模式生效）。 |
| `AUTH_SESSION_MODE` | dev 默认 `legacy_local_storage`；staging/production 强制 `secure_session` | backend | 否 | 会话模式。`secure_session`：refresh token 走 HttpOnly Cookie，access token 必须带 session 绑定；`legacy_local_storage`：仅限开发/遗留本地部署。 |
| `AUTH_REGISTER_SKIP_EMAIL_VERIFY` | `false` | backend | 否 | 本地/自托管无 SMTP 时显式关闭注册邮箱验证（见下方说明）。 |
| `UPLOAD_MAX_MB` | `10` | backend | 否 | 单文件上传大小上限。 |
| `TRUSTED_PROXIES` | 空（不信任任何代理） | backend | 否 | 可信反向代理 IP/CIDR 列表（逗号分隔）。留空时忽略 `X-Forwarded-For`，客户端 IP 取 TCP peer 地址，登录、鉴权失败预算、限流等每 IP 口径无法被伪造该头绕过。部署在 nginx / LB 之后且入口唯一时，填该代理的具体 IP；若同时对外发布了 backend 端口，请保持留空（否则外部请求经端口映射同样落在代理网段内，可伪造该头）。 |

### secure_session 模式的 legacy token 收紧与迁移

`secure_session` 模式下（staging/production 默认且强制），后端不再接受无 session 绑定的 legacy JWT（登录接口旧版签发、claims 中无 `session_id` 的 token），统一返回 `401` + `AUTH_SESSION_BINDING_REQUIRED`。

- **现网升级路径**：从旧版本（或从 `legacy_local_storage` 切到 `secure_session`）升级后，存量 legacy token 首次请求即被 401 拒绝，前端会话守卫会弹出「登录已过期」引导重新登录；重新登录后即获得 session 绑定 token，无需额外迁移操作。
- **开发/遗留部署**：`APP_ENV=development` 下默认 `legacy_local_storage`，legacy token 行为不变；显式设 `AUTH_SESSION_MODE=secure_session` 时与生产同口径。
- **CSRF Origin 校验**：`secure_session` 下写请求校验 `Origin`/`Referer`，必须同时设置 `ADMIN_PUBLIC_URL` 为管理端对外地址（如 `http://localhost:8000`），否则登录等写请求返回 `403 ORIGIN_NOT_ALLOWED`（Docker 部署同理，见 `.env.docker.example`）。
- **数据库故障 fail-closed**：认证状态（账号/租户/session 绑定校验）在数据库瞬断时仅接受不超过 30 秒的 last-known-good 快照；无新鲜快照则统一 `401 AUTH_STATE_UNAVAILABLE`（可重试），不会误报为 `AUTH_SESSION_REVOKED`（后者会触发前端强制登出）。session 确实不存在/已吊销仍返回 `AUTH_SESSION_REVOKED`。

### 无 SMTP 部署的注册降级（AUTH_REGISTER_SKIP_EMAIL_VERIFY）

注册默认依赖邮箱验证码（SMTP 在「设置 → 邮件设置」配置）。SMTP 未配置时：

- `POST /api/v1/auth/send-email-code` 返回 503 + 中文引导（提示管理员配置 SMTP 或使用本开关），验证码不会写入 Redis，注册无法完成——这是安全默认。
- 本地/自托管部署可显式设置 `AUTH_REGISTER_SKIP_EMAIL_VERIFY=true`，注册免邮箱验证码（`code` 可为空），登录页注册表单自动隐藏验证码输入（由 `GET /api/v1/auth/register-config` 下发）。
- 该开关默认 `false`，且仅在非 staging/production 环境生效；`APP_ENV=staging|production` 下设 `true` 会在启动配置校验时直接报错（insecure_auth_config）。开启即允许任意可达者注册开租，仅限内网/本机部署使用。

## 可观测性与 OTLP

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `OBSERVABILITY_ENABLED` | `true` | backend | 否 | 是否启用日志、指标、追踪等可观测性基础能力。 |
| `OBSERVABILITY_MODE` | `local` / `hybrid` | backend | 否 | 本地、Prometheus、OTel 或混合模式。 |
| `OBSERVABILITY_ENVIRONMENT` | `development` | backend | 否 | 低基数环境标签。 |
| `TRACING_ENABLED` | `false` | backend | 否 | 是否启用 tracing。真实 OTLP backend 未配置时保持 `false` 或本地 Mock 验证。 |
| `OTEL_SERVICE_NAME` | `trademind-api` | backend | 否 | OTLP resource `service.name`。 |
| `OTEL_SERVICE_VERSION` | 空 | backend | 否 | OTLP resource `service.version`。 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 空 | backend | 否 | OTLP/HTTP endpoint；代码会规范化为 `/v1/traces`。为空表示真实 telemetry backend 验证 Deferred。 |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/json` | backend | 否 | P5-V 使用标准 OTLP/HTTP JSON。 |
| `OTEL_EXPORTER_OTLP_HEADERS` | 空 | backend | 是 | 可选 header 列表，格式 `k=v,k2=v2`；不得提交真实 Token，日志不得输出。 |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` / `false` | backend | 否 | 本地 Mock Collector 可为 `true`；生产建议 `false`。 |
| `OTEL_TRACE_SAMPLE_RATIO` | `0` / `0.1` | backend | 否 | 采样比例，生产上限由配置校验限制。 |
| `OTEL_EXPORT_TIMEOUT_SECONDS` | `10` | backend | 否 | 单次导出超时，代码限制最大 30 秒。 |
| `OTEL_EXPORT_QUEUE_SIZE` | `1024` | backend | 否 | OTel batcher 有界队列大小。 |
| `OTEL_EXPORT_BATCH_SIZE` | `128` | backend | 否 | 单批导出数量，不得超过队列大小。 |
| `OTEL_EXPORT_RETRY_MAX` | `2` | backend | 否 | 429/5xx 受控重试次数，上限 5。 |

## Webhook 入站（公开 HTTP）

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `WEBHOOK_MAX_BODY_KB` | `512` | backend | 否 | `POST /api/v1/webhooks/:platform/:eventType` 请求体上限（KiB）。 |
| `WEBHOOK_MAX_CLOCK_SKEW_SECONDS` | `300` | backend | 否 | 允许的时间戳时钟偏差；超时或远未来时间戳返回 `WEBHOOK_TIMESTAMP_EXPIRED`。 |
| `WEBHOOK_ENABLE_TEST_VERIFIER` | `false` | backend | 否 | 启用 `internal-test` HMAC-SHA256 测试验签；**仅** `APP_ENV=development` / `test` 生效，production 强制关闭。 |
| `WEBHOOK_WORKER_INTERVAL_SECONDS` | `3` | backend | 否 | DB 轮询 `webhook_events.status=queued` 的间隔秒数。 |
| `DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID` | 空 | backend | 否 | P3.2 多店铺 Webhook 显式测试兜底绑定 ID；仅 `development` / `test` 生效，`staging` / `production` 配置后 fail-fast。 |
| `ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK` | `false` | backend | 否 | Demo 环境是否允许使用显式 Webhook 兜底绑定；`staging` / `production` 必须为 `false`。 |

## 数据库

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `DB_DRIVER` | `postgres` | backend | 否 | 默认 PostgreSQL；仅遗留库或明确要求时用 MySQL。 |
| `DB_HOST` | `127.0.0.1` / `postgres` | backend | 否 | 数据库地址。容器内互访用服务名 `postgres`；在宿主机直跑后端命令（如 `pnpm seed:demo:full` / `go run ./cmd/seeddemo`）而 `.env` 写的是 `postgres` 时，需临时覆盖为 `DB_HOST=127.0.0.1`（PowerShell：`$env:DB_HOST="127.0.0.1"`）。 |
| `DB_PORT` | `5432` | backend | 否 | PostgreSQL 默认 5432。 |
| `DB_USER` | `trademind` | backend | 否 | 数据库用户。 |
| `DB_PASSWORD` | `trademind` | backend | 是 | 数据库密码。 |
| `DB_NAME` | `trademind` | backend | 否 | 数据库名。 |
| `DB_TIMEZONE` | `UTC` | backend | 否 | 数据库时区。 |
| `DB_MAX_OPEN_CONNECTIONS` | `100` | backend | 否 | P7 数据库连接池最大打开连接数；生产非法配置 fail-fast。 |
| `DB_MAX_IDLE_CONNECTIONS` | `10` | backend | 否 | P7 数据库连接池最大空闲连接数。 |
| `DB_CONN_MAX_LIFETIME_SECONDS` | `3600` | backend | 否 | P7 单连接最大生命周期。 |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | `900` | backend | 否 | P7 空闲连接最长保留时间。 |
| `DB_QUERY_TIMEOUT_MS` | `5000` | backend | 否 | P7 查询超时预算；逐步接入仓储查询。 |
| `DB_TRANSACTION_TIMEOUT_MS` | `10000` | backend | 否 | P7 事务超时预算；必须大于等于查询超时。 |
| `POSTGRES_DB` | `trademind` | docker postgres | 否 | Docker Postgres 初始化库名。 |
| `POSTGRES_USER` | `trademind` | docker postgres | 否 | Docker Postgres 用户。 |
| `POSTGRES_PASSWORD` | 示例密码 | docker postgres | 是 | Docker Postgres 密码。 |

## Redis

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `REDIS_ADDR` | `127.0.0.1:6379` / `redis:6379` | backend | 否 | Redis 地址。 |
| `REDIS_PASSWORD` | 空 | backend | 是 | Redis 密码。 |
| `REDIS_DB` | `0` | backend | 否 | Redis DB 编号。 |

## Collector

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `COLLECTOR_BASE_URL` | `http://127.0.0.1:3100` | backend | 否 | Go API 调用 Collector 的基础地址。 |
| `COLLECTOR_TIMEOUT_SECONDS` | `120` | backend | 否 | 后端调用 Collector 超时；淘宝/天猫任务会按页面打开超时自动放宽（约 `gotoTimeoutMs + 90s`）。 |
| `COLLECTOR_HTTP_ADDR` | `:3100` / `:3001` | collector | 否 | Collector 监听地址。 |
| `COLLECTOR_MAIN_SERVICE_URL` | `http://127.0.0.1:8080` | collector | 否 | Collector 回调或访问后端的基础地址预留。 |
| `COLLECTOR_GOTO_TIMEOUT_MS` | `45000` | collector | 否 | Playwright 页面打开超时。 |
| `COLLECTOR_HEADLESS` | `1` | collector | 否 | 是否无头浏览器运行；本地打开登录浏览器时可设为 `0`。 |
| `COLLECTOR_BROWSER_PROFILE_DIR` / `BROWSER_PROFILE_ROOT` | `collector/data/browser-profiles`（相对 collector 包根目录） | collector | 否 | 1688 持久化 Profile 根目录（1688 使用子目录 `1688`）。Docker 通常设为 `/workspace/data/browser-profiles`。 |
| `COLLECTOR_STORAGE_STATE_DIR` | `data/storage-states` | collector | 否 | Playwright storageState 导出目录（预留）。 |
| `COLLECTOR_1688_AUTH_PROBE_URL` | 注释示例 | collector | 否 | 登录态检测时用于探测的商品详情 URL。 |
| `COLLECTOR_USER_AGENT` | 注释示例 | collector | 否 | 可选 UA 覆盖；未设置时按 bundled Chromium 实际主版本自动生成 Chrome UA。 |
| `COLLECTOR_PROXY_SERVER` | 注释示例 | collector | 否 | 可选采集出口代理地址（`http://host:port` 或 `socks5://host:port`）；仅配置项，不内置任何第三方代理。 |
| `COLLECTOR_PROXY_USERNAME` / `COLLECTOR_PROXY_PASSWORD` | 注释示例 | collector | 否 | 代理认证（可选）。 |
| `COLLECTOR_PROXY_BYPASS` | 注释示例 | collector | 否 | 逗号分隔的不走代理主机列表。 |

## 队列与任务

| 变量前缀 | 示例变量 | 服务 | 说明 |
| --- | --- | --- | --- |
| `COLLECT_*` | `COLLECT_QUEUE_ENABLED`、`COLLECT_WORKER_CONCURRENCY`、`COLLECT_QUEUE_NAME`、`COLLECT_BATCH_MAX_URLS`、`COLLECT_BATCH_CONCURRENCY_1688`、`COLLECT_BATCH_DELAY_MIN_MS_1688`、`COLLECT_BATCH_DELAY_MAX_MS_1688`、`COLLECT_BATCH_RETRY_ON_BLOCKED`、`COLLECT_BATCH_RETRY_ON_TIMEOUT`、`COLLECT_BATCH_MAX_RETRIES_1688` | backend | 采集任务队列、批量 URL 限制、1688 批量节流与重试。settings **`collector`** 分组可覆盖 1688 批量项；任务处理超时阈值走 settings `collector.collect_task_processing_timeout_seconds`（默认 600 秒，最小 30 秒，无对应环境变量），超时任务由 task reaper 自动置 `failed`（原因「任务超时」，可手动重试）。 |
| `SELECTION_*` | `SELECTION_QUEUE_ENABLED`、`SELECTION_QUEUE_NAME`、`SELECTION_WORKER_CONCURRENCY`、`SELECTION_TASK_TIMEOUT_SECONDS` | backend | AI 选品任务队列（Redis LIST + Worker）与任务租约 TTL。利润参数（汇率/佣金/物流/退货率）走 settings **`selection`** 分组或每任务 params 覆盖。 |
| `IMAGE_*` / `AI_IMAGE_*` | `IMAGE_QUEUE_ENABLED`、`IMAGE_TASK_TIMEOUT_SECONDS`、`AI_IMAGE_PROVIDER_TIMEOUT_SECONDS`、`AI_IMAGE_TASK_MAX_RUNTIME_SECONDS`、`AI_IMAGE_POLL_INTERVAL_SECONDS`、`AI_IMAGE_TRIAL_TIMEOUT_SECONDS` | backend / scripts | 图片任务队列、Provider/Worker 超时、Demo trial 轮询和总等待上限。 |
| `TRANSLATE_FONT_PATH` | — | backend | 可选。图片文字翻译程序绘制所用字体（TTF/TTC）；未设置时自动查找 Noto CJK / 微软雅黑 / 内置英文字体。Docker 镜像默认安装 `fonts-noto-cjk`。 |
| `ORDER_SYNC_*` | `ORDER_SYNC_QUEUE_ENABLED`、`ORDER_SYNC_QUEUE_NAME` | backend | 平台订单同步任务。 |
| `CUSTOMER_MESSAGE_SYNC_*` | `CUSTOMER_MESSAGE_SYNC_QUEUE_ENABLED` | backend | 客服消息同步任务。 |
| `PRODUCT_PUBLISH_*` | `PRODUCT_PUBLISH_QUEUE_ENABLED`、`PUBLISH_BATCH_MAX_PRODUCTS`（100）、`PUBLISH_BATCH_MAX_TARGETS`（20）、`PUBLISH_BATCH_MAX_TASKS`（300） | backend | 商品刊登任务队列与批量矩阵上限。 |
| `INVENTORY_SYNC_*` | `INVENTORY_SYNC_QUEUE_ENABLED` | backend | 库存同步任务。 |
| `WORKER_*` | `WORKER_HEARTBEAT_ENABLED`、`WORKER_REAPER_ENABLED` | backend | 多实例 Worker 心跳、过期判断和回收。 |
| `TASK_ALERT_*` | `TASK_ALERT_SCAN_ENABLED`、`TASK_ALERT_SCAN_INTERVAL_SECONDS` | backend | 任务告警扫描。 |
| `BUYER_MESSAGE_*` | `BUYER_MESSAGE_SCAN_ENABLED`、`BUYER_MESSAGE_SCAN_INTERVAL_SECONDS` | backend | 买家自动消息节点扫描：按租户规则为订单节点生成「待发送草稿」，只生成站内草稿，绝不自动外发。 |
| `BACKUP_*` | `BACKUP_ENABLED`、`BACKUP_MODE`、`BACKUP_STORAGE_PROVIDER`、`BACKUP_ENCRYPTION_ENABLED`、`BACKUP_RETENTION_DAILY` | backend | P6 备份、加密、校验、保留与恢复演练门闸。生产环境要求加密开启，且不得使用本地单副本。 |
| 备份定时器 | `BACKUP_SCHEDULE`、`BACKUP_SCHEDULE_ENABLED` | backend | R143 内置备份定时器。`BACKUP_SCHEDULE_ENABLED=true` 时 backend 按 `BACKUP_SCHEDULE` 自动触发备份（含 S3 上传与对象保留策略）；表达式支持 5 字段 cron（如 `0 3 * * *`）或简化 interval（`@every 6h` / `6h`，最小 1 分钟），非法表达式启动即报错。同一触发时隔（分钟精度 scheduleKey 唯一索引）重复触发幂等，不会重复跑备份；失败落 `status=failed` + `errorSummary`，可在 Ops 备份页查看。`false`（默认）时 `BACKUP_SCHEDULE` 仅为元数据，定时触发可继续用宿主机 crontab（备选路径）。 |
| 生产恢复演练开关 | `BACKUP_RESTORE_ALLOW_PRODUCTION` | backend | R143 防误操作开关。production AppEnv 下 `POST /api/v1/ops/restores` 与 `POST /api/v1/ops/restores/:id/verify` 默认拒绝（`RESTORE_APP_ENV_FORBIDDEN` / `RESTORE_VERIFY_APP_ENV_FORBIDDEN`）；显式设 `true` 才允许隔离恢复演练，且仍保留全部既有护栏：目标库必须为 `trademind_p6v_restore_*` 隔离库、需操作者重认证 + 高危二次确认、永远禁止恢复到 `production` 目标环境。权限上仅平台管理员（`restore.execute`/`restore.verify`）可调用，readonly 角色一律 403。 |
| `BACKUP_S3_*` / 上传 | `BACKUP_S3_ENDPOINT`、`BACKUP_S3_REGION`、`BACKUP_S3_ACCESS_KEY_ID`、`BACKUP_S3_SECRET_ACCESS_KEY`、`BACKUP_S3_USE_PATH_STYLE`、`BACKUP_UPLOAD_MAX_ATTEMPTS`、`BACKUP_OBJECT_RETENTION_COUNT` | backend | R138 备份产物 S3 兼容对象存储上传（AWS S3 / MinIO / 阿里 OSS S3 兼容端点，桶名复用 `BACKUP_STORAGE_BUCKET`、前缀复用 `BACKUP_STORAGE_PREFIX`）。全部留空时为降级模式：备份仅保存本地路径，不阻塞部署；半配置（只有 AK 无 SK、有凭据无桶等）会在启动时报配置错误。`BACKUP_S3_SECRET_ACCESS_KEY` 为敏感配置，任何日志/接口/错误信息中都不会输出。上传失败会按 `BACKUP_UPLOAD_MAX_ATTEMPTS` 重试并把失败状态落库，可在 Ops 备份页重试；`BACKUP_OBJECT_RETENTION_COUNT` 控制对象存储保留最近 N 份（0=不清理，retention hold 的备份不清理；有效前缀为空时拒绝清理整桶，且仅清理 `bk_*.dump`/`bk_*.dump.enc` 命名的备份产物）。`BACKUP_S3_ENDPOINT` 必须是合法 http(s) URL；生产环境要求 `https://` 且拒绝 localhost/回环/link-local（含 169.254.169.254 元数据地址）。本地 production 模式演练 MinIO 时：用非回环主机名（如 compose 网络内 `https://minio:9000`）+ 带 SAN 的自签证书启用 MinIO TLS，并通过 `BACKUP_S3_CA_BUNDLE`（见下行）显式指定自签 CA；真实生产对象存储用公信 CA 证书，无需此步。 |
| 备份 S3 自签 CA | `BACKUP_S3_CA_BUNDLE` | backend | R143 自签 CA 显式配置项（PEM 文件路径），替代仅靠容器挂载合并系统 CA 的约定：配置后备份 S3 客户端在系统信任库基础上额外信任该 bundle 中的证书。Docker 部署时把证书文件只读挂载进 backend 容器并填容器内路径；生产用 `deploy-prod.sh` 时该挂载应写进 `docker-compose.prod.override.yml`（脚本自动叠加，重跑部署不丢挂载，见 docs/production-deployment.md「日常升级」）。路径不存在或非法 PEM 会在启动时 fail-fast，避免静默降级。本地/内网 production-like MinIO 演练才需要；真实生产对象存储用公信 CA，留空即可。 |
| `POSTGRES_*` | `POSTGRES_BACKUP_FORMAT`、`POSTGRES_PG_DUMP_PATH`、`POSTGRES_PG_RESTORE_PATH`、`POSTGRES_WAL_ARCHIVE_ENABLED`、`POSTGRES_PITR_ENABLED` | backend | PostgreSQL 逻辑备份与 PITR 基础配置。真实生产 PITR 演练保持 Deferred。 |
| `RELEASE_*` | `RELEASE_ENABLED`、`RELEASE_ROOT`、`RELEASE_REQUIRE_PRE_BACKUP`、`RELEASE_STRATEGY`、`RELEASE_ROLLBACK_ON_FAILURE` | backend | P6 发布制品、发布前备份、受控发布与应用回滚配置。生产发布必须要求发布前备份。 |
| `PERFORMANCE_*` | `PERFORMANCE_TEST_MODE`、`PERFORMANCE_DATASET_MAX_ROWS`、`PERFORMANCE_TEST_MAX_VUS`、`PERFORMANCE_TEST_MAX_DURATION_SECONDS` | backend / scripts | P7 隔离性能测试与数据集保护；production 禁止开启测试模式。 |
| `PAGINATION_*` | `PAGINATION_DEFAULT_LIMIT`、`PAGINATION_MAX_LIMIT`、`PAGINATION_MAX_OFFSET`、`PAGINATION_CURSOR_SIGNING_KEY` | backend | P7 列表分页默认值、最大 limit、深 offset 保护与 Cursor HMAC 签名密钥；production 必须显式配置签名密钥。 |
| `RATE_LIMIT_*` | `RATE_LIMIT_ENABLED`、`RATE_LIMIT_MODE`、`RATE_LIMIT_FAIL_MODE`、`RATE_LIMIT_POLICY_VERSION` | backend | P7 HTTP 限流配置；production 禁用需显式审批变量。 |
| `MCP_*` | `MCP_ENABLED`、`MCP_RATE_RPS`、`MCP_RATE_BURST`、`MCP_WRITE_ENABLED` | backend | R144 MCP 只读入口（`POST /api/mcp`）：`MCP_ENABLED` 控制入口开关（默认 true）；`MCP_RATE_RPS` / `MCP_RATE_BURST` 为每个 token 的持续速率与突发上限（默认 5 / 10）。鉴权用租户级只读 API token（设置页「MCP 只读接入」创建）。R146：限流状态在 Redis 可用时复用队列 `REDIS_URL`（多副本共享额度，无新变量），Redis 不可用时降级为进程内限流（多副本下额度按副本数放大，需调低 RPS/BURST）。R179：`MCP_WRITE_ENABLED`（默认 false）为 MCP 写白名单的全局闸门，开启后仍需租户级设置 `mcp/write_enabled=true` 且 token 携带 `write:ops` scope 三层同时满足，详见 `docs/mcp.md`。R184 提示：写限额的跨进程硬保证依赖 PostgreSQL advisory lock；`DB_DRIVER` 非 postgres 且多副本部署时并发限额仅为软保证（详见 `docs/mcp.md`「fail-closed 审计与限额」）。 |
| `OPENAPI_*` | `OPENAPI_ENABLED`、`OPENAPI_RATE_RPS`、`OPENAPI_RATE_BURST` | backend | R152 开放 API 只读入口（`GET /api/open/v1/*`）：`OPENAPI_ENABLED` 控制入口开关（默认 true）；`OPENAPI_RATE_RPS` / `OPENAPI_RATE_BURST` 为每个 token 的持续速率与突发上限（默认 5 / 10）。与 MCP 共用只读 token 体系（token 用途须为 `openapi` 或 `both`），限流桶与 MCP 相互独立，Redis 可用时共享额度，详见 `docs/open-api.md`。 |
| `CACHE_*` | `CACHE_ENABLED`、`CACHE_DEFAULT_TTL_SECONDS`、`CACHE_MAX_ENTRIES`、`CACHE_SINGLEFLIGHT_ENABLED` | backend | P7 缓存与 singleflight 治理基础配置。 |
| `EXPORT_*` | `EXPORT_BATCH_SIZE`、`EXPORT_MAX_ROWS`、`EXPORT_MAX_BYTES`、`EXPORT_MAX_CONCURRENT` | backend | P7 导出批量、行数、字节数和并发上限。 |
| `PPROF_*` | `PPROF_ENABLED`、`PPROF_INTERNAL_ONLY` | backend | P7 Profiling 安全开关；production 禁止 public pprof。 |

新增队列变量时，还要同步健康检查说明、任务中心页面和 `docs/PROGRESS.md`。

## Docker 端口覆盖

`.env.docker.example` 支持以下宿主机端口覆盖：

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `ADMIN_PUBLISH_PORT` | `8000` | 管理端宿主机端口。 |
| `BACKEND_PUBLISH_PORT` | `8080` | 后端 API 宿主机端口。 |
| `COLLECTOR_PUBLISH_PORT` | `3001` | Collector 宿主机端口。 |
| `POSTGRES_PUBLISH_PORT` | `5432` | PostgreSQL 宿主机端口。 |
| `REDIS_PUBLISH_PORT` | `6379` | Redis 宿主机端口。 |

## 前端

| 变量 | 示例 / 默认 | 服务 | 说明 |
| --- | --- | --- | --- |
| `VITE_API_BASE` | `/api` | admin | 管理端 API 基础路径，当前在 `.env.example` 中为预留注释。 |

## 新增变量检查清单

新增或修改环境变量时必须检查：

- `.env.example`
- `.env.docker.example`
- `docker-compose.yml`
- `docker-compose.full.yml`
- `docs/env.md`
- `docs/development.md`
- `docs/docker-deployment.md`
- `README.md` / `README.en.md` 中的启动说明
- 相关代码默认值与安全校验
