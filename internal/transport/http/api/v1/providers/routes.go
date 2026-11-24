package providers

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/providers",
			OperationID: "ListProviders",
			Summary:     "List providers",
			Permission:  "providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/providers",
			OperationID: "CreateProvider",
			Summary:     "Create provider",
			Permission:  "providers:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/providers/{id}",
			OperationID: "GetProvider",
			Summary:     "Get provider detail",
			Permission:  "providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/providers/{id}",
			OperationID: "PatchProvider",
			Summary:     "Patch provider",
			Permission:  "providers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/providers/{id}/config",
			OperationID: "GetProviderConfig",
			Summary:     "Get provider config",
			Permission:  "providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PUT",
			Pattern:     "/api/v1/providers/{id}/config",
			OperationID: "UpdateProviderConfig",
			Summary:     "Update provider config",
			Permission:  "providers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/providers/{id}/secret-status",
			OperationID: "ListProviderSecretStatuses",
			Summary:     "List provider secret statuses",
			Permission:  "providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/providers/{id}",
			OperationID: "DeleteProvider",
			Summary:     "Delete provider",
			Permission:  "providers:delete",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/providers/{id}/test-connection",
			OperationID: "TestProviderConnection",
			Summary:     "Test provider connection",
			Permission:  "providers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/providers/{id}/enable",
			OperationID: "EnableProvider",
			Summary:     "Enable provider after a successful probe",
			Permission:  "providers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/global/providers",
			OperationID: "ListGlobalProviders",
			Summary:     "List providers across all organizations",
			Permission:  "providers:read",
			Surface:     route.SurfaceManagement,
		},
	}
}
