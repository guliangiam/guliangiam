package ldaps

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/stretchr/testify/require"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	ldap := &domain.Ldap{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, ldap)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	ldap := &domain.Ldap{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, ldap)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	ldap := &domain.Ldap{ID: "ldap-1", OrganizationID: "org-1", Revision: 1}
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
	err := repo.Update(ctx, ldap)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), ldap.Revision)
	require.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($25)")
	require.Len(t, gotArgs, 25)
	require.Equal(t, "ldap-1", gotArgs[23])
	require.Equal(t, "org-1", gotArgs[24], "update must carry the entity organization predicate")
}
