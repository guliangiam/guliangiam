package webauthn

const (
	queryInsertChallenge = `
INSERT INTO "guliangiam".webauthn_challenges
  (id, challenge_hash, ceremony_type, user_id, organization_id, relying_party_id, expires_at, created_at, updated_at)
VALUES ($1::uuid, $2, $3, $4::uuid, $5::uuid, $6, $7, $8, $9)`

	// 清理作业：删除早已过期的挑战行（finish 的消费谓词自带 expires_at > now()，
	// 因此过期行只是占空间，不影响正确性）。
	queryDeleteExpiredChallenges = `
DELETE FROM "guliangiam".webauthn_challenges WHERE expires_at < $1`

	// 一次性语义全部压在这条条件 UPDATE 上：只有「未消费且未过期」才会命中一行，
	// 并发 finish 里只有一个事务能拿到 RETURNING 的结果（RowsAffected=0 → NotFound）。
	queryConsumeChallenge = `
UPDATE "guliangiam".webauthn_challenges
SET consumed_at = $2, updated_at = $2
WHERE id = $1::uuid
  AND consumed_at IS NULL
  AND expires_at > $2
RETURNING id, challenge_hash, ceremony_type, user_id, COALESCE(organization_id::text, ''), relying_party_id, expires_at, consumed_at, created_at, updated_at`

	queryInsertCredential = `
INSERT INTO "guliangiam".webauthn_credentials
  (id, user_id, credential_id, public_key, attestation_type, transport, sign_count, name, aaguid, status, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	queryGetByCredentialID = `
SELECT id, user_id, credential_id, public_key, attestation_type, transport, sign_count, name, aaguid, status, created_at, updated_at
FROM "guliangiam".webauthn_credentials
WHERE credential_id = $1`

	queryListByUserID = `
SELECT id, user_id, credential_id, public_key, attestation_type, transport, sign_count, name, aaguid, status, created_at, updated_at
FROM "guliangiam".webauthn_credentials
WHERE user_id = $1::uuid
ORDER BY created_at DESC`

	queryUpdateSignCount = `
UPDATE "guliangiam".webauthn_credentials
SET sign_count = $1, updated_at = $2
WHERE id = $3::uuid
  AND sign_count < $1`

	queryUpdateCredential = `
UPDATE "guliangiam".webauthn_credentials
SET name = $1, updated_at = $2
WHERE id = $3::uuid`

	// 状态迁移：只在状态确实变化时写（幂等，重复吊销不报错也不刷新 updated_at）。
	queryUpdateCredentialStatus = `
UPDATE "guliangiam".webauthn_credentials
SET status = $1, updated_at = $2
WHERE id = $3::uuid
  AND status <> $1`

	queryDeleteCredential = `DELETE FROM "guliangiam".webauthn_credentials WHERE id = $1::uuid`
)
