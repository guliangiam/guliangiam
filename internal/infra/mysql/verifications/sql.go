package verifications

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid                       → ?
//   "guliangiam".table             → `table`
//   ILIKE '%' || $4 || '%'         → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)  → 直接 organization_id = ?
//   expires_at > NOW()             → MySQL 保留 NOW()

const btVerifications = "`verifications`"

// queryGetVerificationByID: id(1)
const queryGetVerificationByID = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM ` + btVerifications + `
WHERE id = ?`

// queryGetVerificationByIDForOrganization: id(1)、organization_id(2)
const queryGetVerificationByIDForOrganization = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM ` + btVerifications + `
WHERE id = ? AND organization_id = ?`

// queryListVerifications
//
//	org: CASE WHEN ?='' THEN TRUE ELSE organization_id=? END → 2
//	type: ?='' OR type=?                                     → 2
//	status: ?='' OR status=?                                 → 2
//	recipient: ?='' OR recipient LIKE CONCAT('%', ?, '%')    → 3
//	limit, offset                                            → 2
//	总计：11
const queryListVerifications = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM ` + btVerifications + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR type = ?)
  AND (? = '' OR status = ?)
  AND (? = '' OR recipient LIKE CONCAT('%', ?, '%'))
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountVerifications: org×2 + type×2 + status×2 + recipient×3 = 9
const queryCountVerifications = `
SELECT COUNT(*)
FROM ` + btVerifications + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR type = ?)
  AND (? = '' OR status = ?)
  AND (? = '' OR recipient LIKE CONCAT('%', ?, '%'))`

// queryCreateVerification: 8 个占位符
const queryCreateVerification = `
INSERT INTO ` + btVerifications + ` (id, organization_id, type, recipient, code, status, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateVerificationStatus: status(1), id(1) → 2
// WHERE 要求旧值 status='pending' → 让一次性码真正一次性
const queryUpdateVerificationStatus = `
UPDATE ` + btVerifications + `
SET status = ?
WHERE id = ?
  AND status = 'pending'`

// queryGetActiveVerificationByRecipientAndCode: recipient(1), code(1), type(1) → 3
const queryGetActiveVerificationByRecipientAndCode = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM ` + btVerifications + `
WHERE recipient = ?
  AND code = ?
  AND type = ?
  AND status = 'pending'
  AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1`

// queryGetActiveVerificationByRecipient: recipient(1), type(1) → 2
const queryGetActiveVerificationByRecipient = `
SELECT id, organization_id, type, recipient, code, status, expires_at, created_at
FROM ` + btVerifications + `
WHERE recipient = ?
  AND type = ?
  AND status = 'pending'
  AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1`

// queryUpdateVerificationCode: code(1), id(1) → 2
const queryUpdateVerificationCode = `
UPDATE ` + btVerifications + `
SET code = ?
WHERE id = ?`

// queryRecordVerificationBinding: application_id(1), provider_id(1), id(1) → 3。
// 空串写入 NULL，与 PG 的 NULLIF($n, ”)::uuid 等价。
const queryRecordVerificationBinding = `
UPDATE ` + btVerifications + `
SET application_id = NULLIF(?, ''),
    provider_id = NULLIF(?, '')
WHERE id = ?`

// queryGetVerificationApplicationBinding: id(1)
const queryGetVerificationApplicationBinding = `
SELECT COALESCE(application_id, ''), COALESCE(provider_id, '')
FROM ` + btVerifications + `
WHERE id = ?`
