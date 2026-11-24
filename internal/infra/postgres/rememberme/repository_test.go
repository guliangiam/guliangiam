package rememberme

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

type recordingDBTX struct {
	calls   int
	queries []string
	args    [][]any
	tags    []pgconn.CommandTag
}

func (r *recordingDBTX) Exec(_ context.Context, query string, arguments ...any) (pgconn.CommandTag, error) {
	index := r.calls
	r.calls++
	r.queries = append(r.queries, query)
	r.args = append(r.args, arguments)
	if index < len(r.tags) {
		return r.tags[index], nil
	}
	return pgconn.CommandTag{}, nil
}

func (r *recordingDBTX) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }

func (r *recordingDBTX) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

var _ shared.DBTX = (*recordingDBTX)(nil)

func TestDeleteExpiredDeletesTokensThenFamilies(t *testing.T) {
	db := &recordingDBTX{tags: []pgconn.CommandTag{
		pgconn.NewCommandTag("DELETE 2"),
		pgconn.NewCommandTag("DELETE 1"),
	}}
	repo := NewRepository(db)
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

	deleted, err := repo.DeleteExpired(context.Background(), now, 500)
	require.NoError(t, err)
	require.Equal(t, int64(3), deleted)
	require.Equal(t, 2, db.calls)
	require.Contains(t, db.queries[0], "remember_me_tokens")
	require.Contains(t, db.queries[1], "remember_me_families")
	for _, args := range db.args {
		require.Equal(t, now, args[0])
		require.Equal(t, 500, args[1])
	}
}

func TestDeleteExpiredZeroLimitSkipsSQL(t *testing.T) {
	repo := NewRepository(&recordingDBTX{})
	deleted, err := repo.DeleteExpired(context.Background(), time.Now(), 0)
	require.NoError(t, err)
	require.Zero(t, deleted)
}
