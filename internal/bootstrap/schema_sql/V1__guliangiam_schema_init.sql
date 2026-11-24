-- ============================================================
-- GuliangIAM PostgreSQL Schema (V1)
-- 生成物 — 请勿手工修改；唯一事实数据源为 contracts/schema/*.yaml，
-- 由 scripts/gen_schema.py 生成，CI 通过 check-generated-zero-diff.sh 校验零差异。
-- ============================================================

-- ============================================================
-- 1. 扩展与 Schema
-- ============================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- pg_stat_statements 需 shared_preload_libraries；不可用时静默跳过
DO $$ BEGIN
    CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'pg_stat_statements extension not available, skipping';
END $$;

CREATE SCHEMA IF NOT EXISTS guliangiam;

-- ============================================================
-- 1.5 应用角色与权限（RLS 强制隔离配套）
-- ============================================================

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'guliangiam_app') THEN
        CREATE ROLE guliangiam_app LOGIN;
    END IF;
END $$;
GRANT USAGE ON SCHEMA guliangiam TO guliangiam_app;
GRANT SELECT ON ALL TABLES IN SCHEMA guliangiam TO guliangiam_app;
GRANT INSERT ON ALL TABLES IN SCHEMA guliangiam TO guliangiam_app;
GRANT UPDATE ON ALL TABLES IN SCHEMA guliangiam TO guliangiam_app;
GRANT DELETE ON ALL TABLES IN SCHEMA guliangiam TO guliangiam_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA guliangiam TO guliangiam_app;

-- ============================================================
-- 2. 基础函数（不依赖表结构）
-- ============================================================

CREATE OR REPLACE FUNCTION guliangiam.set_tenant_context(p_organization_id uuid, p_role text) RETURNS void
    LANGUAGE plpgsql VOLATILE AS $$
BEGIN
    PERFORM set_config('app.current_organization_id',
                        COALESCE(p_organization_id::text, ''), true);
    PERFORM set_config('app.current_role', COALESCE(p_role, ''), true);
END;
$$;

CREATE OR REPLACE FUNCTION guliangiam.current_organization_id() RETURNS uuid
    LANGUAGE sql STABLE AS $$
SELECT NULLIF(current_setting('app.current_organization_id', true), '')::uuid;
$$;

CREATE OR REPLACE FUNCTION guliangiam.is_super_admin() RETURNS boolean
    LANGUAGE sql STABLE AS $$
SELECT COALESCE(current_setting('app.current_role', true) = 'super_admin', false);
$$;

-- ============================================================
-- 3. 表结构（拓扑序：被引用表先创建）
-- ============================================================


-- ------------------------------------------------------------
-- 分组：00_platform.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.encryption_keys (
    id UUID NOT NULL PRIMARY KEY,
    key_id TEXT NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    algorithm VARCHAR(64) NOT NULL DEFAULT 'AES-256-GCM',
    purpose VARCHAR(64) NOT NULL DEFAULT 'field-encryption',
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    key_material_ref VARCHAR(255) NOT NULL DEFAULT '',
    key_fingerprint VARCHAR(255) NOT NULL DEFAULT '',
    tenant_scope VARCHAR(64) NOT NULL DEFAULT 'global',
    scope_id VARCHAR(255) NOT NULL DEFAULT '',
    region_code VARCHAR(255) NOT NULL DEFAULT '',
    sovereignty_mode VARCHAR(64) NOT NULL DEFAULT 'platform-default',
    activated_at TIMESTAMPTZ,
    retired_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    rotate_after TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_encryption_keys_purpose_key_version UNIQUE (purpose, key_id, key_version),
    CONSTRAINT ck_encryption_keys_algorithm CHECK (algorithm IN ('AES-256-GCM', 'SM4-GCM')),
    CONSTRAINT ck_encryption_keys_status CHECK (status IN ('pending', 'active', 'retiring', 'retired', 'revoked')),
    CONSTRAINT ck_encryption_keys_key_version CHECK (key_version >= 1),
    CONSTRAINT ck_encryption_keys_tenant_scope CHECK (tenant_scope IN ('global', 'region', 'organization')),
    CONSTRAINT ck_encryption_keys_sovereignty_mode CHECK (sovereignty_mode IN ('platform-default', 'byok', 'hyok'))
);
CREATE INDEX IF NOT EXISTS idx_encryption_keys_purpose_status ON guliangiam.encryption_keys (purpose, status);
CREATE INDEX IF NOT EXISTS idx_encryption_keys_rotate_after ON guliangiam.encryption_keys (rotate_after) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_encryption_keys_tenant_scope ON guliangiam.encryption_keys (tenant_scope, scope_id);

CREATE TABLE IF NOT EXISTS guliangiam.encryption_reencrypt_progress (
    col_key TEXT NOT NULL PRIMARY KEY,
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    last_revision BIGINT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revision BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_encryption_reencrypt_status CHECK (status IN ('pending', 'in_progress', 'completed', 'failed'))
);
COMMENT ON TABLE guliangiam.encryption_reencrypt_progress IS 'Re-encryption job checkpoint table (DB-based alternative to file checkpoint)';

CREATE TABLE IF NOT EXISTS guliangiam.platform_admins (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    created_by UUID,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_platform_admins_user_id UNIQUE (user_id)
);
COMMENT ON TABLE guliangiam.platform_admins IS 'Platform admin registry — cross-tenant operator credentials and scope';
COMMENT ON COLUMN guliangiam.platform_admins.id IS 'Platform admin primary key';
COMMENT ON COLUMN guliangiam.platform_admins.user_id IS 'FK -> users.id (admin user)';
COMMENT ON COLUMN guliangiam.platform_admins.created_by IS 'FK -> users.id (creating admin; nullable for bootstrap)';
COMMENT ON COLUMN guliangiam.platform_admins.is_active IS 'Master on/off switch for this platform admin';
ALTER TABLE guliangiam.platform_admins ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.platform_admins FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'platform_admins'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.platform_admins
            USING (guliangiam.is_super_admin())
            WITH CHECK (guliangiam.is_super_admin());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.platform_admin_scopes (
    id UUID NOT NULL PRIMARY KEY,
    platform_admin_id UUID NOT NULL,
    scope_type VARCHAR(32) NOT NULL,
    target_tenants UUID[] NOT NULL DEFAULT '{}',
    approved_by UUID,
    approved_at TIMESTAMPTZ,
    is_bootstrap BOOLEAN NOT NULL DEFAULT false,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_pas_scope_type CHECK (scope_type IN ('all', 'targeted')),
    CONSTRAINT ck_pas_all_empty_tenants CHECK (scope_type <> 'all' OR cardinality(target_tenants) = 0),
    CONSTRAINT ck_pas_targeted_has_tenants CHECK (scope_type <> 'targeted' OR cardinality(target_tenants) > 0),
    CONSTRAINT ck_pas_expires_within_year CHECK (expires_at <= created_at + INTERVAL '1 year'),
    CONSTRAINT ck_pas_expires_after_created CHECK (expires_at >= created_at),
    CONSTRAINT fk_platform_admin_scopes_platform_admin_id_platform_admins FOREIGN KEY (platform_admin_id) REFERENCES guliangiam.platform_admins(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_pas_platform_admin_id ON guliangiam.platform_admin_scopes (platform_admin_id);
COMMENT ON TABLE guliangiam.platform_admin_scopes IS 'Cross-tenant scopes granted to platform admins (with expiry and approval)';
COMMENT ON COLUMN guliangiam.platform_admin_scopes.id IS 'Scope primary key';
COMMENT ON COLUMN guliangiam.platform_admin_scopes.platform_admin_id IS 'FK -> platform_admins.id';
COMMENT ON COLUMN guliangiam.platform_admin_scopes.scope_type IS 'Scope kind: all | targeted';
COMMENT ON COLUMN guliangiam.platform_admin_scopes.target_tenants IS 'Tenant UUIDs — empty for scope_type=''all''';
COMMENT ON COLUMN guliangiam.platform_admin_scopes.approved_by IS 'FK -> users.id (approver; NULL for bootstrap)';
COMMENT ON COLUMN guliangiam.platform_admin_scopes.approved_at IS 'Approval timestamp (must be set when approved_by is set)';
COMMENT ON COLUMN guliangiam.platform_admin_scopes.is_bootstrap IS 'Bootstrap exemption — seed-created built-in admin ONLY; never backfill other approved_by IS NULL rows';
COMMENT ON COLUMN guliangiam.platform_admin_scopes.expires_at IS 'Scope expiry (max 1 year from creation)';
ALTER TABLE guliangiam.platform_admin_scopes ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.platform_admin_scopes FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'platform_admin_scopes'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.platform_admin_scopes
            USING (guliangiam.is_super_admin())
            WITH CHECK (guliangiam.is_super_admin());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：10_tenant.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.organizations (
    id UUID NOT NULL PRIMARY KEY,
    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    website_url TEXT,
    logo_url TEXT,
    logo_dark_url TEXT,
    favicon_url TEXT,
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    region_code VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,
    revision BIGINT NOT NULL DEFAULT 1,
    init_score INTEGER NOT NULL DEFAULT 0,
    default_avatar TEXT,
    disable_signin BOOLEAN NOT NULL DEFAULT FALSE,
    enable_soft_deletion BOOLEAN NOT NULL DEFAULT FALSE,
    ip_whitelist VARCHAR(255) NOT NULL DEFAULT '',
    initiate_login_url VARCHAR(255) NOT NULL DEFAULT '',
    disable_signup BOOLEAN NOT NULL DEFAULT FALSE,
    parent_id UUID,
    CONSTRAINT fk_organizations_parent_id_organizations FOREIGN KEY (parent_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_organizations_slug_active ON guliangiam.organizations (slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_organizations_created_at_desc ON guliangiam.organizations (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orgs_slug_deleted ON guliangiam.organizations (slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_organizations_parent_id ON guliangiam.organizations (parent_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN guliangiam.organizations.parent_id IS 'Parent organization in the tenant hierarchy; NULL = root. Tree does not change data-plane isolation (ADR-028).';
ALTER TABLE guliangiam.organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organizations FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organizations'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organizations
            USING (guliangiam.is_super_admin() OR id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.organization_auth_config (
    organization_id UUID NOT NULL PRIMARY KEY,
    password_type VARCHAR(64) NOT NULL DEFAULT 'argon2id',
    password_salt VARCHAR(255) NOT NULL DEFAULT '',
    password_obfuscator_type TEXT,
    password_obfuscator_key VARCHAR(255) NOT NULL DEFAULT '',
    password_options VARCHAR(255)[] NOT NULL DEFAULT '{}',
    master_password VARCHAR(255) NOT NULL DEFAULT '',
    master_verification_code VARCHAR(255) NOT NULL DEFAULT '',
    default_password VARCHAR(255) NOT NULL DEFAULT '',
    use_email_as_username BOOLEAN NOT NULL DEFAULT FALSE,
    ldap_attributes VARCHAR(255)[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_organization_auth_config_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.organization_auth_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organization_auth_config FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organization_auth_config'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organization_auth_config
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.organization_kerberos_config (
    organization_id UUID NOT NULL PRIMARY KEY,
    kerberos_realm VARCHAR(255) NOT NULL DEFAULT '',
    kerberos_kdc_host VARCHAR(255) NOT NULL DEFAULT '',
    kerberos_keytab VARCHAR(255) NOT NULL DEFAULT '',
    kerberos_service_name VARCHAR(64) NOT NULL DEFAULT 'HTTP',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_organization_kerberos_config_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.organization_kerberos_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organization_kerberos_config FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organization_kerberos_config'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organization_kerberos_config
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.organization_billing (
    organization_id UUID NOT NULL PRIMARY KEY,
    balance DECIMAL(19, 4) NOT NULL DEFAULT 0,
    balance_credit DECIMAL(19, 4) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    default_currency VARCHAR(3) NOT NULL DEFAULT 'CNY',
    enable_deposit BOOLEAN NOT NULL DEFAULT FALSE,
    mu INTEGER NOT NULL DEFAULT 0,
    default_quota INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_organization_billing_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.organization_billing ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organization_billing FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organization_billing'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organization_billing
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.organization_branding (
    organization_id UUID NOT NULL PRIMARY KEY,
    enable_dark_logo BOOLEAN NOT NULL DEFAULT FALSE,
    has_privilege_consent BOOLEAN NOT NULL DEFAULT FALSE,
    is_profile_public BOOLEAN NOT NULL DEFAULT FALSE,
    use_permanent_avatar BOOLEAN NOT NULL DEFAULT FALSE,
    enable_tour BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_organization_branding_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.organization_branding ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organization_branding FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organization_branding'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organization_branding
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.organization_settings (
    organization_id UUID NOT NULL PRIMARY KEY,
    languages JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_application_id TEXT,
    account_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    nav_preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
    feature_flags JSONB NOT NULL DEFAULT '{}'::jsonb,
    country_codes JSONB NOT NULL DEFAULT '[]'::jsonb,
    user_types JSONB NOT NULL DEFAULT '[]'::jsonb,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    mfa_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    account_menu VARCHAR(64) NOT NULL DEFAULT 'horizontal',
    mfa_remember_in_hours INTEGER NOT NULL DEFAULT 0,
    dcr_policy VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_organization_settings_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_orgs_account_items_gin ON guliangiam.organization_settings USING gin (account_items JSONB_PATH_OPS) WHERE account_items IS NOT NULL;
ALTER TABLE guliangiam.organization_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organization_settings FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organization_settings'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organization_settings
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.organization_theme_settings (
    organization_id UUID NOT NULL PRIMARY KEY,
    theme_name VARCHAR(255) NOT NULL DEFAULT '',
    tokens JSONB NOT NULL DEFAULT '{}'::jsonb,
    custom_css VARCHAR(255) NOT NULL DEFAULT '',
    assets JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_organization_theme_settings_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.organization_theme_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organization_theme_settings FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organization_theme_settings'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organization_theme_settings
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.organization_security_policies (
    organization_id UUID NOT NULL PRIMARY KEY,
    password_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    password_derive_algorithm VARCHAR(16) NOT NULL DEFAULT 'hmac_sha256',
    mfa_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    session_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    risk_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    lockout_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    concurrent_session_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_organization_security_policies_password_derive_algorithm CHECK (password_derive_algorithm IN ('hmac_sha256', 'hmac_sm3')),
    CONSTRAINT fk_organization_security_policies_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_org_security_password_policy ON guliangiam.organization_security_policies USING gin (password_policy) WHERE password_policy != '{}'::jsonb;
CREATE INDEX IF NOT EXISTS idx_orgs_password_policy_gin ON guliangiam.organization_security_policies USING gin (password_policy JSONB_PATH_OPS) WHERE password_policy IS NOT NULL;
COMMENT ON COLUMN guliangiam.organization_security_policies.password_derive_algorithm IS '密码派生摘要算法；存储哈希固定 argon2id，不提供国密降级选项';
ALTER TABLE guliangiam.organization_security_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organization_security_policies FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organization_security_policies'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organization_security_policies
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.outbox (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID,
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    event_version INTEGER NOT NULL DEFAULT 1,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 12,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_by TEXT,
    locked_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_outbox_status CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead')),
    CONSTRAINT ck_outbox_attempts CHECK (attempts >= 0 AND attempts <= max_attempts),
    CONSTRAINT fk_outbox_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_outbox_pending_available_at ON guliangiam.outbox (available_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_outbox_status_created_at ON guliangiam.outbox (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON guliangiam.outbox (aggregate_type, aggregate_id);
CREATE INDEX IF NOT EXISTS idx_outbox_organization_id_created_at ON guliangiam.outbox (organization_id, created_at DESC);
COMMENT ON COLUMN guliangiam.outbox.id IS 'Outbox event primary key (UUID)';
COMMENT ON COLUMN guliangiam.outbox.aggregate_type IS 'Source aggregate type (users / orders / sessions / ...)';
COMMENT ON COLUMN guliangiam.outbox.aggregate_id IS 'Source aggregate UUID';
COMMENT ON COLUMN guliangiam.outbox.event_type IS 'Domain event type name (e.g. user.created, order.paid)';
COMMENT ON COLUMN guliangiam.outbox.payload IS 'Full event body (structured document); consumed by subscribers without needing to re-read the aggregate';
COMMENT ON COLUMN guliangiam.outbox.status IS 'Dispatch lifecycle: pending / dispatching / delivered / failed / dead_lettered';
COMMENT ON COLUMN guliangiam.outbox.last_error IS 'Last dispatch failure message, truncated';
ALTER TABLE guliangiam.outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.outbox FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'outbox'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.outbox
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.outbox_dead_letters (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    headers JSONB,
    status VARCHAR(64) NOT NULL DEFAULT 'dead',
    attempts INTEGER NOT NULL DEFAULT 12,
    last_error TEXT,
    dead_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    replayed_at TIMESTAMPTZ,
    replayed_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_outbox_dead_letters_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_outbox_dead_letters_status_created ON guliangiam.outbox_dead_letters (status, dead_at DESC);
CREATE INDEX IF NOT EXISTS idx_outbox_dead_letters_event_id ON guliangiam.outbox_dead_letters (event_id);
COMMENT ON COLUMN guliangiam.outbox_dead_letters.organization_id IS 'Tenant owner copied from outbox; NULL rows are super-admin only';
ALTER TABLE guliangiam.outbox_dead_letters ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.outbox_dead_letters FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'outbox_dead_letters'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.outbox_dead_letters
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.idempotency_keys (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID,
    scope TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_method TEXT NOT NULL,
    request_path TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'in_progress',
    response_status INTEGER,
    response_body JSONB,
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '24 hours',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_idempotency_keys_scope_key UNIQUE (scope, idempotency_key),
    CONSTRAINT ck_idempotency_keys_status CHECK (status IN ('in_progress', 'succeeded', 'failed')),
    CONSTRAINT fk_idempotency_keys_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at ON guliangiam.idempotency_keys (expires_at);
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_organization_id ON guliangiam.idempotency_keys (organization_id);
ALTER TABLE guliangiam.idempotency_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.idempotency_keys FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'idempotency_keys'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.idempotency_keys
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.deployment_regions (
    id UUID NOT NULL PRIMARY KEY,
    region_code TEXT NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    country_code VARCHAR(255) NOT NULL DEFAULT '',
    timezone VARCHAR(255) NOT NULL DEFAULT '',
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    rto_minutes INTEGER NOT NULL DEFAULT 0,
    rpo_minutes INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_deployment_regions_region_code UNIQUE (region_code),
    CONSTRAINT ck_deployment_regions_country_code CHECK (char_length(country_code) = 2),
    CONSTRAINT ck_deployment_regions_rto_minutes CHECK (rto_minutes >= 0),
    CONSTRAINT ck_deployment_regions_rpo_minutes CHECK (rpo_minutes >= 0)
);

CREATE TABLE IF NOT EXISTS guliangiam.tenant_instances (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    region_id UUID NOT NULL,
    region_code VARCHAR(255) NOT NULL DEFAULT '',
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    placement_kind VARCHAR(64) NOT NULL DEFAULT 'shared_cluster',
    storage_boundary VARCHAR(64) NOT NULL DEFAULT 'shared',
    backup_boundary VARCHAR(64) NOT NULL DEFAULT 'shared',
    data_residency_country VARCHAR(2) NOT NULL DEFAULT '',
    capacity_quota_units BIGINT NOT NULL DEFAULT 0,
    provisioned_at TIMESTAMPTZ,
    effective_from TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_tenant_instances_org_region UNIQUE (organization_id, region_id),
    CONSTRAINT ck_tenant_instances_status CHECK (status IN ('pending', 'active', 'failed', 'decommissioned')),
    CONSTRAINT ck_tenant_instances_country CHECK (data_residency_country = '' OR char_length(data_residency_country) = 2),
    CONSTRAINT fk_tenant_instances_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_tenant_instances_region_id_deployment_regions FOREIGN KEY (region_id) REFERENCES guliangiam.deployment_regions(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_instances_primary ON guliangiam.tenant_instances (organization_id) WHERE is_primary AND status <> 'decommissioned';
CREATE INDEX IF NOT EXISTS idx_tenant_instances_region_id ON guliangiam.tenant_instances (region_id);
COMMENT ON COLUMN guliangiam.tenant_instances.status IS 'Topology lifecycle: pending / active / failed / decommissioned';
COMMENT ON COLUMN guliangiam.tenant_instances.placement_kind IS 'Placement kind: shared_cluster today; dedicated_cluster requires routing work';
COMMENT ON COLUMN guliangiam.tenant_instances.storage_boundary IS 'Storage boundary: shared / tenant (metadata only, never an RLS input)';
COMMENT ON COLUMN guliangiam.tenant_instances.backup_boundary IS 'Backup boundary: shared / tenant';
COMMENT ON COLUMN guliangiam.tenant_instances.data_residency_country IS 'ISO 3166-1 alpha-2; defaults from deployment_regions.country_code';
COMMENT ON COLUMN guliangiam.tenant_instances.capacity_quota_units IS 'Capacity quota units; 0 means unlimited';
ALTER TABLE guliangiam.tenant_instances ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.tenant_instances FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'tenant_instances'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.tenant_instances
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.tenant_identities (
    organization_id UUID NOT NULL PRIMARY KEY,
    status VARCHAR(64) NOT NULL DEFAULT 'provisioning',
    isolation_tier VARCHAR(64) NOT NULL DEFAULT 'row',
    is_built_in BOOLEAN NOT NULL DEFAULT FALSE,
    provisioned_at TIMESTAMPTZ,
    decommissioned_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_tenant_identities_status CHECK (status IN ('provisioning', 'active', 'failed', 'decommissioned')),
    CONSTRAINT ck_tenant_identities_isolation_tier CHECK (isolation_tier IN ('row')),
    CONSTRAINT fk_tenant_identities_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_tenant_identities_status ON guliangiam.tenant_identities (status);
COMMENT ON COLUMN guliangiam.tenant_identities.organization_id IS 'Tenant identity primary key; equals organizations.id (single tenant_id source)';
COMMENT ON COLUMN guliangiam.tenant_identities.status IS 'Identity lifecycle: provisioning / active / failed / decommissioned';
COMMENT ON COLUMN guliangiam.tenant_identities.isolation_tier IS 'Isolation tier: row only today; schema/database require a new ADR';
COMMENT ON COLUMN guliangiam.tenant_identities.is_built_in IS 'Built-in organization identity cannot be decommissioned';
COMMENT ON COLUMN guliangiam.tenant_identities.last_error IS 'Sanitized provisioning failure reason; never contains credentials';
ALTER TABLE guliangiam.tenant_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.tenant_identities FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'tenant_identities'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.tenant_identities
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.organization_domains (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    domain VARCHAR(253) NOT NULL,
    verification_token VARCHAR(255) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts INT NOT NULL DEFAULT 0,
    last_error VARCHAR(255),
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_checked_at TIMESTAMPTZ,
    CONSTRAINT uq_org_domain UNIQUE (organization_id, domain),
    CONSTRAINT ck_organization_domains_status CHECK (status IN ('pending', 'verified', 'failed')),
    CONSTRAINT fk_organization_domains_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_organization_domains_domain ON guliangiam.organization_domains (domain, status);
CREATE INDEX IF NOT EXISTS idx_organization_domains_status ON guliangiam.organization_domains (status, created_at);
CREATE INDEX IF NOT EXISTS idx_organization_domains_retry ON guliangiam.organization_domains (status, next_retry_at);
COMMENT ON COLUMN guliangiam.organization_domains.attempts IS 'Number of failed DNS TXT verification attempts while pending';
COMMENT ON COLUMN guliangiam.organization_domains.last_error IS 'Sanitized failure classification code + short reason, truncated to 255 chars';
COMMENT ON COLUMN guliangiam.organization_domains.next_retry_at IS 'Earliest time this pending row may be claimed again; frozen once terminal';
COMMENT ON COLUMN guliangiam.organization_domains.last_checked_at IS 'Last DNS TXT query time';
ALTER TABLE guliangiam.organization_domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organization_domains FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organization_domains'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organization_domains
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.organization_policy_inheritance (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    policy_key VARCHAR(128) NOT NULL,
    mode VARCHAR(16) NOT NULL,
    source_organization_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_organization_policy_inheritance_key UNIQUE (organization_id, policy_key),
    CONSTRAINT ck_organization_policy_inheritance_mode CHECK (mode IN ('inherit', 'override', 'lock')),
    CONSTRAINT fk_organization_policy_inheritance_organization_id_org_5d6d705e FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_organization_policy_inheritance_source_organization_698f8308 FOREIGN KEY (source_organization_id) REFERENCES guliangiam.organizations(id) ON DELETE SET NULL
);
ALTER TABLE guliangiam.organization_policy_inheritance ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.organization_policy_inheritance FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'organization_policy_inheritance'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.organization_policy_inheritance
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：20_identity.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.users (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    username TEXT NOT NULL,
    external_id TEXT,
    user_type VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    is_admin BOOLEAN NOT NULL DEFAULT false,
    is_forbidden BOOLEAN NOT NULL DEFAULT false,
    is_verified BOOLEAN NOT NULL DEFAULT false,
    is_online BOOLEAN NOT NULL DEFAULT false,
    created_ip VARCHAR(50) NOT NULL DEFAULT '',
    signup_application_id UUID,
    register_source TEXT,
    register_type VARCHAR(255) NOT NULL DEFAULT '',
    tag VARCHAR(255) NOT NULL DEFAULT '',
    score INTEGER NOT NULL DEFAULT 0,
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    ip_whitelist VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_users_status CHECK (status IN ('active', 'disabled', 'locked', 'pending_verification')),
    CONSTRAINT fk_users_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_org_username_active ON guliangiam.users (organization_id, username) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_org_created_at_desc ON guliangiam.users (organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_users_organization_id ON guliangiam.users (organization_id);
CREATE INDEX IF NOT EXISTS idx_users_org_deleted ON guliangiam.users (organization_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_users_org_username_deleted ON guliangiam.users (organization_id, username) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_status ON guliangiam.users (status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_org_status ON guliangiam.users (organization_id, status) WHERE deleted_at IS NULL;
COMMENT ON COLUMN guliangiam.users.id IS 'User primary key (UUID)';
COMMENT ON COLUMN guliangiam.users.organization_id IS 'Tenant org this user belongs to (RLS predicate key)';
COMMENT ON COLUMN guliangiam.users.username IS 'User login name, unique within organization';
COMMENT ON COLUMN guliangiam.users.external_id IS 'IdP subject identifier when account was created via SSO; NULL for local users';
COMMENT ON COLUMN guliangiam.users.user_type IS 'User classification: human / service_account / bot';
COMMENT ON COLUMN guliangiam.users.status IS 'Account lifecycle state: active / disabled / locked / pending_verification';
COMMENT ON COLUMN guliangiam.users.is_admin IS 'Org-level admin flag; when true the user bypasses role-based permission checks';
COMMENT ON COLUMN guliangiam.users.is_forbidden IS 'Emergency block flag; when true all auth flows reject this user';
COMMENT ON COLUMN guliangiam.users.is_verified IS 'Whether email/phone verification has completed';
COMMENT ON COLUMN guliangiam.users.is_online IS 'Cached session presence flag updated by periodic heartbeat';
COMMENT ON COLUMN guliangiam.users.created_ip IS 'First-seen creation IP address (varchar to tolerate IPv6 variants)';
COMMENT ON COLUMN guliangiam.users.signup_application_id IS 'Application through which the user registered; NULL for admin-created accounts';
COMMENT ON COLUMN guliangiam.users.register_source IS 'Creation channel: self_signup | admin_create | invitation | SCIM | OpenID | SAML';
COMMENT ON COLUMN guliangiam.users.register_type IS 'Auth method used at signup: password | oidc | saml | webauthn | magic_link';
COMMENT ON COLUMN guliangiam.users.tag IS 'Free-form admin label (no schema constraint; use for internal cohorting)';
COMMENT ON COLUMN guliangiam.users.score IS 'Composite risk / engagement score used by admin UX sorting';
COMMENT ON COLUMN guliangiam.users.is_deleted IS 'Soft-delete marker; rows with true are hidden from normal queries';
COMMENT ON COLUMN guliangiam.users.ip_whitelist IS 'Semicolon-separated CIDR list; when non-empty, login restricted to these ranges';
COMMENT ON COLUMN guliangiam.users.created_by IS 'User UUID that created this account (admin self-serve flow)';
COMMENT ON COLUMN guliangiam.users.updated_by IS 'User UUID that last modified this account';
ALTER TABLE guliangiam.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.users FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'users'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.users
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_signin_tracking (
    user_id UUID NOT NULL PRIMARY KEY,
    signin_wrong_times INTEGER NOT NULL DEFAULT 0,
    last_signin_wrong_time TIMESTAMPTZ,
    last_signin_at TIMESTAMPTZ,
    last_signin_ip INET,
    signup_ip INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_user_signin_tracking_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.user_signin_tracking ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_signin_tracking FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_signin_tracking'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_signin_tracking
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_mfa_config (
    user_id UUID NOT NULL PRIMARY KEY,
    preferred_mfa_type TEXT,
    recovery_code_hashes TEXT[] NOT NULL DEFAULT '{}',
    recovery_generated_at TIMESTAMPTZ,
    totp_secret TEXT,
    mfa_radius_provider VARCHAR(255) NOT NULL DEFAULT '',
    mfa_radius_username VARCHAR(255) NOT NULL DEFAULT '',
    mfa_push_receiver VARCHAR(255) NOT NULL DEFAULT '',
    mfa_push_provider VARCHAR(255) NOT NULL DEFAULT '',
    mfa_push_enabled BOOLEAN NOT NULL DEFAULT false,
    mfa_remember_deadline TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_user_mfa_config_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
COMMENT ON COLUMN guliangiam.user_mfa_config.recovery_code_hashes IS 'Argon2id hashes of one-time recovery codes; plaintext never stored';
COMMENT ON COLUMN guliangiam.user_mfa_config.recovery_generated_at IS 'When the current recovery-code set was generated';
ALTER TABLE guliangiam.user_mfa_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_mfa_config FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_mfa_config'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_mfa_config
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_financials (
    user_id UUID NOT NULL PRIMARY KEY,
    balance DECIMAL(19, 4) NOT NULL DEFAULT 0,
    balance_credit DECIMAL(19, 4) NOT NULL DEFAULT 0,
    balance_currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_user_financials_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.user_financials ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_financials FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_financials'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_financials
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_custom_attributes (
    user_id UUID NOT NULL PRIMARY KEY,
    custom1 VARCHAR(255) NOT NULL DEFAULT '',
    custom2 VARCHAR(255) NOT NULL DEFAULT '',
    custom3 VARCHAR(255) NOT NULL DEFAULT '',
    custom4 VARCHAR(255) NOT NULL DEFAULT '',
    custom5 VARCHAR(255) NOT NULL DEFAULT '',
    custom6 VARCHAR(255) NOT NULL DEFAULT '',
    custom7 VARCHAR(255) NOT NULL DEFAULT '',
    custom8 VARCHAR(255) NOT NULL DEFAULT '',
    custom9 VARCHAR(255) NOT NULL DEFAULT '',
    custom10 VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_user_custom_attributes_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.user_custom_attributes ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_custom_attributes FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_custom_attributes'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_custom_attributes
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_collections (
    user_id UUID NOT NULL PRIMARY KEY,
    addresses JSONB NOT NULL DEFAULT '[]'::jsonb,
    managed_accounts JSONB NOT NULL DEFAULT '[]'::jsonb,
    face_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    cart JSONB NOT NULL DEFAULT '[]'::jsonb,
    mfa_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    mfa_accounts JSONB NOT NULL DEFAULT '[]'::jsonb,
    managed_accounts_sealed TEXT,
    mfa_items_sealed TEXT,
    mfa_accounts_sealed TEXT,
    application_scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_user_collections_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_collections_addresses_gin ON guliangiam.user_collections USING gin (addresses JSONB_PATH_OPS);
CREATE INDEX IF NOT EXISTS idx_user_collections_face_ids_gin ON guliangiam.user_collections USING gin (face_ids JSONB_PATH_OPS);
COMMENT ON COLUMN guliangiam.user_collections.managed_accounts_sealed IS 'v2 envelope of the managed_accounts plaintext payload (AAD table/column/user_id); NULL = not yet sealed, legacy column remains authoritative';
COMMENT ON COLUMN guliangiam.user_collections.mfa_items_sealed IS 'v2 envelope of the mfa_items plaintext payload (AAD table/column/user_id); NULL = not yet sealed';
COMMENT ON COLUMN guliangiam.user_collections.mfa_accounts_sealed IS 'v2 envelope of the mfa_accounts plaintext payload (AAD table/column/user_id); NULL = not yet sealed';
ALTER TABLE guliangiam.user_collections ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_collections FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_collections'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_collections
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_profiles (
    user_id UUID NOT NULL PRIMARY KEY,
    display_name TEXT,
    first_name TEXT,
    last_name TEXT,
    avatar_url TEXT,
    email TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    phone TEXT,
    phone_verified BOOLEAN NOT NULL DEFAULT false,
    country_code TEXT,
    region TEXT,
    location TEXT,
    birthday DATE,
    gender TEXT,
    bio TEXT,
    real_name TEXT,
    id_card_type TEXT,
    id_card TEXT,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_user_profiles_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_profiles_email_active ON guliangiam.user_profiles (email) WHERE deleted_at IS NULL AND email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_email_deleted ON guliangiam.user_profiles (email) WHERE deleted_at IS NULL AND email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_phone_deleted ON guliangiam.user_profiles (phone) WHERE deleted_at IS NULL AND phone IS NOT NULL;
ALTER TABLE guliangiam.user_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_profiles FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_profiles'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_profiles
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_credentials (
    user_id UUID NOT NULL PRIMARY KEY,
    password_hash VARCHAR(255) NOT NULL DEFAULT '',
    password_algo VARCHAR(255) NOT NULL DEFAULT '',
    password_updated_at TIMESTAMPTZ,
    access_key VARCHAR(255) NOT NULL DEFAULT '',
    access_secret_hash VARCHAR(255) NOT NULL DEFAULT '',
    access_secret_rotated_at TIMESTAMPTZ,
    need_update_password BOOLEAN NOT NULL DEFAULT false,
    revision BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_user_credentials_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.user_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_credentials FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_credentials'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_credentials
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_password_history (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    password_hash TEXT NOT NULL,
    password_algo VARCHAR(64) NOT NULL DEFAULT 'argon2id',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_user_password_history_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_password_history_user_created ON guliangiam.user_password_history (user_id, created_at DESC);
ALTER TABLE guliangiam.user_password_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_password_history FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_password_history'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_password_history
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_identities (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    provider_id VARCHAR(255) NOT NULL DEFAULT '',
    identity_type VARCHAR(255) NOT NULL DEFAULT '',
    subject VARCHAR(255) NOT NULL DEFAULT '',
    provider_account VARCHAR(255) NOT NULL DEFAULT '',
    profile_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    linked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    link_state VARCHAR(64) NOT NULL DEFAULT 'auto_linked',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_identity_provider_subject UNIQUE (identity_type, provider_id, subject),
    CONSTRAINT ck_user_identities_link_state CHECK (link_state IN ('auto_linked', 'merge_pending', 'linked', 'rejected')),
    CONSTRAINT fk_user_identities_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_identities_subject ON guliangiam.user_identities (subject);
CREATE INDEX IF NOT EXISTS idx_user_identities_user_id ON guliangiam.user_identities (user_id, linked_at DESC);
COMMENT ON COLUMN guliangiam.user_identities.link_state IS 'Identity merge lifecycle state declared by the state machine registry (user_identities_merge) and by the generated domain constants.';
ALTER TABLE guliangiam.user_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_identities FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_identities'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_identities
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_mfa_methods (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    method_type VARCHAR(255) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT false,
    verified BOOLEAN NOT NULL DEFAULT false,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_user_mfa_methods_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_mfa_methods_user_id ON guliangiam.user_mfa_methods (user_id);
ALTER TABLE guliangiam.user_mfa_methods ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_mfa_methods FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_mfa_methods'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_mfa_methods
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.applications (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    category VARCHAR(255) NOT NULL DEFAULT '',
    app_type VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT,
    homepage_url TEXT,
    domain TEXT,
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    client_id TEXT,
    tags VARCHAR(255)[] NOT NULL DEFAULT '{}',
    logo VARCHAR(255) NOT NULL DEFAULT '',
    is_portal BOOLEAN NOT NULL DEFAULT FALSE,
    is_visible BOOLEAN NOT NULL DEFAULT TRUE,
    is_shared BOOLEAN NOT NULL DEFAULT FALSE,
    title VARCHAR(255) NOT NULL DEFAULT '',
    favicon VARCHAR(255) NOT NULL DEFAULT '',
    menu_order INTEGER NOT NULL DEFAULT 0,
    menu_mode VARCHAR(64) NOT NULL DEFAULT 'side',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_applications_org_slug UNIQUE (organization_id, slug),
    CONSTRAINT fk_applications_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_applications_client_id_non_empty ON guliangiam.applications (client_id) WHERE client_id <> '';
CREATE INDEX IF NOT EXISTS idx_apps_org_deleted ON guliangiam.applications (organization_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_applications_organization_id_active ON guliangiam.applications (organization_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_applications_client_id_active ON guliangiam.applications (client_id) WHERE client_id IS NOT NULL AND deleted_at IS NULL;
ALTER TABLE guliangiam.applications ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.applications FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'applications'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.applications
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.app_network_config (
    application_id UUID NOT NULL PRIMARY KEY,
    provider_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    forced_redirect_origin VARCHAR(255) NOT NULL DEFAULT '',
    other_domains VARCHAR(255)[] NOT NULL DEFAULT '{}',
    upstream_host VARCHAR(255) NOT NULL DEFAULT '',
    ssl_mode VARCHAR(255) NOT NULL DEFAULT '',
    ssl_cert VARCHAR(255) NOT NULL DEFAULT '',
    client_cert VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_app_network_config_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.app_network_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.app_network_config FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'app_network_config'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.app_network_config
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.app_oauth_config (
    application_id UUID NOT NULL PRIMARY KEY,
    redirect_uris VARCHAR(255)[] NOT NULL DEFAULT '{}',
    grant_types VARCHAR(255)[] NOT NULL DEFAULT '{}',
    token_endpoint_auth_method VARCHAR(255) NOT NULL DEFAULT '',
    client_secret_hash TEXT,
    registration_access_token_hash TEXT,
    custom_scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    token_format VARCHAR(64) NOT NULL DEFAULT 'JWT',
    expire_in_hours INTEGER NOT NULL DEFAULT 24,
    refresh_token_expire_in_hours INTEGER NOT NULL DEFAULT 168,
    token_signing_method VARCHAR(64) NOT NULL DEFAULT 'RS256',
    token_fields JSONB NOT NULL DEFAULT '[]'::jsonb,
    token_attributes JSONB NOT NULL DEFAULT '[]'::jsonb,
    dpop_required BOOLEAN NOT NULL DEFAULT FALSE,
    pkce_required BOOLEAN NOT NULL DEFAULT FALSE,
    cookie_expire_in_hours INTEGER NOT NULL DEFAULT 168,
    enable_beta_oidc BOOLEAN NOT NULL DEFAULT FALSE,
    consent_mode VARCHAR(255) NOT NULL DEFAULT '',
    backchannel_logout_uri VARCHAR(255) NOT NULL DEFAULT '',
    frontchannel_logout_uri VARCHAR(255) NOT NULL DEFAULT '',
    frontchannel_logout_session_required BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_app_oauth_config_token_signing_method CHECK (token_signing_method IN ('RS256', 'SM2-SM3')),
    CONSTRAINT ck_app_oauth_config_token_endpoint_auth_method CHECK (token_endpoint_auth_method IN ('', 'none', 'client_secret_basic', 'client_secret_post', 'private_key_jwt', 'tls_client_auth', 'self_signed_tls_client_auth')),
    CONSTRAINT fk_app_oauth_config_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_app_oauth_config_grant_types ON guliangiam.app_oauth_config USING gin (grant_types) WHERE grant_types != '{}';
ALTER TABLE guliangiam.app_oauth_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.app_oauth_config FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'app_oauth_config'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.app_oauth_config
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.app_client_secrets (
    id UUID NOT NULL PRIMARY KEY,
    application_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    secret_prefix VARCHAR(16) NOT NULL,
    secret_hash VARCHAR(255) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    rotated_at TIMESTAMPTZ,
    grace_until TIMESTAMPTZ,
    retired_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_app_client_secrets_status CHECK (status IN ('active', 'grace', 'retired')),
    CONSTRAINT fk_app_client_secrets_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_app_client_secrets_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_app_client_secrets_active ON guliangiam.app_client_secrets (application_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_app_client_secrets_application_status ON guliangiam.app_client_secrets (application_id, status);
CREATE INDEX IF NOT EXISTS idx_app_client_secrets_org_application_status ON guliangiam.app_client_secrets (organization_id, application_id, status);
COMMENT ON TABLE guliangiam.app_client_secrets IS 'Client secret lifecycle rows; secret_hash is the only stored credential material (client_secret_basic / client_secret_post)';
COMMENT ON COLUMN guliangiam.app_client_secrets.id IS 'Secret row identifier (UUIDv7, generated by the application layer)';
COMMENT ON COLUMN guliangiam.app_client_secrets.application_id IS 'Owning OAuth client application';
COMMENT ON COLUMN guliangiam.app_client_secrets.organization_id IS 'Tenant owner (denormalised from applications.organization_id) so every repository query can filter explicitly without relying on RLS';
COMMENT ON COLUMN guliangiam.app_client_secrets.secret_prefix IS 'Non-secret display prefix (gl_cs_ + first six random characters)';
COMMENT ON COLUMN guliangiam.app_client_secrets.secret_hash IS 'Argon2id hash used by client_secret_basic and client_secret_post';
COMMENT ON COLUMN guliangiam.app_client_secrets.status IS 'Lifecycle state: active / grace / retired';
COMMENT ON COLUMN guliangiam.app_client_secrets.rotated_at IS 'Time this secret was superseded by a rotation';
COMMENT ON COLUMN guliangiam.app_client_secrets.grace_until IS 'End of the acceptance window for a rotated secret';
COMMENT ON COLUMN guliangiam.app_client_secrets.retired_at IS 'Time the secret was permanently retired';
ALTER TABLE guliangiam.app_client_secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.app_client_secrets FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'app_client_secrets'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.app_client_secrets
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.app_client_jwks (
    id UUID NOT NULL PRIMARY KEY,
    application_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    kid VARCHAR(128) NOT NULL,
    alg VARCHAR(16) NOT NULL DEFAULT 'RS256',
    jwk JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    retired_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_app_client_jwks_alg CHECK (alg IN ('RS256', 'PS256', 'ES256')),
    CONSTRAINT ck_app_client_jwks_status CHECK (status IN ('active', 'retired')),
    CONSTRAINT fk_app_client_jwks_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_app_client_jwks_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_app_client_jwks_application_kid ON guliangiam.app_client_jwks (application_id, kid);
CREATE INDEX IF NOT EXISTS idx_app_client_jwks_org_application_status ON guliangiam.app_client_jwks (organization_id, application_id, status);
COMMENT ON TABLE guliangiam.app_client_jwks IS 'Public keys registered by a client for private_key_jwt assertions; private key parameters are rejected before persistence';
COMMENT ON COLUMN guliangiam.app_client_jwks.id IS 'JWK row identifier (UUIDv7, generated by the application layer)';
COMMENT ON COLUMN guliangiam.app_client_jwks.application_id IS 'Owning OAuth client application';
COMMENT ON COLUMN guliangiam.app_client_jwks.organization_id IS 'Tenant owner (denormalised from applications.organization_id) so every repository query can filter explicitly without relying on RLS';
COMMENT ON COLUMN guliangiam.app_client_jwks.kid IS 'Key identifier the client places in the assertion JWT header';
COMMENT ON COLUMN guliangiam.app_client_jwks.alg IS 'Signature algorithm allowed for this key: RS256 / PS256 / ES256';
COMMENT ON COLUMN guliangiam.app_client_jwks.jwk IS 'Public JWK document only (no d/p/q/dp/dq/qi/k members)';
COMMENT ON COLUMN guliangiam.app_client_jwks.status IS 'Lifecycle state: active / retired';
COMMENT ON COLUMN guliangiam.app_client_jwks.retired_at IS 'Time the key was retired';
ALTER TABLE guliangiam.app_client_jwks ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.app_client_jwks FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'app_client_jwks'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.app_client_jwks
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.app_client_certificates (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    application_id UUID NOT NULL,
    auth_method VARCHAR(32) NOT NULL,
    certificate_pem TEXT NOT NULL,
    fingerprint_sha256 VARCHAR(43) NOT NULL,
    subject_dn VARCHAR(512) NOT NULL,
    issuer_dn VARCHAR(512) NOT NULL,
    not_before TIMESTAMPTZ,
    not_after TIMESTAMPTZ,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    retired_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_app_client_certificates_auth_method CHECK (auth_method IN ('tls_client_auth', 'self_signed_tls_client_auth')),
    CONSTRAINT ck_app_client_certificates_status CHECK (status IN ('active', 'retired')),
    CONSTRAINT fk_app_client_certificates_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_app_client_certificates_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_app_client_certificates_org_fingerprint ON guliangiam.app_client_certificates (organization_id, fingerprint_sha256);
CREATE INDEX IF NOT EXISTS idx_app_client_certificates_application_status ON guliangiam.app_client_certificates (application_id, status);
COMMENT ON TABLE guliangiam.app_client_certificates IS 'Client certificates registered for tls_client_auth and self_signed_tls_client_auth; only public certificate PEM is stored';
COMMENT ON COLUMN guliangiam.app_client_certificates.id IS 'Certificate row identifier (UUIDv7, generated by the application layer)';
COMMENT ON COLUMN guliangiam.app_client_certificates.organization_id IS 'Tenant org of the owning application (RLS predicate and fingerprint scope)';
COMMENT ON COLUMN guliangiam.app_client_certificates.application_id IS 'Owning OAuth client application';
COMMENT ON COLUMN guliangiam.app_client_certificates.auth_method IS 'tls_client_auth (CA-issued) or self_signed_tls_client_auth (fingerprint only)';
COMMENT ON COLUMN guliangiam.app_client_certificates.certificate_pem IS 'Public certificate PEM (private key material is rejected on upload)';
COMMENT ON COLUMN guliangiam.app_client_certificates.fingerprint_sha256 IS 'base64url(SHA-256(DER)) RFC 8705 x5t#S256 value, 43 characters';
COMMENT ON COLUMN guliangiam.app_client_certificates.subject_dn IS 'Subject distinguished name used for tls_client_auth binding';
COMMENT ON COLUMN guliangiam.app_client_certificates.issuer_dn IS 'Issuer distinguished name; equals subject_dn for self-signed certificates';
COMMENT ON COLUMN guliangiam.app_client_certificates.not_before IS 'Certificate validity start';
COMMENT ON COLUMN guliangiam.app_client_certificates.not_after IS 'Certificate validity end';
COMMENT ON COLUMN guliangiam.app_client_certificates.status IS 'Lifecycle state: active / retired';
COMMENT ON COLUMN guliangiam.app_client_certificates.retired_at IS 'Time the certificate binding was retired';
ALTER TABLE guliangiam.app_client_certificates ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.app_client_certificates FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'app_client_certificates'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.app_client_certificates
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.app_signin_config (
    application_id UUID NOT NULL PRIMARY KEY,
    disable_signin BOOLEAN NOT NULL DEFAULT FALSE,
    enable_exclusive_signin BOOLEAN NOT NULL DEFAULT FALSE,
    enable_signin_session BOOLEAN NOT NULL DEFAULT FALSE,
    enable_signup BOOLEAN NOT NULL DEFAULT TRUE,
    enable_guest_signin BOOLEAN NOT NULL DEFAULT FALSE,
    enable_auto_signin BOOLEAN NOT NULL DEFAULT TRUE,
    enable_link_with_email BOOLEAN NOT NULL DEFAULT FALSE,
    signin_url VARCHAR(255) NOT NULL DEFAULT '',
    logout_url VARCHAR(255) NOT NULL DEFAULT '',
    logout_redirect_url VARCHAR(255) NOT NULL DEFAULT '',
    prompt_url VARCHAR(255) NOT NULL DEFAULT '',
    signup_url VARCHAR(255) NOT NULL DEFAULT '',
    forget_url VARCHAR(255) NOT NULL DEFAULT '',
    affiliation_url VARCHAR(255) NOT NULL DEFAULT '',
    signin_methods TEXT[] NOT NULL DEFAULT '{}',
    signin_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    failed_signin_limit INTEGER NOT NULL DEFAULT 0,
    failed_signin_frozen_time INTEGER NOT NULL DEFAULT 0,
    code_resend_timeout INTEGER NOT NULL DEFAULT 0,
    code_expire INTEGER NOT NULL DEFAULT 0,
    default_group VARCHAR(255) NOT NULL DEFAULT '',
    org_choice_mode VARCHAR(255) NOT NULL DEFAULT '',
    ip_whitelist VARCHAR(255) NOT NULL DEFAULT '',
    ip_blacklist VARCHAR(255) NOT NULL DEFAULT '',
    cors_allowed_origins VARCHAR(255)[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_app_signin_config_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.app_signin_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.app_signin_config FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'app_signin_config'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.app_signin_config
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.app_saml_config (
    application_id UUID NOT NULL PRIMARY KEY,
    saml_reply_url VARCHAR(255) NOT NULL DEFAULT '',
    certificate VARCHAR(255) NOT NULL DEFAULT '',
    saml_metadata TEXT NOT NULL DEFAULT '',
    saml_metadata_url VARCHAR(2048) NOT NULL DEFAULT '',
    enable_saml_compress BOOLEAN NOT NULL DEFAULT FALSE,
    saml_name_id_format VARCHAR(255) NOT NULL DEFAULT '',
    saml_name_id_content VARCHAR(255) NOT NULL DEFAULT '',
    saml_sign_request BOOLEAN NOT NULL DEFAULT FALSE,
    saml_attributes JSONB NOT NULL DEFAULT '[]'::jsonb,
    enable_saml_c14n10 BOOLEAN NOT NULL DEFAULT FALSE,
    enable_saml_post_binding BOOLEAN NOT NULL DEFAULT FALSE,
    disable_saml_attributes BOOLEAN NOT NULL DEFAULT FALSE,
    enable_saml_assertion_signature BOOLEAN NOT NULL DEFAULT FALSE,
    is_email_saml_nameid BOOLEAN NOT NULL DEFAULT FALSE,
    saml_hash_algorithm VARCHAR(255) NOT NULL DEFAULT '',
    saml_c14n_prefix VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_app_saml_config_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE
);
COMMENT ON COLUMN guliangiam.app_saml_config.saml_metadata IS 'IdP SAML metadata XML; multi-KB document, so unbounded text rather than a 255-char column';
COMMENT ON COLUMN guliangiam.app_saml_config.saml_metadata_url IS 'IdP metadata URL; 2048 is the practical browser/HTTP URL ceiling';
ALTER TABLE guliangiam.app_saml_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.app_saml_config FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'app_saml_config'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.app_saml_config
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.app_form_config (
    application_id UUID NOT NULL PRIMARY KEY,
    signup_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    custom_css VARCHAR(255) NOT NULL DEFAULT '',
    custom_header VARCHAR(255) NOT NULL DEFAULT '',
    form_css VARCHAR(255) NOT NULL DEFAULT '',
    form_css_mobile VARCHAR(255) NOT NULL DEFAULT '',
    form_side_html VARCHAR(255) NOT NULL DEFAULT '',
    form_background_url VARCHAR(255) NOT NULL DEFAULT '',
    form_background_url_mobile VARCHAR(255) NOT NULL DEFAULT '',
    form_offset VARCHAR(255) NOT NULL DEFAULT '',
    form_position VARCHAR(255) NOT NULL DEFAULT '',
    header_html VARCHAR(255) NOT NULL DEFAULT '',
    footer_html VARCHAR(255) NOT NULL DEFAULT '',
    signup_html VARCHAR(255) NOT NULL DEFAULT '',
    signin_html VARCHAR(255) NOT NULL DEFAULT '',
    page_html VARCHAR(255) NOT NULL DEFAULT '',
    custom_js VARCHAR(255) NOT NULL DEFAULT '',
    terms_of_use VARCHAR(255) NOT NULL DEFAULT '',
    privacy_policy VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_app_form_config_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.app_form_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.app_form_config FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'app_form_config'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.app_form_config
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.applications a WHERE a.id = application_id AND a.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.application_provider_bindings (
    application_id UUID NOT NULL,
    provider_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (application_id, provider_id),
    CONSTRAINT fk_application_provider_bindings_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_app_provider_bindings_provider_id ON guliangiam.application_provider_bindings (provider_id);
ALTER TABLE guliangiam.application_provider_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.application_provider_bindings FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'application_provider_bindings'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.application_provider_bindings
            USING (guliangiam.is_super_admin() OR application_id IN (SELECT id FROM guliangiam.applications WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR application_id IN (SELECT id FROM guliangiam.applications WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.providers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    display_name TEXT NOT NULL,
    category TEXT NOT NULL,
    provider_type TEXT NOT NULL,
    sub_type VARCHAR(255) NOT NULL DEFAULT '',
    family VARCHAR(32),
    kind VARCHAR(64),
    variant VARCHAR(32),
    method VARCHAR(255) NOT NULL DEFAULT '',
    client_id VARCHAR(255) NOT NULL DEFAULT '',
    client_secret TEXT NOT NULL,
    client_id2 VARCHAR(255) NOT NULL DEFAULT '',
    client_secret2 TEXT NOT NULL,
    provider_url VARCHAR(255) NOT NULL DEFAULT '',
    auth_url VARCHAR(255) NOT NULL DEFAULT '',
    token_url VARCHAR(255) NOT NULL DEFAULT '',
    user_info_url VARCHAR(255) NOT NULL DEFAULT '',
    callback_url VARCHAR(255) NOT NULL DEFAULT '',
    signing_alg VARCHAR(255) NOT NULL DEFAULT '',
    scopes VARCHAR(255)[] NOT NULL DEFAULT '{}',
    idp_entity_id VARCHAR(255) NOT NULL DEFAULT '',
    sso_url VARCHAR(255) NOT NULL DEFAULT '',
    slo_url VARCHAR(255) NOT NULL DEFAULT '',
    metadata VARCHAR(255) NOT NULL DEFAULT '',
    issuer_url VARCHAR(255) NOT NULL DEFAULT '',
    region VARCHAR(255) NOT NULL DEFAULT '',
    host VARCHAR(255) NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 0,
    database_type VARCHAR(255) NOT NULL DEFAULT '',
    user_mapping JSONB NOT NULL DEFAULT '{}'::jsonb,
    template VARCHAR(255) NOT NULL DEFAULT '',
    custom_api VARCHAR(255) NOT NULL DEFAULT '',
    custom_header VARCHAR(255) NOT NULL DEFAULT '',
    custom_body VARCHAR(255) NOT NULL DEFAULT '',
    admin VARCHAR(255) NOT NULL DEFAULT '',
    password TEXT NOT NULL,
    app_id VARCHAR(255) NOT NULL DEFAULT '',
    app_secret TEXT NOT NULL,
    agent_id VARCHAR(255) NOT NULL DEFAULT '',
    content_type VARCHAR(255) NOT NULL DEFAULT '',
    sender VARCHAR(255) NOT NULL DEFAULT '',
    receiver VARCHAR(255) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_providers_family CHECK (family IS NULL OR family IN ('identity', 'ai', 'payment', 'notification')),
    CONSTRAINT ck_providers_kind CHECK (kind IS NULL OR (family = 'identity' AND kind IN ('github', 'google', 'azure_ad', 'azure_ad_b2c', 'facebook', 'linkedin', 'twitter', 'gitlab', 'gitee', 'okta', 'wechat', 'wechat_mobile', 'wechat_miniprogram', 'wecom_internal', 'wecom_third', 'dingtalk', 'lark', 'feishu', 'qq', 'weibo', 'baidu', 'douyin', 'kwai', 'bilibili', 'infoflow_internal', 'infoflow_third', 'telegram', 'ldap', 'adfs', 'saml', 'oidc', 'custom', 'goth', 'metamask', 'web3onboard', 'alipay', 'wechat_pay', 'smtp', 'default', 'local', 'faceid', 'apple', 'keycloak', 'slack', 'discord', 'steam', 'amazon', 'sendgrid', 'azure_acs', 'resend', 'submail_email', 'mailtrap_email', 'custom_http', 'sms', 'twilio_sms', 'alibaba_sms', 'alibaba_pnvs', 'tencent_sms', 'volcengine_sms', 'huawei_sms', 'amazon_sns', 'azure_acs_sms', 'infobip', 'ucloud_sms', 'emay_sms', 'baidu_sms', 'submail', 'mailtrap_sms', 'oson_sms', 'mock_sms', 'twilio_verify', 'smsbao', 'msg91', 'custom_http_sms', 'aws_s3', 'azure_blob', 'gcs', 'minio', 'ali_oss', 'tencent_cos', 'qiniu', 'cu_cloud', 'huawei_obs', 'synology', 'recaptcha', 'recaptcha_v3', 'hcaptcha', 'turnstile', 'geetest', 'aliyun_captcha', 'totp', 'radius', 'push', 'aliyun_face_body', 'wechat_scan', 'dingtalk_scan', 'jumio', 'aliyun_idv', 'elasticsearch', 'loki')) OR (family = 'ai' AND kind IN ('openai', 'anthropic', 'gemini', 'deepseek', 'glm', 'kimi', 'qwen', 'seed', 'ollama', 'vllm', 'custom')) OR (family = 'payment' AND kind IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy')) OR (family = 'notification' AND kind IN ('email', 'sms', 'dingtalk', 'lark', 'wecom', 'slack', 'discord', 'rocketchat', 'teams', 'bark', 'telegram', 'pushover', 'line', 'matrix', 'pushbullet', 'viber', 'webpush', 'twitter', 'reddit'))),
    CONSTRAINT ck_providers_variant CHECK (variant IS NULL OR (family = 'payment' AND variant IN ('sandbox', 'production')) OR (family IN ('identity', 'ai', 'notification') AND variant = '')),
    CONSTRAINT fk_providers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_providers_registration ON guliangiam.providers (organization_id, family, kind, name, variant) WHERE family IS NOT NULL AND kind IS NOT NULL AND variant IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_providers_organization_id ON guliangiam.providers (organization_id);
CREATE INDEX IF NOT EXISTS idx_providers_category ON guliangiam.providers (category) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_providers_org_category ON guliangiam.providers (organization_id, category) WHERE deleted_at IS NULL;
ALTER TABLE guliangiam.providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.providers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'providers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.providers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.provider_configs (
    provider_id UUID NOT NULL,
    config_type VARCHAR(255) NOT NULL DEFAULT '',
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider_id, config_type),
    CONSTRAINT fk_provider_configs_provider_id_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.providers(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.provider_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.provider_configs FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'provider_configs'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.provider_configs
            USING (guliangiam.is_super_admin() OR provider_id IN (SELECT id FROM guliangiam.providers WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR provider_id IN (SELECT id FROM guliangiam.providers WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.provider_secrets (
    id UUID NOT NULL PRIMARY KEY,
    provider_id UUID NOT NULL,
    secret_type VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    last_rotated_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_provider_secrets_provider_id_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.providers(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_provider_secrets_provider_id ON guliangiam.provider_secrets (provider_id, updated_at DESC);
ALTER TABLE guliangiam.provider_secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.provider_secrets FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'provider_secrets'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.provider_secrets
            USING (guliangiam.is_super_admin() OR provider_id IN (SELECT id FROM guliangiam.providers WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR provider_id IN (SELECT id FROM guliangiam.providers WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.webhooks (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    url VARCHAR(255) NOT NULL DEFAULT '',
    method VARCHAR(64) NOT NULL DEFAULT 'POST',
    content_type VARCHAR(64) NOT NULL DEFAULT 'application/json',
    headers VARCHAR(255) NOT NULL DEFAULT '',
    events JSONB NOT NULL DEFAULT '[]',
    custom_body VARCHAR(255) NOT NULL DEFAULT '',
    custom_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    cert_id VARCHAR(255) NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    secret VARCHAR(255) NOT NULL DEFAULT '',
    is_error BOOLEAN NOT NULL DEFAULT false,
    error_text VARCHAR(255) NOT NULL DEFAULT '',
    organization VARCHAR(255) NOT NULL DEFAULT '',
    token_fields VARCHAR(255)[] NOT NULL DEFAULT '{}',
    object_fields VARCHAR(255)[] NOT NULL DEFAULT '{}',
    is_user_extended BOOLEAN NOT NULL DEFAULT false,
    single_org_only BOOLEAN NOT NULL DEFAULT false,
    retry_count INTEGER NOT NULL DEFAULT 3,
    max_retries INTEGER NOT NULL DEFAULT 3,
    retry_interval INTEGER NOT NULL DEFAULT 60,
    use_exponential_backoff BOOLEAN NOT NULL DEFAULT false,
    timeout_ms INTEGER NOT NULL DEFAULT 5000,
    retry_policy JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_webhooks_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_webhooks_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.webhooks ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.webhooks FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'webhooks'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.webhooks
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.agents (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(64) NOT NULL DEFAULT 'gateway-proxy',
    host VARCHAR(255) NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 0,
    url VARCHAR(255) NOT NULL DEFAULT '',
    token VARCHAR(255) NOT NULL DEFAULT '',
    application_id UUID,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    last_ping_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_agents_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_agents_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE SET NULL,
    CONSTRAINT fk_agents_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.agents ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.agents FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'agents'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.agents
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.verifications (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    type TEXT NOT NULL,
    recipient TEXT NOT NULL,
    code TEXT NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_verifications_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_verifications_org_created_at_desc ON guliangiam.verifications (organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_verifications_recipient ON guliangiam.verifications (recipient);
CREATE INDEX IF NOT EXISTS idx_verifications_type_status ON guliangiam.verifications (type, status);
CREATE INDEX IF NOT EXISTS idx_verifications_recipient_code_type_pending ON guliangiam.verifications (recipient, code, type) WHERE status = 'pending';
ALTER TABLE guliangiam.verifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.verifications FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'verifications'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.verifications
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.captcha_providers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    provider_type VARCHAR(255) NOT NULL DEFAULT '',
    site_key VARCHAR(255) NOT NULL DEFAULT '',
    secret_key TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_captcha_providers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_captcha_providers_organization_id ON guliangiam.captcha_providers (organization_id);
ALTER TABLE guliangiam.captcha_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.captcha_providers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'captcha_providers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.captcha_providers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.face_id_providers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    provider_type VARCHAR(255) NOT NULL DEFAULT '',
    access_key_id VARCHAR(255) NOT NULL DEFAULT '',
    access_key_secret VARCHAR(255) NOT NULL DEFAULT '',
    endpoint VARCHAR(255) NOT NULL DEFAULT '',
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_face_id_providers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_face_id_providers_organization_id ON guliangiam.face_id_providers (organization_id);
ALTER TABLE guliangiam.face_id_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.face_id_providers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'face_id_providers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.face_id_providers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.dpop_replay (
    jti TEXT NOT NULL PRIMARY KEY,
    seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS guliangiam.webhook_events (
    id UUID NOT NULL PRIMARY KEY,
    webhook_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_code INTEGER,
    response_body TEXT,
    status VARCHAR(64) NOT NULL DEFAULT 'pending'::text,
    retry_count INTEGER NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_webhook_events_webhook_id_webhooks FOREIGN KEY (webhook_id) REFERENCES guliangiam.webhooks(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.webhook_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.webhook_events FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'webhook_events'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.webhook_events
            USING (guliangiam.is_super_admin() OR webhook_id IN (SELECT id FROM guliangiam.webhooks WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR webhook_id IN (SELECT id FROM guliangiam.webhooks WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.web3_identity_mappings (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    credential_id UUID NOT NULL,
    wallet_address CHAR(42) NOT NULL,
    derivation_method VARCHAR(64) NOT NULL DEFAULT 'keccak256-passkey',
    derivation_seed CHAR(64) NOT NULL,
    chain_ids INTEGER[] NOT NULL DEFAULT '{1,137,56,10,42161,42170}',
    ens_name TEXT,
    is_primary BOOLEAN NOT NULL DEFAULT TRUE,
    verified_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_web3_identity_mappings_wallet_address UNIQUE (wallet_address),
    CONSTRAINT uq_web3_identity_mappings_credential UNIQUE (credential_id),
    CONSTRAINT fk_web3_identity_mappings_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_web3_identity_mappings_user ON guliangiam.web3_identity_mappings (user_id);
COMMENT ON COLUMN guliangiam.web3_identity_mappings.user_id IS 'Owning user — FK to guliangiam.users';
COMMENT ON COLUMN guliangiam.web3_identity_mappings.credential_id IS 'FK to guliangiam.webauthn_credentials.id — the passkey whose pubkey was used to derive this address';
COMMENT ON COLUMN guliangiam.web3_identity_mappings.wallet_address IS 'Derived EVM address (0x + 40 hex). Deterministic from passkey pubkey via keccak256(pubkey || "guliangiam-web3-v1")[12:]';
COMMENT ON COLUMN guliangiam.web3_identity_mappings.derivation_method IS 'Method used to derive the address. Currently only ''keccak256-passkey''. Reserved for future extension.';
COMMENT ON COLUMN guliangiam.web3_identity_mappings.derivation_seed IS 'Hex-encoded keccak256(compressed_pubkey || "guliangiam-web3-v1") — full 32-byte hash captured for auditability';
COMMENT ON COLUMN guliangiam.web3_identity_mappings.chain_ids IS 'Supported EVM chain IDs for this identity mapping';
COMMENT ON COLUMN guliangiam.web3_identity_mappings.ens_name IS 'Optional ENS name resolved at registration time';
COMMENT ON COLUMN guliangiam.web3_identity_mappings.verified_at IS 'When the mapping was created/verified';
COMMENT ON COLUMN guliangiam.web3_identity_mappings.last_used_at IS 'Updated on each successful login via this passkey → address';
ALTER TABLE guliangiam.web3_identity_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.web3_identity_mappings FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'web3_identity_mappings'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.web3_identity_mappings
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.identity_verifications (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    user_id UUID,
    provider VARCHAR(32) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    redirect_url TEXT,
    session_token VARCHAR(255),
    provider_ref VARCHAR(255),
    first_name VARCHAR(64),
    last_name VARCHAR(64),
    date_of_birth DATE,
    nationality VARCHAR(3),
    id_type VARCHAR(32),
    id_number VARCHAR(128),
    confidence DECIMAL(19, 4),
    reason TEXT,
    raw_result JSONB,
    id_number_sealed TEXT,
    raw_result_sealed TEXT,
    initiated_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_idv_status CHECK (status IN ('pending', 'in_progress', 'approved', 'rejected', 'expired')),
    CONSTRAINT fk_identity_verifications_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_identity_verifications_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_idv_org_user ON guliangiam.identity_verifications (organization_id, user_id, initiated_at);
CREATE INDEX IF NOT EXISTS idx_idv_provider_ref ON guliangiam.identity_verifications (provider, provider_ref);
COMMENT ON TABLE guliangiam.identity_verifications IS 'Identity verification durable records — survives provider session lifetime';
COMMENT ON COLUMN guliangiam.identity_verifications.id IS 'Verification primary key';
COMMENT ON COLUMN guliangiam.identity_verifications.organization_id IS 'FK -> organizations.id (tenant boundary)';
COMMENT ON COLUMN guliangiam.identity_verifications.user_id IS 'FK -> users.id (nullable — pre-registration verification)';
COMMENT ON COLUMN guliangiam.identity_verifications.provider IS 'Verification provider (jumio/onfido/manual/etc)';
COMMENT ON COLUMN guliangiam.identity_verifications.status IS 'pending/in_progress/approved/rejected/expired';
COMMENT ON COLUMN guliangiam.identity_verifications.redirect_url IS 'Post-verification redirect URL for provider callback';
COMMENT ON COLUMN guliangiam.identity_verifications.session_token IS 'Provider session correlation token';
COMMENT ON COLUMN guliangiam.identity_verifications.provider_ref IS 'Provider-side reference (session id / verification id)';
COMMENT ON COLUMN guliangiam.identity_verifications.first_name IS 'Given name (from provider)';
COMMENT ON COLUMN guliangiam.identity_verifications.last_name IS 'Family name (from provider)';
COMMENT ON COLUMN guliangiam.identity_verifications.date_of_birth IS 'Date of birth (from provider)';
COMMENT ON COLUMN guliangiam.identity_verifications.nationality IS 'ISO 3166-1 alpha-2 country code';
COMMENT ON COLUMN guliangiam.identity_verifications.id_type IS 'Government-issued ID type (passport/driver_license/etc)';
COMMENT ON COLUMN guliangiam.identity_verifications.id_number IS 'Government-issued ID number (PII — encrypted at rest)';
COMMENT ON COLUMN guliangiam.identity_verifications.confidence IS 'Provider confidence score (0.0 – 1.0)';
COMMENT ON COLUMN guliangiam.identity_verifications.reason IS 'Reject reason or completion note';
COMMENT ON COLUMN guliangiam.identity_verifications.raw_result IS 'Complete provider payload (PII — encrypted at rest)';
COMMENT ON COLUMN guliangiam.identity_verifications.id_number_sealed IS 'v2 envelope of id_number (PII); NULL = legacy plaintext row remains';
COMMENT ON COLUMN guliangiam.identity_verifications.raw_result_sealed IS 'v2 envelope of the raw_result plaintext payload (PII/audit); NULL = legacy plaintext row remains';
COMMENT ON COLUMN guliangiam.identity_verifications.initiated_at IS 'When user started verification';
COMMENT ON COLUMN guliangiam.identity_verifications.completed_at IS 'When provider returned final result';
ALTER TABLE guliangiam.identity_verifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.identity_verifications FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'identity_verifications'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.identity_verifications
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.custom_attribute_definitions (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    key TEXT NOT NULL,
    data_type VARCHAR(16) NOT NULL DEFAULT 'string',
    required BOOLEAN NOT NULL DEFAULT false,
    options JSONB NOT NULL DEFAULT '[]'::jsonb,
    writable BOOLEAN NOT NULL DEFAULT true,
    visibility VARCHAR(16) NOT NULL DEFAULT 'private',
    description TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_custom_attribute_definitions_data_type CHECK (data_type IN ('string', 'number', 'boolean', 'date', 'enum')),
    CONSTRAINT ck_custom_attribute_definitions_visibility CHECK (visibility IN ('private', 'public', 'admin')),
    CONSTRAINT fk_custom_attribute_definitions_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_custom_attribute_definitions_org_key ON guliangiam.custom_attribute_definitions (organization_id, key);
CREATE INDEX IF NOT EXISTS idx_custom_attribute_definitions_org_enabled ON guliangiam.custom_attribute_definitions (organization_id, enabled);
COMMENT ON COLUMN guliangiam.custom_attribute_definitions.key IS 'Attribute key used inside user_profiles.properties (for example department)';
COMMENT ON COLUMN guliangiam.custom_attribute_definitions.data_type IS 'string | number | boolean | date | enum';
COMMENT ON COLUMN guliangiam.custom_attribute_definitions.options IS 'Allowed values for data_type=enum (JSON array of strings)';
COMMENT ON COLUMN guliangiam.custom_attribute_definitions.visibility IS 'private | public | admin';
ALTER TABLE guliangiam.custom_attribute_definitions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.custom_attribute_definitions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'custom_attribute_definitions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.custom_attribute_definitions
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.app_client_jwks_uris (
    application_id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    jwks_uri TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_app_client_jwks_uris_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_app_client_jwks_uris_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.app_client_jwks_uris ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.app_client_jwks_uris FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'app_client_jwks_uris'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.app_client_jwks_uris
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：30_authz.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.roles (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    group_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    role_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    domains JSONB NOT NULL DEFAULT '[]'::jsonb,
    users JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_roles_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_roles_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_roles_org_enabled ON guliangiam.roles (organization_id, is_enabled) WHERE is_enabled = true;
ALTER TABLE guliangiam.roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.roles FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'roles'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.roles
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_role_bindings (
    user_id UUID NOT NULL,
    role_id UUID NOT NULL,
    source VARCHAR(64) NOT NULL DEFAULT 'manual',
    starts_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    lifecycle_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_user_role_bindings_role_id_roles FOREIGN KEY (role_id) REFERENCES guliangiam.roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_role_bindings_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_role_bindings_role_id ON guliangiam.user_role_bindings (role_id);
CREATE INDEX IF NOT EXISTS idx_user_role_bindings_user_id ON guliangiam.user_role_bindings (user_id);
ALTER TABLE guliangiam.user_role_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_role_bindings FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_role_bindings'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_role_bindings
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.roles r WHERE r.id = role_id AND r.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.roles r WHERE r.id = role_id AND r.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.permissions (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    users JSONB NOT NULL DEFAULT '[]'::jsonb,
    groups JSONB NOT NULL DEFAULT '[]'::jsonb,
    roles JSONB NOT NULL DEFAULT '[]'::jsonb,
    domains JSONB NOT NULL DEFAULT '[]'::jsonb,
    resource_type VARCHAR(255) NOT NULL DEFAULT '',
    resources JSONB NOT NULL DEFAULT '[]'::jsonb,
    actions JSONB NOT NULL DEFAULT '[]'::jsonb,
    effect VARCHAR(64) NOT NULL DEFAULT 'Allow',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    submitter VARCHAR(255) NOT NULL DEFAULT '',
    approver VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(255) NOT NULL DEFAULT '',
    model VARCHAR(255) NOT NULL DEFAULT '',
    adapter VARCHAR(255) NOT NULL DEFAULT '',
    sorter VARCHAR(255) NOT NULL DEFAULT '',
    application VARCHAR(255) NOT NULL DEFAULT '',
    approve_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_permissions_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_permissions_organization_id ON guliangiam.permissions (organization_id);
CREATE INDEX IF NOT EXISTS idx_permissions_org_enabled ON guliangiam.permissions (organization_id, is_enabled) WHERE is_enabled = true;
ALTER TABLE guliangiam.permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.permissions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'permissions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.permissions
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.groups (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    manager VARCHAR(255) NOT NULL DEFAULT '',
    contact_email VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(255) NOT NULL DEFAULT '',
    parent_id UUID,
    is_top_group BOOLEAN NOT NULL DEFAULT false,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    homepage VARCHAR(255) NOT NULL DEFAULT '',
    users JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_groups_organization_id_name UNIQUE (organization_id, name),
    CONSTRAINT fk_groups_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_groups_parent_id_groups FOREIGN KEY (parent_id) REFERENCES guliangiam.groups(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_groups_parent_id ON guliangiam.groups (organization_id, parent_id);
CREATE INDEX IF NOT EXISTS idx_groups_org_enabled ON guliangiam.groups (organization_id, is_enabled) WHERE is_enabled = true;
ALTER TABLE guliangiam.groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.groups FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'groups'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.groups
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.user_group_memberships (
    user_id UUID NOT NULL,
    group_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, group_id),
    CONSTRAINT fk_user_group_memberships_group_id_groups FOREIGN KEY (group_id) REFERENCES guliangiam.groups(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_group_memberships_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_group_memberships_group_id ON guliangiam.user_group_memberships (group_id);
CREATE INDEX IF NOT EXISTS idx_user_group_memberships_user_id ON guliangiam.user_group_memberships (user_id);
ALTER TABLE guliangiam.user_group_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.user_group_memberships FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'user_group_memberships'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.user_group_memberships
            USING (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.casbin_rules (
    id UUID NOT NULL PRIMARY KEY,
    enforcer_id UUID NOT NULL,
    ptype VARCHAR(255) NOT NULL DEFAULT '',
    v0 VARCHAR(255) NOT NULL DEFAULT '',
    v1 VARCHAR(255) NOT NULL DEFAULT '',
    v2 VARCHAR(255) NOT NULL DEFAULT '',
    v3 VARCHAR(255) NOT NULL DEFAULT '',
    v4 VARCHAR(255) NOT NULL DEFAULT '',
    v5 VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_casbin_rules_rule UNIQUE (enforcer_id, ptype, v0, v1, v2, v3, v4, v5)
);
CREATE INDEX IF NOT EXISTS idx_casbin_rules_ptype ON guliangiam.casbin_rules (ptype);

CREATE TABLE IF NOT EXISTS guliangiam.resources (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    application_id UUID,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(64) NOT NULL DEFAULT 'file',
    tag VARCHAR(255) NOT NULL DEFAULT '',
    file_path VARCHAR(255) NOT NULL DEFAULT '',
    file_size BIGINT NOT NULL DEFAULT 0,
    mime_type VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_resources_org_app_name UNIQUE (organization_id, application_id, name),
    CONSTRAINT fk_resources_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE SET NULL,
    CONSTRAINT fk_resources_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_resources_type ON guliangiam.resources (type);
ALTER TABLE guliangiam.resources ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.resources FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'resources'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.resources
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.models (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    description VARCHAR(255) NOT NULL DEFAULT '',
    model_text TEXT NOT NULL,
    owner VARCHAR(255) NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_models_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_models_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.models ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.models FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'models'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.models
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.adapters (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    description VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(64) NOT NULL DEFAULT 'database',
    server VARCHAR(255) NOT NULL DEFAULT '',
    database VARCHAR(255) NOT NULL DEFAULT '',
    table_name VARCHAR(255) NOT NULL DEFAULT '',
    username VARCHAR(255) NOT NULL DEFAULT '',
    password VARCHAR(255) NOT NULL DEFAULT '',
    owner VARCHAR(255) NOT NULL DEFAULT '',
    use_same_db BOOLEAN NOT NULL DEFAULT TRUE,
    database_type VARCHAR(64) NOT NULL DEFAULT 'postgresql',
    host VARCHAR(255) NOT NULL DEFAULT '',
    port INTEGER,
    db_user VARCHAR(255) NOT NULL DEFAULT '',
    db_password VARCHAR(255) NOT NULL DEFAULT '',
    database_name VARCHAR(255) NOT NULL DEFAULT '',
    policy VARCHAR(255) NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_adapters_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_adapters_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.adapters ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.adapters FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'adapters'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.adapters
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.enforcers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    description VARCHAR(255) NOT NULL DEFAULT '',
    model_id UUID,
    adapter_id UUID,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    owner VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_enforcers_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_enforcers_adapter_id_adapters FOREIGN KEY (adapter_id) REFERENCES guliangiam.adapters(id) ON DELETE SET NULL,
    CONSTRAINT fk_enforcers_model_id_models FOREIGN KEY (model_id) REFERENCES guliangiam.models(id) ON DELETE SET NULL,
    CONSTRAINT fk_enforcers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_enforcers_is_enabled ON guliangiam.enforcers (is_enabled);
ALTER TABLE guliangiam.enforcers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.enforcers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'enforcers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.enforcers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.syncers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(64) NOT NULL DEFAULT 'database',
    database_type VARCHAR(255) NOT NULL DEFAULT '',
    host VARCHAR(255) NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 0,
    database VARCHAR(255) NOT NULL DEFAULT '',
    table_name VARCHAR(255) NOT NULL DEFAULT '',
    username VARCHAR(255) NOT NULL DEFAULT '',
    password VARCHAR(255) NOT NULL DEFAULT '',
    ssl_mode VARCHAR(64) NOT NULL DEFAULT 'disable',
    ssh_type VARCHAR(255) NOT NULL DEFAULT '',
    ssh_host VARCHAR(255) NOT NULL DEFAULT '',
    ssh_port INTEGER NOT NULL DEFAULT 0,
    ssh_username VARCHAR(255) NOT NULL DEFAULT '',
    ssh_password VARCHAR(255) NOT NULL DEFAULT '',
    ssh_cert_id VARCHAR(255) NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    sync_interval INTEGER NOT NULL DEFAULT 0,
    field_mapping VARCHAR(255) NOT NULL DEFAULT '',
    column_mapping JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_read_only BOOLEAN NOT NULL DEFAULT false,
    affinity VARCHAR(255) NOT NULL DEFAULT '',
    affiliation_table VARCHAR(255) NOT NULL DEFAULT '',
    avatar_base_url VARCHAR(255) NOT NULL DEFAULT '',
    error_text VARCHAR(255) NOT NULL DEFAULT '',
    last_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_syncers_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_syncers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_syncers_is_enabled ON guliangiam.syncers (is_enabled) WHERE is_enabled = true;
ALTER TABLE guliangiam.syncers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.syncers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'syncers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.syncers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.relation_namespaces (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    relation_defs JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    revision_counter BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT uq_relation_namespaces_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_relation_namespaces_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.relation_namespaces ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.relation_namespaces FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'relation_namespaces'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.relation_namespaces
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.relation_tuples (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    namespace_id UUID NOT NULL,
    subject TEXT NOT NULL,
    subject_relation TEXT,
    relation TEXT NOT NULL,
    object TEXT NOT NULL,
    zookie VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_relation_tuples_org_ns_sub_rel_obj UNIQUE (organization_id, namespace_id, subject, relation, object),
    CONSTRAINT fk_relation_tuples_namespace_id_relation_namespaces FOREIGN KEY (namespace_id) REFERENCES guliangiam.relation_namespaces(id) ON DELETE CASCADE,
    CONSTRAINT fk_relation_tuples_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_relation_tuples_object ON guliangiam.relation_tuples (object);
CREATE INDEX IF NOT EXISTS idx_relation_tuples_subject ON guliangiam.relation_tuples (subject);
ALTER TABLE guliangiam.relation_tuples ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.relation_tuples FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'relation_tuples'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.relation_tuples
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：40_session.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.sessions (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    application_id UUID NOT NULL,
    session_state VARCHAR(64) NOT NULL DEFAULT 'active',
    sid TEXT,
    device_info JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip INET,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    authentication_level VARCHAR(20) NOT NULL DEFAULT 'password',
    assurance_level VARCHAR(10) NOT NULL DEFAULT 'aal1',
    is_privileged BOOLEAN NOT NULL DEFAULT false,
    lifecycle_id UUID,
    ticket_id UUID,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_sessions_sid UNIQUE (sid),
    CONSTRAINT ck_sessions_state CHECK (session_state IN ('active', 'idle', 'expired', 'revoked')),
    CONSTRAINT ck_sessions_assurance_level CHECK (assurance_level IN ('aal1', 'aal2')),
    CONSTRAINT fk_sessions_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_sessions_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON guliangiam.sessions (expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_user_active ON guliangiam.sessions (user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_application_id ON guliangiam.sessions (application_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_app_active ON guliangiam.sessions (user_id, application_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_user_expires_revoked ON guliangiam.sessions (user_id, expires_at, revoked_at);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_revoked ON guliangiam.sessions (expires_at, revoked_at);
CREATE INDEX IF NOT EXISTS idx_sessions_org_app_created ON guliangiam.sessions (user_id, application_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id_created ON guliangiam.sessions (user_id, created_at DESC);
COMMENT ON COLUMN guliangiam.sessions.id IS 'Session primary key (UUID)';
COMMENT ON COLUMN guliangiam.sessions.user_id IS 'Owner user';
COMMENT ON COLUMN guliangiam.sessions.application_id IS 'Client application the session was issued for';
COMMENT ON COLUMN guliangiam.sessions.session_state IS 'Lifecycle: active / revoked / expired / idle_timeout';
COMMENT ON COLUMN guliangiam.sessions.sid IS 'Opaque session identifier used for session management lookups';
COMMENT ON COLUMN guliangiam.sessions.device_info IS 'Client device fingerprint + OS/browser detail snapshot captured at login';
COMMENT ON COLUMN guliangiam.sessions.ip IS 'IP address observed at session creation / last-seen refresh';
COMMENT ON COLUMN guliangiam.sessions.user_agent IS 'Raw User-Agent header captured at session creation';
COMMENT ON COLUMN guliangiam.sessions.expires_at IS 'Hard expiration timestamp; session becomes unusable after this point';
COMMENT ON COLUMN guliangiam.sessions.revoked_at IS 'Explicit revocation timestamp (admin revoke or logout); NULL when active';
COMMENT ON COLUMN guliangiam.sessions.last_seen_at IS 'Last observed activity timestamp for session restore, idle timeout, and device management flows.';
COMMENT ON COLUMN guliangiam.sessions.authentication_level IS 'How the user authenticated when this session was created: password / mfa / fido2. Used by concurrent-session policy (admin session exemption requires level >= mfa).';
COMMENT ON COLUMN guliangiam.sessions.assurance_level IS 'NIST SP 800-63B authenticator assurance level: aal1 (password only) / aal2 (MFA or FIDO2 satisfied). step-up raises only the current session.';
COMMENT ON COLUMN guliangiam.sessions.is_privileged IS 'Session elevated through an approved IGA access request';
COMMENT ON COLUMN guliangiam.sessions.lifecycle_id IS 'IGA lifecycle that authorizes the privileged session';
COMMENT ON COLUMN guliangiam.sessions.ticket_id IS 'Access request that authorized the privileged session';
ALTER TABLE guliangiam.sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.sessions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'sessions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.sessions
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.access_tokens (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    application_id UUID NOT NULL,
    token_hash TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    device_secret_hash TEXT,
    device_secret_expires_in INTEGER NOT NULL DEFAULT 0,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    audience TEXT[] NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_access_tokens_hash UNIQUE (token_hash),
    CONSTRAINT fk_access_tokens_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_access_tokens_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_access_tokens_authorization_code_id ON guliangiam.access_tokens ((metadata ->> 'authorizationCodeId')) WHERE metadata ? 'authorizationCodeId';
CREATE INDEX IF NOT EXISTS idx_access_tokens_expires_at ON guliangiam.access_tokens (expires_at);
CREATE INDEX IF NOT EXISTS idx_access_tokens_user_active ON guliangiam.access_tokens (user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_access_tokens_application_id ON guliangiam.access_tokens (application_id);
CREATE INDEX IF NOT EXISTS idx_access_tokens_expires_at_not_revoked ON guliangiam.access_tokens (expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_access_tokens_device_secret_hash ON guliangiam.access_tokens (device_secret_hash) WHERE device_secret_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tokens_app_expires ON guliangiam.access_tokens (application_id, expires_at, revoked_at);
CREATE INDEX IF NOT EXISTS idx_access_tokens_user_app ON guliangiam.access_tokens (user_id, application_id);
CREATE INDEX IF NOT EXISTS idx_access_tokens_user_id_created ON guliangiam.access_tokens (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_access_tokens_application_id_created ON guliangiam.access_tokens (application_id, created_at DESC);
COMMENT ON COLUMN guliangiam.access_tokens.id IS 'Access token primary key (UUID)';
COMMENT ON COLUMN guliangiam.access_tokens.user_id IS 'Token owner user';
COMMENT ON COLUMN guliangiam.access_tokens.application_id IS 'Client application the token was issued for';
COMMENT ON COLUMN guliangiam.access_tokens.token_hash IS 'SHA-256 hash of the token value used for lookup and revocation.';
COMMENT ON COLUMN guliangiam.access_tokens.token_prefix IS 'Non-sensitive prefix shown in management UIs instead of exposing the full token value.';
COMMENT ON COLUMN guliangiam.access_tokens.device_secret_hash IS 'SHA-256 of the bound device secret; set only for Proof-of-Possession tokens';
COMMENT ON COLUMN guliangiam.access_tokens.device_secret_expires_in IS 'Seconds until device_secret_hash expires; 0 when not bound';
COMMENT ON COLUMN guliangiam.access_tokens.scopes IS 'Space-separated OAuth scopes granted to this token';
COMMENT ON COLUMN guliangiam.access_tokens.audience IS 'Intended API audiences (JWT aud claim)';
COMMENT ON COLUMN guliangiam.access_tokens.metadata IS 'Extension attribute bag; may contain authorization_code_id, dpop_jkt, org_idp context etc.';
COMMENT ON COLUMN guliangiam.access_tokens.expires_at IS 'Token lifetime end; unix timestamp in seconds';
COMMENT ON COLUMN guliangiam.access_tokens.revoked_at IS 'Revocation timestamp; NULL when token is active';
ALTER TABLE guliangiam.access_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.access_tokens FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'access_tokens'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.access_tokens
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.refresh_tokens (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    application_id UUID NOT NULL,
    token_hash TEXT NOT NULL,
    token_prefix VARCHAR(255) NOT NULL DEFAULT '',
    device_secret_hash TEXT,
    device_secret_expires_in INTEGER NOT NULL DEFAULT 0,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    audience TEXT[] NOT NULL DEFAULT '{}',
    access_token_id UUID,
    family_id UUID,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_refresh_tokens_hash UNIQUE (token_hash),
    CONSTRAINT fk_refresh_tokens_access_token_id_access_tokens FOREIGN KEY (access_token_id) REFERENCES guliangiam.access_tokens(id) ON DELETE SET NULL,
    CONSTRAINT fk_refresh_tokens_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_refresh_tokens_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE,
    CONSTRAINT fk_refresh_tokens_family_id_refresh_tokens FOREIGN KEY (family_id) REFERENCES guliangiam.refresh_tokens(id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON guliangiam.refresh_tokens (family_id) WHERE family_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_access_token_id ON guliangiam.refresh_tokens (access_token_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON guliangiam.refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at_not_revoked ON guliangiam.refresh_tokens (expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_application_id ON guliangiam.refresh_tokens (application_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON guliangiam.refresh_tokens (token_hash) WHERE token_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_revoked ON guliangiam.refresh_tokens (family_id, revoked_at) WHERE family_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_app ON guliangiam.refresh_tokens (user_id, application_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id_created ON guliangiam.refresh_tokens (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_application_id_created ON guliangiam.refresh_tokens (application_id, created_at DESC);
COMMENT ON COLUMN guliangiam.refresh_tokens.id IS 'Refresh token primary key (UUID)';
COMMENT ON COLUMN guliangiam.refresh_tokens.user_id IS 'Token owner user';
COMMENT ON COLUMN guliangiam.refresh_tokens.application_id IS 'Client application the token was issued for';
COMMENT ON COLUMN guliangiam.refresh_tokens.token_hash IS 'SHA-256 of the refresh token; stored instead of plaintext';
COMMENT ON COLUMN guliangiam.refresh_tokens.token_prefix IS 'First N chars of plaintext token shown in admin UIs for identification';
COMMENT ON COLUMN guliangiam.refresh_tokens.device_secret_hash IS 'Bound-device proof hash; refresh token rotation verifies same device';
COMMENT ON COLUMN guliangiam.refresh_tokens.device_secret_expires_in IS 'Seconds until device_secret_hash expires';
COMMENT ON COLUMN guliangiam.refresh_tokens.scopes IS 'Scopes this refresh token is restricted to (narrowest of parent access token)';
COMMENT ON COLUMN guliangiam.refresh_tokens.audience IS 'Audience carried forward from the access token family';
COMMENT ON COLUMN guliangiam.refresh_tokens.access_token_id IS 'The access token this refresh token was first minted with (link for revocation chains)';
COMMENT ON COLUMN guliangiam.refresh_tokens.family_id IS 'Rotating-token family UUID; shared across token rotation generations for reuse detection';
COMMENT ON COLUMN guliangiam.refresh_tokens.expires_at IS 'Hard expiration; refresh rejected after this point even if unused';
COMMENT ON COLUMN guliangiam.refresh_tokens.revoked_at IS 'Revocation timestamp; part of reuse-detection flow (rotation marks previous gen revoked)';
ALTER TABLE guliangiam.refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.refresh_tokens FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'refresh_tokens'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.refresh_tokens
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.login_challenges (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID,
    user_id UUID,
    application_id UUID,
    challenge_type VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    method_type VARCHAR(255) NOT NULL DEFAULT '',
    verified_at TIMESTAMPTZ,
    identity_provider VARCHAR(255) NOT NULL DEFAULT '',
    identity_sub VARCHAR(255) NOT NULL DEFAULT '',
    redirect_url VARCHAR(255) NOT NULL DEFAULT '',
    device_info JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip INET,
    user_agent TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_login_challenges_status CHECK (status IN ('pending', 'push_pending', 'approved', 'rejected')),
    CONSTRAINT fk_login_challenges_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_login_challenges_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_login_challenges_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_auth_login_challenges_user_id ON guliangiam.login_challenges (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_login_challenges_expires_at ON guliangiam.login_challenges (expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_login_challenges_status ON guliangiam.login_challenges (status);
CREATE INDEX IF NOT EXISTS idx_login_challenges_application_id ON guliangiam.login_challenges (application_id);
COMMENT ON TABLE guliangiam.login_challenges IS 'Pending MFA login challenges created after password verification and consumed by second-step MFA verification.';
COMMENT ON COLUMN guliangiam.login_challenges.organization_id IS 'Tenant org of the user attempting login';
COMMENT ON COLUMN guliangiam.login_challenges.user_id IS 'User being authenticated (populated after account identification step)';
COMMENT ON COLUMN guliangiam.login_challenges.application_id IS 'Client app that started the login flow';
COMMENT ON COLUMN guliangiam.login_challenges.challenge_type IS 'MFA factor requested: totp / webauthn / sms / email / push / qr';
COMMENT ON COLUMN guliangiam.login_challenges.status IS 'Current login challenge state such as pending, push_pending, approved, or rejected.';
COMMENT ON COLUMN guliangiam.login_challenges.method_type IS 'MFA method currently associated with the challenge when state transitions need method-specific handling.';
COMMENT ON COLUMN guliangiam.login_challenges.verified_at IS 'Timestamp when an external approval path marked the challenge approved.';
COMMENT ON COLUMN guliangiam.login_challenges.identity_provider IS 'IdP identifier for federated MFA; empty for local factors';
COMMENT ON COLUMN guliangiam.login_challenges.identity_sub IS 'IdP subject claim for federated MFA correlation';
COMMENT ON COLUMN guliangiam.login_challenges.redirect_url IS 'Post-login callback URL; stored at challenge creation, sent to the approval channel';
COMMENT ON COLUMN guliangiam.login_challenges.device_info IS 'Device fingerprint at challenge initiation';
COMMENT ON COLUMN guliangiam.login_challenges.ip IS 'Request IP at challenge creation';
COMMENT ON COLUMN guliangiam.login_challenges.user_agent IS 'User-Agent header at challenge creation';
COMMENT ON COLUMN guliangiam.login_challenges.metadata IS 'Attribute bag for factor-specific state (e.g. TOTP window, WebAuthn cred ID)';
COMMENT ON COLUMN guliangiam.login_challenges.expires_at IS 'Expiration timestamp for the pending MFA login challenge.';
COMMENT ON COLUMN guliangiam.login_challenges.resolved_at IS 'Timestamp when challenge transitioned to approved/rejected state';
ALTER TABLE guliangiam.login_challenges ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.login_challenges FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'login_challenges'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.login_challenges
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.webauthn_credentials (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    credential_id BYTEA NOT NULL,
    public_key BYTEA NOT NULL,
    attestation_type VARCHAR(255) NOT NULL DEFAULT '',
    transport VARCHAR(255)[] NOT NULL DEFAULT '{}',
    sign_count BIGINT NOT NULL DEFAULT 0,
    name VARCHAR(255) NOT NULL DEFAULT '',
    aaguid UUID,
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_webauthn_credentials_status CHECK (status IN ('active', 'revoked', 'deactivated')),
    CONSTRAINT fk_webauthn_credentials_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_user_id ON guliangiam.webauthn_credentials (user_id);
CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_aaguid ON guliangiam.webauthn_credentials (aaguid) WHERE aaguid IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_webauthn_credentials_credential_id ON guliangiam.webauthn_credentials (credential_id);
ALTER TABLE guliangiam.webauthn_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.webauthn_credentials FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'webauthn_credentials'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.webauthn_credentials
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.webauthn_challenges (
    id UUID NOT NULL PRIMARY KEY,
    challenge_hash TEXT NOT NULL,
    ceremony_type TEXT NOT NULL,
    user_id UUID NOT NULL,
    organization_id UUID,
    relying_party_id TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_webauthn_challenges_ceremony_type CHECK (ceremony_type IN ('registration', 'authentication', 'web3_authentication')),
    CONSTRAINT fk_webauthn_challenges_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE,
    CONSTRAINT fk_webauthn_challenges_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_webauthn_challenges_user_id ON guliangiam.webauthn_challenges (user_id);
CREATE INDEX IF NOT EXISTS idx_webauthn_challenges_expires_at ON guliangiam.webauthn_challenges (expires_at);
COMMENT ON COLUMN guliangiam.webauthn_challenges.challenge_hash IS 'SHA-256 (hex) of the challenge issued by begin. The plaintext challenge is never persisted, so a database leak cannot be replayed as a valid ceremony.';
COMMENT ON COLUMN guliangiam.webauthn_challenges.ceremony_type IS 'registration or authentication — the two ceremony kinds share one table but must not be interchangeable at finish. No TEXT default (check-schema-defaults.py freezes the legacy count of TEXT defaults; the service always sets this column).';
COMMENT ON COLUMN guliangiam.webauthn_challenges.relying_party_id IS 'RP ID captured at begin; finish verifies the assertion against this value rather than against whatever the client sends. No TEXT default for the same reason as ceremony_type.';
COMMENT ON COLUMN guliangiam.webauthn_challenges.consumed_at IS 'Set exactly once by the atomic consume UPDATE. NULL means pending — the one-time guarantee is the RowsAffected=0 check on that conditional update.';
ALTER TABLE guliangiam.webauthn_challenges ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.webauthn_challenges FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'webauthn_challenges'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.webauthn_challenges
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.trusted_devices (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    user_id UUID NOT NULL,
    device_credential_id TEXT NOT NULL,
    device_public_key BYTEA NOT NULL,
    ua_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_trusted_devices_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_trusted_devices_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_trusted_devices_user_active ON guliangiam.trusted_devices (user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_trusted_devices_org_user ON guliangiam.trusted_devices (organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_trusted_devices_expires_at ON guliangiam.trusted_devices (expires_at);
COMMENT ON COLUMN guliangiam.trusted_devices.id IS 'Trusted device primary key (UUIDv7 generated by the application)';
COMMENT ON COLUMN guliangiam.trusted_devices.organization_id IS 'Tenant that owns the trust record; also the RLS anchor';
COMMENT ON COLUMN guliangiam.trusted_devices.user_id IS 'User this device is trusted for';
COMMENT ON COLUMN guliangiam.trusted_devices.device_credential_id IS 'SHA-256 digest of the client device credential, stored as 64 lowercase hex characters; the plaintext credential is never persisted';
COMMENT ON COLUMN guliangiam.trusted_devices.device_public_key IS 'Device public key (PKIX DER) used to verify the trust ceremony signature';
COMMENT ON COLUMN guliangiam.trusted_devices.ua_hash IS 'SHA-256 hex of the User-Agent observed at registration; device trust is bound to the client profile, never to an IP address';
COMMENT ON COLUMN guliangiam.trusted_devices.last_seen_at IS 'Last time the device presented a valid trust proof';
COMMENT ON COLUMN guliangiam.trusted_devices.expires_at IS 'Hard expiration (created_at + 30 days); expired rows are no longer trusted';
COMMENT ON COLUMN guliangiam.trusted_devices.revoked_at IS 'Revocation timestamp; NULL while the device is trusted';
ALTER TABLE guliangiam.trusted_devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.trusted_devices FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'trusted_devices'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.trusted_devices
            USING (guliangiam.is_super_admin() OR (organization_id = guliangiam.current_organization_id() AND EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id())))
            WITH CHECK (guliangiam.is_super_admin() OR (organization_id = guliangiam.current_organization_id() AND EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id())));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.remember_me_families (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    user_id UUID NOT NULL,
    application_id UUID,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_remember_me_families_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_remember_me_families_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE,
    CONSTRAINT fk_remember_me_families_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_remember_me_families_user_active ON guliangiam.remember_me_families (user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_remember_me_families_org_user ON guliangiam.remember_me_families (organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_remember_me_families_expires_at ON guliangiam.remember_me_families (expires_at);
COMMENT ON COLUMN guliangiam.remember_me_families.id IS 'Remember-me family primary key (UUID)';
COMMENT ON COLUMN guliangiam.remember_me_families.organization_id IS 'Tenant that owns the family';
COMMENT ON COLUMN guliangiam.remember_me_families.user_id IS 'User the family restores';
COMMENT ON COLUMN guliangiam.remember_me_families.application_id IS 'Application the family was issued for';
COMMENT ON COLUMN guliangiam.remember_me_families.expires_at IS 'Hard family expiration (issued_at + 30 days)';
COMMENT ON COLUMN guliangiam.remember_me_families.revoked_at IS 'Revocation timestamp; reuse detection revokes the whole family';
COMMENT ON COLUMN guliangiam.remember_me_families.revoke_reason IS 'logout / reuse_detected / admin / user';
ALTER TABLE guliangiam.remember_me_families ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.remember_me_families FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'remember_me_families'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.remember_me_families
            USING (guliangiam.is_super_admin() OR (organization_id = guliangiam.current_organization_id() AND EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id())))
            WITH CHECK (guliangiam.is_super_admin() OR (organization_id = guliangiam.current_organization_id() AND EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id())));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.remember_me_tokens (
    id UUID NOT NULL PRIMARY KEY,
    family_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    token_hash TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    consumed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_remember_me_tokens_hash UNIQUE (token_hash),
    CONSTRAINT fk_remember_me_tokens_family_id_remember_me_families FOREIGN KEY (family_id) REFERENCES guliangiam.remember_me_families(id) ON DELETE CASCADE,
    CONSTRAINT fk_remember_me_tokens_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_remember_me_tokens_family ON guliangiam.remember_me_tokens (family_id);
CREATE INDEX IF NOT EXISTS idx_remember_me_tokens_expires_at ON guliangiam.remember_me_tokens (expires_at) WHERE consumed_at IS NULL;
COMMENT ON COLUMN guliangiam.remember_me_tokens.id IS 'Remember-me token primary key (UUID)';
COMMENT ON COLUMN guliangiam.remember_me_tokens.family_id IS 'Rotating family this token belongs to';
COMMENT ON COLUMN guliangiam.remember_me_tokens.organization_id IS 'Tenant that owns the token (RLS anchor)';
COMMENT ON COLUMN guliangiam.remember_me_tokens.token_hash IS 'SHA-256 hex of the plaintext token; plaintext is returned only once';
COMMENT ON COLUMN guliangiam.remember_me_tokens.token_prefix IS 'Non-sensitive prefix for management UIs';
COMMENT ON COLUMN guliangiam.remember_me_tokens.consumed_at IS 'Single-use consumption timestamp; non-NULL means already rotated';
COMMENT ON COLUMN guliangiam.remember_me_tokens.expires_at IS 'Token expiration (mirrors the family expiration)';
ALTER TABLE guliangiam.remember_me_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.remember_me_tokens FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'remember_me_tokens'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.remember_me_tokens
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：50_protocol.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.authorization_codes (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    application_id UUID NOT NULL,
    code_hash TEXT NOT NULL,
    code_prefix VARCHAR(255) NOT NULL DEFAULT '',
    redirect_uri VARCHAR(255) NOT NULL DEFAULT '',
    scopes VARCHAR(255)[] NOT NULL DEFAULT '{}',
    status VARCHAR(255) NOT NULL DEFAULT '',
    sid VARCHAR(255) NOT NULL DEFAULT '',
    nonce VARCHAR(255) NOT NULL DEFAULT '',
    resource VARCHAR(255) NOT NULL DEFAULT '',
    code_challenge VARCHAR(255) NOT NULL DEFAULT '',
    code_challenge_method VARCHAR(255) NOT NULL DEFAULT '',
    auth_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_authorization_codes_hash UNIQUE (code_hash),
    CONSTRAINT fk_authorization_codes_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_authorization_codes_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_authorization_codes_expires_at ON guliangiam.authorization_codes (expires_at);
ALTER TABLE guliangiam.authorization_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.authorization_codes FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'authorization_codes'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.authorization_codes
            USING (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.consents (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    username VARCHAR(255) NOT NULL DEFAULT '',
    organization_id UUID,
    organization_slug VARCHAR(255) NOT NULL DEFAULT '',
    application_id UUID NOT NULL,
    application_slug VARCHAR(255) NOT NULL DEFAULT '',
    scopes VARCHAR(255)[] NOT NULL DEFAULT '{}',
    granted_scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_consents_user_application UNIQUE (user_id, application_id),
    CONSTRAINT fk_consents_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_consents_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_consents_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_consents_application_active ON guliangiam.consents (application_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_consents_user_active ON guliangiam.consents (user_id, application_id) WHERE revoked_at IS NULL;
ALTER TABLE guliangiam.consents ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.consents FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'consents'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.consents
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.device_auth_codes (
    id UUID NOT NULL PRIMARY KEY,
    device_code TEXT NOT NULL,
    user_code TEXT NOT NULL,
    client_id VARCHAR(255) NOT NULL DEFAULT '',
    scope VARCHAR(255) NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    poll_interval_sec INT NOT NULL DEFAULT 5,
    verified BOOLEAN NOT NULL DEFAULT false,
    verified_at TIMESTAMPTZ,
    user_id UUID,
    username VARCHAR(255) NOT NULL DEFAULT '',
    organization_id UUID,
    last_polled_at TIMESTAMPTZ,
    slow_down_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_device_auth_codes_device_code UNIQUE (device_code),
    CONSTRAINT uq_device_auth_codes_user_code UNIQUE (user_code),
    CONSTRAINT fk_device_auth_codes_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_device_auth_codes_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL
);
ALTER TABLE guliangiam.device_auth_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.device_auth_codes FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'device_auth_codes'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.device_auth_codes
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.qr_code_sessions (
    id UUID NOT NULL PRIMARY KEY,
    code_id TEXT NOT NULL,
    code VARCHAR(255) NOT NULL DEFAULT '',
    organization_id UUID NOT NULL,
    application_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    user_id UUID,
    username VARCHAR(255) NOT NULL DEFAULT '',
    ip_address VARCHAR(255) NOT NULL DEFAULT '',
    user_agent VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_qr_code_sessions_code_id UNIQUE (code_id),
    CONSTRAINT uq_qr_code_sessions_code UNIQUE (code),
    CONSTRAINT fk_qr_code_sessions_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_qr_code_sessions_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_qr_code_sessions_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE ON UPDATE CASCADE
);
ALTER TABLE guliangiam.qr_code_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.qr_code_sessions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'qr_code_sessions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.qr_code_sessions
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.web3_nonces (
    id UUID NOT NULL PRIMARY KEY,
    nonce TEXT NOT NULL,
    wallet_address VARCHAR(255) NOT NULL DEFAULT '',
    organization_id UUID NOT NULL,
    application_id UUID,
    chain_id VARCHAR(255) NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    consumed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_web3_nonces_nonce UNIQUE (nonce),
    CONSTRAINT fk_web3_nonces_application_id_applications FOREIGN KEY (application_id) REFERENCES guliangiam.applications(id) ON DELETE CASCADE,
    CONSTRAINT fk_web3_nonces_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.web3_nonces ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.web3_nonces FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'web3_nonces'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.web3_nonces
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.face_id_sessions (
    id UUID NOT NULL PRIMARY KEY,
    session_id TEXT NOT NULL,
    challenge VARCHAR(255) NOT NULL DEFAULT '',
    user_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT false,
    liveness_ok BOOLEAN NOT NULL DEFAULT false,
    attempt_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_face_id_sessions_session_id UNIQUE (session_id),
    CONSTRAINT fk_face_id_sessions_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.face_id_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.face_id_sessions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'face_id_sessions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.face_id_sessions
            USING (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.cas_tickets (
    id UUID NOT NULL PRIMARY KEY,
    ticket_id TEXT NOT NULL,
    ticket_type VARCHAR(64) NOT NULL DEFAULT 'service',
    user_id UUID,
    username VARCHAR(255) NOT NULL DEFAULT '',
    service VARCHAR(255) NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    pgt_iou TEXT,
    pgt_iou_expires_at TIMESTAMPTZ,
    pgt_iou_used BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT uq_cas_tickets_ticket_id UNIQUE (ticket_id),
    CONSTRAINT uq_cas_tickets_pgt_iou UNIQUE (pgt_iou),
    CONSTRAINT fk_cas_tickets_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.cas_tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.cas_tickets FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'cas_tickets'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.cas_tickets
            USING (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.kerberos_tickets (
    id UUID NOT NULL PRIMARY KEY,
    ticket_id TEXT NOT NULL,
    user_id UUID,
    username VARCHAR(255) NOT NULL DEFAULT '',
    service VARCHAR(255) NOT NULL DEFAULT '',
    realm VARCHAR(255) NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_kerberos_tickets_ticket_id UNIQUE (ticket_id),
    CONSTRAINT fk_kerberos_tickets_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.kerberos_tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.kerberos_tickets FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'kerberos_tickets'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.kerberos_tickets
            USING (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：60_audit.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.records (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    actor_id UUID,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id VARCHAR(255) NOT NULL DEFAULT '',
    ip INET,
    user_agent VARCHAR(255) NOT NULL DEFAULT '',
    surface VARCHAR(64) NOT NULL DEFAULT 'management',
    request_id VARCHAR(255) NOT NULL DEFAULT '',
    trace_id VARCHAR(255) NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    chain_id VARCHAR(255) NOT NULL DEFAULT '',
    chain_seq BIGINT NOT NULL DEFAULT 0,
    prev_hash BYTEA,
    entry_hash BYTEA,
    chain_status VARCHAR(16) NOT NULL DEFAULT 'pending',
    chain_algo VARCHAR(8) NOT NULL DEFAULT 'sha256',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_records_chain_seq_nonneg CHECK (chain_seq >= 0),
    CONSTRAINT ck_records_chain_hash_len CHECK ((entry_hash IS NULL AND prev_hash IS NULL) OR (octet_length(entry_hash) = 32 AND (prev_hash IS NULL OR octet_length(prev_hash) = 32))),
    CONSTRAINT ck_records_chain_status CHECK (chain_status IN ('pending', 'chained', 'anchored', 'broken')),
    CONSTRAINT ck_records_chain_algo CHECK (chain_algo IN ('sha256', 'sm3')),
    CONSTRAINT uq_records_chain_seq UNIQUE (organization_id, chain_id, chain_seq),
    CONSTRAINT fk_records_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_records_actor_id_users FOREIGN KEY (actor_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_records_org_created_at_desc ON guliangiam.records (organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_records_action ON guliangiam.records (action);
CREATE INDEX IF NOT EXISTS idx_audit_records_resource_type ON guliangiam.records (resource_type);
CREATE INDEX IF NOT EXISTS idx_audit_records_actor_id ON guliangiam.records (actor_id);
CREATE INDEX IF NOT EXISTS idx_records_org_action_created_at ON guliangiam.records (organization_id, action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_records_action_created_at_date ON guliangiam.records (action, created_at);
CREATE INDEX IF NOT EXISTS idx_records_surface_created ON guliangiam.records (surface, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_records_request_id ON guliangiam.records (request_id) WHERE request_id <> '';
CREATE INDEX IF NOT EXISTS idx_records_trace_id ON guliangiam.records (trace_id) WHERE trace_id <> '';
CREATE INDEX IF NOT EXISTS idx_records_created_at_desc ON guliangiam.records (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_records_user_created ON guliangiam.records (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_records_actor_id_created_at ON guliangiam.records (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_records_action_created_at_desc ON guliangiam.records (action, created_at DESC);
COMMENT ON COLUMN guliangiam.records.id IS 'Audit record primary key (UUID)';
COMMENT ON COLUMN guliangiam.records.organization_id IS 'Tenant org (RLS predicate key)';
COMMENT ON COLUMN guliangiam.records.actor_id IS 'User or service account UUID that performed the action; NULL for system events';
COMMENT ON COLUMN guliangiam.records.action IS 'Verb-style action name (create / update / delete / login / token_revoke / ...)';
COMMENT ON COLUMN guliangiam.records.resource_type IS 'Domain entity type: users / roles / orders / ...';
COMMENT ON COLUMN guliangiam.records.resource_id IS 'Entity UUID (or string ID) the action targeted';
COMMENT ON COLUMN guliangiam.records.ip IS 'Client IP observed at the time of the action';
COMMENT ON COLUMN guliangiam.records.user_agent IS 'User-Agent header captured with the request';
COMMENT ON COLUMN guliangiam.records.surface IS 'Interface where the action originated: management / user_portal / api / cli / system';
COMMENT ON COLUMN guliangiam.records.request_id IS 'Incoming request correlation ID (X-Request-ID header); empty when unavailable';
COMMENT ON COLUMN guliangiam.records.trace_id IS 'OpenTelemetry trace ID for distributed tracing correlation';
COMMENT ON COLUMN guliangiam.records.metadata IS 'Action-specific payload (diff fields, risk scores, geo info, etc.)';
COMMENT ON COLUMN guliangiam.records.chain_id IS 'Audit chain identifier; links records into tamper-evident append-only chains per (org, surface, period)';
COMMENT ON COLUMN guliangiam.records.chain_seq IS 'Monotonic sequence within the chain; used to verify append order';
COMMENT ON COLUMN guliangiam.records.prev_hash IS 'Previous entry_hash in the chain (NULL for the first entry); digest algorithm per chain_algo (sha256 or sm3)';
COMMENT ON COLUMN guliangiam.records.entry_hash IS 'Tamper-evident digest of the canonical record body plus prev_hash; algorithm per chain_algo (sha256 or sm3)';
COMMENT ON COLUMN guliangiam.records.chain_status IS 'Chain build state: worker advances pending -> chained -> anchored asynchronously; broken marks a verification failure';
COMMENT ON COLUMN guliangiam.records.chain_algo IS 'Hash algorithm backing prev_hash / entry_hash: sha256 or sm3';
ALTER TABLE guliangiam.records ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.records FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'records'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.records
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.secret_rotation_audit (
    id UUID NOT NULL PRIMARY KEY,
    provider_id UUID NOT NULL,
    secret_type TEXT NOT NULL,
    old_version INT NOT NULL,
    new_version INT NOT NULL,
    rotated_by UUID,
    rotated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_secret_rotation_audit_provider_id_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.providers(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_secret_rotation_audit_provider ON guliangiam.secret_rotation_audit (provider_id);
CREATE INDEX IF NOT EXISTS idx_secret_rotation_audit_rotated_at ON guliangiam.secret_rotation_audit (rotated_at);
ALTER TABLE guliangiam.secret_rotation_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.secret_rotation_audit FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'secret_rotation_audit'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.secret_rotation_audit
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.providers p WHERE p.id = provider_id AND p.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.providers p WHERE p.id = provider_id AND p.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.audit_chain_anchors (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    chain_id TEXT NOT NULL,
    anchor_seq BIGINT NOT NULL,
    entry_hash BYTEA NOT NULL,
    proof VARCHAR(255) NOT NULL DEFAULT '',
    anchored_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    anchor_source VARCHAR(64) NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_audit_chain_anchors_hash_len CHECK (octet_length(entry_hash) = 32),
    CONSTRAINT uq_audit_chain_anchors_org_chain_seq UNIQUE (organization_id, chain_id, anchor_seq),
    CONSTRAINT fk_audit_chain_anchors_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_audit_chain_anchors_anchored_at ON guliangiam.audit_chain_anchors (anchored_at DESC);
ALTER TABLE guliangiam.audit_chain_anchors ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.audit_chain_anchors FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'audit_chain_anchors'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.audit_chain_anchors
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.authorization_chain_state (
    id INTEGER NOT NULL PRIMARY KEY,
    last_hash VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_authorization_chain_state_singleton CHECK (id = 1)
);
ALTER TABLE guliangiam.authorization_chain_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.authorization_chain_state FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'authorization_chain_state'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.authorization_chain_state
            USING (guliangiam.is_super_admin())
            WITH CHECK (guliangiam.is_super_admin());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.cross_tenant_operations (
    id UUID NOT NULL PRIMARY KEY,
    scope_id UUID,
    actor_id UUID NOT NULL,
    source_tenant UUID,
    target_tenant UUID NOT NULL,
    resource_type TEXT NOT NULL,
    action TEXT NOT NULL,
    risk_level VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    approved_by UUID,
    approved_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    trace_id VARCHAR(255),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_cross_tenant_operations_risk CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT ck_cross_tenant_operations_status CHECK (status IN ('pending', 'approved', 'rejected', 'used', 'expired')),
    CONSTRAINT fk_cross_tenant_operations_scope_id_platform_admin_scopes FOREIGN KEY (scope_id) REFERENCES guliangiam.platform_admin_scopes(id) ON DELETE SET NULL,
    CONSTRAINT fk_cross_tenant_operations_actor_id_users FOREIGN KEY (actor_id) REFERENCES guliangiam.users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_cross_tenant_operations_source_tenant_organizations FOREIGN KEY (source_tenant) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT,
    CONSTRAINT fk_cross_tenant_operations_target_tenant_organizations FOREIGN KEY (target_tenant) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT,
    CONSTRAINT fk_cross_tenant_operations_approved_by_platform_admins FOREIGN KEY (approved_by) REFERENCES guliangiam.platform_admins(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_cross_tenant_operations_actor ON guliangiam.cross_tenant_operations (actor_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cross_tenant_operations_scope ON guliangiam.cross_tenant_operations (scope_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cross_tenant_operations_target ON guliangiam.cross_tenant_operations (target_tenant, expires_at);
ALTER TABLE guliangiam.cross_tenant_operations ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.cross_tenant_operations FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'cross_tenant_operations'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.cross_tenant_operations
            USING (guliangiam.is_super_admin())
            WITH CHECK (guliangiam.is_super_admin());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.authorization_audit_records (
    id UUID NOT NULL PRIMARY KEY,
    actor_id UUID NOT NULL,
    actor_type VARCHAR(32) NOT NULL,
    source_tenant UUID,
    target_tenant UUID,
    resource_type TEXT NOT NULL,
    action TEXT NOT NULL,
    risk_level VARCHAR(16) NOT NULL,
    scope_id UUID,
    ticket_id UUID,
    decision VARCHAR(16) NOT NULL,
    reason TEXT,
    prev_hash VARCHAR(64),
    hash VARCHAR(64) NOT NULL,
    trace_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_authorization_audit_actor_type CHECK (actor_type IN ('user', 'platform_admin', 'service_account', 'system')),
    CONSTRAINT ck_authorization_audit_risk CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT ck_authorization_audit_decision CHECK (decision IN ('allow', 'deny')),
    CONSTRAINT ck_authorization_audit_hash CHECK (char_length(hash) = 64 AND (prev_hash IS NULL OR char_length(prev_hash) = 64)),
    CONSTRAINT fk_authorization_audit_records_source_tenant_organizations FOREIGN KEY (source_tenant) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT,
    CONSTRAINT fk_authorization_audit_records_target_tenant_organizations FOREIGN KEY (target_tenant) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT,
    CONSTRAINT fk_authorization_audit_records_scope_id_platform_admin_scopes FOREIGN KEY (scope_id) REFERENCES guliangiam.platform_admin_scopes(id) ON DELETE SET NULL,
    CONSTRAINT fk_authorization_audit_records_ticket_id_cross_tenant__dc79ba9b FOREIGN KEY (ticket_id) REFERENCES guliangiam.cross_tenant_operations(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_authorization_audit_created ON guliangiam.authorization_audit_records (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_authorization_audit_actor ON guliangiam.authorization_audit_records (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_authorization_audit_tenants ON guliangiam.authorization_audit_records (source_tenant, target_tenant, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_authorization_audit_ticket ON guliangiam.authorization_audit_records (ticket_id);
ALTER TABLE guliangiam.authorization_audit_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.authorization_audit_records FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'authorization_audit_records'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.authorization_audit_records
            USING (guliangiam.is_super_admin())
            WITH CHECK (guliangiam.is_super_admin());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：70_commerce.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.products (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    image VARCHAR(500) DEFAULT '',
    detail VARCHAR(2000) DEFAULT '',
    description VARCHAR(500) DEFAULT '',
    tag VARCHAR(255) DEFAULT '',
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    price DECIMAL(19, 4) NOT NULL DEFAULT 0,
    quantity INTEGER NOT NULL DEFAULT 0,
    sold INTEGER NOT NULL DEFAULT 0,
    is_recharge BOOLEAN NOT NULL DEFAULT false,
    recharge_options JSONB DEFAULT '[]',
    disable_custom_recharge BOOLEAN NOT NULL DEFAULT false,
    providers JSONB DEFAULT '[]',
    success_url VARCHAR(1000) DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_products_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_products_status CHECK (status IN ('draft', 'active', 'discontinued')),
    CONSTRAINT fk_products_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.products ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.products FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'products'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.products
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.orders (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    user_id UUID,
    products JSONB DEFAULT '[]',
    product_infos JSONB DEFAULT '[]',
    payment VARCHAR(255) DEFAULT '',
    price DECIMAL(19, 4) NOT NULL DEFAULT 0,
    currency VARCHAR(3) DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'created',
    message VARCHAR(2000) DEFAULT '',
    payment_url VARCHAR(500) DEFAULT '',
    trade_no VARCHAR(255) DEFAULT '',
    paid_at TIMESTAMPTZ,
    coupon_name VARCHAR(255) DEFAULT '',
    coupon_discount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    idempotency_key TEXT,
    CONSTRAINT uq_orders_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_orders_status CHECK (status IN ('created', 'payment_pending', 'paid', 'completed', 'failed', 'cancelled', 'refunded', 'closed')),
    CONSTRAINT ck_orders_paid_at CHECK ((status IN ('paid', 'completed', 'refunded', 'closed')) = (paid_at IS NOT NULL)),
    CONSTRAINT fk_orders_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_orders_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_orders_idempotency_key ON guliangiam.orders (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_orders_org_user ON guliangiam.orders (organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON guliangiam.orders (status);
CREATE INDEX IF NOT EXISTS idx_orders_created_at_desc ON guliangiam.orders (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_trade_no ON guliangiam.orders (trade_no);
CREATE INDEX IF NOT EXISTS idx_orders_org_status ON guliangiam.orders (organization_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON guliangiam.orders (user_id);
ALTER TABLE guliangiam.orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.orders FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'orders'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.orders
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.payments (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    provider VARCHAR(255) DEFAULT '',
    type VARCHAR(100) DEFAULT '',
    products JSONB DEFAULT '[]',
    product_name VARCHAR(255) DEFAULT '',
    product_display_name VARCHAR(255) DEFAULT '',
    detail VARCHAR(500) DEFAULT '',
    currency VARCHAR(3) DEFAULT '',
    price DECIMAL(19, 4) NOT NULL DEFAULT 0,
    user_id UUID,
    person_name VARCHAR(255) DEFAULT '',
    person_id_card VARCHAR(255) DEFAULT '',
    person_email VARCHAR(255) DEFAULT '',
    person_phone VARCHAR(255) DEFAULT '',
    invoice_type VARCHAR(100) DEFAULT '',
    invoice_title VARCHAR(255) DEFAULT '',
    invoice_tax_id VARCHAR(255) DEFAULT '',
    invoice_remark VARCHAR(500) DEFAULT '',
    invoice_url VARCHAR(500) DEFAULT '',
    order_id UUID,
    out_order_id VARCHAR(255) DEFAULT '',
    pay_url VARCHAR(2000) DEFAULT '',
    success_url VARCHAR(2000) DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'created',
    message VARCHAR(2000) DEFAULT '',
    channel_order_no VARCHAR(255) DEFAULT '',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    refunded_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    refunding_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    last_error_code VARCHAR(255) DEFAULT '',
    last_error_message VARCHAR(500) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    idempotency_key TEXT,
    CONSTRAINT uq_payments_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_payments_status CHECK (status IN ('created', 'processing', 'succeeded', 'failed', 'cancelled', 'refunded')),
    CONSTRAINT fk_payments_order_id_orders FOREIGN KEY (order_id) REFERENCES guliangiam.orders(id) ON DELETE SET NULL,
    CONSTRAINT fk_payments_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_payments_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_idempotency_key ON guliangiam.payments (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_org_status ON guliangiam.payments (organization_id, status);
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON guliangiam.payments (user_id);
ALTER TABLE guliangiam.payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.payments FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'payments'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.payments
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.transactions (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    user_id UUID,
    order_id UUID,
    payment_id UUID,
    transaction_type TEXT NOT NULL,
    amount DECIMAL(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    provider_type VARCHAR(255) NOT NULL DEFAULT '',
    provider_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_transactions_status CHECK (status IN ('pending', 'completed', 'failed', 'cancelled')),
    CONSTRAINT ck_transactions_type CHECK (transaction_type IN ('payment', 'refund', 'cancellation')),
    CONSTRAINT fk_transactions_order_id_orders FOREIGN KEY (order_id) REFERENCES guliangiam.orders(id) ON DELETE SET NULL,
    CONSTRAINT fk_transactions_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_transactions_payment_id_payments FOREIGN KEY (payment_id) REFERENCES guliangiam.payments(id) ON DELETE SET NULL,
    CONSTRAINT fk_transactions_provider_id_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.providers(id) ON DELETE SET NULL,
    CONSTRAINT fk_transactions_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_transactions_org ON guliangiam.transactions (organization_id);
CREATE INDEX IF NOT EXISTS idx_transactions_order ON guliangiam.transactions (order_id);
CREATE INDEX IF NOT EXISTS idx_transactions_payment ON guliangiam.transactions (payment_id);
CREATE INDEX IF NOT EXISTS idx_transactions_user ON guliangiam.transactions (user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON guliangiam.transactions (status);
ALTER TABLE guliangiam.transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.transactions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'transactions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.transactions
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.plans (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    description VARCHAR(500) DEFAULT '',
    price DECIMAL(19, 4) NOT NULL DEFAULT 0,
    currency VARCHAR(3) DEFAULT '',
    period VARCHAR(50) DEFAULT '',
    product VARCHAR(255) DEFAULT '',
    payment_providers JSONB DEFAULT '[]',
    status VARCHAR(64) NOT NULL DEFAULT 'draft',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    is_exclusive BOOLEAN NOT NULL DEFAULT false,
    role VARCHAR(255) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_plans_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_plans_status CHECK (status IN ('draft', 'active', 'archived')),
    CONSTRAINT ck_plans_period CHECK (period IS NULL OR period IN ('daily', 'weekly', 'monthly', 'quarterly', 'yearly', 'lifetime')),
    CONSTRAINT fk_plans_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.plans FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'plans'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.plans
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.pricings (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    description VARCHAR(500) DEFAULT '',
    plans JSONB DEFAULT '[]',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    trial_duration INTEGER NOT NULL DEFAULT 0,
    application VARCHAR(255) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_pricings_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_pricings_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.pricings ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.pricings FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'pricings'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.pricings
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.subscriptions (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    user_id UUID,
    plan_id UUID,
    pricing_id UUID,
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    expire_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_subscriptions_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_subscriptions_status CHECK (status IN ('pending', 'trialing', 'active', 'past_due', 'expired', 'cancelled')),
    CONSTRAINT fk_subscriptions_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_subscriptions_plan_id_plans FOREIGN KEY (plan_id) REFERENCES guliangiam.plans(id) ON DELETE SET NULL,
    CONSTRAINT fk_subscriptions_pricing_id_pricings FOREIGN KEY (pricing_id) REFERENCES guliangiam.pricings(id) ON DELETE SET NULL,
    CONSTRAINT fk_subscriptions_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON guliangiam.subscriptions (user_id);
ALTER TABLE guliangiam.subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.subscriptions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'subscriptions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.subscriptions
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.coupons (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    description VARCHAR(255) NOT NULL DEFAULT '',
    code VARCHAR(255) NOT NULL DEFAULT '',
    discount_type VARCHAR(64) NOT NULL DEFAULT 'percentage',
    discount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    max_discount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    scope VARCHAR(255) NOT NULL DEFAULT '',
    products JSONB NOT NULL DEFAULT '[]',
    users JSONB NOT NULL DEFAULT '[]',
    quantity INT NOT NULL DEFAULT 0,
    used_count INT NOT NULL DEFAULT 0,
    max_usage_per_user INT NOT NULL DEFAULT 0,
    start_time VARCHAR(255) NOT NULL DEFAULT '',
    expire_time VARCHAR(255) NOT NULL DEFAULT '',
    min_order_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_coupons_code UNIQUE (code),
    CONSTRAINT ck_coupons_status CHECK (status IN ('active', 'disabled', 'expired')),
    CONSTRAINT fk_coupons_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_coupons_org ON guliangiam.coupons (organization_id);
ALTER TABLE guliangiam.coupons ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.coupons FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'coupons'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.coupons
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.entries (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    provider VARCHAR(255) NOT NULL DEFAULT '',
    application VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(255) NOT NULL DEFAULT '',
    client_ip VARCHAR(255) NOT NULL DEFAULT '',
    user_agent VARCHAR(255) NOT NULL DEFAULT '',
    message VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_entries_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_entries_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_entries_type ON guliangiam.entries (type);
CREATE INDEX IF NOT EXISTS idx_entries_application ON guliangiam.entries (application);
ALTER TABLE guliangiam.entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.entries FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'entries'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.entries
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.coupon_usage (
    id UUID NOT NULL PRIMARY KEY,
    coupon_id UUID NOT NULL,
    user_id UUID NOT NULL,
    order_id UUID,
    used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_coupon_usage_coupon_id_coupons FOREIGN KEY (coupon_id) REFERENCES guliangiam.coupons(id) ON DELETE CASCADE,
    CONSTRAINT fk_coupon_usage_order_id_orders FOREIGN KEY (order_id) REFERENCES guliangiam.orders(id) ON DELETE SET NULL,
    CONSTRAINT fk_coupon_usage_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_cu_coupon_id ON guliangiam.coupon_usage (coupon_id);
CREATE INDEX IF NOT EXISTS idx_cu_user_id ON guliangiam.coupon_usage (user_id);
CREATE INDEX IF NOT EXISTS idx_cu_order_id ON guliangiam.coupon_usage (order_id);
ALTER TABLE guliangiam.coupon_usage ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.coupon_usage FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'coupon_usage'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.coupon_usage
            USING (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.pay_channel_configs (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    provider_id UUID,
    channel VARCHAR(50) NOT NULL,
    environment VARCHAR(20) NOT NULL DEFAULT 'production',
    config_sealed TEXT NOT NULL,
    config_fingerprint VARCHAR(16) NOT NULL,
    config_set JSONB DEFAULT '[]',
    version_no INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    test_passed_at TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_pay_channel_configs_status CHECK (status IN ('draft', 'testing', 'active', 'disabled')),
    CONSTRAINT ck_pay_channel_configs_env CHECK (environment IN ('sandbox', 'production')),
    CONSTRAINT ck_pay_channel_configs_channel CHECK (channel IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy')),
    CONSTRAINT fk_pay_channel_configs_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_pay_channel_configs_provider_id_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.providers(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_pay_channel_configs_org_channel_env_version ON guliangiam.pay_channel_configs (organization_id, channel, environment, version_no) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_pay_channel_configs_provider_version ON guliangiam.pay_channel_configs (provider_id, version_no) WHERE provider_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_pay_channel_configs_one_active ON guliangiam.pay_channel_configs (provider_id) WHERE provider_id IS NOT NULL AND status = 'active' AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pay_channel_configs_org ON guliangiam.pay_channel_configs (organization_id);
CREATE INDEX IF NOT EXISTS idx_pay_channel_configs_status ON guliangiam.pay_channel_configs (status);
CREATE INDEX IF NOT EXISTS idx_pay_channel_configs_channel_env ON guliangiam.pay_channel_configs (channel, environment);
ALTER TABLE guliangiam.pay_channel_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.pay_channel_configs FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'pay_channel_configs'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.pay_channel_configs
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.pay_callback_receipts (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    channel VARCHAR(50) NOT NULL,
    payload_sha256 VARCHAR(64) NOT NULL,
    payload BYTEA NOT NULL,
    headers JSONB DEFAULT '{}',
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    client_ip VARCHAR(64),
    process_state VARCHAR(32) NOT NULL DEFAULT 'received',
    event_key VARCHAR(255),
    out_trade_no VARCHAR(255),
    raw_event JSONB,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_receipts_process_state CHECK (process_state IN ('received', 'verified', 'rejected', 'duplicated', 'normalized', 'accepted')),
    CONSTRAINT ck_receipts_channel CHECK (channel IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy')),
    CONSTRAINT uq_receipts_channel_sha256 UNIQUE (channel, payload_sha256),
    CONSTRAINT uq_receipts_channel_event_key UNIQUE (channel, event_key),
    CONSTRAINT fk_pay_callback_receipts_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_receipts_org ON guliangiam.pay_callback_receipts (organization_id);
CREATE INDEX IF NOT EXISTS idx_receipts_out_trade_no ON guliangiam.pay_callback_receipts (out_trade_no);
CREATE INDEX IF NOT EXISTS idx_receipts_process_state ON guliangiam.pay_callback_receipts (process_state);
CREATE INDEX IF NOT EXISTS idx_receipts_received_at ON guliangiam.pay_callback_receipts (received_at);
ALTER TABLE guliangiam.pay_callback_receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.pay_callback_receipts FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'pay_callback_receipts'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.pay_callback_receipts
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.pay_refunds (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    payment_id UUID NOT NULL,
    out_refund_no VARCHAR(255) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    amount DECIMAL(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    state VARCHAR(32) NOT NULL DEFAULT 'requested',
    reason VARCHAR(500) DEFAULT '',
    channel_refund_no VARCHAR(255),
    error_code VARCHAR(100),
    error_message VARCHAR(2000),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_pay_refunds_state CHECK (state IN ('requested', 'processing', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT ck_pay_refunds_channel CHECK (channel IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy')),
    CONSTRAINT fk_pay_refunds_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_pay_refunds_payment_id_payments FOREIGN KEY (payment_id) REFERENCES guliangiam.payments(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_pay_refunds_channel_out_no ON guliangiam.pay_refunds (channel, out_refund_no) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pay_refunds_payment ON guliangiam.pay_refunds (payment_id);
CREATE INDEX IF NOT EXISTS idx_pay_refunds_state ON guliangiam.pay_refunds (state);
ALTER TABLE guliangiam.pay_refunds ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.pay_refunds FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'pay_refunds'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.pay_refunds
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.pay_attempts (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    payment_id UUID NOT NULL,
    refund_id UUID,
    channel VARCHAR(50) NOT NULL,
    attempt_type VARCHAR(20) NOT NULL,
    attempt_no INTEGER NOT NULL DEFAULT 1,
    channel_order_no VARCHAR(255),
    channel_request_id VARCHAR(255),
    state VARCHAR(32) NOT NULL DEFAULT 'pending',
    amount DECIMAL(19, 4),
    currency VARCHAR(3),
    error_code VARCHAR(100),
    error_message VARCHAR(2000),
    latency_ms INTEGER,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw_response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_pay_attempts_state CHECK (state IN ('pending', 'succeeded', 'failed', 'unknown', 'rejected')),
    CONSTRAINT ck_pay_attempts_type CHECK (attempt_type IN ('charge', 'query', 'refund', 'query_refund', 'close', 'callback')),
    CONSTRAINT ck_pay_attempts_channel CHECK (channel IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy')),
    CONSTRAINT fk_pay_attempts_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_pay_attempts_payment_id_payments FOREIGN KEY (payment_id) REFERENCES guliangiam.payments(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_pay_attempts_refund_id_pay_refunds FOREIGN KEY (refund_id) REFERENCES guliangiam.pay_refunds(id) ON DELETE SET NULL ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_pay_attempts_payment ON guliangiam.pay_attempts (payment_id);
CREATE INDEX IF NOT EXISTS idx_pay_attempts_refund ON guliangiam.pay_attempts (refund_id);
CREATE INDEX IF NOT EXISTS idx_pay_attempts_occurred ON guliangiam.pay_attempts (occurred_at);
ALTER TABLE guliangiam.pay_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.pay_attempts FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'pay_attempts'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.pay_attempts
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.pay_recon_batches (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    channel VARCHAR(50) NOT NULL,
    batch_date DATE NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    diff_count INTEGER NOT NULL DEFAULT 0,
    total_local INTEGER NOT NULL DEFAULT 0,
    total_channel INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_pay_recon_batches_status CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    CONSTRAINT fk_pay_recon_batches_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_pay_recon_batches_org_channel_date ON guliangiam.pay_recon_batches (organization_id, channel, batch_date) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pay_recon_batches_org ON guliangiam.pay_recon_batches (organization_id);
CREATE INDEX IF NOT EXISTS idx_pay_recon_batches_date ON guliangiam.pay_recon_batches (batch_date);
ALTER TABLE guliangiam.pay_recon_batches ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.pay_recon_batches FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'pay_recon_batches'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.pay_recon_batches
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.pay_recon_items (
    id UUID NOT NULL PRIMARY KEY,
    batch_id UUID NOT NULL,
    diff_type VARCHAR(30) NOT NULL,
    channel_order_no VARCHAR(255),
    local_payment_id UUID,
    channel_amount DECIMAL(19, 4),
    local_amount DECIMAL(19, 4),
    channel_state VARCHAR(50),
    local_state VARCHAR(50),
    resolved BOOLEAN NOT NULL DEFAULT false,
    resolved_by VARCHAR(255) DEFAULT '',
    resolved_at TIMESTAMPTZ,
    note VARCHAR(1000) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_pay_recon_items_diff_type CHECK (diff_type IN ('missing_local', 'missing_channel', 'amount_mismatch', 'state_mismatch', 'refund_mismatch')),
    CONSTRAINT fk_pay_recon_items_batch_id_pay_recon_batches FOREIGN KEY (batch_id) REFERENCES guliangiam.pay_recon_batches(id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_pay_recon_items_batch ON guliangiam.pay_recon_items (batch_id);
CREATE INDEX IF NOT EXISTS idx_pay_recon_items_resolved ON guliangiam.pay_recon_items (resolved);
ALTER TABLE guliangiam.pay_recon_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.pay_recon_items FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'pay_recon_items'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.pay_recon_items
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.pay_recon_batches b WHERE b.id = batch_id AND b.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.pay_recon_batches b WHERE b.id = batch_id AND b.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：75_ai_gateway.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.llm_providers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    provider_id UUID,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    provider_type VARCHAR(50) NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    api_key_sealed TEXT NOT NULL,
    api_key_fingerprint VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    request_timeout_ms INTEGER NOT NULL DEFAULT 30000,
    max_retries INTEGER NOT NULL DEFAULT 2,
    priority INTEGER NOT NULL DEFAULT 100,
    weight INTEGER NOT NULL DEFAULT 1,
    rpm_limit INTEGER,
    tpm_limit INTEGER,
    supported_protocols JSONB NOT NULL DEFAULT '[]',
    preferred_protocol VARCHAR(30) NOT NULL DEFAULT 'openai',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_llm_providers_status CHECK (status IN ('active', 'disabled', 'testing')),
    CONSTRAINT ck_llm_providers_provider_type CHECK (provider_type IN ('openai', 'anthropic', 'gemini', 'deepseek', 'glm', 'kimi', 'qwen', 'seed', 'ollama', 'vllm', 'custom')),
    CONSTRAINT ck_llm_providers_preferred_protocol CHECK (preferred_protocol IN ('openai', 'anthropic', 'gemini')),
    CONSTRAINT fk_llm_providers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_providers_provider_id_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.providers(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_llm_providers_org_name ON guliangiam.llm_providers (organization_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_llm_providers_provider ON guliangiam.llm_providers (provider_id) WHERE provider_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_llm_providers_org ON guliangiam.llm_providers (organization_id);
CREATE INDEX IF NOT EXISTS idx_llm_providers_status ON guliangiam.llm_providers (status);
ALTER TABLE guliangiam.llm_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_providers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_providers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_providers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.llm_models (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    provider_id UUID NOT NULL,
    provider_model_name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    capabilities JSONB NOT NULL DEFAULT '{}',
    max_context_tokens INTEGER,
    max_output_tokens INTEGER,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_llm_models_status CHECK (status IN ('active', 'deprecated')),
    CONSTRAINT fk_llm_models_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_models_provider_id_llm_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.llm_providers(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_llm_models_provider_model ON guliangiam.llm_models (provider_id, provider_model_name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_llm_models_provider ON guliangiam.llm_models (provider_id);
CREATE INDEX IF NOT EXISTS idx_llm_models_org ON guliangiam.llm_models (organization_id);
ALTER TABLE guliangiam.llm_models ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_models FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_models'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_models
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.llm_model_aliases (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    alias VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_llm_aliases_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT uq_llm_aliases_org_alias UNIQUE (organization_id, alias),
    CONSTRAINT fk_llm_model_aliases_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
ALTER TABLE guliangiam.llm_model_aliases ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_model_aliases FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_model_aliases'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_model_aliases
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.llm_routes (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    alias_id UUID NOT NULL,
    model_id UUID NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 100,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_llm_routes_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT fk_llm_routes_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_routes_alias_id_llm_model_aliases FOREIGN KEY (alias_id) REFERENCES guliangiam.llm_model_aliases(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_routes_model_id_llm_models FOREIGN KEY (model_id) REFERENCES guliangiam.llm_models(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_llm_routes_alias ON guliangiam.llm_routes (alias_id);
CREATE INDEX IF NOT EXISTS idx_llm_routes_model ON guliangiam.llm_routes (model_id);
ALTER TABLE guliangiam.llm_routes ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_routes FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_routes'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_routes
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.llm_api_keys (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    key_hash VARCHAR(128) NOT NULL,
    allowed_aliases JSONB NOT NULL DEFAULT '[]',
    rpm_limit INTEGER,
    tpm_limit INTEGER,
    monthly_budget DECIMAL(19, 4),
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_llm_api_keys_status CHECK (status IN ('active', 'revoked', 'expired')),
    CONSTRAINT fk_llm_api_keys_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_llm_api_keys_org_name ON guliangiam.llm_api_keys (organization_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_llm_api_keys_org ON guliangiam.llm_api_keys (organization_id);
ALTER TABLE guliangiam.llm_api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_api_keys FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_api_keys'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_api_keys
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.llm_quota_policies (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    scope VARCHAR(20) NOT NULL,
    scope_ref UUID,
    limit_type VARCHAR(20) NOT NULL,
    limit_value DECIMAL(19, 4) NOT NULL,
    reset_cycle VARCHAR(20) NOT NULL DEFAULT 'monthly',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_llm_quota_scope CHECK (scope IN ('organization', 'api_key', 'alias')),
    CONSTRAINT ck_llm_quota_reset_cycle CHECK (reset_cycle IN ('daily', 'monthly')),
    CONSTRAINT ck_llm_quota_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT fk_llm_quota_policies_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_llm_quota_org ON guliangiam.llm_quota_policies (organization_id);
CREATE INDEX IF NOT EXISTS idx_llm_quota_scope_ref ON guliangiam.llm_quota_policies (scope, scope_ref);
ALTER TABLE guliangiam.llm_quota_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_quota_policies FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_quota_policies'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_quota_policies
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.llm_price_versions (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    model_id UUID NOT NULL,
    version_no INTEGER NOT NULL DEFAULT 1,
    input_price_per_1m DECIMAL(19, 4) NOT NULL DEFAULT 0,
    output_price_per_1m DECIMAL(19, 4) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    effective_from TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_llm_price_model_version UNIQUE (model_id, version_no),
    CONSTRAINT fk_llm_price_versions_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_price_versions_model_id_llm_models FOREIGN KEY (model_id) REFERENCES guliangiam.llm_models(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_llm_price_effective ON guliangiam.llm_price_versions (effective_from);
ALTER TABLE guliangiam.llm_price_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_price_versions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_price_versions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_price_versions
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.llm_provider_health (
    id UUID NOT NULL PRIMARY KEY,
    provider_id UUID NOT NULL,
    state VARCHAR(32) NOT NULL DEFAULT 'healthy',
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    open_until TIMESTAMPTZ,
    last_checked_at TIMESTAMPTZ,
    last_latency_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_llm_health_state CHECK (state IN ('healthy', 'degraded', 'open', 'half_open')),
    CONSTRAINT fk_llm_provider_health_provider_id_llm_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.llm_providers(id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_llm_health_provider ON guliangiam.llm_provider_health (provider_id);
ALTER TABLE guliangiam.llm_provider_health ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_provider_health FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_provider_health'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_provider_health
            USING (guliangiam.is_super_admin() OR provider_id IN (SELECT id FROM guliangiam.llm_providers WHERE organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR provider_id IN (SELECT id FROM guliangiam.llm_providers WHERE organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.llm_usage_logs (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    api_key_id UUID,
    alias_id UUID NOT NULL,
    provider_id UUID NOT NULL,
    model_id UUID NOT NULL,
    price_version_id UUID,
    protocol_face VARCHAR(30) NOT NULL,
    attempt_no INTEGER NOT NULL DEFAULT 1,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cost_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status TEXT NOT NULL,
    error_code VARCHAR(100),
    ttft_ms INTEGER,
    total_latency_ms INTEGER,
    upstream_request_id VARCHAR(255),
    failover_from VARCHAR(255),
    request_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT ck_llm_usage_status CHECK (status IN ('succeeded', 'failed', 'timeout')),
    CONSTRAINT ck_llm_usage_protocol_face CHECK (protocol_face IN ('openai', 'anthropic', 'gemini')),
    CONSTRAINT fk_llm_usage_logs_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_api_key_id_llm_api_keys FOREIGN KEY (api_key_id) REFERENCES guliangiam.llm_api_keys(id) ON DELETE SET NULL ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_alias_id_llm_model_aliases FOREIGN KEY (alias_id) REFERENCES guliangiam.llm_model_aliases(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_provider_id_llm_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.llm_providers(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_model_id_llm_models FOREIGN KEY (model_id) REFERENCES guliangiam.llm_models(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_price_version_id_llm_price_versions FOREIGN KEY (price_version_id) REFERENCES guliangiam.llm_price_versions(id) ON DELETE SET NULL ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_llm_usage_org ON guliangiam.llm_usage_logs (organization_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_alias ON guliangiam.llm_usage_logs (alias_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_provider ON guliangiam.llm_usage_logs (provider_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_api_key ON guliangiam.llm_usage_logs (api_key_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_created ON guliangiam.llm_usage_logs (created_at);
ALTER TABLE guliangiam.llm_usage_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_usage_logs FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_usage_logs'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_usage_logs
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.llm_metering_events (
    id UUID NOT NULL PRIMARY KEY,
    outbox_id UUID NOT NULL,
    organization_id UUID,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id UUID,
    event_type VARCHAR(100) NOT NULL,
    provider_id UUID,
    model_id UUID,
    payload_json JSONB NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_llm_metering_events_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE SET NULL,
    CONSTRAINT fk_llm_metering_events_provider_id_llm_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.llm_providers(id) ON DELETE SET NULL,
    CONSTRAINT fk_llm_metering_events_model_id_llm_models FOREIGN KEY (model_id) REFERENCES guliangiam.llm_models(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_llm_metering_events_org ON guliangiam.llm_metering_events (organization_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_metering_events_provider ON guliangiam.llm_metering_events (provider_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_metering_events_event_type ON guliangiam.llm_metering_events (event_type);
ALTER TABLE guliangiam.llm_metering_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.llm_metering_events FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'llm_metering_events'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.llm_metering_events
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：80_governance.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.invitations (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    email TEXT NOT NULL,
    code TEXT NOT NULL,
    role_ids UUID[] NOT NULL DEFAULT '{}',
    group_ids UUID[] NOT NULL DEFAULT '{}',
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    invited_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_invitations_code UNIQUE (code),
    CONSTRAINT ck_invitations_status CHECK (status IN ('pending', 'accepted', 'expired', 'revoked')),
    CONSTRAINT fk_invitations_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_invitations_org_created_at_desc ON guliangiam.invitations (organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invitations_email ON guliangiam.invitations (email);
CREATE INDEX IF NOT EXISTS idx_invitations_status ON guliangiam.invitations (status);
ALTER TABLE guliangiam.invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.invitations FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'invitations'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.invitations
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.certs (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    scope VARCHAR(64) NOT NULL DEFAULT 'JWT',
    type VARCHAR(64) NOT NULL DEFAULT 'x509',
    crypto_algorithm VARCHAR(64) NOT NULL DEFAULT 'RS256',
    bit_size INTEGER NOT NULL DEFAULT 2048,
    expire_in_years INTEGER NOT NULL DEFAULT 1,
    expire_time TIMESTAMPTZ,
    certificate VARCHAR(255) NOT NULL DEFAULT '',
    private_key TEXT NOT NULL,
    provider VARCHAR(255) NOT NULL DEFAULT '',
    account VARCHAR(255) NOT NULL DEFAULT '',
    access_key VARCHAR(255) NOT NULL DEFAULT '',
    access_secret TEXT NOT NULL,
    domain JSONB NOT NULL DEFAULT '[]',
    authority_public_key VARCHAR(255) NOT NULL DEFAULT '',
    authority_root_public_key VARCHAR(255) NOT NULL DEFAULT '',
    acme_issuer VARCHAR(255) NOT NULL DEFAULT '',
    acme_endpoint VARCHAR(255) NOT NULL DEFAULT '',
    domain_expire_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_certs_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_certs_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_certs_scope ON guliangiam.certs (scope);
CREATE INDEX IF NOT EXISTS idx_certs_type ON guliangiam.certs (type);
ALTER TABLE guliangiam.certs ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.certs FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'certs'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.certs
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.lockout_attempts (
    id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_lockout_attempts_user_id UNIQUE (user_id),
    CONSTRAINT fk_lockout_attempts_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.lockout_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.lockout_attempts FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'lockout_attempts'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.lockout_attempts
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.ip_rules (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    rule_type VARCHAR(64) NOT NULL DEFAULT 'block',
    cidr CIDR NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_ip_rules_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_ip_rules_org ON guliangiam.ip_rules (organization_id);
ALTER TABLE guliangiam.ip_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.ip_rules FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'ip_rules'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.ip_rules
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.rules (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    scope VARCHAR(255) NOT NULL DEFAULT '',
    rule_type VARCHAR(255) NOT NULL DEFAULT '',
    conditions JSONB NOT NULL DEFAULT '[]'::jsonb,
    action VARCHAR(255) NOT NULL DEFAULT '',
    priority INTEGER NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_rules_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_rules_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.rules FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'rules'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.rules
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.mcp_servers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    url VARCHAR(255) NOT NULL DEFAULT '',
    token VARCHAR(255) NOT NULL DEFAULT '',
    application VARCHAR(255) NOT NULL DEFAULT '',
    tools JSONB NOT NULL DEFAULT '[]',
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_mcp_servers_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_mcp_servers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.mcp_servers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.mcp_servers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'mcp_servers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.mcp_servers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.radius_accountings (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    username VARCHAR(255) NOT NULL DEFAULT '',
    service_type BIGINT NOT NULL DEFAULT 0,
    nas_id VARCHAR(255) NOT NULL DEFAULT '',
    nas_ip_addr VARCHAR(255) NOT NULL DEFAULT '',
    nas_port_id VARCHAR(255) NOT NULL DEFAULT '',
    nas_port_type BIGINT NOT NULL DEFAULT 0,
    nas_port BIGINT NOT NULL DEFAULT 0,
    framed_ip_addr VARCHAR(255) NOT NULL DEFAULT '',
    framed_ip_netmask VARCHAR(255) NOT NULL DEFAULT '',
    acct_session_id VARCHAR(255) NOT NULL DEFAULT '',
    acct_session_time BIGINT NOT NULL DEFAULT 0,
    acct_input_total BIGINT NOT NULL DEFAULT 0,
    acct_output_total BIGINT NOT NULL DEFAULT 0,
    acct_input_packets BIGINT NOT NULL DEFAULT 0,
    acct_output_packets BIGINT NOT NULL DEFAULT 0,
    acct_terminate_cause BIGINT NOT NULL DEFAULT 0,
    acct_start_time TIMESTAMPTZ NOT NULL,
    acct_stop_time TIMESTAMPTZ,
    last_update TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_radius_accountings_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_radius_accountings_org ON guliangiam.radius_accountings (organization_id);
CREATE INDEX IF NOT EXISTS idx_radius_accountings_username ON guliangiam.radius_accountings (username);
ALTER TABLE guliangiam.radius_accountings ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.radius_accountings FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'radius_accountings'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.radius_accountings
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.servers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    protocol VARCHAR(64) NOT NULL DEFAULT 'https',
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_servers_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_servers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_servers_org_created_at_desc ON guliangiam.servers (organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_servers_status ON guliangiam.servers (status) WHERE deleted_at IS NULL;
ALTER TABLE guliangiam.servers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.servers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'servers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.servers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.sites (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    ssl_mode VARCHAR(20) NOT NULL DEFAULT 'off',
    cert_id UUID,
    force_https BOOLEAN NOT NULL DEFAULT false,
    enable_health_check BOOLEAN NOT NULL DEFAULT false,
    health_check_interval INTEGER NOT NULL DEFAULT 60,
    last_health_status VARCHAR(20) NOT NULL DEFAULT 'unknown',
    last_health_at TIMESTAMPTZ,
    enable_alert BOOLEAN NOT NULL DEFAULT false,
    alert_interval INTEGER NOT NULL DEFAULT 300,
    alert_try_times INTEGER NOT NULL DEFAULT 3,
    alert_provider_ids TEXT[] NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_sites_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_sites_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_sites_cert_id_certs FOREIGN KEY (cert_id) REFERENCES guliangiam.certs(id) ON DELETE SET NULL ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_sites_host_active ON guliangiam.sites (host) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sites_org_created_at_desc ON guliangiam.sites (organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sites_status ON guliangiam.sites (status) WHERE deleted_at IS NULL;
ALTER TABLE guliangiam.sites ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.sites FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'sites'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.sites
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.keys (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    key_type TEXT NOT NULL,
    public_key TEXT NOT NULL,
    private_key TEXT,
    algorithm VARCHAR(64) NOT NULL DEFAULT 'RS256',
    expires_at TIMESTAMPTZ,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_keys_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_keys_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_keys_type ON guliangiam.keys (key_type);
ALTER TABLE guliangiam.keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.keys FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'keys'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.keys
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.notification_providers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    provider_id UUID,
    name TEXT NOT NULL,
    provider_type VARCHAR(255) NOT NULL DEFAULT '',
    webhook_url VARCHAR(255) NOT NULL DEFAULT '',
    access_token TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_notification_providers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_notification_providers_provider_id_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.providers(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_notification_providers_provider ON guliangiam.notification_providers (provider_id) WHERE provider_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_notification_providers_organization_id ON guliangiam.notification_providers (organization_id);
ALTER TABLE guliangiam.notification_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.notification_providers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'notification_providers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.notification_providers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.radius_configs (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    host VARCHAR(255) NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 1812,
    shared_secret TEXT NOT NULL,
    auth_protocol VARCHAR(64) NOT NULL DEFAULT 'PAP',
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_radius_configs_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_radius_configs_organization_id ON guliangiam.radius_configs (organization_id);
ALTER TABLE guliangiam.radius_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.radius_configs FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'radius_configs'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.radius_configs
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.ldaps (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    server_name VARCHAR(255) NOT NULL DEFAULT '',
    host VARCHAR(255) NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 389,
    admin VARCHAR(255) NOT NULL DEFAULT '',
    bind_dn VARCHAR(255) NOT NULL DEFAULT '',
    bind_password VARCHAR(255) NOT NULL DEFAULT '',
    base_dn VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(255) NOT NULL DEFAULT '',
    attribute_name VARCHAR(255) NOT NULL DEFAULT '',
    password_type VARCHAR(64) NOT NULL DEFAULT 'Plain',
    filter VARCHAR(255) NOT NULL DEFAULT '',
    filter_fields TEXT[] NOT NULL DEFAULT '{}',
    enable_ssl BOOLEAN NOT NULL DEFAULT false,
    allow_self_signed_cert BOOLEAN NOT NULL DEFAULT false,
    enable_groups BOOLEAN NOT NULL DEFAULT false,
    default_groups TEXT[] NOT NULL DEFAULT '{}',
    custom_attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    sync_interval INTEGER NOT NULL DEFAULT 0,
    last_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_ldaps_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_ldaps_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.ldaps ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.ldaps FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'ldaps'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.ldaps
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.ldap_sync_configs (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    ldap_id UUID NOT NULL,
    sync_interval_minutes INTEGER NOT NULL DEFAULT 60,
    base_dn TEXT NOT NULL,
    user_filter VARCHAR(64) DEFAULT '(objectClass=person)',
    group_filter VARCHAR(64) DEFAULT '(objectClass=group)',
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_sync_at TIMESTAMPTZ,
    next_sync_at TIMESTAMPTZ,
    sync_status VARCHAR(64) DEFAULT 'idle',
    sync_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_ldap_sync_configs_ldap_id_ldaps FOREIGN KEY (ldap_id) REFERENCES guliangiam.ldaps(id) ON DELETE CASCADE,
    CONSTRAINT fk_ldap_sync_configs_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_ldap_sync_configs_org_id ON guliangiam.ldap_sync_configs (organization_id);
CREATE INDEX IF NOT EXISTS idx_ldap_sync_configs_ldap_id ON guliangiam.ldap_sync_configs (ldap_id);
ALTER TABLE guliangiam.ldap_sync_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.ldap_sync_configs FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'ldap_sync_configs'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.ldap_sync_configs
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.ldap_sync_tasks (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    ldap_id UUID NOT NULL,
    config_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    trigger_type VARCHAR(32) NOT NULL,
    users_synced INTEGER NOT NULL DEFAULT 0,
    users_created INTEGER NOT NULL DEFAULT 0,
    users_updated INTEGER NOT NULL DEFAULT 0,
    users_failed INTEGER NOT NULL DEFAULT 0,
    groups_synced INTEGER NOT NULL DEFAULT 0,
    groups_created INTEGER NOT NULL DEFAULT 0,
    error_message VARCHAR(2000) NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_ldap_sync_tasks_status CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    CONSTRAINT ck_ldap_sync_tasks_trigger_type CHECK (trigger_type IN ('manual', 'scheduled')),
    CONSTRAINT fk_ldap_sync_tasks_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_ldap_sync_tasks_ldap_id_ldaps FOREIGN KEY (ldap_id) REFERENCES guliangiam.ldaps(id) ON DELETE CASCADE,
    CONSTRAINT fk_ldap_sync_tasks_config_id_ldap_sync_configs FOREIGN KEY (config_id) REFERENCES guliangiam.ldap_sync_configs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_ldap_sync_tasks_ldap_created ON guliangiam.ldap_sync_tasks (ldap_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ldap_sync_tasks_config ON guliangiam.ldap_sync_tasks (config_id);
CREATE INDEX IF NOT EXISTS idx_ldap_sync_tasks_org_status ON guliangiam.ldap_sync_tasks (organization_id, status);
ALTER TABLE guliangiam.ldap_sync_tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.ldap_sync_tasks FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'ldap_sync_tasks'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.ldap_sync_tasks
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.scim_schemas (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    schema_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    attributes JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_scim_schemas_org_schema_id UNIQUE (organization_id, schema_id),
    CONSTRAINT fk_scim_schemas_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.scim_schemas ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.scim_schemas FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'scim_schemas'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.scim_schemas
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.notifications (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    provider_id UUID NOT NULL,
    recipient TEXT NOT NULL,
    subject TEXT,
    content TEXT NOT NULL,
    template_id TEXT,
    variables JSONB,
    status VARCHAR(64) DEFAULT 'pending',
    error TEXT,
    sent_at TIMESTAMPTZ,
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_notifications_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_notifications_provider_id_notification_providers FOREIGN KEY (provider_id) REFERENCES guliangiam.notification_providers(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_notifications_org_status ON guliangiam.notifications (organization_id, status);
ALTER TABLE guliangiam.notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.notifications FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'notifications'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.notifications
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.access_campaigns (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'draft',
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_access_campaigns_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_access_campaigns_status CHECK (status IN ('draft', 'active', 'completed', 'cancelled')),
    CONSTRAINT fk_access_campaigns_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.access_campaigns ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.access_campaigns FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'access_campaigns'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.access_campaigns
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.sod_rules (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    role_a_id UUID NOT NULL,
    role_b_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_sod_rules_org_name UNIQUE (organization_id, name),
    CONSTRAINT uq_sod_rules_org_roles UNIQUE (organization_id, role_a_id, role_b_id),
    CONSTRAINT fk_sod_rules_role_a_id_roles FOREIGN KEY (role_a_id) REFERENCES guliangiam.roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_sod_rules_role_b_id_roles FOREIGN KEY (role_b_id) REFERENCES guliangiam.roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_sod_rules_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.sod_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.sod_rules FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'sod_rules'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.sod_rules
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.access_lifecycle (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role_id UUID NOT NULL,
    ticket_id UUID,
    enforcer_id UUID,
    resource VARCHAR(255) NOT NULL DEFAULT '',
    action VARCHAR(64) NOT NULL DEFAULT '',
    policy_id UUID,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    starts_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error VARCHAR(2000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_access_lifecycle_status CHECK (status IN ('active', 'expired', 'revoked')),
    CONSTRAINT fk_access_lifecycle_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE,
    CONSTRAINT fk_access_lifecycle_role_id_roles FOREIGN KEY (role_id) REFERENCES guliangiam.roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_access_lifecycle_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_access_lifecycle_org_user ON guliangiam.access_lifecycle (organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_access_lifecycle_expires ON guliangiam.access_lifecycle (expires_at) WHERE status = 'active';
ALTER TABLE guliangiam.access_lifecycle ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.access_lifecycle FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'access_lifecycle'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.access_lifecycle
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.risk_signals (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    user_id UUID,
    signal_type TEXT NOT NULL,
    signal_value JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_risk_signals_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL,
    CONSTRAINT fk_risk_signals_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_risk_signals_org_type ON guliangiam.risk_signals (organization_id, signal_type);
CREATE INDEX IF NOT EXISTS idx_risk_signals_user ON guliangiam.risk_signals (user_id);
CREATE INDEX IF NOT EXISTS idx_risk_signals_created ON guliangiam.risk_signals (created_at DESC);
ALTER TABLE guliangiam.risk_signals ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.risk_signals FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'risk_signals'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.risk_signals
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.risk_decisions (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    user_id UUID,
    score INTEGER NOT NULL DEFAULT 0,
    decision VARCHAR(64) NOT NULL DEFAULT 'allow',
    factors JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_risk_decisions_decision CHECK (decision IN ('allow', 'challenge', 'deny')),
    CONSTRAINT ck_risk_decisions_score CHECK (score >= 0 AND score <= 100),
    CONSTRAINT fk_risk_decisions_user_id_users FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL,
    CONSTRAINT fk_risk_decisions_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_risk_decisions_org_user ON guliangiam.risk_decisions (organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_risk_decisions_created ON guliangiam.risk_decisions (created_at DESC);
ALTER TABLE guliangiam.risk_decisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.risk_decisions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'risk_decisions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.risk_decisions
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.session_governance (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    session_id UUID,
    risk_score INTEGER NOT NULL DEFAULT 0,
    action VARCHAR(64) NOT NULL DEFAULT 'monitor',
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_session_governance_action CHECK (action IN ('monitor', 'step_up', 'revoke')),
    CONSTRAINT fk_session_governance_session_id_sessions FOREIGN KEY (session_id) REFERENCES guliangiam.sessions(id) ON DELETE SET NULL,
    CONSTRAINT fk_session_governance_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_session_governance_org_session ON guliangiam.session_governance (organization_id, session_id);
CREATE INDEX IF NOT EXISTS idx_session_governance_created ON guliangiam.session_governance (created_at DESC);
ALTER TABLE guliangiam.session_governance ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.session_governance FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'session_governance'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.session_governance
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.identity_actions (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    trigger_type VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_identity_actions_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_identity_actions_status CHECK (status IN ('draft', 'active', 'disabled', 'deleted')),
    CONSTRAINT fk_identity_actions_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.identity_actions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.identity_actions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'identity_actions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.identity_actions
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.action_versions (
    id UUID NOT NULL PRIMARY KEY,
    action_id UUID NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    code VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_action_versions_action_version UNIQUE (action_id, version),
    CONSTRAINT fk_action_versions_action_id_identity_actions FOREIGN KEY (action_id) REFERENCES guliangiam.identity_actions(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.action_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.action_versions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'action_versions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.action_versions
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.identity_actions a WHERE a.id = action_id AND a.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.identity_actions a WHERE a.id = action_id AND a.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.execution_logs (
    id UUID NOT NULL PRIMARY KEY,
    action_id UUID NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    input JSONB NOT NULL DEFAULT '{}'::jsonb,
    output JSONB,
    error TEXT,
    executed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_execution_logs_status CHECK (status IN ('pending', 'running', 'success', 'failed', 'timeout')),
    CONSTRAINT fk_execution_logs_action_id_identity_actions FOREIGN KEY (action_id) REFERENCES guliangiam.identity_actions(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_execution_logs_action ON guliangiam.execution_logs (action_id);
CREATE INDEX IF NOT EXISTS idx_execution_logs_status ON guliangiam.execution_logs (status);
ALTER TABLE guliangiam.execution_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.execution_logs FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'execution_logs'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.execution_logs
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.identity_actions a WHERE a.id = action_id AND a.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.identity_actions a WHERE a.id = action_id AND a.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.scim_reconciliation_reports (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    connector_id TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'completed',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_report JSONB NOT NULL DEFAULT '{}'::jsonb,
    group_report JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message VARCHAR(2048) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_scim_reconciliation_reports_status CHECK (status IN ('completed', 'failed')),
    CONSTRAINT fk_scim_reconciliation_reports_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_scim_reconciliation_reports_org_created ON guliangiam.scim_reconciliation_reports (organization_id, created_at);
ALTER TABLE guliangiam.scim_reconciliation_reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.scim_reconciliation_reports FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'scim_reconciliation_reports'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.scim_reconciliation_reports
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.connectors (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    scim_base_url TEXT NOT NULL,
    bearer_token_sealed TEXT NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    timeout_ms INTEGER NOT NULL DEFAULT 5000,
    max_rate_per_min INTEGER NOT NULL DEFAULT 300,
    revision BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_connectors_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_connectors_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_connectors_org_enabled ON guliangiam.connectors (organization_id, is_enabled);
ALTER TABLE guliangiam.connectors ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.connectors FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'connectors'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.connectors
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.ssf_receivers (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    receiver_url TEXT NOT NULL,
    auth_token_hash VARCHAR(255) NOT NULL DEFAULT '',
    event_types JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_ssf_receivers_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_ssf_receivers_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT fk_ssf_receivers_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_ssf_receivers_org_status ON guliangiam.ssf_receivers (organization_id, status);
ALTER TABLE guliangiam.ssf_receivers ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.ssf_receivers FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'ssf_receivers'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.ssf_receivers
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.dsar_requests (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    subject_type VARCHAR(16) NOT NULL,
    subject_id UUID NOT NULL,
    request_type VARCHAR(24) NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'received',
    sla_due_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    rejection_reason VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_dsar_requests_type CHECK (request_type IN ('access', 'rectify', 'freeze', 'erase', 'portability', 'withdraw_consent')),
    CONSTRAINT ck_dsar_requests_state CHECK (state IN ('received', 'verifying', 'in_progress', 'fulfilled', 'rejected')),
    CONSTRAINT ck_dsar_requests_subject_type CHECK (subject_type IN ('user', 'contact')),
    CONSTRAINT ck_dsar_requests_closed CHECK (state NOT IN ('fulfilled', 'rejected') OR closed_at IS NOT NULL),
    CONSTRAINT fk_dsar_requests_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_dsar_requests_org_state ON guliangiam.dsar_requests (organization_id, state);
CREATE INDEX IF NOT EXISTS idx_dsar_requests_sla ON guliangiam.dsar_requests (state, sla_due_at);
ALTER TABLE guliangiam.dsar_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.dsar_requests FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'dsar_requests'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.dsar_requests
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.dsar_events (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    dsar_id UUID NOT NULL,
    action VARCHAR(32) NOT NULL,
    actor_id UUID,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_dsar_events_action CHECK (action IN ('created', 'verifying', 'in_progress', 'fulfilled', 'rejected')),
    CONSTRAINT fk_dsar_events_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_dsar_events_dsar_id_dsar_requests FOREIGN KEY (dsar_id) REFERENCES guliangiam.dsar_requests(id) ON DELETE CASCADE,
    CONSTRAINT fk_dsar_events_actor_id_users FOREIGN KEY (actor_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_dsar_events_dsar_created ON guliangiam.dsar_events (dsar_id, created_at);
CREATE INDEX IF NOT EXISTS idx_dsar_events_org_created ON guliangiam.dsar_events (organization_id, created_at);
ALTER TABLE guliangiam.dsar_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.dsar_events FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'dsar_events'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.dsar_events
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.consent_records (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    subject_type VARCHAR(16) NOT NULL,
    subject_id UUID NOT NULL,
    purpose VARCHAR(64) NOT NULL,
    version INTEGER NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    withdrawn_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_consent_records_version UNIQUE (organization_id, subject_type, subject_id, purpose, version),
    CONSTRAINT ck_consent_records_subject_type CHECK (subject_type IN ('user', 'contact')),
    CONSTRAINT ck_consent_records_withdrawn CHECK (withdrawn_at IS NULL OR withdrawn_at >= granted_at),
    CONSTRAINT fk_consent_records_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.consent_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.consent_records FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'consent_records'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.consent_records
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：82_support.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.tickets (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(100) DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'open',
    priority VARCHAR(50) DEFAULT '',
    assignee VARCHAR(255) DEFAULT '',
    creator VARCHAR(255) DEFAULT '',
    description VARCHAR(255) DEFAULT '',
    requester_id UUID,
    role_id UUID,
    application_id UUID,
    enforcer_id UUID,
    resource VARCHAR(255) NOT NULL DEFAULT '',
    action VARCHAR(64) NOT NULL DEFAULT '',
    starts_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    workflow_state VARCHAR(64),
    manager_approved_by UUID,
    sap_approved_by UUID,
    lifecycle_id UUID,
    attachment VARCHAR(500) DEFAULT '',
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_tickets_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_tickets_status CHECK (status IN ('open', 'pending', 'resolved', 'closed')),
    CONSTRAINT fk_tickets_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.tickets FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'tickets'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.tickets
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.ticket_messages (
    id UUID NOT NULL PRIMARY KEY,
    ticket_id UUID NOT NULL,
    author_id UUID,
    action VARCHAR(64) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_ticket_messages_ticket_id_tickets FOREIGN KEY (ticket_id) REFERENCES guliangiam.tickets(id) ON DELETE CASCADE,
    CONSTRAINT fk_ticket_messages_author_id_users FOREIGN KEY (author_id) REFERENCES guliangiam.users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_ticket_messages_ticket_created ON guliangiam.ticket_messages (ticket_id, created_at);
ALTER TABLE guliangiam.ticket_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.ticket_messages FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'ticket_messages'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.ticket_messages
            USING (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.tickets t WHERE t.id = ticket_id AND t.organization_id = guliangiam.current_organization_id()))
            WITH CHECK (guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.tickets t WHERE t.id = ticket_id AND t.organization_id = guliangiam.current_organization_id()));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.forms (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    description VARCHAR(255) DEFAULT '',
    type VARCHAR(100) DEFAULT '',
    properties VARCHAR(255) DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_forms_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_forms_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.forms ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.forms FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'forms'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.forms
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：85_workload.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.identity_pools (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_identity_pools_org_name UNIQUE (organization_id, name),
    CONSTRAINT fk_identity_pools_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.identity_pools ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.identity_pools FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'identity_pools'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.identity_pools
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.federation_trusts (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    pool_id UUID NOT NULL,
    name TEXT NOT NULL,
    issuer TEXT NOT NULL,
    jwks_uri VARCHAR(255) NOT NULL DEFAULT '',
    audience TEXT[] NOT NULL DEFAULT '{}',
    subjects TEXT[] NOT NULL DEFAULT '{}',
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_federation_trusts_org_name UNIQUE (organization_id, name),
    CONSTRAINT ck_federation_trusts_status CHECK (status IN ('active', 'disabled', 'deleted')),
    CONSTRAINT fk_federation_trusts_pool_id_identity_pools FOREIGN KEY (pool_id) REFERENCES guliangiam.identity_pools(id) ON DELETE CASCADE,
    CONSTRAINT fk_federation_trusts_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_federation_trusts_pool ON guliangiam.federation_trusts (pool_id);
ALTER TABLE guliangiam.federation_trusts ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.federation_trusts FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'federation_trusts'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.federation_trusts
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.service_accounts (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    pool_id UUID,
    name TEXT NOT NULL,
    description TEXT,
    client_id TEXT NOT NULL,
    client_secret_hash VARCHAR(255) NOT NULL DEFAULT '',
    scopes TEXT[] NOT NULL DEFAULT '{}',
    access_token_ttl_seconds INTEGER NOT NULL DEFAULT 900,
    last_rotated_at TIMESTAMPTZ,
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_service_accounts_org_name UNIQUE (organization_id, name),
    CONSTRAINT uq_service_accounts_client_id UNIQUE (client_id),
    CONSTRAINT ck_service_accounts_status CHECK (status IN ('active', 'disabled', 'deleted')),
    CONSTRAINT ck_service_accounts_access_token_ttl CHECK (access_token_ttl_seconds BETWEEN 60 AND 3600),
    CONSTRAINT fk_service_accounts_pool_id_identity_pools FOREIGN KEY (pool_id) REFERENCES guliangiam.identity_pools(id) ON DELETE SET NULL,
    CONSTRAINT fk_service_accounts_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_service_accounts_pool ON guliangiam.service_accounts (pool_id);
ALTER TABLE guliangiam.service_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.service_accounts FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'service_accounts'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.service_accounts
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;


-- ------------------------------------------------------------
-- 分组：90_metering.yaml
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS guliangiam.metering_events (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    subject VARCHAR(255) NOT NULL DEFAULT '',
    quantity BIGINT NOT NULL DEFAULT 0,
    unit VARCHAR(255) NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_metering_events_org_idem UNIQUE (organization_id, idempotency_key),
    CONSTRAINT fk_metering_events_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_metering_events_org_type ON guliangiam.metering_events (organization_id, event_type);
CREATE INDEX IF NOT EXISTS idx_metering_events_occurred ON guliangiam.metering_events (organization_id, occurred_at DESC);
ALTER TABLE guliangiam.metering_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.metering_events FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'metering_events'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.metering_events
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.metering_aggregates (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    period TEXT NOT NULL,
    subject VARCHAR(255) NOT NULL DEFAULT '',
    metric TEXT NOT NULL,
    quantity BIGINT NOT NULL DEFAULT 0,
    unit VARCHAR(255) NOT NULL DEFAULT '',
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_metering_aggregates_org_period_subject_metric UNIQUE (organization_id, period, subject, metric),
    CONSTRAINT fk_metering_aggregates_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.metering_aggregates ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.metering_aggregates FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'metering_aggregates'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.metering_aggregates
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.plan_quotas (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    plan_id UUID NOT NULL,
    quota_key VARCHAR(64) NOT NULL,
    soft_limit BIGINT,
    hard_limit BIGINT NOT NULL,
    unit VARCHAR(32) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_plan_quotas_key UNIQUE (organization_id, plan_id, quota_key),
    CONSTRAINT ck_plan_quotas_hard_limit CHECK (hard_limit >= 0),
    CONSTRAINT ck_plan_quotas_soft_limit CHECK (soft_limit IS NULL OR (soft_limit >= 0 AND soft_limit <= hard_limit)),
    CONSTRAINT fk_plan_quotas_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_plan_quotas_plan_id_plans FOREIGN KEY (plan_id) REFERENCES guliangiam.plans(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_plan_quotas_plan ON guliangiam.plan_quotas (plan_id);
ALTER TABLE guliangiam.plan_quotas ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.plan_quotas FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'plan_quotas'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.plan_quotas
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS guliangiam.tenant_quotas (
    id UUID NOT NULL PRIMARY KEY,
    organization_id UUID NOT NULL,
    quota_key VARCHAR(64) NOT NULL,
    limit_value BIGINT NOT NULL,
    used_value BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_tenant_quotas_key UNIQUE (organization_id, quota_key),
    CONSTRAINT ck_tenant_quotas_limit CHECK (limit_value >= 0),
    CONSTRAINT ck_tenant_quotas_used CHECK (used_value >= 0),
    CONSTRAINT fk_tenant_quotas_organization_id_organizations FOREIGN KEY (organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE
);
ALTER TABLE guliangiam.tenant_quotas ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.tenant_quotas FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'guliangiam'
          AND tablename = 'tenant_quotas'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON guliangiam.tenant_quotas
            USING (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin() OR organization_id = guliangiam.current_organization_id());
    END IF;
END $$;

-- ============================================================
-- 4. 依赖表结构的函数与触发器
-- ============================================================

CREATE OR REPLACE FUNCTION guliangiam.resolve_org_id(text) RETURNS uuid
    LANGUAGE sql STABLE AS $$
SELECT CASE WHEN $1 ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  THEN $1::uuid
  ELSE (SELECT id FROM guliangiam.organizations WHERE slug = $1 LIMIT 1)
END;
$$;

CREATE OR REPLACE FUNCTION guliangiam.sync_users_is_deleted() RETURNS trigger
    LANGUAGE plpgsql VOLATILE AS $$
BEGIN
    NEW.is_deleted := (NEW.deleted_at IS NOT NULL);
    RETURN NEW;
END;
$$;

CREATE OR REPLACE TRIGGER trg_users_sync_is_deleted
    BEFORE INSERT OR UPDATE ON guliangiam.users
    FOR EACH ROW EXECUTE FUNCTION guliangiam.sync_users_is_deleted();

-- ============================================================
-- 5. 延迟外键（目标表在声明序中更晚出现，避免 FK 前向引用）
-- ============================================================

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_platform_admins_user_id_users'
          AND conrelid = 'guliangiam.platform_admins'::regclass
    ) THEN
        ALTER TABLE guliangiam.platform_admins
            ADD CONSTRAINT fk_platform_admins_user_id_users
            FOREIGN KEY (user_id) REFERENCES guliangiam.users(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_platform_admins_created_by_users'
          AND conrelid = 'guliangiam.platform_admins'::regclass
    ) THEN
        ALTER TABLE guliangiam.platform_admins
            ADD CONSTRAINT fk_platform_admins_created_by_users
            FOREIGN KEY (created_by) REFERENCES guliangiam.users(id) ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_platform_admin_scopes_approved_by_users'
          AND conrelid = 'guliangiam.platform_admin_scopes'::regclass
    ) THEN
        ALTER TABLE guliangiam.platform_admin_scopes
            ADD CONSTRAINT fk_platform_admin_scopes_approved_by_users
            FOREIGN KEY (approved_by) REFERENCES guliangiam.users(id) ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_users_signup_application_id'
          AND conrelid = 'guliangiam.users'::regclass
    ) THEN
        ALTER TABLE guliangiam.users
            ADD CONSTRAINT fk_users_signup_application_id
            FOREIGN KEY (signup_application_id) REFERENCES guliangiam.applications(id) ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_app_provider_bindings_provider_id'
          AND conrelid = 'guliangiam.application_provider_bindings'::regclass
    ) THEN
        ALTER TABLE guliangiam.application_provider_bindings
            ADD CONSTRAINT fk_app_provider_bindings_provider_id
            FOREIGN KEY (provider_id) REFERENCES guliangiam.providers(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_web3_identity_mappings_credential_id_webauthn_credentials'
          AND conrelid = 'guliangiam.web3_identity_mappings'::regclass
    ) THEN
        ALTER TABLE guliangiam.web3_identity_mappings
            ADD CONSTRAINT fk_web3_identity_mappings_credential_id_webauthn_credentials
            FOREIGN KEY (credential_id) REFERENCES guliangiam.webauthn_credentials(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_casbin_rules_enforcer_id_enforcers'
          AND conrelid = 'guliangiam.casbin_rules'::regclass
    ) THEN
        ALTER TABLE guliangiam.casbin_rules
            ADD CONSTRAINT fk_casbin_rules_enforcer_id_enforcers
            FOREIGN KEY (enforcer_id) REFERENCES guliangiam.enforcers(id) ON DELETE CASCADE;
    END IF;
END $$;
