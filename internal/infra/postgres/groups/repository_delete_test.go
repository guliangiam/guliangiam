package groups

import (
	"context"
	"fmt"
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
)

func TestGroups_Delete_Success(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		assert.Equal(t, []any{"test-id", "org-1"}, args, "delete must scope by organization")
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	repo := NewRepository(db)
	require.NoError(t, repo.Delete(context.Background(), "test-id", "org-1"))
}

func TestGroups_Delete_ForeignTenantNotFound(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "org-1")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestGroups_Update_CarriesEntityOrganization(t *testing.T) {
	ctx := context.Background()
	group := &domain.Group{OrganizationID: "org-1", Revision: 2}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			assert.Equal(t, "org-1", args[len(args)-1], "update must carry the entity organization")
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	assert.Error(t, repo.Update(ctx, group))
}
