package coupons

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func couponRowValues(usedCount int, revision int64) []any {
	now := time.Now()
	return []any{
		"c-1", "org-1", "name", "display", "desc", "SAVE10", "fixed", 10.0, 0.0, "",
		[]byte("[]"), []byte("[]"), 5, usedCount, 0, "", "", 0.0, "USD", "active",
		now, now, revision,
	}
}

// BE-C-07: exact code equality, tenant-scoped, never ILIKE.
func TestCoupons_GetByCode_ExactTenantScopedQuery(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow(couponRowValues(0, 1), nil)
		},
	}
	repo := NewRepository(db)
	c, err := repo.GetByCode(context.Background(), "org-1", "SAVE10")
	require.NoError(t, err)
	assert.Equal(t, "SAVE10", c.Code)
	assert.Contains(t, gotSQL, "code = $2")
	assert.NotContains(t, gotSQL, "ILIKE")
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($1)")
	require.Len(t, gotArgs, 2)
	assert.Equal(t, "org-1", gotArgs[0])
	assert.Equal(t, "SAVE10", gotArgs[1])
}

func TestCoupons_GetByCode_NotFound(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByCode(context.Background(), "org-1", "NOPE")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// BE-C-06: the increment is one guarded atomic statement, not read-modify-write.
func TestCoupons_IncrementUsage_AtomicGuardedSQL(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	now := time.Now()
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow(couponRowValues(1, 2), nil)
		},
	}
	repo := NewRepository(db)
	c, err := repo.IncrementUsage(context.Background(), "c-1", "u-1", now, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, c.UsedCount)
	assert.Equal(t, int64(2), c.Revision)
	assert.Contains(t, gotSQL, "used_count = used_count + 1")
	assert.Contains(t, gotSQL, "revision = revision + 1")
	assert.Contains(t, gotSQL, "status = 'active'")
	assert.Contains(t, gotSQL, "revision = $4")
	assert.Contains(t, gotSQL, "quantity <= 0 OR used_count < quantity")
	assert.Contains(t, gotSQL, "max_usage_per_user <= 0 OR")
	assert.Contains(t, gotSQL, "coupon_usage")
	require.Len(t, gotArgs, 4)
	assert.Equal(t, "c-1", gotArgs[0])
	assert.Equal(t, "u-1", gotArgs[1])
	assert.Equal(t, now, gotArgs[2])
	assert.Equal(t, int64(1), gotArgs[3])
}

func TestCoupons_IncrementUsage_GuardRejectionIsConflict(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)
	_, err := repo.IncrementUsage(context.Background(), "c-1", "u-1", time.Now(), 1)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
}

// BE-C-06: a zero-row Update with the row still present is a revision conflict,
// not a silent success and not a misleading NotFound.
func TestCoupons_Update_ZeroRowsWithRowPresentIsConflict(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(couponRowValues(0, 9), nil)
		},
	}
	repo := NewRepository(db)
	err := repo.Update(context.Background(), &domain.Coupon{ID: "c-1", OrganizationID: "org-1", Revision: 3})
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
}

func TestCoupons_Update_ZeroRowsWithRowGoneIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)
	err := repo.Update(context.Background(), &domain.Coupon{ID: "c-1", OrganizationID: "org-1", Revision: 3})
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}
