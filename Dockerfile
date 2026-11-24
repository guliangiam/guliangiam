# ============================================================
# GuliangIAM Multi-Stage Dockerfile
# Optimized for minimal image size with health check support
# ============================================================

# ---------- Stage 1: Build Frontend ----------
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder
WORKDIR /web

# Install pnpm via corepack
RUN corepack enable && corepack prepare pnpm@12.18.0 --activate

# Copy dependency manifests first for layer caching
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

# Copy source and build
COPY web/ ./
RUN NODE_OPTIONS="--max-old-space-size=4096" pnpm run build

# ---------- Stage 2: Build Backend ----------
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend-builder
WORKDIR /go/src/guliangiam

# Copy dependency manifests first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Frontend build output -> the canonical delivery path web/dist (手册 10.1).
# 此前这里是 web/build（CRA 时代路径），与手册、Vite outDir 以及 .goreleaser
# 三处不一致：镜像与发行包里放了一份没有任何进程会读取的前端产物。
COPY --from=frontend-builder /web/dist ./web/dist

# Build static binary with optimizations
# -trimpath: strip build-time file paths from binary
# -s -w: strip symbol table and debug info
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o server ./

# ---------- Stage 3: Production Runtime ----------
FROM alpine:3.22 AS production
LABEL maintainer="https://bglt.cn/"
ARG USER=guliangiam

# Install minimal runtime dependencies in single layer
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    sudo \
    && update-ca-certificates

# Create non-root user with sudo access
RUN adduser -D $USER -u 1000 \
    && echo "$USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER \
    && chmod 0440 /etc/sudoers.d/$USER \
    && mkdir -p /app/logs /app/conf \
    && chown -R $USER:$USER /app/logs /app/conf

USER 1000
WORKDIR /app

# Copy artifacts from build stages
COPY --from=backend-builder --chown=$USER:$USER /go/src/guliangiam/server ./server

# Health check: uses /healthz endpoint (registered in bootstrap/chi.go)
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -fsSL http://localhost:8000/healthz || exit 1

EXPOSE 8000
ENTRYPOINT ["/app/server"]

# ---------- Stage 4: All-in-One (Debian, for debugging) ----------
FROM debian:12-slim AS allinone
LABEL maintainer="https://bglt.cn/"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    lsof \
    && update-ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN adduser --disabled-password --gecos "" guliang -u 1000

WORKDIR /app
COPY --from=backend-builder --chown=1000:1000 /go/src/guliangiam/server ./server
COPY --from=backend-builder --chown=1000:1000 /go/src/guliangiam/docker-entrypoint.sh /docker-entrypoint.sh

# Health check for all-in-one variant
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -fsSL http://localhost:8000/healthz || exit 1

USER 1000
EXPOSE 8000
ENTRYPOINT ["/bin/bash"]
CMD ["/docker-entrypoint.sh"]
