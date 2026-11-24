package aigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type secretToolExecutor struct{ err error }

func (e secretToolExecutor) ExecuteTool(context.Context, *domain.MCPBinding, string, map[string]any) (any, error) {
	return nil, e.err
}

func captureToolSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func TestExecuteTools_SanitizesToolResultAndLog(t *testing.T) {
	const secret = "super-secret-key"
	logs := captureToolSlog(t)
	exec := secretToolExecutor{err: fmt.Errorf(
		"POST https://mcp.example.com/tools?key=%s rejected x-api-key: %s", secret, secret)}
	bindings := []domain.CanonicalTool{
		{Name: "leaky_tool", MCPTarget: &domain.MCPBinding{ServerName: "bad-server"}},
	}
	content := []domain.CanonicalContentBlock{
		{Type: domain.ContentBlockToolUse, ToolName: "leaky_tool", ToolCallID: "tc-leak"},
	}

	out := ExecuteTools(context.Background(), exec, content, bindings, "", "")

	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), secret)
	assert.NotContains(t, logs.String(), secret)

	require.Len(t, out, 1)
	result, ok := out[0].ToolResult.(map[string]any)
	require.True(t, ok)
	detail, ok := result["error"].(string)
	require.True(t, ok)
	assert.NotContains(t, detail, secret)
	assert.NotContains(t, detail, "?key=")
}

func TestToolErrorText_PrefersTypedMessage(t *testing.T) {
	typed := apperrors.New(apperrors.CodeUnavailable, "mcp upstream timeout")
	assert.Equal(t, "mcp upstream timeout", toolErrorText(typed))
	assert.Empty(t, toolErrorText(nil))
}
