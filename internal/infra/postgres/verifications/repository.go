package verifications

import (
	"context"
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Verification, error) {
	return r.scanVerification(r.db.QueryRow(ctx, queryGetVerificationByID, id))
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Verification, error) {
	return r.scanVerification(r.db.QueryRow(ctx, queryGetVerificationByIDForOrganization, id, organizationID))
}

func (r Repository) scanVerification(row pgx.Row) (*domain.Verification, error) {
	var scanned verificationRow
	err := row.Scan(
		&scanned.ID, &scanned.OrganizationID, &scanned.Type, &scanned.Recipient, &scanned.Code,
		&scanned.Status, &scanned.ExpiresAt, &scanned.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("verification not found")
		}
		return nil, err
	}

	verification := toVerification(scanned)
	return &verification, nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Verification, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListVerifications, query.OrganizationID, query.Type, query.Status, query.Recipient, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Verification, 0)
	for rows.Next() {
		var row verificationRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Type, &row.Recipient, &row.Code,
			&row.Status, &row.ExpiresAt, &row.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		verification := toVerification(row)
		items = append(items, &verification)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountVerifications, query.OrganizationID, query.Type, query.Status, query.Recipient).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, verification *domain.Verification) error {
	_, err := r.db.Exec(ctx, queryCreateVerification,
		verification.ID, verification.OrganizationID, verification.Type,
		verification.Recipient, verification.Code, verification.Status,
		verification.ExpiresAt, verification.CreatedAt,
	)
	return err
}

// UpdateStatus transitions an *active* (pending) verification to its terminal
// state. The `status = 'pending'` guard is what makes a one-time code actually
// one-time: two concurrent requests both read the same pending row through
// GetActiveByRecipientAndCode, but only the first UPDATE matches, so the second
// gets a conflict instead of silently consuming the code twice.
func (r Repository) UpdateStatus(ctx context.Context, id, status string) error {
	result, err := r.db.Exec(ctx, queryUpdateVerificationStatus, id, status)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewConflict("verification code has already been used or expired")
	}
	return nil
}

func (r Repository) GetActiveByRecipientAndCode(ctx context.Context, recipient, code, verificationType string) (*domain.Verification, error) {
	var row verificationRow
	err := r.db.QueryRow(ctx, queryGetActiveVerificationByRecipientAndCode, recipient, code, verificationType).Scan(
		&row.ID, &row.OrganizationID, &row.Type, &row.Recipient, &row.Code,
		&row.Status, &row.ExpiresAt, &row.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("verification not found or expired")
		}
		return nil, err
	}

	verification := toVerification(row)
	return &verification, nil
}

// GetActiveByRecipient returns the newest pending verification for a
// recipient regardless of the stored code. Provider-managed senders keep an
// empty code because the vendor generated it.
func (r Repository) GetActiveByRecipient(ctx context.Context, recipient, verificationType string) (*domain.Verification, error) {
	var row verificationRow
	err := r.db.QueryRow(ctx, queryGetActiveVerificationByRecipient, recipient, verificationType).Scan(
		&row.ID, &row.OrganizationID, &row.Type, &row.Recipient, &row.Code,
		&row.Status, &row.ExpiresAt, &row.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("verification not found or expired")
		}
		return nil, err
	}
	verification := toVerification(row)
	return &verification, nil
}

// UpdateCode persists the code a provider-approved verification used, so later
// checks of the same flow match through the normal path.
func (r Repository) UpdateCode(ctx context.Context, id, code string) error {
	_, err := r.db.Exec(ctx, queryUpdateVerificationCode, id, code)
	return err
}

// RecordApplicationBinding stores the application and provider that delivered a
// code. Both values are nullable; empty strings map to NULL.
func (r Repository) RecordApplicationBinding(ctx context.Context, id, applicationID, providerID string) error {
	_, err := r.db.Exec(ctx, queryRecordVerificationBinding, id, applicationID, providerID)
	return err
}

// ApplicationBinding returns the application/provider recorded for a
// verification. A row without a binding yields empty strings.
func (r Repository) ApplicationBinding(ctx context.Context, id string) (string, string, error) {
	var applicationID, providerID string
	err := r.db.QueryRow(ctx, queryGetVerificationApplicationBinding, id).Scan(&applicationID, &providerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", apperrors.NewNotFound("verification not found")
		}
		return "", "", err
	}
	return applicationID, providerID, nil
}

var _ domain.Repository = Repository{}
