package radiusconfigs

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/radius-configs",
			OperationID: "ListRadiusConfigs",
			Summary:     "List radius configs",
			Permission:  "radius-configs:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/radius-configs",
			OperationID: "CreateRadiusConfig",
			Summary:     "Create radius config",
			Permission:  "radius-configs:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/radius-configs/{id}",
			OperationID: "GetRadiusConfig",
			Summary:     "Get radius config detail",
			Permission:  "radius-configs:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/radius-configs/{id}",
			OperationID: "PatchRadiusConfig",
			Summary:     "Patch radius config",
			Permission:  "radius-configs:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/radius-configs/{id}",
			OperationID: "DeleteRadiusConfig",
			Summary:     "Delete radius config",
			Permission:  "radius-configs:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
