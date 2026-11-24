package faceidproviders

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/face-id-providers",
			OperationID: "ListFaceIdProviders",
			Summary:     "List face id providers",
			Permission:  "face-id-providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/face-id-providers",
			OperationID: "CreateFaceIdProvider",
			Summary:     "Create face id provider",
			Permission:  "face-id-providers:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/face-id-providers/{id}",
			OperationID: "GetFaceIdProvider",
			Summary:     "Get face id provider detail",
			Permission:  "face-id-providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/face-id-providers/{id}",
			OperationID: "PatchFaceIdProvider",
			Summary:     "Patch face id provider",
			Permission:  "face-id-providers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/face-id-providers/{id}",
			OperationID: "DeleteFaceIdProvider",
			Summary:     "Delete face id provider",
			Permission:  "face-id-providers:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
