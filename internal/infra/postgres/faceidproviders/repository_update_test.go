package faceidproviders

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	faceIdProvider := &domain.FaceIdProvider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, faceIdProvider)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	faceIdProvider := &domain.FaceIdProvider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, faceIdProvider)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	faceIdProvider := &domain.FaceIdProvider{ID: "provider-1", OrganizationID: "org-1"}
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
	err := repo.Update(ctx, faceIdProvider)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), faceIdProvider.Revision)
	require.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($11)")
	require.Len(t, gotArgs, 11)
	require.Equal(t, "provider-1", gotArgs[9])
	require.Equal(t, "org-1", gotArgs[10], "update must carry the entity organization predicate")
}
