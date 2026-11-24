package ssf

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	ssfdomain "github.com/guliangiam/guliangiam/internal/domain/ssf"
	"github.com/stretchr/testify/require"
)

// TestReceiverRepositoryMySQL proves the SSF receiver registry works on the
// generated MySQL DDL (OPEN-API-37 non-identity runtime evidence).
func TestReceiverRepositoryMySQL(t *testing.T) {
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

	repo := NewReceiverRepository(db)
	receiver := &ssfdomain.Receiver{
		ID: uuid.NewString(), OrganizationID: orgID, Name: "mysql-" + uuid.NewString(),
		ReceiverURL: "https://receiver.example/ssf", EventTypes: []byte(`["session-revoked"]`),
		Status: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Revision: 1,
	}
	require.NoError(t, repo.Create(ctx, receiver))
	t.Cleanup(func() { _ = repo.Delete(context.Background(), orgID, receiver.ID) })

	loaded, err := repo.GetByID(ctx, orgID, receiver.ID)
	require.NoError(t, err)
	require.Equal(t, receiver.Name, loaded.Name)

	items, err := repo.ListByOrganization(ctx, orgID)
	require.NoError(t, err)
	require.NotEmpty(t, items)
}
