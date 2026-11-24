# 部署指南

> 目标态规范「部署指南」；写作约束见 [docs/README.md](../README.md)。

---

## 1. Docker 部署

### 1.1 Quick Start

```bash
git clone https://github.com/guliangiam/guliangiam.git
cd guliangiam
cp .env.example .env.local
docker compose up -d
```

### 1.2 生产环境变量注入基线

生产部署的 compose 清单必须满足以下注入要求；生产必填项、敏感标记与默认值以 [../generated/env-registry.md](../generated/env-registry.md) 的注册表标记为准（唯一事实源 `contracts/registries/env.yaml`），本文件不复写变量清单：

- 所有生产必填项由编排层注入；口令与令牌一律来自 Secret，禁止明文写入镜像、ConfigMap 或 Git
- 对外发行方地址必须是可达的 HTTPS URL；数据库连接串必须 `sslmode=require`（或更强），生产路径禁止 `sslmode=disable`
- 密钥轮换窗口的并行注入口径见 [security-ops.md](security-ops.md) §2
- `APP_ENV` 必须显式设为 `production`（唯一环境标识，ADR-021），不得依赖默认值推断
- 应用启动路径不执行迁移/播种（启动开关显式关闭）：结构变更一律走受控迁移流水线
- 缺失任一生产必填项时进程 fail-close（启动校验见 `internal/platform/config/validate.go`）

> 生产进程不加载 `.env`/`.env.local`（见 [开发规范附录 D](../GuliangIAM-开发规范.md#附录-d-环境变量完整清单引用)），必填项由编排层注入；本地复制 `.env.example` 后可用开发哨兵启动，但生产缺少任一必填项都会 fail-close。

**验收红线**：compose 生产服务的 `environment` 不出现 Beego 遗留变量；全仓不得存在 `:-password` 形态的默认口令注入与生产路径的 `sslmode=disable`；`docker compose config` 输出包含上述全部目标变量且值来自 secret/env 文件。

### 1.3 Docker Image

构建与运行基础命令：

```bash
docker build -t guliangiam:latest .
docker run -d --name guliangiam -p 8000:8000 guliangiam:latest
```

---

## 2. Kubernetes 部署

### 2.1 Prerequisites

- Kubernetes 1.28+
- PostgreSQL 18（CI 验证）；生产版本需经支持矩阵批准
- Ingress controller（nginx, traefik 等）
- cert-manager for TLS

### 2.2 生产基线模板（Deployment + Service + Ingress）

> 以下为关键配置摘要；仓库内部署产物为 `deploy/`（告警规则与发布校验），K8s 清单按本摘要在部署仓库维护。

**Deployment 关键参数**：

| 参数 | 目标值 | 说明 |
|------|--------|------|
| replicas | 3 | 认证面不可用敏感：先补后缩 |
| minReadySeconds | 30 | 每个新副本先通过 readiness 观察 |
| rollingUpdate.maxSurge | 1 | 先补后缩 |
| rollingUpdate.maxUnavailable | 0 | 任意时刻保持满容量 |
| terminationGracePeriodSeconds | ≥ 40 | ≥ preStop 10s + 在途请求 + 余量 |
| topologySpreadConstraints | zone 打散（maxSkew=1, ScheduleAnyway） | 跨 AZ 尽力打散 |
| podAntiAffinity | hostname 反亲和（preferred） | 同主机不落两副本 |
| securityContext | runAsNonRoot + runAsUser 10001 + readOnlyRootFilesystem + drop ALL + RuntimeDefault seccomp | 不可变 |
| image | 固定版本 tag + digest Pin；禁用 `:latest` | cosign 验签 |
| preStop | sleep 10s | 先摘 Endpoint 再睡 10s |

**ConfigMap 与 Secret**：

`ConfigMap` 只存放非敏感配置，`Secret` 只存放敏感值；生产必填项与敏感标记以 [../generated/env-registry.md](../generated/env-registry.md) 为准。生产 Secret 由 External Secrets Operator 或 Secrets Store CSI 从 KMS/Secret Manager 同步，集群内不落明文 stringData；缺失任一生产必填项时 Pod 在配置校验阶段拒绝启动。

> 进程内 TLS/mTLS（可选）：同时设置 `TLS_CERT_FILE`/`TLS_KEY_FILE` 时应用自行终止 TLS；再设置 `TLS_CLIENT_CA_FILE` 时启用可选客户端证书校验（`VerifyClientCertIfGiven`），用于 RFC 8705 mTLS 令牌绑定。仅设置 CA 而未设置证书/私钥（且未启用代理模式）会启动失败；集群由 Ingress 终止 TLS 时保持三者皆空。

> 反向代理终止 TLS 且需要 mTLS（可选）：设置 `MTLS_PROXY_CLIENT_CERT_HEADER`（例如 `X-Forwarded-Client-Cert`），并把 Ingress/网关配置为「校验客户端证书 + 用校验后的证书链覆盖该头」。该模式默认关闭（空值=忽略所有转发证书头），且必须同时设置 `TRUSTED_PROXY_CIDRS` 为代理的真实出口 CIDR，否则启动失败。应用只读取直连 peer 命中该列表的请求；XFCC 取最后一个 `Cert` 元素，畸形或超限值一律拒绝。使用 `tls_client_auth` 时还需设置 `TLS_CLIENT_CA_FILE` 供应用层复验证书链（此场景无需 `TLS_CERT_FILE`/`TLS_KEY_FILE`）。

> `TRUSTED_PROXY_CIDRS` 必须配置为 Ingress/LB 的真实出口 CIDR；留空表示不信任任何转发头（安全默认）。配错会让客户端 IP 退化为代理 IP，影响限流、IP 白名单与会话 IP 绑定。

### 2.3 K8s 生产清单 8 项上线门槛

| # | 项 | 目标要求 |
|---|---|---|
| 1 | NetworkPolicy | 每命名空间默认 deny-all；放行边界见 README.md §2 |
| 2 | ServiceMonitor | Prometheus Operator 抓取 metrics；保留 28d 滚动窗口 |
| 3 | 反亲和/topologySpread | zone 打散 + hostname 反亲和 |
| 4 | SecurityContext | runAsNonRoot + readOnlyRootFilesystem + drop ALL + RuntimeDefault seccomp |
| 5 | ExternalSecret/CSI 注入 | 集群内不落明文 stringData |
| 6 | 独立迁移 Job | `guliangiam-migrate`，`AUTO_MIGRATE=false` |
| 7 | 镜像不可变 | 固定 tag + digest Pin，禁用 `:latest` |
| 8 | 三探针 + preStop | 见 runbook.md §1 |

---

## 3. 数据库设置

### 3.1 数据库基线与验证范围

| 角色 | 数据库 | 版本基线 |
|--------|--------|---------|
| 运行期基线（必测） | PostgreSQL | 18.6（`postgres:18-alpine`） |
| MySQL 运行期（能力边界受注册表约束）+ DDL/仓储兼容性验证 | MySQL / MariaDB | 8.4 LTS+ / 12.3+（ADR-023、`OPEN-API-35/36/37`） |
| 兼容 PostgreSQL 的国产库 | KingbaseES、GaussDB | 厂商 LTS |
| 兼容 MySQL 的国产/云原生库 | OceanBase、TiDB、PolarDB、神舟通用 | 厂商 LTS |
| 独立方言 | 达梦 DM8 | DM8 |

PostgreSQL 是运行期基线（必测）；`DB_DRIVER=mysql` 由组合根选择 MySQL 运行时（`internal/bootstrap/runtime_factory.go` 的 `newRuntimeMySQLWithDialect`），可启动并处理请求，能力边界受 `OPEN-API-35/36`（审计哈希链不提供、dashboard/LDAP Sync 503 降级）与 `OPEN-API-37` 条目约束（请求级 E2E 边界见 `OPEN-API-73/74`），与 §3.4 的 MySQL 镜像验证口径一致。驱动选择、方言能力矩阵与受限 SQL 子集见 [architecture/data-model.md §3.3](../architecture/data-model.md#33-能力基线postgresql-18rls-强制) 与 [ADR-023](../adr/ADR-023-multi-dialect-abstraction.md)。

### 3.2 结构链与数据链

结构链与数据链是两条独立的版本链，账本主键为 `(kind, version)`：

- **结构链**：`db/postgresql/V{N}__*.sql`，配套 `db/rollback/V{N}__*.down.sql`，由 `AUTO_MIGRATE` 控制
- **数据链**：`db/seeds/V1__guliangiam_seed_init.sql` 单文件（禁止新增种子版本，AGENTS.md §2.6），由 `AUTO_SEED` 控制，为人工编写的幂等脚本；种子建内置组织/应用、管理员与基础目录数据，管理员凭据优先级为「`BOOTSTRAP_ADMIN_PASSWORD` > 数据链 V1 的开发哨兵哈希」，两种来源都强制首登改密；用户自定义的非空哈希不会被覆盖，数据链哨兵会在提供环境口令时被替换。`APP_ENV=production` 且检测到开发哨兵时执行器拒绝迁移。

### 3.3 PostgreSQL 初始化

本地开发：对空库按结构链版本号升序应用生成器产出的迁移。结构链为声明式生成的迁移序列（ADR-022）：V1 全量基线 + 按版本号升序追加的增量迁移，已应用版本不可就地改写；版本清单以 `db/postgresql/` 为准（复现：`bash scripts/check-migrations.sh`）。生产必须走受控迁移流水线（§3.4）；应用启动在所有环境默认不重放数据库，需要时显式开启。

```bash
# 空库：结构链 + 数据链 + 内置管理员一次性凭据初始化。
# 从本地安全输入或 Secret Manager 注入，禁止提交真实口令。
export BOOTSTRAP_ADMIN_PASSWORD='<one-time-strong-secret>'
go run ./cmd/migrate up --seed -dsn "$POSTGRES_DSN"
```

服务启动默认不重放迁移、不导入种子（`AUTO_MIGRATE`/`AUTO_SEED` 默认 `false`）；需要时用启动参数显式开启，命令行优先于环境变量：

```bash
go run .                 # 只启动：校验结构存在，不重放、不导入
go run . -migrate        # 启动前应用待执行的结构迁移（幂等）
go run . -migrate -seed  # 启动前应用结构迁移并导入种子（-seed 隐含 -migrate）
```

启动日志 `startup database switches resolved` 会打印两个开关及其来源（`flag`/`env`/`default`），`database bootstrap skipped|start|finished` 会打印是否执行以及实际应用条数，可直接确认本次启动是否重放或导入。

本地/测试环境可不设置 `BOOTSTRAP_ADMIN_PASSWORD`：先导入种子（服务启动参数 `-seed`，或 `go run ./cmd/migrate up --seed`），再用 `admin` / `Guliang@2026` 首次登录，登录后必须立即改密。该哨兵不得用于 staging/production；生产迁移 Job 必须注入 Secret 口令。

### 3.4 受控迁移流水线

依据 ADR-022：生产 `AUTO_MIGRATE=false`，结构变更一律走独立 K8s Job（`guliangiam-migrate`），与应用发布解耦。首次初始化或旧默认凭据撤销后的迁移 Job 必须通过 Secret 注入 `BOOTSTRAP_ADMIN_PASSWORD`；该口令会替换数据链中的开发哨兵，但不会覆盖用户自定义的非空哈希，初始化完成后应从应用运行配置移除该变量。

迁移 Job 关键配置：`backoffLimit: 0`（失败不自动重试）、`activeDeadlineSeconds: 600`。迁移账本记录当前结构版本，是灰度期判断多实例结构版本一致性的依据。

**发布顺序**：变更评审（CI 结构门禁，ADR-022 决策 4）→ 回滚演练（存在 V{N}+ 增量时执行升级 → 回滚 → 再升级；V1 全量基线以备份恢复验证替代）→ 发布前逻辑备份（`pg_dump -Fc`）→ 执行迁移 Job → **已有库升级前置：平台管理员 scope 回填（见下）** → 迁移成功后再发布应用。

**MySQL 镜像验证（OPEN-API-37，非运行期支持声明）**：先在隔离 MySQL 8.4+ 上用 `ALLOW_MYSQL_SCHEMA_RESET=1 MYSQL_SOCKET/... bash scripts/dev/replay_mysql_schema.sh` 重放生成 DDL（脚本会校验表数与 `db/mysql/V1` 声明一致），再设置 MYSQL_DSN 执行 scripts/dev/verify_mysql_runtime.sh，复核身份/组织/Casbin 策略存储仓储与 DDL 同构；该脚本通过后，DB_DRIVER=mysql 可从组合根启动并处理首个请求（MySQL 授权审计哈希链已镜像、分区探测仅 PG 启用）；AI Gateway 请求级 E2E 已拆分为 OPEN-API-73；SSF 投递已接入 MySQL 并有真实库端到端用例。OPEN-API-73 关闭前仍不得宣称完整 MySQL 请求级支持。

**已有库升级前置（平台管理员 scope bootstrap）**：已有库的数据链 V1 已记入迁移账本，`migrate -seed` 不会重放种子，`platform_admin_scopes` 的种子行可能仍是 `is_bootstrap=false`。发布应用前执行一次性幂等回填（仅命中 built-in 组织 `admin` 的 all-scope 一行）：

```bash
psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 \
  -c "SELECT set_config('app.current_role','super_admin',false)" \
  -f scripts/dev/backfill-platform-admin-scope-bootstrap.sql
```

判据：执行后按业务键复核，恰好 1 行且 `is_bootstrap=true`：

```sql
SELECT s.id, s.is_bootstrap
FROM guliangiam.platform_admin_scopes s
JOIN guliangiam.platform_admins pa ON pa.id = s.platform_admin_id
JOIN guliangiam.users u ON u.id = pa.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
WHERE u.username = 'admin' AND o.slug = 'built-in'
  AND u.deleted_at IS NULL AND o.deleted_at IS NULL
  AND s.scope_type = 'all';
```

未执行回填时，应用在 `APP_ENV=production` 下启动即 fail-fast 并拒绝起服，错误信息包含修复脚本路径：

```text
{"level":"ERROR","msg":"create server","env":"production","caller":"internal.bootstrap","error":"built-in bootstrap admin has no valid all-tenant scope (requires expires_at > now() AND (is_bootstrap OR approved_by IS NOT NULL)); re-run seeding with -seed, or apply scripts/dev/backfill-platform-admin-scope-bootstrap.sql, then restart"}
```

判定依据：缺少有效 all-scope 时平台管理员无法解析任何租户，控制台组织列表为空、平台跨租户可见性静默消失；继续起服会把授权缺失伪装成业务空数据，因此生产 fail-fast 而不是 WARN。开发/测试环境该检查仅 WARN，因为本地库可能处于部分播种状态，不阻塞本地启动。

**迁移相关脚本**：
- `scripts/gen_schema.py --dialect postgresql --check`：声明式 Schema → DDL 零差异
- `scripts/check-migrations.sh`：迁移链完整性（结构链连续性/回滚配对/种子链纯净/双副本 SHA-256）
- `scripts/dev/validate_schema_replay.sh`：空库重放验证
- `scripts/check-db-conventions.sh`（9 项）+ `scripts/check-sql-portability.sh`

### 3.5 PostgreSQL 高可用（Patroni）

| 组件 | 要求 |
|------|------|
| 集群管理 | Patroni（1 主 2 副本，跨 ≥3 AZ）；DCS 用 etcd 3 节点 |
| 持久化 | StatefulSet + 独立 PVC；主/备不共盘 |
| 一致性 | `synchronous_commit = on` + 1 个同步副本；保障 L2 RPO 目标（数值唯一事实源 `contracts/registries/dr.yaml` 的 `tiers[L2].rpo`） |
| 访问入口 | 应用 `POSTGRES_DSN` 指向 Patroni 读写 VIP（主库） |
| 读写分离 | 只读流量（报表/审计查询）走副本；未配置只读 DSN 前所有读写都走主库 |
| 切换 | 计划内 `patronictl switchover`；故障自动 failover |

### 3.6 Redis 部署拓扑与 HA

Redis 承载分布式限流计数、缓存、SAML 重放去重、DPoP nonce 共享与短期认证状态。DPoP jti 去重以 `dpop_replay`（PostgreSQL/MySQL）为事实源，不经 Redis。Session/Refresh Token/用户与授权状态的唯一事实源始终是 PostgreSQL，Redis 不得成为唯一事实源（架构总则）。

- 版本：Redis 8+
- HA 形态：Sentinel（3 哨兵 + 1 主 2 从）或云托管高可用实例，跨 ≥2 AZ
- 故障语义：Redis 不可用时对高风险操作保守拒绝、低风险只读回源（ADR-019）；Redis 故障不触发数据恢复流程，只触发降级告警与容量核查（runbook.md #9）
- SAML 重放去重：Response/Assertion 一次性消费依赖 Redis SET NX（跨副本共享，Redis 错误 fail-close）；未配置 Redis 时回退进程内存储（启动日志 Warn `SAML replay protection uses process-local state`），**多副本部署下跨副本重放检测不可用**——多副本生产必须配置 Redis（`internal/bootstrap/wire_saml_replay.go`）
- DPoP nonce 共享：nonce 经 session-cache Redis 连接跨副本共享（SET NX 签发 / GETDEL 原子一次性消费，TTL 5 分钟，Redis 错误 fail-close；`internal/platform/dpop/redis_nonce.go`）；未配置 Redis 时回退进程内存储（启动日志 Warn `DPoP nonce store uses process-local state`），**多副本部署下跨副本 nonce 识别不可用**——多副本生产必须配置 Redis 会话缓存（`internal/bootstrap/wire_auth_security.go` newDPoPNonceStore，OPEN-API-86 已关闭）；jti 重放去重不受影响（经数据库共享）
- 泄露口令 range 缓存（可选能力）：`PASSWORD_BREACH_CHECK_ENABLED=true` 时注册/改密/重置走 HIBP k-匿名 range 查询（仅 SHA-1 前 5 位出域，需出网访问 `api.pwnedpasswords.com` 或经 `PASSWORD_BREACH_CHECK_BASE_URL` 指向自建镜像）；range 响应缓存 24h 经 session-cache Redis 跨副本共享（键前缀 `guliangiam:breach:range:v1:`），未配置 Redis 回退进程内（启动 Warn，容量 4096）；查询/缓存故障 fail-open（审计标记 `breachCheck=unverified`），不阻断口令操作（`internal/infra/hibp/`，OPEN-API-85 已关闭）

### 3.7 PostgreSQL 运行参数参考

| 参数 | 参考值 |
|------|--------|
| `shared_buffers` | 256MB |
| `work_mem` | 4MB |
| 主键 | UUIDv7（应用层生成，ADR-007） |
| 金额列 | `DECIMAL(19,4)` |
| 币种列 | `VARCHAR(3)` |
| TimeZone | UTC（匹配 `TIMESTAMPTZ` 语义） |
| 必需扩展 | `pgcrypto` |

### 3.8 多方言 DDL 兼容性验证

验证步骤：用生成 DDL 在 MySQL 空库初始化 → 结构等价核对（表/约束逐项一致）→ 能力探测日志核对；关键路径与 API 契约回归只在 PostgreSQL 运行期基线执行。该步骤不构成方言切换或运行期支持。

CI 分层：PR 阻断只执行 PostgreSQL 运行期门禁（`scripts/check-schema-vs-db.py --dialect postgresql`、`scripts/check-rls-policy-content.py`、`scripts/check-sql-runtime.py`）与 PG 回滚独立性；MySQL 作业见 `.github/workflows/nightly-db.yml`，openGauss 与 Tier3 国产库见 `.github/workflows/national-db-ci.yml`，均按夜间或手动模式执行。

数据库相关门禁的连接串：`scripts/check-schema-vs-db.py` 与 `scripts/check-rls-policy-content.py` 读取 `POSTGRES_DSN` 或 `--dsn`；`scripts/check-sql-runtime.py` 读取 `SQL_CHECK_DSN`，并需 `REQUIRE_SQL_RUNTIME_CHECK=1` 才会在缺失连接串时失败而不是跳过。未提供连接串时按脚本定义跳过并输出告警，不等同于通过。

**红线**：若某方言在「活跃唯一性数据库强制」「租户隔离强度」「事件不丢失（Outbox）」任一项上无法等价实现，必须拒绝该方言进入运行期支持范围。

---

## 4. TLS 与边界安全

> 完整 TLS 配置见 security-ops.md §3。

cert-manager 自动签发续期（ACME `letsencrypt-prod`，续期提前 15 天）。Ingress 注解加固：
- `ssl-protocols: TLSv1.2 TLSv1.3`
- `ssl-ciphers` 限定 ECDHE-ECDSA/RSA AES-GCM 套件
- `ssl-prefer-server-ciphers: true`
- HSTS：`max-age=31536000; includeSubDomains; preload`
