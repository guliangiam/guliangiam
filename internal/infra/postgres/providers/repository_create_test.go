package providers

import (
	"context"
	"testing"

	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func TestRepository_Create_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			require.Equal(t, queryInsertProvider, sql)
			require.Contains(t, sql, "revision, family, kind, variant")
			require.Len(t, args, 48)
			require.Equal(t, []any{domain.FamilyPayment, domain.Kind("alipay"), domain.VariantSandbox}, args[45:])
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Provider{
		Family:  domain.FamilyPayment,
		Kind:    domain.Kind("alipay"),
		Variant: domain.VariantSandbox,
	}
	err := repo.Create(context.Background(), entity)
	require.NoError(t, err)
}

func TestRepository_Create_EncryptsSecrets(t *testing.T) {
	const (
		plainClientSecret  = "client-secret"
		plainClientSecret2 = "client-secret-2"
		plainPassword      = "db-password"
		plainAppSecret     = "app-secret"
	)
	provider := &domain.Provider{
		ID:            "provider-secret",
		ClientSecret:  plainClientSecret,
		ClientSecret2: plainClientSecret2,
		Password:      plainPassword,
		AppSecret:     plainAppSecret,
	}
	var captured []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			require.Equal(t, queryInsertProvider, sql)
			captured = args
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	require.NoError(t, repo.Create(context.Background(), provider))

	for _, tc := range []struct {
		index     int
		column    string
		plaintext string
	}{
		{9, providerColumnClientSecret, plainClientSecret},
		{11, providerColumnClientSecret2, plainClientSecret2},
		{34, providerColumnPassword, plainPassword},
		{36, providerColumnAppSecret, plainAppSecret},
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
}

func TestRepository_Create_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Provider{}
	err := repo.Create(context.Background(), entity)
	require.Error(t, err)
}
