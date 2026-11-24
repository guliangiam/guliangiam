package faceidsessions

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid                       → ?
//   "guliangiam".table             → `table`
//   now()                          → NOW()
//   $1::uuid                       → ?

const btFaceIDSessions = "`face_id_sessions`"

// queryGetBySessionID: session_id(1)
const queryGetBySessionID = `
SELECT id, session_id, challenge, user_id, expires_at, verified, liveness_ok, attempt_count, created_at, updated_at
FROM ` + btFaceIDSessions + `
WHERE session_id = ?`

// queryCreate: 10 个占位符（attempt_count 固定 0）
const queryCreate = `
INSERT INTO ` + btFaceIDSessions + ` (id, session_id, challenge, user_id, expires_at, verified, liveness_ok, attempt_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`

// queryUpdate: attempt_count(1), verified(1), liveness_ok(1), updated_at(1), session_id(1) → 5
// WHERE verified = false → CAS 语义
const queryUpdate = `
UPDATE ` + btFaceIDSessions + `
SET verified = ?, liveness_ok = ?, attempt_count = ?, updated_at = ?
WHERE session_id = ?
  AND verified = false`

// queryConsumeVerified: session_id(1)
// WHERE verified = true → 只消费已验证的
const queryConsumeVerified = `
DELETE FROM ` + btFaceIDSessions + `
WHERE session_id = ?
  AND verified = true`

// queryDeleteBySessionID: session_id(1)
const queryDeleteBySessionID = `
DELETE FROM ` + btFaceIDSessions + `
WHERE session_id = ?`

// queryDeleteExpired: 无参数
const queryDeleteExpired = `
DELETE FROM ` + btFaceIDSessions + `
WHERE expires_at < NOW()`
