package aiupstream

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Gemini fixture ─────────────────────────────────────────────────────

// newGeminiFixture builds an httptest.Server that handles Gemini paths.
// Handlers are keyed by method (e.g. "generateContent" / "streamGenerateContent")
// and match anything under /v1beta/models/{model}:{handler}.
func newGeminiFixture(t *testing.T, handlers map[string]http.HandlerFunc) (adapter *GeminiGenerateContentAdapter, captured map[string]string) {
	t.Helper()
	captured = make(map[string]string)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// path: /v1beta/models/{model}:{handler}
		path := r.URL.Path
		var key string
		if strings.HasSuffix(path, ":generateContent") {
			key = "generateContent"
		} else if strings.HasSuffix(path, ":streamGenerateContent") {
			key = "streamGenerateContent"
		}
		if key == "" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		captured[key] = string(body)
		if h, ok := handlers[key]; ok {
			h(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return NewGeminiGenerateContent(srv.URL, "gemini-key-abc", 2*time.Second), captured
}

func baseGeminiRequest() *domain.CanonicalRequest {
	return &domain.CanonicalRequest{
		ModelAlias: "gemini-1.5-pro",
		Messages: []domain.CanonicalMessage{
			{Role: domain.RoleUser, Content: []domain.CanonicalContentBlock{
				{Type: domain.ContentBlockText, Text: "hello"},
			}},
		},
	}
}

// ── Chat happy path ────────────────────────────────────────────────────

func TestGeminiGenerateContentAdapter_Chat_HappyPath(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"generateContent": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{
				"candidates": [{
					"content": {
						"role": "model",
						"parts": [{"text": "hi there"}]
					},
					"finishReason": "STOP"
				}],
				"usageMetadata": {
					"promptTokenCount": 5,
					"candidatesTokenCount": 3,
					"totalTokenCount": 8
				}
			}`))
		},
	})

	resp, err := a.Chat(context.Background(), baseGeminiRequest())
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, 5, resp.InputTokens)
	assert.Equal(t, 3, resp.OutputTokens)
	assert.Equal(t, 8, resp.TotalTokens)
	assert.Equal(t, "stop", resp.FinishReason, "STOP → canonical 'stop'")

	require.Len(t, resp.Content, 1)
	assert.Equal(t, "hi there", resp.Content[0].Text)
	assert.Equal(t, domain.ContentBlockText, resp.Content[0].Type)
}

// ── Gemini finish reason mapping ───────────────────────────────────────

func TestGeminiGenerateContentAdapter_Chat_FinishReasonMapping(t *testing.T) {
	mapping := map[string]string{
		"STOP":               "stop",
		"MAX_TOKENS":         "length",
		"SAFETY":             "content_filter",
		"RECITATION":         "content_filter",
		"OTHER":              "other",
		"UNKNOWN_NEW_REASON": "UNKNOWN_NEW_REASON", // unknown → passthrough
	}
	for gemini, canonical := range mapping {
		a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
			"generateContent": func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{
					"candidates": [{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"` + gemini + `"}],
					"usageMetadata": {"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}
				}`))
			},
		})
		resp, err := a.Chat(context.Background(), baseGeminiRequest())
		require.NoError(t, err, "gemini=%s", gemini)
		assert.Equal(t, canonical, resp.FinishReason, "gemini=%s", gemini)
	}
}

// ── Upstream errors ────────────────────────────────────────────────────

func TestGeminiGenerateContentAdapter_Chat_Upstream401(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"generateContent": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
		},
	})
	_, err := a.Chat(context.Background(), baseGeminiRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnauthorized, ae.Code)
}

func TestGeminiGenerateContentAdapter_Chat_Upstream429(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"generateContent": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limit"}}`))
		},
	})
	_, err := a.Chat(context.Background(), baseGeminiRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeTooManyRequests, ae.Code)
}

func TestGeminiGenerateContentAdapter_Chat_UpstreamGeneric500(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"generateContent": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		},
	})
	_, err := a.Chat(context.Background(), baseGeminiRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnavailable, ae.Code)
}

// ── NDJSON streaming ───────────────────────────────────────────────────

func TestGeminiGenerateContentAdapter_ChatStream_NDJSON(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"streamGenerateContent": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			// Gemini NDJSON = one JSON object per line (no SSE prefix)
			chunks := []string{
				`{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]}}]}`,
				`{"candidates":[{"content":{"role":"model","parts":[{"text":" there"}]}}]}`,
				`{"candidates":[{"content":{"role":"model","parts":[{"text":""}]},"finishReason":"STOP"}]}`,
				`{"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":5,"totalTokenCount":8}}`,
			}
			for _, c := range chunks {
				_, _ = w.Write([]byte(c + "\n"))
			}
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := a.ChatStream(ctx, baseGeminiRequest())
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

// ── Embedding returns Unimplemented ────────────────────────────────────

func TestGeminiGenerateContentAdapter_Embedding_Unimplemented(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{})
	_, err := a.Embedding(context.Background(), baseGeminiRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnimplemented, ae.Code)
}

// ── Auth: ?key= query param (NOT Bearer header) ────────────────────────

func TestGeminiGenerateContentAdapter_Chat_UsesQueryParamAuth(t *testing.T) {
	var capturedReq *http.Request
	a, captured := newGeminiFixture(t, map[string]http.HandlerFunc{
		"generateContent": func(w http.ResponseWriter, r *http.Request) {
			capturedReq = r
			_, _ = w.Write([]byte(`{
				"candidates": [{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],
				"usageMetadata": {"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}
			}`))
		},
	})

	_, _ = a.Chat(context.Background(), baseGeminiRequest())

	require.NotNil(t, capturedReq)
	assert.Equal(t, "gemini-key-abc", capturedReq.URL.Query().Get("key"), "API key in query param")
	assert.Empty(t, capturedReq.Header.Get("Authorization"), "Gemini must NOT use Authorization: Bearer")
	assert.Empty(t, capturedReq.Header.Get("x-api-key"))

	// Model alias IS in path
	assert.Contains(t, capturedReq.URL.Path, "/gemini-1.5-pro:generateContent")
	assert.Contains(t, captured["generateContent"], `"role":"user"`)
	assert.Contains(t, captured["generateContent"], `"text":"hello"`)
}

// ── Gemini NDJSON usage-only chunk (no candidates) ──────────────────────

func TestGeminiGenerateContentAdapter_ChatStream_UsageOnlyChunk(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"streamGenerateContent": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			// A chunk that is only usage metadata (no candidates)
			_, _ = w.Write([]byte(`{"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":5,"totalTokenCount":8}}` + "\n"))
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := a.ChatStream(ctx, baseGeminiRequest())
	require.NoError(t, err)

	var events []*domain.CanonicalStreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	require.Len(t, events, 1)
	assert.Equal(t, "message_delta", events[0].Type)
	assert.Equal(t, 8, events[0].UsageMetadata["total_tokens"])
}

// ── Missing model_alias guard (Chat + ChatStream both check it) ─────────

func TestGeminiGenerateContentAdapter_Chat_MissingModelAlias(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"generateContent": func(w http.ResponseWriter, r *http.Request) {},
	})
	_, err := a.Chat(context.Background(), &domain.CanonicalRequest{
		Messages: []domain.CanonicalMessage{{Role: domain.RoleUser}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model_alias")
}

func TestGeminiGenerateContentAdapter_ChatStream_MissingModelAlias(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"streamGenerateContent": func(w http.ResponseWriter, r *http.Request) {},
	})
	_, err := a.ChatStream(context.Background(), &domain.CanonicalRequest{
		Messages: []domain.CanonicalMessage{{Role: domain.RoleUser}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model_alias")
}

// ── Request body shape: generationConfig + systemInstruction ───────────

func TestGeminiGenerateContentAdapter_Chat_RequestBodyShape(t *testing.T) {
	a, captured := newGeminiFixture(t, map[string]http.HandlerFunc{
		"generateContent": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{
				"candidates": [{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],
				"usageMetadata": {"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}
			}`))
		},
	})

	req := baseGeminiRequest()
	req.System = "You are helpful."
	req.MaxTokens = 128
	req.Temperature = 0.5

	_, _ = a.Chat(context.Background(), req)

	body := captured["generateContent"]
	// System → systemInstruction (content with role "system")
	assert.Contains(t, body, `"role":"system"`)
	assert.Contains(t, body, `"parts":[{"text":"You are helpful."}]`)
	// Generation config
	assert.Contains(t, body, `"maxOutputTokens":128`)
	assert.Contains(t, body, `"temperature":0.5`)
}

// ── mapCanonicalRoleToGemini: assistant → model ─────────────────────────

func TestGeminiGenerateContentAdapter_Chat_RoleMapping(t *testing.T) {
	a, captured := newGeminiFixture(t, map[string]http.HandlerFunc{
		"generateContent": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{
				"candidates": [{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],
				"usageMetadata": {"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}
			}`))
		},
	})

	req := &domain.CanonicalRequest{
		ModelAlias: "gemini-1.5-pro",
		Messages: []domain.CanonicalMessage{
			{Role: domain.RoleUser, Content: []domain.CanonicalContentBlock{
				{Type: domain.ContentBlockText, Text: "hi"},
			}},
			{Role: domain.RoleAssistant, Content: []domain.CanonicalContentBlock{
				{Type: domain.ContentBlockText, Text: "hello"},
			}},
		},
	}
	_, _ = a.Chat(context.Background(), req)

	body := captured["generateContent"]
	// assistant role must be mapped to "model" per Gemini 1.5+ API
	assert.Contains(t, body, `"role":"user"`)
	assert.Contains(t, body, `"role":"model"`)
	assert.NotContains(t, body, `"role":"assistant"`)
}

// ── Adapter metadata ──────────────────────────────────────────────────

func TestGeminiGenerateContentAdapter_Metadata(t *testing.T) {
	a := NewGeminiGenerateContent("https://generativelanguage.googleapis.com/", "k", 0)
	assert.Equal(t, "gemini", a.Name())
	assert.Equal(t, "gemini", a.SupportedProtocol())
	assert.Equal(t, "https://generativelanguage.googleapis.com", a.baseURL, "trailing slash trimmed")
}

// ── Default base URL when empty ────────────────────────────────────────

func TestGeminiGenerateContentAdapter_DefaultBaseURL(t *testing.T) {
	a := NewGeminiGenerateContent("", "", 0)
	assert.Equal(t, "https://generativelanguage.googleapis.com", a.baseURL)
}

// ── Role mapper unit test ──────────────────────────────────────────────

func TestMapCanonicalRoleToGemini(t *testing.T) {
	cases := map[string]string{
		"user":      "user",
		"assistant": "model",
		"system":    "user", // system in message list → fallback to user
		"unknown":   "unknown",
	}
	for in, want := range cases {
		assert.Equal(t, want, mapCanonicalRoleToGemini(in), "input=%s", in)
	}
}

// ── NDJSON stream empty lines / invalid JSON silently skipped ──────────

func TestGeminiGenerateContentAdapter_ChatStream_EmptyAndInvalidSkipped(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"streamGenerateContent": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			// Mix: empty line, invalid JSON, valid text delta, valid finishReason
			_, _ = w.Write([]byte("\n"))
			_, _ = w.Write([]byte("not json at all\n"))
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"final"}]}}]}` + "\n"))
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":""}]},"finishReason":"STOP"}]}` + "\n"))
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := a.ChatStream(ctx, baseGeminiRequest())
	require.NoError(t, err)

	var events []*domain.CanonicalStreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	require.Len(t, events, 2)
	assert.Equal(t, "content_delta", events[0].Type)
	assert.Equal(t, "final", events[0].ContentDelta.Text)
	assert.Equal(t, "message_stop", events[1].Type)
}

// ── Upstream body is classified but never reflected ─────────────────────

func TestGeminiGenerateContentAdapter_Chat_ErrorBodyNotPersisted(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{
		"generateContent": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid model: xyz"}}`))
		},
	})
	_, err := a.Chat(context.Background(), baseGeminiRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	// Status classification is unchanged (400 → invalid_argument), but the
	// upstream body is no longer reflected: this error feeds
	// llm_usage_logs.error_code and the outbox payload, and an upstream may
	// echo ?key=<plaintext> or Authorization into its error message.
	assert.Equal(t, apperrors.CodeInvalidArgument, ae.Code)
	assert.Contains(t, err.Error(), "upstream 400")
	assert.NotContains(t, err.Error(), "invalid model")
	assert.NotContains(t, err.Error(), "xyz")
}

// ── Transport errors never echo the URL or provider key ─────────────────

func TestGeminiGenerateContentAdapter_TransportError_RedactsCredential(t *testing.T) {
	const (
		providerKey = "gemini-key-abc"
		baseURL     = "https://generativelanguage.googleapis.com"
	)

	cases := []struct {
		name       string
		err        error
		wantClass  string
		isDeadline bool
		isCanceled bool
	}{
		{name: "transport failure", err: errors.New("dial tcp: connection refused"), wantClass: "transport failure"},
		{name: "timeout", err: context.DeadlineExceeded, wantClass: "request timed out", isDeadline: true},
		{name: "canceled", err: context.Canceled, wantClass: "request canceled", isCanceled: true},
	}

	for _, tc := range cases {
		tc := tc
		for _, stream := range []bool{false, true} {
			stream := stream
			name := tc.name
			if stream {
				name += "/stream"
			}
			t.Run(name, func(t *testing.T) {
				adapter := NewGeminiGenerateContent(baseURL, providerKey, time.Second)
				adapter.httpClient = &http.Client{Transport: transportRoundTripper(func(*http.Request) (*http.Response, error) {
					return nil, tc.err
				})}

				ctx := context.Background()
				if tc.isCanceled {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}

				var err error
				if stream {
					_, err = adapter.ChatStream(ctx, baseGeminiRequest())
				} else {
					_, err = adapter.Chat(ctx, baseGeminiRequest())
				}
				require.Error(t, err)

				var ae *apperrors.Error
				require.ErrorAs(t, err, &ae)
				assert.Equal(t, apperrors.CodeUnavailable, ae.Code)
				assert.Contains(t, err.Error(), tc.wantClass)

				// The original transport error stays reachable through the
				// sanitized wrapper, so callers can still classify it.
				var urlErr *url.Error
				assert.ErrorAs(t, err, &urlErr,
					"the *url.Error must stay in the chain for errors.As")

				// The *url.Error would render the full URL including ?key=.
				assert.NotContains(t, err.Error(), "key=")
				assert.NotContains(t, err.Error(), providerKey)
				assert.NotContains(t, err.Error(), baseURL)
				assert.NotContains(t, err.Error(), "generateContent")

				if tc.isDeadline {
					assert.ErrorIs(t, err, context.DeadlineExceeded,
						"sanitizing must keep the timeout chain for errors.Is")
				}
				if tc.isCanceled {
					assert.ErrorIs(t, err, context.Canceled,
						"sanitizing must keep the cancellation chain for errors.Is")
				}
			})
		}
	}
}

// ── Embedding path: always Unimplemented ───────────────────────────────

func TestGeminiGenerateContentAdapter_Embedding_AlwaysUnimplemented(t *testing.T) {
	a, _ := newGeminiFixture(t, map[string]http.HandlerFunc{})
	req := &domain.CanonicalRequest{
		Messages: []domain.CanonicalMessage{{Role: domain.RoleUser, Content: []domain.CanonicalContentBlock{
			{Type: domain.ContentBlockText, Text: "embed this"},
		}}},
	}
	_, err := a.Embedding(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not implemented")
}
