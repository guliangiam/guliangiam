package invitations

const (
	queryGetInvitationByID = `
SELECT id, organization_id, email, code, role_ids, group_ids, status, expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM guliangiam.invitations
WHERE id = $1::uuid`

	queryGetInvitationByIDForOrganization = `
SELECT id, organization_id, email, code, role_ids, group_ids, status, expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM guliangiam.invitations
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryGetInvitationByCode = `
SELECT id, organization_id, email, code, role_ids, group_ids, status, expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM guliangiam.invitations
WHERE code = $1`

	queryGetInvitationByEmail = `
SELECT id, organization_id, email, code, role_ids, group_ids, status, expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM guliangiam.invitations
WHERE organization_id = guliangiam.resolve_org_id($1) AND email = $2 AND status = 'pending'
ORDER BY created_at DESC
LIMIT 1`

	queryListInvitations = `
SELECT id, organization_id, email, code, role_ids, group_ids, status, expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM guliangiam.invitations
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR email ILIKE '%' || $2 || '%')
  AND ($3 = '' OR status = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountInvitations = `
SELECT COUNT(*)
FROM guliangiam.invitations
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR email ILIKE '%' || $2 || '%')
  AND ($3 = '' OR status = $3)`

	queryInsertInvitation = `
INSERT INTO guliangiam.invitations (id, organization_id, email, code, role_ids, group_ids, status, expires_at, accepted_at, invited_by, created_at, updated_at, revision)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	queryUpdateInvitation = `
UPDATE guliangiam.invitations
SET email = $1,
    code = $2,
    role_ids = $3,
    group_ids = $4,
    status = $5,
    expires_at = $6,
    accepted_at = $7,
    invited_by = $8,
    updated_at = $9,
    revision = revision + 1
WHERE id = $11::uuid AND revision = $10 AND organization_id = guliangiam.resolve_org_id($12) RETURNING revision`

	queryDeleteInvitation = `
DELETE FROM guliangiam.invitations
WHERE id = $1::uuid`

	// queryListExpiringInvitations 是系统作用域的跨组织扫描：只取仍待接受且
	// 到期时间落在提醒窗口内（now, deadline] 的行，按最近到期排序。
	queryListExpiringInvitations = `
SELECT id, organization_id, email, code, role_ids, group_ids, status, expires_at, accepted_at, invited_by, created_at, updated_at, revision
FROM guliangiam.invitations
WHERE status = 'pending'
  AND expires_at > $1
  AND expires_at <= $2
ORDER BY expires_at ASC, id ASC
LIMIT $3`
)
