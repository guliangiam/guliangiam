package actions

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{Method: "GET", Pattern: "/api/v1/actions", OperationID: "ListActions", Summary: "List identity action definitions", Permission: "actions:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/actions", OperationID: "CreateAction", Summary: "Create an identity action definition", Permission: "actions:write", Surface: route.SurfaceManagement, SuccessStatus: 201, ResponseHeaders: []string{"Location"}},
		{Method: "GET", Pattern: "/api/v1/actions/{id}", OperationID: "GetAction", Summary: "Get an action definition by ID", Permission: "actions:read", Surface: route.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/actions/{id}/status", OperationID: "UpdateActionStatus", Summary: "Transition action status", Permission: "actions:write", Surface: route.SurfaceManagement},
		{Method: "GET", Pattern: "/api/v1/actions/{actionId}/versions", OperationID: "ListActionVersions", Summary: "List versions of an action", Permission: "actions:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/actions/{actionId}/versions", OperationID: "PublishActionVersion", Summary: "Publish a new code version for an action", Permission: "actions:write", Surface: route.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/actions/{actionId}/execution-logs", OperationID: "ListActionExecutionLogs", Summary: "List execution logs for an action", Permission: "actions:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/actions/{actionId}/run", OperationID: "RunAction", Summary: "Execute an identity action on demand", Permission: "actions:write", Surface: route.SurfaceManagement, SuccessStatus: 202},
	}
}
