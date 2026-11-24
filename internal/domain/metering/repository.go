package metering

import (
	"context"
	"time"
)

// Repository is the persistence contract. Everything is organization-scoped.
type Repository interface {
	// Events
	ListEvents(ctx context.Context, q ListEventsQuery) ([]*MeteringEvent, int64, error)
	InsertEvent(ctx context.Context, event *MeteringEvent) (bool, error) // (inserted, error) — false on duplicate
	IngestBatch(ctx context.Context, events []*MeteringEvent) (inserted int, duplicates int, err error)

	// Combos — distinct (org, subject, event_type) 组合扫描
	// 供后台 worker 批量 RefreshAggregate 用（避免逐事件扫描）
	ScanDistinctCombos(ctx context.Context, since time.Time) ([]Combo, error)

	// Aggregates
	ListAggregates(ctx context.Context, q ListAggregatesQuery) ([]*MeteringAggregate, int64, error)
	UpsertAggregate(ctx context.Context, agg *MeteringAggregate) error
	RefreshAggregate(ctx context.Context, cmd RefreshAggregateCommand) (*MeteringAggregate, error)
}

// DetailReader is the optional tenant-scoped single-row read port used by the
// detail endpoints. Keeping it separate from Repository lets read-only fakes
// stay small; the app layer reports 501 when a backend does not implement it.
type DetailReader interface {
	GetEventByID(ctx context.Context, id, organizationID string) (*MeteringEvent, error)
	GetAggregateByID(ctx context.Context, id, organizationID string) (*MeteringAggregate, error)
}

// Combo 是 metering_events 的 distinct (org, subject, event_type) 组合。
type Combo struct {
	OrganizationID string
	Subject        string
	EventType      string
}
