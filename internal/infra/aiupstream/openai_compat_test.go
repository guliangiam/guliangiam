package aiupstream

import (
	"context"
	"encoding/json"
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

// openaiUpstreamFixtures holds canned OpenAI-surface responses served by
// httptest.Server. Each test customizes what the mock returns for a given path.
type openaiFixture struct {
	t        *testing.T
	server   *httptest.Server
	adapter  *OpenAICompatAdapter
	captured map[string]string // last request data per path
}

func newOpenAIFixture(t *testing.T, handlers map[string]http.HandlerFunc) *openaiFixture {
	t.Helper()
	captured := make(map[string]string)
	mux := http.NewServeMux()
	for path, h := range handlers {
		p := path // capture
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			captured[p] = string(body)
			h(w, r)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	adapter := NewOpenAICompat(srv.URL, "test-key-xyz", 2*time.Second)
	return &openaiFixture{t: t, server: srv, adapter: adapter, captured: captured}
}

// baseCanonicalRequest is a minimal valid request — most tests fill just
// the fields they care about.
func baseCanonicalRequest() *domain.CanonicalRequest {
	return &domain.CanonicalRequest{
		ModelAlias: "gpt-4o",
		Messages: []domain.CanonicalMessage{
			{Role: domain.RoleUser, Content: []domain.CanonicalContentBlock{
				{Type: domain.ContentBlockText, Text: "hello"},
			}},
		},
	}
}

// ── Chat happy path ─────────────────────────────────────────────────────

func TestOpenAICompatAdapter_Chat_HappyPath(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/chat/completions": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": "chatcmpl-abc123",
				"choices": [{
					"index": 0,
					"finish_reason": "stop",
					"message": {"role": "assistant", "content": "hi there"}
				}],
				"usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
			}`))
		},
	})

	ctx := context.Background()
	resp, err := fx.adapter.Chat(ctx, baseCanonicalRequest())
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "chatcmpl-abc123", resp.UpstreamRequestID)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 5, resp.InputTokens)
	assert.Equal(t, 3, resp.OutputTokens)
	assert.Equal(t, 8, resp.TotalTokens)

	require.Len(t, resp.Content, 1)
	assert.Equal(t, "hi there", resp.Content[0].Text)
	assert.Equal(t, domain.ContentBlockText, resp.Content[0].Type)

	// Authorization header was propagated
	assert.Contains(t, fx.captured["/chat/completions"], `"model":"gpt-4o"`)
}

// ── Chat with tools response (tool_calls) ────────────────────────────────

func TestOpenAICompatAdapter_Chat_WithToolCalls(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/chat/completions": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "chatcmpl-tool",
				"choices": [{
					"index": 0,
					"finish_reason": "tool_calls",
					"message": {
						"role": "assistant",
						"content": null,
						"tool_calls": [{
							"id": "call_xyz",
							"type": "function",
							"function": {
								"name": "get_weather",
								"arguments": "{\"city\":\"beijing\"}"
							}
						}]
					}
				}],
				"usage": {"prompt_tokens": 10, "completion_tokens": 15, "total_tokens": 25}
			}`))
		},
	})

	resp, err := fx.adapter.Chat(context.Background(), baseCanonicalRequest())
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Len(t, resp.Content, 1)
	assert.Equal(t, domain.ContentBlockToolUse, resp.Content[0].Type)
	assert.Equal(t, "call_xyz", resp.Content[0].ToolCallID)
	assert.Equal(t, "get_weather", resp.Content[0].ToolName)
	assert.Equal(t, map[string]any{"city": "beijing"}, resp.Content[0].ToolInput)
}

// ── Embedding happy path ────────────────────────────────────────────────

func TestOpenAICompatAdapter_Embedding_HappyPath(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/embeddings": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{
				"object": "list",
				"data": [{"index": 0, "embedding": [0.1, 0.2, 0.3]}],
				"usage": {"prompt_tokens": 4, "total_tokens": 4}
			}`))
		},
	})

	req := baseCanonicalRequest()
	req.Messages[0].Content[0].Text = "embed me please"

	resp, err := fx.adapter.Embedding(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, 4, resp.InputTokens)
	assert.Equal(t, 4, resp.TotalTokens)

	require.Len(t, resp.Content, 1)
	var emb []float64
	require.NoError(t, json.Unmarshal([]byte(resp.Content[0].Text), &emb))
	assert.Equal(t, []float64{0.1, 0.2, 0.3}, emb)
}

// ── Chat upstream error handling ────────────────────────────────────────

func TestOpenAICompatAdapter_Chat_Upstream401(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/chat/completions": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid api key","type":"authentication_error"}}`))
		},
	})

	_, err := fx.adapter.Chat(context.Background(), baseCanonicalRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnauthorized, ae.Code)
	// The upstream body is classified by status but never reflected; see
	// TestOpenAICompatAdapter_UpstreamBodyDoesNotEchoCredential.
	assert.Contains(t, err.Error(), "upstream 401")
	assert.NotContains(t, err.Error(), "invalid api key")
}

func TestOpenAICompatAdapter_Chat_Upstream429(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/chat/completions": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limit","type":"rate_limit_exceeded"}}`))
		},
	})

	_, err := fx.adapter.Chat(context.Background(), baseCanonicalRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeTooManyRequests, ae.Code)
}

func TestOpenAICompatAdapter_Chat_UpstreamGeneric500(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/chat/completions": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		},
	})

	_, err := fx.adapter.Chat(context.Background(), baseCanonicalRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	// 5xx defaults to Unavailable — not unauthorized and not too-many
	assert.Equal(t, apperrors.CodeUnavailable, ae.Code)
}

// ── ChatStream SSE ──────────────────────────────────────────────────────

func TestOpenAICompatAdapter_ChatStream_SSE(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/chat/completions": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			// httptest.ResponseRecorder buffers everything — all SSE data
			// is available at once on the client side. We don't need Flusher.
			payloads := []string{
				`data: {"choices":[{"delta":{"content":"hi"},"index":0}]}` + "\n\n",
				`data: {"choices":[{"delta":{"content":" there"},"index":0}]}` + "\n\n",
				`data: {"choices":[{"delta":{},"finish_reason":"stop","index":0}]}` + "\n\n",
				`data: {"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}}` + "\n\n",
				`data: [DONE]` + "\n\n",
			}
			for _, p := range payloads {
				_, _ = w.Write([]byte(p))
			}
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := fx.adapter.ChatStream(ctx, baseCanonicalRequest())
	require.NoError(t, err)

	var events []*domain.CanonicalStreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	require.Len(t, events, 4)
	assert.Equal(t, "content_delta", events[0].Type)
	assert.Equal(t, "hi", events[0].ContentDelta.Text)
	assert.Equal(t, "content_delta", events[1].Type)
	assert.Equal(t, " there", events[1].ContentDelta.Text)
	assert.Equal(t, "message_stop", events[2].Type)
	assert.Equal(t, "message_delta", events[3].Type)
	assert.Equal(t, 8, events[3].UsageMetadata["total_tokens"])
}

// ── Request body: system + tools + tool_choice ───────────────────────────

func TestOpenAICompatAdapter_Chat_RequestBodyShape(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/chat/completions": func(w http.ResponseWriter, r *http.Request) {
			// Don't respond — we just want to inspect the body
		},
	})

	req := baseCanonicalRequest()
	req.System = "You are a helpful assistant."
	req.Temperature = 0.7
	req.MaxTokens = 64
	req.Stop = []string{"END"}

	_, _ = fx.adapter.Chat(context.Background(), req)

	raw := fx.captured["/chat/completions"]
	var body openaiChatBody
	require.NoError(t, json.Unmarshal([]byte(raw), &body))

	assert.Equal(t, "gpt-4o", body.Model)
	assert.False(t, body.Stream, "non-stream path must set stream=false")
	assert.Len(t, body.Messages, 2)
	assert.Equal(t, "system", body.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", body.Messages[0].Content)
	assert.Equal(t, "user", body.Messages[1].Role)

	require.NotNil(t, body.Temperature)
	assert.InDelta(t, 0.7, *body.Temperature, 0.001)
	require.NotNil(t, body.MaxTokens)
	assert.Equal(t, 64, *body.MaxTokens)
	assert.Equal(t, []string{"END"}, body.Stop)
}

// ── buildEmbeddingBody empty text guard ─────────────────────────────────

func TestOpenAICompatAdapter_Embedding_NoTextToEmbed(t *testing.T) {
	req := &domain.CanonicalRequest{
		ModelAlias: "text-embedding-3-small",
		Messages: []domain.CanonicalMessage{
			{Role: domain.RoleUser, Content: []domain.CanonicalContentBlock{}},
		},
	}
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/embeddings": func(w http.ResponseWriter, r *http.Request) {},
	})

	_, err := fx.adapter.Embedding(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "no text")
}

// ── Embedding missing model_alias ───────────────────────────────────────

func TestOpenAICompatAdapter_Embedding_MissingModelAlias(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/embeddings": func(w http.ResponseWriter, r *http.Request) {},
	})

	req := &domain.CanonicalRequest{
		Messages: []domain.CanonicalMessage{{Role: domain.RoleUser}},
	}
	_, err := fx.adapter.Embedding(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model_alias")
}

// ── Chat missing model_alias ────────────────────────────────────────────

func TestOpenAICompatAdapter_Chat_MissingModelAlias(t *testing.T) {
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/chat/completions": func(w http.ResponseWriter, r *http.Request) {},
	})
	req := &domain.CanonicalRequest{
		Messages: []domain.CanonicalMessage{{Role: domain.RoleUser}},
	}
	_, err := fx.adapter.Chat(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model_alias")
}

func TestBlocksToOpenAIContent_Shape(t *testing.T) {
	// 单文本块 → 纯字符串（兼容面最广）
	got := blocksToOpenAIContent([]domain.CanonicalContentBlock{
		{Type: domain.ContentBlockText, Text: "hi"},
	})
	assert.Equal(t, "hi", got)

	// 文本 + 图片 → 结构化 parts
	got = blocksToOpenAIContent([]domain.CanonicalContentBlock{
		{Type: domain.ContentBlockText, Text: "hi"},
		{Type: domain.ContentBlockImageURL, ImageURL: "https://example.com/a.png"},
	})
	parts, ok := got.([]openaiContentPart)
	require.True(t, ok)
	require.Len(t, parts, 2)
	assert.Equal(t, "text", parts[0].Type)
	assert.Equal(t, "hi", parts[0].Text)
	assert.Equal(t, "image_url", parts[1].Type)
	assert.Equal(t, "https://example.com/a.png", parts[1].ImageURL)

	// 工具块由 tools/tool_calls 承载，不在 content parts 中重复下发
	got = blocksToOpenAIContent([]domain.CanonicalContentBlock{
		{Type: domain.ContentBlockToolUse, ToolCallID: "tu-1", ToolName: "lookup"},
		{Type: domain.ContentBlockToolResult, ToolCallID: "tu-1", ToolResult: "done"},
		{Type: domain.ContentBlockText, Text: "after"},
	})
	parts, ok = got.([]openaiContentPart)
	require.True(t, ok)
	require.Len(t, parts, 1)
	assert.Equal(t, "after", parts[0].Text)
}
