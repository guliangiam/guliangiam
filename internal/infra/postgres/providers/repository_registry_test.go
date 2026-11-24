package providers

import (
	"context"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryGetConfigByType(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	db := &shared.MockDBTX{QueryRowFn: func(ctx context.Context, query string, args ...any) pgx.Row {
		assert.Equal(t, queryGetProviderConfigByType, query)
		assert.Equal(t, []any{"provider-1", "_provider_registry_v1"}, args)
		return shared.NewMockRow([]any{"provider-1", "_provider_registry_v1", []byte(`{"family":"ai","kind":"openai"}`), now}, nil)
	}}
	repo := NewRepository(db)

	config, err := repo.GetConfigByType(context.Background(), "provider-1", "_provider_registry_v1")
	require.NoError(t, err)
	assert.Equal(t, "provider-1", config.ProviderID)
	assert.Equal(t, "_provider_registry_v1", config.ConfigType)
	assert.Equal(t, "ai", config.Config["family"])
	assert.Equal(t, "openai", config.Config["kind"])
}

func TestRepositoryGetConfigByTypeNotFound(t *testing.T) {
	db := &shared.MockDBTX{QueryRowFn: func(ctx context.Context, query string, args ...any) pgx.Row {
		return shared.NewMockRow(nil, pgx.ErrNoRows)
	}}
	repo := NewRepository(db)

	config, err := repo.GetConfigByType(context.Background(), "missing", "_provider_registry_v1")
	require.Error(t, err)
	assert.Nil(t, config)
}

func TestRepositoryListConfigsByType(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	db := &shared.MockDBTX{QueryFn: func(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
		assert.True(t, strings.Contains(query, "provider_id IN ($2::uuid, $3::uuid)"), query)
		assert.Equal(t, []any{"_provider_registry_v1", "provider-1", "provider-2"}, args)
		return shared.NewMockRows([][]any{
			{"provider-1", "_provider_registry_v1", []byte(`{"family":"ai"}`), now},
			{"provider-2", "_provider_registry_v1", []byte(`{"family":"identity"}`), now},
		}), nil
	}}
	repo := NewRepository(db)

	configs, err := repo.ListConfigsByType(context.Background(), []string{"provider-1", "provider-2"}, "_provider_registry_v1")
	require.NoError(t, err)
	require.Len(t, configs, 2)
	assert.Equal(t, "provider-1", configs[0].ProviderID)
	assert.Equal(t, "ai", configs[0].Config["family"])
	assert.Equal(t, "provider-2", configs[1].ProviderID)
	assert.Equal(t, "identity", configs[1].Config["family"])
}

func TestRepositoryListConfigsByTypeEmpty(t *testing.T) {
	db := &shared.MockDBTX{QueryFn: func(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
		t.Fatal("empty provider IDs must not issue a query")
		return nil, nil
	}}
	repo := NewRepository(db)

	configs, err := repo.ListConfigsByType(context.Background(), nil, "_provider_registry_v1")
	require.NoError(t, err)
	assert.NotNil(t, configs)
	assert.Empty(t, configs)
}

func TestRepositoryRotateSecretStatus(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	current := domain.ProviderSecretStatus{ID: "secret-2", ProviderID: "provider-1", SecretType: "client_secret", Status: "active", LastRotatedAt: now, UpdatedAt: now, Version: 2}
	type execCall struct {
		query string
		args  []any
	}
	var calls []execCall
	db := &shared.MockDBTX{ExecFn: func(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
		calls = append(calls, execCall{query: query, args: args})
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}}
	repo := NewRepository(db)

	err := repo.RotateSecretStatus(context.Background(), current, "grace", expiresAt)
	require.NoError(t, err)
	require.Len(t, calls, 2)
	assert.Equal(t, queryRetirePreviousProviderSecrets, calls[0].query)
	assert.Equal(t, []any{"provider-1", "client_secret", int64(2), "grace", expiresAt, now}, calls[0].args)
	assert.Equal(t, queryInsertProviderSecretStatus, calls[1].query)
	assert.Equal(t, []any{"secret-2", "provider-1", "client_secret", "active", now, now, int64(2)}, calls[1].args)
}
