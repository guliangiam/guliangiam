package entries

import (
	"context"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestRepository_Create(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db)
	require.NotNil(t, repo)
}

func TestRepository_List(t *testing.T) {
	now := time.Now()
	rows := shared.NewMockRows([][]any{
		{"e-1", "org-1", "login", "Login", "github", "app-1", "login", "192.168.1.1", "Mozilla/5.0", "msg", now, now, int64(1)},
	})
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	repo := NewRepository(db)
	require.NotNil(t, repo)
}
