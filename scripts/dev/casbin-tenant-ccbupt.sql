-- ============================================================================
-- ccbupt（北京邮电大学世纪学院，高校域）Casbin 访问控制初始化脚本
--
-- 命名规范：本租户所有角色 / 模型名称统一使用 `ccbupt_` 前缀；
--           p 策略的 sub（角色）同样带前缀，v1(dom) 固定为租户 slug `ccbupt`。
--           资源名（obj）与动作（act）是运行期路由契约（resourceFromPath +
--           read/write/delete 别名），不能加租户前缀，否则鉴权永不命中；
--           其租户隔离由 v1(dom) + 每租户独立 enforcer 保证。
--
-- 覆盖表：models、enforcers.model_id、roles、casbin_rules（p/g）、
--         user_role_bindings；另补 7 个演示账号作为角色绑定主体。
--         permissions 表是“动态权限定义”，创建时由应用写入 casbin_rules；
--         此处直接写静态策略，不重复登记，避免双来源。
--
-- SEDPU 映射：本仓库无 /api/v1/lims/*、/api/v1/hrms/* 路由，高校域角色先
--             映射到 IAM 现有人员 / 网关 / 支付 / 审计资源；行级数据范围
--             （DEPT/SELF）由业务层闭包表裁剪，Casbin 只保证资源+动作+租户。
--
-- 幂等：先清理旧的非前缀配置，再按业务键 upsert；重复执行不产生重复数据。
-- 生效：直接写库绕过运行期 EnforceCache，执行后需重启服务。
--
-- 用法：
--   psql "$DATABASE_URL" -f scripts/dev/casbin-tenant-ccbupt.sql
-- ============================================================================

\set ON_ERROR_STOP on

BEGIN;

-- RLS FORCE 表要求显式超级管理员上下文（脚本无会话组织）。
SELECT set_config('app.current_role', 'super_admin', true);

-- ----------------------------------------------------------------------------
-- 0. 清理旧版非前缀配置（上一版脚本写入的角色 / 策略 / g 行）
-- ----------------------------------------------------------------------------
DELETE FROM guliangiam.casbin_rules cr
USING guliangiam.enforcers e
JOIN guliangiam.organizations o ON o.id = e.organization_id
WHERE cr.enforcer_id = e.id
  AND o.slug = 'ccbupt'
  AND (
        (cr.ptype = 'p' AND cr.v0 IN (
            'sys_admin', 'hr_admin', 'lab_admin',
            'dept_admin', 'finance_auditor', 'teacher', 'student'))
     OR (cr.ptype = 'g' AND cr.v0 IN (
            'admin_it', 'zhang_hr', 'li_lab', 'wang_cs',
            'fin_zhao', 'prof_liu', 'student_zhao')
         AND cr.v1 IN (
            'sys_admin', 'hr_admin', 'lab_admin',
            'dept_admin', 'finance_auditor', 'teacher', 'student'))
  );

DELETE FROM guliangiam.user_role_bindings b
USING guliangiam.roles r
JOIN guliangiam.organizations o ON o.id = r.organization_id
WHERE b.role_id = r.id
  AND o.slug = 'ccbupt'
  AND r.name IN (
        'sys_admin', 'hr_admin', 'lab_admin',
        'dept_admin', 'finance_auditor', 'teacher', 'student');

DELETE FROM guliangiam.roles r
USING guliangiam.organizations o
WHERE r.organization_id = o.id
  AND o.slug = 'ccbupt'
  AND r.name IN (
        'sys_admin', 'hr_admin', 'lab_admin',
        'dept_admin', 'finance_auditor', 'teacher', 'student');

-- ----------------------------------------------------------------------------
-- 1. Casbin 模型（与运行期 defaultRBACModel 一致，名称为租户前缀）
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.models AS m (
    id, organization_id, name, display_name, description,
    model_text, owner, is_enabled, created_at, updated_at
)
SELECT gen_random_uuid(),
       o.id,
       'ccbupt_tenant_casbin_rbac',
       'CCBUPT Tenant Casbin RBAC',
       '高校域 RBAC：资源 + 动作，org 维度强制隔离',
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
WHERE o.slug = 'ccbupt'
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
  AND m.name = 'ccbupt_tenant_casbin_rbac'
  AND o.slug = 'ccbupt'
  AND o.deleted_at IS NULL
  AND e.model_id IS DISTINCT FROM m.id;

DELETE FROM guliangiam.models m
USING guliangiam.organizations o
WHERE m.organization_id = o.id
  AND o.slug = 'ccbupt'
  AND m.name = 'tenant-casbin-rbac';

-- ----------------------------------------------------------------------------
-- 2. 角色定义（ccbupt_ 前缀）
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.roles (
    id, organization_id, name, display_name, description,
    is_enabled, created_at, updated_at
)
SELECT gen_random_uuid(), o.id, v.name, v.display_name, v.description, true, now(), now()
FROM guliangiam.organizations o
CROSS JOIN (VALUES
    ('ccbupt_sys_admin',       '信息中心超级管理员', '高校域全量权限；不含跨租户能力'),
    ('ccbupt_hr_admin',        '人事专员',           '人员、用户组、邀请与目录同步'),
    ('ccbupt_lab_admin',       '实验室管理处',       '网关设备、Agent、RADIUS、工单等基础设施面'),
    ('ccbupt_dept_admin',      '院系管理员',         '本院系人员受限写；数据范围由业务层闭包表裁剪'),
    ('ccbupt_finance_auditor', '财务审计员',         '支付、订单、订阅与对账只读；与处置角色职责分离'),
    ('ccbupt_teacher',         '教师',               '自助 + 应用门户只读'),
    ('ccbupt_student',         '学生',               '自助 + 应用门户只读')
) AS v(name, display_name, description)
WHERE o.slug = 'ccbupt'
  AND o.deleted_at IS NULL
ON CONFLICT ON CONSTRAINT uq_roles_org_name DO NOTHING;

-- ----------------------------------------------------------------------------
-- 3. 权限策略（p）
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.casbin_rules (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5)
SELECT gen_random_uuid(), e.id, 'p', v.role, 'ccbupt', v.obj, v.act, '', ''
FROM guliangiam.enforcers e
JOIN guliangiam.organizations o ON o.id = e.organization_id
CROSS JOIN (VALUES
    -- ===== ccbupt_sys_admin：校内全量（不含跨租户） =====
    ('ccbupt_sys_admin', '*', '*'),

    -- ===== ccbupt_hr_admin：人事域 =====
    ('ccbupt_hr_admin', 'users', 'read'),
    ('ccbupt_hr_admin', 'users', 'write'),
    ('ccbupt_hr_admin', 'users', 'delete'),
    ('ccbupt_hr_admin', 'users', 'password:reset'),
    ('ccbupt_hr_admin', 'groups', 'read'),
    ('ccbupt_hr_admin', 'groups', 'write'),
    ('ccbupt_hr_admin', 'groups', 'delete'),
    ('ccbupt_hr_admin', 'roles', 'read'),
    ('ccbupt_hr_admin', 'invitations', 'read'),
    ('ccbupt_hr_admin', 'invitations', 'write'),
    ('ccbupt_hr_admin', 'invitations', 'delete'),
    ('ccbupt_hr_admin', 'ldaps', 'read'),
    ('ccbupt_hr_admin', 'syncers', 'read'),
    ('ccbupt_hr_admin', 'syncers', 'write'),
    ('ccbupt_hr_admin', 'tickets', 'read'),
    ('ccbupt_hr_admin', 'tickets', 'write'),
    ('ccbupt_hr_admin', 'records', 'read'),
    ('ccbupt_hr_admin', 'organizations', 'read'),
    ('ccbupt_hr_admin', 'dashboard', 'read'),

    -- ===== ccbupt_lab_admin：基础设施面（LIMS 设备/安全资源的 IAM 侧等价映射） =====
    ('ccbupt_lab_admin', 'gateway', 'read'),
    ('ccbupt_lab_admin', 'gateway', 'write'),
    ('ccbupt_lab_admin', 'gateway', 'delete'),
    ('ccbupt_lab_admin', 'servers', 'read'),
    ('ccbupt_lab_admin', 'sites', 'read'),
    ('ccbupt_lab_admin', 'agents', 'read'),
    ('ccbupt_lab_admin', 'agents', 'write'),
    ('ccbupt_lab_admin', 'agents', 'delete'),
    ('ccbupt_lab_admin', 'radius-configs', 'read'),
    ('ccbupt_lab_admin', 'radius-configs', 'write'),
    ('ccbupt_lab_admin', 'tickets', 'read'),
    ('ccbupt_lab_admin', 'tickets', 'write'),
    ('ccbupt_lab_admin', 'tickets', 'delete'),
    ('ccbupt_lab_admin', 'providers', 'read'),
    ('ccbupt_lab_admin', 'webhooks', 'read'),
    ('ccbupt_lab_admin', 'users', 'read'),
    ('ccbupt_lab_admin', 'applications', 'read'),
    ('ccbupt_lab_admin', 'records', 'read'),
    ('ccbupt_lab_admin', 'dashboard', 'read'),

    -- ===== ccbupt_dept_admin：院系范围（行级范围由业务层裁剪） =====
    ('ccbupt_dept_admin', 'users', 'read'),
    ('ccbupt_dept_admin', 'users', 'write'),
    ('ccbupt_dept_admin', 'groups', 'read'),
    ('ccbupt_dept_admin', 'tickets', 'read'),
    ('ccbupt_dept_admin', 'tickets', 'write'),
    ('ccbupt_dept_admin', 'applications', 'read'),
    ('ccbupt_dept_admin', 'organizations', 'read'),
    ('ccbupt_dept_admin', 'records', 'read'),
    ('ccbupt_dept_admin', 'dashboard', 'read'),

    -- ===== ccbupt_finance_auditor：财务只读 =====
    ('ccbupt_finance_auditor', 'payments', 'read'),
    ('ccbupt_finance_auditor', 'orders', 'read'),
    ('ccbupt_finance_auditor', 'transactions', 'read'),
    ('ccbupt_finance_auditor', 'subscriptions', 'read'),
    ('ccbupt_finance_auditor', 'plans', 'read'),
    ('ccbupt_finance_auditor', 'pricings', 'read'),
    ('ccbupt_finance_auditor', 'products', 'read'),
    ('ccbupt_finance_auditor', 'coupons', 'read'),
    ('ccbupt_finance_auditor', 'pay-channel-configs', 'read'),
    ('ccbupt_finance_auditor', 'records', 'read'),
    ('ccbupt_finance_auditor', 'dashboard', 'read'),

    -- ===== ccbupt_teacher：自助 + 应用门户 =====
    ('ccbupt_teacher', 'sessions', 'read'),
    ('ccbupt_teacher', 'tokens', 'read'),
    ('ccbupt_teacher', 'webauthn', 'read'),
    ('ccbupt_teacher', 'webauthn', 'write'),
    ('ccbupt_teacher', 'auth', 'step_up'),
    ('ccbupt_teacher', 'auth', 'trusted_device'),
    ('ccbupt_teacher', 'applications', 'read'),
    ('ccbupt_teacher', 'notifications', 'read'),
    ('ccbupt_teacher', 'privacy', 'read'),

    -- ===== ccbupt_student：自助 + 应用门户 =====
    ('ccbupt_student', 'sessions', 'read'),
    ('ccbupt_student', 'tokens', 'read'),
    ('ccbupt_student', 'webauthn', 'read'),
    ('ccbupt_student', 'webauthn', 'write'),
    ('ccbupt_student', 'auth', 'step_up'),
    ('ccbupt_student', 'auth', 'trusted_device'),
    ('ccbupt_student', 'applications', 'read'),
    ('ccbupt_student', 'notifications', 'read'),
    ('ccbupt_student', 'privacy', 'read')
) AS v(role, obj, act)
WHERE o.slug = 'ccbupt'
  AND o.deleted_at IS NULL
  AND e.name = 'default'
ON CONFLICT ON CONSTRAINT uq_casbin_rules_rule DO NOTHING;

-- ----------------------------------------------------------------------------
-- 4. 演示账号（ccbupt 当前无用户，绑定需要主体；凭据复用开发哨兵、免强制改密）
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.users (
    id, organization_id, username, user_type, status, is_admin, created_at, updated_at
)
SELECT gen_random_uuid(), o.id, v.username, 'normal', 'active', false, now(), now()
FROM guliangiam.organizations o
CROSS JOIN (VALUES
    ('admin_it'),
    ('zhang_hr'),
    ('li_lab'),
    ('wang_cs'),
    ('fin_zhao'),
    ('prof_liu'),
    ('student_zhao')
) AS v(username)
WHERE o.slug = 'ccbupt'
  AND o.deleted_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO guliangiam.user_profiles (user_id, display_name, created_at, updated_at)
SELECT u.id, v.display_name, now(), now()
FROM guliangiam.users u
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN (VALUES
    ('admin_it',     '信息中心管理员'),
    ('zhang_hr',     '人事专员'),
    ('li_lab',       '实验室管理处'),
    ('wang_cs',      '计算机学院管理员'),
    ('fin_zhao',     '财务审计员'),
    ('prof_liu',     '刘教授'),
    ('student_zhao', '赵同学')
) AS v(username, display_name) ON v.username = u.username
WHERE o.slug = 'ccbupt'
  AND o.deleted_at IS NULL
  AND u.deleted_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO guliangiam.user_credentials AS uc (
    user_id, password_hash, password_algo, password_updated_at, need_update_password
)
SELECT u.id, uc.password_hash, uc.password_algo, now(), false
FROM guliangiam.users u
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.users au
  ON au.username = 'admin'
JOIN guliangiam.organizations ao ON ao.id = au.organization_id
JOIN guliangiam.user_credentials uc ON uc.user_id = au.id
WHERE o.slug = 'ccbupt'
  AND ao.slug = 'built-in'
  AND o.deleted_at IS NULL
  AND u.deleted_at IS NULL
  AND u.username IN ('admin_it', 'zhang_hr', 'li_lab', 'wang_cs', 'fin_zhao', 'prof_liu', 'student_zhao')
ON CONFLICT (user_id) DO UPDATE
SET need_update_password = EXCLUDED.need_update_password,
    updated_at = now()
WHERE uc.need_update_password IS DISTINCT FROM EXCLUDED.need_update_password;

-- ----------------------------------------------------------------------------
-- 5. 角色分配（IAM 绑定表）
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.user_role_bindings (user_id, role_id)
SELECT u.id, r.id
FROM guliangiam.users u
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.roles r ON r.organization_id = o.id
JOIN (VALUES
    ('admin_it',     'ccbupt_sys_admin'),
    ('zhang_hr',     'ccbupt_hr_admin'),
    ('li_lab',       'ccbupt_lab_admin'),
    ('wang_cs',      'ccbupt_dept_admin'),
    ('fin_zhao',     'ccbupt_finance_auditor'),
    ('prof_liu',     'ccbupt_teacher'),
    ('student_zhao', 'ccbupt_student')
) AS b(username, role_name) ON b.username = u.username AND b.role_name = r.name
WHERE o.slug = 'ccbupt'
  AND o.deleted_at IS NULL
  AND u.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- ----------------------------------------------------------------------------
-- 6. Casbin g 行：用户 → 角色
-- ----------------------------------------------------------------------------
INSERT INTO guliangiam.casbin_rules (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5)
SELECT gen_random_uuid(), e.id, 'g', v.username, v.role_name, 'ccbupt', '', '', ''
FROM guliangiam.enforcers e
JOIN guliangiam.organizations o ON o.id = e.organization_id
JOIN guliangiam.users u ON u.organization_id = o.id
JOIN (VALUES
    ('admin_it',     'ccbupt_sys_admin'),
    ('zhang_hr',     'ccbupt_hr_admin'),
    ('li_lab',       'ccbupt_lab_admin'),
    ('wang_cs',      'ccbupt_dept_admin'),
    ('fin_zhao',     'ccbupt_finance_auditor'),
    ('prof_liu',     'ccbupt_teacher'),
    ('student_zhao', 'ccbupt_student')
) AS v(username, role_name) ON v.username = u.username
WHERE o.slug = 'ccbupt'
  AND o.deleted_at IS NULL
  AND u.deleted_at IS NULL
  AND e.name = 'default'
ON CONFLICT ON CONSTRAINT uq_casbin_rules_rule DO NOTHING;

-- ----------------------------------------------------------------------------
-- 7. 完整性核对
-- ----------------------------------------------------------------------------
SELECT 'roles' AS item,
       count(*) AS total,
       count(*) FILTER (WHERE r.name LIKE 'ccbupt\_%') AS prefixed,
       count(*) FILTER (WHERE r.name NOT LIKE 'ccbupt\_%') AS unprefixed
FROM guliangiam.roles r
JOIN guliangiam.organizations o ON o.id = r.organization_id
WHERE o.slug = 'ccbupt';

SELECT cr.ptype,
       count(*) AS total,
       count(*) FILTER (
           WHERE (cr.ptype = 'p' AND cr.v1 <> 'ccbupt')
              OR (cr.ptype = 'g' AND cr.v2 <> 'ccbupt')
       ) AS wrong_domain,
       count(*) FILTER (WHERE cr.ptype = 'p' AND cr.v0 NOT LIKE 'ccbupt\_%') AS unprefixed_subject
FROM guliangiam.casbin_rules cr
JOIN guliangiam.enforcers e ON e.id = cr.enforcer_id
JOIN guliangiam.organizations o ON o.id = e.organization_id
WHERE o.slug = 'ccbupt'
GROUP BY cr.ptype
ORDER BY cr.ptype;

SELECT count(*) AS duplicate_rows
FROM (
    SELECT enforcer_id, ptype, v0, v1, v2, v3
    FROM guliangiam.casbin_rules cr
    JOIN guliangiam.enforcers e ON e.id = cr.enforcer_id
    JOIN guliangiam.organizations o ON o.id = e.organization_id
    WHERE o.slug = 'ccbupt'
    GROUP BY enforcer_id, ptype, v0, v1, v2, v3
    HAVING count(*) > 1
) d;

COMMIT;
