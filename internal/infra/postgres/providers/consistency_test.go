package providers

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

func TestConsistencyRepositoryQueriesAndScans(t *testing.T) {
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, query string, _ ...any) (pgx.Rows, error) {
		switch query {
		case queryConsistencyRegistrations:
			return shared.NewMockRows([][]any{
				{"provider-1", "org-1", "OpenAI", "OpenAI Display", true, "ai", "openai", "", "active", "ai-1", true, nil},
				{"provider-2", "org-1", "Legacy", "Legacy", false, "", "", "", "", "", false, nil},
			}), nil
		case queryConsistencyAI:
			return shared.NewMockRows([][]any{{"ai-1", "provider-1", "org-1", "OpenAI", "OpenAI Display", "openai", "active", nil}}), nil
		case queryConsistencyPayment:
			return shared.NewMockRows([][]any{{"payment-1", "", "org-1", "alipay", "production", "draft", nil}}), nil
		case queryConsistencyNotification:
			return shared.NewMockRows([][]any{{"notification-1", "provider-1", "org-1", "Mail", "email", true}}), nil
		default:
			t.Fatalf("unexpected consistency query: %s", query)
			return nil, nil
		}
	}}
	repo := NewRepository(db)

	registrations, err := repo.ListConsistencyRegistrations(context.Background())
	require.NoError(t, err)
	require.Len(t, registrations, 2)
	assert.True(t, registrations[0].Classified)
	assert.Equal(t, domain.FamilyAI, registrations[0].Family)
	assert.Equal(t, domain.StatusActive, registrations[0].Status)
	assert.Equal(t, "ai-1", registrations[0].DetailID)
	assert.False(t, registrations[1].Classified)

	ai, err := repo.ListConsistencyDetails(context.Background(), domain.FamilyAI)
	require.NoError(t, err)
	require.Len(t, ai, 1)
	assert.Equal(t, "OpenAI Display", ai[0].DisplayName)
	assert.Equal(t, "active", ai[0].Status)

	payment, err := repo.ListConsistencyDetails(context.Background(), domain.FamilyPayment)
	require.NoError(t, err)
	require.Len(t, payment, 1)
	assert.Empty(t, payment[0].ProviderID)
	assert.Equal(t, "production", payment[0].Variant)

	notification, err := repo.ListConsistencyDetails(context.Background(), domain.FamilyNotification)
	require.NoError(t, err)
	require.Len(t, notification, 1)
	assert.True(t, notification[0].Enabled)
	assert.Equal(t, "active", notification[0].Status)

	identity, err := repo.ListConsistencyDetails(context.Background(), domain.FamilyIdentity)
	require.NoError(t, err)
	assert.Empty(t, identity)
	_, err = repo.ListConsistencyDetails(context.Background(), domain.Family("unknown"))
	require.Error(t, err)
}
