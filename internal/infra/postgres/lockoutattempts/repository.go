package lockoutattempts

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByUserID(ctx context.Context, userID string) (*domain.Attempt, error) {
	var row attemptRow
	err := r.db.QueryRow(ctx, queryGetByUserID, userID).Scan(
		&row.ID, &row.UserID, &row.Attempts, &row.LockedUntil, &row.WindowStartedAt,
		&row.LockoutLevel, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found is not an error - means no lockout record
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r Repository) Create(ctx context.Context, attempt *domain.Attempt) error {
	revision := attempt.Revision
	if revision <= 0 {
		revision = 1
	}
	_, err := r.db.Exec(ctx, queryCreate,
		attempt.ID, attempt.UserID, attempt.Attempts, attempt.LockedUntil,
		attempt.WindowStartedAt, attempt.LockoutLevel, attempt.CreatedAt, attempt.UpdatedAt, revision,
	)
	if err == nil {
		attempt.Revision = revision
	}
	return err
}

func (r Repository) Update(ctx context.Context, attempt *domain.Attempt) error {
	revision := attempt.Revision
	if revision <= 0 {
		revision = 1
	}
	tag, err := r.db.Exec(ctx, queryUpdate,
		attempt.UserID, attempt.Attempts, attempt.LockedUntil, attempt.WindowStartedAt,
		attempt.LockoutLevel, attempt.UpdatedAt, revision,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewConflict("lockout attempt revision conflict")
	}
	attempt.Revision = revision + 1
	return nil
}

// RecordFailure is the concurrency-safe replacement for the Get + mutate + Update
// sequence: one statement, so N parallel failed logins produce N increments
// instead of N writes of the same value.
func (r Repository) RecordFailure(ctx context.Context, userID string, maxAttempts, lockDurationMinutes int, now time.Time) (*domain.Attempt, error) {
	id, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return nil, err
	}

	attempt := &domain.Attempt{
		ID:        id,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.db.QueryRow(ctx, queryRecordFailure, id, userID, maxAttempts, now, lockDurationMinutes).
		Scan(&attempt.Attempts, &attempt.LockedUntil, &attempt.WindowStartedAt, &attempt.LockoutLevel,
			&attempt.CreatedAt, &attempt.UpdatedAt, &attempt.Revision); err != nil {
		return nil, err
	}
	return attempt, nil
}

func (r Repository) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, queryDeleteByUserID, userID)
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
