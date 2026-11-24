package entries

const (
	QueryGetByID = `SELECT id, organization_id, name, display_name, provider, application, type, client_ip, user_agent, message, created_at, updated_at, revision FROM "guliangiam".entries WHERE id = $1::uuid`
	// QueryGetByIDForOrganization binds the by-ID read to the trusted tenant;
	// a foreign row and a missing row both return no rows (NotFound).
	QueryGetByIDForOrganization = `SELECT id, organization_id, name, display_name, provider, application, type, client_ip, user_agent, message, created_at, updated_at, revision FROM "guliangiam".entries WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList                   = `SELECT id, organization_id, name, display_name, type, application, created_at FROM "guliangiam".entries WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3 = '' OR type = $3) AND ($4 = '' OR application = $4) ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	QueryCount                  = `SELECT COUNT(*) FROM "guliangiam".entries WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3 = '' OR type = $3) AND ($4 = '' OR application = $4)`
	QueryInsert                 = `INSERT INTO "guliangiam".entries (id, organization_id, name, display_name, provider, application, type, client_ip, user_agent, message, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	QueryUpdate                 = `UPDATE "guliangiam".entries SET display_name=$1, message=$2, updated_at=$3, revision=$4 WHERE id=$5::uuid AND organization_id = guliangiam.resolve_org_id($6)`
	QueryDelete                 = `DELETE FROM "guliangiam".entries WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
