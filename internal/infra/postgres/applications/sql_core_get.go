package applications

// Auto-split from sql.go — domain: core_get

const (
	queryApplicationAggregateSelect = `
SELECT apps.id,
	apps.organization_id,
	apps.slug,
	apps.display_name,
	apps.category,
	apps.app_type,
	COALESCE(apps.description, ''),
	COALESCE(apps.homepage_url, ''),
	COALESCE(apps.domain, ''),
	apps.status,
	COALESCE(apps.client_id, ''),
	COALESCE(oauth.redirect_uris, ARRAY[]::text[]),
	COALESCE(oauth.grant_types, ARRAY[]::text[]),
	oauth.client_secret_hash,
	COALESCE(oauth.token_endpoint_auth_method, ''),
	COALESCE(oauth.custom_scopes, '[]'::jsonb),
	COALESCE(signin.disable_signin, false),
	apps.tags,
	COALESCE(signin.enable_exclusive_signin, false),
	COALESCE(signin.enable_signin_session, false),
	apps.logo,
	COALESCE(saml.enable_saml_compress, false),
	COALESCE(form.signup_items, '[]'::jsonb),
	COALESCE(oauth.token_format, 'JWT'),
	COALESCE(oauth.expire_in_hours, 24),
	COALESCE(oauth.refresh_token_expire_in_hours, 168),
	COALESCE(oauth.token_signing_method, 'RS256'),
	COALESCE(net.provider_items, '[]'::jsonb),
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
	COALESCE(signin.enable_signup, true),
	COALESCE(signin.enable_guest_signin, false),
	COALESCE(signin.enable_auto_signin, true),
	COALESCE(signin.enable_link_with_email, false),
	COALESCE(oauth.cookie_expire_in_hours, 168),
	COALESCE(signin.signup_url, ''),
	COALESCE(signin.forget_url, ''),
	COALESCE(signin.affiliation_url, ''),
	COALESCE(oauth.token_fields, '[]'::jsonb),
	COALESCE(oauth.token_attributes, '[]'::jsonb),
	COALESCE(net.forced_redirect_origin, ''),
	COALESCE(signin.code_expire, 0),
	COALESCE(saml.saml_name_id_format, ''),
	COALESCE(saml.saml_name_id_content, ''),
	COALESCE(saml.saml_sign_request, false),
	COALESCE(saml.saml_metadata_url, ''),
	COALESCE(saml.saml_attributes, '[]'::jsonb),
	COALESCE(saml.enable_saml_c14n10, false),
	COALESCE(saml.enable_saml_post_binding, false),
	COALESCE(saml.disable_saml_attributes, false),
	COALESCE(saml.enable_saml_assertion_signature, false),
	COALESCE(saml.is_email_saml_nameid, false),
	COALESCE(oauth.consent_mode, ''),
	COALESCE(oauth.dpop_required, false),
	COALESCE(oauth.pkce_required, false),
	COALESCE(signin.ip_whitelist, ''),
	COALESCE(signin.ip_blacklist, ''),
	COALESCE(signin.failed_signin_limit, 0),
	COALESCE(signin.failed_signin_frozen_time, 0),
	COALESCE(signin.code_resend_timeout, 0),
	COALESCE(array_to_string(signin.cors_allowed_origins, ','), ''),
	COALESCE(oauth.enable_beta_oidc, false),
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
		COALESCE(oauth.frontchannel_logout_session_required, false),
	COALESCE(net.other_domains, ARRAY[]::text[]),
	COALESCE(net.upstream_host, ''),
	COALESCE(net.ssl_mode, ''),
	COALESCE(net.ssl_cert, ''),
	COALESCE(net.client_cert, ''),
	COALESCE(saml.saml_hash_algorithm, ''),
	COALESCE(saml.saml_c14n_prefix, ''),
	COALESCE(form.page_html, ''),
	COALESCE(to_jsonb(signin.signin_methods), '[]'::jsonb),
	COALESCE(signin.signin_items, '[]'::jsonb),
	COALESCE(form.custom_js, ''),
	apps.created_at,
	apps.updated_at,
	apps.deleted_at,
	apps.created_by,
	apps.updated_by,
	apps.revision
FROM guliangiam.applications apps
LEFT JOIN guliangiam.app_oauth_config oauth ON oauth.application_id = apps.id
	LEFT JOIN guliangiam.app_signin_config signin ON signin.application_id = apps.id
	LEFT JOIN guliangiam.app_saml_config saml ON saml.application_id = apps.id
	LEFT JOIN guliangiam.app_form_config form ON form.application_id = apps.id
	LEFT JOIN guliangiam.app_network_config net ON net.application_id = apps.id
`

	queryGetApplicationByID = queryApplicationAggregateSelect + `
WHERE apps.id = $1::uuid AND deleted_at IS NULL`

	// queryGetApplicationByIDForOrganization 是管理面 by-ID 路由的读取口：
	// 跨租户 id 与不存在都返回 0 行，调用方统一映射为 NotFound（不可探测）。
	queryGetApplicationByIDForOrganization = queryApplicationAggregateSelect + `
WHERE apps.id = $1::uuid
  AND apps.organization_id = guliangiam.resolve_org_id($2)
  AND deleted_at IS NULL`

	queryGetApplicationByClientID = queryApplicationAggregateSelect + `
WHERE apps.client_id = $1 AND deleted_at IS NULL`

	queryGetApplicationByOrganizationAndSlug = queryApplicationAggregateSelect + `
WHERE apps.organization_id = $1::uuid AND slug = $2 AND deleted_at IS NULL`
)
