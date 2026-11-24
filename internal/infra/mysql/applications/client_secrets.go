package applications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

const (
	btClientSecrets = "`app_client_secrets`"

	queryInsertClientSecret = `
INSERT INTO ` + btClientSecrets + ` (id, application_id, organization_id, secret_prefix, secret_hash, status, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`

	queryListClientSecrets = `
SELECT ` + clientSecretColumns + `
FROM ` + btClientSecrets + `
WHERE organization_id = ? AND application_id = ?
ORDER BY created_at DESC, id DESC`

	queryListUsableClientSecrets = `
SELECT ` + clientSecretColumns + `
FROM ` + btClientSecrets + `
WHERE organization_id = ? AND application_id = ?
  AND (status = 'active' OR (status = 'grace' AND grace_until > ?))
ORDER BY created_at DESC, id DESC`

	queryGraceActiveClientSecrets = `
UPDATE ` + btClientSecrets + `
SET status = 'grace',
    rotated_at = ?,
    grace_until = ?,
    updated_at = ?,
    revision = revision + 1
WHERE organization_id = ? AND application_id = ? AND status = 'active'`

	queryRetireClientSecrets = `
UPDATE ` + btClientSecrets + `
SET status = 'retired',
    retired_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE organization_id = ? AND application_id = ? AND status IN ('active', 'grace')`

	clientSecretColumns = `
id, application_id, organization_id, secret_prefix, secret_hash, status,
rotated_at, grace_until, retired_at, created_at, updated_at, revision`
)

// CreateClientSecret 插入一条 secret 行（MySQL 镜像，受 OPEN-API-37 约束未做真库验证）。
// organization_id 显式写入：MySQL 没有 RLS，租户隔离只能靠查询条件。
func (r Repository) CreateClientSecret(ctx context.Context, organizationID string, secret *domain.ClientSecret) error {
	status := secret.Status
	if status == "" {
		status = domain.ClientSecretStatusActive
	}
	now := time.Now().UTC()
	if _, err := r.db.ExecContext(ctx, queryInsertClientSecret,
		secret.ID, secret.ApplicationID, organizationID, secret.SecretPrefix, secret.SecretHash, string(status), now, now,
	); err != nil {
		return err
	}
	secret.CreatedAt = now
	secret.UpdatedAt = now
	secret.Revision = 0
	return nil
}

// ListClientSecrets 返回某应用的全部 secret 行（管理读，仅用于掩码展示）。
func (r Repository) ListClientSecrets(ctx context.Context, organizationID, applicationID string) ([]*domain.ClientSecret, error) {
	return r.queryClientSecrets(ctx, queryListClientSecrets, organizationID, applicationID)
}

// ListUsableClientSecrets 返回 active 与仍在 grace 窗口内的行。
func (r Repository) ListUsableClientSecrets(ctx context.Context, organizationID, applicationID string, now time.Time) ([]*domain.ClientSecret, error) {
	return r.queryClientSecrets(ctx, queryListUsableClientSecrets, organizationID, applicationID, now)
}

// RotateClientSecret 在同一事务内把当前 active 行降级为 grace 并插入新 active 行。
func (r Repository) RotateClientSecret(ctx context.Context, organizationID, applicationID string, next *domain.ClientSecret, rotatedAt, graceUntil time.Time) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryGraceActiveClientSecrets, rotatedAt, graceUntil, rotatedAt, organizationID, applicationID); err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, queryInsertClientSecret,
			next.ID, next.ApplicationID, organizationID, next.SecretPrefix, next.SecretHash, string(domain.ClientSecretStatusActive), now, now,
		); err != nil {
			return err
		}
		next.CreatedAt = now
		next.UpdatedAt = now
		next.Revision = 0
		return nil
	})
}

// RetireClientSecrets 把 active/grace 行标记为 retired，保留行用于审计与回滚。
func (r Repository) RetireClientSecrets(ctx context.Context, organizationID, applicationID string, retiredAt time.Time) error {
	_, err := r.db.ExecContext(ctx, queryRetireClientSecrets, retiredAt, retiredAt, organizationID, applicationID)
	return err
}

func (r Repository) queryClientSecrets(ctx context.Context, query string, args ...any) ([]*domain.ClientSecret, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
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
