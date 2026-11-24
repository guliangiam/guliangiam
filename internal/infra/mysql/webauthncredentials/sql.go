package webauthncredentials

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::boolean              → ?
//   "guliangiam".table                 → `table`
//   pgx.Query/QueryRow/Exec            → QueryContext/QueryRowContext/ExecContext
//   text[] → JSON TEXT → []byte，mapper 层 UnmarshalJSON
//   uuid AAGUID → VARCHAR，直接 string

const btWebAuthnChallenges = `webauthn_challenges`

// queryInsertChallenge 占位符：9
const queryInsertChallenge = `INSERT INTO ` + btWebAuthnChallenges + ` (id, challenge_hash, ceremony_type, user_id, organization_id, relying_party_id, expires_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryConsumeChallenge 占位符：consumed_at/updated_at(2), id(3), now(4)
// 一次性语义全部压在这条条件 UPDATE 上：只有「未消费且未过期」才会命中一行。
const queryConsumeChallenge = `UPDATE ` + btWebAuthnChallenges + ` SET consumed_at = ?, updated_at = ? WHERE id = ? AND consumed_at IS NULL AND expires_at > ?`

// queryGetChallengeByID 占位符：id(1)
const queryGetChallengeByID = `SELECT id, challenge_hash, ceremony_type, user_id, organization_id, relying_party_id, expires_at, consumed_at, created_at, updated_at FROM ` + btWebAuthnChallenges + ` WHERE id = ?`

// queryDeleteExpiredChallenges 占位符：cutoff(1)
const queryDeleteExpiredChallenges = `DELETE FROM ` + btWebAuthnChallenges + ` WHERE expires_at < ?`

const btWebAuthn = "`webauthn_credentials`"

// queryInsertCredential 占位符：12
const queryInsertCredential = `INSERT INTO ` + btWebAuthn + ` (id, user_id, credential_id, public_key, attestation_type, transport, sign_count, name, aaguid, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryGetByCredentialID 占位符：credential_id(1)
const queryGetByCredentialID = `SELECT id, user_id, credential_id, public_key, attestation_type, transport, sign_count, name, aaguid, status, created_at, updated_at FROM ` + btWebAuthn + ` WHERE credential_id = ?`

// queryListByUserID 占位符：user_id(1)
const queryListByUserID = `SELECT id, user_id, credential_id, public_key, attestation_type, transport, sign_count, name, aaguid, status, created_at, updated_at FROM ` + btWebAuthn + ` WHERE user_id = ? ORDER BY created_at DESC`

// queryUpdateSignCount 占位符：sign_count(1), updated_at(2), id(3)
const queryUpdateSignCount = `UPDATE ` + btWebAuthn + ` SET sign_count = ?, updated_at = ? WHERE id = ? AND sign_count < ?`

// queryUpdateCredential 占位符：name(1), updated_at(2), id(3)
const queryUpdateCredential = `UPDATE ` + btWebAuthn + ` SET name = ?, updated_at = ? WHERE id = ?`

// queryUpdateCredentialStatus 占位符：status(1), updated_at(2), id(3), status(4)
// 与 PG 版同口径：状态未变化时不写行（幂等）。
const queryUpdateCredentialStatus = `UPDATE ` + btWebAuthn + ` SET status = ?, updated_at = ? WHERE id = ? AND status <> ?`

// queryDeleteCredential 占位符：id(1)
const queryDeleteCredential = `DELETE FROM ` + btWebAuthn + ` WHERE id = ?`
