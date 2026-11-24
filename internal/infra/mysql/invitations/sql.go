package invitations

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::timestamptz      → ?
//   guliangiam.table                → `table`
//   guliangiam.resolve_org_id($1)   → 直接 organization_id = ?
//   ILIKE '%' || $2 || '%'          → LIKE CONCAT('%', ?, '%')
//   RETURNING revision              → MySQL 用 UPDATE + SELECT 两步
//   text[] (role_ids, group_ids)    → TEXT (JSON 数组字符串)

const btInvitations = "`invitations`"

// queryGetInvitationByID: id(1) - 13 列
const queryGetInvitationByID = `
SELECT id, organization_id, email, code, role_ids, group_ids, status,
       expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM ` + btInvitations + `
WHERE id = ?`

// queryGetInvitationByIDForOrganization: id(1), organization_id(2) - 13 列
const queryGetInvitationByIDForOrganization = `
SELECT id, organization_id, email, code, role_ids, group_ids, status,
       expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM ` + btInvitations + `
WHERE id = ? AND organization_id = ?`

// queryGetInvitationByCode: code(1)
const queryGetInvitationByCode = `
SELECT id, organization_id, email, code, role_ids, group_ids, status,
       expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM ` + btInvitations + `
WHERE code = ?`

// queryGetInvitationByEmail: org_id(1), email(2) - resolve_org_id → 直接
const queryGetInvitationByEmail = `
SELECT id, organization_id, email, code, role_ids, group_ids, status,
       expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM ` + btInvitations + `
WHERE organization_id = ? AND email = ? AND status = 'pending'
ORDER BY created_at DESC
LIMIT 1`

// invitationSelectBase: 所有 SELECT 共用的列
const invitationSelectBase = `
SELECT id, organization_id, email, code, role_ids, group_ids, status,
       expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM ` + btInvitations

// queryListInvitations: org_id×2(2), keyword(1), status(1), limit(1), offset(1) = 6
//
//	CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END
//	AND (? = '' OR email LIKE CONCAT('%', ?, '%'))
//	AND (? = '' OR status = ?)
const queryListInvitations = invitationSelectBase + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR email LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR status = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountInvitations: 去掉 LIMIT/OFFSET，占位符数 = 6
const queryCountInvitations = `
SELECT COUNT(*)
FROM ` + btInvitations + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR email LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR status = ?)`

// queryInsertInvitation: 13 个占位符
const queryInsertInvitation = `
INSERT INTO ` + btInvitations + ` (id, organization_id, email, code, role_ids, group_ids, status, expires_at, accepted_at, invited_by, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateInvitation: PG RETURNING revision 改写
// 参数：email, code, role_ids, group_ids, status, expires_at, accepted_at,
//
//	invited_by, updated_at, id, revision, organization_id
const queryUpdateInvitation = `
UPDATE ` + btInvitations + `
SET email = ?, code = ?, role_ids = ?, group_ids = ?,
    status = ?, expires_at = ?, accepted_at = ?, invited_by = ?,
    updated_at = ?, revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

// querySelectRevisionAfterUpdate: UPDATE 之后拿新 revision
const querySelectRevisionAfterUpdate = `
SELECT revision FROM ` + btInvitations + ` WHERE id = ?`

// queryDeleteInvitation: id(1)
const queryDeleteInvitation = `
DELETE FROM ` + btInvitations + ` WHERE id = ?`

// queryListExpiringInvitations 是系统作用域的跨组织扫描：now(1), deadline(2), limit(3)。
const queryListExpiringInvitations = invitationSelectBase + `
WHERE status = 'pending'
  AND expires_at > ?
  AND expires_at <= ?
ORDER BY expires_at ASC, id ASC
LIMIT ?`
