package transactions

const (
	QueryGetByID = `SELECT id, organization_id, user_id, order_id, payment_id, transaction_type, amount, currency, status, provider_type, provider_id, metadata, created_at, updated_at, revision FROM "guliangiam".transactions WHERE id = $1::uuid`
	// QueryGetByIDForOrganization — 占位符：id(1)、organization_id(2)。
	QueryGetByIDForOrganization = `SELECT id, organization_id, user_id, order_id, payment_id, transaction_type, amount, currency, status, provider_type, provider_id, metadata, created_at, updated_at, revision FROM "guliangiam".transactions WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList                   = `SELECT id, organization_id, user_id, order_id, payment_id, transaction_type, amount, currency, status, provider_type, provider_id, metadata, created_at, updated_at, revision FROM "guliangiam".transactions WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR transaction_type ILIKE '%' || $2 || '%') AND (CASE WHEN $3 = '' THEN TRUE ELSE user_id = $3::uuid END) AND (CASE WHEN $4 = '' THEN TRUE ELSE order_id = $4::uuid END) AND ($5::text IS NULL OR status = $5) AND ($6::text IS NULL OR transaction_type = $6) ORDER BY created_at DESC LIMIT $7 OFFSET $8`
	QueryCount                  = `SELECT COUNT(*) FROM "guliangiam".transactions WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR transaction_type ILIKE '%' || $2 || '%') AND (CASE WHEN $3 = '' THEN TRUE ELSE user_id = $3::uuid END) AND (CASE WHEN $4 = '' THEN TRUE ELSE order_id = $4::uuid END) AND ($5::text IS NULL OR status = $5) AND ($6::text IS NULL OR transaction_type = $6)`
	QueryInsert                 = `INSERT INTO "guliangiam".transactions (id, organization_id, user_id, order_id, payment_id, transaction_type, amount, currency, status, provider_type, provider_id, metadata, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6, $7, $8, $9, $10, $11::uuid, $12, $13, $14, $15)`
	// QueryUpdate — 租户谓词取自实体 OrganizationID（$6）；调用方必须先做 scoped GetByID。
	QueryUpdate = `UPDATE "guliangiam".transactions SET status=$1, metadata=$2, updated_at=$3, revision=$4 WHERE id=$5 AND organization_id=$6::uuid`
	// QueryDelete — 占位符：id(1)、organization_id(2)。
	QueryDelete = `DELETE FROM "guliangiam".transactions WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
