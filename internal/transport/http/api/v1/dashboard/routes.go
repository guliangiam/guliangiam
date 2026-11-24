package dashboard

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/stats",
			OperationID: "GetDashboardStats",
			Summary:     "Get dashboard statistics",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/users",
			OperationID: "GetDashboardUsers",
			Summary:     "Get dashboard user statistics",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/auth",
			OperationID: "GetDashboardAuth",
			Summary:     "Get dashboard authentication statistics",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/resources",
			OperationID: "GetDashboardResources",
			Summary:     "Get dashboard resource statistics",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/provider-distribution",
			OperationID: "GetDashboardProviderDistribution",
			Summary:     "Get provider distribution for charts",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/mfa-coverage",
			OperationID: "GetDashboardMfaCoverage",
			Summary:     "Get MFA coverage statistics",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/login-heatmap",
			OperationID: "GetDashboardLoginHeatmap",
			Summary:     "Get login heatmap data",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/trends",
			OperationID: "GetDashboardTrends",
			Summary:     "Get 30-day trend data",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/activity-heatmap",
			OperationID: "GetDashboardActivityHeatmap",
			Summary:     "Get activity heatmap data",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/dashboard/org-user-distribution",
			OperationID: "GetDashboardOrgUserDistribution",
			Summary:     "Get organization user distribution",
			Permission:  "dashboard:read",
			Surface:     route.SurfaceManagement,
		},
	}
}
