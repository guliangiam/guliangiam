package tokens

const (
	queryInsertToken = `
INSERT INTO guliangiam.access_tokens (id, user_id, application_id, token_hash, token_prefix, device_secret_hash, device_secret_expires_in, scopes, audience, metadata, expires_at, revoked_at, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8::text[], $9::text[], $10::jsonb, $11, $12, $13, $14, $15)`

	queryUpdateTokenDeviceSecret = `
UPDATE guliangiam.access_tokens
SET device_secret_hash = $2,
    device_secret_expires_in = $3,
    updated_at = now(),
    revision = revision + 1
WHERE id = $1::uuid`

	queryInsertRefreshToken = `INSERT INTO guliangiam.refresh_tokens (id, access_token_id, user_id, application_id, token_hash, token_prefix, family_id, scopes, audience, expires_at, revoked_at, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7::uuid, $8::text[], $9::text[], $10, $11, $12, $13, $14)`

	queryGetTokenByID = `
SELECT t.id,
       t.user_id,
       u.username,
       u.organization_id,
       o.slug,
       t.application_id,
       a.slug,
       t.token_prefix,
       t.token_hash,
       COALESCE(t.device_secret_hash, ''),
       COALESCE(t.device_secret_expires_in, 0),
       t.scopes,
       t.audience,
       t.metadata,
       t.expires_at,
       t.revoked_at,
       t.created_at,
       t.updated_at,
       t.revision
FROM guliangiam.access_tokens t
JOIN guliangiam.users u ON u.id = t.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = t.application_id
WHERE t.id = $1`

	// queryGetTokenByIDForOrganization — 占位符：id(1)、organization_id(2)。
	// access_tokens 无 organization_id 列，租户归属经 users 表 join 绑定；
	// 跨租户与不存在同样返回 ErrNoRows。
	queryGetTokenByIDForOrganization = `
SELECT t.id,
       t.user_id,
       u.username,
       u.organization_id,
       o.slug,
       t.application_id,
       a.slug,
       t.token_prefix,
       t.token_hash,
       COALESCE(t.device_secret_hash, ''),
       COALESCE(t.device_secret_expires_in, 0),
       t.scopes,
       t.audience,
       t.metadata,
       t.expires_at,
       t.revoked_at,
       t.created_at,
       t.updated_at,
       t.revision
FROM guliangiam.access_tokens t
JOIN guliangiam.users u ON u.id = t.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = t.application_id
WHERE t.id = $1
  AND u.organization_id = guliangiam.resolve_org_id($2)`

	queryGetTokenByHash = `
SELECT t.id,
       t.user_id,
       u.username,
       u.organization_id,
       o.slug,
       t.application_id,
       a.slug,
       t.token_prefix,
       t.token_hash,
       COALESCE(t.device_secret_hash, ''),
       COALESCE(t.device_secret_expires_in, 0),
       t.scopes,
       t.audience,
       t.metadata,
       t.expires_at,
       t.revoked_at,
       t.created_at,
       t.updated_at,
       t.revision
FROM guliangiam.access_tokens t
JOIN guliangiam.users u ON u.id = t.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = t.application_id
WHERE t.token_hash = $1`

	queryGetTokenByDeviceSecretHash = `
SELECT t.id,
       t.user_id,
       u.username,
       u.organization_id,
       o.slug,
       t.application_id,
       a.slug,
       t.token_prefix,
       t.token_hash,
       COALESCE(t.device_secret_hash, ''),
       COALESCE(t.device_secret_expires_in, 0),
       t.scopes,
       t.audience,
       t.metadata,
       t.expires_at,
       t.revoked_at,
       t.created_at,
       t.updated_at,
       t.revision
FROM guliangiam.access_tokens t
JOIN guliangiam.users u ON u.id = t.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = t.application_id
WHERE t.device_secret_hash = $1`

	queryGetRefreshTokenByHash = `
SELECT t.id,
       t.access_token_id,
       t.user_id,
       u.username,
       u.organization_id,
       o.slug,
       t.application_id,
       a.slug,
       t.token_hash,
       COALESCE(t.family_id::text, ''),
       t.scopes,
       t.audience,
       t.expires_at,
       t.revoked_at,
       t.created_at,
       t.updated_at,
       t.revision
FROM guliangiam.refresh_tokens t
JOIN guliangiam.users u ON u.id = t.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = t.application_id
WHERE t.token_hash = $1
FOR UPDATE OF t`

	queryListTokens = `
SELECT t.id,
       t.user_id,
       u.username,
       u.organization_id,
       o.slug,
       t.application_id,
       a.slug,
       t.token_prefix,
       t.token_hash,
       COALESCE(t.device_secret_hash, ''),
       COALESCE(t.device_secret_expires_in, 0),
       t.scopes,
       t.audience,
       t.metadata,
       t.expires_at,
       t.revoked_at,
       t.created_at,
       t.updated_at,
       t.revision
FROM guliangiam.access_tokens t
JOIN guliangiam.users u ON u.id = t.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = t.application_id
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE u.organization_id = guliangiam.resolve_org_id($1) END)
  AND (CASE WHEN $2 = '' THEN TRUE ELSE t.user_id = $2::uuid END)
  AND (CASE WHEN $3 = '' THEN TRUE ELSE t.application_id = $3::uuid END)
  AND ($4::boolean IS NULL OR (($4 = true AND t.revoked_at IS NULL AND t.expires_at > now()) OR ($4 = false AND (t.revoked_at IS NOT NULL OR t.expires_at <= now()))))
  AND ($5 = '' OR u.username ILIKE '%' || $5 || '%' OR o.slug ILIKE '%' || $5 || '%' OR a.slug ILIKE '%' || $5 || '%' OR t.token_prefix ILIKE '%' || $5 || '%' OR t.token_hash ILIKE '%' || $5 || '%')
ORDER BY t.created_at DESC
LIMIT $6 OFFSET $7`

	queryCountTokens = `
SELECT COUNT(*)
FROM guliangiam.access_tokens t
JOIN guliangiam.users u ON u.id = t.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = t.application_id
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE u.organization_id = guliangiam.resolve_org_id($1) END)
  AND (CASE WHEN $2 = '' THEN TRUE ELSE t.user_id = $2::uuid END)
  AND (CASE WHEN $3 = '' THEN TRUE ELSE t.application_id = $3::uuid END)
  AND ($4::boolean IS NULL OR (($4 = true AND t.revoked_at IS NULL AND t.expires_at > now()) OR ($4 = false AND (t.revoked_at IS NOT NULL OR t.expires_at <= now()))))
  AND ($5 = '' OR u.username ILIKE '%' || $5 || '%' OR o.slug ILIKE '%' || $5 || '%' OR a.slug ILIKE '%' || $5 || '%' OR t.token_prefix ILIKE '%' || $5 || '%' OR t.token_hash ILIKE '%' || $5 || '%')`

	queryRevokeToken = `
UPDATE guliangiam.access_tokens
SET revoked_at = COALESCE(revoked_at, $2),
    updated_at = $2,
    revision = revision + 1
WHERE id = $1::uuid
  AND revoked_at IS NULL`

	queryRevokeTokensByQuery = `
UPDATE guliangiam.access_tokens t
SET revoked_at = COALESCE(t.revoked_at, $4),
    updated_at = $4,
    revision = t.revision + 1
FROM guliangiam.users u
WHERE u.id = t.user_id
  AND (CASE WHEN $1 = '' THEN TRUE ELSE u.organization_id = guliangiam.resolve_org_id($1) END)
  AND (CASE WHEN $2 = '' THEN TRUE ELSE t.user_id = $2::uuid END)
  AND (CASE WHEN $3 = '' THEN TRUE ELSE t.application_id = $3::uuid END)
  AND (CASE WHEN $5 = '' THEN TRUE ELSE t.token_prefix = $5 END)
  AND t.revoked_at IS NULL
  AND t.expires_at > $4`

	queryRevokeRefreshToken = `
UPDATE guliangiam.refresh_tokens
SET revoked_at = COALESCE(revoked_at, $2),
    updated_at = $2,
    revision = revision + 1
WHERE id = $1::uuid
  AND revoked_at IS NULL`

	queryDeleteToken = `
DELETE FROM guliangiam.access_tokens
WHERE id = $1::uuid`

	// RevokeByFamilyID revokes all refresh tokens and their paired access tokens
	// in the same token family. This is the OAuth 2.1 response to token reuse
	// detection. It returns the number of *refresh* tokens in the family, which is
	// what the reuse detector reports as "tokens_revoked"; the paired access
	// tokens are revoked by the second CTE.
	queryRevokeByFamilyID = `
WITH revoked_refresh AS (
    UPDATE guliangiam.refresh_tokens
    SET revoked_at = COALESCE(revoked_at, $2),
        updated_at = $2,
        revision = revision + 1
    WHERE family_id = $1
      AND revoked_at IS NULL
    RETURNING id, access_token_id
),
revoked_access AS (
    UPDATE guliangiam.access_tokens
    SET revoked_at = COALESCE(revoked_at, $2),
        updated_at = $2,
        revision = revision + 1
    WHERE id IN (SELECT access_token_id FROM revoked_refresh WHERE access_token_id IS NOT NULL)
      AND revoked_at IS NULL
    RETURNING id
)
SELECT count(*) FROM revoked_refresh`

	// queryRevokeRefreshByUser bulk-revokes refresh tokens for a user. Unlike
	// RevokeByFamilyID which targets one family, this touches every refresh
	// token row for the given user. Used by credential-altering sensitive ops
	// (AC-AUTH-009) to force refresh + access token death together.
	queryRevokeRefreshByUser = `
WITH revoked_refresh AS (
    UPDATE guliangiam.refresh_tokens
    SET revoked_at = COALESCE(revoked_at, $2),
        updated_at = $2,
        revision = revision + 1
    WHERE user_id = $1::uuid
      AND revoked_at IS NULL
    RETURNING id, access_token_id
),
revoked_access AS (
    UPDATE guliangiam.access_tokens
    SET revoked_at = COALESCE(revoked_at, $2),
        updated_at = $2,
        revision = revision + 1
    WHERE id IN (SELECT access_token_id FROM revoked_refresh WHERE access_token_id IS NOT NULL)
      AND revoked_at IS NULL
    RETURNING id
)
SELECT count(*) FROM revoked_refresh`

	// queryRevokeByAuthorizationCodeID revokes the tokens a redeemed authorization
	// code produced. The access token carries the code id in its metadata, so the
	// code's own row stays the link between a leaked code and everything it bought;
	// the refresh tokens follow their paired access token.
	queryRevokeByAuthorizationCodeID = `
WITH revoked_access AS (
    UPDATE guliangiam.access_tokens
    SET revoked_at = COALESCE(revoked_at, $2),
        updated_at = $2,
        revision = revision + 1
    WHERE metadata ->> 'authorizationCodeId' = $1
      AND revoked_at IS NULL
    RETURNING id
),
revoked_refresh AS (
    UPDATE guliangiam.refresh_tokens
    SET revoked_at = COALESCE(revoked_at, $2),
        updated_at = $2,
        revision = revision + 1
    WHERE access_token_id IN (SELECT id FROM revoked_access)
      AND revoked_at IS NULL
    RETURNING id
)
SELECT (SELECT count(*) FROM revoked_access) + (SELECT count(*) FROM revoked_refresh)`
)
