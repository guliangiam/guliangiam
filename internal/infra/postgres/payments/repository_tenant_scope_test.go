package payments

import (
	"context"
	"strings"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertScopedQuery(t *testing.T, query string) {
	t.Helper()
	assert.True(t, strings.Contains(query, "organization_id = guliangiam.resolve_org_id"),
		"scoped by-ID SQL must carry the tenant predicate")
}

func TestAttemptsRepository_GetByIDForOrganization_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewAttemptsRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "attempt-1", "org-1")
	require.Error(t, err)
	assertScopedQuery(t, gotSQL)
	require.Len(t, gotArgs, 2)
	assert.Equal(t, "attempt-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[1])
}

func TestAttemptsRepository_ListByPaymentForOrganization_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRows(nil), nil
		},
	}
	repo := NewAttemptsRepository(db)
	_, err := repo.ListByPaymentForOrganization(context.Background(), "payment-1", "org-1")
	require.NoError(t, err)
	assertScopedQuery(t, gotSQL)
	require.Len(t, gotArgs, 2)
	assert.Equal(t, "payment-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[1])
}

func TestRefundsRepository_GetByIDForOrganization_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRefundsRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "refund-1", "org-1")
	require.Error(t, err)
	assertScopedQuery(t, gotSQL)
	require.Len(t, gotArgs, 2)
	assert.Equal(t, "refund-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[1])
}

func TestChannelConfigsRepository_GetByIDForOrganization_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewChannelConfigsRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "cfg-1", "org-1")
	require.Error(t, err)
	assertScopedQuery(t, gotSQL)
	require.Len(t, gotArgs, 2)
	assert.Equal(t, "cfg-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[1])
}

func TestChannelConfigsRepository_UpdateCarriesEntityOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewChannelConfigsRepository(db)
	err := repo.Update(context.Background(), newChannelConfigForTenantTest())
	require.NoError(t, err)
	assertScopedQuery(t, gotSQL)
	assert.Contains(t, gotSQL, "$9")
	require.Len(t, gotArgs, 9)
	assert.Equal(t, "org-1", gotArgs[8])
}

func newChannelConfigForTenantTest() *domain.PayChannelConfig {
	return &domain.PayChannelConfig{
		ID:             "cfg-1",
		OrganizationID: "org-1",
		Channel:        "alipay",
		Environment:    "production",
		Status:         "draft",
		Revision:       1,
	}
}

func TestReconRepository_GetBatchForOrganization_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewReconRepository(db)
	_, err := repo.GetBatchForOrganization(context.Background(), "batch-1", "org-1")
	require.Error(t, err)
	assertScopedQuery(t, gotSQL)
	require.Len(t, gotArgs, 2)
	assert.Equal(t, "batch-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[1])
}

func TestReconRepository_ListItemsForOrganization_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRows(nil), nil
		},
	}
	repo := NewReconRepository(db)
	_, err := repo.ListItemsForOrganization(context.Background(), "batch-1", "org-1")
	require.NoError(t, err)
	assertScopedQuery(t, gotSQL)
	require.Len(t, gotArgs, 2)
	assert.Equal(t, "batch-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[1])
}

func TestReconRepository_ResolveItemForOrganization_CarriesTenantAndNotFound(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := NewReconRepository(db)
	err := repo.ResolveItemForOrganization(context.Background(), "item-1", "org-1", "ops")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assertScopedQuery(t, gotSQL)
	require.Len(t, gotArgs, 5)
	assert.Equal(t, "ops", gotArgs[0])
	assert.Equal(t, "item-1", gotArgs[3])
	assert.Equal(t, "org-1", gotArgs[4])
}
