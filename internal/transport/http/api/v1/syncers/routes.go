package syncers

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the Casbin permission definitions for syncer endpoints.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/syncers",
			OperationID: "ListSyncers",
			Summary:     "List syncers",
			Permission:  "syncers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/syncers",
			OperationID: "CreateSyncer",
			Summary:     "Create a syncer",
			Permission:  "syncers:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/syncers/{id}",
			OperationID: "GetSyncer",
			Summary:     "Get a syncer by ID",
			Permission:  "syncers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/syncers/{id}",
			OperationID: "PatchSyncer",
			Summary:     "Patch a syncer",
			Permission:  "syncers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/syncers/{id}",
			OperationID: "DeleteSyncer",
			Summary:     "Delete a syncer",
			Permission:  "syncers:admin",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/syncers/{id}/run",
			OperationID: "RunSyncer",
			Summary:     "Manually run a syncer",
			Permission:  "syncers.run",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/syncers/{id}/test-connection",
			OperationID: "TestSyncerConnection",
			Summary:     "Test syncer connection",
			Permission:  "syncers.run",
			Surface:     route.SurfaceManagement,
		},
	}
}
