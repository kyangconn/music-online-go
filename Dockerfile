# syntax=docker/dockerfile:1.7

FROM node:24-alpine3.24 AS frontend-builder

WORKDIR /src/web

RUN corepack enable

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store \
    pnpm config set store-dir /pnpm/store && \
    pnpm install --frozen-lockfile

COPY web/index.html web/tsconfig.json web/tsconfig.app.json web/tsconfig.node.json web/vite.config.ts ./
COPY web/public ./public
COPY web/src ./src

RUN pnpm build

FROM golang:1.26-alpine3.24 AS backend-builder

ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=1970-01-01T00:00:00Z

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend-builder /src/cmd/server/dist ./cmd/server/dist

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w \
        -X github.com/kyangconn/music-online-go/internal/version.Version=${VERSION} \
        -X github.com/kyangconn/music-online-go/internal/version.Commit=${VCS_REF} \
        -X github.com/kyangconn/music-online-go/internal/version.BuildTime=${BUILD_DATE}" \
      -o /out/music-online ./cmd/server

FROM alpine:3.24.1 AS runtime

ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=1970-01-01T00:00:00Z

LABEL org.opencontainers.image.title="Music Online" \
      org.opencontainers.image.description="Self-hosted music management platform" \
      org.opencontainers.image.source="https://github.com/kyangconn/music-online-go" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S -g 10001 music-online && \
    adduser -S -D -H -h /data -s /sbin/nologin -u 10001 -G music-online music-online && \
    mkdir -p /app /data/uploads /etc/music-online && \
    chown -R 10001:10001 /app /data

COPY --from=backend-builder --chown=10001:10001 /out/music-online /app/music-online

WORKDIR /data
USER 10001:10001

VOLUME ["/data"]
EXPOSE 8080
STOPSIGNAL SIGTERM

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -q -T 3 --spider "http://127.0.0.1:${SERVER_PORT:-8080}/ready" || exit 1

ENTRYPOINT ["/app/music-online"]
