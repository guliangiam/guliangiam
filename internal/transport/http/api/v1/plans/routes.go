package plans

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{Method: "GET", Pattern: "/api/v1/plans", OperationID: "ListPlans", Summary: "List plans", Permission: "plans:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/plans", OperationID: "CreatePlan", Summary: "Create a plan", Permission: "plans:write", Surface: route.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/plans/{id}", OperationID: "GetPlan", Summary: "Get a plan by ID", Permission: "plans:read", Surface: route.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/plans/{id}", OperationID: "PatchPlan", Summary: "Patch a plan", Permission: "plans:write", Surface: route.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/plans/{id}", OperationID: "DeletePlan", Summary: "Delete a plan", Permission: "plans:admin", Surface: route.SurfaceManagement},
	}
}
