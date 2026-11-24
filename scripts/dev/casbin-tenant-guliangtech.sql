-- ============================================================================
-- guliangtech（北京谷梁科技有限公司，企业域）Casbin 访问控制初始化脚本
--
-- 命名规范：本租户所有角色 / 模型名称统一使用 `guliang_` 前缀；
--           p 策略的 sub（角色）同样带前缀，v1(dom) 固定为租户 slug `guliangtech`。
--           资源名（obj）与动作（act）是运行期路由契约（resourceFromPath +
--           read/write/delete 别名），不能加租户前缀，否则鉴权永不命中；
--           其租户隔离由 v1(dom) + 每租户独立 enforcer 保证。
--
-- 覆盖表：models（模型）、enforcers.model_id（模型挂接）、roles（角色定义）、
--         casbin_rules（p 权限策略 / g 用户-角色绑定）、user_role_bindings
--         （角色分配）。permissions 表是“动态权限定义”，创建时由应用写入
--         casbin_rules；此处直接写静态策略，不重复登记，避免双来源。
--
-- 幂等：先清理旧的非前缀配置（同批次升级），再按业务键 upsert；
--       重复执行不会产生重复角色 / 策略 / 绑定。
-- 生效：直接写库绕过运行期 EnforceCache，执行后需重启服务。
--
-- 用法：
--   psql "$DATABASE_URL" -f scripts/dev/casbin-tenant-guliangtech.sql
-- ============================================================================

\set ON_ERROR_STOP on

BEGIN;

-- RLS FORCE 表要求显式超级管理员上下文（脚本无会话组织）。
SELECT set_config('app.current_role', 'super_admin', true);

-- ----------------------------------------------------------------------------
-- 0. 清理旧版非前缀配置（上一版脚本写入的 super_admin 等 + 手工遗留的
--    guliang-admin）。仅作用于本租户，不影响 ccbupt / built-in。
-- ----------------------------------------------------------------------------
DELETE FROM guliangiam.casbin_rules cr
USING guliangiam.enforcers e
JOIN guliangiam.organizations o ON o.id = e.organization_id
WHERE cr.enforcer_id = e.id
  AND o.slug = 'guliangtech'
  AND (
        (cr.ptype = 'p' AND cr.v0 IN (
            'super_admin', 'security_admin', 'audit_viewer',
            'ai_ops', 'app_dev', 'ops', 'employee'))
     OR (cr.ptype = 'g' AND cr.v0 IN ('admin', 'carl') AND cr.v1 IN (
            'super_admin', 'security_admin', 'audit_viewer',
            'ai_ops', 'app_dev', 'ops', 'employee', 'guliang-admin'))
  );

DELETE FROM guliangiam.user_role_bindings b
USING guliangiam.roles r
JOIN guliangiam.organizations o ON o.id = r.organization_id
WHERE b.role_id = r.id
  AND o.slug = 'guliangtech'
  AND r.name IN (
        'super_admin', 'security_admin', 'audit_viewer',
        'ai_ops', 'app_dev', 'ops', 'employee', 'guliang-admin');

DELETE FROM guliangiam.roles r
USING guliangiam.organizations o
WHERE r.organization_id = o.id
  AND o.slug = 'guliangtech'
  AND r.name IN (
        'super_admin', 'security_admin', 'audit_viewer',
        'ai_ops', 'app_dev', 'ops', 'employee', 'guliang-admin');

-- ----------------------------------------------------------------------------
-- 1. Casbin 模型（与运行期 defaultRBACModel 一致，名称为租户前缀）
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.models AS m (
    id, organization_id, name, display_name, description,
    model_text, owner, is_enabled, created_at, updated_at
)
SELECT gen_random_uuid(),
       o.id,
       'guliang_tenant_casbin_rbac',
       'GuliangTenant Casbin RBAC',
       '企业域 RBAC：资源 + 动作，org 维度强制隔离',
       $model$[request_definition]
r = sub, org, obj, act

[policy_definition]
p = sub, org, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub, r.org)) && r.org == p.org && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)$model$,
       o.slug,
       true,
       now(),
       now()
FROM guliangiam.organizations o
WHERE o.slug = 'guliangtech'
  AND o.deleted_at IS NULL
ON CONFLICT ON CONSTRAINT uq_models_org_name
DO UPDATE SET model_text = EXCLUDED.model_text,
              display_name = EXCLUDED.display_name,
              description = EXCLUDED.description,
              updated_at = now()
WHERE m.model_text IS DISTINCT FROM EXCLUDED.model_text
   OR m.display_name IS DISTINCT FROM EXCLUDED.display_name
   OR m.description IS DISTINCT FROM EXCLUDED.description;

UPDATE guliangiam.enforcers e
SET model_id = m.id,
    updated_at = now()
FROM guliangiam.models m
JOIN guliangiam.organizations o ON o.id = m.organization_id
WHERE e.organization_id = o.id
  AND e.name = 'default'
  AND m.name = 'guliang_tenant_casbin_rbac'
  AND o.slug = 'guliangtech'
  AND o.deleted_at IS NULL
  AND e.model_id IS DISTINCT FROM m.id;

-- 旧模型（上一版脚本的 tenant-casbin-rbac）已无引用，删除避免双来源。
DELETE FROM guliangiam.models m
USING guliangiam.organizations o
WHERE m.organization_id = o.id
  AND o.slug = 'guliangtech'
  AND m.name = 'tenant-casbin-rbac';

-- ----------------------------------------------------------------------------
-- 2. 角色定义（guliang_ 前缀）
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.roles (
    id, organization_id, name, display_name, description,
    is_enabled, created_at, updated_at
)
SELECT gen_random_uuid(), o.id, v.name, v.display_name, v.description, true, now(), now()
FROM guliangiam.organizations o
CROSS JOIN (VALUES
    ('guliang_super_admin',    '企业超级管理员', '企业域全量权限；不含跨租户能力'),
    ('guliang_security_admin', '安全合规官',     '安全策略、密钥、规则、隐私擦除与会话处置'),
    ('guliang_audit_viewer',   '审计员',         '审计、交易与治理面只读；与操作角色职责分离'),
    ('guliang_ai_ops',         'AI 网关运营',    'AI 网关供应商/模型/路由/密钥/配额/价格'),
    ('guliang_app_dev',        '应用开发者',     '应用、身份/验证码/通知/人脸识别提供商与证书只读'),
    ('guliang_ops',            '运维工程师',     'Webhook、同步器、LDAP、网关、RADIUS、Agent、工单'),
    ('guliang_employee',       '普通员工',       '仅自助：会话、令牌、Passkey、应用门户')
) AS v(name, display_name, description)
WHERE o.slug = 'guliangtech'
  AND o.deleted_at IS NULL
ON CONFLICT ON CONSTRAINT uq_roles_org_name DO NOTHING;

-- ----------------------------------------------------------------------------
-- 3. 权限策略（p）：obj = 路径首段资源，act = read/write/delete 或专用动作
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.casbin_rules (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5)
SELECT gen_random_uuid(), e.id, 'p', v.role, 'guliangtech', v.obj, v.act, '', ''
FROM guliangiam.enforcers e
JOIN guliangiam.organizations o ON o.id = e.organization_id
CROSS JOIN (VALUES
    -- ===== guliang_super_admin：企业内全量（不含跨租户） =====
    ('guliang_super_admin', '*', '*'),

    -- ===== guliang_security_admin：安全合规 =====
    ('guliang_security_admin', 'users', 'read'),
    ('guliang_security_admin', 'users', 'write'),
    ('guliang_security_admin', 'users', 'password:reset'),
    ('guliang_security_admin', 'sessions', 'read'),
    ('guliang_security_admin', 'sessions', 'write'),
    ('guliang_security_admin', 'keys', 'read'),
    ('guliang_security_admin', 'keys', 'write'),
    ('guliang_security_admin', 'keys', 'delete'),
    ('guliang_security_admin', 'rules', 'read'),
    ('guliang_security_admin', 'rules', 'write'),
    ('guliang_security_admin', 'rules', 'delete'),
    ('guliang_security_admin', 'policies', 'read'),
    ('guliang_security_admin', 'policies', 'write'),
    ('guliang_security_admin', 'organizations', 'read'),
    ('guliang_security_admin', 'organizations', 'write'),
    ('guliang_security_admin', 'certs', 'read'),
    ('guliang_security_admin', 'certs', 'write'),
    ('guliang_security_admin', 'records', 'read'),
    ('guliang_security_admin', 'risk', 'read'),
    ('guliang_security_admin', 'idv', 'read'),
    ('guliang_security_admin', 'privacy', 'read'),
    ('guliang_security_admin', 'privacy', 'write'),
    ('guliang_security_admin', 'privacy', 'erase'),
    ('guliang_security_admin', 'b2b', 'read'),
    ('guliang_security_admin', 'b2b', 'write'),
    ('guliang_security_admin', 'dashboard', 'read'),

    -- ===== guliang_audit_viewer：只读，不得继承操作角色 =====
    ('guliang_audit_viewer', 'records', 'read'),
    ('guliang_audit_viewer', 'entries', 'read'),
    ('guliang_audit_viewer', 'payments', 'read'),
    ('guliang_audit_viewer', 'orders', 'read'),
    ('guliang_audit_viewer', 'transactions', 'read'),
    ('guliang_audit_viewer', 'subscriptions', 'read'),
    ('guliang_audit_viewer', 'pay-channel-configs', 'read'),
    ('guliang_audit_viewer', 'users', 'read'),
    ('guliang_audit_viewer', 'organizations', 'read'),
    ('guliang_audit_viewer', 'sessions', 'read'),
    ('guliang_audit_viewer', 'risk', 'read'),
    ('guliang_audit_viewer', 'iga', 'read'),
    ('guliang_audit_viewer', 'idv', 'read'),
    ('guliang_audit_viewer', 'privacy', 'read'),
    ('guliang_audit_viewer', 'dashboard', 'read'),

    -- ===== guliang_ai_ops：AI 网关全能力 =====
    ('guliang_ai_ops', 'aigateway:providers', 'read'),
    ('guliang_ai_ops', 'aigateway:providers', 'write'),
    ('guliang_ai_ops', 'aigateway:providers', 'delete'),
    ('guliang_ai_ops', 'aigateway:models', 'read'),
    ('guliang_ai_ops', 'aigateway:models', 'write'),
    ('guliang_ai_ops', 'aigateway:models', 'delete'),
    ('guliang_ai_ops', 'aigateway:aliases', 'read'),
    ('guliang_ai_ops', 'aigateway:aliases', 'write'),
    ('guliang_ai_ops', 'aigateway:aliases', 'delete'),
    ('guliang_ai_ops', 'aigateway:routes', 'read'),
    ('guliang_ai_ops', 'aigateway:routes', 'write'),
    ('guliang_ai_ops', 'aigateway:routes', 'delete'),
    ('guliang_ai_ops', 'aigateway:api_keys', 'read'),
    ('guliang_ai_ops', 'aigateway:api_keys', 'write'),
    ('guliang_ai_ops', 'aigateway:api_keys', 'delete'),
    ('guliang_ai_ops', 'aigateway:quotas', 'read'),
    ('guliang_ai_ops', 'aigateway:quotas', 'write'),
    ('guliang_ai_ops', 'aigateway:quotas', 'delete'),
    ('guliang_ai_ops', 'aigateway:prices', 'read'),
    ('guliang_ai_ops', 'aigateway:prices', 'write'),
    ('guliang_ai_ops', 'aigateway:prices', 'delete'),
    ('guliang_ai_ops', 'aigateway:usage', 'read'),
    ('guliang_ai_ops', 'aigateway:health', 'read'),
    ('guliang_ai_ops', 'aigateway:chat', 'read'),
    ('guliang_ai_ops', 'aigateway:chat', 'write'),
    ('guliang_ai_ops', 'metering', 'read'),
    ('guliang_ai_ops', 'metering', 'write'),
    ('guliang_ai_ops', 'dashboard', 'read'),

    -- ===== guliang_app_dev：应用与提供商，用户只读 =====
    ('guliang_app_dev', 'applications', 'read'),
    ('guliang_app_dev', 'applications', 'write'),
    ('guliang_app_dev', 'applications', 'delete'),
    ('guliang_app_dev', 'providers', 'read'),
    ('guliang_app_dev', 'providers', 'write'),
    ('guliang_app_dev', 'providers', 'delete'),
    ('guliang_app_dev', 'captcha-providers', 'read'),
    ('guliang_app_dev', 'captcha-providers', 'write'),
    ('guliang_app_dev', 'notification-providers', 'read'),
    ('guliang_app_dev', 'notification-providers', 'write'),
    ('guliang_app_dev', 'face-id-providers', 'read'),
    ('guliang_app_dev', 'face-id-providers', 'write'),
    ('guliang_app_dev', 'forms', 'read'),
    ('guliang_app_dev', 'forms', 'write'),
    ('guliang_app_dev', 'certs', 'read'),
    ('guliang_app_dev', 'users', 'read'),
    ('guliang_app_dev', 'groups', 'read'),
    ('guliang_app_dev', 'roles', 'read'),
    ('guliang_app_dev', 'organizations', 'read'),
    ('guliang_app_dev', 'dashboard', 'read'),

    -- ===== guliang_ops：基础设施集成与运维 =====
    ('guliang_ops', 'webhooks', 'read'),
    ('guliang_ops', 'webhooks', 'write'),
    ('guliang_ops', 'webhooks', 'delete'),
    ('guliang_ops', 'syncers', 'read'),
    ('guliang_ops', 'syncers', 'write'),
    ('guliang_ops', 'syncers', 'delete'),
    ('guliang_ops', 'ldaps', 'read'),
    ('guliang_ops', 'ldaps', 'write'),
    ('guliang_ops', 'ldaps', 'delete'),
    ('guliang_ops', 'ldap-sync-configs', 'read'),
    ('guliang_ops', 'ldap-sync-configs', 'write'),
    ('guliang_ops', 'gateway', 'read'),
    ('guliang_ops', 'gateway', 'write'),
    ('guliang_ops', 'gateway', 'delete'),
    -- 网关菜单按 server:read / site:read 派生；运行期仍只校验 gateway。
    ('guliang_ops', 'servers', 'read'),
    ('guliang_ops', 'sites', 'read'),
    ('guliang_ops', 'radius-configs', 'read'),
    ('guliang_ops', 'radius-configs', 'write'),
    ('guliang_ops', 'radius-configs', 'delete'),
    ('guliang_ops', 'agents', 'read'),
    ('guliang_ops', 'agents', 'write'),
    ('guliang_ops', 'agents', 'delete'),
    ('guliang_ops', 'tickets', 'read'),
    ('guliang_ops', 'tickets', 'write'),
    ('guliang_ops', 'tickets', 'delete'),
    ('guliang_ops', 'mcp-servers', 'read'),
    ('guliang_ops', 'mcp-servers', 'write'),
    ('guliang_ops', 'notifications', 'read'),
    ('guliang_ops', 'records', 'read'),
    ('guliang_ops', 'sessions', 'read'),
    ('guliang_ops', 'sessions', 'write'),
    ('guliang_ops', 'certs', 'read'),
    ('guliang_ops', 'organizations', 'read'),
    ('guliang_ops', 'dashboard', 'read'),

    -- ===== guliang_employee：仅自助 =====
    ('guliang_employee', 'sessions', 'read'),
    ('guliang_employee', 'tokens', 'read'),
    ('guliang_employee', 'webauthn', 'read'),
    ('guliang_employee', 'webauthn', 'write'),
    ('guliang_employee', 'auth', 'step_up'),
    ('guliang_employee', 'auth', 'trusted_device'),
    ('guliang_employee', 'applications', 'read'),
    ('guliang_employee', 'notifications', 'read'),
    ('guliang_employee', 'privacy', 'read')
) AS v(role, obj, act)
WHERE o.slug = 'guliangtech'
  AND o.deleted_at IS NULL
  AND e.name = 'default'
ON CONFLICT ON CONSTRAINT uq_casbin_rules_rule DO NOTHING;

-- ----------------------------------------------------------------------------
-- 4. 角色分配（IAM 绑定表）——绑定已有用户
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.user_role_bindings (user_id, role_id)
SELECT u.id, r.id
FROM guliangiam.users u
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.roles r ON r.organization_id = o.id
JOIN (VALUES
    ('admin', 'guliang_super_admin'),
    ('carl',    'guliang_employee')
) AS b(username, role_name) ON b.username = u.username AND b.role_name = r.name
WHERE o.slug = 'guliangtech'
  AND o.deleted_at IS NULL
  AND u.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- ----------------------------------------------------------------------------
-- 5. Casbin g 行：用户 → 角色（鉴权侧直接读取 g(r.sub, p.sub, r.org)）
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.casbin_rules (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5)
SELECT gen_random_uuid(), e.id, 'g', v.username, v.role_name, 'guliangtech', '', '', ''
FROM guliangiam.enforcers e
JOIN guliangiam.organizations o ON o.id = e.organization_id
JOIN guliangiam.users u ON u.organization_id = o.id
JOIN (VALUES
    ('admin', 'guliang_super_admin'),
    ('carl',    'guliang_employee')
) AS v(username, role_name) ON v.username = u.username
WHERE o.slug = 'guliangtech'
  AND o.deleted_at IS NULL
  AND u.deleted_at IS NULL
  AND e.name = 'default'
ON CONFLICT ON CONSTRAINT uq_casbin_rules_rule DO NOTHING;

-- ----------------------------------------------------------------------------
-- 6. 完整性核对
-- ----------------------------------------------------------------------------
SELECT 'roles' AS item,
       count(*) AS total,
       count(*) FILTER (WHERE r.name LIKE 'guliang\_%') AS prefixed,
       count(*) FILTER (WHERE r.name NOT LIKE 'guliang\_%') AS unprefixed
FROM guliangiam.roles r
JOIN guliangiam.organizations o ON o.id = r.organization_id
WHERE o.slug = 'guliangtech';

SELECT cr.ptype,
       count(*) AS total,
       count(*) FILTER (
           WHERE (cr.ptype = 'p' AND cr.v1 <> 'guliangtech')
              OR (cr.ptype = 'g' AND cr.v2 <> 'guliangtech')
       ) AS wrong_domain,
       count(*) FILTER (WHERE cr.ptype = 'p' AND cr.v0 NOT LIKE 'guliang\_%') AS unprefixed_subject
FROM guliangiam.casbin_rules cr
JOIN guliangiam.enforcers e ON e.id = cr.enforcer_id
JOIN guliangiam.organizations o ON o.id = e.organization_id
WHERE o.slug = 'guliangtech'
GROUP BY cr.ptype
ORDER BY cr.ptype;

SELECT count(*) AS duplicate_rows
FROM (
    SELECT enforcer_id, ptype, v0, v1, v2, v3
    FROM guliangiam.casbin_rules cr
    JOIN guliangiam.enforcers e ON e.id = cr.enforcer_id
    JOIN guliangiam.organizations o ON o.id = e.organization_id
    WHERE o.slug = 'guliangtech'
    GROUP BY enforcer_id, ptype, v0, v1, v2, v3
    HAVING count(*) > 1
) d;

COMMIT;
