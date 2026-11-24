package coupons

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoupons_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestCoupons_Create(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.Coupon{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT")
}

func TestCoupons_Create_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.Coupon{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	assert.Error(t, err)
}

func TestCoupons_Update(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.Coupon{ID: "test-1", OrganizationID: "org-1", Revision: 3}
	err := repo.Update(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "UPDATE")
	require.Len(t, capturedArgs, 19)
	assert.Equal(t, "org-1", capturedArgs[17], "update must carry the entity organization predicate")
	assert.Equal(t, int64(2), capturedArgs[18], "update must pin the revision the caller read")
	assert.Contains(t, capturedSQL, "revision=$19", "update must carry the optimistic-lock predicate")
}

func TestCoupons_Update_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.Coupon{ID: "test-1"}
	err := repo.Update(t.Context(), item)
	assert.Error(t, err)
}

func TestCoupons_Delete(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.Delete(t.Context(), "test-1", "org-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "DELETE")
	require.Len(t, capturedArgs, 2)
	assert.Equal(t, "test-1", capturedArgs[0])
	assert.Equal(t, "org-1", capturedArgs[1])
}

func TestCoupons_Delete_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	err := repo.Delete(t.Context(), "test-1", "org-1")
	assert.Error(t, err)
}

func TestCoupons_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.Delete(t.Context(), "test-1", "org-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}
