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
| 10 | 手工备份 | `POST /api/v1/ops/backups`（或等 crontab） | status=completed；备份文件落盘/入桶 |
| 11 | 系统设置密钥 | 管理端配置 AI/对象存储密钥 | 加密入库、脱敏展示 |

## 四、回滚方案

```bash
cd trademind
git log --oneline -10                # 找到上一个正常 commit
git checkout <上一个正常commit>
./scripts/deploy-prod.sh --no-pull   # 用旧版本重建
# 恢复后切回分支：git checkout main
```

- 回滚仅回退代码与镜像；数据卷（PostgreSQL/Redis/上传/证书）不受影响。
- 若新版本已做不兼容迁移：先 `stop backend`，用 `pg_restore --clean --if-exists` 恢复最近备份，再启动旧版本（详见 production-deployment.md「备份与恢复」）。
- **严禁** `down -v`（删库删证书）。

## 五、演练结论与已知注意点（2026-08-02 本地演练）

- 从零（`cp .env.prod.example` 起）到 6 服务全 healthy + HTTPS 登录成功，实测 < 15 分钟（含镜像构建），30 分钟目标有充分余量。
- `BACKUP_SCHEDULE` 仅是配置元数据，后端**没有内置定时器**：每日自动备份靠宿主机 crontab（上文第 4 步），勿以为配了变量就有自动备份。
- 后端镜像已内置 postgresql-client-16（`pg_dump`/`pg_restore`/`psql`），`/api/v1/ops/backups` 手工备份可用（演练中发现缺失并已修复）。
- `BACKUP_STORAGE_BUCKET` 为空时对象存储备份会失败，上线后尽快在 `.env` 补齐并重启 backend。
- Let's Encrypt 正式签发依赖真实 DNS + 公网 80/443，为本地演练唯一未覆盖项；异常按 production-deployment.md「常见问题」第一条排查（先切 staging CA 联调）。
- 登录接口 body 为 `{"account": "...", "password": "..."}`（account=邮箱或手机号，不是 email 字段）。
