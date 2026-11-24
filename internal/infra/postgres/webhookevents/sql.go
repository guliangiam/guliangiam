package webhookevents

// webhook_events 无 organization_id 列，租户归属只能经父表 webhooks（RLS 谓词
// 同为父表绑定）。List/Count 必须以同一 EXISTS 谓词绑定父 webhook 的
// organization_id，否则空租户在 PG（super_admin 放行）与 MySQL（无 RLS）都会
// 退化为跨租户读取。
const tenantPredicate = `AND (CASE WHEN $4 = '' THEN TRUE ELSE EXISTS (SELECT 1 FROM "guliangiam".webhooks w WHERE w.id = webhook_events.webhook_id AND w.organization_id = $4::uuid) END)`

const (
	QueryGetByID = `SELECT id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision FROM "guliangiam".webhook_events WHERE id = $1::uuid`
	// QueryGetByIDForOrganization — 占位符：id(1)、organization_id(2)。
	// webhook_events 无 organization_id 列，租户归属经父表 webhooks 绑定。
	QueryGetByIDForOrganization = `SELECT id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision FROM "guliangiam".webhook_events WHERE id = $1::uuid AND EXISTS (SELECT 1 FROM "guliangiam".webhooks w WHERE w.id = webhook_events.webhook_id AND w.organization_id = guliangiam.resolve_org_id($2))`
	QueryList                   = `SELECT id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision FROM "guliangiam".webhook_events WHERE (CASE WHEN $1 = '' THEN TRUE ELSE webhook_id = $1::uuid END) AND ($2 = '' OR event_type = $2) AND ($3::text IS NULL OR status = $3) ` + tenantPredicate + ` ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	QueryCount                  = `SELECT COUNT(*) FROM "guliangiam".webhook_events WHERE (CASE WHEN $1 = '' THEN TRUE ELSE webhook_id = $1::uuid END) AND ($2 = '' OR event_type = $2) AND ($3::text IS NULL OR status = $3) ` + tenantPredicate
	QueryInsert                 = `INSERT INTO "guliangiam".webhook_events (id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision) VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	// QueryListExistingIDs — 参数：ids(1)。扇出批量幂等检查，替代逐订阅者 GetByID。
	QueryListExistingIDs = `SELECT id::text FROM "guliangiam".webhook_events WHERE id = ANY($1::uuid[])`
	QueryUpdate          = `UPDATE "guliangiam".webhook_events SET response_code=$1, response_body=$2, status=$3, retry_count=$4, next_retry_at=$5, idempotency_key=$6, updated_at=$7, revision=$8 WHERE id=$9::uuid`
	QueryDelete          = `DELETE FROM "guliangiam".webhook_events WHERE id=$1::uuid`
)
