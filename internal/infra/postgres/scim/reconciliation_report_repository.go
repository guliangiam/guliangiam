package scim

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	scimdomain "github.com/guliangiam/guliangiam/internal/domain/scim"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const queryInsertReconciliationReport = `
INSERT INTO guliangiam.scim_reconciliation_reports (
	id, organization_id, connector_id, status, started_at, finished_at,
	user_report, group_report, error_message, created_at
) VALUES (
	$1::uuid, $2::uuid, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10
)`

const queryListReconciliationReports = `
SELECT id, organization_id, connector_id, status, started_at, finished_at,
	user_report, group_report, error_message, created_at
FROM guliangiam.scim_reconciliation_reports
WHERE organization_id = $1::uuid
ORDER BY created_at DESC
LIMIT $2 OFFSET $3`

const queryCountReconciliationReports = `
SELECT COUNT(*) FROM guliangiam.scim_reconciliation_reports WHERE organization_id = $1::uuid`

const querySetReconciliationTenant = `
SELECT guliangiam.set_tenant_context($1::uuid, 'tenant')`

// ReconciliationReportRepository is the PostgreSQL persistence adapter for
// SCIM outbound reconciliation reports.
type ReconciliationReportRepository struct {
	pool *pgxpool.Pool
}

// NewReconciliationReportRepository constructs the adapter.
func NewReconciliationReportRepository(pool *pgxpool.Pool) *ReconciliationReportRepository {
	return &ReconciliationReportRepository{pool: pool}
}

// withTenantTx runs fn inside a transaction whose RLS context is the report's
// organization. This adapter is also used by the operator CLI, which has no
// HTTP session to inject the tenant GUC; without this the FORCE RLS insert
// would fail closed.
func (r *ReconciliationReportRepository) withTenantTx(ctx context.Context, organizationID string, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, querySetReconciliationTenant, organizationID); err != nil {
		return apperrors.WrapInternalf(err, "set scim reconciliation tenant context")
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Create inserts one reconciliation report.
func (r *ReconciliationReportRepository) Create(ctx context.Context, report *scimdomain.ReconciliationReport) error {
	if report == nil {
		return apperrors.NewInvalidArgument("reconciliation report is required")
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now().UTC()
	}
	err := r.withTenantTx(ctx, report.OrganizationID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, queryInsertReconciliationReport,
			report.ID,
			report.OrganizationID,
			report.ConnectorID,
			report.Status,
			report.StartedAt,
			report.FinishedAt,
			report.UserReport,
			report.GroupReport,
			report.ErrorMessage,
			report.CreatedAt,
		)
		return err
	})
	if err != nil {
		return apperrors.WrapInternalf(err, "create scim reconciliation report")
	}
	return nil
}

// ListByOrganization pages through reports newest-first.
func (r *ReconciliationReportRepository) ListByOrganization(ctx context.Context, organizationID string, limit, offset int) ([]*scimdomain.ReconciliationReport, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var reports []*scimdomain.ReconciliationReport
	var total int64
	err := r.withTenantTx(ctx, organizationID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, queryListReconciliationReports, organizationID, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		reports = make([]*scimdomain.ReconciliationReport, 0, limit)
		for rows.Next() {
			var report scimdomain.ReconciliationReport
			if err := rows.Scan(
				&report.ID,
				&report.OrganizationID,
				&report.ConnectorID,
				&report.Status,
				&report.StartedAt,
				&report.FinishedAt,
				&report.UserReport,
				&report.GroupReport,
				&report.ErrorMessage,
				&report.CreatedAt,
			); err != nil {
				return err
			}
			reports = append(reports, &report)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return tx.QueryRow(ctx, queryCountReconciliationReports, organizationID).Scan(&total)
	})
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "list scim reconciliation reports")
	}
	return reports, total, nil
}
