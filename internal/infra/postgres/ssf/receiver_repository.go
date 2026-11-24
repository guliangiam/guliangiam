package ssf

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	ssfdomain "github.com/guliangiam/guliangiam/internal/domain/ssf"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const queryInsertSSFReceiver = `
INSERT INTO guliangiam.ssf_receivers (
	id, organization_id, name, receiver_url, auth_token_hash, event_types,
	status, created_at, updated_at, created_by, revision
) VALUES (
	$1::uuid, $2::uuid, $3, $4, $5, $6::jsonb, $7, $8, $9, $10::uuid, $11
)`

const queryGetSSFReceiver = `
SELECT id, organization_id, name, receiver_url, auth_token_hash, event_types,
	status, created_at, updated_at, created_by, revision
FROM guliangiam.ssf_receivers
WHERE organization_id = $1::uuid AND id = $2::uuid`

const queryListSSFReceivers = `
SELECT id, organization_id, name, receiver_url, auth_token_hash, event_types,
	status, created_at, updated_at, created_by, revision
FROM guliangiam.ssf_receivers
WHERE organization_id = $1::uuid
ORDER BY created_at DESC`

const queryDeleteSSFReceiver = `
DELETE FROM guliangiam.ssf_receivers
WHERE organization_id = $1::uuid AND id = $2::uuid`

const querySetSSFTenant = `
SELECT guliangiam.set_tenant_context($1::uuid, 'tenant')`

// ReceiverRepository is the PostgreSQL adapter for SSF receivers.
type ReceiverRepository struct {
	pool *pgxpool.Pool
}

// NewReceiverRepository constructs the adapter.
func NewReceiverRepository(pool *pgxpool.Pool) *ReceiverRepository {
	return &ReceiverRepository{pool: pool}
}

func (r *ReceiverRepository) withTenantTx(ctx context.Context, organizationID string, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, querySetSSFTenant, organizationID); err != nil {
		return apperrors.WrapInternalf(err, "set ssf receiver tenant context")
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Create inserts one receiver.
func (r *ReceiverRepository) Create(ctx context.Context, receiver *ssfdomain.Receiver) error {
	if receiver == nil {
		return apperrors.NewInvalidArgument("ssf receiver is required")
	}
	now := time.Now().UTC()
	if receiver.CreatedAt.IsZero() {
		receiver.CreatedAt = now
	}
	if receiver.UpdatedAt.IsZero() {
		receiver.UpdatedAt = now
	}
	if receiver.Revision == 0 {
		receiver.Revision = 1
	}
	return r.withTenantTx(ctx, receiver.OrganizationID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, queryInsertSSFReceiver,
			receiver.ID,
			receiver.OrganizationID,
			receiver.Name,
			receiver.ReceiverURL,
			receiver.AuthTokenHash,
			receiver.EventTypes,
			receiver.Status,
			receiver.CreatedAt,
			receiver.UpdatedAt,
			receiver.CreatedBy,
			receiver.Revision,
		)
		if err != nil {
			return apperrors.WrapInternalf(err, "create ssf receiver")
		}
		return nil
	})
}

// GetByID returns one receiver scoped to the organization.
func (r *ReceiverRepository) GetByID(ctx context.Context, organizationID, id string) (*ssfdomain.Receiver, error) {
	var receiver ssfdomain.Receiver
	err := r.withTenantTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryGetSSFReceiver, organizationID, id).Scan(
			&receiver.ID,
			&receiver.OrganizationID,
			&receiver.Name,
			&receiver.ReceiverURL,
			&receiver.AuthTokenHash,
			&receiver.EventTypes,
			&receiver.Status,
			&receiver.CreatedAt,
			&receiver.UpdatedAt,
			&receiver.CreatedBy,
			&receiver.Revision,
		)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("ssf receiver not found")
		}
		return nil, apperrors.WrapInternalf(err, "get ssf receiver")
	}
	return &receiver, nil
}

// ListByOrganization returns all receivers for a tenant.
func (r *ReceiverRepository) ListByOrganization(ctx context.Context, organizationID string) ([]*ssfdomain.Receiver, error) {
	var receivers []*ssfdomain.Receiver
	err := r.withTenantTx(ctx, organizationID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, queryListSSFReceivers, organizationID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var receiver ssfdomain.Receiver
			if err := rows.Scan(
				&receiver.ID,
				&receiver.OrganizationID,
				&receiver.Name,
				&receiver.ReceiverURL,
				&receiver.AuthTokenHash,
				&receiver.EventTypes,
				&receiver.Status,
				&receiver.CreatedAt,
				&receiver.UpdatedAt,
				&receiver.CreatedBy,
				&receiver.Revision,
			); err != nil {
				return err
			}
			receivers = append(receivers, &receiver)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list ssf receivers")
	}
	return receivers, nil
}

// Delete removes one receiver scoped to the organization.
func (r *ReceiverRepository) Delete(ctx context.Context, organizationID, id string) error {
	return r.withTenantTx(ctx, organizationID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, queryDeleteSSFReceiver, organizationID, id)
		return err
	})
}
