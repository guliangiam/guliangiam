package dashboard

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStatsRepo struct {
	countUsersFn           func(ctx context.Context, orgID string) (*UserCountResult, error)
	countOrganizationsFn   func(ctx context.Context) (int64, error)
	countApplicationsFn    func(ctx context.Context, orgID string) (int64, error)
	countRolesFn           func(ctx context.Context, orgID string) (int64, error)
	countGroupsFn          func(ctx context.Context, orgID string) (int64, error)
	countPermissionsFn     func(ctx context.Context, orgID string) (int64, error)
	countProvidersFn       func(ctx context.Context, orgID string) (int64, error)
	countActiveSessionsFn  func(ctx context.Context, orgID string) (*SessionCountResult, error)
	countTokensFn          func(ctx context.Context, orgID string) (*TokenCountResult, error)
	countFailedLogins24hFn func(ctx context.Context, orgID string) (int64, error)
	getLoginHeatmapFn      func(ctx context.Context, orgID string) ([]HeatmapPoint, error)
	getMfaCoverageStatsFn  func(ctx context.Context, orgID string) (*MfaCoverageResult, error)
	getProviderDistFn      func(ctx context.Context, orgID string) ([]ProviderDistPoint, error)
	getTrendsFn            func(ctx context.Context, orgID string) ([]TrendPoint, error)
	getActivityHeatmapFn   func(ctx context.Context, orgID string) ([]ActivityPoint, error)
	getOrgUserDistFn       func(ctx context.Context) ([]OrgUserDistPoint, error)
}

func (m *mockStatsRepo) CountUsers(ctx context.Context, orgID string) (*UserCountResult, error) {
	if m.countUsersFn != nil {
		return m.countUsersFn(ctx, orgID)
	}
	return &UserCountResult{Total: 100, Active: 80, Disabled: 20, NewToday: 5}, nil
}
func (m *mockStatsRepo) CountOrganizations(ctx context.Context) (int64, error) {
	if m.countOrganizationsFn != nil {
		return m.countOrganizationsFn(ctx)
	}
	return 10, nil
}
func (m *mockStatsRepo) CountApplications(ctx context.Context, orgID string) (int64, error) {
	if m.countApplicationsFn != nil {
		return m.countApplicationsFn(ctx, orgID)
	}
	return 5, nil
}
func (m *mockStatsRepo) CountRoles(ctx context.Context, orgID string) (int64, error) {
	if m.countRolesFn != nil {
		return m.countRolesFn(ctx, orgID)
	}
	return 8, nil
}
func (m *mockStatsRepo) CountGroups(ctx context.Context, orgID string) (int64, error) {
	if m.countGroupsFn != nil {
		return m.countGroupsFn(ctx, orgID)
	}
	return 12, nil
}
func (m *mockStatsRepo) CountPermissions(ctx context.Context, orgID string) (int64, error) {
	if m.countPermissionsFn != nil {
		return m.countPermissionsFn(ctx, orgID)
	}
	return 30, nil
}
func (m *mockStatsRepo) CountProviders(ctx context.Context, orgID string) (int64, error) {
	if m.countProvidersFn != nil {
		return m.countProvidersFn(ctx, orgID)
	}
	return 3, nil
}
func (m *mockStatsRepo) CountActiveSessions(ctx context.Context, orgID string) (*SessionCountResult, error) {
	if m.countActiveSessionsFn != nil {
		return m.countActiveSessionsFn(ctx, orgID)
	}
	return &SessionCountResult{Total: 50, Active: 30}, nil
}
func (m *mockStatsRepo) CountTokens(ctx context.Context, orgID string) (*TokenCountResult, error) {
	if m.countTokensFn != nil {
		return m.countTokensFn(ctx, orgID)
	}
	return &TokenCountResult{Total: 200, Active: 150}, nil
}
func (m *mockStatsRepo) CountFailedLogins24h(ctx context.Context, orgID string) (int64, error) {
	if m.countFailedLogins24hFn != nil {
		return m.countFailedLogins24hFn(ctx, orgID)
	}
	return 7, nil
}
func (m *mockStatsRepo) GetLoginHeatmap(ctx context.Context, orgID string) ([]HeatmapPoint, error) {
	if m.getLoginHeatmapFn != nil {
		return m.getLoginHeatmapFn(ctx, orgID)
	}
	return []HeatmapPoint{{Hour: 9, Count: 15}}, nil
}
func (m *mockStatsRepo) GetMfaCoverageStats(ctx context.Context, orgID string) (*MfaCoverageResult, error) {
	if m.getMfaCoverageStatsFn != nil {
		return m.getMfaCoverageStatsFn(ctx, orgID)
	}
	return &MfaCoverageResult{Total: 100, WithMfa: 60}, nil
}
func (m *mockStatsRepo) GetProviderDistribution(ctx context.Context, orgID string) ([]ProviderDistPoint, error) {
	if m.getProviderDistFn != nil {
		return m.getProviderDistFn(ctx, orgID)
	}
	return []ProviderDistPoint{{Category: "saml", Count: 2}}, nil
}
func (m *mockStatsRepo) GetTrends(ctx context.Context, orgID string) ([]TrendPoint, error) {
	if m.getTrendsFn != nil {
		return m.getTrendsFn(ctx, orgID)
	}
	return []TrendPoint{{Date: "2024-01-01", Users: 10, Logins: 50}}, nil
}
func (m *mockStatsRepo) GetActivityHeatmap(ctx context.Context, orgID string) ([]ActivityPoint, error) {
	if m.getActivityHeatmapFn != nil {
		return m.getActivityHeatmapFn(ctx, orgID)
	}
	return []ActivityPoint{{Date: "2024-01-01", Count: 50}}, nil
}
func (m *mockStatsRepo) GetOrgUserDistribution(ctx context.Context) ([]OrgUserDistPoint, error) {
	if m.getOrgUserDistFn != nil {
		return m.getOrgUserDistFn(ctx)
	}
	return []OrgUserDistPoint{{OrganizationName: "org1", UserCount: 50}}, nil
}

func TestGetDashboardStats(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	stats, err := svc.GetDashboardStats(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(100), stats.TotalUsers)
	assert.Equal(t, int64(10), stats.TotalOrganizations)
	assert.Equal(t, int64(5), stats.TotalApplications)
}

func TestGetDashboardStats_Error(t *testing.T) {
	svc := NewService(&mockStatsRepo{
		countUsersFn: func(ctx context.Context, orgID string) (*UserCountResult, error) {
			return nil, fmt.Errorf("db error")
		},
	})
	_, err := svc.GetDashboardStats(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestGetUserStats(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	stats, err := svc.GetUserStats(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(100), stats.TotalUsers)
	assert.Equal(t, int64(80), stats.ActiveUsers)
}

func TestGetAuthStats(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	stats, err := svc.GetAuthStats(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(7), stats.FailedLogins24h)
}

func TestGetResourceStats(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	stats, err := svc.GetResourceStats(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(8), stats.TotalRoles)
	assert.Equal(t, int64(12), stats.TotalGroups)
}

func TestGetProviderDistribution(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	dist, err := svc.GetProviderDistribution(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, dist)
	assert.Len(t, dist, 1)
}

func TestGetMfaCoverage(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	coverage, err := svc.GetMfaCoverage(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, coverage)
	assert.Equal(t, int64(60), coverage.UsersWithMfa)
	assert.Equal(t, int64(40), coverage.UsersWithoutMfa)
	assert.Equal(t, 60.0, coverage.CoveragePercent)
}

func TestGetLoginHeatmap(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	heatmap, err := svc.GetLoginHeatmap(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, heatmap)
	assert.Len(t, heatmap, 1)
}

func TestGetTrends(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	trends, err := svc.GetTrends(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, trends)
	assert.Len(t, trends, 1)
}

func TestGetActivityHeatmap(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	heatmap, err := svc.GetActivityHeatmap(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, heatmap)
	assert.Len(t, heatmap, 1)
}

func TestGetOrgUserDistribution(t *testing.T) {
	svc := NewService(&mockStatsRepo{})
	dist, err := svc.GetOrgUserDistribution(context.Background())
	require.NoError(t, err)
	require.NotNil(t, dist)
	assert.Len(t, dist, 1)
}
