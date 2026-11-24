# ADR-036: Scope-aware RLS 混合模式（C1）

- 状态：[Current] — C1 已落地：`TenantFilterCross` 显式跨租户以 `member` + 单一 target 精确过滤，`system`/无 target 平台管理员旁路已可审计（`OPEN-API-42` 已关闭，取证见 `contracts/registries/open-items.yaml`）。B/C2 多租户集合仍暂缓，启动前需产品承诺与新 ADR。
- 日期：2026-11-03
- 取代：ADR-016 中「平台管理员/显式跨租户一律以 `super_admin`/`system` 全库旁路」的角色旁路条款；ADR-016 的 RLS `ENABLE`/`FORCE`、GUC 名称、`set_tenant_context` 与策略强制层继续有效。

## 背景

现行实现（下称 A）是 role-based GUC：`app.current_role` 与 `app.current_organization_id` 由事务内 `guliangiam.set_tenant_context(uuid, text)` 注入，RLS 策略统一为 `guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id()`。连接级还有 AfterConnect fail-closed 默认、PrepareConn 会话级注入与 AfterRelease 重置。GUC 名称与函数的事实源是 `contracts/schema/00_infra.yaml`，Go 侧常量是 `internal/platform/db/rls.go`。

A 的缺口集中在「显式跨租户」（以下为决策时（2026-11-03，C1 实施前）的缺口描述，现行注入已按 D2 收敛）：`ScopeAuthorizerMiddleware` 批准一个显式 target 后，当时注入的是 `TenantFilterSystem` + 空组织，事务内再映射为 `super_admin` + 空 org（`internal/infra/postgres/uow/uow.go`），RLS 因此放行全库。授权只保证 `target ∈ scope`，数据库层不再把可见范围限制到 target；剩余边界依赖 handler 是否都正确使用 `auth.TrustedOrganizationID`。这与「已授权到 X，但 RLS 放行全库」的最小授权目标不符。

同时必须保留两条既有语义：

1. **系统旁路**：Worker/Cron/迁移等无 HTTP 上下文的内部任务依赖 `system` 模式映射到 `super_admin`；该旁路不得由客户端请求触发，且必须伴随审计。
2. **平台管理员无显式 target 的全租户总览**：平台管理员未指定 target 时仍映射为 `super_admin`，以保留 dashboard 等全局端点的多租户总览语义。

多租户集合（同一请求访问 A/B/C 多个租户、`platform_admin_scopes.tenant_ids` 数组直接进入 RLS）此前在 `docs/design/iam/permissions.md` 中被描述为 `app.current_tenant_scope UUID[]` + `app.tenant_mode` + `app.current_actor_type`（下称 B）。B 需要改 `contracts/schema/00_infra.yaml` 函数、全部 `rls_predicate`、生成器默认模板、迁移与回滚、MySQL 能力边界与 mode 分支，当前没有产品承诺要求同一请求并发访问多个租户，收益与成本不匹配。

## 决策

### D1. 保留 A 作为 RLS 强制层

`app.current_role` + `app.current_organization_id` 仍是唯一 RLS GUC 契约；`guliangiam.set_tenant_context`、`current_organization_id`、`is_super_admin`、`resolve_org_id` 与各表 `rls` / `rls_predicate` 声明不改。普通业务请求固定 `role=member` + session/target 组织，策略按 `organization_id` 精确过滤。

### D2. 显式跨租户采用 C1：目标租户精确过滤

新增 `TenantFilterCross` 模式（`internal/platform/tenantctx`）：仅当 ScopeAuthorizer 已批准显式 target 时注入，携带**单一、非空**目标租户。该模式下：

- RLS role 固定 `member`，`app.current_organization_id = target`；不允许落入 `system`/`super_admin` 的空组织旁路。
- 数据库可见范围收敛到 target；`auth.TrustedOrganizationID` 返回受信 target，而不是请求里的任意 `requested`。
- target 为空或无法解析时必须 fail-closed（拒绝或空结果），不得回退为全库旁路。
- 该收紧只影响 PostgreSQL RLS 注入；MySQL 仍按仓储显式 `organization_id` 过滤，不增加等价承诺。

### D3. `system` 旁路保留，但必须可审计且不可由 HTTP 触发

`TenantFilterSystem` 仅允许由框架层内部任务入口注入（Worker/Cron/迁移/后台作业），不得由任意 HTTP 请求直接设置。每次 `system`/`super_admin` 旁路必须留下审计记录：主体（任务标识或平台管理员）、原因、目标租户或全局范围、决策与 trace；旁路审计缺失视为未完成项，纳入 `OPEN-API-42` 出口条件。

### D4. 平台管理员无显式 target 的全租户总览语义保留

未携带显式 target 的平台管理员继续使用 `TenantFilterSuperAdmin` → `super_admin`，保留全租户总览。该路径是 C1 的已知残余：它仍是全库旁路，因此必须伴随审计；是否收紧为 C1b（scoped admin 也只能看 scope 内租户）由产品另行裁决，不在本 ADR 内改变。

### D5. 多租户集合 / scope 数组暂缓

不引入 `app.current_tenant_scope UUID[]`、`app.tenant_mode`、`app.current_actor_type`，不改任何 `rls_predicate`，不新增迁移。B/C2（同一请求访问多租户）暂缓、不在 `OPEN-API-42` 交付范围内（该项已随 C1 落地关闭）；启动出口条件为产品明确承诺多租户集合，并先立新 ADR、再完成 schema、GUC、全部策略、生成器、迁移回滚与测试的成套变更。

### D6. 交付边界

C1 的代码实现（`TenantFilterCross` 枚举、`TenantWhereClause`/`TenantAndClause` 的 cross 分支、`tenant_resolution.go` 与 `tenant_filter.go` 的注入、`postgres.go` 的 role 映射、相关测试与门禁）不属于本决策文档的交付范围，由独立实施批次交付；该批次已完成，`OPEN-API-42` 已关闭（落地状态与取证见「验证证据」节）。本 ADR 记录决策、边界与出口条件本身。

## what / why / how / trade_off

| 维度 | 内容 |
| --- | --- |
| **what** | C1 混合模式：保留 role-based GUC 与 `system`/无 target 平台管理员旁路；把「ScopeAuthorizer 已批准的显式跨租户」从 `system` + 空 org 改为 target + `cross`，RLS 以 `member` + target 精确过滤。多租户集合暂缓。 |
| **why** | 安全收益集中在最危险的缺口：授权到 X 之后不再全库可见，RLS 重新成为目标租户的兜底；同时不破坏 Worker/Cron/迁移与平台管理员全局总览；不改 schema、不新增迁移，风险/收益比高于 B。 |
| **how** | 新增 `TenantFilterCross`；ScopeAuthorizer allow 后注入 target + cross；`rlsRoleForMode` 将 cross 映射 `member`；`setTenantContext`/`prepareRLSConn` 对 cross 要求非空 org 并写 `set_tenant_context(target, 'member')`；`TenantFilterMiddleware` 对 cross pass-through；补齐 cross 隔离、写他租户拒绝、`TrustedOrganizationID`、system 旁路与审计用例。 |
| **trade_off** | 少一个「多租户集合」能力：B/C2 的批量跨租户仍需后续 ADR 与 schema 变更；多一个 mode 分支，需要所有 mode 分支（tenantctx、uow、adapter、middleware、auth 上下文）同步补齐；无 target 平台管理员仍是全库旁路，C1 只缩小显式 target 路径的暴露面。 |

## 理由与后果

- 正面：显式跨租户不再以「授权后全库旁路」运行，RLS 对 target 重新生效；GUC 契约不变，既有仓储 SQL、声明式 Schema、迁移链与 MySQL 镜像不受影响；`system` 与平台管理员总览语义保留，避免全局端点回归；B 的高成本被推迟到有产品承诺时。
- 负面：C1 需要跨层补齐 mode 分支；`system` 与无 target 平台管理员的旁路仍是残余风险，只能靠审计与后续 C1b/C2 收敛。旁路审计已建成：无 target 平台管理员旁路经 `AuditTenantBypass`（`internal/platform/scopeauthorizer/allow_if_scoped.go`）写审计哈希链，Worker/system 旁路在启动期集中审计（`worker.system_scope`，`internal/bootstrap/wire_domains.go`），取证见 `OPEN-API-42` 闭环记录。
- 约束：任何新增绕过 RLS 的路径必须显式说明主体、目标与审计写入点；不得以「跨租户已授权」为由把 role 提升为 `super_admin`；不得让客户端 body/query/header 的 `organizationId` 决定 target。

## 替代方案

| 方案 | 内容 | 否决/暂缓理由 |
| --- | --- | --- |
| **A：维持现状** | 保留 role-based GUC；显式跨租户继续 `system` + 空 org 全库旁路；只修文档与旁路审计 | 无法收敛「授权到 target、RLS 放行全库」的核心缺口；旁路审计仍缺失；文档可改为 A 口径，但代码缺口继续存在 |
| **B：scope 数组** | `app.current_tenant_scope UUID[]` + `app.tenant_mode`；策略 `organization_id = ANY(current_tenant_scope)` | 需改函数、全部策略、生成器、迁移/回滚、MySQL 能力边界与全部 mode 分支；当前无多租户集合产品承诺，成本/收益不成立 |
| **C1：混合（本 ADR）** | 保留 role 旁路，显式跨租户 target 精确过滤；多租户集合暂缓 | 采纳：最小安全收紧、无 migration；残余为无 target 平台管理员与 system 旁路的审计 |
| **C1b：连无 target 平台管理员也收紧** | 平台管理员全局端点也必须带 scope/target | 不在本 ADR：会改 dashboard/webhooks/webhookevents 等全局语义，需产品裁决后另立决策 |
| **C2：C1 之上再加 scope 数组** | 先落 C1，再按 B 的 schema 路径扩展多租户集合 | 暂缓：出口条件与 B 相同，登记 `OPEN-API-42` |

## 与 ADR-016、ADR-023、OPEN-API-37 的关系

- **ADR-016**：RLS `ENABLE`/`FORCE`、GUC 名称、`set_tenant_context`/`current_organization_id`/`is_super_admin` 与策略强制层继续有效。本 ADR 取代其「平台管理员/显式跨租户一律全库旁路」的角色旁路条款：显式跨租户改为 target 精确过滤；`system` 与无 target 平台管理员旁路保留并附加审计约束。不引入 `[Superseded]` 等额外状态词。
- **ADR-023**：MySQL 无 RLS，靠仓储显式 `organization_id` 过滤。C1 只改变 PostgreSQL 的 GUC 注入与 role 映射，不改变 MySQL 仓储语义，也不宣称双方言能力等价；MySQL 运行期边界仍按 ADR-023 执行。
- **`OPEN-API-37`**：已关闭（MySQL 运行期可用，启动探针保留）；C1 未新增 MySQL 能力、未改 MySQL 仓储。MySQL 与 PostgreSQL 的能力边界差异以 `contracts/registries/open-items.yaml` 与 ADR-023 为准。
- **`OPEN-API-42`**：已关闭——C1 全链路（cross 注入、role 映射、精确过滤、旁路审计）落地并完成真实库取证；B/C2 多租户集合明确暂缓、不在该项范围内，启动前需产品承诺与新 ADR。

## 验证证据

决策时快照（核验日期 2026-11-03，C1 实施前，用于锁定决策所依据的当时事实）：

| 检查点 | 命令 | 当时结论 |
| --- | --- | --- |
| GUC 契约 | `rg -n "RLS_GUCRole\|RLSGUCOrganizationID" internal/platform/db/rls.go` | 命中 `app.current_role` 与 `app.current_organization_id` |
| 显式跨租户的当时注入 | `rg -n "TenantFilterSystem" internal/transport/http/middleware/tenant_resolution.go` | 命中 `WithTenantScope(ctx, "", "", TenantFilterSystem)` |
| `system`/`super_admin` 的 RLS 提升 | `rg -n "isPrivileged\|RLSRoleSuperAdmin" internal/infra/postgres/uow/uow.go internal/platform/db/postgres.go` | 命中 privileged 分支写 `super_admin` + 空 org |
| 平台管理员无 target 的当时提升 | `rg -n "PrincipalType == \"platform_admin\"" internal/transport/http/middleware/tenant_filter.go` | 命中 `TenantFilterSuperAdmin` |
| C1 当时尚未建成 | `rg -n "TenantFilterCross" internal` | 零命中 |

C1 落地取证（以 `OPEN-API-42` 闭环记录为准，`contracts/registries/open-items.yaml`；核验日期 2026-11-21）：

| 检查点 | 事实源/命令 | 结论 |
| --- | --- | --- |
| cross 枚举与精确过滤子句 | `internal/platform/tenantctx/scope.go`（`TenantFilterCross`，空 target panic）；`scope_test.go` cross 分支只生成 `column = $N` | 已落地 ✅ |
| ScopeAuthorizer allow 后注入 target + cross | `internal/transport/http/middleware/tenant_resolution.go`（`WithTenantScope(ctx, target, "", TenantFilterCross)`） | 已落地 ✅ |
| role 映射 cross→member | `internal/platform/db/postgres.go: rlsRoleForMode`；`prepareRLSConn`/`setTenantContext` 对 cross 写 target + member | 已落地 ✅ |
| cross 只见 target、写他租户被拒、system 全见、无 scope fail-closed | `TestTenantFilterCrossScopesToSingleTenant`（`internal/infra/postgres/uow/rls_cross_integration_test.go`，真实 PostgreSQL） | PASS ✅ |
| fail-closed 与池连接归还重置 | `TestSetTenantContextFailsClosedAndResetsPooledConnection` | PASS ✅ |
| 旁路审计 | `AuditTenantBypass`（`allow_if_scoped.go`，无 target 平台管理员）+ `worker.system_scope` 启动期审计（`wire_domains.go`） | 已落地 ✅ |

原 Target 出口条件（cross 枚举与注入、role 映射与 fail-closed、回归用例、旁路审计、门禁、`OPEN-API-42` 关闭）已全部满足；B/C2 若启动仍须先有产品承诺与新 ADR，不直接改 schema。

## 复审条件

出现以下任一情况时复审本 ADR：

1. 产品明确承诺「同一请求并发访问多租户集合」→ 启动 B/C2，先立新 ADR 或修订本 ADR，再改 schema/GUC/策略/迁移。
2. 产品决定取消「平台管理员无显式 target 的全租户总览」→ 进入 C1b，并同步改 dashboard/webhooks/webhookevents 等全局端点语义。
3. MySQL 成为合规运行期目标 → 按 ADR-023 / `OPEN-API-37` 重新评估 RLS 等价能力与旁路边界。
4. 发现任何 `system`/`super_admin` 旁路可由非框架路径触发，或旁路审计缺失 → 立即安全整改，并回填本 ADR 的验证证据。
