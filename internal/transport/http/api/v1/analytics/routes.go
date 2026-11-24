package analytics

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the Casbin permission definitions for analytics endpoints.
//
// The endpoint is currently registered inline in bootstrap (see
// registerSystemRoutes) because the RUM ingest handler is a thin stub; this
// declaration exists so the route-contract gate can verify that the public
// contract and the runtime registration stay in sync.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "POST",
			Pattern:     "/api/v1/analytics/web-vitals",
			OperationID: "IngestWebVitals",
			Summary:     "Ingest frontend Web Vitals (RUM) samples",
			Permission:  "public",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 202,
		},
	}
}
