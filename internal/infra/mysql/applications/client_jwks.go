package applications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

const (
	btClientJWKs = "`app_client_jwks`"

	queryListActiveClientJWKs = `
SELECT ` + clientJWKColumns + `
FROM ` + btClientJWKs + `
WHERE organization_id = ? AND application_id = ? AND status = 'active'
ORDER BY created_at ASC, id ASC`

	queryRetireActiveClientJWKs = `
UPDATE ` + btClientJWKs + `
SET status = 'retired',
    retired_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE organization_id = ? AND application_id = ? AND status = 'active'`

	// MySQL 没有 RLS，organization_id 必须是查询条件的一部分；ON DUPLICATE KEY
	// UPDATE 也只在同一 application_id（因此同一租户）内命中。
	queryUpsertClientJWK = `
INSERT INTO ` + btClientJWKs + ` (id, application_id, organization_id, kid, alg, jwk, status, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?, 0)
ON DUPLICATE KEY UPDATE
    organization_id = VALUES(organization_id),
    alg = VALUES(alg),
    jwk = VALUES(jwk),
    status = 'active',
    retired_at = NULL,
    updated_at = VALUES(updated_at),
    revision = revision + 1`

	clientJWKColumns = `
id, application_id, organization_id, kid, alg, jwk, status,
retired_at, created_at, updated_at, revision`
)

// ListActiveClientJWKs 返回某应用登记为 active 的公钥（MySQL 镜像，受 OPEN-API-37 约束）。
func (r Repository) ListActiveClientJWKs(ctx context.Context, organizationID, applicationID string) ([]*domain.ClientJWK, error) {
	rows, err := r.db.QueryContext(ctx, queryListActiveClientJWKs, organizationID, applicationID)
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

// ReplaceClientJWKs 在同一事务内先退役所有 active 行，再 upsert 新集合。
func (r Repository) ReplaceClientJWKs(ctx context.Context, organizationID, applicationID string, keys []*domain.ClientJWK, replacedAt time.Time) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryRetireActiveClientJWKs, replacedAt, replacedAt, organizationID, applicationID); err != nil {
			return err
		}
		for _, key := range keys {
			if key == nil {
				continue
			}
			if _, err := tx.ExecContext(ctx, queryUpsertClientJWK,
				key.ID, applicationID, organizationID, key.Kid, key.Alg, key.JWK, replacedAt, replacedAt,
			); err != nil {
				return err
			}
			key.ApplicationID = applicationID
			key.OrganizationID = organizationID
			key.Status = domain.ClientJWKStatusActive
			key.RetiredAt = nil
			key.CreatedAt = replacedAt
			key.UpdatedAt = replacedAt
		}
		return nil
	})
}
