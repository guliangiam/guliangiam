package system

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/system/health",
			OperationID: "GetSystemHealth",
			Summary:     "Get system health status (legacy liveness alias)",
			Permission:  "public",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/system/live",
			OperationID: "GetSystemLive",
			Summary:     "Liveness probe — pure process-level, 无外部依赖",
			Permission:  "public",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/system/ready",
			OperationID: "GetSystemReady",
			Summary:     "Readiness probe — liveness + DB pool.Ping()",
			Permission:  "system:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/system/info",
			OperationID: "GetSystemInfo",
			Summary:     "Get runtime statistics",
			Permission:  "system:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/system/version",
			OperationID: "GetSystemVersion",
			Summary:     "Get version information",
			Permission:  "system:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/system/metrics",
			OperationID: "GetSystemMetrics",
			Summary:     "Get Prometheus-compatible metrics in JSON format",
			Permission:  "system:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/system/prometheus-info",
			OperationID: "GetPrometheusInfo",
			Summary:     "Get Prometheus scrape configuration",
			Permission:  "system:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/system/cli/{language}",
			OperationID: "DownloadCLI",
			Summary:     "Download CLI binary for the specified language",
			Permission:  "public",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   302,
			ResponseHeaders: []string{"Location", "X-CLI-SHA256", "X-CLI-Filename"},
		},
	}
}
