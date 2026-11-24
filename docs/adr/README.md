# Architecture Decision Records

ADR 记录架构决策（背景、决策、理由、后果、替代方案、验证证据、复审条件），不复制架构正文，也不登记变更历史。ADR 编号与状态以 `ls docs/adr/ADR-*.md` 和各文件头为准。

## 状态词规范（三态）

状态统一使用 `[Current]`、`[Target]`、`[Open]`：

- `[Current]`：决策已定且在代码/结构中已落地（允许存在已登记的 `[Open]` 偏差，偏差事实写入「验证证据」）。
- `[Target]`：决策已定方向但尚未建成；必须给出出口条件与待取证项。
- `[Open]`：已登记的偏差/风险/未决项。

决策被取代时，以新 ADR 明确「取代 ADR-xxx」，并在被取代 ADR 的复审条件中注明；不引入 `[Deprecated]`/`[Superseded]` 等额外流程态。

**混合状态写法**：同一 ADR 内存在两个并列事实、各自状态不同时，文件头标注 `[Current]/[Target]` 并在正文分小节分别给出证据或出口条件，禁止把混合状态压缩成单一状态后以偏概全。

每个 ADR 必须包含七要素：**标题/状态、背景、决策、理由、后果、替代方案、验证证据、复审条件**。验证证据要求：`[Current]` 项给「可复现命令 + 当前结论」；`[Target]` 项给「出口条件 + 待取证项」，禁止把规划写成已建成。证据行中的历史核验日期仅作追溯，不作为状态依据。

## 分层与依赖图

ADR 按**依赖方向**分为五层：上层依赖下层，同层互相解耦。跨层依赖视为架构违规，需登记新 ADR 理由。

```text
┌─────────────────────────────────────────────────────────────────┐
│  L5 目标态能力域（状态见索引）                                   │
│  ADR-024 ReBAC │ ADR-025 IGA │ ADR-026 Workload │ ADR-027 风险 │
│  ADR-028 组织树 │ ADR-029 Actions │ ADR-030 SCIM │ ADR-031 审计 │
│  ADR-034 AI 网关部署 │ ADR-035 支付 ChannelGateway SPI         │
├─────────────────────────────────────────────────────────────────┤
│  L4 安全与安全基础设施                                           │
│  ADR-008 Outbox │ ADR-009 KMS/Vault │ ADR-015 密钥轮换          │
│  ADR-017 领域事件 │ ADR-018 幂等 │ ADR-021 环境变量语义          │
├─────────────────────────────────────────────────────────────────┤
│  L3 权限模型（三层纵深防御，有向决策链见《layers》§5）          │
│  ADR-003 Casbin 策略决策（中间层）                               │
│  ADR-016 RLS 数据库强制（最底层）│ ADR-036 混合 RLS（C1）          │
│  ADR-024 OpenFGA ReBAC（目标态决策层）──→ 叠加 ADR-003（不替换） │
├─────────────────────────────────────────────────────────────────┤
│  L2 架构与工程实践                                               │
│  ADR-005 端口接口 │ ADR-006 无 ORM │ ADR-007 UUIDv7              │
│  ADR-012 OpenAPI 生成链 │ ADR-022 声明式 Schema │ ADR-023 多方言 │
│  ADR-032 前端 Controller/View 分离                               │
├─────────────────────────────────────────────────────────────────┤
│  L1 基础设施与事实源（无依赖，被几乎所有其他层依赖）              │
│  ADR-001 模块化单体 │ ADR-002 PG 事实源 │ ADR-004 PKCE-S256      │
│  ADR-010 Redis 缓存 │ ADR-011 时间分区 │ ADR-020 SLO 错误预算   │
│  ADR-013 会话状态机 │ ADR-014 令牌绑定 │ ADR-019 分布式限流      │
└─────────────────────────────────────────────────────────────────┘
```

### 权限模型分层契约（重点）

三层纵深防御的决策链、调用顺序与 PEP/PDP/PIP 映射以 [architecture/layers.md §5](../architecture/layers.md#5-授权决策顺序) 为唯一正文；[security-model.md §5](../architecture/security-model.md#5-权限决策基础引用) 只做引用索引，不另立顺序；[ADR-003](ADR-003-casbin-tenant-boundary.md)（Casbin 策略决策）、[ADR-016](ADR-016-rls-tenant-isolation.md)（RLS 数据库强制）、[ADR-036](ADR-036-scope-aware-rls-hybrid.md)（显式跨租户的 target 精确过滤）、[ADR-024](ADR-024-rebac-relation-tuple-authz.md)（ReBAC 目标态）只保留各自决策与出口条件。

### Outbox 与领域事件边界（重点）

领域事件是 Outbox 的 payload 类型，不是两套独立机制；投递语义、状态机与禁止事项以 [design/infrastructure/outbox.md](../design/infrastructure/outbox.md) 为唯一正文，决策见 [ADR-008](ADR-008-outbox.md) 与 [ADR-017](ADR-017-domain-events.md)。

## 索引

| ADR | 标题 | 状态 | 分层 |
|---|---|---|---|
| [ADR-001](ADR-001-modular-monolith.md) | 模块化单体优先 | [Current] | L1 |
| [ADR-002](ADR-002-postgresql-source-of-truth.md) | PostgreSQL 作为身份事实源 | [Current] | L1 |
| [ADR-003](ADR-003-casbin-tenant-boundary.md) | Casbin 与租户边界分工 | [Current] | L3 |
| [ADR-004](ADR-004-pkce-s256.md) | Authorization Code + PKCE S256 | [Current] | L1 |
| [ADR-005](ADR-005-port-interfaces.md) | 端口接口隔离基础设施 | [Current] | L2 |
| [ADR-006](ADR-006-no-orm.md) | pgx/raw SQL 而非 ORM | [Current] | L2 |
| [ADR-007](ADR-007-uuidv7.md) | UUIDv7 主键策略 | [Current]/[Target] | L2 |
| [ADR-008](ADR-008-outbox.md) | Outbox 与幂等 Worker | [Current]/[Target] | L4 |
| [ADR-009](ADR-009-kms-vault.md) | KMS/Vault 根密钥托管 | [Current]/[Target] | L4 |
| [ADR-010](ADR-010-redis-cache.md) | Redis 仅作可重建缓存 | [Current]/[Target] | L1 |
| [ADR-011](ADR-011-time-partitioning.md) | 高增长表时间分区 | [Current]/[Target] | L1 |
| [ADR-012](ADR-012-openapi-source.md) | OpenAPI 作为 API 契约源 | [Current]/[Target] | L2 |
| [ADR-013](ADR-013-session-state-machine.md) | 会话状态机与固定防护 | [Current] | L1 |
| [ADR-014](ADR-014-token-binding.md) | 令牌绑定（DPoP 与 mTLS） | [Current]/[Target] | L1 |
| [ADR-015](ADR-015-key-version-rotation.md) | 密钥版本与轮换状态机 | [Current]/[Target] | L4 |
| [ADR-016](ADR-016-rls-tenant-isolation.md) | PostgreSQL RLS 多租户隔离 | [Current]/[Target] | L3 |
| [ADR-017](ADR-017-domain-events.md) | 领域事件与事件总线 | [Current]/[Target] | L4 |
| [ADR-018](ADR-018-unified-idempotency.md) | 统一幂等性（Idempotency-Key 中间件） | [Current]/[Target] | L4 |
| [ADR-019](ADR-019-distributed-rate-limit.md) | 分布式限流 | [Current]/[Target] | L1 |
| [ADR-020](ADR-020-slo-error-budget.md) | SLO 与错误预算 | [Current]/[Target] | L1 |
| [ADR-021](ADR-021-environment-variable-semantics.md) | 环境标识与密钥变量语义统一 | [Current] | L4 |
| [ADR-022](ADR-022-declarative-schema-versioned-migration.md) | 声明式 Schema 唯一源与版本化迁移 | [Current]/[Target] | L2 |
| [ADR-023](ADR-023-multi-dialect-abstraction.md) | 多方言抽象与 MySQL 镜像边界 | [Current] | L2 |
| [ADR-024](ADR-024-rebac-relation-tuple-authz.md) | ReBAC 关系元组授权（OpenFGA 兼容 API + PostgreSQL 自研存储适配） | [Current]/[Target] | L3 |
| [ADR-025](ADR-025-iga-jit-access-lifecycle.md) | IGA 与 JIT 访问生命周期 | [Current]/[Target] | L5 |
| [ADR-026](ADR-026-workload-identity-federation.md) | Workload Identity 联邦 | [Current]/[Target] | L5 |
| [ADR-027](ADR-027-adaptive-auth-continuous-eval.md) | 自适应认证与持续访问评估 | [Current]/[Target] | L5 |
| [ADR-028](ADR-028-multi-level-org-domain-federation.md) | 多级组织与企业域名联邦 | [Current]/[Target] | L5 |
| [ADR-029](ADR-029-identity-actions-sandbox.md) | Identity Actions 登录管线沙箱 | [Current]/[Target] | L5 |
| [ADR-030](ADR-030-scim-outbound-jml-provisioning.md) | SCIM 出站与 JML 身份生命周期编排 | [Current]/[Target] | L5 |
| [ADR-031](ADR-031-region-residency-sm-crypto-audit-chain.md) | 多区域驻留、国密适配与审计证据链 | [Current]/[Target] | L5 |
| [ADR-032](ADR-032-frontend-controller-view-separation.md) | 前端页面 Controller / View 分离 | [Current] | L2 |
| [ADR-033](ADR-033-face-id-provider-spi.md) | Face ID Provider SPI 位置与形式 | [Current] | L2 |
| [ADR-034](ADR-034-aigateway-deployment-topology.md) | AI 网关部署形态（同进程 vs 独立数据面） | [Current]/[Target] | L5 |
| [ADR-035](ADR-035-payment-channel-spi.md) | 支付域 ChannelGateway SPI 位置 | [Current] | L5 |
| [ADR-036](ADR-036-scope-aware-rls-hybrid.md) | Scope-aware RLS 混合模式（C1） | [Current] | L3 |
| [ADR-037](ADR-037-provider-registry-boundary.md) | Provider 双轨边界与统一注册入口封口 | [Current] | L2 |
| [ADR-038](ADR-038-access-token-subject-model.md) | access_tokens 的主体模型（Principal Model） | [Target] | L3 |
