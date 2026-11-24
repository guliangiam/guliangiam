package iga

import domain "github.com/guliangiam/guliangiam/internal/domain/iga"

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid              → ?
//   "guliangiam".table    → `table`
//   now()                 → NOW()
//   fmt.Sprintf(" LIMIT $%d OFFSET $%d", ...) → 参数化 LIMIT ? OFFSET ?

const btCampaigns = "`access_campaigns`"
const btSODRules = "`sod_rules`"
const btLifecycle = "`access_lifecycle`"

// paginationSuffix 是 MySQL 参数化分页片段（含前导空格，SQL 文本逐字不变）。
const paginationSuffix = " LIMIT ? OFFSET ?"

// ─── Campaigns ────────────────────────────────────────────────────

const qGetCampaign = `
SELECT id, organization_id, name, status, start_date, end_date, created_at, updated_at, revision
FROM ` + btCampaigns + `
WHERE id = ? AND organization_id = ?`

const qInsertCampaign = `
INSERT INTO ` + btCampaigns + ` (id, organization_id, name, status, start_date, end_date)
VALUES (?, ?, ?, 'draft', ?, ?)`

const qUpdateCampaignStatus = `
UPDATE ` + btCampaigns + `
SET status = ?, updated_at = NOW()
WHERE id = ? AND organization_id = ?`

const qDeleteCampaign = `DELETE FROM ` + btCampaigns + ` WHERE id = ? AND organization_id = ?`

// buildListCampaignsSQL 动态拼接 List/Count Campaigns SQL。
func buildListCampaignsSQL(q domain.ListCampaignsQuery, withPaging bool) (queryOut, whereClauseOut string, argsOut []any) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.Status != nil && *q.Status != "" {
		args = append(args, string(*q.Status))
		extra += " AND status = ?"
	}

	countSQL := `SELECT COUNT(*) FROM ` + btCampaigns + ` WHERE organization_id = ?` + extra
	selectSQL := `SELECT id, organization_id, name, status, start_date, end_date, created_at, updated_at, revision FROM ` + btCampaigns + ` WHERE organization_id = ?` + extra + " ORDER BY updated_at DESC"
	if withPaging {
		selectSQL += paginationSuffix
	}
	return selectSQL, countSQL, args
}

// ─── SOD Rules ────────────────────────────────────────────────────

const qListSODRules = `
SELECT id, organization_id, name, role_a_id, role_b_id, created_at, updated_at, revision
FROM ` + btSODRules + ` WHERE organization_id = ? ORDER BY name DESC
LIMIT ? OFFSET ?`

const qCountSODRules = `SELECT COUNT(*) FROM ` + btSODRules + ` WHERE organization_id = ?`

const qGetSODRule = `
SELECT id, organization_id, name, role_a_id, role_b_id, created_at, updated_at, revision
FROM ` + btSODRules + ` WHERE id = ? AND organization_id = ?`

const qInsertSODRule = `
INSERT INTO ` + btSODRules + ` (id, organization_id, name, role_a_id, role_b_id)
VALUES (?, ?, ?, ?, ?)`

const qUpdateSODRule = `UPDATE ` + btSODRules + ` SET name = ?, role_a_id = ?, role_b_id = ? WHERE id = ? AND organization_id = ?`

const qDeleteSODRule = `DELETE FROM ` + btSODRules + ` WHERE id = ? AND organization_id = ?`

// ─── Lifecycle ────────────────────────────────────────────────────

const qLifecycleColumns = `id, organization_id, user_id, role_id, COALESCE(ticket_id, ''), COALESCE(enforcer_id, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(policy_id, ''), granted_at, starts_at, expires_at, revoked_at, status, retry_count, COALESCE(last_error, ''), created_at, revision`

const qInsertLifecycle = `
INSERT INTO ` + btLifecycle + ` (id, organization_id, user_id, role_id, ticket_id, enforcer_id, resource, action, policy_id, granted_at, starts_at, expires_at, status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const qGetLifecycle = `SELECT ` + qLifecycleColumns + ` FROM ` + btLifecycle + ` WHERE id = ? AND organization_id = ?`
const qGetLifecycleForUpdate = `SELECT ` + qLifecycleColumns + ` FROM ` + btLifecycle + ` WHERE id = ? AND organization_id = ? FOR UPDATE`

const qUpdateLifecycleStatus = `UPDATE ` + btLifecycle + ` SET status = ? WHERE id = ? AND organization_id = ?`

const qRevokeLifecycle = `UPDATE ` + btLifecycle + ` SET status = 'revoked' WHERE id = ? AND organization_id = ?`

const qExpireLifecycle = `UPDATE ` + btLifecycle + ` SET status = 'expired' WHERE organization_id = ? AND status = 'active' AND expires_at < ?`

// buildListLifecycleSQL 动态拼接 List/Count Lifecycle SQL。
func buildListLifecycleSQL(q domain.ListLifecycleQuery, withPaging bool) (queryOut, whereClauseOut string, argsOut []any) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.UserID != nil && *q.UserID != "" {
		args = append(args, *q.UserID)
		extra += " AND user_id = ?"
	}
	if q.RoleID != nil && *q.RoleID != "" {
		args = append(args, *q.RoleID)
		extra += " AND role_id = ?"
	}
	if q.Status != nil && *q.Status != "" {
		args = append(args, string(*q.Status))
		extra += " AND status = ?"
	}

	countSQL := `SELECT COUNT(*) FROM ` + btLifecycle + ` WHERE organization_id = ?` + extra
	selectSQL := `SELECT id, organization_id, user_id, role_id, granted_at, expires_at, status, created_at, revision FROM ` + btLifecycle + ` WHERE organization_id = ?` + extra + " ORDER BY expires_at DESC"
	if withPaging {
		selectSQL += paginationSuffix
	}
	return selectSQL, countSQL, args
}

// Access requests and JIT artifacts.
const qGetAccessRequest = `SELECT id, organization_id, COALESCE(requester_id, ''), COALESCE(role_id, ''), COALESCE(application_id, ''), COALESCE(enforcer_id, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(description, ''), COALESCE(workflow_state, status), starts_at, expires_at, COALESCE(manager_approved_by, ''), COALESCE(sap_approved_by, ''), COALESCE(lifecycle_id, ''), created_at, updated_at, revision FROM ` + "`tickets`" + ` WHERE organization_id = ? AND id = ? AND type = 'iga_access_request' AND deleted_at IS NULL`
const qGetAccessRequestForUpdate = `SELECT id, organization_id, COALESCE(requester_id, ''), COALESCE(role_id, ''), COALESCE(application_id, ''), COALESCE(enforcer_id, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(description, ''), COALESCE(workflow_state, status), starts_at, expires_at, COALESCE(manager_approved_by, ''), COALESCE(sap_approved_by, ''), COALESCE(lifecycle_id, ''), created_at, updated_at, revision FROM ` + "`tickets`" + ` WHERE organization_id = ? AND id = ? AND type = 'iga_access_request' AND deleted_at IS NULL FOR UPDATE`
const qListAccessRequestsBase = "SELECT id, organization_id, COALESCE(requester_id, ''), COALESCE(role_id, ''), COALESCE(application_id, ''), COALESCE(enforcer_id, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(description, ''), COALESCE(workflow_state, status), starts_at, expires_at, COALESCE(manager_approved_by, ''), COALESCE(sap_approved_by, ''), COALESCE(lifecycle_id, ''), created_at, updated_at, revision FROM `tickets` WHERE organization_id = ? AND type = 'iga_access_request' AND deleted_at IS NULL"
const qInsertAccessRequest = `INSERT INTO ` + "`tickets`" + ` (id, organization_id, name, display_name, type, status, creator, description, requester_id, role_id, application_id, enforcer_id, resource, action, starts_at, expires_at, workflow_state, created_at, updated_at, revision) VALUES (?, ?, ?, ?, 'iga_access_request', 'open', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
const qUpdateAccessRequest = `UPDATE ` + "`tickets`" + ` SET workflow_state = ?, status = CASE WHEN ? IN ('rejected', 'withdrawn') THEN 'closed' WHEN ? = 'provisioned' THEN 'resolved' ELSE 'open' END, manager_approved_by = ?, sap_approved_by = ?, lifecycle_id = ?, updated_at = ?, revision = revision + 1 WHERE organization_id = ? AND id = ? AND type = 'iga_access_request' AND workflow_state = ? AND revision = ?`
const qInsertTicketMessage = `INSERT INTO ` + "`ticket_messages`" + ` (id, ticket_id, author_id, action, content, created_at) VALUES (?, ?, ?, ?, ?, ?)`
const qListTicketMessages = `SELECT m.id, m.ticket_id, COALESCE(m.author_id, ''), m.action, m.content, m.created_at FROM ` + "`ticket_messages`" + ` m JOIN ` + "`tickets`" + ` t ON t.id = m.ticket_id WHERE t.organization_id = ? AND m.ticket_id = ? ORDER BY m.created_at, m.id`
const qValidateAccessTargets = `SELECT EXISTS (SELECT 1 FROM ` + "`users`" + ` WHERE organization_id = ? AND id = ? AND deleted_at IS NULL), EXISTS (SELECT 1 FROM ` + "`roles`" + ` WHERE organization_id = ? AND id = ? AND is_enabled = 1), (? = '' OR EXISTS (SELECT 1 FROM ` + "`applications`" + ` WHERE organization_id = ? AND id = ?)), EXISTS (SELECT 1 FROM ` + "`enforcers`" + ` WHERE organization_id = ? AND id = ? AND is_enabled = 1), COALESCE((SELECT username FROM ` + "`users`" + ` WHERE organization_id = ? AND id = ? AND deleted_at IS NULL), ''), COALESCE((SELECT slug FROM ` + "`organizations`" + ` WHERE id = ? AND deleted_at IS NULL), '')`
const qListEffectiveRoleIDs = `SELECT DISTINCT b.role_id FROM ` + "`user_role_bindings`" + ` b JOIN ` + "`roles`" + ` r ON r.id = b.role_id WHERE r.organization_id = ? AND b.user_id = ? AND (b.starts_at IS NULL OR b.starts_at <= ?) AND (b.expires_at IS NULL OR b.expires_at > ?) UNION SELECT l.role_id FROM ` + "`access_lifecycle`" + ` l WHERE l.organization_id = ? AND l.user_id = ? AND l.status = 'active' AND l.starts_at <= ? AND l.expires_at > ?`
const qResolveDefaultEnforcer = `SELECT id FROM ` + "`enforcers`" + ` WHERE organization_id = ? AND is_enabled = 1 ORDER BY created_at LIMIT 1`
const qInsertJITBinding = `INSERT INTO ` + "`user_role_bindings`" + ` (user_id, role_id, source, starts_at, expires_at, lifecycle_id) VALUES (?, ?, 'iga_jit', ?, ?, ?) ON DUPLICATE KEY UPDATE starts_at = IF(source = 'iga_jit', VALUES(starts_at), starts_at), expires_at = IF(source = 'iga_jit', VALUES(expires_at), expires_at), lifecycle_id = IF(source = 'iga_jit', VALUES(lifecycle_id), lifecycle_id), updated_at = IF(source = 'iga_jit', CURRENT_TIMESTAMP, updated_at), revision = IF(source = 'iga_jit', revision + 1, revision)`
const qDeleteJITBinding = `DELETE FROM ` + "`user_role_bindings`" + ` WHERE lifecycle_id = ? AND source = 'iga_jit'`
const qInsertJITPolicy = `INSERT INTO ` + "`casbin_rules`" + ` (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5) VALUES (?, ?, 'p', ?, ?, ?, ?, ?, ?)`
const qDeleteJITPolicy = `DELETE FROM ` + "`casbin_rules`" + ` WHERE id = ? AND ptype = 'p'`
const qRevokePrivilegedSessions = `UPDATE ` + "`sessions`" + ` SET session_state = 'revoked', revoked_at = ?, is_privileged = 0, updated_at = ?, revision = revision + 1 WHERE lifecycle_id = ? AND revoked_at IS NULL`
const qListExpiringLifecycle = `SELECT ` + qLifecycleColumns + ` FROM ` + btLifecycle + ` l WHERE l.status = 'active' AND l.expires_at <= ? ORDER BY l.expires_at LIMIT ?`
const qListExpiringLifecycleByOrg = `SELECT ` + qLifecycleColumns + ` FROM ` + btLifecycle + ` l WHERE l.organization_id = ? AND l.status = 'active' AND l.expires_at <= ? ORDER BY l.expires_at LIMIT ?`
const qCompleteLifecycle = `UPDATE ` + btLifecycle + ` SET status = ?, revoked_at = ?, retry_count = 0, last_error = '', revision = revision + 1 WHERE organization_id = ? AND id = ? AND status = 'active'`
const qRecordLifecycleFailure = `UPDATE ` + btLifecycle + ` SET retry_count = retry_count + 1, last_error = ?, revision = revision + 1 WHERE organization_id = ? AND id = ? AND status = 'active'`
