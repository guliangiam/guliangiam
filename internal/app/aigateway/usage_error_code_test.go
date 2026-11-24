// usage_error_code_test.go — persisted usage error codes stay normalized.
package aigateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatewayService_UsageErrorCode_IsNormalizedAndBounded(t *testing.T) {
	const providerKey = "gemini-key-abc"
	raw := `gemini: request failed: Post "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro:generateContent?key=` +
		providerKey + `": ` + strings.Repeat("detail ", 40)

	svc, _, upstream, usageLogs := makeService(t)
	outbox := &mockOutboxRepo{}
	svc.Outbox = outbox
	upstream.chatErr = apperrors.NewUnavailablef("%s", raw)

	_, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	require.Error(t, err)
	require.Equal(t, 1, usageLogs.calls)
	require.NotNil(t, usageLogs.last)
	require.NotNil(t, usageLogs.last.ErrorCode)

	// llm_usage_logs.error_code is VARCHAR(100); raw text is never stored.
	assert.Equal(t, usageErrorCodeUpstream, *usageLogs.last.ErrorCode)
	assert.LessOrEqual(t, len(*usageLogs.last.ErrorCode), usageErrorCodeMaxLen)
	assert.NotContains(t, *usageLogs.last.ErrorCode, providerKey)
	assert.NotContains(t, *usageLogs.last.ErrorCode, "key=")

	evt := findOutboxEvent(outbox, "llm_usage", "ai.usage.recorded")
	require.NotNil(t, evt)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(evt.Payload, &payload))
	assert.Equal(t, usageErrorCodeUpstream, payload["error_code"])
	assert.NotContains(t, string(evt.Payload), providerKey)
	assert.NotContains(t, string(evt.Payload), "key=")
}

func TestGatewayService_UsageErrorCode_TimeoutIsPreserved(t *testing.T) {
	svc, _, upstream, usageLogs := makeService(t)
	outbox := &mockOutboxRepo{}
	svc.Outbox = outbox
	upstream.chatErr = apperrors.Wrap(apperrors.CodeUnavailable, context.DeadlineExceeded, "gemini: request failed")

	_, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	require.Error(t, err)
	require.NotNil(t, usageLogs.last)
	require.NotNil(t, usageLogs.last.ErrorCode)
	assert.Equal(t, usageErrorCodeUpstreamTimeout, *usageLogs.last.ErrorCode)

	evt := findOutboxEvent(outbox, "llm_usage", "ai.usage.recorded")
	require.NotNil(t, evt)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(evt.Payload, &payload))
	assert.Equal(t, usageErrorCodeUpstreamTimeout, payload["error_code"])
}
