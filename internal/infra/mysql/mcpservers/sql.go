package mcpservers

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::jsonb / $N::bool   → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   $3::bool IS NULL OR col=$3          → ? IS NULL OR col = ?（传 nil 或 bool）
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   pgx.Query/QueryRow/Exec            → QueryContext/QueryRowContext/ExecContext
//   tools jsonb                        → TEXT（JSON 字符串）

const btMcpServers = "`mcp_servers`"

// token 沿用历史列名，但库内保存的是绑定 (table, column, id) 的 v2 信封密文
// （A-6.2）；空串表示“未配置”，仓储在写入前封装、读取时解封。

// queryGetMcpServerByID 占位符：id(1)
const queryGetMcpServerByID = `SELECT id, organization_id, name, display_name, url, token, application, tools, is_enabled, created_at, updated_at, revision FROM ` + btMcpServers + ` WHERE id = ?`

// queryGetMcpServerByIDForOrganization 占位符：id(1)、organization_id(2)
// 跨租户行必须表现得与不存在的行一致。
const queryGetMcpServerByIDForOrganization = `SELECT id, organization_id, name, display_name, url, token, application, tools, is_enabled, created_at, updated_at, revision FROM ` + btMcpServers + ` WHERE id = ? AND organization_id = ?`

// queryListMcpServers 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 3
//	is_enabled: ? IS NULL OR is_enabled=?                 → 2
//	limit, offset                                         → 2
//	总计：9
const queryListMcpServers = `SELECT id, organization_id, name, display_name, url, token, application, tools, is_enabled, created_at, updated_at, revision FROM ` + btMcpServers + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountMcpServers: org_id×2 + keyword×3 + is_enabled×2 = 7
const queryCountMcpServers = `SELECT COUNT(*) FROM ` + btMcpServers + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?)`

const queryInsertMcpServer = `INSERT INTO ` + btMcpServers + ` (id, organization_id, name, display_name, url, token, application, tools, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateMcpServer 占位符：8 个 SET 值 + id + organization_id
const queryUpdateMcpServer = `UPDATE ` + btMcpServers + ` SET display_name=?, url=?, token=?, application=?, tools=?, is_enabled=?, updated_at=?, revision=? WHERE id=? AND organization_id=?`

// queryDeleteMcpServer 占位符：id(1)、organization_id(2)
const queryDeleteMcpServer = `DELETE FROM ` + btMcpServers + ` WHERE id = ? AND organization_id = ?`
