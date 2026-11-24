package tickets

const (
	QueryGetByID = `SELECT id, organization_id, name, display_name, type, status, priority, assignee, creator, description, attachment, resolved_at, created_at, updated_at, revision FROM "guliangiam".tickets WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList    = `SELECT id, organization_id, name, display_name, type, status, priority, assignee, creator, description, attachment, resolved_at, created_at, updated_at, revision FROM "guliangiam".tickets WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::text IS NULL OR status = $3) AND ($4 = '' OR assignee = $4) AND ($5 = '' OR creator = $5) AND ($6 = '' OR priority = $6) AND ($7 = '' OR type = $7) ORDER BY created_at DESC LIMIT $8 OFFSET $9`
	QueryCount   = `SELECT COUNT(*) FROM "guliangiam".tickets WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::text IS NULL OR status = $3) AND ($4 = '' OR assignee = $4) AND ($5 = '' OR creator = $5) AND ($6 = '' OR priority = $6) AND ($7 = '' OR type = $7)`
	QueryInsert  = `INSERT INTO "guliangiam".tickets (id, organization_id, name, display_name, type, status, priority, assignee, creator, description, attachment, resolved_at, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`
	QueryUpdate  = `UPDATE "guliangiam".tickets SET display_name=$1, type=$2, status=$3, priority=$4, assignee=$5, description=$6, attachment=$7, resolved_at=$8, updated_at=$9, revision=$10 WHERE id=$11::uuid AND organization_id = guliangiam.resolve_org_id($12)`
	QueryDelete  = `DELETE FROM "guliangiam".tickets WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
