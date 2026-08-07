#!/usr/bin/env bash
# TradeMind 生产一键部署脚本（docker-compose.prod.yml）
# 用法：
#   ./scripts/deploy-prod.sh                     # 拉最新代码 + 构建 + 启动 + 健康检查
#   ./scripts/deploy-prod.sh --no-pull           # 跳过 git pull（回滚到指定 commit 后重建时使用）
#   ./scripts/deploy-prod.sh --pre-upgrade-check # 仅执行升级前检查（全量备份 + 迁移预检），不部署
#                                                # 备份目录默认 /var/backups，可用 BACKUP_DIR=... 覆盖
# 额外挂载（如 BACKUP_S3_CA_BUNDLE 自签 CA）：写入 docker-compose.prod.override.yml（自动叠加）
#   或 COMPOSE_OVERRIDE_FILES=a.yml:b.yml ./scripts/deploy-prod.sh
# 前置：已 cp .env.prod.example .env 并填入必填项。详见 docs/production-deployment.md

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

COMPOSE_FILE=docker-compose.prod.yml
COMPOSE=(docker compose -f "$COMPOSE_FILE")
# 额外 compose override（如挂载 BACKUP_S3_CA_BUNDLE 自签 CA）：
# 存在 docker-compose.prod.override.yml 时自动叠加，保证重跑部署不丢挂载；
# 也可用 COMPOSE_OVERRIDE_FILES 指定多个文件（冒号分隔）。
OVERRIDE_FILES="${COMPOSE_OVERRIDE_FILES:-}"
if [ -z "$OVERRIDE_FILES" ] && [ -f docker-compose.prod.override.yml ]; then
  OVERRIDE_FILES=docker-compose.prod.override.yml
fi
if [ -n "$OVERRIDE_FILES" ]; then
  IFS=':' read -r -a _override_arr <<< "$OVERRIDE_FILES"
  for f in "${_override_arr[@]}"; do
    [ -f "$f" ] || { printf '\033[1;31m[deploy][失败]\033[0m compose override 文件不存在: %s\n' "$f" >&2; exit 1; }
    COMPOSE+=(-f "$f")
  done
fi
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-300}"
NO_PULL=0
PRE_UPGRADE_CHECK=0
BACKUP_DIR="${BACKUP_DIR:-/var/backups}"

for arg in "$@"; do
  case "$arg" in
    --no-pull) NO_PULL=1 ;;
    --pre-upgrade-check) PRE_UPGRADE_CHECK=1 ;;
    *) echo "未知参数: $arg" >&2; exit 2 ;;
  esac
done

log() { printf '\n\033[1;34m[deploy]\033[0m %s\n' "$*"; }
fail() { printf '\n\033[1;31m[deploy][失败]\033[0m %s\n' "$*" >&2; exit 1; }

# ---------- 0. 前置检查 ----------
command -v docker >/dev/null || fail "未安装 docker"
docker compose version >/dev/null 2>&1 || fail "未安装 docker compose v2"
[ -f .env ] || fail "缺少 .env：请先 cp .env.prod.example .env 并填入必填项"

# 必填项非空检查（不打印值）
REQUIRED_VARS=(DOMAIN ACME_EMAIL ADMIN_PUBLIC_URL API_PUBLIC_URL APP_MASTER_KEY JWT_SECRET PAGINATION_CURSOR_SIGNING_KEY POSTGRES_PASSWORD DB_PASSWORD ADMIN_BOOTSTRAP_EMAIL ADMIN_BOOTSTRAP_PASSWORD STORAGE_PROVIDER CORS_ALLOWED_ORIGINS)
missing=()
for var in "${REQUIRED_VARS[@]}"; do
  value="$(grep -E "^${var}=" .env | head -n1 | cut -d= -f2- || true)"
  [ -n "$value" ] || missing+=("$var")
done
[ ${#missing[@]} -eq 0 ] || fail "以下 .env 必填项为空：${missing[*]}"

# 弱示例值拦截
if grep -qE '^APP_MASTER_KEY=a{64}$' .env; then
  fail "APP_MASTER_KEY 仍是示例值，请用 openssl rand -hex 32 生成"
fi

# ---------- 0.5 升级前检查（--pre-upgrade-check：仅备份 + 预检，不部署）----------
if [ "$PRE_UPGRADE_CHECK" -eq 1 ]; then
  log "升级前检查：全量备份 + 迁移预检（对当前运行中的旧版本执行）"
  cid="$("${COMPOSE[@]}" ps -q postgres)"
  [ -n "$cid" ] || fail "postgres 容器未运行，无法执行升级前检查"
  PG_USER="$(grep -E '^POSTGRES_USER=' .env | head -n1 | cut -d= -f2- || true)"; PG_USER="${PG_USER:-trademind}"
  PG_DB="$(grep -E '^POSTGRES_DB=' .env | head -n1 | cut -d= -f2- || true)"; PG_DB="${PG_DB:-trademind}"

  mkdir -p "$BACKUP_DIR" 2>/dev/null || fail "无法创建备份目录 $BACKUP_DIR（可用 BACKUP_DIR=路径 覆盖）"
  BACKUP_FILE="$BACKUP_DIR/trademind-pre-upgrade-$(date +%F-%H%M%S).dump"
  log "全量备份到 $BACKUP_FILE"
  "${COMPOSE[@]}" exec -T postgres pg_dump -U "$PG_USER" -d "$PG_DB" -Fc > "$BACKUP_FILE" \
    || fail "pg_dump 备份失败"
  [ -s "$BACKUP_FILE" ] || fail "备份文件为空：$BACKUP_FILE"

  log "预检：同租户重复订单号（会中断 R95 唯一索引迁移）"
  DUP="$("${COMPOSE[@]}" exec -T postgres psql -U "$PG_USER" -d "$PG_DB" -At -c \
    "SELECT tenant_id, order_no, COUNT(*) FROM orders WHERE deleted_at IS NULL GROUP BY tenant_id, order_no HAVING COUNT(*) > 1;")" \
    || fail "预检 SQL 执行失败"
  if [ -n "$DUP" ]; then
    printf '%s\n' "$DUP" >&2
    fail "预检失败：存在同租户重复订单号，先按 docs/upgrade-guide.md「迁移中断处置」清理再升级"
  fi
  log "升级前检查通过：备份 $BACKUP_FILE，无同租户重复订单号。可继续 checkout 目标版本后执行部署。"
  exit 0
fi

# ---------- 1. 拉取代码 ----------
if [ "$NO_PULL" -eq 1 ]; then
  log "跳过 git pull（--no-pull），当前 commit: $(git rev-parse --short HEAD)"
else
  log "拉取最新代码"
  git pull --ff-only
  log "当前 commit: $(git rev-parse --short HEAD)"
fi

# ---------- 2. 校验 compose 配置 ----------
log "校验 compose 配置（compose 命令：${COMPOSE[*]}）"
"${COMPOSE[@]}" config >/dev/null || fail "compose 配置校验失败"

# ---------- 3. 构建并启动 ----------
log "构建镜像（可能需要数分钟）"
"${COMPOSE[@]}" build

log "启动/更新服务（数据库迁移由 backend 启动时自动执行）"
"${COMPOSE[@]}" up -d --remove-orphans

# ---------- 4. 健康检查 ----------
log "等待所有服务健康（最长 ${HEALTH_TIMEOUT_SECONDS}s）"
deadline=$(( $(date +%s) + HEALTH_TIMEOUT_SECONDS ))
services=(postgres redis collector backend admin caddy)
while true; do
  unhealthy=()
  for svc in "${services[@]}"; do
    cid="$("${COMPOSE[@]}" ps -q "$svc")"
    [ -n "$cid" ] || { unhealthy+=("$svc(未启动)"); continue; }
    status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$cid")"
    case "$status" in
      healthy|running) ;;
      *) unhealthy+=("$svc($status)") ;;
    esac
  done
  if [ ${#unhealthy[@]} -eq 0 ]; then
    break
  fi
  if [ "$(date +%s)" -ge "$deadline" ]; then
    "${COMPOSE[@]}" ps
    fail "健康检查超时，未就绪：${unhealthy[*]}。排查：${COMPOSE[*]} logs -f <service>"
  fi
  sleep 5
done

# ---------- 5. 端到端探活（经 Caddy HTTPS）----------
DOMAIN_VALUE="$(grep -E '^DOMAIN=' .env | head -n1 | cut -d= -f2-)"
log "经 Caddy 探活 https://${DOMAIN_VALUE}/health-backend"
# --resolve 使探活不依赖公网 DNS；-k 兼容本地自签/staging 证书（正式证书同样通过）
if curl -fsSk --resolve "${DOMAIN_VALUE}:443:127.0.0.1" "https://${DOMAIN_VALUE}/health-backend" >/dev/null; then
  log "HTTPS 探活通过"
else
  fail "HTTPS 探活失败。排查：${COMPOSE[*]} logs -f caddy"
fi

"${COMPOSE[@]}" ps
log "部署完成：https://${DOMAIN_VALUE}"
