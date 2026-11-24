package records

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes 是 records API 的路由声明（Route Contract 用）。
// 规范化主路径：/api/v1/records；/api/v1/audit/records 保留为向后兼容 307 redirect。
func Routes() []route.Definition {
	return []route.Definition{
		// ── 规范化主路由 ──
		{
			Method:      "POST",
			Pattern:     "/api/v1/records",
			OperationID: "CreateRecord",
			Summary:     "Create audit record",
			Permission:  "record:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/records",
			OperationID: "ListRecords",
			Summary:     "List audit records",
			Permission:  "record:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/records/export",
			OperationID: "ExportRecords",
			Summary:     "Export audit evidence bundle with checksum",
			Permission:  "record:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/records/search",
			OperationID: "SearchRecords",
			Summary:     "Search audit records",
			Permission:  "record:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/records/audit-logs",
			OperationID: "ListAuditLogs",
			Summary:     "List audit logs grouped by category",
			Permission:  "record:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/records/login-logs",
			OperationID: "ListLoginLogs",
			Summary:     "List login logs across sessions and failed attempts",
			Permission:  "record:read",
			Surface:     route.SurfaceManagement,
		},
		// ── 向后兼容 redirect（307）──
		{
			Method:      "POST",
			Pattern:     "/api/v1/audit/records",
			OperationID: "CreateRecord_Legacy",
			Summary:     "(Deprecated) Create record — redirects to /api/v1/records",
			Permission:  "record:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   307,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/audit/records",
			OperationID: "ListRecords_Legacy",
			Summary:     "(Deprecated) List records — redirects to /api/v1/records",
			Permission:  "record:read",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   307,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/audit/records/search",
			OperationID: "SearchRecords_Legacy",
			Summary:     "(Deprecated) Search records — redirects to /api/v1/records/search",
			Permission:  "record:read",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   307,
			ResponseHeaders: []string{"Location"},
		},
	}
}
