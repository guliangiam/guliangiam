package dashboard

import (
	"context"

	appdashboard "github.com/guliangiam/guliangiam/internal/app/dashboard"
)

type Handler struct {
	DashboardService appdashboard.Service
}

func (h Handler) GetDashboardStats(ctx context.Context, organizationID string) (*appdashboard.DashboardStats, error) {
	return h.DashboardService.GetDashboardStats(ctx, organizationID)
}

func (h Handler) GetUserStats(ctx context.Context, organizationID string) (*appdashboard.UserStats, error) {
	return h.DashboardService.GetUserStats(ctx, organizationID)
}

func (h Handler) GetAuthStats(ctx context.Context, organizationID string) (*appdashboard.AuthStats, error) {
	return h.DashboardService.GetAuthStats(ctx, organizationID)
}

func (h Handler) GetResourceStats(ctx context.Context, organizationID string) (*appdashboard.ResourceStats, error) {
	return h.DashboardService.GetResourceStats(ctx, organizationID)
}

func (h Handler) GetProviderDistribution(ctx context.Context, organizationID string) ([]appdashboard.ProviderDistribution, error) {
	return h.DashboardService.GetProviderDistribution(ctx, organizationID)
}

func (h Handler) GetMfaCoverage(ctx context.Context, organizationID string) (*appdashboard.MfaCoverage, error) {
	return h.DashboardService.GetMfaCoverage(ctx, organizationID)
}

func (h Handler) GetLoginHeatmap(ctx context.Context, organizationID string) ([]appdashboard.LoginHeatmapPoint, error) {
	return h.DashboardService.GetLoginHeatmap(ctx, organizationID)
}

func (h Handler) GetTrends(ctx context.Context, organizationID string) ([]appdashboard.TrendDataPoint, error) {
	return h.DashboardService.GetTrends(ctx, organizationID)
}

func (h Handler) GetActivityHeatmap(ctx context.Context, organizationID string) ([]appdashboard.ActivityHeatmapPoint, error) {
	return h.DashboardService.GetActivityHeatmap(ctx, organizationID)
}

func (h Handler) GetOrgUserDistribution(ctx context.Context) ([]appdashboard.OrgUserDistribution, error) {
	return h.DashboardService.GetOrgUserDistribution(ctx)
}
