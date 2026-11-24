package metering

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/metering/events",
			OperationID: "ListMeteringEvents",
			Summary:     "List metering events",
			Permission:  "metering:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/metering/aggregates",
			OperationID: "ListMeteringAggregates",
			Summary:     "List pre-rolled metering aggregates",
			Permission:  "metering:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/metering/events/ingest",
			OperationID: "IngestMeteringEvents",
			Summary:     "Ingest a batch of metering events",
			Permission:  "metering:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 202,
		},
		// Missing detail endpoints added for completeness (opt-backend-api)
		{
			Method:      "GET",
			Pattern:     "/api/v1/metering/events/{id}",
			OperationID: "GetMeteringEvent",
			Summary:     "Get a metering event by ID",
			Permission:  "metering:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/metering/aggregates/{id}",
			OperationID: "GetMeteringAggregate",
			Summary:     "Get a metering aggregate by ID",
			Permission:  "metering:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/metering/aggregates/refresh",
			OperationID: "RefreshMeteringAggregate",
			Summary:     "Manually refresh a single (org, period, subject, metric) aggregate cell",
			Permission:  "metering:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
