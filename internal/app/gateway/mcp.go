package gateway

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) ListMCPTools(ctx context.Context) ([]domain.MCPTool, error) {
	if s.MCP == nil {
		return nil, apperrors.NewUnavailable("MCP proxy is not configured")
	}
	return s.MCP.ListTools(ctx)
}

func (s *Service) CallMCPTool(ctx context.Context, serverID, toolName string, arguments map[string]interface{}) (interface{}, error) {
	if s.MCP == nil {
		return nil, apperrors.NewUnavailable("MCP proxy is not configured")
	}
	return s.MCP.CallTool(ctx, serverID, toolName, arguments)
}

func (s *Service) SyncMCPTools(ctx context.Context) error {
	if s.MCP == nil {
		return apperrors.NewUnavailable("MCP proxy is not configured")
	}
	return s.MCP.SyncAllTools(ctx)
}
