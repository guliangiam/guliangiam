package deviceauthcodes

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 device_auth_codes 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// deviceAuthCodeRow 是 device_auth_codes 表扫描目标。
type deviceAuthCodeRow struct {
	ID              string
	DeviceCode      string
	UserCode        string
	ClientID        string
	Scope           string
	ExpiresAt       time.Time
	PollIntervalSec int
	Verified        bool
	VerifiedAt      *time.Time
	UserID          string
	Username        string
	OrganizationID  string
	LastPolledAt    *time.Time
	SlowDownCount   int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Revision        int64
}

// scanDeviceAuthCodeRow 扫描一行到 deviceAuthCodeRow。
func scanDeviceAuthCodeRow(scanner interface{ Scan(...any) error }) (*domain.DeviceAuthCode, error) {
	var row deviceAuthCodeRow
	err := scanner.Scan(
		&row.ID, &row.DeviceCode, &row.UserCode, &row.ClientID, &row.Scope,
		&row.ExpiresAt, &row.PollIntervalSec, &row.Verified, &row.VerifiedAt,
		&row.UserID, &row.Username, &row.OrganizationID,
		&row.LastPolledAt, &row.SlowDownCount,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("device auth code not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

// GetByDeviceCode 按 device_code 查 DeviceAuthCode。
func (r Repository) GetByDeviceCode(ctx context.Context, deviceCode string) (*domain.DeviceAuthCode, error) {
	return scanDeviceAuthCodeRow(r.db.QueryRowContext(ctx, queryGetByDeviceCode, deviceCode))
}

// GetByUserCode 按 user_code 查 DeviceAuthCode。
func (r Repository) GetByUserCode(ctx context.Context, userCode string) (*domain.DeviceAuthCode, error) {
	return scanDeviceAuthCodeRow(r.db.QueryRowContext(ctx, queryGetByUserCode, userCode))
}

// Create 插入 DeviceAuthCode。
func (r Repository) Create(ctx context.Context, code *domain.DeviceAuthCode) error {
	_, err := r.db.ExecContext(ctx, queryCreate,
		code.ID, code.DeviceCode, code.UserCode, code.ClientID, code.Scope,
		code.ExpiresAt, code.PollIntervalSec, code.Verified, code.VerifiedAt,
		code.UserID, code.Username, code.OrganizationID,
		code.LastPolledAt, code.SlowDownCount,
		code.CreatedAt, code.UpdatedAt, code.Revision,
	)
	return err
}

// RecordPoll 原子记录一次 poll 并返回新的 slow_down counter。
// PG UPDATE RETURNING → MySQL UPDATE + SELECT 两步。
func (r Repository) RecordPoll(ctx context.Context, deviceCode string, polledAt time.Time) (int, error) {
	result, err := r.db.ExecContext(ctx, queryRecordPoll, polledAt, polledAt, deviceCode)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		return 0, apperrors.NewNotFound("device auth code not found")
	}

	var count int
	if err := r.db.QueryRowContext(ctx, querySelectSlowDownAfterPoll, deviceCode).Scan(&count); err != nil {
		if err == sql.ErrNoRows {
			return 0, apperrors.NewNotFound("device auth code not found")
		}
		return 0, err
	}
	return count, nil
}

// ResetSlowDown 清除 slow_down counter。
func (r Repository) ResetSlowDown(ctx context.Context, deviceCode string, polledAt time.Time) error {
	_, err := r.db.ExecContext(ctx, queryResetSlowDown, polledAt, polledAt, deviceCode)
	return err
}

// UpdateVerification 更新验证状态（乐观锁）。
func (r Repository) UpdateVerification(ctx context.Context, code *domain.DeviceAuthCode) error {
	result, err := r.db.ExecContext(ctx, queryUpdateVerification,
		code.Verified, code.VerifiedAt, code.UserID, code.Username,
		code.OrganizationID, code.UpdatedAt,
		code.DeviceCode, code.Revision,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("device auth code was modified by another request")
	}
	return nil
}

// DeleteByDeviceCode 按 device_code 删除。
func (r Repository) DeleteByDeviceCode(ctx context.Context, deviceCode string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteByDeviceCode, deviceCode)
	return err
}

// DeleteByUserCode 按 user_code 删除。
func (r Repository) DeleteByUserCode(ctx context.Context, userCode string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteByUserCode, userCode)
	return err
}

// DeleteExpired 删除所有已过期 device auth codes。
func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// toDomain 把 MySQL row 转成 domain.DeviceAuthCode。
func toDomain(row deviceAuthCodeRow) *domain.DeviceAuthCode {
	return &domain.DeviceAuthCode{
		ID:              row.ID,
		DeviceCode:      row.DeviceCode,
		UserCode:        row.UserCode,
		ClientID:        row.ClientID,
		Scope:           row.Scope,
		ExpiresAt:       row.ExpiresAt,
		PollIntervalSec: row.PollIntervalSec,
		Verified:        row.Verified,
		VerifiedAt:      row.VerifiedAt,
		UserID:          row.UserID,
		Username:        row.Username,
		OrganizationID:  row.OrganizationID,
		LastPolledAt:    row.LastPolledAt,
		SlowDownCount:   row.SlowDownCount,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Revision:        row.Revision,
	}
}

// 编译期接口断言
var _ domain.Repository = Repository{}
