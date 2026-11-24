# GuliangIAM

A unified identity authentication and access management (IAM) platform: multi-tenant identity, authentication, authorization, protocol integration, commerce and AI gateway capabilities, together with a management console.

## Architecture

- **Backend**: a modular Go monolith following Clean Architecture (Domain / App / Infra / Transport / Platform). Routing uses chi v5, data access uses pgx/v5 with parameterized SQL (no ORM), PostgreSQL is the primary database, and dialect baselines are maintained for MySQL / GaussDB / KingBaseES (ADR-023).
- **Frontend**: React + Vite + TypeScript + Ant Design; the management console lives in `web/`.
- **Single source of truth**: declarative schemas, registries and route declarations drive all generated artifacts (DDL, migrations, OpenAPI, frontend types, registry rendering layer). Zero-diff gates guarantee that documentation, contracts and implementation stay consistent (ADR-022, ADR-012).
- **Technology stack versions** are governed by `go.mod` and `web/package.json` and are not written by hand in this document.

## Documentation

- [docs/README.md](docs/README.md)
- [docs/architecture/README.md](docs/architecture/README.md) (layering, data model, the three API surfaces, security, observability, multi-region, technology selection, architecture risk register)
- [docs/design/README.md](docs/design/README.md) (authentication / IAM / protocols / commerce / AI gateway / infrastructure / frontend)
- [docs/requirements/README.md](docs/requirements/README.md) / [docs/acceptance-criteria.md](docs/acceptance-criteria.md) / [docs/traceability-matrix.md](docs/traceability-matrix.md)
- [docs/operations/README.md](docs/operations/README.md) / [docs/adr/README.md](docs/adr/README.md)
- [docs/GuliangIAM-开发规范.md](docs/GuliangIAM-开发规范.md)

## Quick Start

```bash
# Backend
go build -o bin/manager ./

# Frontend
cd web && pnpm install && pnpm run build

# Local dependencies (PostgreSQL / Redis, etc.)
docker compose up -d
```

Deployment topologies, migration jobs, backup and rollback are documented in [Deployment](docs/operations/deployment.md); structural changes must follow [ADR-022](docs/adr/ADR-022-declarative-schema-versioned-migration.md).

## Database and Gates

**Runtime baseline**: PostgreSQL 18 is the only runtime database. During development, schema changes are applied explicitly by the developer via `go run ./cmd/migrate up`; on application startup, migrations are not replayed and seeds are not imported by default.

**Dialect verification tiers**: PR blocking covers only the PostgreSQL runtime gates; DDL and runtime checks for MySQL / GaussDB / KingbaseES and other dialects run in nightly or manually triggered workflows (ADR-023).

**Gates requiring a database**: `scripts/check-schema-vs-db.py` and `scripts/check-rls-policy-content.py` require `POSTGRES_DSN` (or `--dsn`); `scripts/check-sql-runtime.py` requires `SQL_CHECK_DSN` and `REQUIRE_SQL_RUNTIME_CHECK=1`. When no connection string is provided, these scripts skip according to their own definitions and emit a warning.

**Gate counts**: the number of gates and the number passing are not written by hand in the documentation; refer to the live output of `bash scripts/verify-docs.sh` and `python3 scripts/check-gate-coverage.py`.

## Contributing

- Contribution workflow and local gates: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security disclosure: [SECURITY.md](SECURITY.md)
- License: [LICENSE](LICENSE)