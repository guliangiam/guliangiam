package radiusaccountings

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/radius-accountings",
			OperationID: "ListRadiusAccountings",
			Summary:     "List RADIUS accounting records",
			Permission:  "radiusaccountings:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/radius-accountings",
			OperationID: "CreateRadiusAccounting",
			Summary:     "Create RADIUS accounting record",
			Permission:  "radiusaccountings:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/radius-accountings/{id}",
			OperationID: "GetRadiusAccounting",
			Summary:     "Get RADIUS accounting detail",
			Permission:  "radiusaccountings:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/radius-accountings/{id}",
			OperationID: "PatchRadiusAccounting",
			Summary:     "Patch RADIUS accounting record",
			Permission:  "radiusaccountings:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/radius-accountings/{id}",
			OperationID: "DeleteRadiusAccounting",
			Summary:     "Delete RADIUS accounting record",
			Permission:  "radiusaccountings:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
