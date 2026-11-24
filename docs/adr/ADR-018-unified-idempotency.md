# ADR-018: 统一幂等性

- 状态：[Current]/[Target] — 统一 Idempotency-Key 中间件、四种领域专用幂等与 PostgreSQL idempotency_keys TTL 清理已落地；异步 Outbox 渠道天然幂等。依据：中间件挂载、两种方言适配、TTL 清理与拒绝分支单测均已取证，见「验证证据」；容量监控/告警与 ADR-008/ADR-017 消费者联动验证为 Target（出口条件：监控告警接线 + 联动验证取证）
- 日期：2026-09-14

## 背景

客户端重试、网关超时重发与用户重复点击，会让同一笔写请求到达服务端多次，后果是重复用户、重复订单、重复扣款与重复 Webhook。当前各模块已经各自长出了“局部幂等”实现，但语义与覆盖面不一致：新模块没有可复用的统一机制，每次都要重新判断“这次算不算重复”。需要把通用 HTTP 写幂等收敛成可测试的中间件，同时保留票据/令牌/事件等领域专用机制各自的正确形态。

## 决策

对有副作用且可安全重试的写接口统一使用 `Idempotency-Key` 请求头：服务端以“幂等键 + 请求体哈希”为唯一约束存储首次响应，重复请求返回缓存响应并标记 `Idempotency-Replayed: true`；“同键不同请求体”判定为冲突（409），不得覆盖首次请求。领域专用幂等保持各自机制：Webhook 以 `event_id` 去重、一次性票据/Nonce 以条件更新原子消费、刷新令牌以 family 重用检测整族撤销（RFC 6819/OAuth 2.1 威胁模型）。

## 实现证据（`[Current]` 子事实）

- **授权码一次性**：`internal/app/auth/token_exchange.go` 以 `UsedAt != nil` 或过期判定重复使用并拒绝。
- **Web3 Nonce 原子消费**：`internal/infra/postgres/web3identity/sql.go` 的 `UPDATE ... WHERE nonce = $1 AND consumed = false AND expires_at > NOW() RETURNING ...`，靠数据库行锁保证只消费一次。
- **刷新令牌 family 重用检测**：`internal/app/auth/token_reuse_detection.go` 的 `detectTokenReuse` + `revokeTokenFamilyUoW`（经 `Tokens().RevokeByFamilyID` 整族撤销）。
- **Webhook 事件投递去重**：outbox 单轨（H40），扇出以 `(事件 ID, webhook ID)` 派生的确定性投递主键 + `ListExistingIDs` 预检去重（`internal/app/webhooks/domain_outbox.go`），投递由 `platform/outbox.Worker` + `DeliveryOutboxHandler`（`internal/app/webhooks/delivery_outbox.go`）执行；接收侧以 `X-GuliangIAM-Delivery`（event_id）吸收 at-least-once 重复。遗留 `webhook_events` 直轮询 Worker 已删除（口径见 ADR-008/ADR-017）。
- **统一承载表已建**：`guliangiam.idempotency_keys`，含 `uq_idempotency_keys_scope_key (scope, idempotency_key)` 唯一约束、`status IN ('in_progress','succeeded','failed')` CHECK 与 `expires_at` TTL 索引。

## 统一中间件实现

- 统一中间件已落地：`internal/platform/middleware/idempotency.go`（请求体 SHA-256、同键同体重放 + `Idempotency-Replayed: true`、同键异体 409、`in_progress` 并发 503 + `Retry-After: 2`）；端口定义在 `internal/domain/idempotency/repository.go`，`internal/app/idempotency/service.go` 仅转出类型别名与 `ScopeFor`；`internal/infra/postgres/idempotency/repository.go` 与 `internal/infra/mysql/idempotency/repository.go` 提供两种方言适配器；组合根在受保护路由挂载（`internal/bootstrap/chi.go` 的 `IdempotencyMiddleware`，中间件内仅处理 POST/PUT/PATCH）。
- `grep -rn 'Idempotency-Key' internal --include='*.go' | grep -v _test` 命中中间件与 CORS 白名单；`grep -rln 'idempotency_keys' internal --include='*.go'` 命中领域端口、两种方言仓储与生成类型。
- TTL 清理已落地：`internal/infra/postgres/idempotency/repository.go` 与 `internal/infra/mysql/idempotency/repository.go` 实现 `DeleteExpired`（有界 `LIMIT` 批量删除），由 `internal/bootstrap/wire_session_cleanup.go` 的维护节拍回收 `expires_at` 已过期的行；请求路径仍以 `expires_at > NOW()` 过滤，清理失败不影响主流程。
- 剩余待办（不改变本 ADR 的 `[Current]` 结论）：与 ADR-008/ADR-017 消费者幂等的联动验证、容量监控与告警；完成后在「验证证据」追加取证结果。

## 理由与后果

- 正面：重复请求的判断从各模块临时逻辑收敛为一处可测试的中间件行为；客户端安全重试；首次响应可复现，避免“服务端已扣款但客户端看到超时”的争议。
- 负面：幂等表持续增长需要 TTL/归档治理（与 ADR-011 相同的容量纪律）；“重放”与“冲突”的区分要求严格按请求体哈希判定，中间件不能只按键存在与否短路；缓存的首次响应涉及响应序列化与权限上下文（重放者必须仍是原调用方）。

## 替代方案

- **各模块继续各自实现幂等**：放弃理由——语义不一致、无法统一测试、漏网模块直接产生重复写；现状已证明同一代码库中长出了至少四种不同机制。
- **只靠数据库唯一约束兜底**：放弃理由——只能防止重复插入行，无法重放首次响应、无法覆盖多步副作用与“先扣款后通知”类链路，也表达不了同键异体冲突。
- **由 API 网关全包幂等、应用不做**：放弃理由作为当前方案——当前部署中没有提供该能力的网关，且下游（Webhook 消费者、Worker）仍需自身幂等；网关方案保留为复审项。

## 验证证据

**已取证（`[Current]`）**：中间件接入 `internal/bootstrap/chi.go` 受保护路由；`go test ./internal/platform/middleware -run TestIdempotencyMiddleware -v` 覆盖三类拒绝分支——请求体不可读 400、同键异体 409、`in_progress` 并发 503 + `Retry-After: 2`；`go test ./internal/app/idempotency -run TestScopeFor -v` 覆盖 `ScopeFor` 的租户/路径归并（组织 slug + 路径，无租户时回退为路径）。

**遗留项（不改变 `[Current]` 结论）**：同键同体重放（缓存响应 + `Idempotency-Replayed: true`）与成功响应 `Complete` 落库尚无专门中间件单测；容量监控与告警、与 ADR-008/ADR-017 消费者幂等联动验证尚未取证。

## 复审条件

网关层提供并强制全局幂等保证、且下游全部支持自身幂等时复审中间件的去留；引入 gRPC/异步消息渠道需要新键模型（消息 id / 幂等属性）时复审；`idempotency_keys` 增长触及容量阈值时联动 ADR-011 复审清理策略。
