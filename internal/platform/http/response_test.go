package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type testEnvelope struct {
	Error     map[string]any `json:"error"`
	RequestID string         `json:"requestId"`
	TraceID   string         `json:"traceId"`
}

func decodeTestEnvelope(t *testing.T, raw []byte) (map[string]any, testEnvelope) {
	t.Helper()
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode top-level object: %v (raw=%s)", err, raw)
	}
	var envelope testEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v (raw=%s)", err, raw)
	}
	return top, envelope
}

func assertKeySet(t *testing.T, got map[string]any, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("field set = %v, want %v", keys(got), want)
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing field %q in %v", key, keys(got))
		}
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}

func serveRequestID(t *testing.T, requestID string, write func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	handler := chimw.RequestID(http.HandlerFunc(write))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/test", http.NoBody)
	request.Header.Set("X-Request-Id", requestID)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestWriteError_ForbiddenUnifiedEnvelope(t *testing.T) {
	recorder := serveRequestID(t, "req-e1b2-403", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperrors.NewForbidden("access denied"))
	})

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	top, envelope := decodeTestEnvelope(t, recorder.Body.Bytes())
	assertKeySet(t, top, "error", "requestId", "traceId")
	assertKeySet(t, envelope.Error, "code", "message")
	if envelope.Error["code"] != string(apperrors.CodeForbidden) {
		t.Errorf("error.code = %v, want %q", envelope.Error["code"], apperrors.CodeForbidden)
	}
	if envelope.Error["message"] != "access denied" {
		t.Errorf("error.message = %v, want access denied", envelope.Error["message"])
	}
	if envelope.RequestID != "req-e1b2-403" {
		t.Errorf("requestId = %q, want req-e1b2-403", envelope.RequestID)
	}
	if envelope.TraceID == "" || envelope.TraceID != envelope.RequestID {
		t.Errorf("traceId = %q, want non-empty request-id fallback %q", envelope.TraceID, envelope.RequestID)
	}
}

func TestWriteError_TooManyRequestsUnifiedEnvelope(t *testing.T) {
	recorder := serveRequestID(t, "req-e1b2-429", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperrors.NewTooManyRequests("rate limit exceeded"))
	})

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	top, envelope := decodeTestEnvelope(t, recorder.Body.Bytes())
	assertKeySet(t, top, "error", "requestId", "traceId")
	assertKeySet(t, envelope.Error, "code", "message")
	if envelope.Error["code"] != string(apperrors.CodeTooManyRequests) {
		t.Errorf("error.code = %v, want %q", envelope.Error["code"], apperrors.CodeTooManyRequests)
	}
	if envelope.Error["message"] != "rate limit exceeded" {
		t.Errorf("error.message = %v, want rate limit exceeded", envelope.Error["message"])
	}
	if envelope.TraceID == "" || envelope.TraceID != envelope.RequestID {
		t.Errorf("traceId = %q, want non-empty request-id fallback %q", envelope.TraceID, envelope.RequestID)
	}
}

func TestWriteError_TraceIDFromActiveSpan(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("parse trace id: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("parse span id: %v", err)
	}

	handler := chimw.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperrors.NewForbidden("access denied"))
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/test", http.NoBody)
	request.Header.Set("X-Request-Id", "req-e1b2-span")
	request = request.WithContext(trace.ContextWithSpanContext(request.Context(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	_, envelope := decodeTestEnvelope(t, recorder.Body.Bytes())
	if envelope.RequestID != "req-e1b2-span" {
		t.Errorf("requestId = %q, want req-e1b2-span", envelope.RequestID)
	}
	if envelope.TraceID != traceID.String() {
		t.Errorf("traceId = %q, want span trace id %q", envelope.TraceID, traceID.String())
	}
}

func TestWriteError_CorrelationIDsStayAtTopLevel(t *testing.T) {
	recorder := serveRequestID(t, "req-e1b2-top-level", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperrors.NewNotFound("resource missing"))
	})

	_, envelope := decodeTestEnvelope(t, recorder.Body.Bytes())
	assertKeySet(t, envelope.Error, "code", "message")
	if envelope.RequestID != "req-e1b2-top-level" {
		t.Errorf("requestId = %q, want req-e1b2-top-level", envelope.RequestID)
	}
	if envelope.TraceID == "" || envelope.TraceID != envelope.RequestID {
		t.Errorf("traceId = %q, want non-empty request-id fallback %q", envelope.TraceID, envelope.RequestID)
	}
	for _, nested := range []string{"requestId", "traceId", "request_id", "trace_id"} {
		if _, ok := envelope.Error[nested]; ok {
			t.Errorf("error object must not nest %q; correlation ids are top-level only", nested)
		}
	}
}

func TestWriteError_CarriesMessageKeyDetailsAndMetaWhenAvailable(t *testing.T) {
	err := &apperrors.Error{
		Code:       apperrors.CodeUnprocessableEntity,
		Message:    "email must be an RFC 5322 address",
		MessageKey: "validation.email.format",
		Meta:       map[string]any{"field": "email"},
	}
	recorder := serveRequestID(t, "req-e1b2-meta", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, err)
	})

	_, envelope := decodeTestEnvelope(t, recorder.Body.Bytes())
	assertKeySet(t, envelope.Error, "code", "message", "messageKey", "details", "meta")
	if envelope.Error["messageKey"] != "validation.email.format" {
		t.Errorf("error.messageKey = %v, want validation.email.format", envelope.Error["messageKey"])
	}
	details, ok := envelope.Error["details"].(map[string]any)
	if !ok || details["field"] != "email" {
		t.Errorf("error.details = %v, want {field:email}", envelope.Error["details"])
	}
	meta, ok := envelope.Error["meta"].(map[string]any)
	if !ok || meta["field"] != "email" {
		t.Errorf("error.meta = %v, want {field:email}", envelope.Error["meta"])
	}
	if !reflect.DeepEqual(details, meta) {
		t.Errorf("error.details = %v, error.meta = %v, want identical values", details, meta)
	}
}

func TestWriteError_InternalDropsMessageKeyDetailsAndMeta(t *testing.T) {
	err := &apperrors.Error{
		Code:       apperrors.CodeInternal,
		Message:    "insert failed: SQLSTATE 23505 duplicate key on users_pkey",
		MessageKey: "internal.database",
		Meta:       map[string]any{"dsn": "postgres://user:secret@db/app"},
	}
	recorder := serveRequestID(t, "req-e1b2-500", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, err)
	})

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	_, envelope := decodeTestEnvelope(t, recorder.Body.Bytes())
	assertKeySet(t, envelope.Error, "code", "message")
	if envelope.Error["message"] != InternalErrorMessage {
		t.Errorf("error.message = %v, want %q", envelope.Error["message"], InternalErrorMessage)
	}
	if body := recorder.Body.String(); containsAny(body, "SQLSTATE", "users_pkey", "secret") {
		t.Errorf("response leaked internal detail: %s", body)
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if len(needle) > 0 && len(haystack) >= len(needle) {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
		}
	}
	return false
}

func TestCorrelationIDs_FallbackToRequestID(t *testing.T) {
	handler := chimw.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID, traceID := CorrelationIDs(r.Context())
		if requestID != "req-e1b2-fallback" {
			t.Errorf("requestID = %q, want req-e1b2-fallback", requestID)
		}
		if traceID != requestID {
			t.Errorf("traceID = %q, want request id fallback %q", traceID, requestID)
		}
	}))
	request := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	request.Header.Set("X-Request-Id", "req-e1b2-fallback")
	handler.ServeHTTP(httptest.NewRecorder(), request)
}
