package adapters

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/adapters",
			OperationID: "ListAdapters",
			Summary:     "List Casbin adapters",
			Permission:  "adapters:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/adapters",
			OperationID: "CreateAdapter",
			Summary:     "Create Casbin adapter",
			Permission:  "adapters:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/adapters/{id}",
			OperationID: "GetAdapter",
			Summary:     "Get Casbin adapter detail",
			Permission:  "adapters:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/adapters/{id}",
			OperationID: "PatchAdapter",
			Summary:     "Patch Casbin adapter",
			Permission:  "adapters:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/adapters/{id}",
			OperationID: "DeleteAdapter",
			Summary:     "Delete Casbin adapter",
			Permission:  "adapters:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/adapters/{id}/test-connection",
			OperationID: "TestAdapterConnection",
			Summary:     "Test adapter database connection",
			Permission:  "adapters:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/adapters/{id}/policies",
			OperationID: "GetAdapterPolicies",
			Summary:     "Get policies from adapter",
			Permission:  "adapters:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/adapters/{id}/policies",
			OperationID: "AddAdapterPolicy",
			Summary:     "Add policy to adapter",
			Permission:  "adapters:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/adapters/{id}/policies",
			OperationID: "RemoveAdapterPolicy",
			Summary:     "Remove policy from adapter",
			Permission:  "adapters:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PUT",
			Pattern:     "/api/v1/adapters/{id}/policies",
			OperationID: "UpdateAdapterPolicy",
			Summary:     "Update policy in adapter",
			Permission:  "adapters:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
