package facetemplates

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// faceTemplateRow is a pgx.Row fake matching scanOne's dest types exactly
// (shared.MockRow only assigns basic scalar pointers, not *string/time.Time).
type faceTemplateRow struct {
	id, org, user, sealed, status string
	provider, providerRef         *string
	revokedAt                     *time.Time
	createdAt, updatedAt          time.Time
	err                           error
}

func (r faceTemplateRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*string) = r.id
	*dest[1].(*string) = r.org
	*dest[2].(*string) = r.user
	*dest[3].(**string) = &r.sealed
	*dest[4].(**string) = r.provider
	*dest[5].(**string) = r.providerRef
	*dest[6].(*string) = r.status
	*dest[7].(**time.Time) = r.revokedAt
	*dest[8].(*time.Time) = r.createdAt
	*dest[9].(*time.Time) = r.updatedAt
	return nil
}

func TestFaceTemplateAAD_RoundTripAndBinding(t *testing.T) {
	const ref = "base64-face-image-bytes"
	sealed, err := faceTemplateEncryptedMapper.EncryptColumn(faceTemplateAAD("user-1"), ref)
	require.NoError(t, err)
	assert.True(t, crypto.IsEnvelopeToken(sealed))
	assert.NotContains(t, sealed, ref)

	opened, err := faceTemplateEncryptedMapper.DecryptEnvelopeColumn(faceTemplateAAD("user-1"), sealed)
	require.NoError(t, err)
	assert.Equal(t, ref, opened)

	// A different user_id binding must not authenticate the ciphertext.
	_, err = faceTemplateEncryptedMapper.DecryptEnvelopeColumn(faceTemplateAAD("user-2"), sealed)
	assert.Error(t, err)
}

func TestEnroll_SealsReferenceBeforeWrite(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
			captured = args
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db)
	now := time.Now()
	err := repo.Enroll(context.Background(), &domain.FaceTemplate{
		ID: "tpl-1", OrganizationID: "org-1", UserID: "user-1",
		TemplateRef: "base64-face-image-bytes", Status: domain.FaceTemplateStatusActive,
		CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.Len(t, captured, 8)
	assert.Equal(t, "tpl-1", captured[0])
	assert.Equal(t, "org-1", captured[1])
	assert.Equal(t, "user-1", captured[2])
	sealed, ok := captured[3].(string)
	require.True(t, ok)
	assert.True(t, crypto.IsEnvelopeToken(sealed))
	assert.NotContains(t, sealed, "base64-face-image-bytes")
}

func TestEnroll_Validation(t *testing.T) {
	repo := NewRepository(&shared.MockDBTX{})
	err := repo.Enroll(context.Background(), &domain.FaceTemplate{ID: "tpl-1", OrganizationID: "org-1", UserID: "user-1"})
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)

	err = repo.Enroll(context.Background(), &domain.FaceTemplate{OrganizationID: "org-1", UserID: "user-1", TemplateRef: "x"})
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func TestGetActiveByUser_OpensReference(t *testing.T) {
	sealed, err := faceTemplateEncryptedMapper.EncryptColumn(faceTemplateAAD("user-1"), "base64-face-image-bytes")
	require.NoError(t, err)
	now := time.Now()
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return faceTemplateRow{
				id: "tpl-1", org: "org-1", user: "user-1", sealed: sealed,
				status: domain.FaceTemplateStatusActive, createdAt: now, updatedAt: now,
			}
		},
	}
	repo := NewRepository(db)
	template, err := repo.GetActiveByUser(context.Background(), "org-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "base64-face-image-bytes", template.TemplateRef)
	assert.Equal(t, "user-1", template.UserID)
	assert.True(t, template.IsActive())
}

func TestGetActiveByUser_NotFound(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return faceTemplateRow{err: pgx.ErrNoRows}
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetActiveByUser(context.Background(), "org-1", "user-1")
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestRevokeByUser_NotFoundWhenNoActiveRow(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.RevokeByUser(context.Background(), "org-1", "user-1", time.Now())
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestRevokeByUser_Succeeds(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)
	require.NoError(t, repo.RevokeByUser(context.Background(), "org-1", "user-1", time.Now()))
}
