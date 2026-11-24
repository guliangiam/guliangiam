package forms

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::boolean              → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   $3::boolean IS NULL OR col=$3      → ? IS NULL OR col = ?（直接传 nil 或 bool）
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   pgx.Query/QueryRow/Exec            → QueryContext/QueryRowContext/ExecContext

const btForms = "`forms`"

// queryGetFormByID 占位符：id(1)、organization_id(2)
const queryGetFormByID = `SELECT id, organization_id, name, display_name, description, type, properties, is_enabled, created_at, updated_at, revision FROM ` + btForms + ` WHERE id = ? AND organization_id = ?`

// queryListForms 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 3
//	is_enabled: ? IS NULL OR is_enabled=?                 → 2
//	type: ?='' OR type=?                                  → 2
//	limit, offset                                         → 2
//	总计：11
const queryListForms = `SELECT id, organization_id, name, display_name, description, type, properties, is_enabled, created_at, updated_at, revision FROM ` + btForms + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?) AND (? = '' OR type = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountForms: org_id×2 + keyword×3 + is_enabled×2 + type×2 = 9
const queryCountForms = `SELECT COUNT(*) FROM ` + btForms + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?) AND (? = '' OR type = ?)`

const queryInsertForm = `INSERT INTO ` + btForms + ` (id, organization_id, name, display_name, description, type, properties, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const queryUpdateForm = `UPDATE ` + btForms + ` SET display_name=?, description=?, type=?, properties=?, is_enabled=?, updated_at=?, revision=? WHERE id=? AND organization_id=?`

const queryDeleteForm = `DELETE FROM ` + btForms + ` WHERE id = ? AND organization_id = ?`
