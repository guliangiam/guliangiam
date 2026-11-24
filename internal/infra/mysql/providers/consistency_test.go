package providers

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

func TestMySQLConsistencyRegistrations(t *testing.T) {
	mock, repo := newMockDB(t)
	columns := []string{"id", "organization_id", "name", "display_name", "classified", "family", "kind", "variant", "status", "detail_id", "enabled", "deleted_at"}
	mock.ExpectQuery(regexp.QuoteMeta(queryConsistencyRegistrations)).WillReturnRows(
		sqlmock.NewRows(columns).
			AddRow("provider-1", "org-1", "OpenAI", "OpenAI Display", true, "ai", "openai", "", "active", "ai-1", true, nil).
			AddRow("provider-2", "org-1", "Legacy", "Legacy", false, "", "", "", "", "", false, nil),
	)

	items, err := repo.ListConsistencyRegistrations(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.True(t, items[0].Classified)
	assert.Equal(t, domain.FamilyAI, items[0].Family)
	assert.Equal(t, domain.StatusActive, items[0].Status)
	assert.False(t, items[1].Classified)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLConsistencyDetails(t *testing.T) {
	tests := []struct {
		name    string
		family  domain.Family
		query   string
		columns []string
		values  []driver.Value
		verify  func(*testing.T, []domain.DetailConsistency)
	}{
		{
			name: "ai", family: domain.FamilyAI, query: queryConsistencyAI,
			columns: []string{"id", "provider_id", "organization_id", "name", "display_name", "kind", "status", "deleted_at"},
			values:  []driver.Value{"ai-1", "provider-1", "org-1", "OpenAI", "OpenAI Display", "openai", "active", nil},
			verify: func(t *testing.T, items []domain.DetailConsistency) {
				assert.Equal(t, "OpenAI Display", items[0].DisplayName)
				assert.Equal(t, "active", items[0].Status)
			},
		},
		{
			name: "payment null anchor", family: domain.FamilyPayment, query: queryConsistencyPayment,
			columns: []string{"id", "provider_id", "organization_id", "kind", "variant", "status", "deleted_at"},
			values:  []driver.Value{"payment-1", "", "org-1", "alipay", "production", "draft", nil},
			verify: func(t *testing.T, items []domain.DetailConsistency) {
				assert.Empty(t, items[0].ProviderID)
				assert.Equal(t, "production", items[0].Variant)
			},
		},
		{
			name: "notification", family: domain.FamilyNotification, query: queryConsistencyNotification,
			columns: []string{"id", "provider_id", "organization_id", "name", "kind", "enabled"},
			values:  []driver.Value{"notification-1", "provider-1", "org-1", "Mail", "email", true},
			verify: func(t *testing.T, items []domain.DetailConsistency) {
				assert.True(t, items[0].Enabled)
				assert.Equal(t, "active", items[0].Status)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, repo := newMockDB(t)
			mock.ExpectQuery(regexp.QuoteMeta(tt.query)).WillReturnRows(sqlmock.NewRows(tt.columns).AddRow(tt.values...))
			items, err := repo.ListConsistencyDetails(context.Background(), tt.family)
			require.NoError(t, err)
			require.Len(t, items, 1)
			tt.verify(t, items)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}

	mock, repo := newMockDB(t)
	identity, err := repo.ListConsistencyDetails(context.Background(), domain.FamilyIdentity)
	require.NoError(t, err)
	assert.Empty(t, identity)
	_, err = repo.ListConsistencyDetails(context.Background(), domain.Family("unknown"))
	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
