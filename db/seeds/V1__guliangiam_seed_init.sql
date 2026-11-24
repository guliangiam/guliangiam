-- ============================================================================
-- V1__guliangiam_seed_init.sql — GuliangIAM 数据链基线种子（DML）
-- ============================================================================
-- @seed-chain: authored file=V1__guliangiam_seed_init.sql
--
-- 唯一事实源（ADR-022 决策 6）：人工编写，不由生成器产出；数据链不存在声明层。
--
-- 合并说明（2026-10-29 数据链 squash）：
--   原 V1..V8 增量脚本合并为本文件。合并时只保留基线必需的幂等写入，删除
--   纯历史修复步骤：V6（V1 已改用无仲裁键 ON CONFLICT，重复策略行不再需要）、
--   V5 中面向旧明细表（llm/notification/payment）的回填与 fail-closed 断言
--   （基线上这些表为空，保留只会让脚本依赖不存在的历史行）。V5 的提供商
--   注册分类（family/kind/variant）仍需保留，否则种子 provider 不满足注册表约束。
--   原 V2/V3/V4/V7/V8 的数据效果全部内联到对应位置。
--
-- 2026-10-29 第二轮种子收敛（仍为数据链 V1，不新增版本）：
--   * 补齐组织子表（auth_config / billing / branding / theme_settings）；
--   * 开发引导管理员凭据（Argon2id 哨兵哈希 + 首次登录强制改密）；
--   * 分组 / 权限 / 商业目录（product / plan / pricing / coupon）；
--   * AI 网关目录（llm_provider / model / route 全部 disabled 或 deprecated；
--     alias 保持 active，用于覆盖「别名存在但无可用路由」的分支）；
--   * Provider 明细样本（provider_configs / provider_secrets）。
--   开发哨兵口令仅用于本地/测试环境：种子只在显式 `AUTO_SEED=true`、服务启动
--   参数 `-seed` 或 `migrate up --seed` 时执行；执行器在 production 检测到哨兵
--   会拒绝播种，并且 `BOOTSTRAP_ADMIN_PASSWORD` 提供的口令始终优先覆盖哨兵。
--
-- 维护规范：
--   1. 所有种子 INSERT 带 ON CONFLICT DO NOTHING（无仲裁键），任何唯一约束
--      （含主键）冲突都安全跳过，保证连续重放幂等。
--   2. 禁止新增增量种子脚本；内容变更直接修改本文件，并同步逐字节一致的
--      内部副本 internal/bootstrap/seed_sql/（门禁 5 断言）。
--   3. 本脚本在结构链之后执行（AUTO_SEED 控制），执行记录写入
--      guliangiam.schema_migrations 的 (kind='data', version=1)。
--   4. 所有 id 为显式 UUIDv7 字面量（主键不设库函数默认值，ADR-022 §7）。
--   5. 第三方地址、密钥与 AI 网关凭据均为占位值，状态 disabled；禁止用于生产。
--
-- UUIDv7 时间戳基线：2026-09-29 00:00:00 UTC → 01a0ea76-0c00-7XXX-8XXX-…
-- ============================================================================
-- RLS bypass：种子由 psql / 播种执行器直接执行时没有 app.* GUC，必须在事务内
-- 显式设置 super_admin，否则 FORCE RLS 会拦截写入。set_config(..., true) 为
-- LOCAL 级，提交后自动清理。
BEGIN;


SELECT set_config('app.current_role', 'super_admin', true);

-- ============================================================
-- 1. Built-in 组织与应用
-- ============================================================

INSERT INTO guliangiam.organizations (id, slug, display_name, website_url, logo_url, logo_dark_url, status, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-000000000001'::uuid, 'built-in', 'Built-in Organization', 'https://bglt.cn', 'https://bglt.cn/images/logo.png', 'https://bglt.cn/images/logo.png', 'active', now(), now()) ON CONFLICT DO NOTHING;

-- 默认部署区域（本地/测试兜底）：创建组织时从 deployment_regions 中取 is_primary
-- 的行（按 region_code 稳定排序），不读取任何环境变量；缺失时创建组织 fail-fast。

INSERT INTO guliangiam.deployment_regions (id, region_code, name, country_code, timezone, is_primary, rto_minutes, rpo_minutes, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-0000000000a1'::uuid, 'default-local', 'Local Development', 'CN', 'Asia/Shanghai', true, 0, 0, now(), now()) ON CONFLICT DO NOTHING;

-- 租户身份：built-in 组织的 1:1 隔离主体（tenant_id == organizations.id）。

INSERT INTO guliangiam.tenant_identities (organization_id, status, isolation_tier, is_built_in, provisioned_at, created_at, updated_at)
VALUES ((SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'active', 'row', true, now(), now(), now()) ON CONFLICT DO NOTHING;

-- 租户隔离拓扑：built-in 的主放置（每租户恰好一条 is_primary）。

INSERT INTO guliangiam.tenant_instances (id, organization_id, region_id, region_code, is_primary, status, placement_kind, storage_boundary, backup_boundary, data_residency_country, provisioned_at, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-0000000000a2'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), (SELECT id FROM guliangiam.deployment_regions WHERE region_code = 'default-local'), 'default-local', true, 'active', 'shared_cluster', 'shared', 'shared', 'CN', now(), now(), now()) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.applications (id, organization_id, slug, display_name, category, app_type, status, client_id, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-000000000002'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'app-built-in', 'Built-in Application', 'identity', 'web', 'active', 'built-in-client', now(), now()) ON CONFLICT DO NOTHING;

-- ============================================================
-- 2. 默认管理员与开发引导凭据
--    种子写入开发哨兵哈希（明文见 AGENTS.md「开发引导口令」），导入种子后本地/测试
--    环境即可登录并强制改密；BOOTSTRAP_ADMIN_PASSWORD 在播种执行器中优先覆盖它，
--    生产环境检测到哨兵会拒绝执行。已有用户自定义哈希不被覆盖。
-- ============================================================

INSERT INTO guliangiam.users (id, organization_id, username, user_type, status, is_admin, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-000000000003'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'admin', 'normal', 'active', true, now(), now()) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.user_profiles (user_id, display_name, email, created_at, updated_at)
VALUES ((SELECT u.id FROM guliangiam.users u JOIN guliangiam.organizations o ON o.id = u.organization_id WHERE u.username = 'admin' AND o.slug = 'built-in' AND u.deleted_at IS NULL AND o.deleted_at IS NULL), 'Administrator', 'admin@built-in.example.com', now(), now()) ON CONFLICT DO NOTHING;

-- 开发引导凭据（哨兵）：仅用于本地/测试环境，明文见 AGENTS.md「开发引导口令」。
-- 生产环境执行器检测到该哨兵会拒绝播种；BOOTSTRAP_ADMIN_PASSWORD 可覆盖它。

INSERT INTO guliangiam.user_credentials (user_id, password_hash, password_algo, password_updated_at, need_update_password)
VALUES ((SELECT u.id FROM guliangiam.users u JOIN guliangiam.organizations o ON o.id = u.organization_id WHERE u.username = 'admin' AND o.slug = 'built-in' AND u.deleted_at IS NULL AND o.deleted_at IS NULL), '$argon2id$v=19$m=65536,t=1,p=8$tDFXTrRgMV3QvcVxL92Eig$kPprEni7O5SEvxJhFrLbwcoPjb/gq3/91hf0DGo0d9Y', 'argon2id', now(), true) ON CONFLICT DO NOTHING;

-- ============================================================
-- 2.1 演示用户（仅用于本地/测试演示用户详情页各表单）
--     固定 UUID 便于前端联调；所有 INSERT 幂等。凭据复用开发哨兵，
--     生产播种同样会因哨兵检测而失败，不引入新的生产风险。
-- ============================================================
 -- 注册链路字段：演示「账号与权限→注册应用」卡（signup_application_id 指向
-- Built-in Application，register_type/register_source 覆盖自助注册场景）。

INSERT INTO guliangiam.users (id, organization_id, username, user_type, status, is_admin, signup_application_id, register_type, register_source, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-000000000004'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'demo-user', 'normal', 'active', false, '01a0ea76-0c00-7000-8000-000000000002'::uuid, 'password', 'self_signup', now(), now()) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.user_profiles (user_id, display_name, first_name, last_name, email, email_verified, phone, phone_verified, country_code, region, location, birthday, gender, bio, real_name, id_card_type, id_card, properties, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-000000000004'::uuid, 'Demo User', 'Demo', 'User', 'demo.user@example.com', true, '+8613800000000', true, 'CN', '示例省', '示例市示例区', '1990-01-01'::date, 'male', '演示账号：覆盖个人资料、实名、地址、托管账户与 MFA 等表单。', '演示用户', 'id_card', 'DEMO000000000000', '{"affiliation":"Platform Engineering","department":"Identity & Access","employeeId":"EMP-1001","costCenter":"CC-2048"}'::jsonb, now(), now()) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.user_credentials (user_id, password_hash, password_algo, password_updated_at, need_update_password)
VALUES ('01a0ea76-0c00-7000-8000-000000000004'::uuid, '$argon2id$v=19$m=65536,t=1,p=8$tDFXTrRgMV3QvcVxL92Eig$kPprEni7O5SEvxJhFrLbwcoPjb/gq3/91hf0DGo0d9Y', 'argon2id', now(), true) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.user_collections (user_id, addresses, managed_accounts, face_ids, cart, mfa_items, mfa_accounts, application_scopes)
VALUES ('01a0ea76-0c00-7000-8000-000000000004'::uuid, '[{"id":"addr-demo-1","countryCode":"CN","province":"示例省","city":"示例市","district":"示例区","postalCode":"000000","street":"示例路 1 号","details":"示例楼 A 座","isDefault":true},
      {"id":"addr-demo-2","countryCode":"CN","province":"示例省","city":"示例市","district":"示例区","postalCode":"000000","street":"示例路 2 号","details":"示例楼 B 座","isDefault":false}]'::jsonb, '[{"application":"GitLab","username":"demo.user","password":"","signinUrl":"https://gitlab.example.com"},
      {"application":"Jenkins","username":"demo.ci","password":"","signinUrl":"https://ci.example.com"}]'::jsonb, '[{"id":"face-demo-1","imageUrl":"https://cdn.example.com/faces/demo-1.png","createdAt":"2026-01-10T08:00:00Z"}]'::jsonb, '[{"productId":"prod-iam-pro","quantity":2,"price":199.00},
      {"productId":"prod-audit-addon","quantity":1,"price":59.00}]'::jsonb, '[{"mfaType":"totp","secret":""},{"mfaType":"webauthn","secret":""}]'::jsonb, '[{"accountName":"demo.user@example.com","issuer":"GuliangIAM","secretKey":"","origin":"otpauth://totp/GuliangIAM:demo.user@example.com"}]'::jsonb, '[{"application":"Grafana","scopes":["dashboard:read","datasource:read"],"grantedAt":"2026-01-15T08:00:00Z"}]'::jsonb) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.user_financials (user_id, balance, balance_credit, balance_currency)
VALUES ('01a0ea76-0c00-7000-8000-000000000004'::uuid, 1280.5000, 200.0000, 'CNY') ON CONFLICT DO NOTHING;

-- 演示用户已启用的 MFA 方法（user_mfa_methods）：驱动「多因素认证」分区的
-- 方法卡片渲染与重命名/编辑交互；TOTP 已验证，Push 未验证以覆盖两种状态。

INSERT INTO guliangiam.user_mfa_methods (id, user_id, method_type, enabled, verified, display_name, last_used_at)
VALUES ('01a0ea76-0c00-7000-8000-0000000000e3'::uuid, '01a0ea76-0c00-7000-8000-000000000004'::uuid, 'totp', true, true, 'Authenticator App (TOTP)', now()),
       ('01a0ea76-0c00-7000-8000-0000000000e4'::uuid, '01a0ea76-0c00-7000-8000-000000000004'::uuid, 'push', true, false, '手机推送', NULL) ON CONFLICT DO NOTHING;

-- 演示用户已绑定的第三方身份（对应第 11 节的 WeChat / Google Provider 样本），
-- 用于演示「身份关联」分区的读写与 Provider 名称解析。

INSERT INTO guliangiam.user_identities (id, user_id, provider_id, identity_type, subject, provider_account, profile_snapshot, linked_at, link_state, revision)
VALUES ('01a0ea76-0c00-7000-8000-0000000000e1'::uuid, '01a0ea76-0c00-7000-8000-000000000004'::uuid, '01a0ea76-0c00-7000-8000-00000000000f', 'OAuth', 'google-demo-0001', 'demo.user@example.com', '{"name":"Demo User","email":"demo.user@example.com","picture":"https://cdn.example.com/avatars/demo.png"}'::jsonb, now(), 'linked', 1),
       ('01a0ea76-0c00-7000-8000-0000000000e2'::uuid, '01a0ea76-0c00-7000-8000-000000000004'::uuid, '01a0ea76-0c00-7000-8000-00000000000d', 'OAuth', 'wechat-demo-0001', 'demo_wechat', '{"nickname":"Demo User","openid":"wechat-demo-0001"}'::jsonb, now(), 'linked', 1) ON CONFLICT DO NOTHING;

-- 自定义属性定义（定义驱动 + 键值兜底）：资料页用这些 key 作为候选提示。

INSERT INTO guliangiam.custom_attribute_definitions (id, organization_id, key, data_type, required, options, writable, visibility, description, enabled, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-0000000000d1'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'department', 'string', false, '[]'::jsonb, true, 'private', '所属部门（自由文本，用于展示与检索）', true, now(), now()),
       ('01a0ea76-0c00-7000-8000-0000000000d2'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'employeeId', 'string', false, '[]'::jsonb, true, 'private', '员工工号', true, now(), now()),
       ('01a0ea76-0c00-7000-8000-0000000000d3'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'costCenter', 'string', false, '[]'::jsonb, true, 'admin', '成本中心编码', true, now(), now()),
       ('01a0ea76-0c00-7000-8000-0000000000d4'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'accessLevel', 'enum', false, '["standard","privileged","contractor"]'::jsonb, true, 'private', '访问级别', true, now(), now()),
       -- 资料页历史键（PATCH /users 的 karma/ranking 与 affiliation 均落在 profile.properties）：
 -- 属性写入按定义校验，缺少定义会让资料保存返回 422，因此基线必须声明这三个键。
 ('01a0ea76-0c00-7000-8000-0000000000d5'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'affiliation', 'string', false, '[]'::jsonb, true, 'private', '所属机构/院系（自由文本）', true, now(), now()),
 ('01a0ea76-0c00-7000-8000-0000000000d6'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'karma', 'number', false, '[]'::jsonb, true, 'private', '社区贡献值', true, now(), now()),
 ('01a0ea76-0c00-7000-8000-0000000000d7'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'ranking', 'number', false, '[]'::jsonb, true, 'private', '排行榜名次', true, now(), now()) ON CONFLICT DO NOTHING;

-- ============================================================
-- 3. Platform Admin（PrincipalTypeResolver 的唯一判定依据）
-- ============================================================
 -- 重放副作用（"种子即真相"）：本行 UPSERT 会把内置管理员的 is_active 恢复为 true。
-- 排查内置管理员"被停用"时，先确认是否发生过种子重放（-seed / AUTO_SEED），
-- 不要把重放后的状态当成人工变更。

INSERT INTO guliangiam.platform_admins (id, user_id, created_by, is_active, created_at, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-000000000019'::uuid, (SELECT u.id FROM guliangiam.users u JOIN guliangiam.organizations o ON o.id = u.organization_id WHERE u.username = 'admin' AND o.slug = 'built-in' AND u.deleted_at IS NULL AND o.deleted_at IS NULL), (SELECT u.id FROM guliangiam.users u JOIN guliangiam.organizations o ON o.id = u.organization_id WHERE u.username = 'admin' AND o.slug = 'built-in' AND u.deleted_at IS NULL AND o.deleted_at IS NULL), true, now(), now(), 1) ON CONFLICT (id) DO
UPDATE
SET is_active = EXCLUDED.is_active;

-- 重放副作用（"种子即真相"）：本行 UPSERT 会把 is_bootstrap 恢复为 true；
-- 若有人工清除该标记，重放后同样会被恢复。排查平台全租户不可见时，
-- 先确认是否发生过种子重放。
-- platform_admin_scopes 声明层为 revision: false（无 revision/updated_at 列）：
-- 该行 revision 不参与并发控制，UPSERT 只恢复 is_bootstrap，不触碰其它列。

INSERT INTO guliangiam.platform_admin_scopes (id, platform_admin_id, scope_type, target_tenants, expires_at, approved_by, approved_at, is_bootstrap, created_at)
VALUES ('01a0ea76-0c00-7000-8000-00000000001a'::uuid, (SELECT id FROM guliangiam.platform_admins WHERE user_id = (SELECT u.id FROM guliangiam.users u JOIN guliangiam.organizations o ON o.id = u.organization_id WHERE u.username = 'admin' AND o.slug = 'built-in' AND u.deleted_at IS NULL AND o.deleted_at IS NULL)), 'all', ARRAY[]::uuid[], now() + INTERVAL '1 year', NULL, NULL, true, now()) ON CONFLICT (id) DO
UPDATE
SET is_bootstrap = EXCLUDED.is_bootstrap;

-- ============================================================
-- 4. 默认 Casbin 模型、Enforcer 与内置角色策略
-- ============================================================

INSERT INTO guliangiam.models (id, organization_id, name, display_name, model_text, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-000000000004'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-model', 'Built-in RBAC Model', '[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act', now(), now()) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.enforcers (id, organization_id, name, display_name, model_id, adapter_id, is_enabled, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-000000000005'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-enforcer', 'Built-in Enforcer', (SELECT id FROM guliangiam.models WHERE name = 'built-in-model' AND organization_id = (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL)), NULL, true, now(), now()) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.roles (id, organization_id, name, display_name, description, is_enabled, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-000000000006'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-admin', 'Built-in Admin', '', true, now(), now()),
       ('01a0ea76-0c00-7000-8000-000000000007'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-viewer', 'Built-in Viewer', 'Read-only access to resources', true, now(), now()),
       ('01a0ea76-0c00-7000-8000-000000000008'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-editor', 'Built-in Editor', 'Read and write access to resources', true, now(), now()),
       ('01a0ea76-0c00-7000-8000-000000000009'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-operator', 'Built-in Operator', 'Operational access: manage users, sessions, and configs', true, now(), now()) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.casbin_rules (id, enforcer_id, ptype, v0, v1, v2, v3)
VALUES ('01a0ea76-0c00-7000-8000-00000000000e'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'admin', 'built-in', '*', '*'),
       ('01a0ea76-0c00-7000-8000-00000000000f'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'g', 'admin', 'built-in-admin', 'built-in', ''),
       -- 治理面显式授权给 built-in-admin 角色：此前只有用户 `admin` 的 `*`/`*` 通配规则，
 -- 绑定到 built-in-admin 的其他成员在 /auth/permissions 中看不到 Actions/ReBAC/
 -- Risk/Metering 的写按钮。与 permissions.yaml 同批补登记，重复导入幂等。
 ('01a0ea76-0c00-7000-8000-0000000000b1'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-admin', 'built-in', 'rebac', '*'),
 ('01a0ea76-0c00-7000-8000-0000000000b2'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-admin', 'built-in', 'actions', '*'),
 ('01a0ea76-0c00-7000-8000-0000000000b3'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-admin', 'built-in', 'risk', 'write'),
 ('01a0ea76-0c00-7000-8000-0000000000b4'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-admin', 'built-in', 'metering', 'write'),
 ('01a0ea76-0c00-7000-8000-0000000000b5'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-admin', 'built-in', 'auth', 'step_up'),
 ('01a0ea76-0c00-7000-8000-0000000000b6'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-admin', 'built-in', 'auth', 'trusted_device'),
 ('01a0ea76-0c00-7000-8000-0000000000b7'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-editor', 'built-in', 'auth', 'step_up'),
 ('01a0ea76-0c00-7000-8000-0000000000b8'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-editor', 'built-in', 'auth', 'trusted_device'),
 ('01a0ea76-0c00-7000-8000-0000000000b9'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-operator', 'built-in', 'auth', 'step_up'),
 ('01a0ea76-0c00-7000-8000-0000000000ba'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-operator', 'built-in', 'auth', 'trusted_device'),
 ('01a0ea76-0c00-7000-8000-0000000000bb'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-admin', 'built-in', 'pay-recon-items', 'approve'),
 ('01a0ea76-0c00-7000-8000-0000000000bc'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-editor', 'built-in', 'pay-recon-items', 'approve'),
 ('01a0ea76-0c00-7000-8000-0000000000bd'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-operator', 'built-in', 'pay-recon-items', 'approve'),
 ('01a0ea76-0c00-7000-8000-000000000010'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-viewer', 'built-in', '*', 'read'),
 ('01a0ea76-0c00-7000-8000-000000000012'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-editor', 'built-in', '*', 'read'),
 ('01a0ea76-0c00-7000-8000-000000000013'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-editor', 'built-in', '*', 'write'),
 ('01a0ea76-0c00-7000-8000-000000000015'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-operator', 'built-in', '*', 'read'),
 ('01a0ea76-0c00-7000-8000-000000000016'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-operator', 'built-in', '*', 'write'),
 ('01a0ea76-0c00-7000-8000-000000000017'::uuid, '01a0ea76-0c00-7000-8000-000000000005'::uuid, 'p', 'built-in-operator', 'built-in', '*', 'delete') ON CONFLICT DO NOTHING;

-- 角色绑定必须位于 roles 插入之后：role_id 由业务键子查询解析，NOT NULL 约束
-- 会在解析不到角色时直接失败。

INSERT INTO guliangiam.user_role_bindings (user_id, role_id)
VALUES ((SELECT u.id FROM guliangiam.users u JOIN guliangiam.organizations o ON o.id = u.organization_id WHERE u.username = 'admin' AND o.slug = 'built-in' AND u.deleted_at IS NULL AND o.deleted_at IS NULL), (SELECT id FROM guliangiam.roles WHERE name = 'built-in-admin' AND organization_id = (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL))),
       ('01a0ea76-0c00-7000-8000-000000000004'::uuid, (SELECT id FROM guliangiam.roles WHERE name = 'built-in-operator' AND organization_id = (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL))) ON CONFLICT DO NOTHING;

-- 分组：管理员组（含 admin）与运维组（空成员，供测试作为可分配目标）。
-- type 闭集裁定（G7-s）：内置组是按角色划分的逻辑分组，取 virtual；
-- 历史 'admin'/'operator' 不在 virtual/physical/department 闭集内。

INSERT INTO guliangiam.groups (id, organization_id, name, display_name, manager, contact_email, type, is_top_group, is_enabled, homepage, users, created_at, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-000000000021'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-administrators', 'Built-in Administrators', 'admin', 'admin@built-in.example.com', 'virtual', true, true, '', '["admin"]'::jsonb, now(), now(), 1),
       ('01a0ea76-0c00-7000-8000-000000000022'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-operators', 'Built-in Operators', '', '', 'virtual', true, true, '', '[]'::jsonb, now(), now(), 1) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.user_group_memberships (user_id, group_id)
VALUES ((SELECT u.id FROM guliangiam.users u JOIN guliangiam.organizations o ON o.id = u.organization_id WHERE u.username = 'admin' AND o.slug = 'built-in' AND u.deleted_at IS NULL AND o.deleted_at IS NULL), (SELECT g.id FROM guliangiam.groups g JOIN guliangiam.organizations o ON o.id = g.organization_id WHERE g.name = 'built-in-administrators' AND o.slug = 'built-in' AND g.is_enabled = true)) ON CONFLICT DO NOTHING;

-- 权限样本：管理员全量、只读角色。roles 字段用于页面展示/绑定参考；
-- 运行时裁决仍由 casbin_rules 与 user_role_bindings 承担。

INSERT INTO guliangiam.permissions (id, organization_id, name, display_name, description, users, groups, roles, domains, resource_type, resources, actions, effect, is_enabled, status, created_at, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-000000000023'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-admin-all', 'Built-in Admin All', 'Full access to every resource', '[]'::jsonb, '["built-in-administrators"]'::jsonb, '["built-in-admin"]'::jsonb, '[]'::jsonb, '', '["*"]'::jsonb, '["*"]'::jsonb, 'Allow', true, 'approved', now(), now(), 1),
       ('01a0ea76-0c00-7000-8000-000000000024'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-viewer-read', 'Built-in Viewer Read', 'Read-only access to every resource', '[]'::jsonb, '[]'::jsonb, '["built-in-viewer"]'::jsonb, '[]'::jsonb, '', '["*"]'::jsonb, '["read"]'::jsonb, 'Allow', true, 'approved', now(), now(), 1) ON CONFLICT DO NOTHING;

-- ============================================================
-- 5. 内置提供商（基础 / 社交登录 / 常见 OAuth 目录）
-- ============================================================
 -- built-in-smtp 为占位演示数据（host/凭据未配置），显式 enabled=false，
-- 避免验证码发送链路选中它做真实 SMTP 拨号；生产需配置真实凭据后再启用。
-- 用 DO UPDATE 幂等修正现存库中被旧种子写成 enabled=true 的占位行（仅限本行）。

INSERT INTO guliangiam.providers (id, organization_id, name, display_name, category, provider_type, family, kind, variant, host, port, client_secret, client_secret2, password, app_secret, enabled, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-00000000000a'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-smtp', 'Built-in SMTP Provider', 'Email', 'SMTP', 'identity', 'smtp', '', 'smtp.example.com', 587, '', '', '', '', false, now(), now()) ON CONFLICT (id) DO
UPDATE
SET enabled = FALSE;


INSERT INTO guliangiam.providers (id, organization_id, name, display_name, category, provider_type, family, kind, variant, client_secret, client_secret2, password, app_secret, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-00000000000b'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-captcha-default', 'Built-in Default Captcha', 'Captcha', 'Default', 'identity', 'default', '', '', '', '', '', now(), now()),
       ('01a0ea76-0c00-7000-8000-00000000000c'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-storage-local', 'Built-in Local Storage', 'Storage', 'Local', 'identity', 'local', '', '', '', '', '', now(), now()) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.providers (id, organization_id, name, display_name, category, provider_type, family, kind, variant, callback_url, scopes, client_id, client_secret, client_secret2, password, app_secret)
VALUES ('01a0ea76-0c00-7000-8000-00000000000d'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-wechat', 'WeChat 微信扫码登录', 'OAuth', 'wechat', 'identity', 'wechat', '', 'http://localhost:8000/api/v1/auth/social/callback', ARRAY['snsapi_login']::text[], '', '', '', '', '') ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.providers (id, organization_id, name, display_name, category, provider_type, family, kind, variant, callback_url, client_secret, client_secret2, password, app_secret)
VALUES ('01a0ea76-0c00-7000-8000-00000000000e'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-faceid', 'FaceID 人脸登录', 'FaceId', 'FaceId', 'identity', 'faceid', '', '', '', '', '', '') ON CONFLICT DO NOTHING;

-- kind 是注册表闭集（ck_providers_kind），必须使用规范键而不是 lower(provider_type)：
-- AzureAD → azure_ad；Apple 无独立 kind，归入 custom（展示名仍是 Apple）。

INSERT INTO guliangiam.providers (id, organization_id, name, display_name, category, provider_type, family, kind, variant, provider_url, client_secret, client_secret2, password, app_secret)
VALUES ('01a0ea76-0c00-7000-8000-00000000000f'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-google', 'Google', 'OAuth', 'Google', 'identity', 'google', '', 'https://accounts.google.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000010'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-github', 'GitHub', 'OAuth', 'GitHub', 'identity', 'github', '', 'https://github.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000011'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-facebook', 'Facebook', 'OAuth', 'Facebook', 'identity', 'facebook', '', 'https://www.facebook.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000012'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-linkedin', 'LinkedIn', 'OAuth', 'LinkedIn', 'identity', 'linkedin', '', 'https://www.linkedin.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000013'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-apple', 'Apple', 'OAuth', 'Apple', 'identity', 'custom', '', 'https://appleid.apple.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000014'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-azuread', 'Azure AD', 'OAuth', 'AzureAD', 'identity', 'azure_ad', '', 'https://login.microsoftonline.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000015'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-lark', 'Lark', 'OAuth', 'Lark', 'identity', 'lark', '', 'https://open.feishu.cn', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000016'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-dingtalk', 'DingTalk', 'OAuth', 'DingTalk', 'identity', 'dingtalk', '', 'https://open.dingtalk.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000017'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-alipay', 'Alipay', 'OAuth', 'Alipay', 'identity', 'alipay', '', 'https://open.alipay.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000018'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-qq', 'QQ', 'OAuth', 'QQ', 'identity', 'qq', '', 'https://connect.qq.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-000000000019'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-weibo', 'Weibo', 'OAuth', 'Weibo', 'identity', 'weibo', '', 'https://open.weibo.com', '', '', '', ''),
       ('01a0ea76-0c00-7000-8000-00000000001a'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-douyin', 'Douyin', 'OAuth', 'Douyin', 'identity', 'douyin', '', 'https://developer.open-douyin.com', '', '', '', '') ON CONFLICT DO NOTHING;

-- FaceID SPI 适配器（独立表；默认禁用，生产需换成真实 endpoint 再启用）

INSERT INTO guliangiam.face_id_providers (id, organization_id, name, provider_type, endpoint, config, is_enabled)
VALUES ('01a0ea76-0c00-7000-8000-00000000000f'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-faceid-spi', 'arcface', 'http://localhost:9000', '{"liveness_threshold": 0.8, "match_threshold": 0.75}'::jsonb, false) ON CONFLICT DO NOTHING;

-- 注册分类直接写在 INSERT 里（family/kind/variant），不再对全表执行
-- lower(provider_type) 推导：kind 是闭集约束，推导会把 AzureAD/Apple 之类
-- 展示名映射成非法键。
 -- AI 提供商注册层样本（family=ai）。明细行见 §10，状态统一 disabled，
-- 避免未配置真实密钥时触发上游探测或计费。

INSERT INTO guliangiam.providers (id, organization_id, name, display_name, category, provider_type, family, kind, variant, client_secret, client_secret2, password, app_secret, enabled, created_at, updated_at)
VALUES ('01a0ea76-0c00-7000-8000-000000000020'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-llm-disabled', 'Built-in LLM (disabled)', 'AI', 'OpenAI', 'ai', 'openai', '', '', '', '', '', false, now(), now()) ON CONFLICT DO NOTHING;

-- ============================================================
-- 6. app-built-in 的配置子表与 Provider 绑定
-- ============================================================

INSERT INTO guliangiam.application_provider_bindings (application_id, provider_id)
SELECT apps.id,
       providers.id
FROM guliangiam.applications apps
CROSS JOIN guliangiam.providers providers
WHERE apps.slug = 'app-built-in'
  AND apps.organization_id =
    (SELECT id
     FROM guliangiam.organizations
     WHERE slug = 'built-in'
       AND deleted_at IS NULL)
  AND providers.organization_id = apps.organization_id
  AND providers.id IN ('01a0ea76-0c00-7000-8000-00000000000d'::uuid,
                       '01a0ea76-0c00-7000-8000-00000000000e'::uuid) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.app_oauth_config (application_id, redirect_uris, grant_types, token_endpoint_auth_method)
SELECT id, ARRAY['http://127.0.0.1:7101/callback'], ARRAY['authorization_code',
                                                          'refresh_token'], 'none'
FROM guliangiam.applications
WHERE slug = 'app-built-in'
  AND organization_id =
    (SELECT id
     FROM guliangiam.organizations
     WHERE slug = 'built-in'
       AND deleted_at IS NULL) ON CONFLICT (application_id) DO
  UPDATE
  SET redirect_uris = EXCLUDED.redirect_uris,
      grant_types = EXCLUDED.grant_types,
      token_endpoint_auth_method = EXCLUDED.token_endpoint_auth_method WHERE guliangiam.app_oauth_config.redirect_uris = '{}';


INSERT INTO guliangiam.app_signin_config (application_id, signin_methods, created_at, updated_at, revision)
SELECT id, ARRAY['Password',
                 'Verification code',
                 'WebAuthn',
                 'Face ID',
                 'Device login']::text[], now(),
                                          now(),
                                          1
FROM guliangiam.applications
WHERE slug = 'app-built-in'
  AND organization_id =
    (SELECT id
     FROM guliangiam.organizations
     WHERE slug = 'built-in'
       AND deleted_at IS NULL) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.app_saml_config (application_id, saml_metadata, created_at, updated_at, revision)
SELECT id,
       '',
       now(),
       now(),
       1
FROM guliangiam.applications
WHERE slug = 'app-built-in'
  AND organization_id =
    (SELECT id
     FROM guliangiam.organizations
     WHERE slug = 'built-in'
       AND deleted_at IS NULL) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.app_form_config (application_id, created_at, updated_at, revision)
SELECT id,
       now(),
       now(),
       1
FROM guliangiam.applications
WHERE slug = 'app-built-in'
  AND organization_id =
    (SELECT id
     FROM guliangiam.organizations
     WHERE slug = 'built-in'
       AND deleted_at IS NULL) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.app_network_config (application_id, created_at, updated_at, revision)
SELECT id,
       now(),
       now(),
       1
FROM guliangiam.applications
WHERE slug = 'app-built-in'
  AND organization_id =
    (SELECT id
     FROM guliangiam.organizations
     WHERE slug = 'built-in'
       AND deleted_at IS NULL) ON CONFLICT DO NOTHING;

-- 公开端点 /applications/resolve 从 provider_items JSONB 读 Provider 列表，
-- 不 JOIN application_provider_bindings；这里同步写入 WeChat + FaceID 快照。

UPDATE guliangiam.app_network_config
SET provider_items = jsonb_build_array(jsonb_build_object('provider', '01a0ea76-0c00-7000-8000-00000000000d', 'canSignUp', true, 'canSignIn', true, 'canUnlink', true), jsonb_build_object('provider', '01a0ea76-0c00-7000-8000-00000000000e', 'canSignUp', true, 'canSignIn', true, 'canUnlink', true))
WHERE application_id =
    (SELECT id
     FROM guliangiam.applications
     WHERE slug = 'app-built-in'
       AND organization_id =
         (SELECT id
          FROM guliangiam.organizations
          WHERE slug = 'built-in'
            AND deleted_at IS NULL))
  AND (provider_items IS NULL
       OR provider_items = '[]'::jsonb);

-- ============================================================
-- 7. 默认密钥占位、验证码 / 通知提供商明细、安全策略与组织设置
-- ============================================================

INSERT INTO guliangiam.keys (id, organization_id, name, display_name, key_type, public_key, private_key, algorithm, is_enabled)
VALUES ('01a0ea76-0c00-7000-8000-00000000000d'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-key', 'Built-in Signing Key Placeholder', 'RSA', '', '', 'RS256', false) ON CONFLICT DO NOTHING;


-- built-in-captcha 为占位演示数据，显式 is_enabled=false：登录/注册的 EnforceCaptcha
-- 按组织级启用行 fail-close，而 app-built-in 未配置 Captcha signin item
-- （docs/design/auth/login.md：必需性由应用 signin_items 控制），前端不会渲染挑战，
-- 启用该行会让新播种库的密码登录被 400 captcha is required 全量阻断且无 UI 出路。
-- 用 DO UPDATE 幂等修正现存库中被旧种子写成 is_enabled=true 的占位行（仅限本行）；
-- 需要人机验证时在控制台启用提供商并为应用绑定 Captcha signin item。
INSERT INTO guliangiam.captcha_providers (id, organization_id, name, provider_type, site_key, secret_key, config, is_enabled, created_at, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-00000000001a'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'Built-in Default Captcha', 'default', 'BUILT_IN_DEFAULT_SITE_KEY', '', '{"variant": "default", "behavior": "pass-through"}'::jsonb, false, now(), now(), 1) ON CONFLICT (id) DO
UPDATE
SET is_enabled = FALSE;


INSERT INTO guliangiam.notification_providers (id, organization_id, name, provider_type, webhook_url, access_token, config, is_enabled, created_at, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-00000000001b'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'Built-in Webhook', 'webhook', 'https://example.com/webhook', '', '{"method": "POST", "timeout_ms": 5000}'::jsonb, true, now(), now(), 1) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.organization_security_policies (organization_id, password_policy, mfa_policy, session_policy)
VALUES ((SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), '{"minLength":8,"maxLength":128,"requireUppercase":true,"requireLowercase":true,"requireDigit":true,"requireSpecial":true,"rejectCommonPasswords":true,"rejectUsername":true,"passwordHistoryCount":5,"passwordMaxAgeDays":0}'::jsonb, '{"enabled":false,"preferredTypes":["totp"]}'::jsonb, '{"maxConcurrentSessions":5,"sessionTTLMinutes":480}'::jsonb) ON CONFLICT DO NOTHING;

-- 内置组织的参考数据取自前端事实源，不手工维护：
-- languages  = web/src/shared/i18n/locales/ 下的语言包目录（字母序）
-- country_codes = web 依赖 libphonenumber-js 的 getCountries()（与
--   web/src/shared/ui/country-data.ts 的回退路径同源；渲染层再用
--   i18n-iso-countries 过滤无英文名的代码）
-- ON CONFLICT DO NOTHING 保留：重放不覆盖租户已改过的设置。

INSERT INTO guliangiam.organization_settings (organization_id, languages, country_codes, user_types, tags, account_items, mfa_items)
SELECT o.id,
       '["de","en","es","fr","ja","ko","pl","pt","ru","tr","uk","vi","zh-CN","zh-TW"]'::jsonb,
       '["CN","US","SG","GB","DE","IN"]'::jsonb,
       '["normal-user","admin"]'::jsonb,
       '[]'::jsonb,
       '[
          {"name":"Name","visible":true,"viewRule":"Public","modifyRule":"Self","tab":"","regex":""},
          {"name":"Display name","visible":true,"viewRule":"Public","modifyRule":"Self","tab":"","regex":""},
          {"name":"Email","visible":true,"viewRule":"Public","modifyRule":"Self","tab":"","regex":""},
          {"name":"Phone","visible":true,"viewRule":"Public","modifyRule":"Self","tab":"","regex":""},
          {"name":"Groups","visible":true,"viewRule":"Public","modifyRule":"Admin","tab":"","regex":""},
          {"name":"Roles","visible":true,"viewRule":"Public","modifyRule":"Admin","tab":"","regex":""}
        ]'::jsonb,
       '[
          {"name":"Email","rule":"Optional"},
          {"name":"TOTP","rule":"Optional"}
        ]'::jsonb
FROM guliangiam.organizations o
WHERE o.slug = 'built-in'
  AND o.deleted_at IS NULL ON CONFLICT (organization_id) DO NOTHING;

-- 组织子表：认证配置 / 计费 / 品牌 / 主题。均为单行（organization_id 主键），
-- 默认值遵循业务模型的「未启用、USD、默认主题」基线。

INSERT INTO guliangiam.organization_auth_config (organization_id, password_type, password_salt, password_obfuscator_type, password_obfuscator_key, password_options, master_password, master_verification_code, default_password, use_email_as_username, ldap_attributes, created_at, updated_at, revision)
SELECT o.id,
       'argon2id',
       '',
       NULL,
       '',
       '{}'::varchar[], '',
                        '',
                        '',
                        false,
                        '{}'::varchar[], now(),
                                         now(),
                                         1
FROM guliangiam.organizations o
WHERE o.slug = 'built-in'
  AND o.deleted_at IS NULL ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.organization_billing (organization_id, balance, balance_credit, currency, default_currency, enable_deposit, mu, default_quota, created_at, updated_at, revision)
SELECT o.id,
       0,
       0,
       'USD',
       'USD',
       false,
       0,
       0,
       now(),
       now(),
       1
FROM guliangiam.organizations o
WHERE o.slug = 'built-in'
  AND o.deleted_at IS NULL ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.organization_branding (organization_id, enable_dark_logo, has_privilege_consent, is_profile_public, use_permanent_avatar, enable_tour, created_at, updated_at, revision)
SELECT o.id,
       false,
       false,
       false,
       false,
       false,
       now(),
       now(),
       1
FROM guliangiam.organizations o
WHERE o.slug = 'built-in'
  AND o.deleted_at IS NULL ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.organization_theme_settings (organization_id, theme_name, tokens, custom_css, assets)
SELECT o.id,
       'default',
       '{"colorPrimary":"#0b8073","colorInfo":"#0b8073","colorSuccess":"#52c41a","colorWarning":"#faad14","colorError":"#ff4d4f","colorBgBase":"#ffffff","colorTextBase":"#000000","colorBgContainer":"#ffffff","colorBorder":"#d9d9d9","borderRadius":8,"fontSize":14,"wireframe":false}'::jsonb,
       '',
       '{}'::jsonb
FROM guliangiam.organizations o
WHERE o.slug = 'built-in'
  AND o.deleted_at IS NULL ON CONFLICT DO NOTHING;

-- ============================================================
-- 8. 历史枚举取值归一化（仅命中旧拼写；基线库通常无匹配行）
-- ============================================================

UPDATE guliangiam.captcha_providers
SET provider_type = CASE provider_type
                        WHEN 'Default' THEN 'default'
                        WHEN 'reCAPTCHA' THEN 'recaptcha'
                        WHEN 'GoogleRecaptcha' THEN 'recaptcha'
                        WHEN 'reCAPTCHAv3' THEN 'recaptcha_v3'
                        WHEN 'reCAPTCHA v3' THEN 'recaptcha_v3'
                        WHEN 'HCaptcha' THEN 'hcaptcha'
                        WHEN 'Turnstile' THEN 'turnstile'
                        WHEN 'Cloudflare' THEN 'turnstile'
                        WHEN 'CloudflareTurnstile' THEN 'turnstile'
                        WHEN 'cloudflare' THEN 'turnstile'
                        WHEN 'GeeTest' THEN 'geetest'
                        WHEN 'Aliyun' THEN 'aliyun_captcha'
                        WHEN 'AliyunCaptcha' THEN 'aliyun_captcha'
                        WHEN 'aliyun' THEN 'aliyun_captcha'
                    END,
                    updated_at = now(),
                    revision = revision + 1
WHERE provider_type IN ('Default',
                        'reCAPTCHA',
                        'GoogleRecaptcha',
                        'reCAPTCHAv3',
                        'reCAPTCHA v3',
                        'HCaptcha',
                        'Turnstile',
                        'Cloudflare',
                        'CloudflareTurnstile',
                        'cloudflare',
                        'GeeTest',
                        'Aliyun',
                        'AliyunCaptcha',
                        'aliyun');

-- B1b 之后 confidential 列只允许空串或 envelope 令牌；历史种子写入的明文哨兵
-- 会让 fail-closed 读取整表 500，重放时一律清空（default 形态 pass-through，不读该列）。

UPDATE guliangiam.captcha_providers
SET secret_key = '',
    updated_at = now(),
    revision = revision + 1
WHERE secret_key = 'BUILT_IN_DEFAULT_SECRET_KEY';


UPDATE guliangiam.notification_providers
SET provider_type = CASE provider_type
                        WHEN 'DingTalk' THEN 'dingtalk'
                        WHEN 'WeCom' THEN 'wecom'
                        WHEN 'Lark' THEN 'lark'
                        WHEN 'Telegram' THEN 'telegram'
                        WHEN 'Discord' THEN 'discord'
                        WHEN 'Slack' THEN 'slack'
                        WHEN 'Bark' THEN 'bark'
                    END,
                    updated_at = now(),
                    revision = revision + 1
WHERE provider_type IN ('DingTalk',
                        'WeCom',
                        'Lark',
                        'Telegram',
                        'Discord',
                        'Slack',
                        'Bark');

-- ============================================================
-- 9. 商业目录样本（product / plan / pricing / coupon）
-- ============================================================

INSERT INTO guliangiam.products (id, organization_id, name, display_name, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-000000000025'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-demo-product', 'Built-in Demo Product', 'Seed fixture for commerce tests', 'Demo catalog item', 'demo', 'USD', 19.90, 0, 0, false, '[]'::jsonb, false, '[]'::jsonb, '', 'active', now(), now(), 1) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.plans (id, organization_id, name, display_name, description, price, currency, period, product, payment_providers, status, is_enabled, is_exclusive, role, created_at, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-000000000026'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-demo-monthly', 'Built-in Demo Monthly', 'Monthly demo plan', 19.90, 'USD', 'monthly', 'built-in-demo-product', '[]'::jsonb, 'active', true, false, 'built-in-viewer', now(), now(), 1) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.pricings (id, organization_id, name, display_name, description, plans, is_enabled, trial_duration, application, created_at, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-000000000027'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-demo-pricing', 'Built-in Demo Pricing', 'Demo pricing with 14-day trial', '["built-in-demo-monthly"]'::jsonb, true, 14, 'app-built-in', now(), now(), 1) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.coupons (id, organization_id, name, display_name, description, code, discount_type, discount, max_discount, scope, products, users, quantity, used_count, max_usage_per_user, start_time, expire_time, min_order_amount, currency, status, created_at, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-000000000028'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-welcome', 'Welcome 10%', 'First-purchase demo coupon', 'WELCOME10', 'percentage', 10, 5, 'all', '["built-in-demo-product"]'::jsonb, '[]'::jsonb, 100, 0, 1, '', '', 0, 'USD', 'active', now(), now(), 1) ON CONFLICT DO NOTHING;

-- ============================================================
-- 10. AI 网关目录样本（provider/model/route 均不可调用；占位凭据不触发外呼）
-- ============================================================

INSERT INTO guliangiam.llm_providers (id, organization_id, provider_id, name, display_name, provider_type, base_url, api_key_sealed, api_key_fingerprint, status, request_timeout_ms, max_retries, priority, weight, rpm_limit, tpm_limit, supported_protocols, preferred_protocol)
VALUES ('01a0ea76-0c00-7000-8000-000000000029'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), '01a0ea76-0c00-7000-8000-000000000020'::uuid, 'built-in-llm-disabled', 'Built-in LLM (disabled)', 'openai', 'https://api.openai.com/v1', '', 'seed-disabled-no-key', 'disabled', 30000, 2, 100, 1, NULL, NULL, '["openai"]'::jsonb, 'openai') ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.llm_models (id, organization_id, provider_id, provider_model_name, display_name, capabilities, max_context_tokens, max_output_tokens, status)
VALUES ('01a0ea76-0c00-7000-8000-00000000002a'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), '01a0ea76-0c00-7000-8000-000000000029'::uuid, 'gpt-4o-mini', 'Built-in GPT-4o mini (disabled)', '{"chat":true,"vision":false}'::jsonb, 128000, 4096, 'deprecated') ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.llm_model_aliases (id, organization_id, alias, display_name, status)
VALUES ('01a0ea76-0c00-7000-8000-00000000002b'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), 'built-in-chat-alias', 'Built-in Chat Alias', 'active') ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.llm_routes (id, organization_id, alias_id, model_id, weight, priority, status)
VALUES ('01a0ea76-0c00-7000-8000-00000000002c'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), '01a0ea76-0c00-7000-8000-00000000002b'::uuid, '01a0ea76-0c00-7000-8000-00000000002a'::uuid, 1, 100, 'disabled') ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.llm_price_versions (id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, currency, effective_from)
VALUES ('01a0ea76-0c00-7000-8000-00000000002d'::uuid, (SELECT id FROM guliangiam.organizations WHERE slug = 'built-in' AND deleted_at IS NULL), '01a0ea76-0c00-7000-8000-00000000002a'::uuid, 1, 0, 0, 'USD', now()) ON CONFLICT DO NOTHING;

-- 注册层元数据：一致性 worker 通过该 config 读取 family/kind/status/detailId。

INSERT INTO guliangiam.provider_configs (provider_id, config_type, config, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-000000000020'::uuid, '_provider_registry_v1', '{"family":"ai","kind":"openai","variant":"","status":"disabled","detailId":"01a0ea76-0c00-7000-8000-000000000029","secrets":{}}'::jsonb, now(), 1) ON CONFLICT DO NOTHING;

-- ============================================================
-- 11. Provider 明细样本（SMTP / WeChat / Google）
-- ============================================================

INSERT INTO guliangiam.provider_configs (provider_id, config_type, config, updated_at, revision)
VALUES ('01a0ea76-0c00-7000-8000-00000000000a'::uuid, 'smtp', '{"host":"smtp.example.com","port":587,"from":"no-reply@example.com","tls":true}'::jsonb, now(), 1),
       ('01a0ea76-0c00-7000-8000-00000000000d'::uuid, 'oauth', '{"authUrl":"https://open.weixin.qq.com/connect/qrconnect","tokenUrl":"https://api.weixin.qq.com/sns/oauth2/access_token","userInfoUrl":"https://api.weixin.qq.com/sns/userinfo","callbackUrl":"http://localhost:8000/api/v1/auth/social/callback"}'::jsonb, now(), 1),
       ('01a0ea76-0c00-7000-8000-00000000000f'::uuid, 'oauth', '{"authUrl":"https://accounts.google.com/o/oauth2/v2/auth","tokenUrl":"https://oauth2.googleapis.com/token","userInfoUrl":"https://openidconnect.googleapis.com/v1/userinfo","callbackUrl":"http://localhost:8000/api/v1/auth/social/callback"}'::jsonb, now(), 1) ON CONFLICT DO NOTHING;


INSERT INTO guliangiam.provider_secrets (id, provider_id, secret_type, status, last_rotated_at, expires_at, updated_at, revision, version)
VALUES ('01a0ea76-0c00-7000-8000-00000000002e'::uuid, '01a0ea76-0c00-7000-8000-00000000000d'::uuid, 'client_secret', 'active', now(), NULL, now(), 1, 1),
       ('01a0ea76-0c00-7000-8000-00000000002f'::uuid, '01a0ea76-0c00-7000-8000-00000000000f'::uuid, 'client_secret', 'active', now(), NULL, now(), 1, 1) ON CONFLICT DO NOTHING;


COMMIT;