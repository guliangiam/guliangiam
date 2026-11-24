# ============================================================
# GuliangIAM Multi-Stage Dockerfile
#
# Stages:
#   frontend-builder — Vite build (Node 24 + pnpm 12)
#   backend-builder  — Go static binary (CGO disabled, all platform variants)
#   production       — minimal runtime (alpine 3.22 + tini + curl + ca-certificates)
#   debug            — Debian all-in-one for incident triage (bash + lsof)
#
# Image hardening (production stage):
#   * tini as PID 1 for proper signal handling / zombie reaping
#   * Non-root runtime user (uid 1000, no sudo)
#   * Single RUN layer for apk installs + user creation
#   * No shell, no package manager in final image (apart from ca-certificates/tzdata/curl/tini)
#
# Entrypoint: tini wraps /app/server. Migrations run via `docker run … /app/server migrate up`.
# ============================================================

# ---------- Stage 1: Build Frontend ----------
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder
WORKDIR /web

RUN corepack enable && corepack prepare pnpm@12.18.0 --activate

# Dependency manifests first for layer caching
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN NODE_OPTIONS="--max-old-space-size=4096" pnpm run build

# ---------- Stage 2: Build Backend ----------
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend-builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Manual copy of the Vite output — matches webpack/vite outDir of project root "web/dist"
# (手册 §10.1 前端交付路径). This must happen before `go build` because the backend
# embeds or references web/dist paths.
COPY --from=frontend-builder /web/dist ./web/dist

# Static cross-compile — CGO disabled, all Go runtime paths are pure Go.
# -trimpath strips build-time file paths; -s -w strips symbol table and debug info.
ARG TARGETARCH=amd64
ARG TARGETOS=linux
ARG LDFLAGS="-trimpath -s -w -X main.BuildArch=${TARGETARCH} -X main.BuildOS=${TARGETOS}"
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="${LDFLAGS}" -o server ./

# ---------- Stage 3: Production Runtime ----------
FROM alpine:3.22 AS production
LABEL org.opencontainers.image.title="GuliangIAM" \
      org.opencontainers.image.description="Unified multi-tenant Identity & Access Management" \
      org.opencontainers.image.source="https://github.com/guliangiam/guliangiam" \
      org.opencontainers.image.licenses="Apache-2.0"

# Runtime deps — tini = PID 1 init (signal handling), curl = HEALTHCHECK, ca-certificates = TLS, tzdata = TZ
# Single RUN layer keeps image small.
RUN apk add --no-cache tini curl ca-certificates tzdata \
    && update-ca-certificates \
    && adduser -D -u 1000 -h /app -s /sbin/nologin guliangiam \
    && mkdir -p /app/logs /app/conf /app/uploads \
    && chown -R guliangiam:guliangiam /app \
    && rm -rf /tmp/* /var/cache/apk/*

WORKDIR /app
USER guliangiam

COPY --from=backend-builder --chown=guliangiam:guliangiam /src/server /app/server

EXPOSE 8000

# tini as PID 1 — forwards SIGTERM/SIGINT to /app/server and reaps zombies
ENTRYPOINT ["/sbin/tini", "--", "/app/server"]
HEALTHCHECK --interval=20s --timeout=3s --start-period=15s --start-interval=5s --retries=3 \
    CMD curl -fsSL http://127.0.0.1:8000/healthz || exit 1

# ---------- Stage 4: Debug (all-in-one, Debian) ----------
FROM debian:12-slim AS debug
LABEL org.opencontainers.image.title="GuliangIAM (debug)" \
      org.opencontainers.image.description="Debian-based image for incident triage" \
      org.opencontainers.image.source="https://github.com/guliangiam/guliangiam"

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl tini lsof procps \
    && update-ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && adduser --disabled-password --gecos "" guliangiam -u 1000 \
    && mkdir -p /app/logs /app/conf /app/uploads \
    && chown -R guliangiam:guliangiam /app

WORKDIR /app
USER guliangiam

COPY --from=backend-builder --chown=1000:1000 /src/server /app/server

EXPOSE 8000
ENTRYPOINT ["/usr/bin/tini", "--", "/app/server"]
HEALTHCHECK --interval=20s --timeout=3s --start-period=15s --start-interval=5s --retries=3 \
    CMD curl -fsSL http://127.0.0.1:8000/healthz || exit 1
