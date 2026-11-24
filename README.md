# GuliangIAM

> Unified multi-tenant Identity and Access Management platform — authentication, authorization, protocol gateway, commerce and AI gateway, plus a React management console.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.27%2B-00ADD8)](https://go.dev/doc/go1.27)
[![React](https://img.shields.io/badge/react-19%2B-61dafb.svg)](https://react.dev/)
[![Ant Design](https://img.shields.io/badge/ant--design-6-1677ff.svg)](https://ant.design/)
[![PostgreSQL](https://img.shields.io/badge/postgresql-18-4169e1.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/redis-8.2-ff69b4.svg)](https://redis.io/)
[![Release](https://img.shields.io/github/v/tag/guliangiam/guliangiam?label=release)](https://github.com/guliangiam/guliangiam/releases)

---

## ✨ Highlights

- **Modular Go monolith** (Clean Architecture: Domain → App → Infra → Transport → Platform). Routing with [chi v5](https://go-chi.io/), data access with [pgx/v5](https://github.com/jackc/pgx) — parameterized SQL, no ORM.
- **Complete protocol coverage**: OAuth 2.1, OpenID Connect, SAML 2.0, CAS, SCIM 2.0, LDAP, SMTP/SMS login.
- **Multi-tenant authorization**: Rebac RBAC + Casbin enforcement, per-organizations roles / permissions / policies.
- **Declarative single source of truth**: DDL, migrations, OpenAPI, frontend types and registry rendering all generated from `contracts/schema/*.yaml`. Zero-diff gates guarantee docs ↔ contracts ↔ implementation never drift (ADR-022, ADR-012).
- **AI gateway**: Multi-model upstream abstraction (Anthropic, OpenAI, custom), tenant-level quota, request audit, cost tracking.
- **Commercial subsystem**: Product / plan / pricing / coupon catalog tied to IAM organizations.
- **Production-grade security**: Argon2id password hashing, AES-256-GCM / SM4-GCM field encryption (no plaintext fallback), JWT key ring, SSRF guard, RLS, CSRF, rate limiting.
- **Dialect baseline**: PostgreSQL 18 is the only runtime baseline; MySQL / openGauss / KingBaseES dialects are generated and verified but follow the runtime gates documented in ADR-023.

## 🧱 Tech Stack

| Layer | Choice |
|-------|--------|
| Language | Go 1.27+ |
| Backend | chi · pgx/v5 · go-redis · Casbin · go-sqlmock |
| Frontend | React 19 · Vite · TypeScript · Ant Design 6 |
| Database | PostgreSQL 18 (runtime baseline) · MySQL (dialect) |
| Cache | Redis (multi-replica fairness; process-memory fallback is safe-by-default) |
| Observability | OpenTelemetry · Prometheus / Grafana |
| Build | GoReleaser · semantic-release · pnpm |
| Deploy | Docker Compose · Kubernetes (manifest in `k8s.yaml`) |

## 🚀 Quick Start (2 steps)

### 1. Bring up dependencies

```bash
docker compose up -d
```

Starts **PostgreSQL + Redis + MailHog** (SMTP dev sink). Redis is **enabled by default** in the compose manifest — the process still boots without it, but enabling it is recommended so HA features (session revocation sharing, fair rate-limiting, AI quota) exercise the shared path out of the box.

### 2. Boot the application

```bash
# Pre-boot: apply schema + import built-in admin / demo fixtures
go run ./cmd/migrate up --seed

# Then start the server (default listen :8000)
go run .
```

Default admin after `--seed`: `admin` / `Guliang@2026` (development sentinel; change it on first login).

### Frontend

```bash
cd web && pnpm install && pnpm run dev    # http://localhost:5173
```

The production binary (via `docker build`) bundles `web/dist` into the same image — no separate static host.

## 🔐 Production Mandatory Configuration

The process refuses to start (`panic` or startup `503`) when any of the values below is missing or malformed. Settings **not** listed here (rate limits, provider credentials, captcha, SLS webhooks, payment channels, AI gateway upstreams, org branding) are either safe-by-default or editable from the admin console after first boot.

| Name | Scope | Notes |
|------|-------|-------|
| `DB_DSN` (or `POSTGRES_DSN` / `DATABASE_URL`) | Every env | Primary PostgreSQL 18 connection string |
| `ENCRYPTION_KEY_CURRENT` (or legacy `ENCRYPTION_KEY`) | Every env | Field-level encryption key — AES-256-GCM (32 B base64) or SM4-GCM (16 B base64); `openssl rand -base64 32` |
| `ISSUER_URL` | production | Public JWT issuer (absolute HTTPS URL) |
| `METRICS_SCRAPE_TOKEN` | production | Bearer token for `/metrics` and `/debug/pprof` |
| `PASSWORD_HISTORY_PEPPER` | production | Must be non-empty and not equal to the built-in dev sentinel |
| `JWT_SIGNING_KEYS_FILE` | production | PEM key-ring path; production refuses process-local ephemeral keys |
| `BOOTSTRAP_ADMIN_PASSWORD` | production (when seeding) | 12–128 chars, upper + lower + digit + special |

Plus conditional requirements when you enable a capability — e.g. `VAULT_ADDR` / `VAULT_TOKEN` when `KMS_PROVIDER=vault`, five SM2 PEM files when `GM_TLS_ENABLED=true`, provider credentials when `STORAGE_TYPE` is non-`local`. Full table and Docker step-by-step are in `docs/operations/deployment.md`.

## 🐘 Redis

Redis is **not a hard prerequisite** — every subsystem falls back to in-process memory when it is absent. That said, it is the only cross-replica store for:

- Session cache revocation / shared invalidation
- Fair rate-limiting (single-instance bucket otherwise)
- AI gateway quota metering
- CAPTCHA answer store across replicas
- SAML replay cache / HIBP k-anonymity range cache / DPoP nonce cache / audit denial recorder

Redis is **enabled by default** in `docker compose up -d`. Production operators should set:

```dotenv
RATE_LIMIT_REDIS_ADDR=redis:6379          # dedicated rate-limit Redis if you split
RATE_LIMIT_REDIS_DB=1                     # to isolate quota/limiting traffic
REDIS_ADDR=redis:6379
REDIS_PASSWORD=...                        # empty in compose dev
REDIS_DB=0                                # session cache / captcha / audit
SESSION_CACHE_DRIVER=redis                # omit or set to redis; memory is also valid
CAPTCHA_STORE_DRIVER=redis
```

Single-replica deployments may safely omit every `REDIS_*` variable — the service boots with memory fallbacks and logs an informational warning at startup.

## 📚 Documentation

- [docs/README.md](docs/README.md) — master document map (layering · design · requirements · ADR index · operations)
- [docs/architecture/README.md](docs/architecture/README.md) — Clean Architecture, data model, security model, observability
- [docs/design/README.md](docs/design/README.md) — auth / IAM / protocols / commerce / AI gateway / frontend
- [docs/operations/deployment.md](docs/operations/deployment.md) — topology, migration jobs, backup and rollback
- [docs/GuliangIAM-开发规范.md](docs/GuliangIAM-开发规范.md) — engineering gates and authoring contract

## 🤝 Contributing

Conventional Commits, all gates must pass, no generated files hand-edited, no production secrets in commits. Full workflow in [CONTRIBUTING.md](CONTRIBUTING.md).

```bash
python3 -m pip install -r scripts/requirements.txt
GOCACHE=$PWD/.gotmp/gocache PNPM_HOME=$PWD/.gotmp/pnpmhome \
  RUN_CODE_CHECKS=1 bash scripts/verify-docs.sh
go build ./... && go vet ./... && go test ./...
bash scripts/check-layers.sh
```

## 📄 License

[Apache License 2.0](LICENSE). Copyright © 2026 The Beijing Gu Liang Technology Co., Ltd.
