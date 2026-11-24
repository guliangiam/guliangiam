package auditrepo

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	domain "github.com/guliangiam/guliangiam/internal/domain/authorization_audit"
	"github.com/stretchr/testify/require"
)

// TestAppendMySQLHashChain verifies the MySQL mirror writes the same
// prev_hash→hash chain shape as PostgreSQL (OPEN-API-37 runtime evidence).
func TestAppendMySQLHashChain(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	repo := NewRepository(db)
	action := "test.authorization_audit." + uuid.NewString()
	rec := &domain.AuditRecord{
		ActorID: uuid.New(), ActorType: "system",
		ResourceType: "test", Action: action, RiskLevel: "low", Decision: "allow",
	}
	firstAt := time.Now().UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
	require.NoError(t, repo.Append(ctx, rec, firstAt))

	secondAt := time.Now().UTC().Add(time.Millisecond).Truncate(time.Microsecond).Format(time.RFC3339Nano)
	require.NoError(t, repo.Append(ctx, rec, secondAt))

	records, err := repo.ListRecords(ctx, 500)
	require.NoError(t, err)
	mine := make([]domain.ChainRecord, 0, 2)
	for _, record := range records {
		if record.Action == action {
			mine = append(mine, record)
		}
	}
	require.Len(t, mine, 2)
	require.NotEmpty(t, mine[0].Hash)
	require.NotEmpty(t, mine[1].Hash)
	require.NotNil(t, mine[0].PrevHash)
	require.NotEqual(t, mine[0].Hash, mine[1].Hash)
}
