package lockoutattempts

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// Repository 是 MySQL 8.0+ 实现的 lockoutattempts 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// attemptRow 是 lockout_attempts 表扫描目标。
type attemptRow struct {
	ID              string
	UserID          string
	Attempts        int
	LockedUntil     sql.NullTime
	WindowStartedAt sql.NullTime
	LockoutLevel    int
	CreatedAt       sql.NullTime
	UpdatedAt       sql.NullTime
	Revision        sql.NullInt64
}

// GetByUserID 按 user_id 查 Attempt。找不到返回 nil, nil（不是 error）。
func (r Repository) GetByUserID(ctx context.Context, userID string) (*domain.Attempt, error) {
	var row attemptRow
	err := r.db.QueryRowContext(ctx, queryGetByUserID, userID).Scan(
		&row.ID, &row.UserID, &row.Attempts, &row.LockedUntil,
		&row.WindowStartedAt, &row.LockoutLevel, &row.CreatedAt, &row.UpdatedAt,
		&row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 没有 lockout 记录是正常情况
		}
		return nil, err
	}
	return toDomain(row), nil
}

// Create 插入 Attempt。
func (r Repository) Create(ctx context.Context, attempt *domain.Attempt) error {
	revision := attempt.Revision
	if revision <= 0 {
		revision = 1
	}
	_, err := r.db.ExecContext(ctx, queryCreate,
		attempt.ID, attempt.UserID, attempt.Attempts, attempt.LockedUntil,
		attempt.WindowStartedAt, attempt.LockoutLevel, attempt.CreatedAt, attempt.UpdatedAt, revision,
	)
	if err == nil {
		attempt.Revision = revision
	}
	return err
}

// Update 更新 Attempt（按 user_id）。
func (r Repository) Update(ctx context.Context, attempt *domain.Attempt) error {
	revision := attempt.Revision
	if revision <= 0 {
		revision = 1
	}
	result, err := r.db.ExecContext(ctx, queryUpdate,
		attempt.Attempts, attempt.LockedUntil, attempt.WindowStartedAt,
		attempt.LockoutLevel, attempt.UpdatedAt,
		attempt.UserID, revision,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("lockout attempt revision conflict")
	}
	attempt.Revision = revision + 1
	return nil
}

// RecordFailure 原子地记录一次登录失败。
// PG 版是 INSERT ... ON CONFLICT DO UPDATE RETURNING。
// MySQL 版拆成 UPSERT + SELECT 两步。
func (r Repository) RecordFailure(ctx context.Context, userID string, maxAttempts, lockDurationMinutes int, now time.Time) (*domain.Attempt, error) {
	if beginner, ok := r.db.(mysqlshared.Beginner); ok {
		tx, err := beginner.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer func() { _ = tx.Rollback() }()

		attempt, err := (Repository{db: tx}).recordFailureInTransaction(ctx, userID, maxAttempts, lockDurationMinutes, now)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return attempt, nil
	}

	return r.recordFailureInTransaction(ctx, userID, maxAttempts, lockDurationMinutes, now)
}

func (r Repository) recordFailureInTransaction(ctx context.Context, userID string, maxAttempts, lockDurationMinutes int, now time.Time) (*domain.Attempt, error) {
	id, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return nil, err
	}

	if _, err := r.db.ExecContext(ctx, queryEnsureAttempt, id, userID, now, now, now); err != nil {
		return nil, err
	}

	var row attemptRow
	if err := r.db.QueryRowContext(ctx, queryLockByUserID, userID).Scan(
		&row.ID, &row.UserID, &row.Attempts, &row.LockedUntil, &row.WindowStartedAt,
		&row.LockoutLevel, &row.CreatedAt, &row.UpdatedAt,
		&row.Revision,
	); err != nil {
		return nil, err
	}
	attempt := toDomain(row)
	attempt.RecordFailure(now, maxAttempts, time.Duration(lockDurationMinutes)*time.Minute)
	attempt.UpdatedAt = now
	if err := r.Update(ctx, attempt); err != nil {
		return nil, err
	}
	return attempt, nil
}

// DeleteByUserID 删除某个 user 的 lockout 记录。
func (r Repository) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteByUserID, userID)
	return err
}

// DeleteExpired 删除已过期的 lockout 记录。
func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// toDomain 把 MySQL row 转成 domain.Attempt。
func toDomain(row attemptRow) *domain.Attempt {
	a := &domain.Attempt{
		ID:           row.ID,
		UserID:       row.UserID,
		Attempts:     row.Attempts,
		LockoutLevel: row.LockoutLevel,
	}
	if row.LockedUntil.Valid {
		t := row.LockedUntil.Time
		a.LockedUntil = &t
	}
	if row.CreatedAt.Valid {
		a.CreatedAt = row.CreatedAt.Time
	}
	if row.WindowStartedAt.Valid {
		a.WindowStartedAt = row.WindowStartedAt.Time
	}
	if row.UpdatedAt.Valid {
		a.UpdatedAt = row.UpdatedAt.Time
	}
	if row.Revision.Valid {
		a.Revision = row.Revision.Int64
	}
	return a
}

// 编译期接口断言
var _ domain.Repository = Repository{}
