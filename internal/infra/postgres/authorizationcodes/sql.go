package authorizationcodes

// The authorization code is a bearer credential, so it is matched by its
// SHA-256 hash (uq_authorization_codes_hash) rather than by plaintext: the
// repository hashes the presented code before calling any of these queries. The
// SELECT therefore never returns the plaintext code — the redemption path only
// needs the PKCE challenge and the binding columns.
//
// u.username / u.organization_id and the two slug joins stay because the token
// endpoint needs the organization and application slugs to mint the access
// token without a second round trip.
const (
	queryGetAuthorizationCodeByCode = `
SELECT c.id,
       c.user_id,
       u.username,
       u.organization_id,
       o.slug,
       c.application_id,
       a.slug,
       c.redirect_uri,
       c.scopes,
       c.status,
       c.nonce,
       c.code_challenge,
       c.code_challenge_method,
       c.resource,
       c.sid,
       c.auth_time,
       c.expires_at,
       c.used_at,
       c.created_at
FROM guliangiam.authorization_codes c
JOIN guliangiam.users u ON u.id = c.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = c.application_id
WHERE c.code_hash = $1`

	queryInsertAuthorizationCode = `
INSERT INTO guliangiam.authorization_codes (id, code_hash, code_prefix, user_id, application_id, redirect_uri, scopes, status, sid, nonce, resource, code_challenge, code_challenge_method, auth_time, expires_at, created_at)
VALUES ($1::uuid, $2, $3, $4::uuid, $5::uuid, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`
	// 一次性消费），所以它不得退化为朴素的 `WHERE id = $1`。
	queryMarkAuthorizationCodeUsed = `
UPDATE guliangiam.authorization_codes
SET used_at = COALESCE(used_at, $2)
WHERE id = $1::uuid
  AND used_at IS NULL`
)
