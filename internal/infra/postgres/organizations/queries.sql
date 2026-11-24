-- name: GetOrganizationByID :one
SELECT
    id,
    slug,
    display_name,
    website_url,
    logo_url,
    logo_dark_url,
    favicon_url,
    status,
    created_at,
    updated_at,
    revision
FROM guliangiam.organizations
WHERE id = $1;

-- name: GetOrganizationBySlug :one
SELECT
    id,
    slug,
    display_name,
    website_url,
    logo_url,
    logo_dark_url,
    favicon_url,
    status,
    created_at,
    updated_at,
    revision
FROM guliangiam.organizations
WHERE slug = $1;

-- name: ListOrganizations :many
SELECT
    id,
    slug,
    display_name,
    website_url,
    logo_url,
    logo_dark_url,
    favicon_url,
    status,
    created_at,
    updated_at,
    revision
FROM guliangiam.organizations
WHERE ($1 = '' OR slug ILIKE '%' || $1 || '%' OR display_name ILIKE '%' || $1 || '%')
  AND ($2 = '' OR status = $2)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountOrganizations :one
SELECT COUNT(*)
FROM guliangiam.organizations
WHERE ($1 = '' OR slug ILIKE '%' || $1 || '%' OR display_name ILIKE '%' || $1 || '%')
  AND ($2 = '' OR status = $2);

-- name: GetOrganizationSettingsByOrganizationID :one
SELECT
    organization_id,
    languages,
    default_application_id,
    account_items,
    nav_preferences,
    feature_flags,
    country_codes,
    user_types,
    tags
FROM guliangiam.organization_settings
WHERE organization_id = $1;

-- name: GetOrganizationThemeByOrganizationID :one
SELECT
    organization_id,
    theme_name,
    tokens,
    custom_css,
    assets
FROM guliangiam.organization_theme_settings
WHERE organization_id = $1;

-- name: GetOrganizationSecurityByOrganizationID :one
SELECT
    organization_id,
    password_policy,
    password_derive_algorithm,
    mfa_policy,
    ip_policy,
    session_policy,
    risk_policy,
    lockout_policy,
    concurrent_session_policy
FROM guliangiam.organization_security_policies
WHERE organization_id = $1;

-- name: InsertOrganization :exec
INSERT INTO guliangiam.organizations (
    id,
    slug,
    display_name,
    website_url,
    logo_url,
    logo_dark_url,
    favicon_url,
    status,
    created_at,
    updated_at,
    revision
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: UpdateOrganization :exec
UPDATE guliangiam.organizations
SET display_name = $1,
    website_url = $2,
    logo_url = $3,
    logo_dark_url = $4,
    favicon_url = $5,
    status = $6,
    updated_at = $7,
    revision = $8
WHERE id = $9;

-- name: UpsertOrganizationSettings :exec
INSERT INTO guliangiam.organization_settings (
    organization_id,
    languages,
    default_application_id,
    account_items,
    nav_preferences,
    feature_flags,
    country_codes,
    user_types,
    tags
) VALUES ($1, $2::jsonb, NULLIF($3, ''), $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb)
ON CONFLICT (organization_id)
DO UPDATE SET
    languages = EXCLUDED.languages,
    default_application_id = EXCLUDED.default_application_id,
    account_items = EXCLUDED.account_items,
    nav_preferences = EXCLUDED.nav_preferences,
    feature_flags = EXCLUDED.feature_flags,
    country_codes = EXCLUDED.country_codes,
    user_types = EXCLUDED.user_types,
    tags = EXCLUDED.tags;

-- name: UpsertOrganizationTheme :exec
INSERT INTO guliangiam.organization_theme_settings (
    organization_id,
    theme_name,
    tokens,
    custom_css,
    assets
) VALUES ($1, $2, $3::jsonb, $4, $5::jsonb)
ON CONFLICT (organization_id)
DO UPDATE SET
    theme_name = EXCLUDED.theme_name,
    tokens = EXCLUDED.tokens,
    custom_css = EXCLUDED.custom_css,
    assets = EXCLUDED.assets;

-- name: UpsertOrganizationSecurityPolicy :exec
INSERT INTO guliangiam.organization_security_policies (
    organization_id,
    password_policy,
    password_derive_algorithm,
    mfa_policy,
    ip_policy,
    session_policy,
    risk_policy,
    lockout_policy,
    concurrent_session_policy
) VALUES ($1, $2::jsonb, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb)
ON CONFLICT (organization_id)
DO UPDATE SET
    password_policy = EXCLUDED.password_policy,
    password_derive_algorithm = EXCLUDED.password_derive_algorithm,
    mfa_policy = EXCLUDED.mfa_policy,
    ip_policy = EXCLUDED.ip_policy,
    session_policy = EXCLUDED.session_policy,
    risk_policy = EXCLUDED.risk_policy,
    lockout_policy = EXCLUDED.lockout_policy,
    concurrent_session_policy = EXCLUDED.concurrent_session_policy;