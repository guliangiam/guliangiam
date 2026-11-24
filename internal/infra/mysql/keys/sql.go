package keys

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text / $N::boolean   → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   $3::text IS NULL OR col=$3         → ? IS NULL OR col = ?（传 nil 或具体值）
//   $4::boolean IS NULL OR col=$4      → ? IS NULL OR col = ?（传 nil 或 bool）
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   pgx.Query/QueryRow/Exec            → QueryContext/QueryRowContext/ExecContext

const btKeys = "`keys`"

// queryGetKeyByID 占位符：id(1)、organization_id(2)
const queryGetKeyByID = `SELECT id, organization_id, name, display_name, key_type, public_key, private_key, algorithm, expires_at, is_enabled, created_at, updated_at, revision FROM ` + btKeys + ` WHERE id = ? AND organization_id = ?`

// queryListKeys 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 3
//	key_type: ? IS NULL OR key_type=?                     → 2
//	is_enabled: ? IS NULL OR is_enabled=?                 → 2
//	limit, offset                                         → 2
//	总计：11
const queryListKeys = `SELECT id, organization_id, name, display_name, key_type, public_key, NULL AS private_key, algorithm, expires_at, is_enabled, created_at, updated_at, revision FROM ` + btKeys + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR key_type = ?) AND (? IS NULL OR is_enabled = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountKeys: org_id×2 + keyword×3 + key_type×2 + is_enabled×2 = 9
const queryCountKeys = `SELECT COUNT(*) FROM ` + btKeys + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR key_type = ?) AND (? IS NULL OR is_enabled = ?)`

const queryInsertKey = `INSERT INTO ` + btKeys + ` (id, organization_id, name, display_name, key_type, public_key, private_key, algorithm, expires_at, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateKey 占位符：SET 8 + id(9) + organization_id(10)
const queryUpdateKey = `UPDATE ` + btKeys + ` SET display_name=?, public_key=?, private_key=?, algorithm=?, expires_at=?, is_enabled=?, updated_at=?, revision=? WHERE id=? AND organization_id=?`

// queryDeleteKey 占位符：id(1)、organization_id(2)
const queryDeleteKey = `DELETE FROM ` + btKeys + ` WHERE id = ? AND organization_id = ?`
