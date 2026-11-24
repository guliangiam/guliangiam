package verifications

const (
	queryGetVerificationByID = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM "guliangiam".verifications
WHERE id = $1::uuid`

	queryGetVerificationByIDForOrganization = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM "guliangiam".verifications
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListVerifications = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM "guliangiam".verifications
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR type = $2)
  AND ($3 = '' OR status = $3)
  AND ($4 = '' OR recipient ILIKE '%' || $4 || '%')
ORDER BY created_at DESC
LIMIT $5 OFFSET $6`

	queryCountVerifications = `
SELECT COUNT(*)
FROM "guliangiam".verifications
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR type = $2)
  AND ($3 = '' OR status = $3)
  AND ($4 = '' OR recipient ILIKE '%' || $4 || '%')`

	queryCreateVerification = `
INSERT INTO "guliangiam".verifications (id, organization_id, type, recipient, code, status, expires_at, created_at)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)`

	queryUpdateVerificationStatus = `
UPDATE "guliangiam".verifications
SET status = $2
WHERE id = $1::uuid
  AND status = 'pending'`

	queryGetActiveVerificationByRecipientAndCode = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM "guliangiam".verifications
WHERE recipient = $1
  AND code = $2
  AND type = $3
  AND status = 'pending'
  AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1`

	queryGetActiveVerificationByRecipient = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM "guliangiam".verifications
WHERE recipient = $1
  AND type = $2
  AND status = 'pending'
  AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1`

	queryUpdateVerificationCode = `
UPDATE "guliangiam".verifications
SET code = $2
WHERE id = $1::uuid`

	queryRecordVerificationBinding = `
UPDATE "guliangiam".verifications
SET application_id = NULLIF($2, '')::uuid,
    provider_id = NULLIF($3, '')::uuid
WHERE id = $1::uuid`

	queryGetVerificationApplicationBinding = `
SELECT COALESCE(application_id::text, ''), COALESCE(provider_id::text, '')
FROM "guliangiam".verifications
WHERE id = $1::uuid`
)
