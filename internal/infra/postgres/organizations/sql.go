package organizations

const (
	queryGetOrganizationByID = `
SELECT o.id, o.slug, o.display_name, o.website_url, o.logo_url, o.logo_dark_url, o.favicon_url, o.status, o.init_score, o.default_avatar, ac.password_obfuscator_type, o.disable_signin, COALESCE(ac.password_options, '{}'), COALESCE(o.enable_soft_deletion, false), COALESCE(o.disable_signup, false), COALESCE(o.ip_whitelist, ''), COALESCE(o.initiate_login_url, ''), COALESCE(bl.balance, 0), COALESCE(bl.currency, 'USD'),
COALESCE(ac.password_type, 'argon2id'), COALESCE(ac.password_salt, ''), COALESCE(ac.password_obfuscator_key, ''), COALESCE(bl.enable_deposit, false), COALESCE(bl.balance_credit, 0), COALESCE(bl.mu, 0), COALESCE(bl.default_quota, 0), COALESCE(bl.default_currency, ''), COALESCE(br.enable_dark_logo, false), COALESCE(br.has_privilege_consent, false), COALESCE(br.is_profile_public, false), COALESCE(ac.use_email_as_username, false), COALESCE(br.use_permanent_avatar, false), COALESCE(br.enable_tour, false), COALESCE(ac.master_password, ''), COALESCE(ac.master_verification_code, ''), COALESCE(ac.default_password, ''), COALESCE(ac.ldap_attributes, '{}'), COALESCE(kb.kerberos_realm, ''), COALESCE(kb.kerberos_kdc_host, ''), COALESCE(kb.kerberos_keytab, ''), COALESCE(kb.kerberos_service_name, ''),
o.created_at, o.updated_at, o.revision,
s.organization_id, s.languages, s.default_application_id, s.account_items, s.nav_preferences, s.feature_flags, s.country_codes, s.user_types, s.tags, s.mfa_items, s.account_menu, s.mfa_remember_in_hours, s.dcr_policy,
t.organization_id, t.theme_name, t.tokens, t.custom_css, t.assets,
p.organization_id, p.password_policy, p.password_derive_algorithm, p.mfa_policy, p.ip_policy, p.session_policy, p.risk_policy, p.lockout_policy, p.concurrent_session_policy, o.parent_id,
COALESCE(o.region_code, '')
FROM guliangiam.organizations o
LEFT JOIN guliangiam.organization_settings s ON s.organization_id = o.id
LEFT JOIN guliangiam.organization_theme_settings t ON t.organization_id = o.id
LEFT JOIN guliangiam.organization_security_policies p ON p.organization_id = o.id
LEFT JOIN guliangiam.organization_auth_config ac ON ac.organization_id = o.id
LEFT JOIN guliangiam.organization_kerberos_config kb ON kb.organization_id = o.id
LEFT JOIN guliangiam.organization_billing bl ON bl.organization_id = o.id
LEFT JOIN guliangiam.organization_branding br ON br.organization_id = o.id
WHERE o.id = $1::uuid AND o.deleted_at IS NULL`

	queryGetOrganizationBySlug = `
SELECT o.id, o.slug, o.display_name, o.website_url, o.logo_url, o.logo_dark_url, o.favicon_url, o.status, o.init_score, o.default_avatar, ac.password_obfuscator_type, o.disable_signin, COALESCE(ac.password_options, '{}'), COALESCE(o.enable_soft_deletion, false), COALESCE(o.disable_signup, false), COALESCE(o.ip_whitelist, ''), COALESCE(o.initiate_login_url, ''), COALESCE(bl.balance, 0), COALESCE(bl.currency, 'USD'),
COALESCE(ac.password_type, 'argon2id'), COALESCE(ac.password_salt, ''), COALESCE(ac.password_obfuscator_key, ''), COALESCE(bl.enable_deposit, false), COALESCE(bl.balance_credit, 0), COALESCE(bl.mu, 0), COALESCE(bl.default_quota, 0), COALESCE(bl.default_currency, ''), COALESCE(br.enable_dark_logo, false), COALESCE(br.has_privilege_consent, false), COALESCE(br.is_profile_public, false), COALESCE(ac.use_email_as_username, false), COALESCE(br.use_permanent_avatar, false), COALESCE(br.enable_tour, false), COALESCE(ac.master_password, ''), COALESCE(ac.master_verification_code, ''), COALESCE(ac.default_password, ''), COALESCE(ac.ldap_attributes, '{}'), COALESCE(kb.kerberos_realm, ''), COALESCE(kb.kerberos_kdc_host, ''), COALESCE(kb.kerberos_keytab, ''), COALESCE(kb.kerberos_service_name, ''),
o.created_at, o.updated_at, o.revision,
s.organization_id, s.languages, s.default_application_id, s.account_items, s.nav_preferences, s.feature_flags, s.country_codes, s.user_types, s.tags, s.mfa_items, s.account_menu, s.mfa_remember_in_hours, s.dcr_policy,
t.organization_id, t.theme_name, t.tokens, t.custom_css, t.assets,
p.organization_id, p.password_policy, p.password_derive_algorithm, p.mfa_policy, p.ip_policy, p.session_policy, p.risk_policy, p.lockout_policy, p.concurrent_session_policy, o.parent_id,
COALESCE(o.region_code, '')
FROM guliangiam.organizations o
LEFT JOIN guliangiam.organization_settings s ON s.organization_id = o.id
LEFT JOIN guliangiam.organization_theme_settings t ON t.organization_id = o.id
LEFT JOIN guliangiam.organization_security_policies p ON p.organization_id = o.id
LEFT JOIN guliangiam.organization_auth_config ac ON ac.organization_id = o.id
LEFT JOIN guliangiam.organization_kerberos_config kb ON kb.organization_id = o.id
LEFT JOIN guliangiam.organization_billing bl ON bl.organization_id = o.id
LEFT JOIN guliangiam.organization_branding br ON br.organization_id = o.id
WHERE o.slug = $1 AND o.deleted_at IS NULL`

	// queryGetOrganizationBySlugPublic feeds the anonymous /by-slug and /resolve
	// discovery endpoints. Only public columns are selected: branding, theme and
	// the default application. Credential-bearing columns
	// (master_password, master_verification_code, default_password,
	// password_salt, password_obfuscator_key, kerberos_keytab) are deliberately
	// absent so no secret is read from the database on this unauthenticated path.
	queryGetOrganizationBySlugPublic = `
SELECT o.id, o.slug, o.display_name, COALESCE(o.website_url, ''), COALESCE(o.logo_url, ''), COALESCE(o.logo_dark_url, ''), COALESCE(o.favicon_url, ''),
COALESCE(o.default_avatar, ''), o.status, COALESCE(o.disable_signin, false), COALESCE(o.initiate_login_url, ''), COALESCE(ac.password_type, 'argon2id'),
o.created_at, o.updated_at,
COALESCE(s.default_application_id::text, ''),
COALESCE(t.theme_name, ''), COALESCE(t.tokens, '{}'::jsonb), COALESCE(t.custom_css, ''), COALESCE(t.assets, '{}'::jsonb)
FROM guliangiam.organizations o
LEFT JOIN guliangiam.organization_auth_config ac ON ac.organization_id = o.id
LEFT JOIN guliangiam.organization_settings s ON s.organization_id = o.id
LEFT JOIN guliangiam.organization_theme_settings t ON t.organization_id = o.id
WHERE o.slug = $1 AND o.deleted_at IS NULL`

	queryListOrganizations = `
SELECT o.id, o.slug, o.display_name, o.website_url, o.logo_url, o.logo_dark_url, o.favicon_url, o.status, o.init_score, o.default_avatar, ac.password_obfuscator_type, o.disable_signin, COALESCE(ac.password_options, '{}'), COALESCE(o.enable_soft_deletion, false), COALESCE(o.disable_signup, false), COALESCE(o.ip_whitelist, ''), COALESCE(o.initiate_login_url, ''), COALESCE(bl.balance, 0), COALESCE(bl.currency, 'USD'),
COALESCE(ac.password_type, 'argon2id'), COALESCE(ac.password_salt, ''), COALESCE(ac.password_obfuscator_key, ''), COALESCE(bl.enable_deposit, false), COALESCE(bl.balance_credit, 0), COALESCE(bl.mu, 0), COALESCE(bl.default_quota, 0), COALESCE(bl.default_currency, ''), COALESCE(br.enable_dark_logo, false), COALESCE(br.has_privilege_consent, false), COALESCE(br.is_profile_public, false), COALESCE(ac.use_email_as_username, false), COALESCE(br.use_permanent_avatar, false), COALESCE(br.enable_tour, false), COALESCE(ac.master_password, ''), COALESCE(ac.master_verification_code, ''), COALESCE(ac.default_password, ''), COALESCE(ac.ldap_attributes, '{}'), COALESCE(kb.kerberos_realm, ''), COALESCE(kb.kerberos_kdc_host, ''), COALESCE(kb.kerberos_keytab, ''), COALESCE(kb.kerberos_service_name, ''),
o.created_at, o.updated_at, o.revision,
COALESCE(s.default_application_id::text, ''), o.parent_id, COALESCE(o.region_code, '')
FROM guliangiam.organizations o
LEFT JOIN guliangiam.organization_settings s ON s.organization_id = o.id
LEFT JOIN guliangiam.organization_auth_config ac ON ac.organization_id = o.id
LEFT JOIN guliangiam.organization_kerberos_config kb ON kb.organization_id = o.id
LEFT JOIN guliangiam.organization_billing bl ON bl.organization_id = o.id
LEFT JOIN guliangiam.organization_branding br ON br.organization_id = o.id
WHERE ($1 = '' OR o.slug ILIKE '%' || $1 || '%' OR o.display_name ILIKE '%' || $1 || '%')
  AND ($2 = '' OR ($2 = 'active' AND o.status IN ('active', 'normal')) OR o.status = $2)
  AND o.deleted_at IS NULL
ORDER BY o.created_at DESC
LIMIT $3 OFFSET $4`

	queryCountOrganizations = `
SELECT COUNT(*)
FROM guliangiam.organizations o
WHERE ($1 = '' OR o.slug ILIKE '%' || $1 || '%' OR o.display_name ILIKE '%' || $1 || '%')
  AND ($2 = '' OR ($2 = 'active' AND o.status IN ('active', 'normal')) OR o.status = $2)
  AND o.deleted_at IS NULL`

	queryInsertOrganizationCore = `INSERT INTO guliangiam.organizations (id, slug, display_name, website_url, logo_url, logo_dark_url, favicon_url, status, init_score, default_avatar, disable_signin, enable_soft_deletion, disable_signup, ip_whitelist, initiate_login_url, created_at, updated_at, revision, parent_id, region_code) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NULLIF($19, '')::uuid, $20)`
	queryUpdateOrganizationCore = `UPDATE guliangiam.organizations
SET display_name = $1,
    website_url = $2,
    logo_url = $3,
    logo_dark_url = $4,
    favicon_url = $5,
    status = $6,
    init_score = $7,
    default_avatar = $8,
    disable_signin = $9,
    enable_soft_deletion = $10,
    disable_signup = $11,
    ip_whitelist = $12,
    initiate_login_url = $13,
    parent_id = NULLIF($14, '')::uuid,
    region_code = $16,
    updated_at = $15,
    revision = revision + 1
WHERE id = $18::uuid AND revision = $17 RETURNING revision`
	queryUpsertOrganizationAuthConfig = `INSERT INTO guliangiam.organization_auth_config (organization_id, password_obfuscator_type, password_options, password_type, password_salt, password_obfuscator_key, master_password, master_verification_code, default_password, use_email_as_username, ldap_attributes) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (organization_id)
DO UPDATE SET
    password_obfuscator_type = EXCLUDED.password_obfuscator_type,
    password_options = EXCLUDED.password_options,
    password_type = EXCLUDED.password_type,
    password_salt = EXCLUDED.password_salt,
    password_obfuscator_key = EXCLUDED.password_obfuscator_key,
    master_password = EXCLUDED.master_password,
    master_verification_code = EXCLUDED.master_verification_code,
    default_password = EXCLUDED.default_password,
    use_email_as_username = EXCLUDED.use_email_as_username,
    ldap_attributes = EXCLUDED.ldap_attributes`
	queryUpsertOrganizationKerberosConfig = `INSERT INTO guliangiam.organization_kerberos_config (organization_id, kerberos_realm, kerberos_kdc_host, kerberos_keytab, kerberos_service_name) VALUES ($1::uuid, $2, $3, $4, $5)
ON CONFLICT (organization_id)
DO UPDATE SET
    kerberos_realm = EXCLUDED.kerberos_realm,
    kerberos_kdc_host = EXCLUDED.kerberos_kdc_host,
    kerberos_keytab = EXCLUDED.kerberos_keytab,
    kerberos_service_name = EXCLUDED.kerberos_service_name`
	queryUpsertOrganizationBilling = `INSERT INTO guliangiam.organization_billing (organization_id, balance, currency, enable_deposit, balance_credit, mu, default_quota, default_currency) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (organization_id)
DO UPDATE SET
    balance = EXCLUDED.balance,
    currency = EXCLUDED.currency,
    enable_deposit = EXCLUDED.enable_deposit,
    balance_credit = EXCLUDED.balance_credit,
    mu = EXCLUDED.mu,
    default_quota = EXCLUDED.default_quota,
    default_currency = EXCLUDED.default_currency`
	queryUpsertOrganizationBranding = `INSERT INTO guliangiam.organization_branding (organization_id, enable_dark_logo, has_privilege_consent, is_profile_public, use_permanent_avatar, enable_tour) VALUES ($1::uuid, $2, $3, $4, $5, $6)
ON CONFLICT (organization_id)
DO UPDATE SET
    enable_dark_logo = EXCLUDED.enable_dark_logo,
    has_privilege_consent = EXCLUDED.has_privilege_consent,
    is_profile_public = EXCLUDED.is_profile_public,
    use_permanent_avatar = EXCLUDED.use_permanent_avatar,
    enable_tour = EXCLUDED.enable_tour`

	queryUpsertOrganizationSettings = `
INSERT INTO guliangiam.organization_settings (organization_id, languages, default_application_id, account_items, nav_preferences, feature_flags, country_codes, user_types, tags, mfa_items, account_menu, mfa_remember_in_hours, dcr_policy) VALUES ($1::uuid, $2::jsonb, NULLIF($3, ''), $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10::jsonb, $11, $12, $13)
ON CONFLICT (organization_id)
DO UPDATE SET
    languages = EXCLUDED.languages,
    default_application_id = EXCLUDED.default_application_id,
    account_items = EXCLUDED.account_items,
    nav_preferences = EXCLUDED.nav_preferences,
    feature_flags = EXCLUDED.feature_flags,
    country_codes = EXCLUDED.country_codes,
    user_types = EXCLUDED.user_types,
    tags = EXCLUDED.tags,
    mfa_items = EXCLUDED.mfa_items,
    account_menu = EXCLUDED.account_menu,
    mfa_remember_in_hours = EXCLUDED.mfa_remember_in_hours,
    dcr_policy = EXCLUDED.dcr_policy`

	queryUpsertOrganizationTheme = `
INSERT INTO guliangiam.organization_theme_settings (organization_id, theme_name, tokens, custom_css, assets) VALUES ($1::uuid, $2, $3::jsonb, $4, $5::jsonb)
ON CONFLICT (organization_id)
DO UPDATE SET
    theme_name = EXCLUDED.theme_name,
    tokens = EXCLUDED.tokens,
    custom_css = EXCLUDED.custom_css,
    assets = EXCLUDED.assets`

	queryUpsertOrganizationSecurityPolicy = `
INSERT INTO guliangiam.organization_security_policies (organization_id, password_policy, password_derive_algorithm, mfa_policy, ip_policy, session_policy, risk_policy, lockout_policy, concurrent_session_policy) VALUES ($1::uuid, $2::jsonb, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb)
ON CONFLICT (organization_id)
DO UPDATE SET
    password_policy = EXCLUDED.password_policy,
    password_derive_algorithm = EXCLUDED.password_derive_algorithm,
    mfa_policy = EXCLUDED.mfa_policy,
    ip_policy = EXCLUDED.ip_policy,
    session_policy = EXCLUDED.session_policy,
    risk_policy = EXCLUDED.risk_policy,
    lockout_policy = EXCLUDED.lockout_policy,
    concurrent_session_policy = EXCLUDED.concurrent_session_policy`

	queryDeleteOrganization = `UPDATE guliangiam.organizations SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1::uuid AND deleted_at IS NULL`

	queryHasDeletedWithSlug = `SELECT EXISTS(SELECT 1 FROM guliangiam.organizations WHERE slug = $1 AND deleted_at IS NOT NULL)`

	// Narrow projection for the authentication read path: one column, one row.
	// The join on organizations preserves the same "organization not found"
	// semantics as the aggregate query while avoiding the eight-way join and
	// the decryption of organization secrets.
	queryGetOrganizationSessionPolicy = `
SELECT COALESCE(p.session_policy, '{}'::jsonb)
FROM guliangiam.organizations o
LEFT JOIN guliangiam.organization_security_policies p ON p.organization_id = o.id
WHERE o.id = $1::uuid AND o.deleted_at IS NULL`
)
