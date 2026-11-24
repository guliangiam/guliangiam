# ADR-003: Casbin 与租户边界分工

- 状态：[Current] — 决策与 Casbin 端点级策略已落地；认证面路由白名单已收敛为「方法 + 路径模板」精确枚举并有启动期 fail-fast 自检（AC-PERM-004 已实现，见 `contracts/registries/security-readiness.yaml` S11）
- 日期：2026-09-14
- 关系：与 ADR-016（RLS 第三层）叠加；与 ADR-024（ReBAC 对象级叠加层）并存，不替换 Casbin；外部 PDP 替代须新 ADR 取代本 ADR

## 背景

多租户 IAM 需要把“谁（主体）能对什么（资源）执行什么动作”策略化：不同租户、不同 enforcer 实例可能使用不同模型文本（RBAC/ABAC），策略还要能热更新而无需发版。纯代码硬编码无法满足；但反过来，若把租户隔离也全部托付给策略引擎，一次策略漏配或缓存失效就会造成跨租户泄露。因此必须在项目初期划清“策略决策”与“租户边界/业务状态”的职责线。

## 决策

Casbin 执行策略决策；App 层在调用策略引擎之前先完成身份认证、组织归属、资源所有权与业务状态（启用/禁用/软删除）校验。任何授权异常、策略加载失败或归属不明都不得默认放行（Fail-Closed）。

## 理由与后果

- 正面：策略可按租户/enforcer 配置与持久化（`enforcers`、`casbin_rules`），鉴权点统一收敛到 `CasbinManager` 端口；App 前置校验保证即使策略漏配，资源归属与状态仍由业务代码兜底，纵深防御的第三层为 RLS（ADR-016，已落地）。
- 负面：策略配置不能替代租户隔离，每个资源查询仍必须显式注入组织范围并有越权测试；Casbin 策略缓存的失效一致性需要持续治理（`internal/infra/casbin/cache.go`）；两层校验意味着鉴权逻辑分布在 App 与策略两处，文档与测试必须明确分工。

## 替代方案

- **纯代码 RBAC（角色判断写死在 `if/switch` 中）**：放弃理由——策略无法按租户热更新、无法支持多模型文本，模型与规则变更都要发版，也无法满足“租户自带 enforcer”的产品形态。
- **Casbin 作为唯一授权边界，App 不做归属/状态校验**：放弃理由——策略漏配即越权，而“资源是否已软删除、是否属于当前组织”并非纯策略问题；本项目选择 App 校验 + Casbin 决策 + RLS 兜底（已落地）的三层结构，不把全部安全押在单一组件上。

## 验证证据（Current）

- `grep casbin go.mod` → `github.com/casbin/casbin/v3`（版本以 go.mod 为准）。
- `grep -rn '\.Enforce(' internal/app --include='*.go'`（排除测试）→ 仅 2 处且均经领域端口调用：`internal/app/permissions/service.go` 与 `internal/app/enforcers/service.go` 的 `s.CasbinManager.Enforce(...)`；适配器实现位于 `internal/infra/casbin/`，App/Domain 不直接依赖实现（check-layers Rule 3 为 0 违例，见 ADR-001）。
- 路由白名单已按「方法 + chi 路径模板」精确枚举：`internal/bootstrap/public_route_whitelist.go`（`rootPublicRoutes` / `explicitPublicRouteExemptions`，同前缀新增兄弟路由不被覆盖）+ `internal/transport/http/api/v1/auth/auth_paths.go` 的认证面精确豁免（有意 exact 且 POST-only）；启动期自检对不上即 panic（fail-fast）。AC-PERM-004 已实现（`contracts/registries/security-readiness.yaml` S11「PEP fail-close（四类放行分支关闭 + 启动自检）」impl_status 已实现）。决策时按 `/api/v1/auth/` 前缀整体豁免的偏差已关闭，历史记录见 `CHANGELOG.md`。

## 复审条件

白名单枚举或启动自检语义变更时复核默认拒绝的实际覆盖面（AC-PERM-004 已验收，回归以启动自检与路由审计为准）；ADR-016 RLS 已落地，App/Casbin/RLS 三层职责按其现行机制划分；若未来引入外部 PDP（OPA 等）替代 Casbin，以新 ADR 取代本 ADR。复核注记：ADR-024 的 ReBAC 是对象级授权的叠加层，不取代本 ADR；Casbin 继续承载端点级策略。
