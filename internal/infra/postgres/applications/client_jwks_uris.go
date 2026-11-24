package applications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// SetClientJWKSURI upserts the remote JWKS location for one application. The
// organization predicate keeps the write tenant-scoped.
func (r Repository) SetClientJWKSURI(ctx context.Context, organizationID, applicationID, jwksURI string, now time.Time) error {
	_, err := r.db.Exec(ctx, queryUpsertClientJWKSURI, applicationID, organizationID, jwksURI, now)
	return err
}

// GetClientJWKSURI returns the stored remote JWKS location, or NotFound.
func (r Repository) GetClientJWKSURI(ctx context.Context, organizationID, applicationID string) (*domain.ClientJWKSURI, error) {
	var uri domain.ClientJWKSURI
	err := r.db.QueryRow(ctx, queryGetClientJWKSURI, organizationID, applicationID).Scan(
		&uri.ApplicationID, &uri.OrganizationID, &uri.JWKSURI, &uri.CreatedAt, &uri.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("client jwks_uri not found")
		}
		return nil, err
	}
	return &uri, nil
}

// DeleteClientJWKSURI removes the remote JWKS location.
func (r Repository) DeleteClientJWKSURI(ctx context.Context, organizationID, applicationID string) error {
	_, err := r.db.Exec(ctx, queryDeleteClientJWKSURI, organizationID, applicationID)
	return err
}
