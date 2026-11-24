package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
)

type unifiedErrorEnvelope struct {
	Error     map[string]any `json:"error"`
	RequestID string         `json:"requestId"`
	TraceID   string         `json:"traceId"`
}

// serveWithRequestID mounts a middleware under chi's RequestID middleware so
// the canonical writer can populate requestId/traceId exactly as in production.
func serveWithRequestID(handler http.Handler, requestID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/test", http.NoBody)
	request.RemoteAddr = "10.0.0.1:1234"
	return serveMiddlewareWithRequestID(handler, request, requestID)
}

func serveMiddlewareWithRequestID(handler http.Handler, request *http.Request, requestID string) *httptest.ResponseRecorder {
	if requestID != "" {
		request.Header.Set("X-Request-Id", requestID)
	}
	recorder := httptest.NewRecorder()
	chimw.RequestID(handler).ServeHTTP(recorder, request)
	return recorder
}

func assertUnifiedErrorBody(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode, wantMessage, wantRequestID string) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body=%s)", recorder.Code, wantStatus, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", contentType)
	}

	var top map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &top); err != nil {
		t.Fatalf("decode top-level object: %v (raw=%s)", err, recorder.Body.String())
	}
	if len(top) != 3 {
		t.Errorf("top-level field set = %v, want exactly {error,requestId,traceId}", top)
	}
	for _, key := range []string{"error", "requestId", "traceId"} {
		if _, ok := top[key]; !ok {
			t.Errorf("missing top-level field %q in %v", key, top)
		}
	}

	var envelope unifiedErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error body: %v (raw=%s)", err, recorder.Body.String())
	}
	if envelope.Error == nil {
		t.Fatalf("missing top-level error object: %s", recorder.Body.String())
	}
	if len(envelope.Error) != 2 {
		t.Errorf("error field set = %v, want exactly {code,message}", envelope.Error)
	}
	if envelope.Error["code"] != wantCode {
		t.Errorf("error.code = %v, want %q", envelope.Error["code"], wantCode)
	}
	if envelope.Error["message"] != wantMessage {
		t.Errorf("error.message = %v, want %q", envelope.Error["message"], wantMessage)
	}
	if envelope.RequestID != wantRequestID {
		t.Errorf("requestId = %q, want %q", envelope.RequestID, wantRequestID)
	}
	if envelope.TraceID == "" {
		t.Error("traceId must not be empty")
	}
	if envelope.TraceID != envelope.RequestID {
		t.Errorf("traceId = %q, want request-id fallback %q when no span is recording", envelope.TraceID, envelope.RequestID)
	}
}
