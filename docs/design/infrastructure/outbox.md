# Outbox 一致性投递

> 设计锚点：[DESIGN:infrastructure/outbox]

> 目标态规范「Outbox 一致性投递」；写作约束见 [docs/README.md](../../README.md)。

---

## 定位

`outbox` 是 GuliangIAM **唯一**的异步投递设施。Webhook 投递、Outbox 事件扇出、SCIM 出站、SAML SLO Back-Channel Logout、计量聚合等**全部**复用本设施，不新建投递栈。

物理载体：`guliangiam.outbox` 表 + `guliangiam.idempotency_keys` 表。

## 依赖失败的处置原则

| 依赖 | 安全行为 |
|------|---------|
| PostgreSQL | 核心读写失败返回结构化错误 |
| Casbin | 授权不确定时 fail-close |
| Redis | 不得丢失事实状态；分布式限流失败时保守本地限流 |
| 外部 IdP | 熔断并允许其他认证方式 |
| 邮件/短信 | 不阻塞主事务；Outbox、重试、死信 |
| KMS/Vault | 无法解包时拒绝敏感操作 |

**统一规则**：事务内不执行不可控网络调用；外部副作用使用 Outbox、幂等键、指数退避、最大重试和死信。

## 领域事件清单

事件实现 `DomainEvent` 接口：`EventID`、`EventType`、`AggregateID`、`AggregateType`、`OrganizationID`、`OccurredAt`、`Payload`、`Version`。

核心事件：`user.created/updated/deleted/verified`、`organization.created/updated`、`application.created`、`session.created/revoked`、`token.issued/revoked/reused`、`policy.changed`、`key.rotated`。事件 schema 版本化（`outbox.event_version`），消费者必须幂等。

## Outbox 状态机

状态机唯一事实源：`contracts/registries/state-machines.yaml` 的 `outbox` 条目（渲染层 `docs/generated/state-machine-registry.md`；复现：`python3 scripts/gen_registry_md.py`）。

| 转换 | 触发条件 |
|------|---------|
| `pending → processing` | Worker 领取（`FOR UPDATE SKIP LOCKED`） |
| `pending → dead` | 入队即判死（不可投递事件直接死信） |
| `processing → succeeded` | 投递成功 |
| `processing → failed` | 投递失败（更新 `attempts`/`last_error`） |
| `failed → pending` | 退避到期后可重投 |
| `failed → dead` | 超过 `max_attempts`（唯一口径见注册表） |

`dead` 只能由人工重放或清理任务处理。人工重放按 `event_id` 去重，保证不产生重复副作用。Worker 支持优雅停机并把未完成批次释放回 `pending`。

## Worker 参数与投递语义

Worker 参数（轮询间隔、批大小、租约时长与回收阈值、最大尝试次数、退避策略、去重 TTL）的唯一口径见 `contracts/registries/outbox.yaml` 与渲染层 `docs/generated/outbox-registry.md`；本文件不复制参数值（复现：`python3 scripts/gen_registry_md.py`）。

- 死信处置：迁入 `outbox_dead_letters` + 审计 + 告警；原行保留供追溯
- `event_id` 去重作用域：`(aggregate_type, aggregate_id, event_type)`；人工重放与消费侧幂等键同口径

> **跨方言约束（ADR-023）**：`FOR UPDATE SKIP LOCKED` 是 PostgreSQL 专有语法。MySQL 8.4+ 支持同一语法；其他方言需由 `Dialect.DequeueBatch` 改写。三条不变式：(1) 同一事件不会被两个 Worker 同时持有；(2) 批次取件在**短事务**内完成；(3) 优雅停机的批次归还语义不变。

## Webhook 投递参数

参数值唯一口径见 `contracts/registries/outbox.yaml`（webhook 节），本文件只引用键名不复制数值：

| 参数 | 注册表键 | 语义 |
|------|---------|------|
| 签名头 | `signature_header` / `signature_v1_desc` | HMAC-SHA256 签名（密钥为解密后的 `webhooks.secret`），实现契约见 `internal/infra/webhook/http_sender.go` |
| 防重放时间窗 | `replay_window`（env `WEBHOOK_REPLAY_WINDOW_SECONDS`） | 窗外拒绝，`(t, v1)` 首次出现才接受 |
| 状态码重试分类 | `status_code_retry_policy`（`no_retry`/`immediate_dead`/`retry_with_backoff`） | 4xx（除 408/429）不重试；410 直接 `dead`；5xx/429/网络超时按退避重试 |
| 顺序保证 | `ordering` | 按 `(aggregate_type, aggregate_id)` 分区内**串行**投递，不提供全局顺序 |
| 出站超时 | `outbound_timeout` | 单次投递 HTTP 超时；`webhooks.timeout_ms` 列可覆盖 |

## SSRF 出口管控（覆盖所有出站 HTTP）

1. 禁内网 IP 段（RFC 1918 全部 + `127.0.0.0/8` + `::1`）与链路本地 `169.254.0.0/16`
2. 禁止跟随重定向；确需支持时**限 1 跳**且对目标重新执行全部校验
3. DNS rebinding 防护：解析后**固定 IP 连接**
4. URL scheme 白名单 `https`（开发环境允许 `http` 并在启动日志标注）

**覆盖范围**：Webhook、pgtUrl 回调、LDAP/Syncer 出站连接、SCIM 出站 Client。

## 统一幂等

`idempotency_keys` 表以 `(scope, idempotency_key)` 唯一，`request_hash` 为请求方法+路径+规范化请求体的指纹：

| 场景 | 行为 |
|------|------|
| 同键 + 同 `request_hash` + 已完成 | 返回已存储的响应，加 `Idempotency-Replayed: true` |
| 同键 + 同 `request_hash` + 处理中 | 返回 `409 conflict`（`Retry-After` 头） |
| 同键 + 不同 `request_hash` | 返回 `409 conflict` |

仅对 `POST`/`PUT`/`PATCH` 且带 `Idempotency-Key` 头的请求生效；键按 `expires_at`（建议 24h）清理。

### 幂等键与 If-Match 组合（判定顺序 = 先幂等、后乐观锁）

| 场景 | 判定结果 |
|------|---------|
| 键已存在且 request_hash 一致 | 直接重放缓存响应，**不**校验 If-Match |
| 键已存在、request_hash 不一致 | `409 conflict` |
| 键不存在、If-Match 不匹配 | `409 conflict`（乐观锁冲突） |
| 键不存在、If-Match 匹配 | 成功；幂等重放本身**不**递增 revision |
| `If-Match: *` | 视为无条件更新 |

### scope 构造

scope 由 `internal/domain/idempotency/repository.go` 的 `ScopeFor(orgSlug, userID, path)` 统一构造：`orgSlug + ":" + userID + ":" + path`（例：`acme:user-uuid:/api/v1/users`）。用户维度强制参与——否则同租户内任一用户可复用他人 Idempotency-Key 重放其缓存响应（跨用户数据泄露）；分隔符恒定输出，空 slug/userID 不会与其他身份组合别名。不存在按端点分级的 route / route+user / route+user+org 多套 scope。

### 双轨收敛

业务自带 `idempotency_key` 列（orders/payments 等）与统一 `idempotency_keys` 表并存。收敛规则：**统一走幂等表**；业务列标记废弃——收敛前双写（列值 = 幂等键），禁止新端点新增业务幂等列。

## 失败场景

| 场景 | 处置 |
|------|------|
| Worker 崩溃 | 租约回收扫描兜底（阈值见 `outbox.yaml` 的 `lease_reclaim_seconds`） |
| 同一事件被两个 Worker 领取 | `SKIP LOCKED` + 短事务领取杜绝 |
| 死信堆积 | 告警阈值（dead 行数 >0 即告警）+ 人工重放走 event_id 去重 |
| 去重表膨胀 | TTL 清理任务（见 `outbox.yaml` 的 `dedup_ttl`；与 `idempotency_keys` 不同层不同 TTL） |
