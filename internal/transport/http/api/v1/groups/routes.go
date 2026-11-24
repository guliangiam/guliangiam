package groups

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/groups",
			OperationID: "ListGroups",
			Summary:     "List groups",
			Permission:  "groups:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/groups",
			OperationID: "CreateGroup",
			Summary:     "Create group",
			Permission:  "groups:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/groups/{id}",
			OperationID: "GetGroup",
			Summary:     "Get group detail",
			Permission:  "groups:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/groups/{id}",
			OperationID: "PatchGroup",
			Summary:     "Patch group",
			Permission:  "groups:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/groups/{id}",
			OperationID: "DeleteGroup",
			Summary:     "Delete group",
			Permission:  "groups:delete",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/groups/bulk-import",
			OperationID: "BulkImportGroups",
			Summary:     "Bulk import groups from JSON",
			Permission:  "groups:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/groups/upload",
			OperationID: "UploadGroups",
			Summary:     "Upload groups from Excel file",
			Permission:  "groups:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
