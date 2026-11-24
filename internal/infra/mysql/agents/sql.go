package agents

// SQL 方言转换规则（PG → MySQL）：
//
//	$N::uuid / $N::boolean → ?
//	"guliangiam".table     → `table`
//	ILIKE                  → LIKE CONCAT('%', ?, '%')
//	$3::boolean IS NULL    → (? IS NULL)
//	guliangiam.resolve_org_id($1) → $1
//	RETURNING revision     → UPDATE + SELECT revision
const btAgents = "`agents`"

const (
	// queryGetAgentByID — 占位符：id(1)、organization_id(2)。
	queryGetAgentByID = `
SELECT id, organization_id, name, display_name, type, host, port, url, token, application_id, is_enabled, last_ping_at, created_at, updated_at, revision
FROM ` + btAgents + ` WHERE id = ? AND organization_id = ?`

	// queryListAgents — orgID×2, keyword×3, is_enabled×2, limit, offset = 2+3+2+2 = 9
	queryListAgents = `
SELECT id, organization_id, name, display_name, type, host, port, url, token, application_id, is_enabled, last_ping_at, created_at, updated_at, revision
FROM ` + btAgents + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountAgents — orgID×2, keyword×3, is_enabled×2 = 7
	queryCountAgents = `
SELECT COUNT(*)
FROM ` + btAgents + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)`

	// queryInsertAgent — 15 个 ?
	queryInsertAgent = `
INSERT INTO ` + btAgents + ` (id, organization_id, name, display_name, type, host, port, url, token, application_id, is_enabled, last_ping_at, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateAgent — SET 9 + id + revision + organization_id = 12 个 ?。
	// app 层已先按租户读取实体并 Revision++，因此 0 affected 只可能是 revision 冲突；
	// 跨租户 id 在读取阶段就已 NotFound。
	queryUpdateAgent = `
UPDATE ` + btAgents + `
SET display_name = ?,
    host = ?,
    port = ?,
    url = ?,
    token = ?,
    application_id = ?,
    is_enabled = ?,
    last_ping_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	// querySelectRevision — 回读 revision；organization_id 谓词防止 MySQL（无 RLS）
	// 在 id 复用时读到其他租户的行。
	querySelectRevision = `SELECT revision FROM ` + btAgents + ` WHERE id = ? AND organization_id = ?`

	// queryDeleteAgent — 占位符：id(1)、organization_id(2)。
	queryDeleteAgent = `DELETE FROM ` + btAgents + ` WHERE id = ? AND organization_id = ?`
)
