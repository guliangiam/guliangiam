package providers

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryGetConfigByTypeMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(queryGetProviderConfigByType)).
		WithArgs("provider-1", "_provider_registry_v1").
		WillReturnRows(sqlmock.NewRows([]string{"provider_id", "config_type", "config", "updated_at"}).
			AddRow("provider-1", "_provider_registry_v1", []byte(`{"family":"ai","kind":"openai"}`), now))

	config, err := repo.GetConfigByType(context.Background(), "provider-1", "_provider_registry_v1")
	require.NoError(t, err)
	assert.Equal(t, "provider-1", config.ProviderID)
	assert.Equal(t, "_provider_registry_v1", config.ConfigType)
	assert.Equal(t, "ai", config.Config["family"])
	assert.Equal(t, "openai", config.Config["kind"])
	assert.True(t, config.UpdatedAt.Equal(now))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryListConfigsByTypeMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(buildListProviderConfigsByTypeQuery(2))).
		WithArgs("_provider_registry_v1", "provider-1", "provider-2").
		WillReturnRows(sqlmock.NewRows([]string{"provider_id", "config_type", "config", "updated_at"}).
			AddRow("provider-1", "_provider_registry_v1", []byte(`{"family":"ai"}`), now).
			AddRow("provider-2", "_provider_registry_v1", []byte(`{"family":"identity"}`), now))

	configs, err := repo.ListConfigsByType(context.Background(), []string{"provider-1", "provider-2"}, "_provider_registry_v1")
	require.NoError(t, err)
	require.Len(t, configs, 2)
	assert.Equal(t, "provider-1", configs[0].ProviderID)
	assert.Equal(t, "ai", configs[0].Config["family"])
	assert.Equal(t, "provider-2", configs[1].ProviderID)
	assert.Equal(t, "identity", configs[1].Config["family"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryListConfigsByTypeEmptyMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	configs, err := repo.ListConfigsByType(context.Background(), nil, "_provider_registry_v1")
	require.NoError(t, err)
	assert.NotNil(t, configs)
	assert.Empty(t, configs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryRotateSecretStatusMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	current := domain.ProviderSecretStatus{ID: "secret-2", ProviderID: "provider-1", SecretType: "client_secret", Status: "active", LastRotatedAt: now, UpdatedAt: now, Version: 2}
	mock.ExpectExec(regexp.QuoteMeta(queryRetirePreviousProviderSecrets)).
		WithArgs("grace", expiresAt, now, "provider-1", "client_secret", int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queryInsertProviderSecretStatus)).
		WithArgs("secret-2", "provider-1", "client_secret", "active", now, now, int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.RotateSecretStatus(context.Background(), current, "grace", expiresAt)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryRotateSecretStatusStopsAfterRetireErrorMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	current := domain.ProviderSecretStatus{ID: "secret-2", ProviderID: "provider-1", SecretType: "client_secret", Status: "active", LastRotatedAt: now, UpdatedAt: now, Version: 2}
	retireErr := errors.New("retire failed")
	mock.ExpectExec(regexp.QuoteMeta(queryRetirePreviousProviderSecrets)).
		WithArgs("rotated", now, now, "provider-1", "client_secret", int64(2)).
		WillReturnError(retireErr)

	err := repo.RotateSecretStatus(context.Background(), current, "rotated", now)
	require.ErrorIs(t, err, retireErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
