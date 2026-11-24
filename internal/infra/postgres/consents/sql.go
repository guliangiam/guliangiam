package consents

const queryGetConsentByUserAndApplication = `
SELECT id,
       user_id,
       username,
       organization_id,
       organization_slug,
       application_id,
       application_slug,
       granted_scopes,
       created_at,
       updated_at,
       revoked_at,
       revision
FROM guliangiam.consents
WHERE user_id = $1::uuid
  AND application_id = $2::uuid`

const queryCountActiveConsentsByApplication = `
SELECT COUNT(*)
FROM guliangiam.consents
WHERE application_id = $1::uuid
  AND revoked_at IS NULL`

const queryUpsertConsent = `
INSERT INTO guliangiam.consents (id, user_id, username, organization_id, organization_slug, application_id, application_slug, granted_scopes, created_at, updated_at, revoked_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5, $6::uuid, $7, $8::jsonb, $9, $10, $11, $12)
ON CONFLICT (user_id, application_id) DO UPDATE
SET username = EXCLUDED.username,
    organization_id = EXCLUDED.organization_id,
    organization_slug = EXCLUDED.organization_slug,
    application_slug = EXCLUDED.application_slug,
    granted_scopes = EXCLUDED.granted_scopes,
    updated_at = EXCLUDED.updated_at,
    revoked_at = EXCLUDED.revoked_at,
    revision = EXCLUDED.revision`

const queryRevokeConsent = `
UPDATE guliangiam.consents
SET revoked_at = NOW(),
    updated_at = NOW(),
    revision = revision + 1
WHERE user_id = $1::uuid
  AND application_id = $2::uuid
  AND revoked_at IS NULL`
