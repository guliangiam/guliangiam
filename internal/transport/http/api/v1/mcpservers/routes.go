package mcpservers

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/mcp-servers",
			OperationID: "ListMcpServers",
			Summary:     "List MCP servers",
			Permission:  "mcpservers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/mcp-servers",
			OperationID: "CreateMcpServer",
			Summary:     "Create MCP server",
			Permission:  "mcpservers:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/mcp-servers/{id}",
			OperationID: "GetMcpServer",
			Summary:     "Get MCP server detail",
			Permission:  "mcpservers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/mcp-servers/{id}",
			OperationID: "PatchMcpServer",
			Summary:     "Patch MCP server",
			Permission:  "mcpservers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/mcp-servers/{id}",
			OperationID: "DeleteMcpServer",
			Summary:     "Delete MCP server",
			Permission:  "mcpservers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/mcp-servers/online",
			OperationID: "GetOnlineMcpServers",
			Summary:     "Get online MCP servers",
			Permission:  "mcpservers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/mcp-servers/sync-intranet",
			OperationID: "SyncIntranetMcpServers",
			Summary:     "Sync intranet MCP servers",
			Permission:  "mcpservers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/mcp-servers/{id}/sync-tools",
			OperationID: "SyncMcpServerTools",
			Summary:     "Sync MCP server tools",
			Permission:  "mcpservers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/mcp-servers/{id}/access-token",
			OperationID: "GetMcpServerAccessToken",
			Summary:     "Get MCP server access token",
			Permission:  "mcpservers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/mcp-servers/{orgSlug}/{serverName}/proxy",
			OperationID: "ProxyMcpServer",
			Summary:     "Proxy request to MCP server",
			Permission:  "mcpservers.proxy",
			Surface:     route.SurfaceManagement,
		},
	}
}
