package aigateway

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/stretchr/testify/require"
)

// TestProviderRepositoryMySQL proves the AI Gateway provider chain works on
// the generated MySQL DDL (OPEN-API-37 non-identity runtime evidence).
func TestProviderRepositoryMySQL(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	const orgID = "01a0ea76-0c00-7000-8000-000000000001"
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO organizations (id, slug, display_name, revision) VALUES (?, 'built-in', 'Built-in', 1)`, orgID)
	require.NoError(t, err)

	providerID := uuid.NewString()
	_, err = db.ExecContext(ctx, `INSERT INTO providers (id, organization_id, name, display_name, category, provider_type, client_secret, client_secret2, scopes, user_mapping, password, app_secret, revision) VALUES (?, ?, ?, 'MySQL Identity Provider', 'OAuth', 'GitHub', '', '', '[]', '{}', '', '', 1)`, providerID, orgID, "idp-"+providerID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM providers WHERE id = ?`, providerID) })

	repo := NewProvidersRepository(db)
	provider := &domain.LlmProvider{
		ID: providerID, OrganizationID: orgID, Name: "mysql-" + uuid.NewString(),
		DisplayName: "MySQL Provider", ProviderType: "openai", BaseURL: "https://api.openai.com/v1",
		SupportedProtocols: []string{"openai"}, PreferredProtocol: "openai",
		Status: "active", RequestTimeoutMs: 30000, MaxRetries: 2, Weight: 1,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Revision: 1,
	}
	require.NoError(t, repo.Create(ctx, provider))
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM llm_providers WHERE id = ?`, provider.ID) })

	loaded, err := repo.GetByIDForOrganization(ctx, orgID, provider.ID)
	require.NoError(t, err)
	require.Equal(t, provider.Name, loaded.Name)
	require.Equal(t, "openai", loaded.ProviderType)

	items, err := repo.List(ctx, orgID)
	require.NoError(t, err)
	require.NotEmpty(t, items)
}
