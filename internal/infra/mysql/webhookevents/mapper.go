package webhookevents

import (
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
)

// toWebhookEvent 把 MySQL 表扫描结果转成 domain.WebhookEvent。
// sql.Null* → 指针/零值的转换与 PG 版 scanWebhookEvent 同构。
func toWebhookEvent(row webhookEventRow) (*domain.WebhookEvent, error) {
	e := &domain.WebhookEvent{
		ID:         row.ID,
		WebhookID:  row.WebhookID,
		EventType:  row.EventType,
		Status:     row.Status,
		RetryCount: row.RetryCount,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		Revision:   row.Revision,
	}
	if len(row.Payload) > 0 && string(row.Payload) != "null" {
		e.Payload = json.RawMessage(row.Payload)
	}
	if row.ResponseCode.Valid {
		e.ResponseCode = int(row.ResponseCode.Int64)
	}
	if row.ResponseBody.Valid {
		e.ResponseBody = row.ResponseBody.String
	}
	if row.NextRetryAt.Valid {
		t := row.NextRetryAt.Time
		e.NextRetryAt = &t
	}
	if row.IdempotencyKey.Valid {
		e.IdempotencyKey = row.IdempotencyKey.String
	}
	return e, nil
}

// scanWebhookEvent 是 sql.Row / sql.Rows 的 Scan 适配层。
type scannable interface {
	Scan(dest ...any) error
}

func scanWebhookEventRow(s scannable) (webhookEventRow, error) {
	var row webhookEventRow
	err := s.Scan(
		&row.ID, &row.WebhookID, &row.EventType, &row.Payload,
		&row.ResponseCode, &row.ResponseBody, &row.Status, &row.RetryCount,
		&row.NextRetryAt, &row.IdempotencyKey,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	return row, err
}

// nullString 把 *string / "" / nil 统一成 sql.NullString，用于 MySQL driver。
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullInt64 把 *int / 0 / nil 统一成 sql.NullInt64，用于 MySQL driver。
func nullInt64(n int) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(n), Valid: true}
}

// nullTime 把 *time.Time / nil 统一成 sql.NullTime，用于 MySQL driver。
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
