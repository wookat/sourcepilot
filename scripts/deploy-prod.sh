#!/usr/bin/env bash
# TradeMind 生产一键部署脚本（docker-compose.prod.yml）
# 用法：
#   ./scripts/deploy-prod.sh            # 拉最新代码 + 构建 + 启动 + 健康检查
#   ./scripts/deploy-prod.sh --no-pull  # 跳过 git pull（回滚到指定 commit 后重建时使用）
# 前置：已 cp .env.prod.example .env 并填入必填项。详见 docs/production-deployment.md

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

COMPOSE_FILE=docker-compose.prod.yml
COMPOSE=(docker compose -f "$COMPOSE_FILE")
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-300}"
NO_PULL=0

for arg in "$@"; do
  case "$arg" in
    --no-pull) NO_PULL=1 ;;
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

# ---------- 1. 拉取代码 ----------
if [ "$NO_PULL" -eq 1 ]; then
  log "跳过 git pull（--no-pull），当前 commit: $(git rev-parse --short HEAD)"
else
  log "拉取最新代码"
  git pull --ff-only
  log "当前 commit: $(git rev-parse --short HEAD)"
fi

# ---------- 2. 校验 compose 配置 ----------
log "校验 compose 配置"
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
