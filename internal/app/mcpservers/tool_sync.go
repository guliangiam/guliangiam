package mcpservers

import (
	"context"
	"log/slog"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SyncToolsResult contains the result of syncing tools from an MCP server.
type SyncToolsResult struct {
	ServerID  string   `json:"serverId"`
	ToolCount int      `json:"toolCount"`
	Tools     []string `json:"tools"`
	// ErrorCode mirrors the stable in-band enum of ProxyRequestResult;
	// Errors carries only the fixed, bounded message.
	ErrorCode string   `json:"errorCode,omitempty"`
	Errors    []string `json:"errors,omitempty"`
}

// SyncTools discovers and syncs the tool list from an MCP server inside the
// trusted tenant. Ownership is resolved before the upstream call so a foreign
// server id never reaches the MCP proxy or the database write.
func (s Service) SyncTools(ctx context.Context, organizationID, serverID string) (*SyncToolsResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if _, err := s.UOW.McpServers().GetByIDForOrganization(ctx, serverID, organizationID); err != nil {
		return nil, err
	}
	if s.MCP == nil {
		return nil, apperrors.NewInvalidArgument("MCP proxy not configured")
	}

	// Sync tools from the remote server via MCP proxy
	if err := s.MCP.SyncToolsFromServer(ctx, serverID); err != nil {
		// Surfaced in-band so the UI can show a partial-sync result. The raw
		// upstream text stays in the error chain; only code+message are emitted.
		failure := classifyMCPFailure(mcpOperationSync, err)
		slog.WarnContext(ctx, "MCP tool sync failed",
			"server_id", serverID, "error_code", failure.ErrorCode, "error", failure.Message)
		return &SyncToolsResult{
			ServerID:  serverID,
			ErrorCode: failure.ErrorCode,
			Errors:    []string{failure.Message},
		}, nil
	}

	// Get the updated tool list from the proxy
	allTools := s.MCP.ListTools(ctx)
	var toolNames []string
	for _, t := range allTools {
		toolNames = append(toolNames, t.Name)
	}

	// Persist in a transaction: the scoped read re-checks ownership and the
	// UPDATE is predicated on the entity's real organization.
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		server, err := tx.McpServers().GetByIDForOrganization(ctx, serverID, organizationID)
		if err != nil {
			return err
		}
		server.Tools = allTools
		server.UpdatedAt = s.Clock.Now()
		server.Revision++
		return tx.McpServers().Update(ctx, server)
	})
	if err != nil {
		return nil, err
	}

	return &SyncToolsResult{
		ServerID:  serverID,
		ToolCount: len(toolNames),
		Tools:     toolNames,
	}, nil
}
