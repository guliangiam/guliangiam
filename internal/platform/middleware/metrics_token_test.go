package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newMetricsRequest() *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	request.RemoteAddr = "10.0.0.1:1234"
	return request
}

func TestMetricsTokenMiddleware_ForbiddenWritesUnifiedErrorBody(t *testing.T) {
	handler := MetricsTokenMiddleware("expected-token")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler must not run with an invalid metrics token")
	}))

	request := newMetricsRequest()
	request.Header.Set("X-Metrics-Token", "wrong-token")
	recorder := serveMiddlewareWithRequestID(handler, request, "req-e1b2-metrics-403")

	assertUnifiedErrorBody(t, recorder, http.StatusForbidden, "forbidden", "invalid metrics token", "req-e1b2-metrics-403")
}

func TestMetricsTokenMiddleware_MissingTokenWritesUnifiedErrorBody(t *testing.T) {
	handler := MetricsTokenMiddleware("expected-token")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler must not run without a metrics token")
	}))

	recorder := serveMiddlewareWithRequestID(handler, newMetricsRequest(), "req-e1b2-metrics-401")

	assertUnifiedErrorBody(t, recorder, http.StatusUnauthorized, "unauthorized",
		"metrics token required (X-Metrics-Token or Authorization: Bearer <token>)", "req-e1b2-metrics-401")
}

func TestMetricsTokenMiddleware_AcceptsValidToken(t *testing.T) {
	handler := MetricsTokenMiddleware("expected-token")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := newMetricsRequest()
	request.Header.Set("Authorization", "Bearer expected-token")
	recorder := serveMiddlewareWithRequestID(handler, request, "req-e1b2-metrics-ok")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}
