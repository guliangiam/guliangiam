package organizations

import (
	"github.com/guliangiam/guliangiam/internal/infra/dialect"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

const (
	btOrgs     = "`organizations`"
	btAuth     = "`organization_auth_config`"
	btKerberos = "`organization_kerberos_config`"
	btBilling  = "`organization_billing`"
	btBranding = "`organization_branding`"
	btSettings = "`organization_settings`"
	btTheme    = "`organization_theme_settings`"
	btSecurity = "`organization_security_policies`"
)

// organizationAggregateSelect 返回 44 个组织基础字段及 settings/theme/security 的 26 个字段。
// 基础字段分别来自 organizations、auth、kerberos、billing、branding，不再读取已移除的主表旧列。
const organizationAggregateSelect = `
SELECT o.id,
	o.slug,
	o.display_name,
	o.website_url,
	o.logo_url,
	o.logo_dark_url,
	o.favicon_url,
	o.status,
	o.init_score,
	o.default_avatar,
	ac.password_obfuscator_type,
	o.disable_signin,
	COALESCE(ac.password_options, JSON_ARRAY()),
	COALESCE(o.enable_soft_deletion, FALSE),
	COALESCE(o.disable_signup, FALSE),
	COALESCE(o.ip_whitelist, ''),
	COALESCE(o.initiate_login_url, ''),
	COALESCE(bl.balance, 0),
	COALESCE(bl.currency, 'USD'),
	COALESCE(ac.password_type, 'argon2id'),
	COALESCE(ac.password_salt, ''),
	COALESCE(ac.password_obfuscator_key, ''),
	COALESCE(bl.enable_deposit, FALSE),
	COALESCE(bl.balance_credit, 0),
	COALESCE(bl.mu, 0),
	COALESCE(bl.default_quota, 0),
	COALESCE(bl.default_currency, ''),
	COALESCE(br.enable_dark_logo, FALSE),
	COALESCE(br.has_privilege_consent, FALSE),
	COALESCE(br.is_profile_public, FALSE),
	COALESCE(ac.use_email_as_username, FALSE),
	COALESCE(br.use_permanent_avatar, FALSE),
	COALESCE(br.enable_tour, FALSE),
	COALESCE(ac.master_password, ''),
	COALESCE(ac.master_verification_code, ''),
	COALESCE(ac.default_password, ''),
	COALESCE(ac.ldap_attributes, JSON_ARRAY()),
	COALESCE(kb.kerberos_realm, ''),
	COALESCE(kb.kerberos_kdc_host, ''),
	COALESCE(kb.kerberos_keytab, ''),
	COALESCE(kb.kerberos_service_name, ''),
	o.created_at,
	o.updated_at,
	o.revision,
	s.organization_id,
	s.languages,
	s.default_application_id,
	s.account_items,
	s.nav_preferences,
	s.feature_flags,
	s.country_codes,
	s.user_types,
	s.tags,
	s.mfa_items,
	s.account_menu,
	s.mfa_remember_in_hours,
	s.dcr_policy,
	t.organization_id,
	t.theme_name,
	t.tokens,
	t.custom_css,
	t.assets,
	p.organization_id,
	p.password_policy,
	p.password_derive_algorithm,
	p.mfa_policy,
	p.ip_policy,
	p.session_policy,
	p.risk_policy,
	p.lockout_policy,
	p.concurrent_session_policy,
	o.parent_id,
	o.region_code
FROM ` + btOrgs + ` o
LEFT JOIN ` + btSettings + ` s ON s.organization_id = o.id
LEFT JOIN ` + btTheme + ` t ON t.organization_id = o.id
LEFT JOIN ` + btSecurity + ` p ON p.organization_id = o.id
LEFT JOIN ` + btAuth + ` ac ON ac.organization_id = o.id
LEFT JOIN ` + btKerberos + ` kb ON kb.organization_id = o.id
LEFT JOIN ` + btBilling + ` bl ON bl.organization_id = o.id
LEFT JOIN ` + btBranding + ` br ON br.organization_id = o.id
`

const (
	queryGetOrganizationByID = organizationAggregateSelect + `
WHERE o.id = ? AND o.deleted_at IS NULL`

	queryGetOrganizationBySlug = organizationAggregateSelect + `
WHERE o.slug = ? AND o.deleted_at IS NULL`

	// queryGetOrganizationBySlugPublic feeds the anonymous /by-slug and /resolve
	// discovery endpoints. Only public columns are selected: branding, theme and
	// the default application. Credential-bearing columns are deliberately
	// absent so the unauthenticated path never reads them from the database.
	queryGetOrganizationBySlugPublic = `
SELECT o.id,
	o.slug,
	o.display_name,
	COALESCE(o.website_url, ''),
	COALESCE(o.logo_url, ''),
	COALESCE(o.logo_dark_url, ''),
	COALESCE(o.favicon_url, ''),
	COALESCE(o.default_avatar, ''),
	o.status,
	COALESCE(o.disable_signin, FALSE),
	COALESCE(o.initiate_login_url, ''),
	COALESCE(ac.password_type, 'argon2id'),
	o.created_at,
	o.updated_at,
	COALESCE(s.default_application_id, ''),
	COALESCE(t.theme_name, ''),
	COALESCE(t.tokens, JSON_OBJECT()),
	COALESCE(t.custom_css, ''),
	COALESCE(t.assets, JSON_OBJECT())
FROM ` + btOrgs + ` o
LEFT JOIN ` + btAuth + ` ac ON ac.organization_id = o.id
LEFT JOIN ` + btSettings + ` s ON s.organization_id = o.id
LEFT JOIN ` + btTheme + ` t ON t.organization_id = o.id
WHERE o.slug = ? AND o.deleted_at IS NULL`

	queryListOrganizations = organizationAggregateSelect + `
WHERE (? = '' OR o.slug LIKE CONCAT('%', ?, '%') OR o.display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR o.status = ?)
  AND o.deleted_at IS NULL
ORDER BY o.created_at DESC
LIMIT ? OFFSET ?`

	queryCountOrganizations = `
SELECT COUNT(*)
FROM ` + btOrgs + ` o
WHERE (? = '' OR o.slug LIKE CONCAT('%', ?, '%') OR o.display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR o.status = ?)
  AND o.deleted_at IS NULL`

	queryInsertOrganization = `
INSERT INTO ` + btOrgs + ` (
	id, slug, display_name, website_url, logo_url, logo_dark_url, favicon_url,
	status, init_score, default_avatar, disable_signin, enable_soft_deletion,
	disable_signup, ip_whitelist, initiate_login_url, created_at, updated_at, revision, parent_id, region_code
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)`

	queryUpdateOrganization = `
UPDATE ` + btOrgs + `
SET display_name = ?,
    website_url = ?,
    logo_url = ?,
    logo_dark_url = ?,
    favicon_url = ?,
    status = ?,
    init_score = ?,
    default_avatar = ?,
    disable_signin = ?,
    enable_soft_deletion = ?,
    disable_signup = ?,
    ip_whitelist = ?,
    initiate_login_url = ?,
    parent_id = NULLIF(?, ''),
    updated_at = ?,
    region_code = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND deleted_at IS NULL`

	querySelectRevision = `SELECT revision FROM ` + btOrgs + ` WHERE id = ? AND deleted_at IS NULL`

	queryDeleteOrganization = `UPDATE ` + btOrgs + ` SET deleted_at = NOW(), updated_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	queryHasDeletedWithSlug = `SELECT EXISTS(SELECT 1 FROM ` + btOrgs + ` WHERE slug = ? AND deleted_at IS NOT NULL)`

	// Narrow projection for the authentication read path: one column, one row.
	// The join on organizations preserves the same "organization not found"
	// semantics as the aggregate query.
	queryGetOrganizationSessionPolicy = `
SELECT COALESCE(p.session_policy, JSON_OBJECT())
FROM ` + btOrgs + ` o
LEFT JOIN ` + btSecurity + ` p ON p.organization_id = o.id
WHERE o.id = ? AND o.deleted_at IS NULL`
)

var (
	queryUpsertOrganizationAuthConfig = `
INSERT INTO ` + btAuth + ` (
	organization_id, password_obfuscator_type, password_options, password_type, password_salt,
	password_obfuscator_key, master_password, master_verification_code, default_password,
	use_email_as_username, ldap_attributes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"password_obfuscator_type", "password_options", "password_type", "password_salt",
		"password_obfuscator_key", "master_password", "master_verification_code", "default_password",
		"use_email_as_username", "ldap_attributes",
	}}, nil)

	queryUpsertOrganizationKerberosConfig = `
INSERT INTO ` + btKerberos + ` (
	organization_id, kerberos_realm, kerberos_kdc_host, kerberos_keytab, kerberos_service_name
) VALUES (?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"kerberos_realm", "kerberos_kdc_host", "kerberos_keytab", "kerberos_service_name",
	}}, nil)

	queryUpsertOrganizationBilling = `
INSERT INTO ` + btBilling + ` (
	organization_id, balance, currency, enable_deposit, balance_credit, mu, default_quota, default_currency
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"balance", "currency", "enable_deposit", "balance_credit", "mu", "default_quota", "default_currency",
	}}, nil)

	queryUpsertOrganizationBranding = `
INSERT INTO ` + btBranding + ` (
	organization_id, enable_dark_logo, has_privilege_consent, is_profile_public, use_permanent_avatar, enable_tour
) VALUES (?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"enable_dark_logo", "has_privilege_consent", "is_profile_public", "use_permanent_avatar", "enable_tour",
	}}, nil)

	queryUpsertOrganizationSettings = `
INSERT INTO ` + btSettings + ` (
	organization_id, languages, default_application_id, account_items, nav_preferences,
	feature_flags, country_codes, user_types, tags, mfa_items, account_menu,
	mfa_remember_in_hours, dcr_policy
) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"languages", "default_application_id", "account_items", "nav_preferences", "feature_flags",
		"country_codes", "user_types", "tags", "mfa_items", "account_menu", "mfa_remember_in_hours", "dcr_policy",
	}}, nil)

	queryUpsertOrganizationTheme = `
INSERT INTO ` + btTheme + ` (organization_id, theme_name, tokens, custom_css, assets)
VALUES (?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"theme_name", "tokens", "custom_css", "assets",
	}}, nil)

	queryUpsertOrganizationSecurityPolicy = `
INSERT INTO ` + btSecurity + ` (
	organization_id, password_policy, password_derive_algorithm, mfa_policy, ip_policy, session_policy,
	risk_policy, lockout_policy, concurrent_session_policy
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"password_policy", "password_derive_algorithm", "mfa_policy", "ip_policy", "session_policy",
		"risk_policy", "lockout_policy", "concurrent_session_policy",
	}}, nil)
)
