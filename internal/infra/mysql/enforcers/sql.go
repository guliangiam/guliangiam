package enforcers

// SQL 方言转换规则（PG → MySQL）：
//
//	$N::uuid → ?；"guliangiam".table → `table`
//	ILIKE → LIKE CONCAT('%', ?, '%')
//	$3::boolean IS NULL → (? IS NULL)
//	guliangiam.resolve_org_id($1) → $1
//	RETURNING revision → UPDATE + SELECT revision
const btEnforcers = "`enforcers`"

const (
	queryGetEnforcerByID = `
SELECT id, organization_id, name, display_name, description, model_id, adapter_id, is_enabled, owner, created_at, updated_at, revision
FROM ` + btEnforcers + ` WHERE id = ?`

	queryGetEnforcerByIDForOrganization = `
SELECT id, organization_id, name, display_name, description, model_id, adapter_id, is_enabled, owner, created_at, updated_at, revision
FROM ` + btEnforcers + ` WHERE id = ? AND organization_id = ?`

	// queryListEnforcers — orgID×2, keyword×3, is_enabled×2, limit, offset = 9
	queryListEnforcers = `
SELECT id, organization_id, name, display_name, description, model_id, adapter_id, is_enabled, owner, created_at, updated_at, revision
FROM ` + btEnforcers + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountEnforcers — orgID×2, keyword×3, is_enabled×2 = 7
	queryCountEnforcers = `
SELECT COUNT(*) FROM ` + btEnforcers + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)`

	// queryInsertEnforcer — 12 个 ?
	queryInsertEnforcer = `
INSERT INTO ` + btEnforcers + ` (id, organization_id, name, display_name, description, model_id, adapter_id, is_enabled, owner, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateEnforcer — SET 6 + updated_at + id + revision + organization_id = 10 个 ?
	queryUpdateEnforcer = `
UPDATE ` + btEnforcers + ` SET display_name = ?, description = ?, model_id = ?, adapter_id = ?, is_enabled = ?, owner = ?, updated_at = ?, revision = revision + 1 WHERE id = ? AND revision = ? AND organization_id = ?`

	querySelectRevision = `SELECT revision FROM ` + btEnforcers + ` WHERE id = ? AND organization_id = ?`

	queryDeleteEnforcer = `DELETE FROM ` + btEnforcers + ` WHERE id = ? AND organization_id = ?`
)
