# B2B 设计（多级组织与域名治理）

> 设计锚点：[DESIGN:b2b/README]
> 决策：[ADR-028](../../adr/ADR-028-multi-level-org-domain-federation.md)；租户隔离口径见 [security-model §2.9.1](../../architecture/security-model.md)。

## 1. 定位

B2B 承载多级组织树、策略继承（继承/覆盖/锁定三模式）、企业域名认领与 SSO 发现。组织树是**控制面的层级关系**，不改变数据面租户隔离：任何层级的资源访问仍按 `organization_id` 行过滤。

## 2. 数据模型

| 表 | 作用 | 隔离 |
|---|---|---|
| `organization_domains` | 企业域名认领与验证状态 | 租户表（RLS） |
| `organizations` | 组织主体；层级关系由 `parent_id`（同租户树，NULL=根）承载 | 租户表（RLS） |
| `organization_policy_inheritance` | 策略继承/覆盖/锁定三模式声明 | 租户表（RLS） |

列与约束以 `contracts/schema/10_tenant.yaml` 为唯一事实源（复现：`grep -n 'organization_domains' contracts/schema/10_tenant.yaml`）。组织树 `parent_id`（FK ON DELETE RESTRICT）与策略继承表 `organization_policy_inheritance`（mode ∈ inherit/override/lock，唯一键 organization_id+policy_key）由声明式 Schema 基线声明；树关系不改变数据面隔离，仓储层不得私建第二套层级语义。

## 3. 流程

1. **域名认领**：管理员提交域名 → 生成 DNS TXT 挑战 → 周期性验证 → 验证通过置为已认领；验证失败保留可重试状态并写审计。
2. **SSO 发现**：登录入口按邮箱域名解析到已认领的组织与其 IdP；未认领域名回落到统一登录。
3. **策略继承**：安全策略/主题/注册项可声明 `inherit` / `override` / `lock`；父级锁定项子级修改一律 403 `forbidden`。
4. **生效链预览**：解析结果为确定性求值，预览与实际生效一致；解析过程不得跨租户读取。

落点：`internal/domain/b2b/organization_domain.go`（状态与端口）、`internal/app/b2b/service.go`（编排、验证、审计）。

## 4. API 与页面

- API 家族：`/api/v1/organizations/{orgId}/domains`（列表/创建）、`/api/v1/organizations/{orgId}/domains/{domainId}`（删除）、`/api/v1/organizations/{orgId}/domains/{domainId}/verify`（触发验证）；策略继承预览 `GET /api/v1/organizations/{orgId}/policy-inheritance/preview?policyKey=...`（`organizations:read`）。权限点 `b2b:read` / `b2b:write` 以 `routes.go` 与 `openapi/openapi.yaml` 为准，并须在 `contracts/registries/permissions.yaml` 登记资源。
- 页面：域名认领入口挂在组织详情/组织列表行操作，路由与页面文件以 `docs/design/frontend/registry.md` 为准。

## 5. 审计

域名认领、验证成败、认领撤销、策略继承解析与管理动作写 `records`（同事务），事件命名空间 `b2b.*`；事件名唯一事实源为 `contracts/registries/events.yaml` 的 `b2b` 命名空间。

## 6. 验收锚点

- `AC-B2B-001`：组织树继承/覆盖/锁定三模式、父级锁定拒绝 403、生效链预览一致、解析不引入跨租户泄露。
- `AC-B2B-002`：域名认领与 SSO 发现，认领失败/重复认领/跨租户认领的拒绝路径。

## 7. 边界与未决

- 组织树扁平化上限与最大深度由策略声明，超限拒绝而非静默截断。深度固定为平台基线 `MaxOrganizationTreeDepth`（`internal/domain/organizations/tree.go`），不提供按组织配置；扇出不设硬上限，由列表分页与数据库约束兜底。
- 策略继承预览（`internal/domain/organizations/policy_inheritance.go`）：root→target 确定性求值，`inherit` 保留祖先值、`override` 覆盖、`lock` 锁定；祖先锁定后子级修改在安全策略更新入口返回 403。当前覆盖 security policy 的 8 个 key，以及 theme/accountItems/mfaItems/languages/featureFlags；安全策略、settings、theme 三个写入口均对祖先 lock 返回 403。
- 运行期证据：`internal/app/organizations/policy_inheritance_pg_test.go` 在隔离库构造 root 锁定 + child 覆盖，断言生效值 12 且 lockedBy=root。
- 校验与接线：`internal/domain/organizations/tree.go` 的 `ValidateOrganizationParent`（自引用/成环/深度三类拒绝，422）；应用层 `internal/app/organizations/tree_validation.go` 读取祖先链；Create/Patch 接受 `parentId`，PG/MySQL 仓储读写 `parent_id`，列表/详情返回 `parentId`。深度以平台基线 `MaxOrganizationTreeDepth` 兜底，不提供按组织策略化配置。
- 域名认领不得作为身份认证的唯一依据；SSO 仍须走协议面校验。
