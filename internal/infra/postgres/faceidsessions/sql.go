package faceidsessions

const (
	queryGetBySessionID = `
SELECT id, session_id, challenge, user_id, expires_at, verified, liveness_ok, attempt_count, created_at, updated_at
FROM "guliangiam".face_id_sessions
WHERE session_id = $1`

	queryCreate = `
INSERT INTO "guliangiam".face_id_sessions
    (id, session_id, challenge, user_id, expires_at, verified, liveness_ok, attempt_count, created_at, updated_at)
VALUES ($1::uuid, $2, $3, $4::uuid, $5, $6, $7, 0, $8, $9)`

	queryUpdate = `
UPDATE "guliangiam".face_id_sessions
SET verified = $2, liveness_ok = $3, attempt_count = $4, updated_at = $5
WHERE session_id = $1
  AND verified = false`

	queryConsumeVerified = `
DELETE FROM "guliangiam".face_id_sessions
WHERE session_id = $1
  AND verified = true`

	queryDeleteBySessionID = `
DELETE FROM "guliangiam".face_id_sessions
WHERE session_id = $1`

	queryDeleteExpired = `
DELETE FROM "guliangiam".face_id_sessions
WHERE expires_at < NOW()`
)
