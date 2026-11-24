package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	provider := &domain.Provider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, provider)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	provider := &domain.Provider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, provider)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	provider := &domain.Provider{
		ID:       "provider-1",
		Name:     "renamed-provider",
		Revision: 2,
		Family:   domain.FamilyPayment,
		Kind:     domain.Kind("alipay"),
		Variant:  domain.VariantSandbox,
	}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			require.Equal(t, queryUpdateProvider, sql)
			require.Contains(t, sql, "family = $43")
			require.Contains(t, sql, "kind = $44")
			require.Contains(t, sql, "variant = $45")
			require.Contains(t, sql, "name = $46")
			require.Len(t, args, 47)
			require.Equal(t, int64(1), args[40])
			require.Equal(t, "provider-1", args[41])
			require.Equal(t, []any{domain.FamilyPayment, domain.Kind("alipay"), domain.VariantSandbox}, args[42:45])
			require.Equal(t, "renamed-provider", args[45])
			require.Equal(t, "", args[46], "background context keeps the all-tenant predicate")
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, provider)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), provider.Revision)
}

func TestUpdate_EncryptsSecrets(t *testing.T) {
	const (
		plainClientSecret  = "rotated-client-secret"
		plainClientSecret2 = "rotated-client-secret-2"
		plainPassword      = "rotated-password"
		plainAppSecret     = "rotated-app-secret"
	)
	provider := &domain.Provider{
		ID:            "provider-secret",
		Revision:      2,
		ClientSecret:  plainClientSecret,
		ClientSecret2: plainClientSecret2,
		Password:      plainPassword,
		AppSecret:     plainAppSecret,
	}
	var captured []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			require.Equal(t, queryUpdateProvider, sql)
			captured = args
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	require.NoError(t, repo.Update(context.Background(), provider))

	for _, tc := range []struct {
		index     int
		column    string
		plaintext string
	}{
		{6, providerColumnClientSecret, plainClientSecret},
		{8, providerColumnClientSecret2, plainClientSecret2},
		{31, providerColumnPassword, plainPassword},
		{33, providerColumnAppSecret, plainAppSecret},
	} {
		sealed, ok := captured[tc.index].(string)
		require.True(t, ok)
		require.NotEqual(t, tc.plaintext, sealed)
		require.True(t, crypto.IsEnvelopeToken(sealed))
		ctx := crypto.AADContext{Table: providerTable, Column: tc.column, PKValue: provider.ID}
		opened, err := crypto.GlobalFieldEncryptor().DecryptEnvelopeFieldWithAAD(ctx, sealed)
		require.NoError(t, err)
		require.Equal(t, tc.plaintext, opened)
	}
	require.Equal(t, plainClientSecret, provider.ClientSecret, "sealing must not mutate the domain object")
	assert.Equal(t, int64(2), provider.Revision)
}

func TestUpdateConfig_ExecError(t *testing.T) {
	ctx := context.Background()
	providerConfig := &domain.ProviderConfig{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateConfig(ctx, providerConfig)
	assert.Error(t, err)
}

func TestUpdateConfig_Success(t *testing.T) {
	ctx := context.Background()
	providerConfig := &domain.ProviderConfig{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateConfig(ctx, providerConfig)
	assert.NoError(t, err)
}
