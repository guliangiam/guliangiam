package radiusaccountings

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid                          → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?

const btRA = "`radius_accountings`"

// queryGetRAByID 占位符：id(1)、organization_id(2)
const queryGetRAByID = `SELECT id, organization_id, name, username, service_type, nas_id, nas_ip_addr, nas_port_id, nas_port_type, nas_port, framed_ip_addr, framed_ip_netmask, acct_session_id, acct_session_time, acct_input_total, acct_output_total, acct_input_packets, acct_output_packets, acct_terminate_cause, acct_start_time, acct_stop_time, last_update, created_at, updated_at, revision FROM ` + btRA + ` WHERE id = ? AND organization_id = ?`

// queryListRA 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR username LIKE CONCAT('%', ?, '%') → 3
//	username: ?='' OR username=?                           → 2
//	limit, offset                                         → 2
//	总计：9
const queryListRA = `SELECT id, organization_id, name, username, acct_session_id, acct_start_time, created_at FROM ` + btRA + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR username LIKE CONCAT('%', ?, '%')) AND (? = '' OR username = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountRA: org_id×2 + keyword×3 + username×2 = 7
const queryCountRA = `SELECT COUNT(*) FROM ` + btRA + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR username LIKE CONCAT('%', ?, '%')) AND (? = '' OR username = ?)`

const queryInsertRA = `INSERT INTO ` + btRA + ` (id, organization_id, name, username, service_type, nas_id, nas_ip_addr, nas_port_id, nas_port_type, nas_port, framed_ip_addr, framed_ip_netmask, acct_session_id, acct_session_time, acct_input_total, acct_output_total, acct_input_packets, acct_output_packets, acct_terminate_cause, acct_start_time, acct_stop_time, last_update, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateRA 占位符：10 个计费字段 + id(11) + organization_id(12)
const queryUpdateRA = `UPDATE ` + btRA + ` SET acct_session_time=?, acct_input_total=?, acct_output_total=?, acct_input_packets=?, acct_output_packets=?, acct_terminate_cause=?, acct_stop_time=?, last_update=?, updated_at=?, revision=? WHERE id=? AND organization_id=?`

// queryDeleteRA 占位符：id(1)、organization_id(2)
const queryDeleteRA = `DELETE FROM ` + btRA + ` WHERE id = ? AND organization_id = ?`
