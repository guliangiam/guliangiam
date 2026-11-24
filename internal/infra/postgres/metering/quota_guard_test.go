package metering

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// TestConsumeQuota_SQLGuardsPositiveDelta pins BE-C-12: the counter update
// must carry the $3 > 0 guard so a non-positive delta can never shrink
// used_value, on top of the existing limit guard.
func TestConsumeQuota_SQLGuardsPositiveDelta(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL, gotArgs = sql, args
			return shared.NewMockRow([]any{int64(12), int64(100)}, nil)
		},
	}
	repo := Repository{db: db}
	status, err := repo.ConsumeQuota(context.Background(), "org-1", "events", 5)
	require.NoError(t, err)
	assert.True(t, status.Allowed)
	assert.Equal(t, int64(12), status.Used)
	assert.Contains(t, gotSQL, "$3 > 0")
	assert.Contains(t, gotSQL, "used_value + $3 <= limit_value")
	assert.Equal(t, []any{"org-1", "events", int64(5)}, gotArgs)
}

// TestConsumeQuota_NonPositiveDeltaIsNoop: a non-positive delta must not
// reach the database at all (and must not fall into the deny read-back).
func TestConsumeQuota_NonPositiveDeltaIsNoop(t *testing.T) {
	for _, delta := range []int64{0, -1000000} {
		db := &shared.MockDBTX{
			QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				t.Fatalf("ConsumeQuota(delta=%d) touched the database", delta)
				return nil
			},
		}
		repo := Repository{db: db}
		status, err := repo.ConsumeQuota(context.Background(), "org-1", "events", delta)
		require.NoError(t, err)
		assert.True(t, status.Allowed)
	}
}

// TestConsumeQuota_GuardMissDenies: when the guarded UPDATE matches nothing
// and a quota row exists, the caller is denied with the current counters.
func TestConsumeQuota_GuardMissDenies(t *testing.T) {
	calls := 0
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			calls++
			if calls == 1 {
				return shared.NewMockRow(nil, pgx.ErrNoRows)
			}
			return shared.NewMockRow([]any{int64(100), int64(100)}, nil)
		},
	}
	repo := Repository{db: db}
	status, err := repo.ConsumeQuota(context.Background(), "org-1", "events", 1)
	require.NoError(t, err)
	assert.False(t, status.Allowed)
	assert.Equal(t, int64(100), status.Used)
	assert.Equal(t, int64(100), status.Limit)
}
