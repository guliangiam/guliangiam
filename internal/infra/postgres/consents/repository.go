package consents

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/consents"
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

func (r Repository) GetByUserAndApplication(ctx context.Context, userID, applicationID string) (*domain.Consent, error) {
	var row consentRow
	err := r.db.QueryRow(ctx, queryGetConsentByUserAndApplication, userID, applicationID).Scan(
		&row.ID,
		&row.UserID,
		&row.Username,
		&row.OrganizationID,
		&row.OrganizationSlug,
		&row.ApplicationID,
		&row.ApplicationSlug,
		&row.GrantedScopes,
		&row.CreatedAt,
		&row.UpdatedAt,
		&row.RevokedAt,
		&row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("consent not found")
		}
		return nil, err
	}

	return toConsent(row)
}

func (r Repository) CountActiveByApplication(ctx context.Context, applicationID string) (int64, error) {
	var count int64
	if err := r.db.QueryRow(ctx, queryCountActiveConsentsByApplication, applicationID).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (r Repository) Upsert(ctx context.Context, consent *domain.Consent) error {
	grantedScopes, err := shared.MarshalJSON(consent.GrantedScopes)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryUpsertConsent,
		consent.ID,
		consent.UserID,
		consent.Username,
		consent.OrganizationID,
		consent.OrganizationSlug,
		consent.ApplicationID,
		consent.ApplicationSlug,
		grantedScopes,
		consent.CreatedAt,
		consent.UpdatedAt,
		consent.RevokedAt,
		consent.Revision,
	)
	return err
}

func (r Repository) Revoke(ctx context.Context, userID, applicationID string) error {
	result, err := r.db.Exec(ctx, queryRevokeConsent, userID, applicationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("consent not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
