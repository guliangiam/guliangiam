package subscriptions

const (
	// QueryGetByIDForOrganization — 占位符：id(1)、organization_id(2)。
	QueryGetByIDForOrganization = `SELECT id, organization_id, name, display_name, user_id, plan_id, pricing_id, status, start_at, end_at, expire_at, created_at, updated_at, revision FROM "guliangiam".subscriptions WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList                   = `SELECT id, organization_id, name, display_name, user_id, plan_id, pricing_id, status, start_at, end_at, expire_at, created_at, updated_at, revision FROM "guliangiam".subscriptions WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND (CASE WHEN $3 = '' THEN TRUE ELSE user_id = $3::uuid END) AND ($4::text IS NULL OR status = $4) ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	QueryCount                  = `SELECT COUNT(*) FROM "guliangiam".subscriptions WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND (CASE WHEN $3 = '' THEN TRUE ELSE user_id = $3::uuid END) AND ($4::text IS NULL OR status = $4)`
	QueryInsert                 = `INSERT INTO "guliangiam".subscriptions (id, organization_id, name, display_name, user_id, plan_id, pricing_id, status, start_at, end_at, expire_at, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid, $6::uuid, $7::uuid, $8, $9, $10, $11, $12, $13, $14)`
	// QueryUpdate — 租户谓词取自实体 OrganizationID（$9）；调用方必须先做 scoped GetByIDForOrganization。
	QueryUpdate = `UPDATE "guliangiam".subscriptions SET display_name=$1, status=$2, start_at=$3, end_at=$4, expire_at=$5, updated_at=$6, revision=$7 WHERE id=$8 AND organization_id=$9::uuid`
	// QueryDelete — 占位符：id(1)、organization_id(2)。
	QueryDelete = `DELETE FROM "guliangiam".subscriptions WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
