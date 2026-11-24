package aigateway

import (
	"context"
	"io"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestProviderHealthRepository_GetByIDForOrganization_UsesProviderOrganizationPredicate(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any

	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			capturedSQL = sql
			capturedArgs = append([]any(nil), args...)
			return shared.NewMockRow(make([]any, 8), nil)
		},
	}

	repo := NewProviderHealthRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "h-1")
	require.NoError(t, err)

	require.Contains(t, capturedSQL, "EXISTS", "provider ownership must be enforced in SQL")
	require.Contains(t, capturedSQL, "guliangiam.llm_providers")
	require.Contains(t, capturedSQL, "organization_id = $2")
	require.Equal(t, []any{"h-1", "org-1"}, capturedArgs)
}

func TestProviderHealthRepository_GetByIDForOrganization_NoRowsIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewProviderHealthRepository(db)

	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "h-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestProviderHealthRepository_ListByOrganization_UsesProviderOrganizationPredicate(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any

	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedSQL = sql
			capturedArgs = append([]any(nil), args...)
			return shared.NewMockRows(nil), nil
		},
	}

	repo := NewProviderHealthRepository(db)
	out, err := repo.ListByOrganization(context.Background(), "org-1")
	require.NoError(t, err)
	require.Empty(t, out)

	require.Contains(t, capturedSQL, "EXISTS")
	require.Contains(t, capturedSQL, "guliangiam.llm_providers")
	require.Contains(t, capturedSQL, "organization_id = $1")
	require.Equal(t, []any{"org-1"}, capturedArgs)
}

func TestProviderHealthRepository_ListByOrganization_PropagatesQueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}
	repo := NewProviderHealthRepository(db)

	_, err := repo.ListByOrganization(context.Background(), "org-1")
	require.Error(t, err)
}

func TestProvidersRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any

	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			capturedSQL = sql
			capturedArgs = append([]any(nil), args...)
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	repo := NewProvidersRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "p-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Contains(t, capturedSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"p-foreign", "org-1"}, capturedArgs)
}

func TestProvidersRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any

	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			capturedSQL = sql
			capturedArgs = append([]any(nil), args...)
			return shared.NewMockRow([]any{int64(3)}, nil)
		},
	}

	repo := NewProvidersRepository(db)
	p := &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: "provider", Revision: 2}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), p, "org-1"))
	require.Equal(t, int64(3), p.Revision)

	require.Contains(t, capturedSQL, "organization_id = guliangiam.resolve_org_id($20)")
	require.Len(t, capturedArgs, 20)
	require.Equal(t, int64(1), capturedArgs[17], "revision guard stays at $18")
	require.Equal(t, "p-1", capturedArgs[18])
	require.Equal(t, "org-1", capturedArgs[19], "trusted tenant is the last predicate")
}

func TestProvidersRepository_DeleteForOrganization_BindsTenantPredicate(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any

	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = append([]any(nil), args...)
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}

	repo := NewProvidersRepository(db)
	require.NoError(t, repo.DeleteForOrganization(context.Background(), "org-1", "p-1"))

	require.Contains(t, capturedSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"p-1", "org-1"}, capturedArgs)
}

func TestProvidersRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}

	repo := NewProvidersRepository(db)
	err := repo.DeleteForOrganization(context.Background(), "org-1", "p-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}
