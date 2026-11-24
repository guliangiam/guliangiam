package products

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/products"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// productRowValues mirrors the full RETURNING/SELECT column list of the
// products table so scanProduct can read a mocked row.
func productRowValues(state string, quantity, sold int) []any {
	return []any{
		"p-1", "org-1", "name", "display", "image", "detail", "desc", "tag",
		"USD", 10.0, quantity, sold, false, []byte(`[]`), false, []byte(`[]`),
		"", state, nil, nil, int64(1),
	}
}

// TestIncrementSold_AtomicGuardSQL pins BE-C-11: the consumption must be a
// single guarded UPDATE (sold = sold + q under sold+q <= quantity), never a
// read-modify-write, and it must stay organization-scoped.
func TestIncrementSold_AtomicGuardSQL(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL, gotArgs = sql, args
			return shared.NewMockRow(productRowValues("active", 5, 1), nil)
		},
	}
	repo := NewRepository(db)
	now := time.Now()
	p, err := repo.IncrementSold(context.Background(), "p-1", "org-1", 2, now)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Contains(t, gotSQL, "sold = sold + $3")
	assert.Contains(t, gotSQL, "sold + $3 <= quantity")
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	assert.Contains(t, gotSQL, "status = 'active'")
	assert.Equal(t, []any{"p-1", "org-1", 2, now}, gotArgs)
}

func TestIncrementSold_NoRowsDistinguishesMissingFromExhausted(t *testing.T) {
	tests := []struct {
		name     string
		readRow  *shared.MockRow
		wantCode apperrors.Code
	}{
		{
			name:     "missing or foreign product is NotFound",
			readRow:  shared.NewMockRow(nil, pgx.ErrNoRows),
			wantCode: apperrors.CodeNotFound,
		},
		{
			name:     "exhausted stock is 422",
			readRow:  shared.NewMockRow(productRowValues("active", 1, 1), nil),
			wantCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name:     "non-active product is 422",
			readRow:  shared.NewMockRow(productRowValues("draft", 5, 0), nil),
			wantCode: apperrors.CodeUnprocessableEntity,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			db := &shared.MockDBTX{
				QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
					calls++
					if calls == 1 {
						return shared.NewMockRow(nil, pgx.ErrNoRows) // guarded UPDATE matched nothing
					}
					return tt.readRow // scoped read-back distinguishes the cause
				},
			}
			repo := NewRepository(db)
			_, err := repo.IncrementSold(context.Background(), "p-1", "org-1", 1, time.Now())
			require.Error(t, err)
			assert.Equal(t, tt.wantCode, apperrors.FromError(err).Code)
		})
	}
}

func TestIncrementSold_DBErrorPropagates(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db down"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.IncrementSold(context.Background(), "p-1", "org-1", 1, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestReleaseSold_ReturnsStockFlooredAtZero(t *testing.T) {
	var gotSQL string
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			return shared.NewMockRow(productRowValues("active", 5, 0), nil)
		},
	}
	repo := NewRepository(db)
	p, err := repo.ReleaseSold(context.Background(), "p-1", "org-1", 2, time.Now())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Contains(t, gotSQL, "GREATEST(sold - $3, 0)")
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
}

func TestReleaseSold_MissingIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)
	_, err := repo.ReleaseSold(context.Background(), "p-1", "org-1", 1, time.Now())
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

var _ interface {
	IncrementSold(context.Context, string, string, int, time.Time) (*domain.Product, error)
	ReleaseSold(context.Context, string, string, int, time.Time) (*domain.Product, error)
} = Repository{}
