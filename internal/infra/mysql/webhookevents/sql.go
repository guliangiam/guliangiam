package webhookevents

// SQL 方言转换规则（PG → MySQL）：
//
//	$N::uuid / $N::text                                → ?
//	"guliangiam".table                                 → `table`（去掉 schema 前缀）
//	$3::text IS NULL                                   → (? IS NULL)
//	guliangiam.resolve_org_id()                        → 直接传值
//	RETURNING                                          → 删除
//
// 表名常量（Go raw string 不能含反引号，用普通字符串拼接）。
const btWebhookEvents = "`webhook_events`"

// btWebhooks 父表；webhook_events 无 organization_id 列且 MySQL 无 RLS，
// 租户归属只能经父 webhook 的 organization_id 显式谓词。
const btWebhooks = "`webhooks`"

// tenantPredicate — List/Count 必须共用同一租户谓词，保证 total 与 items 一致。
const tenantPredicate = " AND (? = '' OR EXISTS (SELECT 1 FROM " + btWebhooks +
	" w WHERE w.id = webhook_events.webhook_id AND w.organization_id = ?))"

const (
	// queryGetByID — 按主键取 webhook_event。
	queryGetByID = `SELECT id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision FROM ` + btWebhookEvents + ` WHERE id = ?`

	// queryGetByIDForOrganization — 管理面 by-ID 读取的唯一入口。
	// webhook_events 无 organization_id 列且 MySQL 无 RLS，租户归属经父表
	// webhook 的 organization_id 显式谓词绑定。参数：event id(1)、org(2)。
	queryGetByIDForOrganization = `SELECT id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision FROM ` + btWebhookEvents + ` WHERE id = ? AND EXISTS (SELECT 1 FROM ` + btWebhooks + ` w WHERE w.id = webhook_events.webhook_id AND w.organization_id = ?)`

	// queryList — 分页列出 webhookevents。
	// 方言替换：
	//   PG CASE WHEN $1='' → (? = '' OR webhook_id = ?)
	//   PG $3::text IS NULL → (? IS NULL)
	//   PG EXISTS(... guliangiam.webhooks) → EXISTS(... `webhooks`)
	//
	// 参数顺序：webhook_id(×2), event_type(×2), status(×2), organization_id(×2), limit, offset
	// 共 2+2+2+2+2 = 10 个
	queryList = `SELECT id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision FROM ` + btWebhookEvents + ` WHERE (? = '' OR webhook_id = ?) AND (? = '' OR event_type = ?) AND (? IS NULL OR status = ?)` + tenantPredicate + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`

	// queryCount — 与 List 条件同构，只返回 COUNT。
	// 参数顺序：webhook_id(×2), event_type(×2), status(×2), organization_id(×2)
	// 共 2+2+2+2 = 8 个
	queryCount = `SELECT COUNT(*) FROM ` + btWebhookEvents + ` WHERE (? = '' OR webhook_id = ?) AND (? = '' OR event_type = ?) AND (? IS NULL OR status = ?)` + tenantPredicate

	// queryInsert — 插入 webhook_event。
	// 参数：id, webhook_id, event_type, payload, response_code, response_body,
	//       status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision
	// 共 13 个
	queryInsert = `INSERT INTO ` + btWebhookEvents + ` (id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryInsertBatchPrefix — CreateBatch 在其后拼接 (?,...,?) × N 行。
	queryInsertBatchPrefix = `INSERT INTO ` + btWebhookEvents + ` (id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision) VALUES `

	// queryListExistingIDsPrefix — ListExistingIDs 在其后拼接 ? × N 与右括号。
	queryListExistingIDsPrefix = `SELECT id FROM ` + btWebhookEvents + ` WHERE id IN (`

	// queryUpdate — 更新 webhook_event。
	// 参数顺序：response_code, response_body, status, retry_count, next_retry_at,
	//           idempotency_key, updated_at, revision, id
	// 共 9 个
	queryUpdate = `UPDATE ` + btWebhookEvents + ` SET response_code=?, response_body=?, status=?, retry_count=?, next_retry_at=?, idempotency_key=?, updated_at=?, revision=? WHERE id=?`

	// queryDelete — 删除 webhook_event。
	queryDelete = `DELETE FROM ` + btWebhookEvents + ` WHERE id=?`
)
