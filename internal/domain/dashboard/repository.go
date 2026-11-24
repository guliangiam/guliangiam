package dashboard

import "context"

// UserCountResult contains user count breakdowns.
type UserCountResult struct {
	Total     int64
	Active    int64
	Disabled  int64
	NewToday  int64
	New7Days  int64
	New30Days int64
}

// SessionCountResult contains session count breakdowns.
type SessionCountResult struct {
	Total  int64
	Active int64
}

// TokenCountResult contains token count breakdowns.
type TokenCountResult struct {
	Total  int64
	Active int64
}

// MfaCoverageResult contains MFA coverage statistics.
type MfaCoverageResult struct {
	Total   int64
	WithMfa int64
}

// HeatmapPoint represents an hour bucket in the login heatmap.
type HeatmapPoint struct {
	Hour  int
	Count int64
}

// ProviderDistPoint represents a provider category distribution entry.
type ProviderDistPoint struct {
	Category string
	Count    int64
}

// TrendPoint represents a daily data point for 30-day trends.
type TrendPoint struct {
	Date         string
	Users        int64
	Applications int64
	Providers    int64
	Logins       int64
}

// ActivityPoint represents a daily login count for the activity heatmap.
type ActivityPoint struct {
	Date  string
	Count int64
}

// OrgUserDistPoint represents user count per organization.
type OrgUserDistPoint struct {
	OrganizationName string
	UserCount        int64
}

// StatsRepository provides aggregate statistics via direct SQL queries.
// Defined in the app layer to avoid Infra imports; implemented in infra/postgres/dashboard.
type StatsRepository interface {
	CountUsers(ctx context.Context, organizationID string) (*UserCountResult, error)
	CountOrganizations(ctx context.Context) (int64, error)
	CountApplications(ctx context.Context, organizationID string) (int64, error)
	CountRoles(ctx context.Context, organizationID string) (int64, error)
	CountGroups(ctx context.Context, organizationID string) (int64, error)
	CountPermissions(ctx context.Context, organizationID string) (int64, error)
	CountProviders(ctx context.Context, organizationID string) (int64, error)
	CountActiveSessions(ctx context.Context, organizationID string) (*SessionCountResult, error)
	CountTokens(ctx context.Context, organizationID string) (*TokenCountResult, error)
	CountFailedLogins24h(ctx context.Context, organizationID string) (int64, error)
	GetLoginHeatmap(ctx context.Context, organizationID string) ([]HeatmapPoint, error)
	GetMfaCoverageStats(ctx context.Context, organizationID string) (*MfaCoverageResult, error)
	GetProviderDistribution(ctx context.Context, organizationID string) ([]ProviderDistPoint, error)
	GetTrends(ctx context.Context, organizationID string) ([]TrendPoint, error)
	GetActivityHeatmap(ctx context.Context, organizationID string) ([]ActivityPoint, error)
	GetOrgUserDistribution(ctx context.Context) ([]OrgUserDistPoint, error)
}
