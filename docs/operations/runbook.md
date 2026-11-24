# Runbook

> 目标态规范「Runbook」；写作约束见 [docs/README.md](../README.md)。

---

## 1. 三探针分离与优雅停机

> 源章节：§3.1

三探针必须分离，生产资源配置、PDB、HPA 和连接池上限必须通过压测确定，示例值不是承诺值。

| 探针 | 端点 | 失败动作 | 语义 |
|------|------|---------|------|
| startupProbe | `/healthz` | 重启容器 | 只负责放宽启动窗口（示例 30×2s=60s）；启动成功后不再执行 |
| livenessProbe | `/healthz` | 重启容器 | 仅判进程存活（死锁/卡死）；响应体含加密能力位 `status`/`profile`/`missing`/`degraded`；`CRYPTO_PROFILE=gm` 且必需能力缺失时进程在监听前 fail-fast（`/healthz` 不可达，不产生运行期 503），仅 `gm_tls` 缺失时返回 200 `status=degraded`；`unavailable`→503 仅覆盖已发布报告的防御/注入用例（`internal/platform/crypto/suite.go`） |
| readinessProbe | `/readyz` | 摘流（不重启） | 需 `X-Metrics-Token` 或 `Authorization: Bearer <METRICS_SCRAPE_TOKEN>`；实际执行当前驱动 DB Ping（pgx `Ping` / MySQL `PingContext`），失败返回 503 |

**readiness 语义**：`/readyz` 必须先完成真实依赖探测再返回 `{"status":"ok"}`，不得以固定响应充当 readiness；`/api/v1/system/ready` 是同一 DB Ping 的管理面等价端点：运行期装配挂在 ops 组、由 `MetricsTokenMiddleware` 保护（需 `X-Metrics-Token`，与 `/metrics` 同一 token，见 `internal/bootstrap/chi.go` 的 RegisterOpsRoutes）；路由声明层的 `Permission: system:read`（`internal/transport/http/api/v1/system/routes.go`）是契约占位，鉴权口径以装配事实为准（与 §3.4 一致）。`/livez`、`/api/v1/system/health`（legacy 别名）、`/api/v1/system/live` 均为 liveness（`{"status":"ok"}`），不得用作 readiness。应用必须注册 SIGTERM 优雅停机（`signal.Notify` → `http.Server.Shutdown`）。

**优雅停机时序**：

1. Pod 进入 Terminating → Endpoint 从 Service 摘除（与 `preStop` 并行）
2. `preStop` 执行 `sleep 10`：给负载均衡留出缓冲
3. 发送 SIGTERM → 应用停止接收新请求、处理完在途请求后退出
4. `terminationGracePeriodSeconds ≥ 40`（≥ preStop 10s + 在途请求上限 + 余量）

---

## 2. 滚动发布与金丝雀

> 源章节：§3.2

### 2.1 滚动参数

`maxSurge: 1`、`maxUnavailable: 0`（先补后缩，认证面任意时刻保持满容量）；`minReadySeconds: 30`、`progressDeadlineSeconds: 600`。

```bash
# 发布
kubectl -n guliangiam set image deployment/guliangiam guliangiam=guliangtech/guliangiam:v1.2.3
kubectl -n guliangiam rollout status deployment/guliangiam --timeout=600s

# 回滚
kubectl -n guliangiam rollout undo deployment/guliangiam
```

### 2.2 金丝雀

| 要素 | 目标值 |
|------|--------|
| 流量比例 | 5% 起步，阶梯 5% → 25% → 50% → 100% |
| 观察窗口 | 每级 ≥ 30 分钟；覆盖至少一次完整登录+刷新+登出链路 |
| 自动回滚指标 | 5xx 率 > 1%；认证面 p95 延迟越过 SLO；登录成功率较基线下跌 > 2pp；readiness 连续 3 次未就绪 |
| 执行方式 | nginx Ingress `canary-weight` 注解，或 Argo Rollouts |

手动金丝雀：

```bash
# 部署 canary Deployment（replicas=1, track=canary）
kubectl -n guliangiam apply -f k8s/canary-deployment.yaml

# Ingress 注解分流 5%
kubectl -n guliangiam annotate ingress guliangiam \
  nginx.ingress.kubernetes.io/canary="true" \
  nginx.ingress.kubernetes.io/canary-weight="5" --overwrite

# 越限立即回滚
kubectl -n guliangiam annotate ingress guliangiam \
  nginx.ingress.kubernetes.io/canary-weight="0" --overwrite
kubectl -n guliangiam scale deployment/guliangiam-canary --replicas=0
```

---

## 3. 监控与告警

> 源章节：§5

### 3.1 Metrics 端点

`/metrics` 暴露要求：

1. 独立端口优先（如 `:9102`），避免与认证面共享管道
2. NetworkPolicy 仅允许 `monitoring` 命名空间抓取
3. ServiceMonitor 配置 `bearerTokenSecret` 或 mTLS；未持凭证返回 404
4. 抓取令牌取自 `METRICS_SCRAPE_TOKEN`（Secret 注入）；`APP_ENV=production` 下缺失该变量时进程在配置校验阶段拒绝启动（SEC-A4 fail-close）
5. 标签不得携带用户 ID/租户 ID/邮箱等 PII

### 3.2 SLO 看板

按管理面（99.5%）/认证面（99.9%）/协议面（99.9%）三面 SLO 组织。指标定义、看板与阈值唯一口径见 [../generated/slo-registry.md](../generated/slo-registry.md)。

### 3.3 日志

- Structured JSON logs to stdout
- Levels: DEBUG, INFO, WARN, ERROR
- 必须包含 `request_id`, `trace_id`, `span_id` 用于追踪

### 3.4 健康检查

```bash
# 进程存活（liveness，并披露加密能力位）
curl http://localhost:8000/healthz

# 就绪（readiness：当前驱动 DB Ping）
curl -H "X-Metrics-Token: $METRICS_SCRAPE_TOKEN" http://localhost:8000/readyz

# 管理面等价端点（与 /metrics 同组，需同一 token）
curl -H "X-Metrics-Token: $METRICS_SCRAPE_TOKEN" http://localhost:8000/api/v1/system/ready
```

readiness 的唯一运行期判据是当前驱动的 DB Ping；迁移内容哈希与 schema 存在性由启动校验与部署门禁承担。

### 3.5 每方言监控指标

多数据库下用量/容量指标可以统一，但**性能阈值不得统一**；每方言必须各自建立测量基线后再设告警。各方言统一采集：连接池占用/等待时长、事务回滚率、锁等待/死锁计数、慢查询与执行计划、方言能力探测结果。

---

## 4. 容量规划与扩缩容

> 源章节：§7

### 4.1 容量推导公式

集群容量 = 单实例 QPS × 副本数 × 超卖比；超卖比 = 1/(1 + 余量)。连接池预算：`副本数 × pool size ≤ DB max_connections × 0.8`。

**使用规则**：扩容（含 HPA 自动扩容）前必须先核对连接池预算——副本数一旦超出 `max_connections × 0.8 ÷ pool size`，新增副本会拖垮数据库，属反向扩容。

### 4.2 三档刻度（压测数据集）

| 档 | 峰值 API QPS 刻度 | 说明 |
|----|------------------|------|
| Small | 100 | 压测数据集刻度，不是生产承诺 |
| Medium | 1,000 | 同上 |
| Large | 10,000 | 同上 |

### 4.3 HPA 配置

HPA `minReplicas`/`maxReplicas`/阈值均需按 L2 实测校准后回填，示例值不作为定稿。HPA 扩容不解除连接池红线。

### 4.4 Pod 中断预算（PDB）

`minAvailable: 1`，确保滚动发布期间至少 1 个副本可用。

---

## 5. 故障排查 Runbook 索引

> 完整告警分级/响应时限/on-call 见《开发规范手册》第 11/12 章；本节是故障场景 → 定位 → 处置的执行索引。

| # | 场景 | 触发症状 | 快速定位 | 处置要点 |
|---|------|---------|---------|---------|
| 1 | 登录失败率突增 | 登录 5xx/429/失败率告警 | `curl` readiness 端点；日志按 `request_id` 聚合失败原因；区分 429（限流）/5xx（依赖）/401（凭据） | 限流误伤→调 `RATE_LIMIT_AUTH_*`；依赖故障→转 #8/#9；撞库攻击→收紧限流 + WAF |
| 2 | 令牌/签名验证异常 | 401/invalid signature 激增 | 核对 JWKS `kid` 是否轮换未同步；核对各副本 JWKS 缓存与节点时钟漂移（会话来源仅仓库会话行，无签名 Cookie） | JWKS 失效联动验证；滚动重启各副本刷新 JWKS 缓存；时钟对时 |
| 3 | 账号批量锁死 | lockout 告警/客服工单激增 | 失败日志聚合来源 IP | 误伤→管理面解锁 + 出口 IP 白名单；攻击→收紧锁定阈值 |
| 4 | 慢查询 | p95 延迟越过 SLO / 连接池等待告警 | `pg_stat_activity` top active query；`pg_stat_statements`；`EXPLAIN ANALYZE` | 缺索引→走 DDL 补索引流程；挂死查询→`pg_terminate_backend`；池饱和→按 4.1 预算调 `POSTGRES_MAX_CONNS` |
| 5 | 磁盘满 | 节点/DB 磁盘 > 80% 告警 | `df -h`；DB 侧查 WAL 积压：`pg_stat_archiver` | 归档断裂→修复 `archive_command`；**严禁直接删 pg_wal 段**；日志/上传卷清理或扩容 |
| 6 | 证书过期 | TLS 到期告警 | `kubectl get certificate guliangiam-tls -o wide` | ACME 限流/ingress class 不匹配 → 查 challenge/order；手动续期 `kubectl cert-manager renew` |
| 7 | Webhook/Outbox 积压 | 死信/积压告警 | 检查接收方健康与出口日志（SSRF 管控拦截记录） | 接收方故障→通知后恢复投递；死信经管理面重放入口补推 |
| 8 | PostgreSQL 故障切换 | DB 不可达/主库失联告警 | Patroni：`patronictl list`；无 HA：Pod 状态 + `pg_isready` | Patroni 自动 failover → 验证 `pg_is_in_recovery()` 与 VIP 漂移；事后核对审计/Outbox 一致性 |
| 9 | Redis 不可用 | 降级告警（高风险操作被保守拒绝属预期，ADR-019） | 确认限流/缓存降级日志 | 恢复后核查缓存回源与限流计数重置；无需数据恢复 |
| 10 | 密钥泄露/批量撤销 | 泄露信号/异常解密调用 | 按 security-ops.md §4 八步流程 | 按 L0–L4 时限执行；批量吊销走管理面入口（游标分批、幂等、可续跑） |

---

## 6. 发布流程摘要

发布采用兼容迁移（deployment.md §3.4：先迁移后应用，含回滚演练）、滚动/金丝雀切换（§2.2）和观察窗口。发布前必须通过编译、测试、安全扫描、迁移演练、备份检查（backup-dr.md §2 发布前快照）、回滚准备和告警确认；发布后验证健康、登录、MFA、OAuth/OIDC、令牌刷新、租户隔离、审计和错误率。

---

## 7. 环境基线矩阵

| 维度 | dev | staging | production |
|------|-----|---------|------------|
| 数据 | 模拟/可重建 | 脱敏数据 | 生产数据 |
| AUTO_MIGRATE | false | false | false |
| AUTO_SEED | false | false | false |
| BOOTSTRAP_ADMIN_PASSWORD | 通常不必注入（导入种子后使用哨兵，仅限本地/测试）；需要替换哨兵时由本地安全输入 | 仅初始化/恢复时由 Secret 注入 | 仅初始化/旧默认凭据撤销后的迁移 Job 由 KMS/Vault 注入 |
| Secret | 专用测试密钥 | 独立 Secret Manager | KMS/Vault |
| 副本 | 1 | 2 | 按容量规划 |
| 审批 | 无 | 1 人 | 2 人 |
| APP_ENV | dev | staging | production（唯一环境标识，ADR-021） |

两个自动开关在 dev 也默认关闭（手册 3 / env 注册表）：启动只校验 schema 是否存在，不重放迁移与种子。

| 启动参数 | 语义 | 默认 |
|---|---|---|
| 无 | 只启动：校验 schema 存在，不重放、不导入 | — |
| `-migrate` | 启动前应用待执行的结构迁移（幂等） | 取 `AUTO_MIGRATE`，未设置则关闭 |
| `-seed` | 启动前导入种子数据；隐含 `-migrate` | 取 `AUTO_SEED`，未设置则关闭 |

优先级：命令行 > 显式 `AUTO_MIGRATE`/`AUTO_SEED`（`true`/`1`/`yes`/`on`） > 默认关闭；`-migrate=false` 可覆盖环境变量。

```bash
go run .                 # 只启动；空库会 fail-fast（schema is empty and AUTO_MIGRATE is not enabled）
go run . -migrate        # 只重放结构迁移
go run . -migrate -seed  # 重放结构迁移并导入种子
```

本地首次初始化或需要升级结构时，也可使用等价的运维入口：

```bash
go run ./cmd/migrate up --seed -dsn "$POSTGRES_DSN"
```

启动日志 `startup database switches resolved` 会打印两个开关及其来源（`flag`/`env`/`default`），`database bootstrap skipped|start|finished` 会打印是否执行以及实际应用条数。若日志显示意外重放，先核对来源列与 `.env` 中的 `AUTO_MIGRATE`/`AUTO_SEED`。

**种子重放副作用（“种子即真相”）**：重放种子会把内置管理员的 `is_active` 恢复为 `true`，并把 built-in 管理员 all-scope 的 `is_bootstrap` 恢复为 `true`。排查“内置管理员被停用”或“平台全租户不可见”时，先核对上一条开关日志确认是否发生过重放，不要把重放后的状态当成人工变更或事故。

生产不使用启动参数：`AUTO_MIGRATE`/`AUTO_SEED` 保持 `false`，迁移由独立迁移 Job 执行（deployment.md §3.4）。

**已有库升级前置**：应用发布前按 deployment.md §3.4 执行 `scripts/dev/backfill-platform-admin-scope-bootstrap.sql`（一次性幂等，仅命中 built-in 组织 `admin` 的 all-scope 行），并复核 `is_bootstrap=true`。不要用 `AUTO_SEED=true` 或 `-seed` 代替——数据链 V1 已记账本，种子不会重放。未执行时生产启动 fail-fast，错误信息包含该脚本路径；开发/测试环境该检查仅 WARN。

**平台管理员停用/恢复语义（OPEN-API-55）**：停用（`SetActive(false)`）在**同一事务内吊销该用户全部既有会话**（`RevokeByUserExcept`），提交后立即失效 principalType 缓存（`PrincipalCache.Invalidate`）——停用即时生效，无缓存滞后；若 UoW 不具备会话吊销能力则 fail-closed 拒绝停用。删除（`DeletePlatformAdmin`）同样立即吊销会话并失效缓存。恢复（`SetActive(true)`）即时生效，但**不会复活旧会话**——被停用的管理员必须重新登录。实现与证据见 `internal/app/platform_admins/service.go` 与注册表 `OPEN-API-55` 条目。据此判断安全事件时，「已停用」即同一时刻会话已吊销；多实例部署下各实例缓存经同一失效路径收敛。

隔离库自动化回归：`ALLOW_SCHEMA_RESET=1 DATABASE_URL=postgres://…/guliangiam_startup_verify bash scripts/dev/verify_startup_scenarios.sh` 覆盖空库拒绝、已初始化库跳过、仅重放、重放+导入四种组合。

**开发引导口令**：dev 可不注入 `BOOTSTRAP_ADMIN_PASSWORD`：导入种子（服务启动参数 `-seed`，或 `go run ./cmd/migrate up --seed`）后，首次登录使用种子写入的哨兵凭据（`admin` / `Guliang@2026`，详见 `AGENTS.md` §3），登录后必须立即改密；未导入种子时不会写入任何管理员凭据。staging/production 必须由 Secret 注入 `BOOTSTRAP_ADMIN_PASSWORD`（策略要求），其中 `APP_ENV=production` 时执行器检测到哨兵会强制拒绝迁移，staging 为策略约束、代码不拦截。

## 8. 本地静态分析工具链（golangci-lint / goimports）

`scripts/check-lint.sh --strict` 由 `verify-docs.sh`（`RUN_CODE_CHECKS=1`）调用。本地若出现下列任一条，说明**工具二进制与当前 Go 工具链不匹配**，门禁会降级为 WARN 并跳过：

```text
export data version N is greater than maximum supported version M
```

处置：

```bash
# 1) 工具版本固定为 CI 使用的 v2.15.0
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

# 2) 必须用当前仓库的 Go 工具链构建，且裸二进制在 PATH 中可见
go version                                   # 与 binary 的 built with 版本一致
"$(go env GOPATH)/bin/golangci-lint" version

# 3) 配置自检 + 增量阻断 + 全量存量观察
PATH="$(go env GOPATH)/bin:$PATH" bash scripts/check-lint.sh --strict
PATH="$(go env GOPATH)/bin:$PATH" bash scripts/check-lint.sh --all
```

- `--strict` 只阻断本次改动引入的新问题；`--all` 输出全量存量，以门禁输出为基线；新增失败阻断。
- `REQUIRE_GOLANGCI_LINT=1`（CI 使用）让「工具缺失」变成失败；本地默认仅警告，避免环境问题把门禁变红灯。
- 若升级 Go 工具链，必须同时重建 linter；CI 的 `setup-go` + 固定版本安装已保证二者一致。

### 8.1 goimports 导入分组门禁

`scripts/verify-docs.sh` 的 `_check_gofmt` 先跑 `gofmt -l .`，再在 `PATH` 中找到 `goimports` 时才追加导入分组检查。门禁使用**裸** `goimports`（不带 `-local`）：它保证标准库与非标准库分组、组内按字典序；手册 1.1.4 要求的「项目内包单独成组」以 `goimports -local github.com/guliangiam/guliangiam -l .` 输出为空为判据，零差异后把 `-local` 写入门禁。工具缺失时该分支**静默跳过**，所以：

goimports 钉版的唯一事实源是 CI `gates` 作业的 `Install goimports` 步骤（`.github/workflows/ci.yml`）；本地安装必须与该步骤钉住的版本一致（本文不复制版本号，避免与 CI 漂移）：

```bash
# 版本号以 .github/workflows/ci.yml 的 Install goimports 步骤为准
go install golang.org/x/tools/cmd/goimports@<CI 钉住版本>

# 复现门禁（与 CI gates 作业的 Go format check 等价）
gofmt -l . ; goimports -l .
PATH="$(go env GOPATH)/bin:$PATH" RUN_CODE_CHECKS=1 bash scripts/verify-docs.sh
```

CI 侧已固定：`gates` 作业在 `setup-go` 之后安装同版本 goimports，格式化步骤同时校验 `gofmt` 与 `goimports`，`Documentation gate` 以 `PATH="$PATH:$(go env GOPATH)/bin"` 调用 `verify-docs.sh`。本地修复用 `goimports -w .`，禁止用 `//nolint` 或改门禁绕过。

## 9. 加密列回填与密钥轮换

适用场景：`internal/platform/crypto/reencrypt_job.go` 的 `DefaultEncryptedColumns` 登记列存在非 envelope 存量值（包括 V8 扩宽后首次纳入加密的 `radius_configs.shared_secret`、`captcha_providers.secret_key`、`certs.private_key`/`access_secret`，以及 `user_mfa_config.totp_secret`），或需要把既有密文统一到当前 KEK 与信封代次。

读取侧对非 envelope 值 fail-closed：回填前这些行不可读；回填写入的 v2 密文旧版本进程无法解析。必须按维护窗口执行。

### 9.1 回填步骤

1. **备份**：`pg_dump` 导出待回填表；备份是回填后唯一可靠的回滚路径。**必须走高权通道**（superuser socket / 专用 `BYPASSRLS` 角色）：FORCE RLS 下应用账号导不全数据，见 backup-dr.md §5.1 与本文 §10。
2. **停流量**：旧版本缩容到 0，或进入维护模式。
3. **结构迁移**：`go run ./cmd/migrate up --dsn "$POSTGRES_DSN"`，应用全部待执行结构迁移。
4. **统计**：`go run ./cmd/migrate backfill-encrypted-columns --dsn "$POSTGRES_DSN" -dry-run`；逐列输出 `scanned/skipped/would-encrypt/legacy-v1/legacy-v0/stale-v2/unreadable`，不写任何行、不生成 checkpoint。
5. **回填**：`go run ./cmd/migrate backfill-encrypted-columns --dsn "$POSTGRES_DSN"`；`-batch-size` 控制每批扫描行数（默认 200）。
6. **校验**：`go run ./cmd/migrate backfill-encrypted-columns --dsn "$POSTGRES_DSN" -verify`，必须 exit 0；判定：exit 0 表示注册列中不存在明文/`unrecognized` 值，非零表示仍有待处置行，按 `table.column pk=... class=...` 队列定位，`-max-failures` 控制队列上限（默认 100，0 不限）。
7. **启动新版本**，恢复流量。

命令语义：

- 扫描范围是 `DefaultEncryptedColumns`；`encrypt-provider-secrets` 仅覆盖 provider 凭据子集，保留用于窄范围操作。
- 非 envelope 行按代次分类：v1 旧信封计为 `legacy-v1`，可解密的 v0 直连 KEK 令牌计为 `legacy-v0`，两者都留给 `reencrypt`，不会被二次加密；既非信封也不能按 v0 解开的计为 `unrecognized`（可能是明文或损坏密文）。
- 按主键分批推进，单行失败计入该列 `failed` 并进入主键队列，同时继续下一行；进程中断后重跑会重新扫描剩余明文，因此命令可重复执行且幂等，无需 checkpoint 文件。
- 空值与 NULL 表示“未配置”，保持原值并计入 `skipped`。

### 9.1.1 PII legacy 明文源列封存（`backfill-pii-sealed-columns`）

PII `_sealed` 目标列另有 legacy 明文源列；映射唯一源为 `internal/platform/crypto/legacy_sealed_columns.go` 的 `LegacySealedColumns`。通用回填不处理这些源→目标映射，必须使用专用命令：

1. **预检**：`go run ./cmd/migrate backfill-pii-sealed-columns --dsn "$POSTGRES_DSN" -dry-run`；只统计将封存的行，不写入。
2. **执行**：`go run ./cmd/migrate backfill-pii-sealed-columns --dsn "$POSTGRES_DSN"`；按主键分批（`-batch-size` 上限 1000），只写 `_sealed` 目标列且不覆盖已有密文；`-max-failures` 控制单列失败上限（0 = 不限）。
3. **校验**：`go run ./cmd/migrate backfill-pii-sealed-columns --dsn "$POSTGRES_DSN" -verify`，输出 `encrypted/unsealed/decrypt_failed` 计数；判定：`unsealed=0` 且 `decrypt_failed=0` 时 exit 0，否则 exit 1。执行模式存在失败行时同样 exit 1，修复后重跑（已封存行会跳过）。

**源列保留边界**：本命令只写 `_sealed` 目标列，legacy 明文源列在回填后仍保留，读取路径在目标列封存后切换；清除源列属于单独的破坏性迁移，需备份/回滚评审、维护窗口与租户范围确认，不在本命令范围内。

### 9.1.2 sealed payload 明文封存（`encrypt-sealed-payloads`）

`llm_providers.api_key_sealed` / `pay_channel_configs.config_sealed` 这类非列式 sealed payload 与普通加密列代次不同：应用写路径以 `EncryptField` 写入 v1 信封（无 AAD），读取侧用 `DecryptColumn` 兼容 v1/v0/v2。通用 `backfill-encrypted-columns` 不写这些列，必须使用专用命令。覆盖范围以 `internal/platform/crypto/sealed_payloads.go` 的 `SealedPayloadColumns` 注册表为准；当前登记为 `llm_providers.api_key_sealed`、`pay_channel_configs.config_sealed`（支付渠道配置的明文 JSON，概念上的 `config`，在 `config_sealed` 列内原位封存），新增登记自动纳入，本文不维护手写清单。用 `-table` 可窄化到单表（接受 `pay_channel_configs` 或 `guliangiam.pay_channel_configs`）。

1. **预检**：`go run ./cmd/migrate encrypt-sealed-payloads --dsn "$POSTGRES_DSN" -dry-run`；只统计并列出 `WOULD-SEAL` 主键（`-max-failures` 控制队列上限，默认 100），输出 `would_encrypt=...`，不写库。需要窄化时追加 `-table pay_channel_configs`。
2. **执行**：`go run ./cmd/migrate encrypt-sealed-payloads --dsn "$POSTGRES_DSN"`；只选中非信封值，写入 v1 信封，按主键单行 CAS 更新（`UPDATE ... WHERE pk = $2 AND column = $3`），不持有批事务。`-batch-size` 复用平台分页边界（默认/上限 100）。
3. **校验**：`go run ./cmd/migrate encrypt-sealed-payloads --dsn "$POSTGRES_DSN" -verify`；逐列输出 `encrypted/plaintext_remaining/aad_mismatch/unreadable/legacy_v1/legacy_v0/v2_current`。判定：`plaintext_remaining=0`、`aad_mismatch=0`、`unreadable=0` 时 exit 0；任一非零 exit 1，按 `FAIL table.column pk=... stage=verify reason=...` 队列定位。`-verify` 从不写库，`-dry-run` 与 `-verify` 互斥。

**AAD 口径**：v2 sealed 值必须按 `table|column|pk`（实际串为 `v2|{table}|{column}|{primary_key}`）通过 GCM 认证；命令使用 `SealedPayloadColumn.AAD(pk)`，与 `LegacySealedColumns` 的 `LegacySealedColumn.AAD` 共用同一构造，不手工拼 AAD。v1/v0 没有 v2 AAD，`-verify` 只验证可解密：v1 计 `legacy_v1`、v0 计 `legacy_v0`；无法解密的信封计 `unreadable`，既非信封也不能按 v0 解开的计 `plaintext_remaining`。写代次当前保持 v1，与现有应用写路径一致；不要在本命令中把 sealed payload 升级为 v2。

**`-force` 风险**：`-force` 会把可读 v1/v0 在当前 KEK 下重封为新 v1 密文。执行前必须备份并安排维护窗口，且所有读实例必须已具备当前与上一版 KEK，否则旧实例可能无法解密；该参数不会降级 v2（v2 原样跳过）。单行 CAS 保证并发写入的新值不会被覆盖，但重封仍会产生一次密文刷新，非必要不要使用。

**幂等**：重跑会跳过已有可读 sealed 值（v1/v0/v2），只处理剩余非信封值，不产生二次密文；进程中断后可直接重跑，无需 checkpoint。

### 9.1.3 租户身份与主拓扑回填（`backfill-tenant-topology`）

适用：`tenant_identities` 缺失，或缺少非退役主拓扑（`tenant_instances.is_primary` 且 `status <> 'decommissioned'`）的历史组织（0-R3-T-L3）。该命令是迁移外的显式运维动作，不在 V1 内，可重复执行。

1. **预检**：`go run ./cmd/migrate backfill-tenant-topology --dsn "$POSTGRES_DSN" -dry-run`；输出 `affected_orgs/missing_identities/missing_topologies`，不写库。
2. **执行**：`go run ./cmd/migrate backfill-tenant-topology --dsn "$POSTGRES_DSN"`；`-batch-size` 控制每批组织数（默认 200）。身份用 `ON CONFLICT (organization_id) DO NOTHING`，主拓扑用 `NOT EXISTS` 守卫并对同区域已退役行做复活（`ON CONFLICT (organization_id, region_id) DO UPDATE`），两个写路径都幂等。
3. **校验**：`go run ./cmd/migrate backfill-tenant-topology --dsn "$POSTGRES_DSN" -verify`，必须 exit 0（无活跃组织缺身份或主拓扑）。
4. **回滚**：命令只新增/复活身份与拓扑，不修改 `organizations`；回滚以备份恢复为准（备份通道见 §10）。

`-dry-run` 与 `-verify` 互斥；命令在写模式结束时会重新聚合扫描，仍有缺失则 exit 1。只处理 `deleted_at IS NULL` 的组织，软删组织不会被回填。

### 9.2 密钥轮换

1. 回填 `-verify` exit 0 后才允许轮换；明文/`unrecognized` 行在 `reencrypt` 中计为错误且不会被加密。
2. 预检：`go run ./cmd/migrate reencrypt --dsn "$POSTGRES_DSN" -dry-run`；只读分类，输出 `would-update/current-v2/legacy-v1/legacy-v0/unreadable`，不写行、不读写 checkpoint。
3. 执行：`go run ./cmd/migrate reencrypt --dsn "$POSTGRES_DSN"`；默认 checkpoint 为 `./reencrypt.checkpoint`，用 `-checkpoint` 指定路径，`-checkpoint ""` 关闭续跑。失败行进入主键队列；checkpoint 停在首个失败行之前，修复后重跑会重试该行。
4. 校验：`go run ./cmd/migrate reencrypt --dsn "$POSTGRES_DSN" -verify`，必须 exit 0；判定：exit 0 表示全部已注册加密列与 sealed payload 均为当前代次，非零表示失败队列非空，输出 `FAIL table.column pk=... class=... action=...` 队列（`-max-failures` 默认 100）。
5. 轮换完成后再次执行 §9.1 第 6 步的回填 `-verify`。

### 9.3 回滚

| 时点 | 动作 |
| --- | --- |
| 回填前 | 执行对应结构迁移的 `db/rollback/*.down.sql`，再回退应用版本 |
| 回填后 | 恢复 §9.1 第 1 步快照；不接受“解密命令”作为回滚路径 |

回填后执行 V8 回滚会因密文长度超过原 `VARCHAR` 失败，属预期行为，不能作为回滚手段。

### 9.4 失败处置

- 缺少 `ENCRYPTION_KEY_CURRENT`（或 legacy `ENCRYPTION_KEY`）：命令在连接前失败，不写任何行。
- 回填 `-verify` 非零退出：`unrecognized`/`unreadable` 行仍需处理——`unrecognized` 若为明文，重新执行回填；若为损坏密文，先从备份恢复。`unreadable` 检查 KEK 版本/算法或从备份恢复。仅 `legacy-v1/legacy-v0/stale-v2` 时回填 `-verify` 仍 exit 0，按提示执行 `reencrypt`。
- `reencrypt -verify` 非零退出：队列中的 `legacy-v1/legacy-v0/stale-v2` 重跑 `reencrypt`；`unrecognized` 按回填处置；`unreadable` 检查 KEK 版本/算法或从备份恢复。
- 单行 `failed`：按 stdout 队列中的 `table.column pk=...` 定位；结构化日志仍只含 `pk_fingerprint`（ADR-015）。修复后重跑，已加密行不再命中扫描条件。
- `-dry-run` 与 `-verify` 互斥，`-batch-size` 必须 ≥ 1，`-max-failures` 必须 ≥ 0。

## 10. 备份通道：FORCE RLS 约束

本库对业务表启用 `FORCE ROW LEVEL SECURITY`（ADR-016）。FORCE 的语义是表 owner 也受策略约束，因此**应用账号不能用于 `pg_dump`**：

- 应用账号即使持有表 owner 身份，没有 `app.current_role='super_admin'` 会话上下文时只能导出当前租户策略放行的行；缺少 schema/表权限时直接报 `permission denied for schema guliangiam`（SQLSTATE 42501）。
- 每日逻辑备份、发布前/回滚快照、恢复校验一律走高权通道（superuser socket、专用 `BYPASSRLS` 备份角色，或 `pg_basebackup` 复制通道）。凭据来自 KMS/Secret，禁止写入仓库、日志或命令行。
- 禁止为完成备份而临时提升应用账号权限或关闭 RLS。

隔离验证库（临时库）在 RLS 下用应用账号跑应用/worker（如 outbox worker 回放）前，若报 `permission denied for schema guliangiam`，由高权账号补授权：

```sql
GRANT USAGE ON SCHEMA guliangiam TO guliangiam;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA guliangiam TO guliangiam;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA guliangiam TO guliangiam;
```

这是隔离库/本地验证的授权动作，不改变生产 RLS 策略；生产应用账号的权限由部署脚本统一管理。

## 11. 运行期数据库角色自检（低权 DSN）

服务进程在构建运行期连接池前执行角色自检（`internal/bootstrap/db_role_check.go` 的 `verifyPostgresRoleIsolation`，由 `internal/bootstrap/runtime_factory.go` 调用）：读取 `current_user` 的 `rolsuper` 与 `rolbypassrls`，任一为 `true` 即 fail-fast 并拒绝启动。原因：`FORCE ROW LEVEL SECURITY` 只对普通角色生效；用 superuser 或 `BYPASSRLS` 角色启动服务会让租户隔离静默失效。错误信息只输出角色名与两个布尔值，不输出 DSN 或口令。

| 进程 / 通道 | DSN 角色要求 | 受启动自检约束 | 说明 |
|---|---|---|---|
| 服务进程（`main.go` → `internal/bootstrap`） | `rolsuper=false` 且 `rolbypassrls=false` | 是（fail-fast） | 日常运行唯一允许的通道 |
| `cmd/migrate`（结构/数据迁移、`--seed`） | 高权角色（schema owner） | 否 | 需要 DDL 与 `guliangiam` schema 权限；走本机 socket 或 Secret 注入 |
| `cmd/outbox-worker` | 应用账号可用 | 否 | 已实测应用账号回放 4 个领域事件 succeeded、死信 0；必须使用 `platformdb.NewPostgresPool` 才有租户 GUC 注入。高权账号也能运行但会绕过 RLS，仅限维护窗口 |
| 备份 `pg_dump` / `pg_restore` | superuser 或专用 `BYPASSRLS` 角色 | 否 | 见 §10；FORCE RLS 下应用账号导不出完整数据 |

两条命令示例（口令由 `.pgpass`/Secret 注入，不写进命令行或仓库）：

```bash
# 例 1：服务进程只用低权应用账号（当前角色必须 rolsuper=false 且 rolbypassrls=false）
DATABASE_URL='postgres://guliangiam@localhost:5432/guliangiam?sslmode=disable' go run .

# 例 2：迁移/播种走本机高权 socket（角色由 socket/pg_hba 认证决定，命令里不写口令）
go run ./cmd/migrate up --seed -dsn 'postgres:///guliangiam?host=/tmp'
```

不要把应用账号提升为 superuser/`BYPASSRLS` 来绕过自检；需要高权的操作走 `cmd/migrate`、备份通道或一次性维护 Job。

## 12. Schema 变更流程（ADR-022 声明式增量迁移）

结构变更只改 `contracts/schema/**` 声明，由 `scripts/gen_schema.py` 生成 `db/postgresql/V{N}` 增量迁移与 `db/rollback/` 配对回滚，再重生成四方言镜像；**已应用的迁移版本不可就地改写**，新变更一律以新版本号追加。门禁：`bash scripts/check-migrations.sh`、`python3 scripts/check-schema-parity.py`、`bash scripts/check-generated-zero-diff.sh`。发布前置与基线保护见 `OPEN-API-57` 条目（注册表为唯一口径）。

**账本语义（必须知道）**：`migrate` 跳过已应用版本。若已应用的迁移文件被就地改写，账本与文件内容失配，存量库不会获得新增列/约束——典型症状是运行期列缺失错误（SQLSTATE 42703，`column ... does not exist`）。`-seed` 不会修复（数据链只写数据、不做 DDL），重复播种也不会续期或补列。合法增量迁移经 `migrate up` 即可补齐，不要误用删库重建。

开发库结构漂移无法以增量迁移收敛时，可删库重建（**仅限本机开发库；staging/production 禁止执行 DROP**）：

```bash
# 1) 先取证：确认目标库名、当前角色、表数量
psql "$DSN" -c "SELECT current_database(), current_user;" \
            -c "SELECT count(*) FROM pg_tables WHERE schemaname='guliangiam';"

# 2) 删除并重建（owner 与应用账号一致）
psql 'postgres://localhost:5432/postgres' -c 'DROP DATABASE guliangiam;'
psql 'postgres://localhost:5432/postgres' -c 'CREATE DATABASE guliangiam OWNER guliangiam;'

# 3) 重放结构与种子（显式 DSN；开发哨兵口令见 AGENTS.md §3）
go run ./cmd/migrate up --seed -dsn "$DSN"
```

发布三条硬前置（缺一不可，`OPEN-API-57`）：

1. **V1 checksum 冻结**：发布前执行
   ```bash
   python3 scripts/release/check_release_readiness.py --write-manifest
   python3 scripts/release/check_release_readiness.py
   ```
   清单写入 `deploy/release/v1-checksums.sha256`（四方言 V1 + `schema_sql` 镜像 + `contracts/schema/history/active/V1/` 快照目录摘要）；`--write-manifest` 仅在冻结窗口执行，释放后任何 V1 变化都必须由 `check_release_readiness.py` 判定失败。

   发布前备份/恢复演练（隔离 `_drill` 库）：
   ```bash
   REHEARSE_SOURCE_DSN='postgres://…' \
   REHEARSE_TARGET_DB=giam_release_drill \
   REHEARSE_TARGET_DSN='postgres://…/giam_release_drill' \
   REHEARSE_TARGET_ADMIN_DSN='postgres://…/postgres' \
   REHEARSE_EVIDENCE=.trae/documents/release-v1-restore-drill-$(date +%F).md \
   python3 scripts/release/rehearse_v1_restore.py
   ```
   脚本会校验目标库名含 `_drill`，dump 后重建目标库、restore、比对表数并归档 SHA-256/证据报告。
2. **生产/预发禁用 `ALLOW_CHECKSUM_DRIFT`**：代码在 `APP_ENV=production` 且该值为真时 fail-fast（`internal/platform/db/migrate.go`）；部署清单不得注入该变量，也不得为「让迁移通过」而放宽。
3. **首版基线 + 回滚演练**：归档首个正式库快照，并完成一次备份恢复或配对回滚的演练记录（增量迁移的配对回滚见 `db/rollback/`；基线恢复以备份恢复为准）。
   ```bash
   RELEASE_BASELINE_SNAPSHOT=/path/to/first-baseline.dump \
   RELEASE_ROLLBACK_EVIDENCE=/path/to/rollback-drill.md \
   python3 scripts/release/check_release_readiness.py --strict-manual
   ```

撤销/回滚：增量迁移优先执行 `db/rollback/` 配对回滚脚本；开发库可直接删库重建；基线级恢复以备份恢复为准。

---

## 测试与性能回归（事实源引用）

### 测试与质量证据（按事实源查询，本文不手写计数）

测试用例数、覆盖率、基准数值与安全就绪度均为**可生成事实**，本文一律不复写；按下列事实源与复现命令查询：

| 关注点 | 事实源 | 复现命令 |
|--------|--------|---------|
| Go 测试 / 基准 | `internal/**` 测试资产 | `go test ./internal/...`；`go test -bench . ./internal/...` |
| 前端单测 / E2E | `web/src/**/*.test.tsx`、`web/e2e/*` | `cd web && pnpm run test -- --run`；`pnpm run e2e` |
| 覆盖率门禁 | `.github/workflows/ci.yml` 覆盖率门槛 | `go test ./internal/... -coverprofile` |
| 基准回归 | `scripts/bench-baseline.txt` + `scripts/bench-gate.sh` | `bash scripts/bench-gate.sh` |
| 安全就绪度 | `contracts/registries/security-readiness.yaml`（渲染层 `../generated/security-readiness.md`） | `python3 scripts/check-security-readiness.py` |

### 性能基准回归

基准数值唯一记录于 `scripts/bench-baseline.txt`，由 `bash scripts/bench-gate.sh` 与 CI 比对；本文不复制数值。

> 运行时就绪度缺口以 `contracts/registries/*`（SLO、安全就绪、开放项）与 CI 门禁为准，本文不维护状态/优先级清单。

## 13. bootstrap 管理员 all-tenant scope 到期续期

启动期自检会在 scope 到期前 30 天输出 WARN（含 scope_id、expires_at、剩余小时数与续期脚本路径）；到期后生产启动 fail-fast。续期是运维动作：

1. 确认变更单与 built-in 管理员身份；
2. 执行 scripts/dev/renew-platform-admin-scope-bootstrap.sql（仅处理 built-in admin、approved_by IS NULL 且 7 天内到期的 all-scope 行，幂等）；
3. SELECT 复核 expires_at 已延长，再重启服务。
