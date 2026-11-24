package radius

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "POST",
			Pattern:     "/api/v1/radius/start",
			OperationID: "StartRadiusServer",
			Summary:     "Start RADIUS server",
			Permission:  "radius:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/radius/stop",
			OperationID: "StopRadiusServer",
			Summary:     "Stop RADIUS server",
			Permission:  "radius:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/radius/status",
			OperationID: "GetRadiusServerStatus",
			Summary:     "Get RADIUS server status",
			Permission:  "radius:read",
			Surface:     route.SurfaceManagement,
		},
	}
}
