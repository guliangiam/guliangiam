package outbox

import (
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/outbox"
)

type outboxRow struct {
	ID             string
	OrganizationID *string
	AggregateType  string
	AggregateID    string
	EventType      string
	EventVersion   int
	Payload        []byte
	Headers        []byte
	Status         string
	Attempts       int
	MaxAttempts    int
	AvailableAt    time.Time
	LockedBy       *string
	LockedAt       *time.Time
	ProcessedAt    *time.Time
	LastError      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

func toOutboxItem(row outboxRow) *outbox.OutboxItem {
	return &outbox.OutboxItem{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		AggregateType:  row.AggregateType,
		AggregateID:    row.AggregateID,
		EventType:      row.EventType,
		EventVersion:   row.EventVersion,
		Payload:        row.Payload,
		Headers:        row.Headers,
		Status:         outbox.Status(row.Status),
		Attempts:       row.Attempts,
		MaxAttempts:    row.MaxAttempts,
		AvailableAt:    row.AvailableAt,
		LockedBy:       row.LockedBy,
		LockedAt:       row.LockedAt,
		ProcessedAt:    row.ProcessedAt,
		LastError:      row.LastError,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
}
