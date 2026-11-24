package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCSRFTokenMiddlewareWithExemption_ExemptPathReachesHandler locks the
// exemption used by the client-authenticated token endpoints: a POST without
// any cookie (neither session nor double-submit) must reach the handler.
func TestCSRFTokenMiddlewareWithExemption_ExemptPathReachesHandler(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRFTokenMiddlewareWithExemption(
		func(r *http.Request) bool { return r.URL.Path == "/api/v1/auth/tokens/exchange" },
		func(context.Context) bool { return false },
	)(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("exempt path must reach the handler, called=%v status=%d", called, rec.Code)
	}
}

// TestCSRFTokenMiddlewareWithExemption_OtherPathsStayProtected proves the
// exemption is not a bypass: a sibling path and a non-exempt request still
// require the double-submit token.
func TestCSRFTokenMiddlewareWithExemption_OtherPathsStayProtected(t *testing.T) {
	exempt := func(r *http.Request) bool {
		return r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/tokens/exchange"
	}
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "sibling path", method: http.MethodPost, path: "/api/v1/auth/tokens/introspect"},
		{name: "path prefix variant", method: http.MethodPost, path: "/api/v1/auth/tokens/exchange/extra"},
		{name: "wrong method", method: http.MethodPut, path: "/api/v1/auth/tokens/exchange"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			handler := CSRFTokenMiddlewareWithExemption(exempt, func(context.Context) bool { return false })(next)

			req := httptest.NewRequest(tc.method, tc.path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if called || rec.Code != http.StatusForbidden {
				t.Fatalf("non-exempt request must stay protected, called=%v status=%d", called, rec.Code)
			}
		})
	}
}
