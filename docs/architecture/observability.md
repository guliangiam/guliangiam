# observability — 可观测性 + 性能基线

> 目标态规范「observability — 可观测性 + 性能基线」；写作约束见 [docs/README.md](../README.md)。

## 1. 可观测性四支柱

### 1.1 Metrics（指标）

**事实源**：Prometheus 指标注册（`internal/platform/middleware/prometheus.go`，手写实现；命名遵循 `contracts/registries/naming.yaml` N14）+ `contracts/registries/slo.yaml`（SLO/错误预算口径）。

| 指标类别 | 说明 | 标签纪律 |
|----------|------|---------|
| RED 指标 | Rate（请求率）、Errors（错误率）、Duration（延迟） | 带 `surface` 标签（management/authentication/protocol）；禁止高基数 |
| 资源指标 | Go runtime、DB 连接池、Redis 连接池、内存、CPU | 标准 exporter；连接池满载时 PoolGuard 快速失败（见 §2） |
| 业务指标 | 登录成功率、令牌签发量、审计写入量、计量事件量 | 分面统计 |

**分面 SLO**：SLO/错误预算唯一口径见 `contracts/registries/slo.yaml`。SLI 精确公式、28 天滚动窗口与错误预算换算、错误预算策略、告警四级响应时限与升级路径，均以该注册表为准。

### 1.2 Tracing（追踪）

**事实源**：OpenTelemetry TracingMiddleware（`internal/platform/middleware/tracing.go`）。

- W3C Trace Context 传播
- OTLP exporter 与采样器为部署配置要求：endpoint/资源属性/采样器 `ParentBased(TraceIDRatioBased(1%))`，错误 100%；TracerProvider 装配位置 `internal/bootstrap/telemetry.go`，exporter 与采样器的配置口径见《开发规范手册》可观测章节
- `OTEL_*` 变量按《开发规范手册》可观测章节配置
- 跨链路关联：`request_id` / `trace_id` / `audit_id` 三元关联

### 1.3 Logs（日志）

**事实源**：slog + 脱敏 Handler（`internal/platform/logging/sanitizer.go`）。

- 结构化日志（slog）
- 敏感字段自动脱敏（Token/Password/Secret/PII → `***`）
- 统一访问日志中间件（`internal/platform/middleware/access_log.go`）：含 `request_id`/`trace_id` 二元关联 + `path`（chi 路由模板，不含 UUID 与 query）、`status`、`durationMs`、`bytes`、`surface`；5xx 记 Error，其余记 Info；`/healthz`、`/livez`、`/readyz`、`/metrics` 不写日志（秒级轮询无诊断价值）。包装器必须透传 `Flush`/`Hijack`/`Unwrap`，否则 SSE 与 WebSocket 会因能力断言失败而不可用。
- 日志级别与输出目标由环境变量控制

### 1.4 Audit（审计）

**事实源**：`records` 表 + 哈希链（ADR-031）。

- 审计记录与业务同事务写入，与业务同生共死；正常路径在同一事务内计算链列并写 `chain_status='chained'`（《data-model》§4.1，ADR-031；`internal/infra/postgres/records/sql.go`）
- `chain_status='pending'` 仅出现在写入器失败/熔断分支：新条目登记 pending，由链构建 Worker 在独立事务中补链；链头/链尾锚定同样由 Worker 完成并推进 `chained → anchored`，失败重试不阻塞业务（《data-model》§4.4）
- 含 `surface`/`request_id`/`trace_id` 列
- 哈希链列（`prev_hash`/`entry_hash`/`chain_seq`/`chain_status`/`chain_algo`）支持审计防篡改
- WORM 归档与签名导出（平台签名密钥 + 清单校验和）
- 指标：`guliangiam_audit_chain_pending_entries`（gauge，`records` 表中 `chain_status='pending'` 的条目数）、`guliangiam_audit_chain_build_duration_seconds`（histogram，链构建 Worker 处理耗时）；跨区域 trace 丢失时审计降级为 `request_id` + `audit_id` 二元关联并暴露 `guliangiam_trace_missing_total`（指标命名遵循命名裁决 N14：`guliangiam_{域}_{对象}_{单位}`）

## 2. 性能基线（L0-L3 分层）

本节定义测量方法与分层口径；**生产 SLO 不复写**，唯一口径见 [contracts/registries/slo.yaml](../../contracts/registries/slo.yaml)。

| 层级 | 测量内容 | 工具 | 能否作为生产 SLO |
|------|---------|------|-----------------|
| L0 | 哈希、加密、UUID、策略缓存等纯函数 | Go benchmark | 否，只用于回归 |
| L1 | Repository、事务、索引和连接池 | Go integration test、pgbench | 部分，可形成数据库目标 |
| L2 | HTTP API、登录、令牌、分页 | k6、OpenTelemetry、Prometheus | 是，需固定环境和数据规模 |
| L3 | 多实例、故障、恢复和峰值流量 | k6、故障注入、恢复演练 | 是，形成发布门槛 |

**L0 回归项示例**（仅用于代码回归判定，不得作为生产 SLO）：

- Session 结构体创建、Session 过期检查
- JWT 签发/验证（RS256）
- Casbin Enforce（缓存命中/未命中）
- SHA-256 Token 哈希
- AES-256-GCM 加解密
- UUIDv7 生成

> **L0 使用红线**：L0 数字**仅用于判定回归**（同一硬件/Go 版本下相对历史基线的变化在 ±10% 以内即视为无回归）。**严禁把 L0 绝对值与 L2 API 链路 SLO 相除或相减得出「N 倍余量」**——微基准不含 DB、网络、中间件、序列化与排队，调用路径完全不同，相除没有工程意义。

**必须在 L2/L3 环境重新验证的指标**：

- API P50/P99 响应时间
- 登录认证完整流程
- 并发用户数
- 列表查询（含 DB 查询）
- Casbin Enforce（真实策略）

## 3. 可观测基线配置

| 组件 | 配置 | 事实源 |
|------|------|--------|
| OTLP exporter | endpoint/资源属性/采样器 `ParentBased(TraceIDRatioBased(1%))`，错误 100% | 《开发规范手册》可观测章节 |
| `/metrics` 端点 | 独立端口 + 鉴权 | — |
| Web Vitals RUM | 前端管道 | — |
| 分面 SLO 度量 | Prometheus 数据管道 | `contracts/registries/slo.yaml` |
| 审计追踪列 | 随 `records` 表结构演进 | 声明式 Schema |

---

## 4. 指标与告警契约

> 本章给出指标**命名与标签契约**及告警规则要求；不登记实现状态，现行实现以 `internal/platform/middleware/prometheus.go` 与 `deploy/alertmanager-rules.yml` 为准。

### 4.1 指标命名与标签契约

指标注册名遵循 `guliangiam_{域}_{对象}_{单位}`：

| 指标 | 归属实现 | 类型 | 标签 |
|------|------|------|------|
| `guliangiam_http_requests_total` | `platform/middleware/prometheus.go` | Counter | method / path / status / surface |
| `guliangiam_http_request_duration_seconds` | 同上 | Histogram | method / path / surface |
| `guliangiam_http_requests_in_flight` | 同上 | Gauge | method |
| `guliangiam_auth_login_total` | 同上 | Counter | organization / method / status |
| `guliangiam_auth_login_errors_total` | 同上 | Counter | organization / method / error_type |
| `guliangiam_db_query_total` | `infra/postgres/shared/query_monitor.go` | Counter | table / operation |
| `guliangiam_db_query_duration_seconds` | 同上 | Histogram | table / operation |
| `guliangiam_db_slow_query_total` | 同上 | Counter | table / operation / threshold |
| `guliangiam_casbin_enforce_duration_seconds` | `infra/casbin/metrics.go` | Histogram | resource / action |
| `guliangiam_jwt_sign_duration_seconds` | `infra/jwt/metrics.go` | Histogram | alg / kid |
| `guliangiam_jwks_refresh_total` | `platform/jwks/metrics.go` | Counter | result（success、failure） |
| `guliangiam_jwks_unknown_kid_total` | 同上 | Counter | outcome（resolved、rejected、cooldown、throttled、refresh_failed） |
| `guliangiam_argon2_duration_seconds` | `platform/crypto/metrics.go` | Histogram | profile |
| `guliangiam_provider_resolution_total` | `app/providers/metrics.go` | Counter | family / channel / result（cache、database、fallback、missing、disabled、build_error、error） |
| `guliangiam_provider_cache_hit_total` | 同上 | Counter | channel |
| `guliangiam_provider_cache_miss_total` | 同上 | Counter | channel |
| `guliangiam_provider_cache_bypass_total` | 同上 | Counter | channel / reason（no_trusted_scope、system_scope、organization_mismatch） |
| `guliangiam_provider_cache_eviction_total` | 同上 | Counter | channel / reason（expired、capacity、invalidate） |

**标签约束**：`surface` 取值由 `transport/http/route` 的请求面分类派生（**management / authentication / protocol**，三值闭集），不得由 handler 自行传入，也不得出现 `admin` / `auth` / `data` 等别名。`provider_*` 指标的 `result`/`reason` 是闭集，`channel` 取值来自 messaging 渠道枚举；任何指标都不得携带组织 ID、用户 ID 或 provider ID。

### 4.2 NFR 阈值 → 告警规则要求

| NFR 阈值 | 指标 | 告警表达式 |
|---------|---------------------|-------------------------------------------------------------|
| Admin API P99 < 500ms | `..._http_request_duration_seconds{surface="management"}` | `histogram_quantile(0.99, rate(..._bucket{surface="management"}[5m])) > 0.5` |
| Auth API P99 < 100ms | `...{surface="authentication"}` | `histogram_quantile(0.99, ...) > 0.1` |
| Auth API P50 < 30ms | 同上 | `histogram_quantile(0.5, ...) > 0.03` |
| DB P99 < 30ms | `..._db_query_duration_seconds` | `histogram_quantile(0.99, ...) > 0.03` |
| 平台吞吐 ≥ 1000 req/s | `..._http_requests_total` | `sum(rate(..._total[1m])) < 1000`（低于基线告警） |
| Casbin Enforce P99 < 2ms | `..._casbin_enforce_duration_seconds` | `histogram_quantile(0.99, ...) > 0.002` |
| JWT 签发 P99 < 1ms | `..._jwt_sign_duration_seconds` | `histogram_quantile(0.99, ...) > 0.001` |
| Argon2id P99 < 100ms | `..._argon2_duration_seconds` | `histogram_quantile(0.99, ...) > 0.1` |
| MTTD < 5min | `up{job="guliangiam"}` | `up{job="guliangiam"} == 0` 持续 5m |
| MTTR < 15min | Pod 重启计数 | `rate(kube_pod_container_status_restarts_total[15m]) > 0` |
| SLA ≥ 99.9% | 错误率 | `sum(rate(..._total{status=~"5.."}[5m])) / sum(rate(..._total[5m])) > 0.001` |

规则文件的唯一位置是 `deploy/alertmanager-rules.yml`，其阈值必须与本表逐条一致。

### 4.3 histogram bucket 定制

| Surface / 指标 | Bucket | 对齐 NFR |
|---------|------------|---------|
| `authentication` | `[0.001, 0.005, 0.01, 0.025, 0.05, 0.1]` | Auth P99 < 100ms |
| `management` | `[0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1]` | Admin P99 < 500ms |
| `protocol` | `[0.005, 0.01, 0.025, 0.05, 0.1]` | OAuth P99 < 100ms |
| `guliangiam_db_query_duration_seconds`（无 surface 标签） | `[0.001, 0.005, 0.01, 0.02, 0.03, 0.05]` | DB P99 < 30ms |

---

## 5. 访问日志、缓存与限流维度核对

访问日志、缓存与限流口径由实现路径与门禁共同约束：访问日志装配见 `internal/bootstrap/chi.go` 与 `internal/platform/middleware/access_log.go`；认证面缓存见 `internal/transport/http/middleware/etag.go`；限流见 `internal/platform/middleware/ratelimit.go`；会话缓存见 `internal/infra/cache`。实现偏差与待关闭项登记在 `contracts/registries/open-items.yaml`，本文不复制实现证据表。

限流命中会进入访问日志（`status=429`）与 Prometheus 计数器；测试环境通过
`RATE_LIMIT_AUTH_RATE/BURST`、`RATE_LIMIT_PER_IP_REQUESTS` 覆盖到压测值，生产默认值不变。
