package metering

import (
	"encoding/json"
	"time"
)

// Period is a billing/aggregation window: YYYY-MM for monthly, YYYY-Www for weekly.
type Period string

// MeteringEvent is an immutable chargeable fact. IdempotencyKey dedupes retries
// (uq_metering_events_org_idem).
type MeteringEvent struct {
	ID             string
	OrganizationID string
	EventType      string
	Subject        string // user_id / app_id / pool_id — whatever dimension charges roll up on
	Quantity       int64
	Unit           string
	IdempotencyKey string
	OccurredAt     time.Time
	Metadata       json.RawMessage
	CreatedAt      time.Time
}

// MeteringAggregate is a pre-rolled total for a (org, period, subject, metric) cell.
// Upserted by a background job or on-demand from the events table.
type MeteringAggregate struct {
	ID             string
	OrganizationID string
	Period         Period
	Subject        string
	Metric         string // event_type (summed) or a derived metric
	Quantity       int64
	Unit           string
	CalculatedAt   time.Time
	CreatedAt      time.Time
}

// ─── Queries ──────────────────────────────────────────────────────────────

type ListEventsQuery struct {
	OrganizationID string
	EventType      *string
	Subject        *string
	From           *time.Time
	To             *time.Time
	Page           int
	PageSize       int
}

type ListAggregatesQuery struct {
	OrganizationID string
	Period         *Period
	Subject        *string
	Metric         *string
	Page           int
	PageSize       int
}

// ─── Commands ─────────────────────────────────────────────────────────────

type IngestEventCommand struct {
	OrganizationID string
	EventType      string
	Subject        string
	Quantity       int64
	Unit           string
	IdempotencyKey string
	OccurredAt     time.Time
	Metadata       json.RawMessage
}

// RefreshAggregateCommand 幂等刷新一个 (org, period, subject, metric) 聚合单元格。
// PeriodStart/PeriodEnd 可选：提供时 SQL 用 occurred_at >= PeriodStart AND < PeriodEnd
// 做边界过滤（支持 hour/day/month 多粒度）；不提供时 fallback 到 to_char/DATE_FORMAT
// 按 Period 字符串匹配（兼容历史调用方）。
type RefreshAggregateCommand struct {
	OrganizationID string
	Period         Period // e.g. "2026-10" / "2026-10-15" / "2026-10-15T11"
	Subject        string
	Metric         string // event_type
	PeriodStart    *time.Time
	PeriodEnd      *time.Time
}
