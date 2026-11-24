package models

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/models",
			OperationID: "ListModels",
			Summary:     "List Casbin models",
			Permission:  "models:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/models",
			OperationID: "CreateModel",
			Summary:     "Create Casbin model",
			Permission:  "models:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/models/{id}",
			OperationID: "GetModel",
			Summary:     "Get Casbin model detail",
			Permission:  "models:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/models/{id}",
			OperationID: "PatchModel",
			Summary:     "Patch Casbin model",
			Permission:  "models:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/models/{id}",
			OperationID: "DeleteModel",
			Summary:     "Delete Casbin model",
			Permission:  "models:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
