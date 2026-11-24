package roles

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/roles",
			OperationID: "ListRoles",
			Summary:     "List roles",
			Permission:  "roles:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/roles",
			OperationID: "CreateRole",
			Summary:     "Create role",
			Permission:  "roles:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/roles/{id}",
			OperationID: "GetRole",
			Summary:     "Get role detail",
			Permission:  "roles:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/roles/{id}",
			OperationID: "PatchRole",
			Summary:     "Patch role",
			Permission:  "roles:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/roles/{id}",
			OperationID: "DeleteRole",
			Summary:     "Delete role",
			Permission:  "roles:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/roles/bulk-import",
			OperationID: "BulkImportRoles",
			Summary:     "Bulk import roles from JSON",
			Permission:  "roles:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/roles/upload",
			OperationID: "UploadRoles",
			Summary:     "Upload roles from Excel file",
			Permission:  "roles:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
