package models

const btModels = "`models`"

const (
	queryGetModelByID = `
SELECT id, organization_id, name, display_name, description, model_text, owner, is_enabled, created_at, updated_at, revision
FROM ` + btModels + ` WHERE id = ?`

	queryGetModelByIDForOrganization = `
SELECT id, organization_id, name, display_name, description, model_text, owner, is_enabled, created_at, updated_at, revision
FROM ` + btModels + ` WHERE id = ? AND organization_id = ?`

	// queryListModels — orgID×2, keyword×3, is_enabled×2, limit, offset = 9
	queryListModels = `
SELECT id, organization_id, name, display_name, description, model_text, owner, is_enabled, created_at, updated_at, revision
FROM ` + btModels + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountModels — orgID×2, keyword×3, is_enabled×2 = 7
	queryCountModels = `
SELECT COUNT(*) FROM ` + btModels + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)`

	// queryInsertModel — 11 个 ?
	queryInsertModel = `
INSERT INTO ` + btModels + ` (id, organization_id, name, display_name, description, model_text, owner, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateModel — SET 5 + updated_at + id + revision + organization_id = 9 个 ?
	queryUpdateModel = `
UPDATE ` + btModels + ` SET display_name = ?, description = ?, model_text = ?, owner = ?, is_enabled = ?, updated_at = ?, revision = revision + 1 WHERE id = ? AND revision = ? AND organization_id = ?`

	querySelectRevision = `SELECT revision FROM ` + btModels + ` WHERE id = ? AND organization_id = ?`

	queryDeleteModel = `DELETE FROM ` + btModels + ` WHERE id = ? AND organization_id = ?`
)
