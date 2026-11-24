package organizations

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/domain/organizations"
)

// 真实库门控用例：List 是控制台登录后第一个读（组织切换器），曾因扫描/映射缺陷
// 在真实库上返回 500。单测用 mock 覆盖不到这条链路，必须用真实 schema 盯住。
func TestList_AgainstRealSchema(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set — real schema integration test skipped")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx))

	repo := NewRepository(pool)
	items, total, err := repo.List(ctx, organizations.ListQuery{Page: 1, PageSize: 20, Status: "active"})
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, items)
	require.NotEmpty(t, items[0].ID)
	require.NotEmpty(t, items[0].Slug)
}
