package mfachallenges

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
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

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Challenge, error) {
	var row challengeRow
	err := r.db.QueryRow(ctx, queryGetChallengeByID, id).Scan(&row.ID, &row.OrganizationID, &row.UserID, &row.ApplicationID, &row.State, &row.MethodType, &row.Metadata, &row.ExpiresAt, &row.VerifiedAt, &row.CreatedAt, &row.UpdatedAt, &row.Revision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("login challenge not found")
		}
		return nil, err
	}

	return toDomain(row), nil
}

func (r Repository) Create(ctx context.Context, challenge *domain.Challenge) error {
	metadata, err := challengeMetadataJSON(challenge)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsertChallenge, challenge.ID, challenge.OrganizationID, challenge.UserID, challenge.ApplicationID, challenge.State, challenge.MethodType, metadata, challenge.ExpiresAt, challenge.VerifiedAt, challenge.CreatedAt, challenge.UpdatedAt, challenge.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, challenge *domain.Challenge) error {
	metadata, err := challengeMetadataJSON(challenge)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateChallenge, challenge.State, challenge.MethodType, metadata, challenge.ExpiresAt, challenge.VerifiedAt, challenge.UpdatedAt, challenge.Revision-1, challenge.ID).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("login challenge revision conflict")
		}
		return err
	}
	challenge.Revision = newRevision
	return nil
}

// challengeMetadataJSON serialises the metadata bag; nil becomes "{}" so the
// NOT NULL JSONB column keeps its default shape.
func challengeMetadataJSON(challenge *domain.Challenge) ([]byte, error) {
	if len(challenge.Metadata) == 0 {
		return []byte("{}"), nil
	}
	return shared.MarshalJSON(challenge.Metadata)
}

func (r Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx, queryDeleteChallenge, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("login challenge not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
