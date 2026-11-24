package actions

import domain "github.com/guliangiam/guliangiam/internal/domain/actions"

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::jsonb  → ?
//   "guliangiam".table    → `table`
//   now()                 → NOW()
//   fmt.Sprintf(" LIMIT $%d OFFSET $%d", ...) → 参数化 LIMIT ? OFFSET ?

const btActions = "`identity_actions`"
const btVersions = "`action_versions`"
const btExecLogs = "`execution_logs`"

// paginationSuffix 是 MySQL 参数化分页片段（含前导空格，SQL 文本逐字不变）。
const paginationSuffix = " LIMIT ? OFFSET ?"

// ─── Actions ────────────────────────────────────────────────────

const queryGetAction = `
SELECT id, organization_id, name, trigger_type, status, created_at, updated_at, revision
FROM ` + btActions + `
WHERE id = ? AND organization_id = ?`

const queryInsertAction = `
INSERT INTO ` + btActions + `
  (id, organization_id, name, trigger_type, status, revision)
VALUES (?, ?, ?, ?, ?, 1)`

const queryUpdateActionStatus = `
UPDATE ` + btActions + `
SET status = ?, updated_at = NOW()
WHERE id = ? AND organization_id = ?`

// buildListActionsSQL 构造 List/Count 的动态 SQL。
func buildListActionsSQL(q domain.ListActionsQuery, withPaging bool) (queryOut, whereClauseOut string, argsOut []any) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.TriggerType != nil && *q.TriggerType != "" {
		args = append(args, string(*q.TriggerType))
		extra += " AND trigger_type = ?"
	}
	if q.Status != nil && *q.Status != "" {
		args = append(args, string(*q.Status))
		extra += " AND status = ?"
	}

	countSQL := `SELECT COUNT(*) FROM ` + btActions + ` WHERE organization_id = ?` + extra
	selectSQL := `SELECT id, organization_id, name, trigger_type, status, created_at, updated_at, revision FROM ` + btActions + ` WHERE organization_id = ?` + extra + " ORDER BY updated_at DESC"
	if withPaging {
		selectSQL += paginationSuffix
	}
	return selectSQL, countSQL, args
}

// ─── Versions ────────────────────────────────────────────────────

const queryListVersionsBase = `FROM ` + btVersions + ` WHERE action_id = ? AND EXISTS (SELECT 1 FROM ` + btActions + ` a WHERE a.id = action_versions.action_id AND a.organization_id = ?)`

const queryListVersionsSelect = `SELECT id, action_id, version, code, created_at `

const queryInsertVersion = `
INSERT INTO ` + btVersions + `
  (id, action_id, version, code)
VALUES (?, ?, ?, ?)`

const queryLatestVersion = `
SELECT id, action_id, version, code, created_at
FROM ` + btVersions + `
WHERE action_id = ?
ORDER BY version DESC
LIMIT 1`

// ─── Execution Logs ──────────────────────────────────────────────

const queryListExecLogsBase = `FROM ` + btExecLogs + ` WHERE action_id = ? AND EXISTS (SELECT 1 FROM ` + btActions + ` a WHERE a.id = execution_logs.action_id AND a.organization_id = ?)`

const queryListExecLogsSelect = `SELECT id, action_id, version, status, input, output, error, executed_at, created_at `

const queryInsertExecLog = `
INSERT INTO ` + btExecLogs + `
  (id, action_id, version, status, input)
VALUES (?, ?, ?, 'pending', ?)`

const queryUpdateExecLog = `
UPDATE ` + btExecLogs + `
SET status = ?, output = ?, error = ?, executed_at = NOW()
WHERE id = ?`

// buildListExecLogsSQL 动态拼接 ListExecutionLogs SQL。
func buildListExecLogsSQL(q domain.ListExecutionLogsQuery, withPaging bool) (queryOut, whereClauseOut string, argsOut []any) {
	args := []any{q.ActionID, q.OrganizationID}
	extra := ""
	if q.Status != nil && *q.Status != "" {
		args = append(args, string(*q.Status))
		extra += " AND status = ?"
	}

	countSQL := `SELECT COUNT(*) ` + queryListExecLogsBase + extra
	selectSQL := queryListExecLogsSelect + queryListExecLogsBase + extra + " ORDER BY created_at DESC"
	if withPaging {
		selectSQL += paginationSuffix
	}
	return selectSQL, countSQL, args
}
