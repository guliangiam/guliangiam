package mfachallenges

const (
	queryGetChallengeByID = `
SELECT id,
       organization_id,
       user_id,
       application_id,
    status,
    method_type,
    metadata,
       expires_at,
    verified_at,
       created_at,
       updated_at,
       revision
FROM guliangiam.login_challenges
WHERE id = $1::uuid`

	queryInsertChallenge = `
INSERT INTO guliangiam.login_challenges (id, organization_id, user_id, application_id, status, method_type, metadata, expires_at, verified_at, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7::jsonb, $8, $9, $10, $11, $12)`

	queryUpdateChallenge = `
UPDATE guliangiam.login_challenges
SET status = $1,
    method_type = $2,
    metadata = $3::jsonb,
    expires_at = $4,
    verified_at = $5,
    updated_at = $6,
    revision = revision + 1
WHERE id = $8::uuid AND revision = $7 RETURNING revision`

	queryDeleteChallenge = `
DELETE FROM guliangiam.login_challenges
WHERE id = $1::uuid`
)
