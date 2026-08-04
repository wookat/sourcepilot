# 生产部署 SOP（Docker Compose + Caddy HTTPS）

本文是 TradeMind 从零到公网上线的标准操作流程，面向不熟悉本项目的运维人员。目标：拿到服务器与域名后 **30 分钟内上线**。

上线所需资源清单、逐步命令、回滚方案与上线后验证清单见 [production-launch-checklist.md](production-launch-checklist.md)。本地试用 Docker 部署见 [docker-deployment.md](docker-deployment.md)；裸机（systemd + nginx）部署见 `deploy/nginx/`、`deploy/systemd/` 与 `.env.production.example`。

## 架构

```text
公网 80/443
   ↓
Caddy（自动 Let's Encrypt HTTPS，域名走 DOMAIN 环境变量）
   ↓
admin（nginx：静态资源 + /api、/static 转发 backend）
   ↓
backend（Go API，仅编排网络内可达）
   ├── postgres（数据卷持久化，不暴露公网）
   ├── redis（数据卷持久化，不暴露公网）
   └── collector（Playwright 采集，不暴露公网）
```

仅 Caddy 监听公网端口；backend / collector / postgres / redis / admin 均不映射宿主机端口。

## 一、前置条件

| 项目 | 要求 |
| --- | --- |
| 云服务器 | 2 vCPU / 4 GB 内存起步（Playwright 采集吃内存），40 GB+ 磁盘，Ubuntu 22.04+ / Debian 12+ |
| 软件 | Docker Engine 24+ 与 Docker Compose v2（`curl -fsSL https://get.docker.com | sh`） |
| 域名 | 一个域名（如 `trademind.example.com`），DNS **A 记录**指向服务器公网 IP，并等待解析生效（`dig +short 域名` 返回服务器 IP） |
| 防火墙/安全组 | 放行 TCP 80、TCP 443、UDP 443（HTTP/3）；SSH 端口按需 |

> 中国大陆服务器需完成 ICP 备案后 80/443 才可对外服务；未备案可先用海外服务器。

## 二、首次上线（约 30 分钟）

```bash
# 1. 获取代码
git clone https://github.com/wookat/sourcepilot.git trademind
cd trademind

# 2. 准备环境变量
cp .env.prod.example .env
# 编辑 .env，填入所有「必填」项：
#   DOMAIN、ACME_EMAIL、ADMIN_PUBLIC_URL、API_PUBLIC_URL
#   APP_MASTER_KEY（openssl rand -hex 32）
#   JWT_SECRET（openssl rand -base64 48）
#   PAGINATION_CURSOR_SIGNING_KEY（openssl rand -hex 32）
#   POSTGRES_PASSWORD 与 DB_PASSWORD（相同的强随机值）
#   ADMIN_BOOTSTRAP_EMAIL / ADMIN_BOOTSTRAP_PASSWORD（密码 ≥12 位且非常见弱口令）
#   STORAGE_PROVIDER（cos/oss/s3/r2/minio，生产禁止 local）
#   CORS_ALLOWED_ORIGINS（与 ADMIN_PUBLIC_URL 一致）
chmod 600 .env

# 3. 一键部署（构建 + 启动 + 迁移 + 健康检查）
./scripts/deploy-prod.sh
```

脚本结束输出「部署完成」后，浏览器访问 `https://<DOMAIN>`，用 `.env` 中的引导管理员账号登录，**立即修改密码**，并在系统设置中配置 AI Provider、存储等密钥（均加密存储，不要写入 `.env` 提交）。

数据库结构迁移由 backend 容器启动时自动执行，无需手工建表。

## 三、日常升级

```bash
cd trademind
./scripts/deploy-prod.sh   # git pull + 重建镜像 + 滚动重启 + 健康检查
```

升级不会清空数据：PostgreSQL / Redis / 上传文件 / 证书均在命名数据卷中持久化。

目标版本包含数据库迁移（发布说明标注「数据库」影响）时，先执行 `./scripts/deploy-prod.sh --pre-upgrade-check`（全量备份 + 迁移预检，不部署；备份目录默认 `/var/backups`，可用 `BACKUP_DIR=...` 覆盖），再按 [upgrade-guide.md](upgrade-guide.md) 升级。

## 四、回滚

```bash
cd trademind
git log --oneline -10               # 找到上一个正常 commit
git checkout <上一个正常commit>
./scripts/deploy-prod.sh --no-pull  # 用该版本重建并启动
# 确认恢复后，切回分支等待修复版本：git checkout <分支名>
```

> 回滚仅回退代码与镜像；若新版本已做不兼容的数据库迁移，需配合下文备份恢复。

## 五、备份与恢复

### 备份（建议每日 cron）

```bash
# PostgreSQL 全量备份（custom 格式，可压缩、可选表恢复）
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U trademind -d trademind -Fc > /var/backups/trademind-$(date +%F).dump

# 上传文件卷备份
docker run --rm -v trademind-prod_trademind_prod_uploads:/data -v /var/backups:/backup \
  busybox tar czf /backup/uploads-$(date +%F).tar.gz -C /data .
```

crontab 示例（每日 03:00，保留 7 天）：

```cron
0 3 * * * cd /path/to/trademind && docker compose -f docker-compose.prod.yml exec -T postgres pg_dump -U trademind -d trademind -Fc > /var/backups/trademind-$(date +\%F).dump && find /var/backups -name 'trademind-*.dump' -mtime +7 -delete
```

### 恢复

```bash
docker compose -f docker-compose.prod.yml stop backend
# 注意：`docker compose exec -T ... < 文件` 的 stdin 重定向在部分 Compose 版本（实测 2.32）
# 会损坏二进制流（pg_restore 报 did not find magic string），恢复请用 `docker exec -i` 直连容器：
docker exec -i "$(docker compose -f docker-compose.prod.yml ps -q postgres)" \
  pg_restore -U trademind -d trademind --clean --if-exists < /var/backups/trademind-<日期>.dump
docker compose -f docker-compose.prod.yml start backend
```

建议每季度做一次恢复演练。备份文件含业务数据，妥善保管、异地一份。

## 六、常用命令

```bash
docker compose -f docker-compose.prod.yml ps                # 状态
docker compose -f docker-compose.prod.yml logs -f backend   # 日志（caddy/admin/collector/postgres/redis 同理）
docker compose -f docker-compose.prod.yml restart backend   # 重启单个服务
docker compose -f docker-compose.prod.yml down              # 停止（保留数据卷）
```

**严禁**在生产随意执行 `down -v`（会删除数据库、上传文件与证书数据卷）。

## 七、常见问题

| 现象 | 排查 |
| --- | --- |
| 证书申请失败 / 浏览器提示不安全 | ① `dig +short $DOMAIN` 是否指向本机公网 IP；② 安全组/防火墙 80、443 是否放行；③ `logs -f caddy` 看 ACME 报错；④ 反复失败先切 `ACME_CA` 为 staging 联调，成功后移除该变量并删除 caddy 数据卷内 staging 证书（`docker volume rm` 前先 `down`，或进容器删除 `/data/caddy/certificates` 下 staging 目录）再正式签发，避免触发 Let's Encrypt 限额 |
| backend 启动即退出 | `logs backend`：APP_ENV=production 下弱密钥/缺失必填项会 fail-fast，按报错补齐 `.env` |
| 登录后接口 401/403 | JWT_SECRET 更换会使旧 token 失效，重新登录即可 |
| 采集任务一直 pending | `logs collector` 与 `logs backend`；确认 `ADMIN_BOOTSTRAP_TENANT_ID` ≥ 1；1688 登录态需按 [docker-deployment.md](docker-deployment.md) 的浏览器 Profile 说明预置 |
| 磁盘增长过快 | `docker system prune -f` 清理悬空镜像；检查备份保留策略 |
| 端口被占用 | `ss -tlnp | grep -E ':80|:443'`，停掉宿主机上占用 80/443 的 nginx/apache |

## 八、验证边界说明（开发环境已验证的范围）

本部署包已在开发 VM 上完成如下验证：`DOMAIN=localhost` 时全栈（caddy + postgres + redis + backend + admin + collector）可通过 `docker compose -f docker-compose.prod.yml up` 启动至全部 healthy，Caddy 使用内部自签 CA 提供 HTTPS，`https://localhost/health-backend` 探活通过，管理端页面可访问。

**未在真实公网验证的部分**（需服务器 + 域名到位后首次上线时确认）：Let's Encrypt 正式证书签发（依赖真实域名 DNS 解析与公网 80/443 可达）、HTTP/3（UDP 443）公网连通性。该部分逻辑由 Caddy 官方实现承担，风险低；若签发异常按「常见问题」第一条排查。
