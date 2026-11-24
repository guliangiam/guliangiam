# 运维总纲

> 目标态规范「运维总纲」；写作约束见 [docs/README.md](../README.md)。

## 1. 部署原则

1. 生产部署优先使用不可变镜像、独立迁移 Job 和外部 PostgreSQL；应用 Pod 不直接负责破坏性回滚
2. Secret 通过 KMS/Vault/云 Secret Manager 注入，禁止将真实值写入 Git、镜像层、ConfigMap 或日志
3. 多实例部署前必须确认 Session、Refresh Token、一次性认证状态和限流的共享存储策略
4. 数据库结构管理遵循 ADR-022：声明式 Schema 是唯一事实源；生产 `AUTO_MIGRATE=false`

## 2. 多 AZ 拓扑与命名空间

生产拓扑以单区域多可用区（≥3 AZ）为最低要求。命名空间规划：`guliangiam`（应用 + 迁移 Job）、`guliangiam-data`（PostgreSQL/Redis）、平台组件各居其位。生产与预发使用独立集群。

信任边界默认每命名空间 deny-all 再逐条放行。公网入口、应用出站、数据面入站、管理通道、指标抓取边界登记于 deployment.md §2.1。

## 3. 阅读路径

| 读者 | 必读 | 选读 |
|------|------|------|
| DevOps/SRE | deployment.md、runbook.md、backup-dr.md | security-ops.md |
| 后端开发 | deployment.md、security-ops.md | runbook.md |
| 安全工程师 | security-ops.md、backup-dr.md | runbook.md |
| DBA | backup-dr.md、deployment.md §3 | — |

## 4. 文件索引

| 文件 | 内容 | 主要来源 |
|------|------|---------|
| [deployment.md](deployment.md) | Docker Compose + K8s 清单 + 环境变量引用 + 数据库初始化 | §1–§4 |
| [runbook.md](runbook.md) | 监控 + 告警 + 健康检查 + 故障排查 + 扩缩容 + 探针 + 发布/回滚/金丝雀 | §3.1–§3.2、§5–§7、§10 |
| [backup-dr.md](backup-dr.md) | pg_basebackup + WAL + PITR + 异地副本 + RTO/RPO + 恢复演练 | §8 |
| [security-ops.md](security-ops.md) | Secret 轮换策略 + TLS 配置 + 密钥生命周期 | 《GuliangIAM-开发规范》§14（密钥与加密规范）+ §6 |

## 5. 关键外部引用

| 引用 | 用途 |
|------|------|
| [../generated/env-registry.md](../generated/env-registry.md) | 环境变量唯一口径；生产必填项以注册表的「生产必填」标记为准 |
| [../generated/limit-registry.md](../generated/limit-registry.md) | 限流器参数唯一权威表 |
| [../generated/slo-registry.md](../generated/slo-registry.md) | SLO 唯一口径 |
| [../generated/state-machine-registry.md](../generated/state-machine-registry.md) | 商业域状态机唯一口径 |
| [../generated/security-readiness.md](../generated/security-readiness.md) | 安全能力就绪度唯一口径 |
| [../requirements/data.md](../requirements/data.md) | 数据生命周期策略 |
| [../../CHANGELOG.md](../../CHANGELOG.md) | 变更历史唯一位置 |

## 6. 运维设计约束与门禁

> 本节给出运维侧的**目标态约束与判据**；不含现状登记、批次与工期。

### 6.1 结构与质量约束

| 维度 | 约束 | 判据 |
|------|------|------|
| 文件体量 | 单文件 ≤ 500 行；God File（> 900 行，非测试非生成）阻塞合并 | 见 [architecture/layers.md §8.3](../architecture/layers.md#83-文件体量阈值) 复现命令 |
| 分层合规 | App → Infra 穿透 = 0；Platform 边界纯净；Bootstrap 不被非 Bootstrap 包 import | `bash scripts/check-layers.sh` |
| 配置集中 | 裸 `os.Getenv`（排除 config/bootstrap/migrate）= 0 | `grep -rn 'os\.Getenv' internal/ --include='*.go' \| grep -v _test.go \| grep -v config \| grep -v bootstrap \| grep -v migrate` |
| 构建门禁 | `go build ./...` 零错误；`npx tsc --noEmit` 零错误 | CI |
| Schema 门禁 | `check-migrations.sh` / `check-db-conventions.sh` / `check-sql-portability.sh` / `check-schema-parity.py` / 重放验证 全部 PASS | CI |
| 审计嵌入 | 每个业务 app 包提供 `audit_outbox.go`（业务写入与审计记录、事件同事务） | `ls internal/app/*/audit_outbox.go` |
| 安全断言 | 安全能力在双方言实现同接口同语义（如 DPoP jti 重放检测） | `ls internal/infra/postgres/dpop/ internal/infra/mysql/dpop/` |
| 错误预算 | 错误预算消耗 > 25% → 收紧发布（冻结新特性与重构） | [generated/slo-registry.md](../generated/slo-registry.md) 口径 |

### 6.2 SLO、容量与探针

| 项 | 约束 |
|------|------|
| SLO 唯一口径 | [generated/slo-registry.md](../generated/slo-registry.md)，按面分桶（管理面 / 认证面 / 协议面），不得用单一数字概括全部接口 |
| 容量口径 | 以 L2 实测校准（[observability.md](../architecture/observability.md) §2 分层口径）；示例值不是承诺值 |
| 探针 | liveness / readiness / startup 三探针分离；`/readyz` 需 token 保护并校验 DB 连通 |
| 资源边界 | 副本数、PDB、HPA 与连接池上限由压测确定；Redis 与数据库故障时的降级姿态按面区分 |
| 燃尽率告警 | `deploy/alertmanager-rules.yml` 的 `guliangiam_slo_budget` 组：5m/1h/28d 错误率 + 14.4x 多窗口燃尽率（99.9% 面 critical、99.5% 面 warning） |
| 发布门禁 | `make slo-budget SLO_PROMETHEUS_URL=…`（剩余预算 < 10% 退出 1）；CI `slo_gate` job 在仓库变量配置后执行同一门禁 |
