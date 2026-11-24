package entries

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text               → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   pgx.Query/QueryRow/Exec            → QueryContext/QueryRowContext/ExecContext

const btEntries = "`entries`"

// queryGetEntryByID 占位符：id(1)
const queryGetEntryByID = `SELECT id, organization_id, name, display_name, provider, application, type, client_ip, user_agent, message, created_at, updated_at, revision FROM ` + btEntries + ` WHERE id = ?`

// queryGetEntryByIDForOrganization 占位符：id(1), organization_id(2)
const queryGetEntryByIDForOrganization = `SELECT id, organization_id, name, display_name, provider, application, type, client_ip, user_agent, message, created_at, updated_at, revision FROM ` + btEntries + ` WHERE id = ? AND organization_id = ?`

// queryListEntries 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 4
//	type: ?='' OR type=?                                  → 2
//	application: ?='' OR application=?                     → 2
//	limit, offset                                         → 2
//	总计：12
const queryListEntries = `SELECT id, organization_id, name, display_name, type, application, created_at FROM ` + btEntries + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? = '' OR type = ?) AND (? = '' OR application = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountEntries: org_id×2 + keyword×4 + type×2 + application×2 = 10
const queryCountEntries = `SELECT COUNT(*) FROM ` + btEntries + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? = '' OR type = ?) AND (? = '' OR application = ?)`

const queryInsertEntry = `INSERT INTO ` + btEntries + ` (id, organization_id, name, display_name, provider, application, type, client_ip, user_agent, message, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const queryUpdateEntry = `UPDATE ` + btEntries + ` SET display_name=?, message=?, updated_at=?, revision=? WHERE id=? AND organization_id=?`

const queryDeleteEntry = `DELETE FROM ` + btEntries + ` WHERE id = ? AND organization_id = ?`
