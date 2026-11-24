package ssf

import (
	"context"
	"database/sql"
	"errors"

	ssfdomain "github.com/guliangiam/guliangiam/internal/domain/ssf"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ReceiverRepository is the MySQL mirror of the SSF receiver registry.
// MySQL has no RLS, so every statement carries the explicit organization_id
// predicate (ADR-023 tenant boundary).
type ReceiverRepository struct {
	db *sql.DB
}

var _ ssfdomain.Repository = (*ReceiverRepository)(nil)

func NewReceiverRepository(db *sql.DB) *ReceiverRepository {
	return &ReceiverRepository{db: db}
}

func (r *ReceiverRepository) Create(ctx context.Context, receiver *ssfdomain.Receiver) error {
	if receiver == nil {
		return apperrors.NewInvalidArgument("ssf receiver is required")
	}
	var createdBy any
	if receiver.CreatedBy != nil {
		createdBy = *receiver.CreatedBy
	}
	_, err := r.db.ExecContext(ctx, queryInsert,
		receiver.ID, receiver.OrganizationID, receiver.Name, receiver.ReceiverURL,
		receiver.AuthTokenHash, receiver.EventTypes, receiver.Status,
		receiver.CreatedAt, receiver.UpdatedAt, createdBy, receiver.Revision,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "create ssf receiver")
	}
	return nil
}

func (r *ReceiverRepository) GetByID(ctx context.Context, organizationID, id string) (*ssfdomain.Receiver, error) {
	receiver, err := scanReceiver(r.db.QueryRowContext(ctx, queryGet, organizationID, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NewNotFound("ssf receiver not found")
		}
		return nil, apperrors.WrapInternalf(err, "get ssf receiver")
	}
	return receiver, nil
}

func (r *ReceiverRepository) ListByOrganization(ctx context.Context, organizationID string) ([]*ssfdomain.Receiver, error) {
	rows, err := r.db.QueryContext(ctx, queryList, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list ssf receivers")
	}
	defer rows.Close()
	items := make([]*ssfdomain.Receiver, 0)
	for rows.Next() {
		receiver, err := scanReceiver(rows)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "scan ssf receiver")
		}
		items = append(items, receiver)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate ssf receivers")
	}
	return items, nil
}

func (r *ReceiverRepository) Delete(ctx context.Context, organizationID, id string) error {
	result, err := r.db.ExecContext(ctx, queryDelete, organizationID, id)
	if err != nil {
		return apperrors.WrapInternalf(err, "delete ssf receiver")
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return apperrors.NewNotFound("ssf receiver not found")
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanReceiver(row scanner) (*ssfdomain.Receiver, error) {
	var receiver ssfdomain.Receiver
	var createdBy sql.NullString
	if err := row.Scan(
		&receiver.ID, &receiver.OrganizationID, &receiver.Name, &receiver.ReceiverURL,
		&receiver.AuthTokenHash, &receiver.EventTypes, &receiver.Status,
		&receiver.CreatedAt, &receiver.UpdatedAt, &createdBy, &receiver.Revision,
	); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		receiver.CreatedBy = &createdBy.String
	}
	return &receiver, nil
}

const (
	queryInsert = `
INSERT INTO ssf_receivers (
  id, organization_id, name, receiver_url, auth_token_hash, event_types,
  status, created_at, updated_at, created_by, revision
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	queryGet = `
SELECT id, organization_id, name, receiver_url, auth_token_hash, event_types,
       status, created_at, updated_at, created_by, revision
FROM ssf_receivers
WHERE organization_id = ? AND id = ?`
	queryList = `
SELECT id, organization_id, name, receiver_url, auth_token_hash, event_types,
       status, created_at, updated_at, created_by, revision
FROM ssf_receivers
WHERE organization_id = ?
ORDER BY created_at DESC`
	queryDelete = `DELETE FROM ssf_receivers WHERE organization_id = ? AND id = ?`
)
