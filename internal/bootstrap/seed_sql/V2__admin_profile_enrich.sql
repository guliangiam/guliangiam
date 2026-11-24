-- Data migration V2 (after seed V1): fill built-in admin profile fields with
-- clearly synthetic demo values so every tab/form has sample data on first
-- login. No real person, company, phone number or identifier is used.
-- Safe to replay (all SETs are COALESCE idempotent).
-- Depends on: data V1 (seed) for the admin user row; runs after all structure.
-- NOTE: user_profiles only has the columns listed below — V6 added 8 new
-- passport/driver_license columns and 2 access_secret transition columns but
-- did NOT add title or addresses.
UPDATE guliangiam.user_profiles AS p
SET
    first_name         = COALESCE(p.first_name, 'Demo'),
    last_name          = COALESCE(p.last_name,  'Admin'),
    phone              = COALESCE(p.phone, '+8613800000000'),
    phone_verified     = TRUE,
    country_code       = COALESCE(p.country_code, 'CN'),
    region             = COALESCE(p.region, '示例省'),
    location           = COALESCE(p.location, '示例市示例区示例路 1 号'),
    birthday           = COALESCE(p.birthday, DATE '1990-01-01'),
    gender             = COALESCE(p.gender, 'male'),
    bio                = COALESCE(p.bio, 'GuliangIAM 内置管理员 — 覆盖所有 Tab 表单的合成演示数据。'),
    real_name          = COALESCE(p.real_name, '示例科技有限公司'),
    id_card_type       = COALESCE(p.id_card_type, 'id_card'),
    id_card            = COALESCE(p.id_card, 'DEMO000000000000'),
    properties         = COALESCE(p.properties, '{"affiliation":"示例科技 / 演示部门","department":"Identity & Access","employeeId":"DEMO-0001","costCenter":"CC-2048","title":"演示架构师"}'::jsonb),
    updated_at         = now()
FROM guliangiam.users AS u
JOIN guliangiam.organizations AS o ON o.id = u.organization_id
WHERE p.user_id = u.id
  AND u.username = 'admin'
  AND o.slug = 'built-in'
  AND u.deleted_at IS NULL
  AND o.deleted_at IS NULL;
