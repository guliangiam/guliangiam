package radiusaccountings

const (
	// QueryGetByID — 占位符：id(1)、organization_id(2)。
	QueryGetByID = `SELECT id, organization_id, name, username, service_type, nas_id, nas_ip_addr, nas_port_id, nas_port_type, nas_port, framed_ip_addr, framed_ip_netmask, acct_session_id, acct_session_time, acct_input_total, acct_output_total, acct_input_packets, acct_output_packets, acct_terminate_cause, acct_start_time, acct_stop_time, last_update, created_at, updated_at, revision FROM "guliangiam".radius_accountings WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList    = `SELECT id, organization_id, name, username, acct_session_id, acct_start_time, created_at FROM "guliangiam".radius_accountings WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR username ILIKE '%' || $2 || '%') AND ($3 = '' OR username = $3) ORDER BY created_at DESC LIMIT $4 OFFSET $5`
	QueryCount   = `SELECT COUNT(*) FROM "guliangiam".radius_accountings WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR username ILIKE '%' || $2 || '%') AND ($3 = '' OR username = $3)`
	QueryInsert  = `INSERT INTO "guliangiam".radius_accountings (id, organization_id, name, username, service_type, nas_id, nas_ip_addr, nas_port_id, nas_port_type, nas_port, framed_ip_addr, framed_ip_netmask, acct_session_id, acct_session_time, acct_input_total, acct_output_total, acct_input_packets, acct_output_packets, acct_terminate_cause, acct_start_time, acct_stop_time, last_update, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)`
	// QueryUpdate — 计费字段 + id + organization_id($12)；实体已在 app 层按租户读取。
	QueryUpdate = `UPDATE "guliangiam".radius_accountings SET acct_session_time=$1, acct_input_total=$2, acct_output_total=$3, acct_input_packets=$4, acct_output_packets=$5, acct_terminate_cause=$6, acct_stop_time=$7, last_update=$8, updated_at=$9, revision=$10 WHERE id=$11 AND organization_id = guliangiam.resolve_org_id($12)`
	// QueryDelete — 占位符：id(1)、organization_id(2)。
	QueryDelete = `DELETE FROM "guliangiam".radius_accountings WHERE id=$1 AND organization_id = guliangiam.resolve_org_id($2)`
)
