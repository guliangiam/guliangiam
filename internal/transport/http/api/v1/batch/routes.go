package batch

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the Casbin permission definitions for batch endpoints. The
// runtime registration lives in chi.go; these declarations are the contract
// consumed by the route-contract gate and the OpenAPI generator.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "POST",
			Pattern:     "/api/v1/batch/users",
			OperationID: "BatchUsers",
			Summary:     "Batch operations on users",
			Permission:  "users:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/batch/organizations",
			OperationID: "BatchOrganizations",
			Summary:     "Batch operations on organizations",
			Permission:  "organizations:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/batch/applications",
			OperationID: "BatchApplications",
			Summary:     "Batch operations on applications",
			Permission:  "applications:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
