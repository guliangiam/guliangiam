# 用户域详细设计

> 设计锚点：[DESIGN:iam/users]

> 目标态规范「用户域详细设计」；写作约束见 [docs/README.md](../../README.md)。

---

## 定位

用户是 IAM 的核心主体。每个用户属于一个组织（多租户边界见 ADR-003 Casbin 租户边界 + ADR-016 RLS 隔离）。用户通过多种方式认证（密码、MFA、OAuth2、SAML、WebAuthn、QR Code），拥有会话与令牌。

## 用户状态

| 状态 | 语义 |
|------|------|
| active | 正常 |
| forbidden | 禁止登录（SCIM DELETE/停用语义） |

## 用户属性（核心）

| 属性 | 来源 | 说明 |
|------|------|------|
| id | UUIDv7 | 主键 |
| username | `users.username` | 必填，组织内唯一 |
| email | `user_profiles.email` | 可验证 |
| displayName | `user_profiles.display_name` | 显示名 |
| organization_id | `users.organization_id` | 所属组织 |
| user_type | `users.user_type` | normal / guest / service_account |
| is_forbidden | `users.is_forbidden` | 状态 |
| external_id | `users.external_id` | SCIM/IdP 外部标识 |

## 身份绑定

用户可绑定多个身份：

| 身份类型 | 绑定表 |
|---------|--------|
| SAML/OIDC 联邦 | `user_identities`（link_state 状态机：auto_linked / merge_pending / linked / rejected） |
| WebAuthn 凭证 | `webauthn_credentials`（状态机：active ↔ deactivated → revoked） |
| 密码 | `user_credentials`（argon2id 哈希；强制改密标记 `need_update_password`；声明事实源 `contracts/schema/20_identity.yaml`） |

**防联邦接管**：外部 IdP 断言中的邮箱声明不构成所有权证明，未验证邮箱不得自动绑定。

## 批量导入

| 方式 | 格式 | 入口 |
|------|------|------|
| JSON | 自定义 schema | 管理面 API |
| Excel | `.xlsx` | 管理面 UI |

幂等键：`(organization_id, username)` + `(organization_id, external_id)` 双键冲突仲裁。

## 用户与会话

每个用户可拥有多个并发会话，并发策略参数（`concurrentSessionPolicy`/`maxActiveSessions`）由组织安全策略配置。会话强制下线 API 见《design/auth/webauthn.md》。

## 用户列表页规范

UsersPage（CrudListPage config 直套）共 16 列 + Actions 列：

| # | 列 | 来源 | 渲染 |
|---|---|---|---|
| 1 | avatar | `avatarUrl` | Avatar + 首字母 fallback（固定左） |
| 2 | username | `username` | Link → UserDetailPage（固定左） |
| 3 | organizationId | `organizationId` | Link → OrganizationDetailPage（用 orgNameMap 替换） |
| 4 | displayName | `displayName` | 纯文本 |
| 5 | email | `email` | `mailto:` link |
| 6 | phone | `phone` | 纯文本 |
| 7 | realName | `realName` | 纯文本 |
| 8 | affiliation | `affiliation` | 纯文本，空值显示 `-` |
| 9 | registerType | `registerType` | 纯文本，空值显示 `-` |
| 10 | userType | `userType` | Tag |
| 11 | signupApplicationId | `signupApplicationId` | Link → ApplicationDetailPage（用 appNameMap 替换） |
| 12 | status | `status` | Switch（active/inactive，乐观更新 + revision） |
| 13 | isAdmin | `isAdmin` | Switch + revision |
| 14 | isVerified | `isVerified` | Green Tag |
| 15 | isForbidden | `isForbidden` | Red Tag |
| 16 | createdAt | `createdAt` | 日期格式化（默认排序） |
| Actions | — | — | Edit + Impersonate + Delete（固定右） |

**Row Actions 保护**：

| 操作 | 保护条件 |
|---|---|
| Impersonate | `disabled` 当 `record.username === session.account.name`（不能假扮自己） |
| Delete（行内） | `disabled` 当 `isProtected(record)` 或 `crud.deleteMutation.isPending` |
| Delete（批量） | `bulkActions` 回调内用 `crud.query.data.items` 过滤：排除所有 `isProtected` 用户 |

**`isProtected` 判定**（与 OrganizationsPage `isBuiltIn` 模式镜像）：

```
isProtected(record) = (record.username === session.account.name)  // 不能删除自己
                   || (record.organizationId === "built-in" && record.username === "admin")
```

**工具栏**：OrganizationSelect（仅 admin 可见）+ New User + Import（BatchImportModal .xlsx/.xls）+ Export。

**不在列表暴露的字段**（REJECT）：region（UserListItem DTO 无此字段）、tag（产品方向暂不做）、balance/balanceCredit/balanceCurrency（按 org 隔离，OrganizationList 已覆盖）、isDeleted（status active/inactive 统一替代）、registerSource（signupApplicationId 更准确）。

## 模拟登录（Impersonate）

| 项 | 规范 |
|---|---|
| 入口 | `POST /api/v1/users/{id}/impersonate`；发起前必须重新授权（按风险等级走 step-up / 操作票） |
| 时限 | 会话携带 impersonation 标记与到期时间，到期自动退出；超时后任何请求按普通会话处理 |
| 界面 | 全局醒目状态条（操作者 / 被模拟用户 / 到期时间）；敏感操作二次确认 |
| 审计 | `user.impersonation.started` / `user.impersonation.ended`，记录操作者、目标用户、原因、到期时间、来源 IP |
| 退出 | `POST /api/v1/auth/exit-impersonate` 或 `POST /api/v1/users/{id}/exit-impersonate`；退出即撤销临时凭证并恢复原会话 |
| 禁止 | 被模拟会话不得修改操作者自身凭证/MFA；不得借模拟登录绕过租户 scope 或 RLS |

## API 事实源

用户域 API 端点清单以 `internal/transport/http/api/v1/users/routes.go` 的 `route.Definition` 声明为准，由 `python3 scripts/gen_openapi.py` 生成到 `openapi/openapi.yaml`。

用户详情读面不得返回托管账号密码、MFA secret、access key 明文；只返回已配置标记与脱敏值。明文仅允许出现在明确的一次性设置或轮换响应中。
