package notificationproviders

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	notificationProvider := &domain.NotificationProvider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, notificationProvider)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	notificationProvider := &domain.NotificationProvider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, notificationProvider)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	notificationProvider := &domain.NotificationProvider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, notificationProvider)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), notificationProvider.Revision)
}

func TestUpdate_EncryptsAccessToken(t *testing.T) {
	const plaintext = "rotated-access-token"
	provider := &domain.NotificationProvider{ID: "provider-1", OrganizationID: "org-1", AccessToken: plaintext, Revision: 2}
	var captured []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			require.Equal(t, queryUpdateNotificationProvider, sql)
			captured = args
			return shared.NewMockRow([]any{int64(3)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	require.NoError(t, repo.Update(context.Background(), provider))

	sealed, ok := captured[3].(string)
	require.True(t, ok)
	require.NotEqual(t, plaintext, sealed)
	require.True(t, crypto.IsEnvelopeToken(sealed))
	opened, err := crypto.GlobalFieldEncryptor().DecryptEnvelopeFieldWithAAD(notificationProviderAAD(provider.ID), sealed)
	require.NoError(t, err)
	require.Equal(t, plaintext, opened)
	require.Equal(t, plaintext, provider.AccessToken, "sealing must not mutate the domain object")
	require.Equal(t, "org-1", captured[len(captured)-1], "update must carry the entity tenant predicate")
}
