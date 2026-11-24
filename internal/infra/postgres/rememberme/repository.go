package rememberme

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/rememberme"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) CreateFamily(ctx context.Context, f *domain.Family) error {
	_, err := r.db.Exec(ctx, queryInsertFamily,
		f.ID, f.OrganizationID, f.UserID, f.ApplicationID, f.ExpiresAt,
		f.RevokedAt, f.RevokeReason, f.CreatedAt, f.UpdatedAt, f.Revision)
	return err
}

func (r Repository) GetFamily(ctx context.Context, id string) (*domain.Family, error) {
	var f domain.Family
	err := r.db.QueryRow(ctx, queryGetFamily, id).Scan(
		&f.ID, &f.OrganizationID, &f.UserID, &f.ApplicationID, &f.ExpiresAt,
		&f.RevokedAt, &f.RevokeReason, &f.CreatedAt, &f.UpdatedAt, &f.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("remember-me family not found")
		}
		return nil, err
	}
	return &f, nil
}

func (r Repository) RevokeFamily(ctx context.Context, id string, at time.Time, reason string) error {
	_, err := r.db.Exec(ctx, queryRevokeFamily, id, at, reason)
	return err
}

func (r Repository) RevokeAllForUser(ctx context.Context, organizationID, userID string, at time.Time, reason string) (int64, error) {
	tag, err := r.db.Exec(ctx, queryRevokeFamiliesForUser, organizationID, userID, at, reason)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r Repository) CreateToken(ctx context.Context, t *domain.Token) error {
	_, err := r.db.Exec(ctx, queryInsertToken,
		t.ID, t.FamilyID, t.OrganizationID, t.TokenHash, t.TokenPrefix,
		t.ConsumedAt, t.ExpiresAt, t.CreatedAt, t.UpdatedAt, t.Revision)
	return err
}

func (r Repository) GetTokenByHash(ctx context.Context, hash string) (*domain.Token, error) {
	var t domain.Token
	err := r.db.QueryRow(ctx, queryGetTokenByHash, hash).Scan(
		&t.ID, &t.FamilyID, &t.OrganizationID, &t.TokenHash, &t.TokenPrefix,
		&t.ConsumedAt, &t.ExpiresAt, &t.CreatedAt, &t.UpdatedAt, &t.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("remember-me token not found")
		}
		return nil, err
	}
	return &t, nil
}

func (r Repository) ConsumeToken(ctx context.Context, id string, at time.Time) error {
	tag, err := r.db.Exec(ctx, queryConsumeToken, id, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("remember-me token already consumed")
	}
	return nil
}

func (r Repository) RevokeTokensByFamily(ctx context.Context, familyID string, at time.Time) error {
	_, err := r.db.Exec(ctx, queryRevokeTokensByFamily, familyID, at)
	return err
}

// DeleteExpired physically deletes expired tokens first, then expired families
// (whose token rows cascade). Each step is bounded by limit so a maintenance
// tick never holds a long delete. Returns the total rows removed.
func (r Repository) DeleteExpired(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	tokenTag, err := r.db.Exec(ctx, queryDeleteExpiredTokens, before, limit)
	if err != nil {
		return 0, err
	}
	familyTag, err := r.db.Exec(ctx, queryDeleteExpiredFamilies, before, limit)
	if err != nil {
		return tokenTag.RowsAffected(), err
	}
	return tokenTag.RowsAffected() + familyTag.RowsAffected(), nil
}
