package mcpservers

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func proxyServiceWithTools(proxy MCPProxy, tools []domain.Tool) Service {
	svc := newSvc(&mockMcpServerRepo{mcpServers: []*domain.McpServer{{
		ID: "mcp-1", OrganizationID: "org-1", Name: "srv", IsEnabled: true, Tools: tools,
	}}})
	svc.MCP = proxy
	return svc
}

func assertAppErrorCode(t *testing.T, err error, want apperrors.Code) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, want, appErr.Code)
}

// TestProxyRequest_DeniesUndeclaredTool pins that a caller cannot reach a tool
// that the server never declared, even with mcpservers.proxy at the endpoint.
func TestProxyRequest_DeniesUndeclaredTool(t *testing.T) {
	proxy := &failingMCPProxy{}
	svc := proxyServiceWithTools(proxy, []domain.Tool{{Name: "echo", IsAllowed: true}})

	result, err := svc.ProxyRequest(context.Background(), "org-1", "srv", "unknown-tool", nil, "u1")

	require.Nil(t, result)
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
	assert.Zero(t, proxy.calls, "an undeclared tool must not reach the upstream server")
}

// TestProxyRequest_DeniesDisallowedTool pins the Tool.IsAllowed decision.
func TestProxyRequest_DeniesDisallowedTool(t *testing.T) {
	proxy := &failingMCPProxy{}
	svc := proxyServiceWithTools(proxy, []domain.Tool{{Name: "echo", IsAllowed: false}})

	result, err := svc.ProxyRequest(context.Background(), "org-1", "srv", "echo", nil, "u1")

	require.Nil(t, result)
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
	assert.Zero(t, proxy.calls, "a tool marked denied must not reach the upstream server")
}

// TestProxyRequest_AllowsDeclaredTool pins the allowed path.
func TestProxyRequest_AllowsDeclaredTool(t *testing.T) {
	proxy := &failingMCPProxy{}
	svc := proxyServiceWithTools(proxy, []domain.Tool{{Name: "echo", IsAllowed: true}})

	result, err := svc.ProxyRequest(context.Background(), "org-1", "srv", "echo", nil, "u1")

	require.NoError(t, err)
	require.True(t, result.Success)
	assert.Equal(t, "echo", result.ToolName)
	assert.Equal(t, 1, proxy.calls)
}

// TestProxyRequest_RequiresSessionUser pins fail-closed when the caller identity
// is missing; the tool allowlist alone is not a substitute for a caller.
func TestProxyRequest_RequiresSessionUser(t *testing.T) {
	proxy := &failingMCPProxy{}
	svc := proxyServiceWithTools(proxy, []domain.Tool{{Name: "echo", IsAllowed: true}})

	result, err := svc.ProxyRequest(context.Background(), "org-1", "srv", "echo", nil, "  ")

	require.Nil(t, result)
	assertAppErrorCode(t, err, apperrors.CodeUnauthorized)
	assert.Zero(t, proxy.calls)
}
