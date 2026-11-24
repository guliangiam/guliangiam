package trusteddevices

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestDeleteExpiredBatchesByLimitAndReturnsRows(t *testing.T) {
	db := &recordingDBTX{tag: pgconn.NewCommandTag("DELETE 3")}
	repo := NewRepository(db)
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

	deleted, err := repo.DeleteExpired(context.Background(), now, 100)
	require.NoError(t, err)
	require.Equal(t, int64(3), deleted)
	require.Equal(t, 1, db.execCalls)
	require.Equal(t, now, db.args[0])
	require.Equal(t, 100, db.args[1])
}

func TestDeleteExpiredZeroLimitSkipsSQL(t *testing.T) {
	db := &recordingDBTX{}
	repo := NewRepository(db)
	deleted, err := repo.DeleteExpired(context.Background(), time.Now(), 0)
	require.NoError(t, err)
	require.Zero(t, deleted)
	require.Zero(t, db.execCalls)
}
