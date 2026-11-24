package faceidsessions

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetBySessionID(ctx context.Context, sessionID string) (*domain.Session, error) {
	var row sessionRow
	err := r.db.QueryRow(ctx, queryGetBySessionID, sessionID).Scan(
		&row.ID, &row.SessionID, &row.Challenge, &row.UserID,
		&row.ExpiresAt, &row.Verified, &row.LivenessOK, &row.AttemptCount, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("face id session not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r Repository) Create(ctx context.Context, session *domain.Session) error {
	_, err := r.db.Exec(ctx, queryCreate,
		session.ID, session.SessionID, session.Challenge, session.UserID,
		session.ExpiresAt, session.Verified, session.LivenessOK, session.CreatedAt, session.UpdatedAt,
	)
	return err
}

// Update writes the verification outcome back. The `verified = false` guard makes
// the write a compare-and-set: only the request that still observes the session
// as unverified may flip it, so concurrent captures cannot both pass the
// "already verified" check in the service layer and both succeed.
func (r Repository) Update(ctx context.Context, session *domain.Session) error {
	result, err := r.db.Exec(ctx, queryUpdate,
		session.SessionID, session.Verified, session.LivenessOK, session.AttemptCount, session.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewConflict("face id session has already been verified")
	}
	return nil
}

// ConsumeVerified atomically deletes a *verified* session and reports whether
// this caller was the one that consumed it. CompleteFaceIDLogin issues a token
// pair per successful call, so the delete must be the gate rather than a
// separate read followed by a delete.
func (r Repository) ConsumeVerified(ctx context.Context, sessionID string) error {
	result, err := r.db.Exec(ctx, queryConsumeVerified, sessionID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewConflict("face id session has already been consumed")
	}
	return nil
}

func (r Repository) DeleteBySessionID(ctx context.Context, sessionID string) error {
	_, err := r.db.Exec(ctx, queryDeleteBySessionID, sessionID)
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
