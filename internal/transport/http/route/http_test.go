package route

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
	platformmw "github.com/guliangiam/guliangiam/internal/platform/middleware"
)

type errorEnvelope struct {
	Error struct {
		Code       string         `json:"code"`
		Message    string         `json:"message"`
		MessageKey string         `json:"messageKey"`
		Meta       map[string]any `json:"meta"`
	} `json:"error"`
	RequestID string `json:"requestId"`
	TraceID   string `json:"traceId"`
}

func decodeErrorEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) errorEnvelope {
	t.Helper()
	var body errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (raw=%s)", err, recorder.Body.String())
	}
	return body
}

func TestWriteError_MatchesPlatformCanonicalEnvelope(t *testing.T) {
	const requestID = "req-e1b2-shared"
	err := apperrors.NewForbidden("access denied")

	serve := func(write func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
		handler := chimw.RequestID(http.HandlerFunc(write))
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
		request.Header.Set("X-Request-Id", requestID)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	routeRecorder := serve(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, err)
	})
	platformRecorder := serve(func(w http.ResponseWriter, r *http.Request) {
		httperr.WriteError(w, r, err)
	})

	if routeRecorder.Code != platformRecorder.Code {
		t.Fatalf("status differs: route=%d platform=%d", routeRecorder.Code, platformRecorder.Code)
	}
	if routeRecorder.Body.String() != platformRecorder.Body.String() {
		t.Fatalf("envelopes differ:\n route:    %s\n platform: %s", routeRecorder.Body.String(), platformRecorder.Body.String())
	}
}

func TestCSRFForbiddenBody_MatchesRouteWriteError(t *testing.T) {
	const requestID = "req-e1b3-shared"
	err := apperrors.NewForbidden("invalid CSRF token")

	// Same input and same request id through the two writers: the CSRF denial
	// must leave the wire byte-for-byte identical to a handler denial.
	serve := func(handler http.Handler) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/users", http.NoBody)
		request.Header.Set("X-Request-Id", requestID)
		recorder := httptest.NewRecorder()
		chimw.RequestID(handler).ServeHTTP(recorder, request)
		return recorder
	}

	csrfRecorder := serve(platformmw.CSRFTokenMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run when the CSRF token is missing")
	})))
	routeRecorder := serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, err)
	}))

	if csrfRecorder.Code != routeRecorder.Code {
		t.Fatalf("status differs: csrf=%d route=%d", csrfRecorder.Code, routeRecorder.Code)
	}
	if csrfRecorder.Header().Get("Content-Type") != routeRecorder.Header().Get("Content-Type") {
		t.Fatalf("content type differs: csrf=%q route=%q",
			csrfRecorder.Header().Get("Content-Type"), routeRecorder.Header().Get("Content-Type"))
	}
	if csrfRecorder.Body.String() != routeRecorder.Body.String() {
		t.Fatalf("envelopes differ:\n csrf:  %s\n route: %s", csrfRecorder.Body.String(), routeRecorder.Body.String())
	}
}

func TestWriteError_InternalSanitizedAndRequestCorrelated(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	request.Header.Set("X-Request-Id", "req-b1-known")

	handler := chimw.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperrors.NewInternal("insert failed: SQLSTATE 23505 duplicate key on users_pkey"))
	}))
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	body := decodeErrorEnvelope(t, recorder)
	if body.Error.Code != string(apperrors.CodeInternal) {
		t.Errorf("code = %q, want %q", body.Error.Code, apperrors.CodeInternal)
	}
	if body.Error.Message != "an internal error occurred" {
		t.Errorf("message = %q, want generic internal message", body.Error.Message)
	}
	if strings.Contains(recorder.Body.String(), "SQLSTATE") || strings.Contains(recorder.Body.String(), "users_pkey") {
		t.Errorf("response leaked internal detail: %s", recorder.Body.String())
	}
	if body.RequestID != "req-b1-known" {
		t.Errorf("requestId = %q, want req-b1-known", body.RequestID)
	}
	if body.TraceID == "" {
		t.Error("traceId must not be empty")
	}
	if body.TraceID != body.RequestID {
		t.Errorf("traceId = %q, want request id fallback %q", body.TraceID, body.RequestID)
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
	ctx := trace.ContextWithSpanContext(
		httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody).Context(),
		trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody).WithContext(ctx)
	request.Header.Set("X-Request-Id", "req-b1-span")
	recorder := httptest.NewRecorder()

	handler := chimw.RequestID(RequestMetadataMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperrors.NewNotFound("resource missing"))
	})))
	handler.ServeHTTP(recorder, request)

	body := decodeErrorEnvelope(t, recorder)
	if body.RequestID != "req-b1-span" {
		t.Errorf("requestId = %q, want req-b1-span", body.RequestID)
	}
	if body.TraceID != traceID.String() {
		t.Errorf("traceId = %q, want span trace id %q", body.TraceID, traceID.String())
	}
}

func captureRouteLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return buf
}

func decodeRouteLogEntries(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	trimmed := strings.TrimSpace(buf.String())
	if trimmed == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestWriteError_LogsServerErrorOnceWithoutInternalDetail(t *testing.T) {
	buf := captureRouteLogs(t)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Use(RequestMetadataMiddleware())
	router.Get("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperrors.NewInternal("insert failed: SQLSTATE 23505 duplicate key on users_pkey"))
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	request.Header.Set("X-Request-Id", "req-e3b-log")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	entries := decodeRouteLogEntries(t, buf)
	if len(entries) != 1 {
		t.Fatalf("5xx must produce exactly one error log record, got %d: %s", len(entries), buf.String())
	}
	entry := entries[0]
	if entry["msg"] != "http server error" {
		t.Errorf("msg = %v, want http server error", entry["msg"])
	}
	if entry["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", entry["level"])
	}
	if entry["error.code"] != string(apperrors.CodeInternal) {
		t.Errorf("error.code = %v, want %s", entry["error.code"], apperrors.CodeInternal)
	}
	if entry["request_id"] != "req-e3b-log" {
		t.Errorf("request_id = %v, want req-e3b-log", entry["request_id"])
	}
	if entry["trace_id"] != "req-e3b-log" {
		t.Errorf("trace_id = %v, want request id fallback", entry["trace_id"])
	}
	if entry["http.method"] != http.MethodGet {
		t.Errorf("http.method = %v, want GET", entry["http.method"])
	}
	if entry["http.route"] != "/api/v1/users" {
		t.Errorf("http.route = %v, want /api/v1/users", entry["http.route"])
	}
	if entry["http.status"] != float64(http.StatusInternalServerError) {
		t.Errorf("http.status = %v, want 500", entry["http.status"])
	}
	if strings.Contains(buf.String(), "SQLSTATE") || strings.Contains(buf.String(), "users_pkey") {
		t.Errorf("server error log leaked internal detail: %s", buf.String())
	}
}

func TestWriteError_DoesNotLogClientErrors(t *testing.T) {
	buf := captureRouteLogs(t)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Use(RequestMetadataMiddleware())
	router.Get("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperrors.NewForbidden("access denied"))
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	request.Header.Set("X-Request-Id", "req-e3b-403")
	router.ServeHTTP(httptest.NewRecorder(), request)

	if buf.Len() != 0 {
		t.Fatalf("4xx must not produce an ERROR log record: %s", buf.String())
	}
}

// 错误码注册表 §1：too_many_requests(429) 必须同时返回 Retry-After。值只能来自
// 上游（MetaRetryAfterSeconds），解析失败时生产者已丢弃，写端不得补默认值。
func TestWriteError_RetryAfterOnlyFromUpstreamMeta(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantHeader string
		wantMeta   bool
	}{
		{
			name:       "429 with upstream delta seconds",
			err:        apperrors.WithRetryAfterSeconds(apperrors.WithRetryable(apperrors.NewTooManyRequests("rate limited"), true), 7),
			wantStatus: http.StatusTooManyRequests,
			wantHeader: "7",
			wantMeta:   true,
		},
		{
			name:       "429 without upstream header",
			err:        apperrors.WithRetryable(apperrors.NewTooManyRequests("rate limited"), true),
			wantStatus: http.StatusTooManyRequests,
			wantHeader: "",
			wantMeta:   true,
		},
		{
			name:       "400 never carries Retry-After",
			err:        apperrors.WithRetryAfterSeconds(apperrors.NewInvalidArgument("bad request"), 9),
			wantStatus: http.StatusBadRequest,
			wantHeader: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/providers/{id}/test-connection", http.NoBody)
			handler := chimw.RequestID(RequestMetadataMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				WriteError(w, r, tt.err)
			})))
			handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if got := recorder.Header().Get("Retry-After"); got != tt.wantHeader {
				t.Fatalf("Retry-After = %q, want %q", got, tt.wantHeader)
			}
			body := decodeErrorEnvelope(t, recorder)
			_, hasRetryable := body.Error.Meta["retryable"]
			if hasRetryable != tt.wantMeta {
				t.Fatalf("error.meta.retryable present = %v, want %v (meta=%v)", hasRetryable, tt.wantMeta, body.Error.Meta)
			}
			if tt.wantMeta && body.Error.Meta["retryable"] != true {
				t.Fatalf("error.meta.retryable = %v, want true", body.Error.Meta["retryable"])
			}
		})
	}
}

func TestWriteCreated_SetsLocationAndCreatedBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteCreated(recorder, "/api/v1/users/u-1", map[string]string{"id": "u-1", "status": "created"})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if got := recorder.Header().Get("Location"); got != "/api/v1/users/u-1" {
		t.Fatalf("Location = %q, want /api/v1/users/u-1", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (raw=%s)", err, recorder.Body.String())
	}
	if body["id"] != "u-1" || body["status"] != "created" {
		t.Fatalf("body = %v, want the unchanged resource representation", body)
	}
}

func TestWriteCreated_EmptyLocationOmitsHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteCreated(recorder, "", map[string]string{"status": "created"})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if got := recorder.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want header to be absent", got)
	}
	if recorder.Body.Len() == 0 {
		t.Fatal("body must still be written when Location is empty")
	}
}

func TestWriteJSON_UnchangedByWriteCreated(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteJSON(recorder, http.StatusOK, map[string]string{"status": "ok"})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Location"); got != "" {
		t.Fatalf("WriteJSON must not set Location, got %q", got)
	}
}
