package applications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

const (
	queryListActiveClientJWKs = `
SELECT ` + clientJWKColumns + `
FROM guliangiam.app_client_jwks
WHERE organization_id = $1 AND application_id = $2 AND status = 'active'
ORDER BY created_at ASC, id ASC`

	queryRetireActiveClientJWKs = `
UPDATE guliangiam.app_client_jwks
SET status = 'retired',
    retired_at = $3,
    updated_at = $3,
    revision = revision + 1
WHERE organization_id = $1 AND application_id = $2 AND status = 'active'`

	queryUpsertClientJWK = `
INSERT INTO guliangiam.app_client_jwks (id, application_id, organization_id, kid, alg, jwk, status)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'active')
ON CONFLICT (application_id, kid) DO UPDATE
SET organization_id = EXCLUDED.organization_id,
    alg = EXCLUDED.alg,
    jwk = EXCLUDED.jwk,
    status = 'active',
    retired_at = NULL,
    updated_at = $7,
    revision = guliangiam.app_client_jwks.revision + 1
WHERE guliangiam.app_client_jwks.organization_id = EXCLUDED.organization_id
RETURNING created_at, updated_at, revision`

	clientJWKColumns = `
id, application_id, organization_id, kid, alg, jwk, status,
retired_at, created_at, updated_at, revision`
)

// ListActiveClientJWKs returns the active public keys for a client. The
// organization predicate is explicit and comes from the authenticated
// application row, never from the presented assertion.
func (r Repository) ListActiveClientJWKs(ctx context.Context, organizationID, applicationID string) ([]*domain.ClientJWK, error) {
	rows, err := r.db.Query(ctx, queryListActiveClientJWKs, organizationID, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]*domain.ClientJWK, 0)
	for rows.Next() {
		var key domain.ClientJWK
		var status string
		var jwkJSON []byte
		if err := rows.Scan(
			&key.ID, &key.ApplicationID, &key.OrganizationID, &key.Kid, &key.Alg, &jwkJSON, &status,
			&key.RetiredAt, &key.CreatedAt, &key.UpdatedAt, &key.Revision,
		); err != nil {
			return nil, err
		}
		key.JWK = string(jwkJSON)
		key.Status = domain.ClientJWKStatus(status)
		keys = append(keys, &key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

// ReplaceClientJWKs retires every active row and then upserts the supplied set
// in one transaction. The (application_id, kid) unique index lets a re-used kid
// reactivate its existing row instead of violating the constraint.
func (r Repository) ReplaceClientJWKs(ctx context.Context, organizationID, applicationID string, keys []*domain.ClientJWK, replacedAt time.Time) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		if _, err := tx.Exec(ctx, queryRetireActiveClientJWKs, organizationID, applicationID, replacedAt); err != nil {
			return err
		}
		for _, key := range keys {
			if key == nil {
				continue
			}
			var createdAt, updatedAt time.Time
			var revision int64
			if err := tx.QueryRow(ctx, queryUpsertClientJWK,
				key.ID, applicationID, organizationID, key.Kid, key.Alg, key.JWK, replacedAt,
			).Scan(&createdAt, &updatedAt, &revision); err != nil {
				return err
			}
			key.ApplicationID = applicationID
			key.OrganizationID = organizationID
			key.Status = domain.ClientJWKStatusActive
			key.RetiredAt = nil
			key.CreatedAt = createdAt
			key.UpdatedAt = updatedAt
			key.Revision = revision
		}
		return nil
	})
}
