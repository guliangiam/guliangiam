package webhookevents

import (
	"database/sql"
	"time"
)

// webhookEventRow 是 MySQL webhookevents 表的扫描目标。
// 对应 13 列，其中 response_code / response_body / next_retry_at / idempotency_key
// 在 DDL 里可为 NULL，用 sql.Null* 承载。
type webhookEventRow struct {
	ID             string
	WebhookID      string
	EventType      string
	Payload        []byte // JSON
	ResponseCode   sql.NullInt64
	ResponseBody   sql.NullString
	Status         string
	RetryCount     int
	NextRetryAt    sql.NullTime
	IdempotencyKey sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
