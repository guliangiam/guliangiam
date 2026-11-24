package qrcodesessions

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 qrcodesessions 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByCodeID 按 code_id 查 Session。
func (r Repository) GetByCodeID(ctx context.Context, codeID string) (*domain.Session, error) {
	var s domain.Session
	err := r.db.QueryRowContext(ctx, queryGetByCodeID, codeID).Scan(
		&s.ID, &s.CodeID, &s.Code, &s.OrganizationID, &s.ApplicationID,
		&s.ExpiresAt, &s.Status, &s.UserID, &s.Username,
		&s.IPAddress, &s.UserAgent, &s.CreatedAt, &s.UpdatedAt, &s.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("qr code session not found")
		}
		return nil, err
	}
	return &s, nil
}

// GetByCode 按二维码载荷 code 查 Session（扫码/确认端点使用）。
func (r Repository) GetByCode(ctx context.Context, code string) (*domain.Session, error) {
	var s domain.Session
	err := r.db.QueryRowContext(ctx, queryGetByCode, code).Scan(
		&s.ID, &s.CodeID, &s.Code, &s.OrganizationID, &s.ApplicationID,
		&s.ExpiresAt, &s.Status, &s.UserID, &s.Username,
		&s.IPAddress, &s.UserAgent, &s.CreatedAt, &s.UpdatedAt, &s.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("qr code session not found")
		}
		return nil, err
	}
	return &s, nil
}

// Create 插入 Session。
func (r Repository) Create(ctx context.Context, session *domain.Session) error {
	_, err := r.db.ExecContext(ctx, queryCreate,
		session.ID, session.CodeID, session.Code, session.OrganizationID,
		session.ApplicationID, session.ExpiresAt, session.Status,
		session.UserID, session.Username, session.IPAddress, session.UserAgent,
		session.CreatedAt, session.UpdatedAt, session.Revision,
	)
	return err
}

// UpdateStatus 按 code_id + revision 乐观锁更新状态。
func (r Repository) UpdateStatus(ctx context.Context, session *domain.Session) error {
	res, err := r.db.ExecContext(ctx, queryUpdateStatus,
		session.Status, session.UserID, session.Username,
		session.IPAddress, session.UserAgent, session.UpdatedAt,
		session.CodeID, session.Revision,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("qr code session was modified by another request")
	}
	return nil
}

// DeleteByCodeID 按 code_id 删除 Session。
func (r Repository) DeleteByCodeID(ctx context.Context, codeID string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteByCodeID, codeID)
	return err
}

// DeleteExpired 删除所有过期 Session，返回受影响行数。
func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// 编译期接口断言
var _ domain.Repository = Repository{}
