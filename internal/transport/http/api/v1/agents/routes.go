package agents

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/agents",
			OperationID: "ListAgents",
			Summary:     "List agents",
			Permission:  "agents:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/agents",
			OperationID: "CreateAgent",
			Summary:     "Create agent",
			Permission:  "agents:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/agents/{id}",
			OperationID: "GetAgent",
			Summary:     "Get agent detail",
			Permission:  "agents:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/agents/{id}",
			OperationID: "PatchAgent",
			Summary:     "Patch agent",
			Permission:  "agents:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/agents/{id}",
			OperationID: "DeleteAgent",
			Summary:     "Delete agent",
			Permission:  "agents:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
