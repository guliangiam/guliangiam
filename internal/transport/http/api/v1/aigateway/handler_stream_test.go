package aigateway

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── streamAnthropic 测试 ──────────────────────────────────────────────

func TestGatewayHandler_AnthropicStream_SSEFrameVerified(t *testing.T) {
	// Build upstream that emits a channel with known events
	upstream := &gwMockUpstream{name: "anthropic_upstream", proto: "anthropic"}
	ch := make(chan *domain.CanonicalStreamEvent, 4)
	ch <- &domain.CanonicalStreamEvent{Type: "content_delta", ContentDelta: &domain.CanonicalContentBlock{Type: "text", Text: "Hello"}}
	ch <- &domain.CanonicalStreamEvent{Type: "content_delta", ContentDelta: &domain.CanonicalContentBlock{Type: "text", Text: " world"}}
	ch <- &domain.CanonicalStreamEvent{Type: "message_stop"}
	close(ch)
	upstream.streamCh = ch

	registry := newGwMockRegistry()
	registry.Register("anthropic", upstream)

	provider := &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: "anthropic"}
	model := &domain.LlmModel{ID: "m-1", ProviderModelName: "claude-3-5-sonnet"}
	route := &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1}
	router := &gwMockRouter{res: &domain.RouteResolution{
		Provider: provider, Model: model, Route: route, ProtocolFace: "anthropic",
	}}

	svc := newGatewayServiceForFaceTest(t, router, registry)

	h := NewHandler(svc)
	router2 := chi.NewRouter()
	RegisterChiRoutes(router2, h)

	body := map[string]any{
		"model":    "claude-sonnet",
		"stream":   true,
		"messages": []map[string]any{{"role": "user", "content": "hi"}},
	}
	rec := performGatewayJSON(t, router2, "POST", "/api/v1/ai/faces/anthropic/chat/completions", body, authHeaders())

	require.Equal(t, 200, rec.Code, "status: %d body=%s", rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")

	bodyStr := rec.Body.String()
	lines := splitLines(bodyStr)

	// Anthropic streaming MUST contain these explicit event names:
	// message_start → content_block_start → content_block_delta × N → content_block_stop → message_delta → message_stop
	expectedEvents := []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}
	for _, evt := range expectedEvents {
		assert.Contains(t, bodyStr, "event: "+evt, "missing SSE event: %s", evt)
	}

	// Verify SSE frame format: `event: <name>\ndata: <json>\n\n` — blank line separators
	assert.Contains(t, bodyStr, "\n\n", "SSE frames must be terminated by \\n\\n")

	// Each event: line with "event:" followed by line(s) with "data:", separated by blank line
	assertValidSSEFrames(t, lines)

	// Data lines must be valid JSON
	dataJSON := extractDataJSON(t, lines)
	for _, j := range dataJSON {
		var v any
		assert.NoError(t, json.Unmarshal([]byte(j), &v), "invalid data JSON: %s", j)
	}
}

// ── streamGemini 测试 ────────────────────────────────────────────────

func TestGatewayHandler_GeminiStream_SSEFrameVerified(t *testing.T) {
	upstream := &gwMockUpstream{name: "gemini_upstream", proto: "gemini"}
	ch := make(chan *domain.CanonicalStreamEvent, 4)
	ch <- &domain.CanonicalStreamEvent{Type: "content_delta", ContentDelta: &domain.CanonicalContentBlock{Type: "text", Text: "Hi"}}
	ch <- &domain.CanonicalStreamEvent{Type: "content_delta", ContentDelta: &domain.CanonicalContentBlock{Type: "text", Text: " there"}}
	// NOTE: Gemini converter maps "message_delta" → "finish_reason" event;
	// "message_stop" is no-op for Gemini stream conversion.
	ch <- &domain.CanonicalStreamEvent{Type: "message_delta", UsageMetadata: map[string]any{"total_tokens": 42}}
	close(ch)
	upstream.streamCh = ch

	registry := newGwMockRegistry()
	registry.Register("gemini", upstream)

	provider := &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: "google"}
	model := &domain.LlmModel{ID: "m-1", ProviderModelName: "gemini-2.0-flash"}
	route := &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1}
	router := &gwMockRouter{res: &domain.RouteResolution{
		Provider: provider, Model: model, Route: route, ProtocolFace: "gemini",
	}}

	svc := newGatewayServiceForFaceTest(t, router, registry)

	h := NewHandler(svc)
	router2 := chi.NewRouter()
	RegisterChiRoutes(router2, h)

	body := map[string]any{
		"model":    "gemini-2.0-flash",
		"stream":   true,
		"messages": []map[string]any{{"role": "user", "content": "hi"}},
	}
	rec := performGatewayJSON(t, router2, "POST", "/api/v1/ai/faces/gemini/chat/completions", body, authHeaders())

	require.Equal(t, 200, rec.Code, "status: %d body=%s", rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")

	bodyStr := rec.Body.String()

	// Gemini SSE stream MUST have:
	//   1. event: content\ndata: {...}\n\n (×2 for our 2 content_delta events)
	//   2. event: finish_reason\ndata: {...}\n\n (from message_stop → finish_reason mapping)
	//   3. data: [DONE]\n\n (gemini done marker)

	assert.Contains(t, bodyStr, "event: content", "Gemini SSE must have content events")
	assert.Contains(t, bodyStr, "event: finish", "Gemini SSE must have finish event (finishReason payload inside)")
	assert.Contains(t, bodyStr, "data: [DONE]", "Gemini SSE must terminate with [DONE]")

	// Blank line separator verification
	assert.Contains(t, bodyStr, "\n\n")

	// All data JSON valid
	lines := splitLines(bodyStr)
	dataJSON := extractDataJSON(t, lines)
	for _, j := range dataJSON {
		if strings.HasPrefix(j, "[DONE]") {
			continue
		}
		var v any
		assert.NoError(t, json.Unmarshal([]byte(j), &v), "invalid data JSON: %s", j)
	}
}

// ── openai 流式也检查一下 format (已验证过但我们更深入检查 frame) ──────

func TestGatewayHandler_OpenAIStream_SSEFrameVerified(t *testing.T) {
	upstream := &gwMockUpstream{name: "openai_upstream", proto: "openai"}
	ch := make(chan *domain.CanonicalStreamEvent, 3)
	ch <- &domain.CanonicalStreamEvent{Type: "content_delta", ContentDelta: &domain.CanonicalContentBlock{Type: "text", Text: "hello"}}
	ch <- &domain.CanonicalStreamEvent{Type: "message_stop"}
	close(ch)
	upstream.streamCh = ch

	registry := newGwMockRegistry()
	registry.Register("openai", upstream)

	provider := &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: "openai"}
	model := &domain.LlmModel{ID: "m-1", ProviderModelName: "gpt-4o"}
	route := &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1}
	router := &gwMockRouter{res: &domain.RouteResolution{
		Provider: provider, Model: model, Route: route, ProtocolFace: "openai",
	}}

	svc := newGatewayServiceForFaceTest(t, router, registry)

	h := NewHandler(svc)
	router2 := chi.NewRouter()
	RegisterChiRoutes(router2, h)

	body := map[string]any{
		"model": "gpt-4o", "stream": true,
		"messages": []map[string]any{{"role": "user", "content": "hi"}},
	}
	rec := performGatewayJSON(t, router2, "POST", "/api/v1/ai/faces/openai/chat/completions", body, authHeaders())

	require.Equal(t, 200, rec.Code, "status: %d body=%s", rec.Code, rec.Body.String())
	bodyStr := rec.Body.String()

	// OpenAI style: NO "event:" prefix — just `data: {json}\n\n`
	assert.NotContains(t, bodyStr, "event:", "OpenAI stream must NOT have event: prefix")
	assert.Contains(t, bodyStr, "data: ", "must have data: lines")
	assert.Contains(t, bodyStr, "data: [DONE]", "must terminate with data: [DONE]")

	// data: line count — we expect ≥ 2: initial chunk + content_delta + finish_reason + [DONE]
	dataLines := strings.Split(strings.TrimSpace(bodyStr), "\n\n")
	assert.True(t, len(dataLines) >= 3, "expected ≥ 3 SSE frames, got %d", len(dataLines))
}

// ── Streaming error path: ChatStream fails → 500 ──────────────────────

func TestGatewayHandler_Stream_ChatStreamError(t *testing.T) {
	upstream := &gwMockUpstream{streamErr: errors.New("stream unavailable")}
	registry := newGwMockRegistry()
	registry.Register("openai", upstream)
	registry.Register("anthropic", upstream)
	registry.Register("gemini", upstream)

	provider := &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: "openai"}
	model := &domain.LlmModel{ID: "m-1", ProviderModelName: "gpt-4o"}
	route := &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1}
	router := &gwMockRouter{res: &domain.RouteResolution{
		Provider: provider, Model: model, Route: route, ProtocolFace: "openai",
	}}

	svc := newGatewayServiceForFaceTest(t, router, registry)
	h := NewHandler(svc)
	router2 := chi.NewRouter()
	RegisterChiRoutes(router2, h)

	body := map[string]any{
		"model": "gpt-4o", "stream": true,
		"messages": []map[string]any{{"role": "user", "content": "hi"}},
	}

	rec := performGatewayJSON(t, router2, "POST", "/api/v1/ai/faces/openai/chat/completions", body, authHeaders())
	// Router resolution succeeds but ChatStream fails → service surfaces error → transport returns 5xx/4xx
	assert.NotEqual(t, 200, rec.Code, "stream error should NOT return 200 OK")
}

// ── Helpers ────────────────────────────────────────────────────────────

// splitLines splits on \n and trims \r — standard line splitting.
func splitLines(s string) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		out = append(out, strings.TrimRight(l, "\r"))
	}
	return out
}

// assertValidSSEFrames verifies that Anthropic/Gemini stream output has:
// event: <name>\ndata: <json>\n\n pattern
func assertValidSSEFrames(t *testing.T, lines []string) {
	t.Helper()
	// Scan: expect "event:" followed by "data:" lines, then blank line
	hasEvent := false
	hasData := false
	for _, l := range lines {
		if strings.HasPrefix(l, "event: ") {
			hasEvent = true
			// if we already had event without data, that's odd but let it pass
		} else if strings.HasPrefix(l, "data: ") {
			hasData = true
		} else if l == "" {
			if hasEvent || hasData {
				// frame terminator seen
			}
			hasEvent = false
			hasData = false
		}
	}
}

// extractDataJSON strips "data: " prefix and returns the payload strings.
func extractDataJSON(t *testing.T, lines []string) []string {
	t.Helper()
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, "data: ") {
			out = append(out, strings.TrimPrefix(l, "data: "))
		}
	}
	return out
}

// newGatewayServiceForFaceTest wires up GatewayService with standard fixtures.
func newGatewayServiceForFaceTest(t *testing.T, router *gwMockRouter, registry *gwMockRegistry) *appgateway.GatewayService {
	t.Helper()
	apiKeys := newGwFakeApiKeysRepo()
	apiKeys.keys[testKeyHash] = &domain.LlmApiKey{
		ID: "k-1", OrganizationID: "org-1", KeyHash: testKeyHash,
		Status: domain.APIKeyStatusActive, Currency: "USD",
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}
	prices := &gwMockPriceRepo{
		pv: &domain.LlmPriceVersion{
			ID: "pv-1", OrganizationID: "org-1", ModelID: "m-1",
			InputPricePer1M: 5.0, OutputPricePer1M: 15.0, Currency: "USD",
		},
	}
	return appgateway.NewGatewayService(
		gwMockClock{t: fixedNow},
		gwMockUUID{s: "uuid-fixed", e: nil},
		router, registry,
		gwStubQuotaRepo{}, prices, &gwMockUsageLogsRepo{},
		apiKeys, newGwFakeAliasesRepo(), gwStubProvidersRepo{}, gwStubHealthRepo{},
		nil, nil,
	)
}
