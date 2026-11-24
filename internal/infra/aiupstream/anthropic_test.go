package aiupstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Fixture ────────────────────────────────────────────────────────────

func newAnthropicFixture(t *testing.T, handlers map[string]http.HandlerFunc) (adapter *AnthropicMessagesAdapter, captured map[string]string) {
	t.Helper()
	captured = make(map[string]string)
	mux := http.NewServeMux()
	for path, h := range handlers {
		p := path
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			captured[p] = string(body)
			h(w, r)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewAnthropicMessages(srv.URL, "anthropic-key-xyz", 2*time.Second), captured
}

func baseAnthropicRequest() *domain.CanonicalRequest {
	return &domain.CanonicalRequest{
		ModelAlias: "claude-3-5-sonnet-20241022",
		Messages: []domain.CanonicalMessage{
			{Role: domain.RoleUser, Content: []domain.CanonicalContentBlock{
				{Type: domain.ContentBlockText, Text: "hello"},
			}},
		},
	}
}

// ── Chat happy path ───────────────────────────────────────────────────

func TestAnthropicMessagesAdapter_Chat_HappyPath(t *testing.T) {
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": "msg_abc123",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "hi there"}],
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 3}
			}`))
		},
	})

	resp, err := a.Chat(context.Background(), baseAnthropicRequest())
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "msg_abc123", resp.UpstreamRequestID)
	assert.Equal(t, "end_turn", resp.FinishReason)
	assert.Equal(t, 5, resp.InputTokens)
	assert.Equal(t, 3, resp.OutputTokens)
	assert.Equal(t, 8, resp.TotalTokens) // Anthropic sums input+output
	require.Len(t, resp.Content, 1)
	assert.Equal(t, "hi there", resp.Content[0].Text)
}

// ── Chat upstream errors ───────────────────────────────────────────────

func TestAnthropicMessagesAdapter_Chat_Upstream401(t *testing.T) {
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid api key","type":"authentication_error"}}`))
		},
	})
	_, err := a.Chat(context.Background(), baseAnthropicRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnauthorized, ae.Code)
}

func TestAnthropicMessagesAdapter_Chat_Upstream429(t *testing.T) {
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limit","type":"rate_limit_exceeded"}}`))
		},
	})
	_, err := a.Chat(context.Background(), baseAnthropicRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeTooManyRequests, ae.Code)
}

// ── SSE streaming (Anthropic event: prefix format) ─────────────────────

func TestAnthropicMessagesAdapter_ChatStream_SSE(t *testing.T) {
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			// Anthropic SSE: event:\ndata:\n\n format with blank-line separators
			payloads := []string{
				"event: message_start\n" +
					`data: {"type":"message_start","message":{"id":"msg_x"}}` + "\n\n",
				"event: content_block_delta\n" +
					`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hi"}}` + "\n\n",
				"event: content_block_delta\n" +
					`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" there"}}` + "\n\n",
				"event: message_delta\n" +
					`data: {"type":"message_delta","usage":{"input_tokens":3,"output_tokens":5}}` + "\n\n",
				"event: message_stop\n" +
					`data: {"type":"message_stop"}` + "\n\n",
			}
			for _, p := range payloads {
				_, _ = w.Write([]byte(p))
			}
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := a.ChatStream(ctx, baseAnthropicRequest())
	require.NoError(t, err)

	var events []*domain.CanonicalStreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	require.Len(t, events, 5)
	assert.Equal(t, "message_start", events[0].Type)
	assert.Equal(t, "content_delta", events[1].Type)
	assert.Equal(t, "hi", events[1].ContentDelta.Text)
	assert.Equal(t, "content_delta", events[2].Type)
	assert.Equal(t, " there", events[2].ContentDelta.Text)
	assert.Equal(t, "message_delta", events[3].Type)
	assert.Equal(t, 8, events[3].UsageMetadata["total_tokens"])
	assert.Equal(t, "message_stop", events[4].Type)
}

// ── Embedding returns Unimplemented ────────────────────────────────────

func TestAnthropicMessagesAdapter_Embedding_Unimplemented(t *testing.T) {
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{})

	_, err := a.Embedding(context.Background(), baseAnthropicRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnimplemented, ae.Code)
}

// ── Request shape: x-api-key + anthropic-version headers ────────────────

func TestAnthropicMessagesAdapter_Chat_UsesAnthropicHeaders(t *testing.T) {
	var capturedReq *http.Request
	a, captured := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, r *http.Request) {
			capturedReq = r
			_, _ = w.Write([]byte(`{
				"id": "msg_hdr",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "ok"}],
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 1, "output_tokens": 1}
			}`))
		},
	})

	req := baseAnthropicRequest()
	req.System = "You are helpful."
	req.MaxTokens = 512
	_, _ = a.Chat(context.Background(), req)

	require.NotNil(t, capturedReq)
	assert.Equal(t, "/v1/messages", capturedReq.URL.Path)
	assert.Equal(t, "anthropic-key-xyz", capturedReq.Header.Get("x-api-key"))
	assert.Equal(t, "2023-06-01", capturedReq.Header.Get("anthropic-version"))
	assert.Empty(t, capturedReq.Header.Get("Authorization"),
		"Anthropic must NOT use Authorization: Bearer")

	body := captured["/v1/messages"]
	assert.Contains(t, body, `"model":"claude-3-5-sonnet-20241022"`)
	assert.Contains(t, body, `"system":"You are helpful."`)
	assert.Contains(t, body, `"max_tokens":512`)
	assert.Contains(t, body, `"stream":false`)
}

// ── Embedded content blocks are dropped (Anthropic only accepts text) ───

func TestAnthropicMessagesAdapter_Chat_NonTextContentDropped(t *testing.T) {
	a, captured := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "msg_nt",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "ok"}],
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 1, "output_tokens": 1}
			}`))
		},
	})

	req := &domain.CanonicalRequest{
		ModelAlias: "claude-3-5-sonnet-20241022",
		Messages: []domain.CanonicalMessage{
			{Role: domain.RoleUser, Content: []domain.CanonicalContentBlock{
				{Type: domain.ContentBlockText, Text: "see this image"},
				{Type: domain.ContentBlockImageURL, ImageURL: "https://example.com/img.png"},
			}},
		},
	}
	resp, err := a.Chat(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Body should contain only the text block, image URL block is dropped.
	body := captured["/v1/messages"]
	assert.Contains(t, body, `"type":"text"`)
	assert.Contains(t, body, "see this image")
	assert.NotContains(t, body, "image_url")
}

// ── Missing model_alias guard ──────────────────────────────────────────

func TestAnthropicMessagesAdapter_Chat_MissingModelAlias(t *testing.T) {
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, r *http.Request) {},
	})
	_, err := a.Chat(context.Background(), &domain.CanonicalRequest{
		Messages: []domain.CanonicalMessage{{Role: domain.RoleUser}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model_alias")
}

// ── NewAnthropicMessages default base URL ──────────────────────────────

func TestNewAnthropicMessages_DefaultBaseURL(t *testing.T) {
	a := NewAnthropicMessages("", "", 0)
	assert.Equal(t, "https://api.anthropic.com", a.baseURL)
}

// ── Stop reason map: Anthropic uses end_turn, max_tokens, etc. ─────────

func TestAnthropicMessagesAdapter_Chat_StopReasonPassthrough(t *testing.T) {
	for _, stopReason := range []string{"end_turn", "max_tokens", "stop_sequence"} {
		a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{
			"/v1/messages": func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{
					"id": "msg_stop",
					"type": "message",
					"role": "assistant",
					"content": [{"type": "text", "text": "bye"}],
					"stop_reason": "` + stopReason + `",
					"usage": {"input_tokens": 1, "output_tokens": 1}
				}`))
			},
		})
		resp, err := a.Chat(context.Background(), baseAnthropicRequest())
		require.NoError(t, err)
		assert.Equal(t, stopReason, resp.FinishReason)
	}
}

// ── SSE with Anthropic image blocks (content_block_start with image) ────
// Anthropic can emit content_block_start for non-text content blocks;
// our parser currently only handles content_block_delta text deltas.

func TestAnthropicMessagesAdapter_ChatStream_IgnoresNonTextDelta(t *testing.T) {
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			// content_block_delta with no text field → should be skipped
			// Also content_block_start which our parser doesn't recognize
			payloads := []string{
				"event: content_block_start\n" +
					`data: {"type":"content_block_start","index":0}` + "\n\n",
				"event: content_block_delta\n" +
					`data: {"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{}}"}}` + "\n\n",
				"event: content_block_delta\n" +
					`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"final"}}` + "\n\n",
				"event: message_stop\n" +
					`data: {"type":"message_stop"}` + "\n\n",
			}
			for _, p := range payloads {
				_, _ = w.Write([]byte(p))
			}
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := a.ChatStream(ctx, baseAnthropicRequest())
	require.NoError(t, err)

	var events []*domain.CanonicalStreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Only content_block_delta with text → content_delta, and message_stop
	require.Len(t, events, 2)
	assert.Equal(t, "content_delta", events[0].Type)
	assert.Equal(t, "final", events[0].ContentDelta.Text)
	assert.Equal(t, "message_stop", events[1].Type)
}

// ── Sanity check: adapter Name/SupportedProtocol ────────────────────────

func TestAnthropicMessagesAdapter_Metadata(t *testing.T) {
	a := NewAnthropicMessages("https://api.anthropic.com/", "k", 0)
	assert.Equal(t, "anthropic", a.Name())
	assert.Equal(t, "anthropic", a.SupportedProtocol())
	assert.Equal(t, "https://api.anthropic.com", a.baseURL, "trailing slash trimmed")
}

// ── Upstream body is classified but never reflected ─────────────────────

func TestAnthropicMessagesAdapter_Chat_ErrorBodyNotPersisted(t *testing.T) {
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"model not found: xyz","type":"not_found_error"}}`))
		},
	})
	_, err := a.Chat(context.Background(), baseAnthropicRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	// Status classification is unchanged (400 → invalid_argument), but the
	// body is no longer reflected into usage/outbox-bound errors.
	assert.Equal(t, apperrors.CodeInvalidArgument, ae.Code)
	assert.Contains(t, err.Error(), "upstream 400")
	assert.NotContains(t, err.Error(), "model not found")
	assert.NotContains(t, err.Error(), "xyz")
}

// ── Embedding path: returns Unimplemented so caller knows to route ──────

func TestAnthropicMessagesAdapter_Embedding_MissingModelAliasStillUnimplemented(t *testing.T) {
	// Even with valid request, Embedding returns Unimplemented —
	// this is by design, Anthropic has no embeddings endpoint.
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{})
	req := &domain.CanonicalRequest{
		Messages: []domain.CanonicalMessage{{Role: domain.RoleUser, Content: []domain.CanonicalContentBlock{
			{Type: domain.ContentBlockText, Text: "embed me"},
		}}},
	}
	_, err := a.Embedding(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not supported")
}
