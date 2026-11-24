package mcpservers

import (
	"context"
	"log/slog"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ProxyRequestResult contains the result of proxying a request to an MCP server.
type ProxyRequestResult struct {
	ServerID string `json:"serverId"`
	ToolName string `json:"toolName"`
	Success  bool   `json:"success"`
	Data     any    `json:"data,omitempty"`
	// ErrorCode is a stable in-band enum (mcp_tool_failed / mcp_upstream_timeout
	// …); Error is a fixed, bounded message. Upstream text never enters either.
	ErrorCode string `json:"errorCode,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ProxyRequest forwards a tool invocation request to the appropriate MCP server
// inside the trusted tenant. The caller is the trusted session user; before the
// upstream call the tool must be declared by the server and marked as allowed.
// Unknown or disabled tools are a 403, so the endpoint-level mcpservers.proxy
// permission is not the only boundary.
func (s Service) ProxyRequest(ctx context.Context, organizationID, serverName, toolName string, arguments map[string]any, userID string) (*ProxyRequestResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(userID) == "" {
		return nil, apperrors.NewUnauthorized("authenticated user is required")
	}
	if s.MCP == nil {
		return nil, apperrors.NewInvalidArgument("MCP proxy not configured")
	}

	// Find the server by trusted organization + exact name. The list query
	// already filters by organization_id, so a foreign server looks missing.
	servers, _, err := s.UOW.McpServers().List(ctx, domain.ListQuery{
		PageSize:       100,
		OrganizationID: organizationID,
		Keyword:        serverName,
	})
	if err != nil {
		return nil, err
	}
	server := matchMcpServerByName(servers, serverName)
	if server == nil {
		return nil, apperrors.NewNotFoundf("MCP server not found: %s", serverName)
	}
	if !server.IsEnabled {
		return &ProxyRequestResult{
			ServerID: server.ID,
			ToolName: toolName,
			Success:  false,
			Error:    "server is disabled",
		}, nil
	}

	// Tool-level allowlist: the tool must be part of the server's synced/declared
	// catalogue and carry IsAllowed. A tool that was never declared (or was
	// explicitly disabled) cannot be reached through the tenant-level permission.
	tool, ok := matchMcpToolByName(server.Tools, toolName)
	if !ok || !tool.IsAllowed {
		return nil, apperrors.NewForbidden("tool is not allowed for this server")
	}

	// Call the tool via MCP proxy
	result, err := s.MCP.CallTool(ctx, server.ID, toolName, arguments)
	if err != nil {
		// Tool invocation failures are reported in-band (success=false) so the
		// gateway can relay a stable code. Only the fixed message and the enum
		// leave the process; the raw upstream text stays in the error chain.
		failure := classifyMCPFailure(mcpOperationTool, err)
		slog.WarnContext(ctx, "MCP tool call failed",
			"server_id", server.ID, "tool", toolName,
			"error_code", failure.ErrorCode, "error", failure.Message)
		return &ProxyRequestResult{
			ServerID:  server.ID,
			ToolName:  toolName,
			Success:   false,
			ErrorCode: failure.ErrorCode,
			Error:     failure.Message,
		}, nil
	}

	return &ProxyRequestResult{
		ServerID: server.ID,
		ToolName: toolName,
		Success:  true,
		Data:     result,
	}, nil
}

// matchMcpServerByName returns the exact-name match, never the first fuzzy
// result the keyword search happened to return.
func matchMcpServerByName(servers []*domain.McpServer, serverName string) *domain.McpServer {
	for _, server := range servers {
		if server.Name == serverName {
			return server
		}
	}
	return nil
}

// matchMcpToolByName returns the declared tool by exact name. An empty catalogue
// means no tool is authorized yet, not "all tools are allowed".
func matchMcpToolByName(tools []domain.Tool, toolName string) (domain.Tool, bool) {
	for _, tool := range tools {
		if tool.Name == toolName {
			return tool, true
		}
	}
	return domain.Tool{}, false
}
