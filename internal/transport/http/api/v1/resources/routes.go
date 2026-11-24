package resources

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/resources",
			OperationID: "ListResources",
			Summary:     "List resources",
			Permission:  "resources:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/resources",
			OperationID: "CreateResource",
			Summary:     "Create resource",
			Permission:  "resources:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/resources/{id}",
			OperationID: "GetResource",
			Summary:     "Get resource detail",
			Permission:  "resources:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/resources/{id}",
			OperationID: "PatchResource",
			Summary:     "Patch resource",
			Permission:  "resources:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/resources/{id}",
			OperationID: "DeleteResource",
			Summary:     "Delete resource",
			Permission:  "resources:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/upload-resource",
			OperationID: "UploadResource",
			Summary:     "Upload resource file",
			Permission:  "resources:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
