# TradeMind API — multi-stage build (do not COPY .env into the image; inject at runtime).
FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/trademind-backend ./cmd/server

FROM debian:bookworm-slim AS runtime

# postgresql-client-16 匹配 compose 中的 postgres:16（pg_dump 不支持导出更高版本的服务端），来自 PGDG 官方源。
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata curl \
    fonts-noto-cjk fonts-dejavu-core fontconfig \
  && install -d /usr/share/postgresql-common/pgdg \
  && curl -fsSL -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc https://www.postgresql.org/media/keys/ACCC4CF8.asc \
  && echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] http://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
  && apt-get update \
  && apt-get install -y --no-install-recommends postgresql-client-16 \
  && fc-cache -f \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/trademind-backend /app/trademind-backend

ENV TZ=UTC

EXPOSE 8080

CMD ["/app/trademind-backend"]
