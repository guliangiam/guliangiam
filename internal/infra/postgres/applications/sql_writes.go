package applications

// Auto-split from sql.go — domain: writes

const (
	queryInsertApplicationCore = `
INSERT INTO guliangiam.applications (
	id, organization_id, slug, display_name, category, app_type, description, homepage_url, domain, status,
	client_id, tags, logo, is_portal, is_visible, is_shared, title, favicon,
	menu_order, menu_mode, created_at, updated_at, revision
) VALUES (
	$1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10,
	$11, $12::varchar[], $13, $14, $15, $16, $17, $18,
	$19, $20, $21, $22, $23
)`

	queryInsertOAuthConfig = `
INSERT INTO guliangiam.app_oauth_config (
	application_id, redirect_uris, grant_types, token_endpoint_auth_method, client_secret_hash,
	registration_access_token_hash, custom_scopes, token_format, expire_in_hours,
	refresh_token_expire_in_hours, token_signing_method, token_fields, token_attributes,
	dpop_required, pkce_required, cookie_expire_in_hours, enable_beta_oidc,
	consent_mode, backchannel_logout_uri,
	frontchannel_logout_uri, frontchannel_logout_session_required
) VALUES (
	$1::uuid, $2::text[], $3::text[], $4::varchar, $5::varchar, $6::varchar, $7::jsonb, $8::varchar, $9::int4, $10::int4, COALESCE(NULLIF($11::varchar, ''), 'RS256'), $12::jsonb, $13::jsonb, $14::bool, $15::bool, $16::int4, $17::bool, $18::varchar, $19::varchar, $20::varchar, $21::bool
) ON CONFLICT (application_id) DO UPDATE SET
	redirect_uris = EXCLUDED.redirect_uris,
	grant_types = EXCLUDED.grant_types,
	token_endpoint_auth_method = EXCLUDED.token_endpoint_auth_method,
	client_secret_hash = EXCLUDED.client_secret_hash,
	registration_access_token_hash = EXCLUDED.registration_access_token_hash,
	custom_scopes = EXCLUDED.custom_scopes,
	token_format = EXCLUDED.token_format,
	expire_in_hours = EXCLUDED.expire_in_hours,
	refresh_token_expire_in_hours = EXCLUDED.refresh_token_expire_in_hours,
	token_signing_method = EXCLUDED.token_signing_method,
	token_fields = EXCLUDED.token_fields,
	token_attributes = EXCLUDED.token_attributes,
	dpop_required = EXCLUDED.dpop_required,
	pkce_required = EXCLUDED.pkce_required,
	cookie_expire_in_hours = EXCLUDED.cookie_expire_in_hours,
	enable_beta_oidc = EXCLUDED.enable_beta_oidc,
	consent_mode = EXCLUDED.consent_mode,
	backchannel_logout_uri = EXCLUDED.backchannel_logout_uri,
	frontchannel_logout_uri = EXCLUDED.frontchannel_logout_uri,
	frontchannel_logout_session_required = EXCLUDED.frontchannel_logout_session_required`

	queryInsertSigninConfig = `
INSERT INTO guliangiam.app_signin_config (
	application_id, disable_signin, enable_exclusive_signin, enable_signin_session,
	enable_signup, enable_guest_signin, enable_auto_signin, enable_link_with_email,
	signin_url, logout_url,
	logout_redirect_url, prompt_url, signup_url, forget_url, affiliation_url,
	signin_methods, signin_items, failed_signin_limit, failed_signin_frozen_time,
	code_resend_timeout, code_expire, default_group, org_choice_mode,
	ip_whitelist, ip_blacklist, cors_allowed_origins
) VALUES (
	$1::uuid, $2::bool, $3::bool, $4::bool, $5::bool, $6::bool, $7::bool, $8::bool, $9::varchar, $10::varchar, $11::varchar, $12::varchar, $13::varchar, $14::varchar, $15::varchar, $16::text[], $17::jsonb, $18::int4, $19::int4, $20::int4, $21::int4, $22::varchar, $23::varchar, $24::varchar, $25::varchar, $26::varchar[]
) ON CONFLICT (application_id) DO UPDATE SET
	disable_signin = EXCLUDED.disable_signin,
	enable_exclusive_signin = EXCLUDED.enable_exclusive_signin,
	enable_signin_session = EXCLUDED.enable_signin_session,
	enable_signup = EXCLUDED.enable_signup,
	enable_guest_signin = EXCLUDED.enable_guest_signin,
	enable_auto_signin = EXCLUDED.enable_auto_signin,
	enable_link_with_email = EXCLUDED.enable_link_with_email,
	signin_url = EXCLUDED.signin_url,
	logout_url = EXCLUDED.logout_url,
	logout_redirect_url = EXCLUDED.logout_redirect_url,
	prompt_url = EXCLUDED.prompt_url,
	signup_url = EXCLUDED.signup_url,
	forget_url = EXCLUDED.forget_url,
	affiliation_url = EXCLUDED.affiliation_url,
	signin_methods = EXCLUDED.signin_methods,
	signin_items = EXCLUDED.signin_items,
	failed_signin_limit = EXCLUDED.failed_signin_limit,
	failed_signin_frozen_time = EXCLUDED.failed_signin_frozen_time,
	code_resend_timeout = EXCLUDED.code_resend_timeout,
	code_expire = EXCLUDED.code_expire,
	default_group = EXCLUDED.default_group,
	org_choice_mode = EXCLUDED.org_choice_mode,
	ip_whitelist = EXCLUDED.ip_whitelist,
	ip_blacklist = EXCLUDED.ip_blacklist,
	cors_allowed_origins = EXCLUDED.cors_allowed_origins`

	queryInsertSamlConfig = `
INSERT INTO guliangiam.app_saml_config (
	application_id, saml_reply_url, certificate, saml_metadata, saml_metadata_url,
	enable_saml_compress, saml_name_id_format, saml_name_id_content, saml_sign_request,
	saml_attributes, enable_saml_c14n10, enable_saml_post_binding, disable_saml_attributes,
	enable_saml_assertion_signature, is_email_saml_nameid, saml_hash_algorithm, saml_c14n_prefix
) VALUES (
	$1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13, $14, $15, $16, $17
) ON CONFLICT (application_id) DO UPDATE SET
	saml_reply_url = EXCLUDED.saml_reply_url,
	certificate = EXCLUDED.certificate,
	saml_metadata = EXCLUDED.saml_metadata,
	saml_metadata_url = EXCLUDED.saml_metadata_url,
	enable_saml_compress = EXCLUDED.enable_saml_compress,
	saml_name_id_format = EXCLUDED.saml_name_id_format,
	saml_name_id_content = EXCLUDED.saml_name_id_content,
	saml_sign_request = EXCLUDED.saml_sign_request,
	saml_attributes = EXCLUDED.saml_attributes,
	enable_saml_c14n10 = EXCLUDED.enable_saml_c14n10,
	enable_saml_post_binding = EXCLUDED.enable_saml_post_binding,
	disable_saml_attributes = EXCLUDED.disable_saml_attributes,
	enable_saml_assertion_signature = EXCLUDED.enable_saml_assertion_signature,
	is_email_saml_nameid = EXCLUDED.is_email_saml_nameid,
	saml_hash_algorithm = EXCLUDED.saml_hash_algorithm,
	saml_c14n_prefix = EXCLUDED.saml_c14n_prefix`

	queryInsertFormConfig = `
INSERT INTO guliangiam.app_form_config (
	application_id, signup_items, custom_css, custom_header, form_css, form_css_mobile,
	form_side_html, form_background_url, form_background_url_mobile, form_offset,
	form_position, header_html, footer_html, signup_html, signin_html, page_html,
	custom_js, terms_of_use, privacy_policy
) VALUES (
	$1::uuid, $2::jsonb, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
) ON CONFLICT (application_id) DO UPDATE SET
	signup_items = EXCLUDED.signup_items,
	custom_css = EXCLUDED.custom_css,
	custom_header = EXCLUDED.custom_header,
	form_css = EXCLUDED.form_css,
	form_css_mobile = EXCLUDED.form_css_mobile,
	form_side_html = EXCLUDED.form_side_html,
	form_background_url = EXCLUDED.form_background_url,
	form_background_url_mobile = EXCLUDED.form_background_url_mobile,
	form_offset = EXCLUDED.form_offset,
	form_position = EXCLUDED.form_position,
	header_html = EXCLUDED.header_html,
	footer_html = EXCLUDED.footer_html,
	signup_html = EXCLUDED.signup_html,
	signin_html = EXCLUDED.signin_html,
	page_html = EXCLUDED.page_html,
	custom_js = EXCLUDED.custom_js,
	terms_of_use = EXCLUDED.terms_of_use,
	privacy_policy = EXCLUDED.privacy_policy`

	queryInsertNetworkConfig = `
INSERT INTO guliangiam.app_network_config (
	application_id, provider_items, forced_redirect_origin, other_domains,
	upstream_host, ssl_mode, ssl_cert, client_cert
) VALUES (
	$1::uuid, $2::jsonb, $3::varchar, $4::varchar[], $5::varchar, $6::varchar, $7::varchar, $8::varchar
) ON CONFLICT (application_id) DO UPDATE SET
	provider_items = EXCLUDED.provider_items,
	forced_redirect_origin = EXCLUDED.forced_redirect_origin,
	other_domains = EXCLUDED.other_domains,
	upstream_host = EXCLUDED.upstream_host,
	ssl_mode = EXCLUDED.ssl_mode,
	ssl_cert = EXCLUDED.ssl_cert,
	client_cert = EXCLUDED.client_cert`
)
