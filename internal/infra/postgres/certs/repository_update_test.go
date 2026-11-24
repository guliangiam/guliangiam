package certs

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	cert := &domain.Cert{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, cert)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	cert := &domain.Cert{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, cert)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	cert := &domain.Cert{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, cert)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), cert.Revision)
}

// The UPDATE predicate must carry the trusted organization so a zero-row result
// (foreign tenant or stale revision) cannot mutate another tenant's certificate.
func TestUpdate_PassesTrustedOrganization(t *testing.T) {
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(context.Background(), &domain.Cert{
		ID:             "cert-1",
		OrganizationID: "org-1",
		// The app layer increments Revision before Update, so the optimistic
		// lock guard is the previous value.
		Revision: 2,
	})
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.CodeConflict, appErr.Code)
	require.Len(t, gotArgs, 23)
	require.Equal(t, int64(1), gotArgs[20])
	require.Equal(t, "cert-1", gotArgs[21])
	require.Equal(t, "org-1", gotArgs[22])
}
