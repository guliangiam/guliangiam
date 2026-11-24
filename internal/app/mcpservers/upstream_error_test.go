package mcpservers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingMCPProxy struct {
	syncErr error
	callErr error
	calls   int
}

func (p *failingMCPProxy) SyncToolsFromServer(context.Context, string) error { return p.syncErr }
func (p *failingMCPProxy) ListTools(context.Context) []domain.Tool           { return nil }
func (p *failingMCPProxy) CallTool(context.Context, string, string, map[string]any) (any, error) {
	p.calls++
	return nil, p.callErr
}

func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func newProxyTestService(t *testing.T, proxy MCPProxy) Service {
	t.Helper()
	svc := newSvc(&mockMcpServerRepo{mcpServers: []*domain.McpServer{{
		ID: "mcp-1", OrganizationID: "org-1", Name: "srv", IsEnabled: true,
		Tools: []domain.Tool{{Name: "tool", IsAllowed: true}},
	}}})
	svc.MCP = proxy
	return svc
}

func TestProxyRequest_SanitizesURLQueryInPayloadAndLog(t *testing.T) {
	const secret = "super-secret-key"
	logs := captureSlog(t)
	svc := newProxyTestService(t, &failingMCPProxy{
		callErr: fmt.Errorf("POST https://mcp.example.com/tools?key=%s failed: boom", secret),
	})

	result, err := svc.ProxyRequest(context.Background(), "org-1", "srv", "tool", nil, "u1")
	require.NoError(t, err)
	require.False(t, result.Success)

	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), secret)
	assert.NotContains(t, string(encoded), "?key=")
	assert.NotContains(t, logs.String(), secret)
	assert.Equal(t, ErrorCodeMCPToolFailed, result.ErrorCode)
	assert.Equal(t, messageMCPToolFailed, result.Error)
}

func TestProxyRequest_SanitizesHeaderCredentialInPayloadAndLog(t *testing.T) {
	const secret = "super-secret-key"
	logs := captureSlog(t)
	svc := newProxyTestService(t, &failingMCPProxy{
		callErr: fmt.Errorf("upstream rejected x-api-key: %s", secret),
	})

	result, err := svc.ProxyRequest(context.Background(), "org-1", "srv", "tool", nil, "u1")
	require.NoError(t, err)

	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), secret)
	assert.NotContains(t, logs.String(), secret)
}

func TestProxyRequest_TimeoutKeepsErrorChainAndStableCode(t *testing.T) {
	const secret = "super-secret-key"
	upstreamErr := fmt.Errorf("POST https://mcp.example.com/tools?key=%s: %w", secret, context.DeadlineExceeded)

	failure := classifyMCPFailure(mcpOperationTool, upstreamErr)
	assert.Equal(t, apperrors.CodeUnavailable, failure.Code)
	assert.Equal(t, ErrorCodeMCPUpstreamTimeout, failure.ErrorCode)
	assert.Equal(t, messageMCPTimeout, failure.Message)
	assert.True(t, errors.Is(failure.PlatformError(), context.DeadlineExceeded),
		"sanitization must not drop the deadline chain")
	assert.NotContains(t, failure.PlatformError().Error(), secret)

	sanitized := SanitizeToolError(upstreamErr)
	var appErr *apperrors.Error
	require.ErrorAs(t, sanitized, &appErr)
	assert.True(t, errors.Is(sanitized, context.DeadlineExceeded))
	assert.NotContains(t, sanitized.Error(), secret)

	logs := captureSlog(t)
	result, err := newProxyTestService(t, &failingMCPProxy{callErr: upstreamErr}).
		ProxyRequest(context.Background(), "org-1", "srv", "tool", nil, "u1")
	require.NoError(t, err)
	assert.Equal(t, ErrorCodeMCPUpstreamTimeout, result.ErrorCode)
	assert.NotContains(t, logs.String(), secret)
}

func TestSyncTools_SanitizesUpstreamError(t *testing.T) {
	const secret = "super-secret-key"
	logs := captureSlog(t)
	svc := newProxyTestService(t, &failingMCPProxy{
		syncErr: fmt.Errorf("GET https://mcp.example.com/tools?api_key=%s rejected x-api-key: %s", secret, secret),
	})

	result, err := svc.SyncTools(context.Background(), "org-1", "mcp-1")
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)

	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), secret)
	assert.NotContains(t, string(encoded), "api_key=")
	assert.NotContains(t, logs.String(), secret)
	assert.Equal(t, ErrorCodeMCPSyncFailed, result.ErrorCode)
	assert.Equal(t, messageMCPSyncFailed, result.Errors[0])
}

func TestSanitizeToolError_Nil(t *testing.T) {
	assert.NoError(t, SanitizeToolError(nil))
}
