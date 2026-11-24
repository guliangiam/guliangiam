package idv

// SQL 方言转换规则（PG → MySQL）：
//
//	$N::uuid / $N::text  → ?
//	"guliangiam".table   → `table`（去掉 schema 前缀）
//	$N::text IS NULL     → (? IS NULL)
const btIdentityVerifications = "`identity_verifications`"

const (
	queryGetByID = `
SELECT id, organization_id, user_id, provider, status,
       redirect_url, session_token, provider_ref,
       first_name, last_name, date_of_birth, nationality,
       id_type, id_number, confidence, reason,
       raw_result, initiated_at, completed_at, created_at, updated_at
FROM ` + btIdentityVerifications + `
WHERE id = ? AND organization_id = ?`

	queryGetByProviderRef = `
SELECT id, organization_id, user_id, provider, status,
       redirect_url, session_token, provider_ref,
       first_name, last_name, date_of_birth, nationality,
       id_type, id_number, confidence, reason,
       raw_result, initiated_at, completed_at, created_at, updated_at
FROM ` + btIdentityVerifications + `
WHERE provider = ? AND provider_ref = ? AND organization_id IS NOT NULL`

	// queryList — 分页列出 verifications。
	// PG ($2 = '' OR user_id = $2::uuid) → (? = '' OR user_id = ?)  userFilter × 2
	// PG ($3 = '' OR provider = $3)      → (? = '' OR provider = ?)  providerFilter × 2
	// PG ($4::text IS NULL OR status = $4) → (? IS NULL OR status = ?)  statusFilter × 2
	// 参数：orgID, userFilter×2, providerFilter×2, statusFilter×2, limit, offset
	queryList = `
SELECT id, organization_id, user_id, provider, status,
       redirect_url, session_token, provider_ref,
       first_name, last_name, date_of_birth, nationality,
       id_type, id_number, confidence, reason,
       raw_result, initiated_at, completed_at, created_at, updated_at
FROM ` + btIdentityVerifications + `
WHERE organization_id = ?
  AND (? = '' OR user_id = ?)
  AND (? = '' OR provider = ?)
  AND (? IS NULL OR status = ?)
ORDER BY initiated_at DESC
LIMIT ? OFFSET ?`

	queryCount = `
SELECT COUNT(*) FROM ` + btIdentityVerifications + `
WHERE organization_id = ?
  AND (? = '' OR user_id = ?)
  AND (? = '' OR provider = ?)
  AND (? IS NULL OR status = ?)`

	// queryInsert — 21 个 ?
	queryInsert = `
INSERT INTO ` + btIdentityVerifications + `
  (id, organization_id, user_id, provider, status,
   redirect_url, session_token, provider_ref,
   first_name, last_name, date_of_birth, nationality,
   id_type, id_number, confidence, reason,
   raw_result, initiated_at, completed_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?,
        ?, ?, ?,
        ?, ?, ?, ?,
        ?, ?, ?, ?,
        ?, ?, ?, ?, ?)`

	// queryUpdate — 15 个 ?（SET 13 + WHERE id + WHERE organization_id）
	// PG COALESCE($12, provider_ref) → MySQL COALESCE(?, provider_ref)
	queryUpdate = `
UPDATE ` + btIdentityVerifications + `
SET status = ?,
    first_name = ?,
    last_name = ?,
    date_of_birth = ?,
    nationality = ?,
    id_type = ?,
    id_number = ?,
    confidence = ?,
    reason = ?,
    raw_result = ?,
    completed_at = ?,
    provider_ref = COALESCE(?, provider_ref),
    updated_at = ?
WHERE id = ? AND organization_id = ?`

	queryDelete = `
DELETE FROM ` + btIdentityVerifications + `
WHERE id = ? AND organization_id = ?`
)
