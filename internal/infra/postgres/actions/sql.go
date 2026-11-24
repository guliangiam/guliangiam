package actions

// ─── identity_actions ─────────────────────────────────────────────────────

const queryListActionsBase = `FROM guliangiam.identity_actions WHERE organization_id = $1`

const queryListActionsSelect = `
SELECT id, organization_id, name, trigger_type, status, created_at, updated_at, revision
`

const queryListActionsOrderPage = `
ORDER BY updated_at DESC
LIMIT $%d OFFSET $%d
`

// queryLimitOffsetPage is the bare pagination tail for child tables that
// already carry their own ORDER BY clause.
const queryLimitOffsetPage = `
LIMIT $%d OFFSET $%d
`

const queryGetAction = `
SELECT id, organization_id, name, trigger_type, status, created_at, updated_at, revision
FROM guliangiam.identity_actions
WHERE id = $1 AND organization_id = $2
`

const queryInsertAction = `
INSERT INTO guliangiam.identity_actions
  (id, organization_id, name, trigger_type, status, revision)
VALUES
  ($1, $2, $3, $4, $5, 1)
`

const queryUpdateActionStatus = `
UPDATE guliangiam.identity_actions
SET status = $1, updated_at = now(), revision = revision + 1
WHERE id = $2 AND organization_id = $3
`

// ─── action_versions ──────────────────────────────────────────────────────

const queryListVersionsBase = `FROM guliangiam.action_versions
WHERE action_id = $1
  AND EXISTS (
    SELECT 1 FROM guliangiam.identity_actions a
    WHERE a.id = action_versions.action_id AND a.organization_id = $2
  )`

const queryListVersionsSelect = `
SELECT id, action_id, version, code, created_at
`

const queryInsertVersion = `
INSERT INTO guliangiam.action_versions
  (id, action_id, version, code)
VALUES
  ($1, $2, $3, $4)
`

const queryLatestVersion = `
SELECT id, action_id, version, code, created_at
FROM guliangiam.action_versions
WHERE action_id = $1
ORDER BY version DESC
LIMIT 1
`

// ─── execution_logs ───────────────────────────────────────────────────────

const queryListExecLogsBase = `FROM guliangiam.execution_logs
WHERE action_id = $1
  AND EXISTS (
    SELECT 1 FROM guliangiam.identity_actions a
    WHERE a.id = execution_logs.action_id AND a.organization_id = $2
  )`

const queryListExecLogsSelect = `
SELECT id, action_id, version, status, input, output, error, executed_at, created_at
`

const queryInsertExecLog = `
INSERT INTO guliangiam.execution_logs
  (id, action_id, version, status, input)
VALUES
  ($1, $2, $3, 'pending', $4::jsonb)
`

const queryUpdateExecLog = `
UPDATE guliangiam.execution_logs
SET status = $1, output = $2::jsonb, error = $3, executed_at = now()
WHERE id = $4
`
