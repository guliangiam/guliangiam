package applications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

const (
	queryInsertClientSecret = `
INSERT INTO guliangiam.app_client_secrets (id, application_id, organization_id, secret_prefix, secret_hash, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING created_at, updated_at, revision`

	queryListClientSecrets = `
SELECT ` + clientSecretColumns + `
FROM guliangiam.app_client_secrets
WHERE organization_id = $1 AND application_id = $2
ORDER BY created_at DESC, id DESC`

	queryListUsableClientSecrets = `
SELECT ` + clientSecretColumns + `
FROM guliangiam.app_client_secrets
WHERE organization_id = $1 AND application_id = $2
  AND (status = 'active' OR (status = 'grace' AND grace_until > $3))
ORDER BY created_at DESC, id DESC`

	queryGraceActiveClientSecrets = `
UPDATE guliangiam.app_client_secrets
SET status = 'grace',
    rotated_at = $3,
    grace_until = $4,
    updated_at = $3,
    revision = revision + 1
WHERE organization_id = $1 AND application_id = $2 AND status = 'active'`

	queryRetireClientSecrets = `
UPDATE guliangiam.app_client_secrets
SET status = 'retired',
    retired_at = $3,
    updated_at = $3,
    revision = revision + 1
WHERE organization_id = $1 AND application_id = $2 AND status IN ('active', 'grace')`

	clientSecretColumns = `
id, application_id, organization_id, secret_prefix, secret_hash, status,
rotated_at, grace_until, retired_at, created_at, updated_at, revision`
)

// CreateClientSecret persists a new secret row. The plaintext never reaches
// this layer: callers pass the argon2id hash plus the non-secret prefix. The
// organization_id is written from the trusted tenant, so every later read can
// filter on it explicitly instead of depending on RLS alone.
func (r Repository) CreateClientSecret(ctx context.Context, organizationID string, secret *domain.ClientSecret) error {
	status := secret.Status
	if status == "" {
		status = domain.ClientSecretStatusActive
	}
	return r.db.QueryRow(ctx, queryInsertClientSecret,
		secret.ID, secret.ApplicationID, organizationID, secret.SecretPrefix, secret.SecretHash, string(status),
	).Scan(&secret.CreatedAt, &secret.UpdatedAt, &secret.Revision)
}

// ListClientSecrets returns all rows for management reads (masked display).
func (r Repository) ListClientSecrets(ctx context.Context, organizationID, applicationID string) ([]*domain.ClientSecret, error) {
	return r.queryClientSecrets(ctx, queryListClientSecrets, organizationID, applicationID)
}

// ListUsableClientSecrets returns active plus still-in-grace rows at now.
func (r Repository) ListUsableClientSecrets(ctx context.Context, organizationID, applicationID string, now time.Time) ([]*domain.ClientSecret, error) {
	return r.queryClientSecrets(ctx, queryListUsableClientSecrets, organizationID, applicationID, now)
}

// RotateClientSecret moves the active row to grace and inserts next as active
// in one transaction. Old rows are retained for the grace window and audit.
func (r Repository) RotateClientSecret(ctx context.Context, organizationID, applicationID string, next *domain.ClientSecret, rotatedAt, graceUntil time.Time) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		if _, err := tx.Exec(ctx, queryGraceActiveClientSecrets, organizationID, applicationID, rotatedAt, graceUntil); err != nil {
			return err
		}
		return tx.QueryRow(ctx, queryInsertClientSecret,
			next.ID, next.ApplicationID, organizationID, next.SecretPrefix, next.SecretHash, string(domain.ClientSecretStatusActive),
		).Scan(&next.CreatedAt, &next.UpdatedAt, &next.Revision)
	})
}

// RetireClientSecrets makes every usable row permanently unusable while keeping
// the rows for audit and rollback.
func (r Repository) RetireClientSecrets(ctx context.Context, organizationID, applicationID string, retiredAt time.Time) error {
	_, err := r.db.Exec(ctx, queryRetireClientSecrets, organizationID, applicationID, retiredAt)
	return err
}

func (r Repository) queryClientSecrets(ctx context.Context, query string, args ...any) ([]*domain.ClientSecret, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	secrets := make([]*domain.ClientSecret, 0)
	for rows.Next() {
		var secret domain.ClientSecret
		var status string
		if err := rows.Scan(
			&secret.ID, &secret.ApplicationID, &secret.OrganizationID, &secret.SecretPrefix, &secret.SecretHash, &status,
			&secret.RotatedAt, &secret.GraceUntil, &secret.RetiredAt,
			&secret.CreatedAt, &secret.UpdatedAt, &secret.Revision,
		); err != nil {
			return nil, err
		}
		secret.Status = domain.ClientSecretStatus(status)
		secrets = append(secrets, &secret)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return secrets, nil
}
