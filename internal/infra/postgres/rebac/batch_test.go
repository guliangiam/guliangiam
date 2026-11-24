package rebac

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-D-18 回归：批量写入单条多行 CTE，一次 bump；冲突行保留原 zookie 并按键回读。

func TestRebac_CreateTuplesBatch_InsertAndConflictFallback(t *testing.T) {
	var batchSQL string
	var batchArgs []any
	var rereadArgs []any
	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			batchSQL, batchArgs = sql, args
			// 只有 tuple A 插入成功；tuple B 命中唯一约束被 DO NOTHING 跳过。
			return shared.NewMockRows([][]any{
				{"id-a", "org-1", "ns-1", "user:ann", nil, "member", "doc:1", "7", nil},
			}), nil
		},
		QueryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			rereadArgs = args
			return shared.NewMockRow([]any{
				"id-b-existing", "org-1", "ns-1", "user:bob", nil, "member", "doc:1", "3", nil,
			}, nil)
		},
	}
	repo := NewRepository(db)

	tuples := []*domain.Tuple{
		{ID: "id-a", OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:ann", Relation: "member", Object: "doc:1"},
		{ID: "id-b", OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:bob", Relation: "member", Object: "doc:1"},
	}
	require.NoError(t, repo.CreateTuplesBatch(context.Background(), tuples))

	assert.Contains(t, batchSQL, "WITH bump")
	assert.Contains(t, batchSQL, "revision_counter = revision_counter + 1")
	assert.Contains(t, batchSQL, "DO NOTHING")
	assert.Equal(t, 1, strings.Count(batchSQL, "WITH bump"), "整批只 bump 一次")
	assert.Equal(t, 2, strings.Count(batchSQL, "(SELECT bump.revision_counter FROM bump)"),
		"两行 VALUES 都引用同一次 bump 结果")
	require.Len(t, batchArgs, 2+5*2)
	assert.Equal(t, "ns-1", batchArgs[0])
	assert.Equal(t, "org-1", batchArgs[1])

	assert.Equal(t, "7", tuples[0].Zookie, "新插入行取 bump 后 revision")
	assert.Equal(t, "id-a", tuples[0].ID)
	assert.Equal(t, "id-b-existing", tuples[1].ID, "冲突行回读存量记录")
	assert.Equal(t, "3", tuples[1].Zookie, "冲突行保留原 zookie，不被 bump 覆盖")
	assert.Equal(t, []any{"org-1", "ns-1", "user:bob", "member", "doc:1"}, rereadArgs)
}

func TestRebac_CreateTuplesBatch_AllConflictsReread(t *testing.T) {
	rereads := 0
	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			rereads++
			return shared.NewMockRow([]any{
				"existing", "org-1", "ns-1", args[2], nil, args[3], args[4], "5", nil,
			}, nil)
		},
	}
	repo := NewRepository(db)
	tuples := []*domain.Tuple{
		{ID: "x1", OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:a", Relation: "member", Object: "doc:1"},
		{ID: "x2", OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:b", Relation: "member", Object: "doc:2"},
	}
	require.NoError(t, repo.CreateTuplesBatch(context.Background(), tuples))
	assert.Equal(t, 2, rereads)
	for _, tuple := range tuples {
		assert.Equal(t, "existing", tuple.ID)
		assert.Equal(t, "5", tuple.Zookie)
	}
}

func TestRebac_CreateTuplesBatch_RejectsMixedScope(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			t.Fatal("mixed-scope batch must be rejected before any SQL")
			return nil, nil
		},
	}
	repo := NewRepository(db)
	err := repo.CreateTuplesBatch(context.Background(), []*domain.Tuple{
		{ID: "a", OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:a", Relation: "member", Object: "doc:1"},
		{ID: "b", OrganizationID: "org-1", NamespaceID: "ns-2", Subject: "user:b", Relation: "member", Object: "doc:1"},
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func TestRebac_CreateTuplesBatch_Empty(t *testing.T) {
	repo := NewRepository(&shared.MockDBTX{})
	require.NoError(t, repo.CreateTuplesBatch(context.Background(), nil))
}
