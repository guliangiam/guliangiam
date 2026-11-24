package keys

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/keys",
			OperationID: "ListKeys",
			Summary:     "List keys",
			Permission:  "keys:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/keys",
			OperationID: "CreateKey",
			Summary:     "Create key",
			Permission:  "keys:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/keys/{id}",
			OperationID: "GetKey",
			Summary:     "Get key detail",
			Permission:  "keys:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/keys/{id}",
			OperationID: "PatchKey",
			Summary:     "Patch key",
			Permission:  "keys:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/keys/{id}",
			OperationID: "DeleteKey",
			Summary:     "Delete key",
			Permission:  "keys:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/global/keys",
			OperationID: "ListGlobalKeys",
			Summary:     "List global keys",
			Permission:  "keys:read",
			Surface:     route.SurfaceManagement,
		},
	}
}
