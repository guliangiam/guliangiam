package trusteddevices

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestRevokeLegacyPlaintextUsesDigestPredicateAndReportsRows(t *testing.T) {
	db := &recordingDBTX{tag: pgconn.NewCommandTag("UPDATE 2")}
	repo := NewRepository(db)
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

	revoked, err := repo.RevokeLegacyPlaintext(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, int64(2), revoked)
	require.Equal(t, 1, db.execCalls)
	require.Contains(t, db.sql, "!~ '^[0-9a-f]{64}$'")
	require.Equal(t, now, db.args[0])
}
