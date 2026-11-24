package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func TestRecoverer_WritesUnifiedErrorBody(t *testing.T) {
	handler := chimw.RequestID(Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("db password=secret")
	})))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	request.Header.Set("X-Request-Id", "req-panic-b1")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"requestId"`
		TraceID   string `json:"traceId"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v (raw=%s)", err, recorder.Body.String())
	}
	if body.Error.Code != "internal" {
		t.Errorf("code = %q, want internal", body.Error.Code)
	}
	if body.Error.Message != "an internal error occurred" {
		t.Errorf("message = %q, want generic internal message", body.Error.Message)
	}
	if body.RequestID != "req-panic-b1" || body.TraceID == "" {
		t.Errorf("requestId/traceId = %q/%q, want req-panic-b1 and a non-empty traceId", body.RequestID, body.TraceID)
	}
	if recorder.Body.String() == "" {
		t.Error("response body must not be empty")
	}
}

func TestRecoverer_DoesNotRewriteCommittedResponse(t *testing.T) {
	handler := chimw.RequestID(Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial-stream"))
		panic("late panic")
	})))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/stream", http.NoBody))

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want committed 200", recorder.Code)
	}
	if recorder.Body.String() != "partial-stream" {
		t.Errorf("body = %q, want untouched partial response", recorder.Body.String())
	}
}

func TestRecoverer_NoPanicPassesThrough(t *testing.T) {
	handler := Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody))

	if recorder.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if recorder.Body.String() != "" {
		t.Errorf("body = %q, want empty", recorder.Body.String())
	}
}
