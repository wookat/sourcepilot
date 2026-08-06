# 上线 Checklist（云服务器 + 公网域名 + HTTPS）

本清单配合 [production-deployment.md](production-deployment.md) 使用，目标：资源到位后 **30 分钟内上线**。
本清单已在开发 VM 上按同一路径完整演练（APP_ENV=production + docker-compose.prod.yml + Caddy 内部 CA + /etc/hosts 假域名 `trademind.local`），全部步骤可复现。

## 一、资源清单（需老板/资源方提供）

| 资源 | 要求 |
| --- | --- |
| 云服务器 | 2 vCPU / 4 GB 内存起步（Playwright 采集吃内存），40 GB+ 磁盘，Ubuntu 22.04+ / Debian 12+，可访问外网（拉镜像/证书签发） |
| 域名 | 1 个（如 `trademind.example.com`）；中国大陆服务器需 ICP 备案，未备案先用海外服务器 |
| DNS 记录 | A 记录：`域名 → 服务器公网 IP`；上线前 `dig +short 域名` 须返回该 IP |
| 防火墙/安全组 | 放行 TCP 80、TCP 443、UDP 443（HTTP/3）；SSH 端口按需收紧 |
| 对象存储 | 生产禁止 local storage：准备 S3/COS/OSS/R2/MinIO 的 bucket 与密钥（上传文件 + 备份各一前缀/桶） |
| SSH 访问 | 具 sudo 权限的部署账号 |

## 二、逐步操作（预计 30 分钟）

```bash
# 0. 安装 Docker（约 3 分钟）
curl -fsSL https://get.docker.com | sh

# 1. 获取代码（约 1 分钟）
git clone https://github.com/wookat/sourcepilot.git trademind && cd trademind

# 2. 准备环境变量（约 10 分钟）
cp .env.prod.example .env && chmod 600 .env
# 必填（deploy-prod.sh 会校验非空）：
#   DOMAIN / ACME_EMAIL / ADMIN_PUBLIC_URL / API_PUBLIC_URL
#   APP_MASTER_KEY=$(openssl rand -hex 32)
#   JWT_SECRET=$(openssl rand -base64 48)
#   PAGINATION_CURSOR_SIGNING_KEY=$(openssl rand -hex 32)
#   POSTGRES_PASSWORD=DB_PASSWORD=$(openssl rand -hex 24)   # 两者一致
#   ADMIN_BOOTSTRAP_EMAIL / ADMIN_BOOTSTRAP_PASSWORD（≥12 位强密码）
#   ADMIN_BOOTSTRAP_TENANT_ID=0（默认；引导账号即平台管理员，登录后在「设置 → 平台租户」开租）
#   STORAGE_PROVIDER=s3|cos|oss|r2|minio（禁止 local）
#   CORS_ALLOWED_ORIGINS=https://<DOMAIN>（与 ADMIN_PUBLIC_URL 一致）

# 3. 一键部署：构建 + 启动 + 迁移 + 健康检查 + HTTPS 探活（约 10-15 分钟，首次构建为主）
./scripts/deploy-prod.sh --no-pull

# 4. 每日备份 crontab（后端无内置定时器，必须配置宿主机 cron）
sudo mkdir -p /var/backups && crontab -l 2>/dev/null | { cat; echo '0 3 * * * cd /path/to/trademind && docker compose -f docker-compose.prod.yml exec -T postgres pg_dump -U trademind -d trademind -Fc > /var/backups/trademind-$(date +\%F).dump && find /var/backups -name "trademind-*.dump" -mtime +7 -delete'; } | crontab -
```

## 三、上线后验证清单

| # | 验证项 | 命令/操作 | 期望 |
| --- | --- | --- | --- |
| 1 | 容器全部健康 | `docker compose -f docker-compose.prod.yml ps` | 6 个服务均 healthy |
| 2 | HTTPS 探活 | `curl -fsS https://<DOMAIN>/health-backend` | `{"code":0,...}`，database/redis 均 ok |
| 3 | 正式证书 | 浏览器访问 `https://<DOMAIN>`，证书为 Let's Encrypt 且无告警 | 锁标志正常 |
| 4 | 管理员登录 | 用 `.env` 引导账号登录（account 字段为邮箱或手机号） | 登录成功进入工作台；**随即改密** |
| 5 | 核心页面抽查 | 工作台 / 商品草稿 / 采集任务 / 系统设置 | 正常渲染，无白屏报错 |
| 6 | 静态资源缓存 | `curl -sI https://<DOMAIN>/<hash>.js` | `cache-control: public, max-age=31536000, immutable`；`/` 为 `no-cache` |
| 7 | 内部端口未暴露 | `curl https://<DOMAIN>/metrics`、`ss -tlnp` | /metrics 返回 SPA 页（非指标）；宿主机仅 80/443 监听 |
| 8 | 生产危险操作禁用 | `POST /api/v1/ops/restores`（带 token） | `RESTORE_APP_ENV_FORBIDDEN`（恢复演练在生产被拒） |
| 9 | 错误不泄露堆栈 | 构造 400/404 请求 | 统一 envelope（code/message/traceId），无堆栈 |
| 10 | 手工备份 | `POST /api/v1/ops/backups`（或等 crontab） | status=completed；加密产物落在 backend 容器 `/tmp/trademind-p6-backups/`（暂不入桶，见下方说明） |
| 11 | 系统设置密钥 | 管理端配置 AI/对象存储密钥 | 加密入库、脱敏展示 |
| 12 | 平台租户开租 | 引导账号登录 →「设置 → 平台租户」新建租户 | 创建成功；新租户初始管理员可用邮箱+密码登录 |
| 13 | 会话治理 | `POST /auth/refresh`（cookie）→ `POST /auth/logout` → 再 refresh/带旧 token 访问 | 续期成功；登出后 `AUTH_REFRESH_TOKEN_REVOKED` / `AUTH_SESSION_REVOKED` |
| 14 | 备份校验/下载 | `POST /ops/backups/{backupId}/verify`、`GET .../download` | checksum/pg_restore_list/manifest/encryption 全 passed；下载文件 SHA-256 与 checksum 一致 |

## 四、回滚方案

```bash
cd trademind
git log --oneline -10                # 找到上一个正常 commit
git checkout <上一个正常commit>
./scripts/deploy-prod.sh --no-pull   # 用旧版本重建
# 恢复后切回分支：git checkout main
```

- 回滚仅回退代码与镜像；数据卷（PostgreSQL/Redis/上传/证书）不受影响。
- 若新版本已做不兼容迁移：先 `stop backend`，用 `pg_restore --clean --if-exists` 恢复最近备份，再启动旧版本（详见 production-deployment.md「备份与恢复」；注意恢复时用 `docker exec -i` 传 stdin，`docker compose exec -T ... < 文件` 在部分 Compose 版本会损坏二进制流）。
- **严禁** `down -v`（删库删证书）。
- **带存量数据的版本升级**（非首次上线）不走本清单，按 [`upgrade-guide.md`](upgrade-guide.md) 执行：先 `./scripts/deploy-prod.sh --pre-upgrade-check`（全量备份 + 迁移预检）再部署。

## 五、演练结论与已知注意点（2026-08-02 首演；2026-08-03 复跑；2026-08-04 R106 复检）

- 从零（`cp .env.prod.example` 起）到 6 服务全 healthy + HTTPS 登录成功，实测 < 15 分钟（含镜像构建；2026-08-03 复跑约 4 分钟；2026-08-04 R106 全新 VM 从零复跑：部署 275 秒、登录累计 < 7 分钟；2026-08-06 R132 复跑：从零到登录 251 秒；2026-08-06 R137 季度复检从零复跑：部署 234 秒、从零到登录 251 秒），30 分钟目标有充分余量。
- 引导管理员口径为 tenant 0 平台管理员（`ADMIN_BOOTSTRAP_TENANT_ID=0`），业务租户一律经「设置 → 平台租户」创建；仅遗留单租户部署才显式设 >0。
- `BACKUP_SCHEDULE` 仅是配置元数据，后端**没有内置定时器**：每日自动备份靠宿主机 crontab（上文第 4 步），勿以为配了变量就有自动备份。
- 后端镜像已内置 postgresql-client-16（`pg_dump`/`pg_restore`/`psql`），`/api/v1/ops/backups` 手工备份可用（演练中发现缺失并已修复）。
- 对象存储上传**尚未实现**：`BACKUP_MODE=object_storage` 只做配置校验，`/api/v1/ops/backups` 的产物仍落在 backend 容器 `/tmp`（容器重建即丢，download 也会失效），记录中的 `storageProvider` 仅反映配置而非实际存储位置。生产的权威备份是宿主机 crontab pg_dump（上文第 4 步），必须配置；`BACKUP_STORAGE_BUCKET` 为空当前不会导致备份失败。
- Let's Encrypt 正式签发依赖真实 DNS + 公网 80/443，为本地演练唯一未覆盖项；异常按 production-deployment.md「常见问题」第一条排查（先切 staging CA 联调）。
- 登录接口 body 为 `{"account": "...", "password": "..."}`（account=邮箱或手机号，不是 email 字段）。
- `./scripts/deploy-prod.sh --pre-upgrade-check` 默认备份目录 `/var/backups`，非 root 部署账号无法创建时会直接失败：先 `sudo mkdir -p /var/backups && sudo chown $USER /var/backups`，或每次显式 `BACKUP_DIR=<可写目录>` 运行。

### Let's Encrypt 真实域名签发注意事项（本地无法覆盖，上线时逐项确认）

1. **签发前置**：`dig +short <DOMAIN>` 必须已返回服务器公网 IP（DNS 生效有传播延迟，改完 A 记录等几分钟再部署）；云安全组与主机防火墙同时放行 TCP 80 + TCP 443（ACME HTTP-01 走 80，TLS-ALPN-01 走 443），任何 CDN/代理（如 Cloudflare 橙云）先关闭或改 DNS-only，否则验证请求到不了 Caddy。
2. **先用 staging CA 联调**：首次上线建议先设 `ACME_CA=https://acme-staging-v02.api.letsencrypt.org/directory` 验证签发链路（staging 证书浏览器会告警属正常），成功后去掉该变量并 `docker compose -f docker-compose.prod.yml restart caddy` 换正式证书。
3. **速率限制**：Let's Encrypt 正式环境每注册域名每周最多 50 张证书、同一组域名 5 次重复签发、每小时 5 次失败验证。反复失败会被限一小时甚至一周——所以**不要在正式 CA 上反复试错**，联调一律走 staging。
4. **签发失败排查顺序**：`docker compose -f docker-compose.prod.yml logs caddy` 看 ACME 错误 → 确认 DNS/80/443/CDN → 确认 `ACME_EMAIL` 有效（Caddyfile 默认 `admin@example.com` 占位必须替换）→ 按 production-deployment.md「常见问题」第一条处理。
5. **续期**：Caddy 自动在到期前约 30 天续期，无需 cron；但要保证 80/443 长期可达且 `caddy_data` 卷不被删除（`down -v` 会连证书一起删，严禁）。上线后可用 `openssl s_client -connect <DOMAIN>:443 -servername <DOMAIN> 2>/dev/null | openssl x509 -noout -dates` 记录到期时间并做外部监控。
