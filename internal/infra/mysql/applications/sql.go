package applications

import (
	"github.com/guliangiam/guliangiam/internal/infra/dialect"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

const (
	btApp              = "`applications`"
	btOAuthConfig      = "`app_oauth_config`"
	btSigninConfig     = "`app_signin_config`"
	btSAMLConfig       = "`app_saml_config`"
	btFormConfig       = "`app_form_config`"
	btNetworkConfig    = "`app_network_config`"
	btProviderBindings = "`application_provider_bindings`"
	btConsents         = "`consents`"
)

// applicationAggregateSelect 按 domain.Application 的字段顺序读取主表与五张配置表。
// JSON_ARRAY() 为缺失配置行提供与 PG 空数组/空 JSON 一致的默认值。
const applicationAggregateSelect = `
SELECT apps.id,
	apps.organization_id,
	apps.slug,
	apps.display_name,
	apps.category,
	apps.app_type,
	apps.description,
	apps.homepage_url,
	apps.domain,
	apps.status,
	apps.client_id,
	COALESCE(oauth.redirect_uris, JSON_ARRAY()),
	COALESCE(oauth.grant_types, JSON_ARRAY()),
	oauth.client_secret_hash,
	COALESCE(oauth.token_endpoint_auth_method, ''),
	COALESCE(oauth.custom_scopes, JSON_ARRAY()),
	COALESCE(signin.disable_signin, FALSE),
	apps.tags,
	COALESCE(signin.enable_exclusive_signin, FALSE),
	COALESCE(signin.enable_signin_session, FALSE),
	apps.logo,
	COALESCE(saml.enable_saml_compress, FALSE),
	COALESCE(form.signup_items, JSON_ARRAY()),
	COALESCE(oauth.token_format, 'JWT'),
	COALESCE(oauth.expire_in_hours, 24),
	COALESCE(oauth.refresh_token_expire_in_hours, 168),
	COALESCE(oauth.token_signing_method, 'RS256'),
	COALESCE(net.provider_items, JSON_ARRAY()),
	COALESCE(saml.saml_reply_url, ''),
	COALESCE(saml.certificate, ''),
	COALESCE(form.custom_css, ''),
	COALESCE(form.custom_header, ''),
	COALESCE(signin.signin_url, ''),
	COALESCE(signin.logout_url, ''),
	COALESCE(signin.logout_redirect_url, ''),
	apps.is_portal,
	apps.is_visible,
	COALESCE(signin.prompt_url, ''),
	COALESCE(saml.saml_metadata, ''),
	apps.is_shared,
	apps.title,
	apps.favicon,
	apps.menu_order,
	apps.menu_mode,
	COALESCE(signin.enable_signup, TRUE),
	COALESCE(signin.enable_guest_signin, FALSE),
	COALESCE(signin.enable_auto_signin, TRUE),
	COALESCE(signin.enable_link_with_email, FALSE),
	COALESCE(oauth.cookie_expire_in_hours, 168),
	COALESCE(signin.signup_url, ''),
	COALESCE(signin.forget_url, ''),
	COALESCE(signin.affiliation_url, ''),
	COALESCE(oauth.token_fields, JSON_ARRAY()),
	COALESCE(oauth.token_attributes, JSON_ARRAY()),
	COALESCE(net.forced_redirect_origin, ''),
	COALESCE(signin.code_expire, 0),
	COALESCE(saml.saml_name_id_format, ''),
	COALESCE(saml.saml_name_id_content, ''),
	COALESCE(saml.saml_sign_request, FALSE),
	COALESCE(saml.saml_metadata_url, ''),
	COALESCE(saml.saml_attributes, JSON_ARRAY()),
	COALESCE(saml.enable_saml_c14n10, FALSE),
	COALESCE(saml.enable_saml_post_binding, FALSE),
	COALESCE(saml.disable_saml_attributes, FALSE),
	COALESCE(saml.enable_saml_assertion_signature, FALSE),
	COALESCE(saml.is_email_saml_nameid, FALSE),
	COALESCE(oauth.consent_mode, ''),
	COALESCE(oauth.dpop_required, FALSE),
	COALESCE(oauth.pkce_required, FALSE),
	COALESCE(signin.ip_whitelist, ''),
	COALESCE(signin.ip_blacklist, ''),
	COALESCE(signin.failed_signin_limit, 0),
	COALESCE(signin.failed_signin_frozen_time, 0),
	COALESCE(signin.code_resend_timeout, 0),
	COALESCE(signin.cors_allowed_origins, JSON_ARRAY()),
	COALESCE(oauth.enable_beta_oidc, FALSE),
	COALESCE(signin.default_group, ''),
	COALESCE(signin.org_choice_mode, ''),
	COALESCE(form.form_css, ''),
	COALESCE(form.form_css_mobile, ''),
	COALESCE(form.form_side_html, ''),
	COALESCE(form.form_background_url, ''),
	COALESCE(form.form_background_url_mobile, ''),
	COALESCE(form.form_offset, ''),
	COALESCE(form.form_position, ''),
	COALESCE(form.header_html, ''),
	COALESCE(form.footer_html, ''),
	COALESCE(form.signup_html, ''),
	COALESCE(form.signin_html, ''),
	COALESCE(form.terms_of_use, ''),
	COALESCE(form.privacy_policy, ''),
	COALESCE(oauth.backchannel_logout_uri, ''),
	COALESCE(oauth.frontchannel_logout_uri, ''),
	COALESCE(oauth.frontchannel_logout_session_required, FALSE),
	COALESCE(net.other_domains, JSON_ARRAY()),
	COALESCE(net.upstream_host, ''),
	COALESCE(net.ssl_mode, ''),
	COALESCE(net.ssl_cert, ''),
	COALESCE(net.client_cert, ''),
	COALESCE(saml.saml_hash_algorithm, ''),
	COALESCE(saml.saml_c14n_prefix, ''),
	COALESCE(form.page_html, ''),
	COALESCE(signin.signin_methods, JSON_ARRAY()),
	COALESCE(signin.signin_items, JSON_ARRAY()),
	COALESCE(form.custom_js, ''),
	apps.created_at,
	apps.updated_at,
	apps.deleted_at,
	apps.created_by,
	apps.updated_by,
	apps.revision
FROM ` + btApp + ` apps
LEFT JOIN ` + btOAuthConfig + ` oauth ON oauth.application_id = apps.id
LEFT JOIN ` + btSigninConfig + ` signin ON signin.application_id = apps.id
LEFT JOIN ` + btSAMLConfig + ` saml ON saml.application_id = apps.id
LEFT JOIN ` + btFormConfig + ` form ON form.application_id = apps.id
LEFT JOIN ` + btNetworkConfig + ` net ON net.application_id = apps.id
`

const (
	queryGetApplicationByID = applicationAggregateSelect + `
WHERE apps.id = ? AND apps.deleted_at IS NULL`

	// queryGetApplicationByIDForOrganization 是管理面 by-ID 路由的读取口（MySQL 无
	// RLS，租户谓词是唯一的隔离层）；跨租户 id 与不存在都返回 0 行。
	queryGetApplicationByIDForOrganization = applicationAggregateSelect + `
WHERE apps.id = ? AND apps.organization_id = ? AND apps.deleted_at IS NULL`

	queryGetApplicationByClientID = applicationAggregateSelect + `
WHERE apps.client_id = ? AND apps.deleted_at IS NULL`

	queryGetApplicationByOrganizationAndSlug = applicationAggregateSelect + `
WHERE apps.organization_id = ? AND apps.slug = ? AND apps.deleted_at IS NULL`

	queryGetApplicationProviderBindingsByApplicationID = `
SELECT application_id, provider_id
FROM ` + btProviderBindings + `
WHERE application_id = ?
ORDER BY provider_id ASC`

	queryBatchGetApplicationProviderBindingsByApplicationIDs = `
SELECT application_id, provider_id
FROM ` + btProviderBindings + `
WHERE application_id IN (%s)
ORDER BY application_id, provider_id ASC`

	queryListApplications = applicationAggregateSelect + `
WHERE (? = '' OR apps.organization_id = ?)
  AND (? = '' OR apps.slug LIKE CONCAT('%', ?, '%') OR apps.display_name LIKE CONCAT('%', ?, '%') OR apps.domain LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR apps.status = ?)
  AND (? = '' OR apps.category = ?)
  AND apps.deleted_at IS NULL
ORDER BY apps.created_at DESC
LIMIT ? OFFSET ?`

	queryCountApplications = `
SELECT COUNT(*)
FROM ` + btApp + ` apps
WHERE (? = '' OR apps.organization_id = ?)
  AND (? = '' OR apps.slug LIKE CONCAT('%', ?, '%') OR apps.display_name LIKE CONCAT('%', ?, '%') OR apps.domain LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR apps.status = ?)
  AND (? = '' OR apps.category = ?)
  AND apps.deleted_at IS NULL`

	queryInsertApplication = `
INSERT INTO ` + btApp + ` (
	id, organization_id, slug, display_name, category, app_type, description, homepage_url, domain, status,
	client_id, tags, logo, is_portal, is_visible, is_shared, title, favicon,
	menu_order, menu_mode, created_at, updated_at, revision
) VALUES (
	?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?
)`

	queryUpdateApplication = `
UPDATE ` + btApp + `
SET slug = ?,
    display_name = ?,
    category = ?,
    app_type = ?,
    description = ?,
    homepage_url = ?,
    domain = ?,
    status = ?,
    client_id = ?,
    tags = ?,
    logo = ?,
    is_portal = ?,
    is_visible = ?,
    is_shared = ?,
    title = ?,
    favicon = ?,
    menu_order = ?,
    menu_mode = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ? AND deleted_at IS NULL`

	// querySelectRevisionUpdate 回读 revision；organization_id 谓词防止 MySQL
	// （无 RLS）在 id 复用时读到其他租户的行。
	querySelectRevisionUpdate = `SELECT revision FROM ` + btApp + ` WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

	queryDeleteApplicationProviderBindings = `
DELETE FROM ` + btProviderBindings + `
WHERE application_id = ?`

	queryDeleteApplicationConsents = `
DELETE FROM ` + btConsents + `
WHERE application_id = ?`

	// queryDeleteApplication 是软删除；organization_id 谓词保证跨租户 id 返回 0 行。
	queryDeleteApplication = `
UPDATE ` + btApp + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

	querySetRegistrationAccessToken = `
UPDATE ` + btOAuthConfig + ` oauth
JOIN ` + btApp + ` apps ON apps.id = oauth.application_id
SET oauth.registration_access_token_hash = ?
WHERE apps.id = ? AND apps.deleted_at IS NULL`

	queryGetRegistrationAccessTokenHash = `
SELECT oauth.registration_access_token_hash
FROM ` + btApp + ` apps
JOIN ` + btOAuthConfig + ` oauth ON oauth.application_id = apps.id
WHERE apps.client_id = ? AND apps.deleted_at IS NULL`

	queryHasDeletedWithOrgAndSlug = `SELECT EXISTS(SELECT 1 FROM ` + btApp + ` WHERE organization_id = ? AND slug = ? AND deleted_at IS NOT NULL)`
)

var (
	queryInsertApplicationProviderBinding = mysqlshared.Dialect.Upsert(dialect.DialectUpsert{DoNothing: true}, nil) + `
INTO ` + btProviderBindings + ` (application_id, provider_id) VALUES (?, ?)`

	// registration_access_token_hash 由窄写方法单独维护，宽更新不得清空它。
	// token_signing_method 的 COALESCE 与列默认值同义：该列 NOT NULL，空串直接写入会
	// 覆盖 DEFAULT 'RS256'；NULLIF 单独使用会把空串变成 NULL 而违反 NOT NULL。
	queryUpsertOAuthConfig = `
INSERT INTO ` + btOAuthConfig + ` (
	application_id, redirect_uris, grant_types, token_endpoint_auth_method, client_secret_hash,
	custom_scopes, token_format, expire_in_hours, refresh_token_expire_in_hours,
	token_signing_method, token_fields, token_attributes, dpop_required, pkce_required,
	cookie_expire_in_hours, enable_beta_oidc, consent_mode, backchannel_logout_uri,
	frontchannel_logout_uri, frontchannel_logout_session_required
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), 'RS256'), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"redirect_uris", "grant_types", "token_endpoint_auth_method", "client_secret_hash",
		"custom_scopes", "token_format", "expire_in_hours", "refresh_token_expire_in_hours",
		"token_signing_method", "token_fields", "token_attributes", "dpop_required", "pkce_required",
		"cookie_expire_in_hours", "enable_beta_oidc", "consent_mode", "backchannel_logout_uri",
		"frontchannel_logout_uri", "frontchannel_logout_session_required",
	}}, nil)

	queryUpsertSigninConfig = `
INSERT INTO ` + btSigninConfig + ` (
	application_id, disable_signin, enable_exclusive_signin, enable_signin_session,
	enable_signup, enable_guest_signin, enable_auto_signin, enable_link_with_email,
	signin_url, logout_url,
	logout_redirect_url, prompt_url, signup_url, forget_url, affiliation_url,
	signin_methods, signin_items, failed_signin_limit, failed_signin_frozen_time,
	code_resend_timeout, code_expire, default_group, org_choice_mode,
	ip_whitelist, ip_blacklist, cors_allowed_origins
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"disable_signin", "enable_exclusive_signin", "enable_signin_session",
		"enable_signup", "enable_guest_signin", "enable_auto_signin", "enable_link_with_email",
		"signin_url", "logout_url",
		"logout_redirect_url", "prompt_url", "signup_url", "forget_url", "affiliation_url",
		"signin_methods", "signin_items", "failed_signin_limit", "failed_signin_frozen_time",
		"code_resend_timeout", "code_expire", "default_group", "org_choice_mode",
		"ip_whitelist", "ip_blacklist", "cors_allowed_origins",
	}}, nil)

	queryUpsertSAMLConfig = `
INSERT INTO ` + btSAMLConfig + ` (
	application_id, saml_reply_url, certificate, saml_metadata, saml_metadata_url,
	enable_saml_compress, saml_name_id_format, saml_name_id_content, saml_sign_request,
	saml_attributes, enable_saml_c14n10, enable_saml_post_binding, disable_saml_attributes,
	enable_saml_assertion_signature, is_email_saml_nameid, saml_hash_algorithm, saml_c14n_prefix
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"saml_reply_url", "certificate", "saml_metadata", "saml_metadata_url",
		"enable_saml_compress", "saml_name_id_format", "saml_name_id_content", "saml_sign_request",
		"saml_attributes", "enable_saml_c14n10", "enable_saml_post_binding", "disable_saml_attributes",
		"enable_saml_assertion_signature", "is_email_saml_nameid", "saml_hash_algorithm", "saml_c14n_prefix",
	}}, nil)

	queryUpsertFormConfig = `
INSERT INTO ` + btFormConfig + ` (
	application_id, signup_items, custom_css, custom_header, form_css, form_css_mobile,
	form_side_html, form_background_url, form_background_url_mobile, form_offset,
	form_position, header_html, footer_html, signup_html, signin_html, page_html,
	custom_js, terms_of_use, privacy_policy
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"signup_items", "custom_css", "custom_header", "form_css", "form_css_mobile",
		"form_side_html", "form_background_url", "form_background_url_mobile", "form_offset",
		"form_position", "header_html", "footer_html", "signup_html", "signin_html", "page_html",
		"custom_js", "terms_of_use", "privacy_policy",
	}}, nil)

	queryUpsertNetworkConfig = `
INSERT INTO ` + btNetworkConfig + ` (
	application_id, provider_items, forced_redirect_origin, other_domains,
	upstream_host, ssl_mode, ssl_cert, client_cert
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{
		"provider_items", "forced_redirect_origin", "other_domains",
		"upstream_host", "ssl_mode", "ssl_cert", "client_cert",
	}}, nil)
)
