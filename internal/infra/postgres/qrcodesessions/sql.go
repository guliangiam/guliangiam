package qrcodesessions

const (
	queryGetByCodeID = `
SELECT id, code_id, code, organization_id, application_id, expires_at, status,
       user_id, username, ip_address, user_agent, created_at, updated_at, revision
FROM "guliangiam".qr_code_sessions
WHERE code_id = $1`

	queryGetByCode = `
SELECT id, code_id, code, organization_id, application_id, expires_at, status,
       user_id, username, ip_address, user_agent, created_at, updated_at, revision
FROM "guliangiam".qr_code_sessions
WHERE code = $1`

	queryCreate = `
INSERT INTO "guliangiam".qr_code_sessions
    (id, code_id, code, organization_id, application_id, expires_at, status,
     user_id, username, ip_address, user_agent, created_at, updated_at, revision)
VALUES ($1::uuid, $2, $3, $4::uuid, $5::uuid, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	queryUpdateStatus = `
UPDATE "guliangiam".qr_code_sessions
SET status = $3, user_id = $4, username = $5, ip_address = $6, user_agent = $7,
    updated_at = $8, revision = revision + 1
WHERE code_id = $1 AND revision = $2`

	queryDeleteByCodeID = `
DELETE FROM "guliangiam".qr_code_sessions
WHERE code_id = $1`

	queryDeleteExpired = `
DELETE FROM "guliangiam".qr_code_sessions
WHERE expires_at < NOW()`
)
