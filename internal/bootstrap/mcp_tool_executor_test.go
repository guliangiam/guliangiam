package bootstrap

import (
	"context"
	"errors"
	"testing"

	appmcpservers "github.com/guliangiam/guliangiam/internal/app/mcpservers"
	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	mcpserversdomain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── fake MCPProxy ─────────────────────────────────────────────────────

type fakeMCPProxy struct {
	callToolFunc func(ctx context.Context, serverID, toolName string, arguments map[string]any) (any, error)

	// recorded calls for assertion
	callCalls  int
	callServer string
	callTool   string
	callArgs   map[string]any
}

func (f *fakeMCPProxy) SyncToolsFromServer(_ context.Context, _ string) error { return nil }
func (f *fakeMCPProxy) ListTools(_ context.Context) []mcpserversdomain.Tool   { return nil }
func (f *fakeMCPProxy) CallTool(ctx context.Context, serverID, toolName string, arguments map[string]any) (any, error) {
	f.callCalls++
	f.callServer = serverID
	f.callTool = toolName
	f.callArgs = arguments
	if f.callToolFunc != nil {
		return f.callToolFunc(ctx, serverID, toolName, arguments)
	}
	return map[string]any{"ok": true}, nil
}

var _ appmcpservers.MCPProxy = (*fakeMCPProxy)(nil)

// ── mcpToolExecutor tests ────────────────────────────────────────────

func TestMcpToolExecutor_NilBinding(t *testing.T) {
	proxy := &fakeMCPProxy{}
	exec := &mcpToolExecutor{mcp: proxy}

	result, err := exec.ExecuteTool(context.Background(), nil, "ignored", map[string]any{"k": "v"})
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, proxy.callCalls, "MCPProxy.CallTool should not be called when binding is nil")
}

func TestMcpToolExecutor_WithBinding_CorrectArgs(t *testing.T) {
	proxy := &fakeMCPProxy{
		callToolFunc: func(_ context.Context, serverID, toolName string, args map[string]any) (any, error) {
			return map[string]any{"server": serverID, "tool": toolName, "args": args}, nil
		},
	}
	exec := &mcpToolExecutor{mcp: proxy}

	binding := &aigatewaydomain.MCPBinding{
		McpServerID: "mcp-server-abc",
		ToolName:    "search_docs",
	}
	input := map[string]any{"query": "refund policy"}

	result, err := exec.ExecuteTool(context.Background(), binding, "canonicalToolName", input)
	require.NoError(t, err)

	// Verify correct call to MCPProxy — uses binding fields, ignores passed toolName
	assert.Equal(t, 1, proxy.callCalls)
	assert.Equal(t, "mcp-server-abc", proxy.callServer)
	assert.Equal(t, "search_docs", proxy.callTool, "should use binding.ToolName, not the passed parameter")
	assert.Equal(t, input, proxy.callArgs)

	// Verify result passed through
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "mcp-server-abc", m["server"])
	assert.Equal(t, "search_docs", m["tool"])
	assert.Equal(t, input, m["args"])
}

func TestMcpToolExecutor_WithBinding_ErrorPropagation(t *testing.T) {
	proxy := &fakeMCPProxy{
		callToolFunc: func(_ context.Context, _, _ string, _ map[string]any) (any, error) {
			return nil, errors.New("mcp server unreachable")
		},
	}
	exec := &mcpToolExecutor{mcp: proxy}

	binding := &aigatewaydomain.MCPBinding{
		McpServerID: "mcp-server-down",
		ToolName:    "any_tool",
	}

	result, err := exec.ExecuteTool(context.Background(), binding, "any", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "mcp server unreachable")
	assert.Equal(t, 1, proxy.callCalls)
}
