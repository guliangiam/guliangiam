package captchaproviders

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	captchaProvider := &domain.CaptchaProvider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, captchaProvider)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	captchaProvider := &domain.CaptchaProvider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, captchaProvider)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	captchaProvider := &domain.CaptchaProvider{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, captchaProvider)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), captchaProvider.Revision)
}

// TestUpdate_TenantPredicate pins that Update carries the entity organization
// as a predicate alongside the id and the optimistic-lock revision.
func TestUpdate_TenantPredicate(t *testing.T) {
	ctx := context.Background()
	captchaProvider := &domain.CaptchaProvider{Revision: 2, OrganizationID: "org-1"}
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL, gotArgs = sql, args
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, captchaProvider)
	assert.NoError(t, err)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($10)")
	require.Len(t, gotArgs, 10)
	assert.Equal(t, "org-1", gotArgs[9])
	assert.Equal(t, int64(1), gotArgs[7], "app layer pre-increments Revision, WHERE uses Revision-1")
}
