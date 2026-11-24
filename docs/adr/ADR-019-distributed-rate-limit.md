# ADR-019: 分布式限流

- 状态：[Current]/[Target] — Redis + Lua 分布式令牌桶适配器已落地，请求级 Fail-Close 已实现（Redis 出错即拒绝 429，`redisLimiter.Try`）；装配级回退已保守收敛（地址未设置 → WARN 宣告单实例假设、保持全额；启动 ping 失败 → 进程内兜底阈值收紧 50%，`internal/bootstrap/wire_cache.go` + `SetRateLimitRedisUnavailable`）；多实例一致性、Redis 宕机演练与配置变量并轨仍为 [Target]
- 日期：2026-09-14

## 背景

现有令牌桶限流在每个进程内独立计数：单实例部署时限流语义正确，但一旦水平扩容到 N 个实例，同一主体（IP/用户/租户）的请求被分散到 N 个桶，实际放行总量等于“单实例阈值 × N”，攻击者只需让请求命中不同实例即可绕过认证面的暴力破解防护。多实例部署已是既定形态（模块化单体的扩容方式，见 ADR-001），限流必须先行收敛到共享计数。

## 决策

多实例部署下限流从进程内计数器迁移到 Redis + Lua 原子令牌桶（取令牌与 TTL 续期在同一脚本内完成，消除竞态）；限流维度包括全局、租户、用户、IP 与敏感端点（登录/令牌/重置密码）分组；拒绝时返回 429 与 `Retry-After`；Redis 不可用时退化为保守的本地限流（Fail-Close 优先，宁紧勿松），限流键必须带过期避免内存无限膨胀。

## 现状事实（`[Current]` 子事实）

- `internal/platform/middleware/ratelimit.go`：`TokenBucketLimiter` 基于 `golang.org/x/time/rate` + 进程内 `sync.Mutex map[string]*tokenBucket`，带过期条目清理；`internal/transport/http/middleware/service_ip_limit.go` 同为进程内 map 的服务-IP 维度限制。两者的限流反馈头由 `internal/platform/ratelimit` **唯一**写出（IETF `RateLimit-Limit`/`RateLimit-Remaining`/`RateLimit-Reset`/`RateLimit-Policy` + 429 时的整数秒 `Retry-After`，见手册 4.4.1）；<!-- naming-allow：本行仅声明该字面量已废弃，不得在代码或契约中使用 --> 已废弃的 `X-RateLimit-*` 不再出现在任何响应中。**各分档的具体数值不在本 ADR 重复**，唯一权威表见《架构设计》8.19（环境变量登记见手册附录 D）。
- `grep -i redis go.mod` 命中 `github.com/redis/go-redis/v9`：分布式限流适配器在 `internal/platform/middleware/ratelimit_redis.go`（Redis + Lua 原子令牌桶，`RATE_LIMIT_REDIS_ADDR` 配置启用）；组合根装配与故障姿态见 `internal/bootstrap/wire_cache.go`。故障姿态分两层：**请求级 Fail-Close 已实现**——启用 Redis 后任何脚本/网络错误由 `redisLimiter.Try` 返回拒绝（429 + `Retry-After`），不放行；**装配级回退已保守收敛**——地址未设置时回退进程内令牌桶并以 WARN 显式宣告单实例假设（多实例部署必须配置 `RATE_LIMIT_REDIS_ADDR`，否则总放行量 = 单实例阈值 × N）；启动 ping 失败时标记 `SetRateLimitRedisUnavailable`，进程内兜底 Rate/Burst 各收紧 50%（Burst 最小 1），客户端可见 Policy 配额同步减半，多实例下总放行量上界 = 实例数 × 半额——仍非分布式共享桶的等效防护（演练出口见下）。
- 进程内实现（`internal/platform/middleware/ratelimit.go` + `service_ip_limit.go`）保留为默认与降级底座；剩余证据是多实例一致性故障演练与阈值标定（装配级回退的保守收敛已实施，见上条）。

## 统一目标（[Target]，避免把规划写成已建成）

- 待建：租户/用户维度的全面装配（`WithKeyFunc` 钩子已存在，默认按可信客户端 IP）；多实例一致性测试（同一主体跨实例取桶总量不超阈值）；Redis 故障注入演练证明端到端 Fail-Close（含启动 ping 失败时 50% 兜底实测）；限流配置变量与现有 `RATE_LIMIT_RPM`、`RATE_LIMIT_GLOBAL_RATE/BURST`、`RATE_LIMIT_AUTH_RATE/BURST` 命名漂移并轨（见 ADR-021 与 STD 附录 D）。Redis Lua 令牌桶脚本、限流器注入端口与装配级回退保守收敛（50% 收紧）已建成（见现状事实），不再列入待建。

## 理由与后果

- 正面：多实例下速率语义与实例数解耦，认证面防爆破保护不再可被“打不同实例”绕过；集中桶可动态调参并提供准确的 `Retry-After`。
- 负面：限流热路径引入 Redis 网络往返与单点依赖（必须设计降级与本地兜底）；Lua 脚本与 Redis 版本需要纳入运维基线；限流失效姿态选 Fail-Close 意味着 Redis 故障会拒绝部分正常流量，需要与 SLO/容量联动评估（ADR-020）。该姿态在请求级成立；装配级回退已保守收紧（50%）但仍非分布式共享桶的等效防护，多实例部署不得把进程内兜底当作等效防护（见现状事实）。

## 替代方案

- **维持进程内计数（现状固化）**：放弃理由作为多实例目标——总放行量随实例数线性放大，等于把限流阈值的决定权交给攻击者的负载均衡命中策略；单实例阶段可以接受，扩容即失效。
- **用 PostgreSQL 做集中计数**：放弃理由——高 QPS 限流写入会对身份事实源造成写放大与行锁竞争，违背 ADR-002 对 PG 的保护定位；限流状态本就属于“可丢失的派生状态”（ADR-010）。
- **网关/服务网格统一限流、应用不做**：放弃理由作为当前方案——当前部署没有承担租户/用户维度的网关，且该维度数据在应用侧；保留为复审项；若落地，敏感端点的应用内防线仍应保留（纵深）。

## 验证证据（出口条件，Target 待取证）

现状证据见上节（装配级回退保守收敛已实施：`TestRateLimit_RedisUnavailableTightensInProcessFallback`、`TestRateLimit_RedisUnavailableKeepsMinimumBurstOne`、`TestRateLimit_RedisInjectionOverridesUnavailableFlag`）。目标态出口：Lua 脚本原子性评审与测试；多实例一致性测试报告；Redis 宕机 Fail-Close 演练记录（含启动 ping 失败时 50% 兜底实测）；限流键 TTL 与内存上界验证；与 ADR-021 配置变量并轨后转 `[Current]`。

## 复审条件

网关或服务网格已提供满足租户级维度的一致全局限流能力时复审分工（应用保留敏感端点兜底）；若部署形态长期维持单实例，本 ADR 可保持 [Target] 不实施但须在扩容前完成；ADR-010 Redis 落地时联动评审，二者共享一次故障演练。
