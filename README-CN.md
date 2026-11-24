# GuliangIAM

统一身份认证与访问管理（IAM）平台：多租户身份、认证、授权、协议接入、商业与 AI 网关能力，配套管理控制台。

## 架构

- **后端**：Go 模块化单体（Clean Architecture：Domain / App / Infra / Transport / Platform），路由 chi v5，数据访问 pgx/v5 参数化 SQL（无 ORM），PostgreSQL 为主数据库，支持 MySQL / GaussDB / KingBaseES 方言基线（ADR-023）。
- **前端**：React + Vite + TypeScript + Ant Design，管理控制台位于 `web/`。
- **唯一事实源**：声明式 Schema、注册表与路由声明驱动生成物（DDL、迁移、OpenAPI、前端类型、注册表渲染层），零差异门禁保障文档/契约/实现一致（ADR-022、ADR-012）。

技术栈版本以 `go.mod`、`web/package.json` 为准，不在本文手写。

## 文档

- [文档体系与写作规范](docs/README.md)
- [架构总纲](docs/architecture/README.md)（分层、数据模型、API 三面、安全、可观测性、多区域、技术选型、架构风险登记）
- [按域设计](docs/design/README.md)（认证 / IAM / 协议 / 商业 / AI 网关 / 基础设施 / 前端）
- [需求](docs/requirements/README.md) / [验收条件](docs/acceptance-criteria.md) / [追溯矩阵](docs/traceability-matrix.md)
- [运维](docs/operations/README.md) / [ADR 决策索引](docs/adr/README.md)
- [开发规范（工程门禁口径）](docs/GuliangIAM-开发规范.md)

## 快速开始

```bash
# 后端
go build -o bin/manager ./

# 前端
cd web && pnpm install && pnpm run build

# 本地依赖（PostgreSQL / Redis 等）
docker compose up -d
```

部署拓扑、迁移 Job、备份与回滚见[运维手册](docs/operations/deployment.md)；结构变更遵循 [ADR-022](docs/adr/ADR-022-declarative-schema-versioned-migration.md)。

## 数据库与门禁

- **运行期基线**：PostgreSQL 18 是唯一运行期数据库。开发期结构变更由开发者显式执行 `go run ./cmd/migrate up`；应用启动默认不重放迁移、不导入种子。
- **方言验证分层**：PR 阻断只覆盖 PostgreSQL 运行期门禁；MySQL / GaussDB / KingbaseES 等方言的 DDL 与运行期检查在夜间或手动工作流执行（ADR-023）。
- **需要数据库的门禁**：`scripts/check-schema-vs-db.py` 与 `scripts/check-rls-policy-content.py` 需要 `POSTGRES_DSN`（或 `--dsn`），`scripts/check-sql-runtime.py` 需要 `SQL_CHECK_DSN` 且 `REQUIRE_SQL_RUNTIME_CHECK=1`；未提供连接串时按脚本定义跳过并输出告警。
- **门禁计数**：门禁数量与通过数不在文档中手写，以 `bash scripts/verify-docs.sh` 和 `python3 scripts/check-gate-coverage.py` 的实时输出为准。

## 参与

- 贡献流程与本地门禁：[CONTRIBUTING.md](CONTRIBUTING.md)
- 安全披露：[SECURITY.md](SECURITY.md)
- 许可证：[LICENSE](LICENSE)
