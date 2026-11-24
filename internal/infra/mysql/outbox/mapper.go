package outbox

import (
	"database/sql"
	"time"

	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// toOutboxItem 把 outboxRow（*sql.Row Scan 结果）转成 platform.OutboxItem。
// sql.NullString/NullTime 展平为 *string / *time.Time。
func toOutboxItem(row outboxRow) *platformoutbox.OutboxItem {
	return &platformoutbox.OutboxItem{
		ID:             row.ID,
		OrganizationID: nullStringPtr(row.OrganizationID),
		AggregateType:  row.AggregateType,
		AggregateID:    row.AggregateID,
		EventType:      row.EventType,
		EventVersion:   row.EventVersion,
		Payload:        row.Payload,
		Headers:        row.Headers,
		Status:         platformoutbox.Status(row.Status),
		Attempts:       row.Attempts,
		MaxAttempts:    row.MaxAttempts,
		AvailableAt:    row.AvailableAt,
		LockedBy:       nullStringPtr(row.LockedBy),
		LockedAt:       nullTimePtr(row.LockedAt),
		ProcessedAt:    nullTimePtr(row.ProcessedAt),
		LastError:      nullStringPtr(row.LastError),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
}

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

func nullTimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	t := nt.Time
	return &t
}
