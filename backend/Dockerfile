# TradeMind API — multi-stage build (do not COPY .env into the image; inject at runtime).
FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/trademind-backend ./cmd/server

FROM debian:bookworm-slim AS runtime

# postgresql-client-16（PGDG）：备份/恢复模块调用 pg_dump/pg_restore/psql，版本须 ≥ 服务端 PostgreSQL 16
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata curl gnupg \
    fonts-noto-cjk fonts-dejavu-core fontconfig \
  && curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/keyrings/pgdg.gpg \
  && echo "deb [signed-by=/usr/share/keyrings/pgdg.gpg] http://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
  && apt-get update \
  && apt-get install -y --no-install-recommends postgresql-client-16 \
  && apt-get purge -y gnupg \
  && apt-get autoremove -y \
  && fc-cache -f \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/trademind-backend /app/trademind-backend

ENV TZ=UTC

EXPOSE 8080

CMD ["/app/trademind-backend"]
