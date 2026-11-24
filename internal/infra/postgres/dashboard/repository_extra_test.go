package dashboard

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// queryRowRepo 构造一个 QueryRow 返回指定值的仓储。
func queryRowRepo(values []any, err error) Repository {
	tx := &fakeTx{
		queryRowFn: func(sql string, args ...any) pgx.Row {
			return shared.NewMockRow(values, err)
		},
	}
	repo, _ := newTestRepo(tx)
	return repo
}

func TestRepository_CountUsers_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(10), int64(8), int64(2), int64(1), int64(3), int64(5)}, nil)
	got, err := repo.CountUsers(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(10), got.Total)
	assert.Equal(t, int64(8), got.Active)
	assert.Equal(t, int64(2), got.Disabled)
	assert.Equal(t, int64(1), got.NewToday)
	assert.Equal(t, int64(3), got.New7Days)
	assert.Equal(t, int64(5), got.New30Days)
}

func TestRepository_CountUsers_ScanError(t *testing.T) {
	// CountUsers 需要 6 个值，只给 1 个 → MockRow.Scan 报 not enough values。
	repo := queryRowRepo([]any{int64(1)}, nil)
	_, err := repo.CountUsers(context.Background(), "org-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count users")
}

func TestRepository_CountOrganizations_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(7)}, nil)
	got, err := repo.CountOrganizations(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(7), got)
}

func TestRepository_CountApplications_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(3)}, nil)
	got, err := repo.CountApplications(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(3), got)
}

func TestRepository_CountRoles_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(5)}, nil)
	got, err := repo.CountRoles(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(5), got)
}

func TestRepository_CountGroups_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(4)}, nil)
	got, err := repo.CountGroups(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(4), got)
}

func TestRepository_CountPermissions_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(20)}, nil)
	got, err := repo.CountPermissions(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(20), got)
}

func TestRepository_CountProviders_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(2)}, nil)
	got, err := repo.CountProviders(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), got)
}

func TestRepository_CountActiveSessions_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(12), int64(9)}, nil)
	got, err := repo.CountActiveSessions(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(12), got.Total)
	assert.Equal(t, int64(9), got.Active)
}

func TestRepository_CountTokens_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(30), int64(10)}, nil)
	got, err := repo.CountTokens(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(30), got.Total)
	assert.Equal(t, int64(10), got.Active)
}

func TestRepository_CountFailedLogins24h_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(6)}, nil)
	got, err := repo.CountFailedLogins24h(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(6), got)
}

func TestRepository_GetMfaCoverageStats_Success(t *testing.T) {
	repo := queryRowRepo([]any{int64(100), int64(40)}, nil)
	got, err := repo.GetMfaCoverageStats(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(100), got.Total)
	assert.Equal(t, int64(40), got.WithMfa)
}

func TestRepository_GetLoginHeatmap_Success(t *testing.T) {
	rows := shared.NewMockRows([][]any{
		{0, int64(5)},
		{12, int64(10)},
		{23, int64(3)},
	})
	tx := &fakeTx{
		queryFn: func(sql string, args ...any) (pgx.Rows, error) { return rows, nil },
	}
	repo, _ := newTestRepo(tx)
	got, err := repo.GetLoginHeatmap(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, got, 24)
	assert.Equal(t, int64(5), got[0].Count)
	assert.Equal(t, int64(10), got[12].Count)
	assert.Equal(t, int64(3), got[23].Count)
}

func TestRepository_GetProviderDistribution_Success(t *testing.T) {
	rows := shared.NewMockRows([][]any{
		{"oauth", int64(3)},
		{"saml", int64(1)},
	})
	tx := &fakeTx{
		queryFn: func(sql string, args ...any) (pgx.Rows, error) { return rows, nil },
	}
	repo, _ := newTestRepo(tx)
	got, err := repo.GetProviderDistribution(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "oauth", got[0].Category)
	assert.Equal(t, int64(3), got[0].Count)
}

func TestRepository_GetTrends_Success(t *testing.T) {
	rows := shared.NewMockRows([][]any{
		{"2026-11-17", int64(1), int64(2), int64(0), int64(5)},
		{"2026-11-18", int64(0), int64(1), int64(1), int64(3)},
	})
	tx := &fakeTx{
		queryFn: func(sql string, args ...any) (pgx.Rows, error) { return rows, nil },
	}
	repo, _ := newTestRepo(tx)
	got, err := repo.GetTrends(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "2026-11-17", got[0].Date)
	assert.Equal(t, int64(1), got[0].Users)
	assert.Equal(t, int64(2), got[0].Applications)
	assert.Equal(t, int64(0), got[0].Providers)
	assert.Equal(t, int64(5), got[0].Logins)
}

func TestRepository_GetActivityHeatmap_Success(t *testing.T) {
	rows := shared.NewMockRows([][]any{
		{"2026-11-17", int64(10)},
		{"2026-11-18", int64(20)},
	})
	tx := &fakeTx{
		queryFn: func(sql string, args ...any) (pgx.Rows, error) { return rows, nil },
	}
	repo, _ := newTestRepo(tx)
	got, err := repo.GetActivityHeatmap(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "2026-11-17", got[0].Date)
	assert.Equal(t, int64(10), got[0].Count)
}

func TestRepository_GetOrgUserDistribution_Success(t *testing.T) {
	rows := shared.NewMockRows([][]any{
		{"Org A", int64(15)},
		{"Org B", int64(8)},
	})
	tx := &fakeTx{
		queryFn: func(sql string, args ...any) (pgx.Rows, error) { return rows, nil },
	}
	repo, _ := newTestRepo(tx)
	got, err := repo.GetOrgUserDistribution(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Org A", got[0].OrganizationName)
	assert.Equal(t, int64(15), got[0].UserCount)
}
