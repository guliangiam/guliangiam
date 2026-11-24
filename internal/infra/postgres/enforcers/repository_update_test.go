package enforcers

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	enforcer := &domain.Enforcer{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, enforcer)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	enforcer := &domain.Enforcer{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, enforcer)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	enforcer := &domain.Enforcer{OrganizationID: "org-1"}
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, enforcer)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), enforcer.Revision)
	assert.Contains(t, gotSQL, "organization_id=$10::uuid", "update must carry the entity organization predicate")
	assert.Len(t, gotArgs, 10)
	assert.Equal(t, "org-1", gotArgs[9])
}
