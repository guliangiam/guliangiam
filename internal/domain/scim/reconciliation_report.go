package scim

import (
	"context"
	"time"
)

// ReconciliationReport is the persisted PROV-011 drift report for one SCIM
// outbound connector run. The JSON payloads contain the full
// UserReconciliationReport / GroupReconciliationReport structures produced by
// the outbound reconciler, so downstream tooling never has to re-fetch to
// explain a drift.
type ReconciliationReport struct {
	ID             string
	OrganizationID string
	ConnectorID    string
	Status         string
	StartedAt      time.Time
	FinishedAt     time.Time
	UserReport     []byte
	GroupReport    []byte
	ErrorMessage   string
	CreatedAt      time.Time
}

// ReconciliationReportRepository stores and lists reconciliation reports.
// Reports are append-only; updates/deletes are not part of the contract.
type ReconciliationReportRepository interface {
	Create(ctx context.Context, report *ReconciliationReport) error
	ListByOrganization(ctx context.Context, organizationID string, limit, offset int) ([]*ReconciliationReport, int64, error)
}
