package trusteddevices

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) Create(ctx context.Context, d *domain.TrustedDevice) error {
	// 持久化边界硬校验：只接受 SHA-256 摘要。非摘要值直接 400，且不触达 SQL，
	// 任何绕过 app 层的调用方也无法把明文凭据落库。
	if err := domain.ValidateCredentialDigest(d.DeviceCredentialID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, queryInsertDevice,
		d.ID, d.OrganizationID, d.UserID, d.DeviceCredentialID, d.DevicePublicKey,
		d.UAHash, d.CreatedAt, d.LastSeenAt, d.ExpiresAt, d.RevokedAt, d.Revision, d.UpdatedAt)
	return err
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.TrustedDevice, error) {
	var d domain.TrustedDevice
	err := r.db.QueryRow(ctx, queryGetByID, id).Scan(
		&d.ID, &d.OrganizationID, &d.UserID, &d.DeviceCredentialID, &d.DevicePublicKey,
		&d.UAHash, &d.CreatedAt, &d.LastSeenAt, &d.ExpiresAt, &d.RevokedAt,
		&d.Revision, &d.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("trusted device not found")
		}
		return nil, err
	}
	return &d, nil
}

func (r Repository) ListByUser(ctx context.Context, organizationID, userID string) ([]*domain.TrustedDevice, error) {
	rows, err := r.db.Query(ctx, queryListByUser, organizationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*domain.TrustedDevice, 0)
	for rows.Next() {
		var d domain.TrustedDevice
		if err := rows.Scan(
			&d.ID, &d.OrganizationID, &d.UserID, &d.DeviceCredentialID, &d.DevicePublicKey,
			&d.UAHash, &d.CreatedAt, &d.LastSeenAt, &d.ExpiresAt, &d.RevokedAt,
			&d.Revision, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, &d)
	}
	return result, rows.Err()
}

func (r Repository) FindActiveByCredential(ctx context.Context, organizationID, userID, credentialID, uaHash string, now time.Time) (*domain.TrustedDevice, error) {
	var d domain.TrustedDevice
	err := r.db.QueryRow(ctx, queryFindActiveByCredential, organizationID, userID, credentialID, uaHash, now).Scan(
		&d.ID, &d.OrganizationID, &d.UserID, &d.DeviceCredentialID, &d.DevicePublicKey,
		&d.UAHash, &d.CreatedAt, &d.LastSeenAt, &d.ExpiresAt, &d.RevokedAt,
		&d.Revision, &d.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("no active trusted device for credential")
		}
		return nil, err
	}
	return &d, nil
}

func (r Repository) Revoke(ctx context.Context, organizationID, id string, now time.Time) error {
	_, err := r.db.Exec(ctx, queryRevoke, organizationID, id, now)
	return err
}

func (r Repository) TouchLastSeen(ctx context.Context, id string, now time.Time) error {
	_, err := r.db.Exec(ctx, queryTouchLastSeen, id, now)
	return err
}

func (r Repository) RevokeByCredential(ctx context.Context, organizationID, userID, credentialID string, now time.Time) error {
	_, err := r.db.Exec(ctx, queryRevokeByCredential, organizationID, userID, credentialID, now)
	return err
}

func (r Repository) RevokeByUser(ctx context.Context, organizationID, userID string, now time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, queryRevokeByUser, organizationID, userID, now)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteExpired physically deletes expired trusted devices in one bounded batch.
func (r Repository) DeleteExpired(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	tag, err := r.db.Exec(ctx, queryDeleteExpired, before, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RevokeLegacyPlaintext revokes every still-active row whose credential is not a
// 64 lowercase hex SHA-256 digest. Idempotent: later calls revoke zero rows.
func (r Repository) RevokeLegacyPlaintext(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, queryRevokeLegacyPlaintext, now)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
