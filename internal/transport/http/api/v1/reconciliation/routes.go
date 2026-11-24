package reconciliation

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes declares the management endpoint for route/OpenAPI governance.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/reconciliation/scim-reports",
			OperationID: "ListReconciliationScimReports",
			Summary:     "List persisted SCIM outbound reconciliation reports",
			Permission:  "scim.users",
			Surface:     route.SurfaceManagement,
		},
	}
}
