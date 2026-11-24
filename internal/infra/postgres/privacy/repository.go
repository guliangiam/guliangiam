package privacy

import (
	"context"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// Repository persists DSAR requests and their immutable events on the DBTX it
// is bound to. The caller (app layer transaction runner) owns the transaction,
// so a state change can never commit without its event.
type Repository struct {
	db shared.DBTX
}

var _ domain.DsarRepository = Repository{}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDsarRequest(row scanner) (*domain.DsarRequest, error) {
	var request domain.DsarRequest
	if err := row.Scan(
		&request.ID, &request.OrganizationID, &request.SubjectType, &request.SubjectID,
		&request.RequestType, &request.State, &request.SLADueAt, &request.ClosedAt, &request.CoolingUntil,
		&request.RejectionReason, &request.CreatedBy, &request.CreatedAt, &request.UpdatedAt, &request.Revision,
	); err != nil {
		return nil, err
	}
	return &request, nil
}

func encodeEventPayload(payload map[string]any) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "marshal dsar event payload")
	}
	return encoded, nil
}

// Create inserts the request and its created event on the caller transaction.
func (r Repository) Create(ctx context.Context, request *domain.DsarRequest, event *domain.DsarEvent) error {
	payload, err := encodeEventPayload(event.Payload)
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(ctx, queryInsertDsarRequest,
		request.ID, request.OrganizationID, request.SubjectType, request.SubjectID,
		request.RequestType, request.State, request.SLADueAt, request.ClosedAt, request.CoolingUntil, request.RejectionReason,
		request.CreatedAt, request.UpdatedAt, request.CreatedBy, request.Revision,
	); err != nil {
		return apperrors.WrapInternalf(err, "insert dsar request")
	}
	if _, err := r.db.Exec(ctx, queryInsertDsarEvent,
		event.ID, event.OrganizationID, event.DsarID, event.Action, event.ActorID, payload, event.CreatedAt,
	); err != nil {
		return apperrors.WrapInternalf(err, "insert dsar event")
	}
	return nil
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.DsarRequest, error) {
	request, err := scanDsarRequest(r.db.QueryRow(ctx, queryGetDsarRequestForOrganization, id, organizationID))
	if err == pgx.ErrNoRows {
		return nil, apperrors.NewNotFound("dsar request not found")
	}
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get dsar request")
	}
	return request, nil
}

// UpdateWithEvent persists a state change guarded by the caller's revision and
// appends the matching event on the same transaction.
func (r Repository) UpdateWithEvent(ctx context.Context, request *domain.DsarRequest, event *domain.DsarEvent, expectedRevision int64) error {
	payload, err := encodeEventPayload(event.Payload)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, queryUpdateDsarRequest,
		request.State, request.SLADueAt, request.ClosedAt, request.CoolingUntil, request.RejectionReason,
		request.UpdatedAt, request.ID, request.OrganizationID, expectedRevision,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "update dsar request")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewConflict("dsar request revision mismatch")
	}
	if _, err := r.db.Exec(ctx, queryInsertDsarEvent,
		event.ID, event.OrganizationID, event.DsarID, event.Action, event.ActorID, payload, event.CreatedAt,
	); err != nil {
		return apperrors.WrapInternalf(err, "insert dsar event")
	}
	return nil
}

func (r Repository) ListOverdue(ctx context.Context, now time.Time, limit int) ([]*domain.DsarRequest, error) {
	rows, err := r.db.Query(ctx, queryListOverdueDsarRequests, now, limit)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list overdue dsar requests")
	}
	defer rows.Close()
	requests := make([]*domain.DsarRequest, 0, limit)
	for rows.Next() {
		request, err := scanDsarRequest(rows)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "scan overdue dsar request")
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate overdue dsar requests")
	}
	return requests, nil
}

// ListByOrganization returns a page of requests scoped to the tenant plus the
// total for pagination. The organization filter is explicit so the tenant
// boundary does not rely on RLS alone.
func (r Repository) ListByOrganization(ctx context.Context, organizationID string, limit, offset int) ([]*domain.DsarRequest, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, queryCountDsarRequestsForOrganization, organizationID).Scan(&total); err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "count dsar requests")
	}
	rows, err := r.db.Query(ctx, queryListDsarRequestsForOrganization, organizationID, limit, offset)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "list dsar requests")
	}
	defer rows.Close()
	requests := make([]*domain.DsarRequest, 0, limit)
	for rows.Next() {
		request, err := scanDsarRequest(rows)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "scan dsar request")
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "iterate dsar requests")
	}
	return requests, total, nil
}
