# 组织域详细设计

> 设计锚点：[DESIGN:iam/organizations]

> 目标态规范「组织域详细设计」；写作约束见 [docs/README.md](../../README.md)。

---

## 定位

组织是 GuliangIAM 的**顶层租户边界**（RLS 隔离、Casbin 租户边界共同构成多租户防线，见 ADR-003 + ADR-016）。所有用户、应用、角色、会话、审计记录都在组织上下文内。

## 组织属性

| 属性 | 说明 |
|------|------|
| id | UUIDv7 |
| slug | URL 友好标识符（唯一） |
| name | 显示名称 |
| plan | 套餐档（free / basic / enterprise） |
| status | active / suspended |
| **logo_url** | **组织品牌 Logo（完整 URL 或相对路径 `/uploads/...`）** — session bootstrap 时映射为 `SessionOrganization.LogoUrl` (JSON: `logoUrl`)，前端用于 Header brand Avatar src（首字母 fallback）和组织下拉选择器 item icon |
| **favicon_url** | **组织 Favicon（完整 URL 或相对路径）** — session bootstrap 时映射为 `SessionOrganization.FaviconUrl` (JSON: `faviconUrl`)，前端 AppShell 通过 useEffect 动态替换 `<link rel="icon">`，组织切换时浏览器 tab 图标实时更新 |

### Branding 字段端到端链路

```
DB organizations.logo_url / favicon_url       (schema/contracts 10_tenant.yaml)
  ↓
infra/postgres/organizations/mapper.go        (Scan + field mapping)
  ↓
domain/organizations.Organization entity      (Go PascalCase)
  ↓
app/auth/mapper.go (session bootstrap)        → SessionOrganization.LogoUrl / FaviconUrl
  ↓ JSON camelCase tag
GET /api/v1/auth/account                       → { organization: { logoUrl, faviconUrl } }
  ↓
前端 sessionOrganizationSchema (Zod)          → 必须声明这两个 .optional().nullable() key
  ↓
Zustand session-store.organization.logoUrl   → AppShell Avatar src + favicon useEffect
```

**禁止环节**：前端 Zod schema 漏声明 key → parse 静默剥离（见 开发规范 4.5.4）。后端 `SessionOrganization` DTO 字段名变更时，同 commit 必须补齐前端 schema + mapper。

## 安全策略（organization_security_policies）

| 策略 | 说明 |
|------|------|
| password_policy | 密码复杂度（最小长度/大小写/数字/特殊字符） |
| password_derive_algorithm | 口令派生算法：`hmac_sha256`（默认）/ `hmac_sm3`；存储哈希固定 argon2id，SM3 仅用于一次性验证码摘要的派生，不是密码存储算法选择器；事实源 `contracts/schema/10_tenant.yaml`；边界见《[security-model.md](../../architecture/security-model.md)》§2.21 |
| session.idle_timeout | 会话空闲超时（默认 30min） |
| session.bind_ip | 会话绑定创建时 IP；请求 IP 变化即 401 重新认证（默认关闭） |
| session.bind_user_agent | 会话绑定创建时 User-Agent；变化即 401 重新认证（默认关闭） |
| session.absolute_timeout | 会话绝对超时（默认 12h） |
| mfa.required | 是否强制 MFA |
| concurrent_session_policy | 并发会话策略（single / evict_oldest / max） |
| max_active_sessions | 最大并发数 |
| signin_disabled | 禁止登录开关 |
| signup_disabled | 禁止注册开关 |
| webauthn_required_uv | WebAuthn UV 策略 |
| attestation_conveyance | WebAuthn attestation 信任分级 |

## 套餐与配额衔接

组织套餐（ORG-019/020）与计量配额（《design/commerce/metering.md》）绑定，三档差异化配额：

| 套餐档 | 配额 |
|--------|------|
| Free | 用户数上限、MAU 上限 |
| Basic | 扩容用户数、应用数、MFA 方法 |
| Enterprise | 自定义配额、多区域驻留、国密适配 |

配额变更即时生效并审计。

## 组织创建表单（OrganizationCreatePage）

创建组织时通过可折叠 Section 分组提交字段，对应 `POST /api/v1/organizations` + `CreateOrganizationCommand`。

| Section | 字段 | 归属子表 | 说明 |
|---------|------|---------|------|
| Basic（必填，展开） | slug, displayName, websiteUrl, status, defaultApplicationId, initScore | organizations | slug 校验 `^[a-z0-9](?:[a-z0-9-]*[a-z0-9])$` |
| Branding | logoUrl, faviconUrl, defaultAvatar, enableDarkLogo | organization_branding / organizations | 品牌外观相关 |
| PasswordPolicy | passwordType（默认 `argon2id`）, passwordSalt, passwordObfuscatorType, passwordObfuscatorKey, passwordOptions（多选：AtLeast6/AtLeast8/Aa123/SpecialChar/NoRepeat）, defaultPassword, masterPassword, masterVerificationCode | organization_auth_config | 子表列对齐 DB DEFAULT 值 |
| SecuritySignin | disableSignin, enableTour, hasPrivilegeConsent, isProfilePublic, useEmailAsUsername | organization_security_policies | 安全/登录开关 |

所有扩展字段均为 optional；空值时走 DB column_default（passwordType=argon2id、passwordObfuscatorType=Plain、kerberosServiceName=HTTP、currency=USD、defaultCurrency=CNY）。

### 字段级硬编码值

| passwordType | 可选值 |
|---|---|
| `argon2id`（默认） / `bcrypt` / `pbkdf2-salt` / `sha512-salt` / `pbkdf2-django` / `salt` / `plain` / `md5-salt` |

| passwordObfuscatorType | 可选值 |
|---|---|
| `Plain`（默认） / `AES` / `SM4` |

| passwordOption | 可选值 |
|---|---|
| `AtLeast6` / `AtLeast8` / `Aa123` / `SpecialChar` / `NoRepeat` |

## 组织列表页扩展

OrganizationsPage 列表共 10 列（CrudListPage config 直套）：基础 7 列 + `status`（Tag）+ `passwordType`（Tag，来自 auth_config 子表）+ `balance`/`currency`（来自 organization_billing 子表）+ `createdAt`。

**内置组织删除保护**：`isBuiltIn=true` 的行 Delete 按钮禁用、bulkDelete 过滤器排除。

**Row Actions**：Edit → Groups → Users → Delete（四选一，4 个 `<Button type="link">` 并排；Actions 列 width 280）。Groups → `/groups`，Users → `/users`（GLIAM session + orgFilter 架构，不做 Casdoor 风格的路径参数嵌套 `/organizations/{name}/users`）。

## 组织域名联邦

组织可配置企业域名（如 `acme.com`），用户使用企业邮箱（`@acme.com`）登录时自动归属该组织（域名联邦，需求锚点 SRS §5.1.13 AUTH-096/097）。

## RLS 隔离验证

组织级 RLS 策略确保每个查询都隐式追加 `organization_id = current_setting('app.current_org_id')`。验收脚本：`scripts/check-rls.sh`（租户表 RLS 声明、ENABLE/FORCE、应用角色）、`scripts/check-rls-policy-content.py`（策略内容：禁止 `USING (TRUE)` 恒真策略）、`scripts/check-tenant-isolation.py`（每张表必须归入 direct/indirect/self_scoped/platform_scoped/global，global 仅白名单）、`scripts/check-tenant-provisioning.py`（创建组织必须同事务写租户身份与主拓扑，退役必须状态流转 + 软删）。

## 租户创建与退役事务边界

| 项 | 规范 |
|---|---|
| 创建顺序 | 单事务内：`Organizations().Create` → `Tenants().CreateIdentity` → `Tenants().CreatePrimaryTopology` → 审计；任一步失败整体回滚 |
| 退役顺序 | `Tenants().DecommissionIdentity` 与 `Tenants().DecommissionTopologies` 先于 `Organizations().Delete`；审计最后写入；禁止物理删除 |
| 状态 | 退役为不可逆状态流转；软删后资源不可再被解析为有效租户 |
| 区域 | `region_code` 由控制面注册表声明（DR-001/002），创建时写入单一区域桶 |
| 复现 | `python3 scripts/check-tenant-provisioning.py` |

## 账户与导航（navPreferences / accountItems）

组织级「账户与导航」配置存放于组织 settings 的 JSON 结构（零 DDL 扩展），编辑入口为组织详情页 NavigationTab（管理端 / 用户端两棵菜单树 + 账户项表格），写入经 `PUT /api/v1/organizations/{id}/configuration` 或 settings 端点提交。

### navPreferences schema 与键语义

```json
{
  "navItems": ["/", "/users", "/roles"],
  "userNavItems": ["/", "/apps", "/account"],
  "navGroupOrder": ["home", "identity", "authz", "integration", "audit", "security", "business", "llm", "admin"]
}
```

| 键 | 语义 |
|---|---|
| navItems | 管理员（isAdmin=true）可见路由键白名单；数组顺序即叶子菜单的展示顺序 |
| userNavItems | 普通用户可见路由键白名单；数组顺序同样决定叶子顺序 |
| navGroupOrder | 分组展示顺序（分组 key 数组）；未列出的分组按前端静态顺序排最后 |
| adminNavItems | 历史遗留键：后端仅在 `navItems` 缺失时回退读取；新写入一律使用 `navItems` 并清除该键 |

语义边界：

- navPreferences 未配置（键缺失或空对象）时后端下发 `[]`，前端解释为「无限制、全量展示」；数组含 `"all"` 同样视为无限制；
- 路由键为以 `/` 开头的路径（与前端 `menuGroups` 叶子一致），不带前导斜杠的键一并接受（兼容历史数据）；
- 编辑器内部的组节点键（`group:*`）与根键 `all` 不写入配置。

### 生效链路

```
NavigationTab（navPreferencesJson 表单字段）
  ↓ PUT configuration/settings
app/organizations 校验并持久化（settings.nav_preferences JSON）
  ↓ 会话引导：app/auth/mapper.go extractNavItems
GET /api/v1/auth/account → SessionOrganization { navItems, userNavItems, navGroupOrder }
  ↓ 前端 zod（sessionOrganizationSchema / authSessionOrganizationSchema 必须同步声明键）
session-store.organization
  ↓
features/navigation/menu.tsx：getVisibleRouteKeys（逐叶过滤）
  + orderedGroups（navGroupOrder）+ sortChildrenByOrder（配置数组顺序）
  ↓
AppShell：侧边栏 buildMenuItems、搜索 searchMenuItems、
直达守卫 isRouteKeyVisible（命中被隐藏路由时内容区渲染 403）
```

后端下发为唯一事实源；侧边栏过滤与直达守卫同源，保证「菜单不可见」与「URL 直达被拦」一致。配置在下一次会话引导（登录 / 切换组织）时生效，旧会话沿用先前快照。菜单过滤仅约束入口与页面可见性，API 授权（权限点 + RLS）仍是越权访问的最终防线（见《[security-model.md](../../architecture/security-model.md)》）。

### 平台模板与组织覆盖

平台默认菜单即前端静态 `menuGroups`（事实源 `web/src/features/navigation/menu.tsx`）；组织 navPreferences 是**覆盖层**：一旦配置即整体替换默认可见集合，不存在系统级分组的强制并入——管理员可以隐藏任意分组。未配置的组织保持全量展示。

### accountItems 字段规则

`accountItems` 数组（name / visible / viewRule / modifyRule / regex / tab）约束账户资料页与用户详情页的字段行为：

| 规则 | 值域 | 语义 |
|---|---|---|
| visible | bool | 字段是否整体显示 |
| viewRule | Public / Self / Admin | 可见范围：所有人 / 本人与管理员 / 仅管理员 |
| modifyRule | Self / Admin / Immutable | 可编辑范围：本人可改 / 仅管理员 / 只读 |
| regex | string（可选） | 前端表单校验正则 |

双侧校验边界：

- 后端 `internal/app/organizations/validation.go` 在写入口裁决：viewRule / modifyRule 枚举白名单、regex 可编译性，违规返回 422，脏数据不落库；
- 前端 `web/src/features/account-items/useAccountItems.ts` 按查看者身份解释规则（账户自助页 viewerRole="self"，用户详情页 viewerRole="admin"）；未配置的字段默认可见可编辑（fail-open），无法编译的历史 regex 在前端被忽略。

## API 事实源

组织域 API 端点清单以 `internal/transport/http/api/v1/organizations/routes.go` 为准。

## 组织详情与聚合保存

组织详情采用锚点导航分区呈现（分区事实源：`web/src/pages/organizations/OrganizationDetailPage.tsx` 的 `navItems`）；保存通过 `PUT /api/v1/organizations/{id}/configuration` 在**单事务**内提交 core / settings / theme / securityPolicy：

- 任一子配置校验失败整体回滚，不允许部分生效；
- 折叠分组保持挂载，隐藏字段仍参与校验与提交；
- settings / theme / securityPolicy 的独立端点保留用于细粒度调用；
- 继承策略在写入口校验祖先 lock，命中返回 403（见 `design/b2b/README.md`）。
