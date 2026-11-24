# ADR-010: Redis 仅作可重建缓存

- 状态：[Current]/[Target] — Redis session cache 适配器 + 端口注入已落地；session cache 故障姿态为 fail-open（Redis 不可用时退化内存/真源查询，不绕过认证/授权，事实源始终是 PG）；限流装配级回退已保守收敛（地址未设置 → WARN 宣告单实例假设；启动 ping 失败 → 进程内兜底阈值收紧 50%，`internal/bootstrap/wire_cache.go`，见 ADR-019）；Redis 宕机演练与三类用途 TTL 复核为 [Target]
- 日期：2026-09-14

## 背景

多实例部署后，短期挑战状态、热点元数据与限流计数需要低延迟、可共享的存取层；但缓存一旦被当作事实源，缓存丢失、淘汰或重启就会造成认证/授权状态分叉。GuliangIAM 的事实源已定为 PostgreSQL（ADR-002），因此在引入任何内存型存储之前必须先限定其角色与故障姿态。

## 决策

Redis 仅用于分布式限流（ADR-019）、短期挑战状态与**可重建**元数据缓存（如 Casbin 策略、规则配置）；身份、授权、会话与令牌事实始终以 PostgreSQL 为准；缓存键必须带 TTL，Redis 不可用时对高风险操作采取保守策略（拒绝或降级到本地保守限流），不得因缓存故障绕过认证/授权，更不得静默放行。

## 理由与后果

- 正面：热点读与限流计数从 PG 卸载，多实例间可共享短期状态；缓存全损后可从 PG 重建，不影响正确性；故障姿态明确（Fail-Close 优先；限流装配级回退已保守收紧、session cache fail-open 不绕过认证/授权，见验证证据）。
- 负面：引入新的中间件运维面（持久化策略、内存淘汰、网络分区）；缓存一致性需要失效/TTL 治理，处理不当会读到陈旧策略；Redis 宕机期限流要保守，可用性上等于“限流依赖故障即拒绝部分流量”，必须让业务知情。

## 替代方案

- **缓存写穿并作为部分事实源（会话/挑战以 Redis 为准）**：放弃理由——Redis 的持久化与备份恢复能力弱于 PG，丢失即认证状态分叉，直接违反 ADR-002。
- **永久不引入 Redis，只做进程内缓存 + PG 计数**：放弃理由作为**目标态**——多实例下进程内限流总阈值随实例数放大（ADR-019），热点元数据压全部落到 PG；该方案是当前实际现状，在单实例/小规模阶段可接受，但不是多实例目标。
- **用 memcached 等纯易失缓存替代**：放弃理由——能力是 Redis 的子集（无 Lua 原子脚本、无成熟限流原语），而 ADR-019 的令牌桶正需要原子脚本，多维护一个组件没有收益。

## 验证证据

**现状事实（[Current] 子事实）**：

- `github.com/redis/go-redis/v9` 已成为依赖；Redis session cache 适配器在 `internal/infra/cache/session_cache_redis.go`，驱动选择与故障姿态在 `internal/bootstrap/wire_cache.go`（`SESSION_CACHE_DRIVER=memory|redis`，默认 memory；Redis 不可用 fail-open 到真源查询、并带电路/指标）。
- 进程内缓存仍保留：`internal/infra/casbin/cache.go`（Casbin 策略缓存）；限流的 Redis 实现对偶见 ADR-019 证据。
- 现状是「适配器已实现 + 部署可选」，不等于「Redis 已成为默认多实例缓存」；出口条件仍以真实多实例部署与故障演练为准。
- **故障姿态（如实登记）**：session cache 回退内存/PG 真源不绕过认证/授权（符合「不得因缓存故障绕过认证/授权」的底线）；限流在 `RATE_LIMIT_REDIS_ADDR` 未设置时回退进程内令牌桶并以 WARN 宣告单实例假设（多实例部署必须配置），启动 ping 失败时标记 `SetRateLimitRedisUnavailable`、进程内兜底 Rate/Burst 各收紧 50%（Burst 最小 1，客户端可见 Policy 配额同步减半）——装配级保守收敛已实施，但多实例下仍非分布式共享桶的等效防护。请求级 Redis 故障（已启用 Redis 后）由 `redisLimiter.Try` Fail-Close 拒绝（429），见 ADR-019。

**出口条件（[Target] 待取证项）**：Redis 访问经端口注入而非全局直连；限流/挑战/元数据三类用途各自的 TTL 与失效策略文档化；完成 Redis 宕机演练并证明：认证/授权不被绕过、限流 Fail-Close（含装配级 50% 保守兜底实测）、事实性查询不依赖缓存命中。

## 复审条件

ADR-019 分布式限流落地时联动评审（两者共享 Redis 依赖与故障姿态）；若实际多实例规模长期可由 PG + 本地缓存承载，则本 ADR 维持 [Target] 不急于实施；引入多区域部署时重评缓存拓扑与跨区一致性。
