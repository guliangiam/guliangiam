package faceidsessions

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 faceidsessions 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// sessionRow 是 face_id_sessions 表扫描目标。
type sessionRow struct {
	ID           string
	SessionID    string
	Challenge    string
	UserID       string
	ExpiresAt    sql.NullTime
	Verified     bool
	LivenessOK   bool
	AttemptCount int
	CreatedAt    sql.NullTime
	UpdatedAt    sql.NullTime
}

// GetBySessionID 按 session_id 查 Session。
func (r Repository) GetBySessionID(ctx context.Context, sessionID string) (*domain.Session, error) {
	var row sessionRow
	err := r.db.QueryRowContext(ctx, queryGetBySessionID, sessionID).Scan(
		&row.ID, &row.SessionID, &row.Challenge, &row.UserID,
		&row.ExpiresAt, &row.Verified, &row.LivenessOK, &row.AttemptCount, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("face id session not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

// Create 插入 Session。
func (r Repository) Create(ctx context.Context, session *domain.Session) error {
	_, err := r.db.ExecContext(ctx, queryCreate,
		session.ID, session.SessionID, session.Challenge, session.UserID,
		session.ExpiresAt, session.Verified, session.LivenessOK,
		session.CreatedAt, session.UpdatedAt,
	)
	return err
}

// Update 写入 verification 结果。
// WHERE verified=false → CAS 语义：只有仍未验证的 session 才能被标记为已验证。
// affected rows == 0 说明已被其他请求抢先处理。
func (r Repository) Update(ctx context.Context, session *domain.Session) error {
	result, err := r.db.ExecContext(ctx, queryUpdate,
		session.Verified, session.LivenessOK, session.AttemptCount, session.UpdatedAt, session.SessionID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("face id session has already been verified")
	}
	return nil
}

// ConsumeVerified 原子删除已验证的 session（发 token 前的 gate）。
// WHERE verified=true → 确保只消费已验证的 session。
// affected rows == 0 说明已被其他请求消费。
func (r Repository) ConsumeVerified(ctx context.Context, sessionID string) error {
	result, err := r.db.ExecContext(ctx, queryConsumeVerified, sessionID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("face id session has already been consumed")
	}
	return nil
}

// DeleteBySessionID 删除 session（不管状态）。
func (r Repository) DeleteBySessionID(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteBySessionID, sessionID)
	return err
}

// DeleteExpired 删除已过期的 session。
func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// toDomain 把 MySQL row 转成 domain.Session。
func toDomain(row sessionRow) *domain.Session {
	s := &domain.Session{
		ID:           row.ID,
		SessionID:    row.SessionID,
		Challenge:    row.Challenge,
		UserID:       row.UserID,
		Verified:     row.Verified,
		LivenessOK:   row.LivenessOK,
		AttemptCount: row.AttemptCount,
	}
	if row.ExpiresAt.Valid {
		s.ExpiresAt = row.ExpiresAt.Time
	}
	if row.CreatedAt.Valid {
		s.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		s.UpdatedAt = row.UpdatedAt.Time
	}
	return s
}

// 编译期接口断言
var _ domain.Repository = Repository{}
