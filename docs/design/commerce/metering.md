# 计量与配额管道

> 设计锚点：[DESIGN:commerce/metering]

> 目标态规范「计量与配额管道」；写作约束见 [docs/README.md](../../README.md)。

---

## 定位

计量管道将业务事实（可计费事件）转化为配额扣减与计费导出的账期记录。本域是**商业计量域**（声明事实源 `contracts/schema/90_metering.yaml`）。AI 网关的用量计量（Token 计数、配额与价格）归 AI 网关域（见《[design/ai-gateway/README.md](../ai-gateway/README.md)》）；其用量事件 `ai.usage.recorded` 经 Outbox 消费者写入本域 `metering_events` 作计费归集（桥接实现 `internal/app/aigateway/outbox_handlers.go`），AI 侧审计表 `llm_metering_events` 仍归 AI 网关域。支付流水不进入本域（见《[payment.md](payment.md)》裁决）。

## 数据模型

结构唯一事实源：`contracts/schema/90_metering.yaml`（复现：`python3 scripts/gen_schema.py --check`）；领域模型：`internal/domain/metering/metering.go`（`MeteringEvent` / `MeteringAggregate` / `Period`）与 `quota.go`（`TenantQuota`）。列/约束/索引清单不复制到本文，只表达语义：

| 表 | 语义 |
|----|------|
| `metering_events` | 追加型可计费事件：`event_type` + `subject`（计费归集维度，如 user/app/pool）+ `quantity`/`unit` + `occurred_at` + `metadata`；幂等键为 `idempotency_key`，组织内唯一防重（`uq_metering_events_org_idem`） |
| `metering_aggregates` | 账期预聚合：按 `(organization_id, period, subject, metric)` 单元 upsert（`uq_metering_aggregates_org_period_subject_metric`），`period` 为账期窗口（月 `YYYY-MM` / 周 `YYYY-Www`，见 domain `Period`），`metric` 为事件类型求和或派生指标；喂数 COMM 计费边界 |
| `plan_quotas` | 套餐配额模板：组织 + plan + `quota_key` 唯一，`soft_limit`（可空告警线）/`hard_limit`（拒绝线）/`unit` |
| `tenant_quotas` | 租户配额实例：组织 + `quota_key` 唯一，`limit_value`/`used_value`，扣减以 `revision` 仲裁并保留账期对账 |

索引以 schema 声明为准（事件表按组织 + 事件类型与组织 + 发生时间两个访问路径建索引）。事件表高增长：按时间分区（ADR-011 证据后启用）。

## 管道时序

```
业务事实 → metering_events → Outbox Worker 聚合 → metering_aggregates → 配额扣减 → 计费导出
```

1. **事件产生**：业务事实在事务内写 `metering_events`（幂等键 `idempotency_key`）；**失败不阻塞主链路**——写入异常旁路降级并告警
2. **聚合**：后台作业/Outbox Worker（投递语义见《[outbox.md](../infrastructure/outbox.md)》）按账期窗口聚合 upsert 入 `metering_aggregates`；乱序容忍；跨实例时钟偏移在窗口吸收
3. **配额扣减**：配额检查点在各写入/认证路径，执行点复用分布式限流设施（Redis+Lua，ADR-010）
4. **计费导出**：`metering_aggregates` 经对外契约导出/投递，**计费逻辑不在本模块**（METER-005 边界）

## 配额模型与套餐衔接

配额维度：限流速率、资源数量、计量速率峰值。与套餐档（free/basic/enterprise）绑定，三档差异化配额。配额变更即时生效并审计。

### 配额与限流协调

- **同一决策点串行**：配额检查（账期用量）与限流检查（Redis Lua）收敛到同一决策点，**先限流后配额**
- **配额超限响应**：429 + `RateLimit-Policy` 标注 quota 维度（可与限流类 429 区分）
- **扣减原子性**：配额扣减执行 Redis Lua check-and-decrement 原子脚本（单次往返）
- **域判定实现**：`internal/domain/metering/quota.go` 的 `TenantQuota.Consume` 给出 ok/soft/hard 三态与 429 元数据（软上限跨越告警、硬上限拒绝且不改变用量），`ApplySoftThreshold` 负责软阈值跃迁；Redis Lua 接线复用分布式限流设施（ADR-010），不另造扣减通道。
- **窗口一致性**：准入判定以 Redis 原子计数为准（实时性优先）；`metering_aggregates` 为账期事实（事后对账）
- **超卖防护**：软上限 110% 告警 + 硬上限拒绝双层；资源数量类在写入事务内计数 + revision 仲裁

## 配额 enforcement 落点

| 执行点 | 配额维度 | 超限响应 | 审计 |
|---|---|---|---|
| 认证 / 令牌签发 | 计量速率峰值 | 429 + `RateLimit-Policy`（quota 维度） | 写 `records` |
| 资源创建（用户 / 应用 / Service Account） | 资源数量 | 403 `forbidden` | 写 `records` |
| 计量写入 | 账期用量 | 旁路降级并告警（不阻塞主链路） | 写 `records` |
| 套餐 / 配额变更 | 配额即时生效 | 并发冲突 409（revision 仲裁） | 写 `records` |

双层阈值：软上限 110% 仅告警，硬上限拒绝；准入判定以 Redis 原子计数为准，账期事实以 `metering_aggregates` 为准。

## 旁路有界队列与溢出策略

- **有界队列**：进程内有界队列（默认值以代码实现为准）；生产端入队永不阻塞——队列满执行**丢弃最旧**（保最新事件）
- **溢出对账**：Worker 心跳事件携带窗口序号与队列水位，丢弃量计入账期对账报告
- **Outbox 写失败**：旁路消费写 `metering_events` 失败重试一次，仍失败落本地暂存；重放以 `idempotency_key` 幂等去重
- **死信与人工重放**：暂存重放超限进死信 + 告警，人工修复后重放入队

## 失败场景

| 场景 | 处置 |
|------|------|
| 计量写入依赖不可用 | 主链路旁路降级不失败；降级窗口丢弃量指标可观测；恢复后不回补（口径声明防争议） |
| 聚合竞态（多 Worker 同窗口重算） | `(org, period, subject, metric)` 唯一键 upsert 仲裁；幂等不双计 |
| 配额并发超卖 | 原子扣减 + 事务内 revision 仲裁兜底；残余竞态以周期对账修正并告警 |
| 计费导出下游失败 | Outbox 退避重试进死信；喂数不双计（聚合单元唯一键幂等对账） |

## 边界与复用

- 复用 Outbox/Worker 参数与死信（`contracts/registries/outbox.yaml`）、分布式限流设施（ADR-010）、套餐面（plans/subscriptions）、分区与清理纪律（`contracts/registries/lifecycle.yaml`）、审计（records）
- 不新建消息队列、不建计费模块（COMM 域边界）
- **关键**：旁路丢弃降级仅适用于计量与通知类旁路写入；认证与授权主链路不适用

## 互链

- Outbox → 《design/infrastructure/outbox.md》
- 套餐衔接 → 《design/iam/organizations.md》
- 分布式限流 → [ADR-019 分布式限流](../../adr/ADR-019-distributed-rate-limit.md)、[ADR-010 Redis 缓存](../../adr/ADR-010-redis-cache.md)
