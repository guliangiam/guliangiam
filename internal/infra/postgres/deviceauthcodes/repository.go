package deviceauthcodes

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// Repository implements domain.Repository with PostgreSQL.
type Repository struct {
	db shared.DBTX
}

// NewRepository creates a new device auth code repository.
func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByDeviceCode(ctx context.Context, deviceCode string) (*domain.DeviceAuthCode, error) {
	var row deviceAuthCodeRow
	err := r.db.QueryRow(ctx, queryGetByDeviceCode, deviceCode).Scan(
		&row.ID, &row.DeviceCode, &row.UserCode, &row.ClientID, &row.Scope,
		&row.ExpiresAt, &row.PollIntervalSec, &row.Verified, &row.VerifiedAt,
		&row.UserID, &row.Username, &row.OrganizationID,
		&row.LastPolledAt, &row.SlowDownCount,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("device auth code not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r Repository) GetByUserCode(ctx context.Context, userCode string) (*domain.DeviceAuthCode, error) {
	var row deviceAuthCodeRow
	err := r.db.QueryRow(ctx, queryGetByUserCode, userCode).Scan(
		&row.ID, &row.DeviceCode, &row.UserCode, &row.ClientID, &row.Scope,
		&row.ExpiresAt, &row.PollIntervalSec, &row.Verified, &row.VerifiedAt,
		&row.UserID, &row.Username, &row.OrganizationID,
		&row.LastPolledAt, &row.SlowDownCount,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("device auth code not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r Repository) Create(ctx context.Context, code *domain.DeviceAuthCode) error {
	_, err := r.db.Exec(ctx, queryCreate,
		code.ID, code.DeviceCode, code.UserCode, code.ClientID, code.Scope,
		code.ExpiresAt, code.PollIntervalSec, code.Verified, code.VerifiedAt,
		code.UserID, code.Username, code.OrganizationID,
		code.LastPolledAt, code.SlowDownCount,
		code.CreatedAt, code.UpdatedAt, code.Revision,
	)
	return err
}

// RecordPoll stamps the poll and returns the incremented slow_down counter.
func (r Repository) RecordPoll(ctx context.Context, deviceCode string, polledAt time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, queryRecordPoll, deviceCode, polledAt).Scan(&count)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, apperrors.NewNotFound("device auth code not found")
		}
		return 0, err
	}
	return count, nil
}

// ResetSlowDown records a compliant poll and clears the violation counter.
func (r Repository) ResetSlowDown(ctx context.Context, deviceCode string, polledAt time.Time) error {
	_, err := r.db.Exec(ctx, queryResetSlowDown, deviceCode, polledAt)
	return err
}

func (r Repository) UpdateVerification(ctx context.Context, code *domain.DeviceAuthCode) error {
	tag, err := r.db.Exec(ctx, queryUpdateVerification,
		code.DeviceCode, code.Revision,
		code.Verified, code.VerifiedAt, code.UserID, code.Username,
		code.OrganizationID, code.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewConflict("device auth code was modified by another request")
	}
	return nil
}

func (r Repository) DeleteByDeviceCode(ctx context.Context, deviceCode string) error {
	_, err := r.db.Exec(ctx, queryDeleteByDeviceCode, deviceCode)
	return err
}

func (r Repository) DeleteByUserCode(ctx context.Context, userCode string) error {
	_, err := r.db.Exec(ctx, queryDeleteByUserCode, userCode)
	return err
}

func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

var _ domain.Repository = Repository{}
