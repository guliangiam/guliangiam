package aigateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGatewayHandler_Stream_ErrorEventDoesNotReportSuccess pins the mid-stream
// failure contract for all three protocol faces: the client receives a
// structured error frame with code/message, and the success terminator
// ([DONE] / message_stop) is never written after it.
func TestGatewayHandler_Stream_ErrorEventDoesNotReportSuccess(t *testing.T) {
	tests := []struct {
		face       string
		path       string
		modelAlias string
		modelName  string
	}{
		{
			face:       "openai",
			path:       "/api/v1/ai/faces/openai/chat/completions",
			modelAlias: "gpt-4o",
			modelName:  "gpt-4o",
		},
		{
			face:       "anthropic",
			path:       "/api/v1/ai/faces/anthropic/chat/completions",
			modelAlias: "claude-sonnet",
			modelName:  "claude-3-5-sonnet",
		},
		{
			face:       "gemini",
			path:       "/api/v1/ai/faces/gemini/chat/completions",
			modelAlias: "gemini-2.0-flash",
			modelName:  "gemini-2.0-flash",
		},
	}

	for _, tc := range tests {
		t.Run(tc.face, func(t *testing.T) {
			upstream := &gwMockUpstream{name: tc.face + "_upstream", proto: tc.face}
			ch := make(chan *domain.CanonicalStreamEvent, 3)
			ch <- &domain.CanonicalStreamEvent{
				Type:         "content_delta",
				ContentDelta: &domain.CanonicalContentBlock{Type: domain.ContentBlockText, Text: "partial"},
			}
			ch <- &domain.CanonicalStreamEvent{
				Type: domain.StreamEventError,
				Error: &domain.CanonicalStreamError{
					Code:    "unavailable",
					Message: "upstream stream interrupted",
				},
			}
			close(ch)
			upstream.streamCh = ch

			registry := newGwMockRegistry()
			registry.Register(tc.face, upstream)

			provider := &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: tc.face}
			model := &domain.LlmModel{ID: "m-1", ProviderModelName: tc.modelName}
			modelRoute := &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1}
			router := &gwMockRouter{res: &domain.RouteResolution{
				Provider: provider, Model: model, Route: modelRoute, ProtocolFace: tc.face,
			}}

			svc := newGatewayServiceForFaceTest(t, router, registry)
			h := NewHandler(svc)
			chiRouter := chi.NewRouter()
			RegisterChiRoutes(chiRouter, h)

			body := map[string]any{
				"model":    tc.modelAlias,
				"stream":   true,
				"messages": []map[string]any{{"role": "user", "content": "hi"}},
			}
			recorder := performGatewayJSON(t, chiRouter, http.MethodPost, tc.path, body, authHeaders())

			require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
			require.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")

			bodyStr := recorder.Body.String()
			assert.Contains(t, bodyStr, `"code":"unavailable"`, "error frame must carry the code")
			assert.Contains(t, bodyStr, `"message":"upstream stream interrupted"`, "error frame must carry the message")
			assert.NotContains(t, bodyStr, `"stack"`, "error frame must not carry an internal stack")

			switch tc.face {
			case "anthropic":
				assert.Contains(t, bodyStr, "event: error")
				assert.NotContains(t, bodyStr, "message_stop", "failed stream must not write the success terminator")
				assert.NotContains(t, bodyStr, "content_block_stop", "failed stream must not close the content block")
			case "gemini":
				assert.Contains(t, bodyStr, "event: error")
				assert.NotContains(t, bodyStr, "data: [DONE]", "failed stream must not write the success terminator")
			default:
				assert.Contains(t, bodyStr, `"error"`)
				assert.NotContains(t, bodyStr, "data: [DONE]", "failed stream must not write the success terminator")
			}
		})
	}
}

// nonFlushingResponseWriter is an http.ResponseWriter without http.Flusher.
type nonFlushingResponseWriter struct {
	header http.Header
	code   int
	body   strings.Builder
}

func (w *nonFlushingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *nonFlushingResponseWriter) WriteHeader(code int) { w.code = code }

func (w *nonFlushingResponseWriter) Write(p []byte) (int, error) { return w.body.Write(p) }

func TestGatewayHandler_Stream_NoFlusherUnifiedEnvelope(t *testing.T) {
	upstream := &gwMockUpstream{name: "openai_upstream", proto: "openai"}
	ch := make(chan *domain.CanonicalStreamEvent)
	close(ch)
	upstream.streamCh = ch

	registry := newGwMockRegistry()
	registry.Register("openai", upstream)

	provider := &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: "openai"}
	model := &domain.LlmModel{ID: "m-1", ProviderModelName: "gpt-4o"}
	modelRoute := &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1}
	router := &gwMockRouter{res: &domain.RouteResolution{
		Provider: provider, Model: model, Route: modelRoute, ProtocolFace: "openai",
	}}

	svc := newGatewayServiceForFaceTest(t, router, registry)
	h := NewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/faces/openai/chat/completions", strings.NewReader(
		`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testKeyRaw)
	req.Header.Set("X-Request-Id", "ai-500-noflusher")

	writer := &nonFlushingResponseWriter{}
	chimw.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleChatForFace("openai", w, r)
	})).ServeHTTP(writer, req)

	require.Equal(t, http.StatusInternalServerError, writer.code)
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"requestId"`
		TraceID   string `json:"traceId"`
	}
	require.NoError(t, json.Unmarshal([]byte(writer.body.String()), &envelope), writer.body.String())
	assert.Equal(t, "internal", envelope.Error.Code)
	assert.Equal(t, "an internal error occurred", envelope.Error.Message)
	assert.Equal(t, "ai-500-noflusher", envelope.RequestID)
	assert.Equal(t, envelope.RequestID, envelope.TraceID)
}
