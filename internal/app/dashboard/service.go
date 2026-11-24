package dashboard

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Service provides dashboard statistics.
type Service struct {
	statsRepo StatsRepository
}

// NewService creates a new dashboard service.
func NewService(statsRepo StatsRepository) Service {
	return Service{statsRepo: statsRepo}
}

// DashboardStats contains aggregate statistics for the dashboard.
type DashboardStats struct {
	TotalUsers         int64 `json:"totalUsers"`
	TotalOrganizations int64 `json:"totalOrganizations"`
	TotalApplications  int64 `json:"totalApplications"`
	TotalRoles         int64 `json:"totalRoles"`
	TotalGroups        int64 `json:"totalGroups"`
	TotalPermissions   int64 `json:"totalPermissions"`
}

// UserStats contains user-related statistics.
type UserStats struct {
	TotalUsers     int64 `json:"totalUsers"`
	ActiveUsers    int64 `json:"activeUsers"`
	DisabledUsers  int64 `json:"disabledUsers"`
	NewUsersToday  int64 `json:"newUsersToday"`
	NewUsers7Days  int64 `json:"newUsers7Days"`
	NewUsers30Days int64 `json:"newUsers30Days"`
}

// AuthStats contains authentication statistics.
type AuthStats struct {
	TotalSessions   int64 `json:"totalSessions"`
	ActiveSessions  int64 `json:"activeSessions"`
	TotalTokens     int64 `json:"totalTokens"`
	ActiveTokens    int64 `json:"activeTokens"`
	FailedLogins24h int64 `json:"failedLogins24h"`
}

// ResourceStats contains resource statistics.
type ResourceStats struct {
	TotalApplications int64 `json:"totalApplications"`
	TotalRoles        int64 `json:"totalRoles"`
	TotalGroups       int64 `json:"totalGroups"`
	TotalPermissions  int64 `json:"totalPermissions"`
	TotalProviders    int64 `json:"totalProviders"`
}

// ProviderDistribution contains provider type distribution data.
type ProviderDistribution struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// MfaCoverage contains MFA coverage statistics.
type MfaCoverage struct {
	UsersWithMfa    int64   `json:"usersWithMfa"`
	UsersWithoutMfa int64   `json:"usersWithoutMfa"`
	CoveragePercent float64 `json:"coveragePercent"`
}

// LoginHeatmapPoint represents a time bucket in the login heatmap.
type LoginHeatmapPoint struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// TrendDataPoint represents a daily data point for 30-day trends.
type TrendDataPoint struct {
	Date         string `json:"date"`
	Users        int64  `json:"users"`
	Applications int64  `json:"applications"`
	Providers    int64  `json:"providers"`
	Logins       int64  `json:"logins"`
}

// ActivityHeatmapPoint represents a daily login count for the activity heatmap.
type ActivityHeatmapPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// OrgUserDistribution represents user count per organization.
type OrgUserDistribution struct {
	OrganizationName string `json:"organizationName"`
	UserCount        int64  `json:"userCount"`
}

// GetDashboardStats returns aggregate statistics for the dashboard.
func (s Service) GetDashboardStats(ctx context.Context, organizationID string) (*DashboardStats, error) {
	stats := &DashboardStats{}

	userCounts, err := s.statsRepo.CountUsers(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get dashboard stats")
	}
	stats.TotalUsers = userCounts.Total

	stats.TotalOrganizations, err = s.statsRepo.CountOrganizations(ctx)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get dashboard stats")
	}

	stats.TotalApplications, err = s.statsRepo.CountApplications(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get dashboard stats")
	}

	stats.TotalRoles, err = s.statsRepo.CountRoles(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get dashboard stats")
	}

	stats.TotalGroups, err = s.statsRepo.CountGroups(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get dashboard stats")
	}

	stats.TotalPermissions, err = s.statsRepo.CountPermissions(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get dashboard stats")
	}

	return stats, nil
}

// GetUserStats returns user-related statistics.
func (s Service) GetUserStats(ctx context.Context, organizationID string) (*UserStats, error) {
	result, err := s.statsRepo.CountUsers(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get user stats")
	}
	return &UserStats{
		TotalUsers:     result.Total,
		ActiveUsers:    result.Active,
		DisabledUsers:  result.Disabled,
		NewUsersToday:  result.NewToday,
		NewUsers7Days:  result.New7Days,
		NewUsers30Days: result.New30Days,
	}, nil
}

// GetAuthStats returns authentication statistics.
func (s Service) GetAuthStats(ctx context.Context, organizationID string) (*AuthStats, error) {
	sessionCounts, err := s.statsRepo.CountActiveSessions(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get auth stats")
	}

	tokenCounts, err := s.statsRepo.CountTokens(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get auth stats")
	}

	failedLogins, err := s.statsRepo.CountFailedLogins24h(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get auth stats")
	}

	return &AuthStats{
		TotalSessions:   sessionCounts.Total,
		ActiveSessions:  sessionCounts.Active,
		TotalTokens:     tokenCounts.Total,
		ActiveTokens:    tokenCounts.Active,
		FailedLogins24h: failedLogins,
	}, nil
}

// GetResourceStats returns resource statistics.
func (s Service) GetResourceStats(ctx context.Context, organizationID string) (*ResourceStats, error) {
	stats := &ResourceStats{}

	var err error
	stats.TotalApplications, err = s.statsRepo.CountApplications(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get resource stats")
	}

	stats.TotalRoles, err = s.statsRepo.CountRoles(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get resource stats")
	}

	stats.TotalGroups, err = s.statsRepo.CountGroups(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get resource stats")
	}

	stats.TotalPermissions, err = s.statsRepo.CountPermissions(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get resource stats")
	}

	stats.TotalProviders, err = s.statsRepo.CountProviders(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get resource stats")
	}

	return stats, nil
}

// GetProviderDistribution returns the distribution of providers by category.
func (s Service) GetProviderDistribution(ctx context.Context, organizationID string) ([]ProviderDistribution, error) {
	items, err := s.statsRepo.GetProviderDistribution(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get provider distribution")
	}
	result := make([]ProviderDistribution, len(items))
	for i, item := range items {
		result[i] = ProviderDistribution{Category: item.Category, Count: item.Count}
	}
	return result, nil
}

// GetMfaCoverage returns the MFA coverage rate for users in the organization.
func (s Service) GetMfaCoverage(ctx context.Context, organizationID string) (*MfaCoverage, error) {
	result, err := s.statsRepo.GetMfaCoverageStats(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get mfa coverage")
	}
	withoutMfa := result.Total - result.WithMfa
	var coverage float64
	if result.Total > 0 {
		coverage = float64(result.WithMfa) / float64(result.Total) * 100
	}
	return &MfaCoverage{UsersWithMfa: result.WithMfa, UsersWithoutMfa: withoutMfa, CoveragePercent: coverage}, nil
}

// GetLoginHeatmap returns login activity distribution by hour of day.
func (s Service) GetLoginHeatmap(ctx context.Context, organizationID string) ([]LoginHeatmapPoint, error) {
	items, err := s.statsRepo.GetLoginHeatmap(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get login heatmap")
	}
	result := make([]LoginHeatmapPoint, len(items))
	for i, item := range items {
		result[i] = LoginHeatmapPoint{Hour: item.Hour, Count: item.Count}
	}
	return result, nil
}

// GetTrends returns 30-day trend data for users, applications, and providers.
func (s Service) GetTrends(ctx context.Context, organizationID string) ([]TrendDataPoint, error) {
	items, err := s.statsRepo.GetTrends(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get trends")
	}
	result := make([]TrendDataPoint, len(items))
	for i, item := range items {
		result[i] = TrendDataPoint{
			Date:         item.Date,
			Users:        item.Users,
			Applications: item.Applications,
			Providers:    item.Providers,
			Logins:       item.Logins,
		}
	}
	return result, nil
}

// GetActivityHeatmap returns daily login counts for the last 365 days.
func (s Service) GetActivityHeatmap(ctx context.Context, organizationID string) ([]ActivityHeatmapPoint, error) {
	items, err := s.statsRepo.GetActivityHeatmap(ctx, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get activity heatmap")
	}
	result := make([]ActivityHeatmapPoint, len(items))
	for i, item := range items {
		result[i] = ActivityHeatmapPoint{Date: item.Date, Count: item.Count}
	}
	return result, nil
}

// GetOrgUserDistribution returns user count per organization.
func (s Service) GetOrgUserDistribution(ctx context.Context) ([]OrgUserDistribution, error) {
	items, err := s.statsRepo.GetOrgUserDistribution(ctx)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get org user distribution")
	}
	result := make([]OrgUserDistribution, len(items))
	for i, item := range items {
		result[i] = OrgUserDistribution{
			OrganizationName: item.OrganizationName,
			UserCount:        item.UserCount,
		}
	}
	return result, nil
}
