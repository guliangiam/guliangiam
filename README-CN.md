# GuliangIAM

> 统一身份认证与访问管理（IAM）平台 —— 多租户身份、认证、授权、协议网关、商业与 AI 网关能力，配套 React 管理控制台。

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.27%2B-00ADD8)](https://go.dev/doc/go1.27)
[![React](https://img.shields.io/badge/react-19%2B-61dafb.svg)](https://react.dev/)
[![Ant Design](https://img.shields.io/badge/ant--design-6-1677ff.svg)](https://ant.design/)
[![PostgreSQL](https://img.shields.io/badge/postgresql-18-4169e1.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/redis-8.2-ff69b4.svg)](https://redis.io/)
[![Release](https://img.shields.io/github/v/tag/guliangiam/guliangiam?label=release)](https://github.com/guliangiam/guliangiam/releases)

---

## ✨ 核心亮点

- **Go 模块化单体**（Clean Architecture：Domain → App → Infra → Transport → Platform），路由 [chi v5](https://go-chi.io/)、数据访问 [pgx/v5](https://github.com/jackc/pgx) 参数化 SQL（无 ORM）。
- **协议全覆盖**：OAuth 2.1、OpenID Connect、SAML 2.0、CAS、SCIM 2.0、LDAP、SMTP/SMS 登录。
- **多租户授权**：Rebac RBAC + Casbin 执行器，按组织分角色/权限/策略。
- **声明式单一事实源**：DDL、迁移、OpenAPI、前端类型、注册表渲染层全部从 `contracts/schema/*.yaml` 生成；零差异门禁确保文档 ↔ 契约 ↔ 实现永不漂移（ADR-022、ADR-012）。
- **AI 网关**：多模型上游抽象（Anthropic / OpenAI / 自定义）、租户级额度、请求审计、成本追踪。
- **商业子系统**：与 IAM 组织绑定的 product / plan / pricing / coupon 目录。
- **生产级安全**：Argon2id 口令哈希、AES-256-GCM / SM4-GCM 字段级加密（禁止明文回落）、JWT 密钥环、SSRF 防护、RLS、CSRF、限流。
- **方言基线**：PostgreSQL 18 是唯一运行期基线；MySQL / openGauss / KingBaseES 方言为生成物，仅接受夜间或手动工作流验证（ADR-023）。

## 🧱 技术栈

| 层 | 选型 |
|---|---|
| 语言 | Go 1.27+ |
| 后端 | chi · pgx/v5 · go-redis · Casbin · go-sqlmock |
| 前端 | React 19 · Vite · TypeScript · Ant Design 6 |
| 数据库 | PostgreSQL 18（运行期基线）· MySQL（方言） |
| 缓存 | Redis（多副本公平性；进程内回落安全） |
| 可观测性 | OpenTelemetry · Prometheus / Grafana |
| 构建 | GoReleaser · semantic-release · pnpm |
| 部署 | Docker Compose · Kubernetes（manifest 见 `k8s.yaml`） |

## 🚀 快速开始（两步）

### 1. 拉起依赖

```bash
docker compose up -d
```

启动 **PostgreSQL + Redis + MailHog**（本地 SMTP sink）。Redis **默认启用**——进程在无 Redis 时仍能启动，但显式配置后才能真正开启多副本会话吊销共享、公平限流、AI 网关额度等 HA 路径。

### 2. 启动应用

```bash
# 首次启动前：应用结构迁移 + 导入内置 admin / demo 种子
go run ./cmd/migrate up --seed

# 启动服务（默认监听 :8000）
go run .
```

播种后内置管理员：`admin` / `Guliang@2026`（开发哨兵，首登强制改密）。

### 前端

```bash
cd web && pnpm install && pnpm run dev    # http://localhost:5173
```

生产镜像（`docker build`）会把 `web/dist` 一并打包进单容器，不需要单独的静态资源服务。

## 🔐 生产环境必须预配置的参数

下表缺失或格式错误时进程**直接启动失败**（`panic` 或 `503`）。未列出的参数（限流、provider 凭据、验证码、SLS webhook、支付渠道、AI 网关上游、组织品牌等）要么已有安全默认，要么可在首次启动后通过管理控制台在线修改。

| 参数 | 作用域 | 说明 |
|---|---|---|
| `DB_DSN`（或 `POSTGRES_DSN` / `DATABASE_URL`） | 所有环境 | PostgreSQL 18 主库连接串 |
| `ENCRYPTION_KEY_CURRENT`（或旧版 `ENCRYPTION_KEY`） | 所有环境 | 字段级加密密钥 — AES-256-GCM 用 32 B base64，SM4-GCM 用 16 B base64；生成：`openssl rand -base64 32` |
| `ISSUER_URL` | production | 公开 JWT issuer（绝对 HTTPS URL） |
| `METRICS_SCRAPE_TOKEN` | production | `/metrics` 与 `/debug/pprof` 的 Bearer token |
| `PASSWORD_HISTORY_PEPPER` | production | 必须非空，且不能等于内置开发哨兵 |
| `JWT_SIGNING_KEYS_FILE` | production | PEM 密钥环文件路径；production 禁止进程内临时密钥 |
| `BOOTSTRAP_ADMIN_PASSWORD` | production（播种时） | 12–128 字符，需含大小写 + 数字 + 特殊字符 |

启用对应能力时还有条件性必需项——例如 `KMS_PROVIDER=vault` 需要 `VAULT_ADDR` / `VAULT_TOKEN`，`GM_TLS_ENABLED=true` 需要 5 个 SM2 PEM 文件，`STORAGE_TYPE` 非 `local` 时需要对应 provider 凭据。完整表格与 Docker 分步见 `docs/operations/deployment.md`。

## 🐘 Redis

Redis **不是**启动硬性依赖——所有子系统在 Redis 缺失时都会回落到进程内内存。但它是唯一跨副本承载以下能力的组件：

- 会话缓存吊销 / 共享失效
- 公平令牌桶限流（缺失时回落单进程桶，多副本限流不均）
- AI 网关额度计量
- 跨副本 CAPTCHA 答案存储
- SAML replay 缓存 / HIBP k-匿名 range cache / DPoP nonce cache / 审计否认记录器

`docker compose up -d` 默认就启用 Redis。生产建议显式配置：

```dotenv
RATE_LIMIT_REDIS_ADDR=redis:6379          # 可与 REDIS_ADDR 共用，也可独立
RATE_LIMIT_REDIS_DB=1                     # 隔离限流/额度流量
REDIS_ADDR=redis:6379
REDIS_PASSWORD=...                        # compose 开发环境为空
REDIS_DB=0                                # 会话缓存 / 验证码 / 审计
SESSION_CACHE_DRIVER=redis                # 省略或显式 redis；memory 也合法
CAPTCHA_STORE_DRIVER=redis
```

单副本部署可以省掉所有 `REDIS_*` 变量——服务正常启动、打印 INFO 级回落日志。

## 📚 文档

- [docs/README.md](docs/README.md) — 主文档地图（分层 · 设计 · 需求 · ADR 索引 · 运维）
- [docs/architecture/README.md](docs/architecture/README.md) — Clean Architecture、数据模型、安全模型、可观测性
- [docs/design/README.md](docs/design/README.md) — 认证 / IAM / 协议 / 商业 / AI 网关 / 前端
- [docs/operations/deployment.md](docs/operations/deployment.md) — 部署拓扑、迁移 Job、备份与回滚
- [docs/GuliangIAM-开发规范.md](docs/GuliangIAM-开发规范.md) — 工程门禁与写作契约

## 🤝 贡献流程

Conventional Commits、全部门禁通过、不改生成物、不提交生产密钥。完整流程见 [CONTRIBUTING.md](CONTRIBUTING.md)。

```bash
python3 -m pip install -r scripts/requirements.txt
GOCACHE=$PWD/.gotmp/gocache PNPM_HOME=$PWD/.gotmp/pnpmhome \
  RUN_CODE_CHECKS=1 bash scripts/verify-docs.sh
go build ./... && go vet ./... && go test ./...
bash scripts/check-layers.sh
```

## 📄 许可

[Apache License 2.0](LICENSE)。Copyright © 2026 北京谷梁科技有限公司。
