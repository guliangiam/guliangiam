package organizations

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

// TestListDefaultApplicationIDMySQL is the real-server evidence for
// OPEN-API-53: the list projection joins organization_settings and assembles
// defaultApplicationId for rows that have one, while rows without settings stay
// empty.
func TestListDefaultApplicationIDMySQL(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN not set; skipping MySQL organizations list integration test")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	require.NoError(t, db.PingContext(ctx), "MYSQL_DSN must point at a migrated MySQL 8.4+ database")

	slugSuffix := time.Now().Format("150405000")
	idWith := "0199aaaa-0000-7000-8000-" + slugSuffix
	idWithout := "0199bbbb-0000-7000-8000-" + slugSuffix
	insertOrganization := func(id, slug string) {
		_, err := db.ExecContext(ctx,
			"INSERT INTO organizations (id, slug, display_name, status, created_at, updated_at, revision) VALUES (?, ?, ?, 'active', NOW(), NOW(), 1)",
			id, slug, slug)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(context.Background(), "DELETE FROM organizations WHERE id = ?", id)
		})
	}
	insertOrganization(idWith, "org-defaultapp-"+slugSuffix)
	insertOrganization(idWithout, "org-nodefaultapp-"+slugSuffix)

	_, err = db.ExecContext(ctx,
		"INSERT INTO organization_settings (organization_id, languages, default_application_id, account_items, nav_preferences, feature_flags, country_codes, user_types, tags, mfa_items) VALUES (?, JSON_ARRAY(), ?, JSON_ARRAY(), JSON_OBJECT(), JSON_OBJECT(), JSON_ARRAY(), JSON_ARRAY(), JSON_ARRAY(), JSON_ARRAY())",
		idWith, "app-default-123")
	require.NoError(t, err)

	repo := NewRepository(db)
	items, total, err := repo.List(ctx, domain.ListQuery{Page: 1, PageSize: 50, Keyword: slugSuffix})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	byID := map[string]*domain.Organization{}
	for _, item := range items {
		byID[item.ID] = item
	}
	require.Equal(t, "app-default-123", byID[idWith].Settings.DefaultApplicationID)
	require.Empty(t, byID[idWithout].Settings.DefaultApplicationID)
}
