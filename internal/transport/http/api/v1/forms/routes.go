package forms

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/forms",
			OperationID: "ListForms",
			Summary:     "List forms",
			Permission:  "forms:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/forms",
			OperationID: "CreateForm",
			Summary:     "Create form",
			Permission:  "forms:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/forms/{id}",
			OperationID: "GetForm",
			Summary:     "Get form detail",
			Permission:  "forms:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/forms/{id}",
			OperationID: "PatchForm",
			Summary:     "Patch form",
			Permission:  "forms:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/forms/{id}",
			OperationID: "DeleteForm",
			Summary:     "Delete form",
			Permission:  "forms:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
