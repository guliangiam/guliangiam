# ADR-028: 多级组织与企业域名联邦（组织树 / 域名认领 / SSO 发现 / 委派管理）

- 状态：[Current]/[Target]（已落地：组织树 `organizations.parent_id`（生成式 V1 基线）+ 环/深度校验（`internal/domain/organizations/tree.go`、`internal/app/organizations/tree_validation.go`）+ 策略继承三模式与生效链预览（`internal/app/organizations/policy_inheritance.go`）；Federation Trusts 联邦信任关系（internal/domain/workload/{workload.go,repository.go} + internal/transport/http/api/v1/identitypools/{routes.go,chi.go} 的 federation-trusts GET/POST 路由）。仍为 [Target]：委派管理员、域名认领 DNS TXT 状态机、SSO 发现、Vanity 托管登录域名）
- 日期：2026-09-14

## 背景

B2B 商业对标（SRS B2B-001~006）要求：多级组织树（B2B-001）、树上策略继承（B2B-002）、委派管理员（B2B-003）、企业域名认领 DNS TXT 状态机（B2B-004）、邮箱域 SSO 发现（B2B-005）、Vanity 托管登录域名（B2B-006）。决策时（2026-09-14）现状：组织为扁平单层，`organizations` 无 parent_id，域名认领/vanity 无任何实现。现行状态：组织树与策略继承已落地（见头部与验证证据）；域名认领/委派/SSO 发现/Vanity 仍无代码路径（`grep -rniE 'verified_domain|domain_claim|vanity' internal/ --include='*.go'` 0 命中，核验 2026-11-21）。租户隔离模型为 `organization_id` 行过滤 + PostgreSQL RLS（[3](../architecture/data-model.md#3-持久化架构)，[ADR-016](ADR-016-rls-tenant-isolation.md) 状态 [Current]）。

## 决策

1. **组织树是管理面结构，不改变数据面隔离**（B2B-001）：`organizations` 目标态新增 `parent_id` 自引用（深度受配置限制），树上操作（创建/移动/合并/删除）有预检（下挂资源摘要）与审计；子组织成员/应用/策略在数据面仍按自身 `organization_id` 行过滤（ORG-004 语义不变，ADR-016 RLS 落地后同受数据库强制），环与孤儿节点被拒。
2. **策略继承三模式**（B2B-002）：安全策略（ORG-009）、主题（ORG-008）、注册项（AUTH-067）等可声明「子组织继承/覆盖/锁定」；父级锁定项子级不可修改；继承解析为确定性求值且可预览生效链。
3. **委派管理员**（B2B-003）：管理权委派到子组织，作用域限定子树内资源，权限点继承自上级角色但按树作用域过滤；委派授予/收回审计，跨子树越权被拒。
4. **域名认领 → SSO 发现 → Vanity 域名链路**：认领走「发起 → 一次性 TXT 记录值 → DNS 校验（定时/手动重试）→ 已认领 → 有效期与续验」全状态机（B2B-004，同一域名全局唯一归属、冲突留争议通道）；已认领且配置 SSO 的域名支持「邮箱即发现」跳转企业 IdP（与 AUTH-094/097 联邦能力衔接），发现响应不泄露组织存在性（防枚举）并限流（B2B-005）；Vanity 域名（B2B-006）要求先认领、证书 ACME 自动化、主机头校验防混淆。**落点声明**：域名认领状态机的持久化表在 [3.2](../architecture/data-model.md#32-repository-约定) 十九张目标表中未单独申报，DDL 设计（Task 28）须在既有表（如 `organization_settings`）扩展或按 [3.2](../architecture/data-model.md#32-repository-约定) 清单补申报，本 ADR 不预占新表名。
5. 企业 IdP 联邦本体（SAML/OIDC 深化）由 AUTH-094~098 承载，本 ADR 只决策「域名 → 组织 → IdP」的发现与归属结构。

## 理由与后果

企业采购决策者按「子公司/部门结构 + 品牌域名」理解 IAM；扁平组织使委派与继承无从谈起，人工开通域名 SSO 使自助率与 TTM 不达商业标准。树作为管理面结构、数据面保持单租户列，是风险最小的演进：不动 RLS/租户过滤模型。代价：组织树变更预检、继承解析、TXT 状态机轮询三类新任务面；Vanity 域名引入 ACME 证书生命周期运维。

## 替代方案

- **扁平组织（现状延续）**：企业组织映射失败，委派管理员与策略继承两项商业承诺直接不可交付；放弃。
- **仅手动关联（域名 SSO 全人工开通）**：自助率低、开通周期以天计、配置漂移无人对账；放弃。
- **树 ID 参与数据面路由（子组织独立租户语义/树键入 WHERE）**：破坏既有单租户列模型与 RLS 边界，授权与过滤逻辑全局复杂化，且 B2B-001 验收 2 明确「数据面隔离不因树关系改变」；放弃。

## 验证证据

- Current（核验 2026-11-21）：`organizations.parent_id` 自引用 FK + 部分索引已随生成式 V1 基线声明（事实源 `contracts/schema/10_tenant.yaml`，生成物 `db/postgresql/V1__guliangiam_schema_init.sql`）；环/孤儿/深度校验在 `internal/domain/organizations/tree.go`（`ValidateOrganizationParent`，深度上限 `MaxOrganizationTreeDepth`）与 `internal/app/organizations/tree_validation.go`（parent 链读取失败统一 422 防枚举），配套 `tree_validation_test.go`；策略继承三模式（继承/覆盖/锁定）与生效链预览在 `internal/app/organizations/policy_inheritance.go`（`PolicyInheritancePreview`，父级锁定项子级拒绝），配套 `policy_inheritance_test.go` / `policy_inheritance_pg_test.go`。数据面隔离仍按自身 `organization_id` 行过滤 + RLS，树不参与数据面路由。
- Target（B2B-003~006）：`grep -rniE 'verified_domain|domain_claim|vanity' internal/ --include='*.go'` → 0 命中；委派管理员无代码路径；DNS/ACME 设施无代码路径。
- Target 出口条件：①委派跨子树操作被拒并审计（B2B-003 验收 2）；②TXT 状态机全流程用例（成功/失败/过期续验/冲突）；③SSO 发现响应不泄露组织存在性并有防枚举负向用例（B2B-005 验收 3）。待取证项：域名认领持久化落点的 Task 28 申报结论、ACME 接线（挂 OPS 证书管理）。

## 复审条件

出口条件满足后转 `[Current]`；若组织树下沉为数据面结构（隔离语义变更）或域名认领采用外部 DNS 服务托管，须复审并以新 ADR 取代。
