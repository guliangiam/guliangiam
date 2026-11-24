package applications

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// TestRepository_FindHostConflicts_ScopedToOrganizationAndExcludingSelf pins the
// predicate that makes R3d tenant-safe: 候选行只取同组织（$1）且排除自身（$2），
// 所以不同组织的同 host 不会命中，应用更新自己时也不会把自己的旧值判为冲突。
func TestRepository_FindHostConflicts_ScopedToOrganizationAndExcludingSelf(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			gotSQL, gotArgs = sql, args
			return shared.NewMockRows([][]any{
				{"app-2", "other", "App.Example.", nil},
			}), nil
		},
	}

	conflicts, err := NewRepository(mockDB).FindHostConflicts(context.Background(), "org-1", "app-self", []string{"app.example"})
	require.NoError(t, err)

	assert.Contains(t, gotSQL, "a.organization_id = $1::uuid")
	assert.Contains(t, gotSQL, "a.id <> $2::uuid")
	assert.Contains(t, gotSQL, "a.deleted_at IS NULL")
	assert.Equal(t, []any{"org-1", "app-self"}, gotArgs)
	assert.Equal(t, []domain.HostConflict{
		{Host: "app.example", ApplicationID: "app-2", ApplicationSlug: "other"},
	}, conflicts, "host 归一后应命中带末尾点的存量域名")
}

// TestRepository_FindHostConflicts_UnrelatedHostNotReported 覆盖反例：同组织内
// 的其他应用并不占用目标 host 时不能误报。
func TestRepository_FindHostConflicts_UnrelatedHostNotReported(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return shared.NewMockRows([][]any{
				{"app-3", "unrelated", "unrelated.example", nil},
			}), nil
		},
	}

	conflicts, err := NewRepository(mockDB).FindHostConflicts(context.Background(), "org-1", "app-self", []string{"app.example"})
	require.NoError(t, err)
	assert.Empty(t, conflicts)
}

// TestRepository_FindHostConflicts_EmptyHostsSkipQuery 保证无 host 可查时不打库。
func TestRepository_FindHostConflicts_EmptyHostsSkipQuery(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			t.Fatal("empty hosts must not query the database")
			return nil, nil
		},
	}

	conflicts, err := NewRepository(mockDB).FindHostConflicts(context.Background(), "org-1", "app-self", nil)
	require.NoError(t, err)
	assert.Empty(t, conflicts)
}
