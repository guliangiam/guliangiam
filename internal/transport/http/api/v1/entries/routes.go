package entries

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/entries",
			OperationID: "ListEntries",
			Summary:     "List entries",
			Permission:  "entries:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/entries",
			OperationID: "CreateEntry",
			Summary:     "Create entry",
			Permission:  "entries:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/entries/{id}",
			OperationID: "GetEntry",
			Summary:     "Get entry detail",
			Permission:  "entries:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/entries/{id}",
			OperationID: "PatchEntry",
			Summary:     "Patch entry",
			Permission:  "entries:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/entries/{id}",
			OperationID: "DeleteEntry",
			Summary:     "Delete entry",
			Permission:  "entries:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/entries/{id}/transcript",
			OperationID: "GetEntryTranscript",
			Summary:     "Read an entry's message sequence by append order",
			Permission:  "entries:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/entries/traces",
			OperationID: "AddOtlpTraces",
			Summary:     "Ingest OpenTelemetry traces",
			Permission:  "entries:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/entries/metrics",
			OperationID: "AddOtlpMetrics",
			Summary:     "Ingest OpenTelemetry metrics",
			Permission:  "entries:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/entries/logs",
			OperationID: "AddOtlpLogs",
			Summary:     "Ingest OpenTelemetry logs",
			Permission:  "entries:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
