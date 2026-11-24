package rules

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/rules",
			OperationID: "ListRules",
			Summary:     "List rules",
			Permission:  "rules:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/rules",
			OperationID: "CreateRule",
			Summary:     "Create rule",
			Permission:  "rules:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/rules/{id}",
			OperationID: "GetRule",
			Summary:     "Get rule detail",
			Permission:  "rules:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/rules/{id}",
			OperationID: "PatchRule",
			Summary:     "Patch rule",
			Permission:  "rules:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/rules/{id}",
			OperationID: "DeleteRule",
			Summary:     "Delete rule",
			Permission:  "rules:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/rules/validate",
			OperationID: "ValidateRule",
			Summary:     "Validate a rule",
			Permission:  "rules:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
