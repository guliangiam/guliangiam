package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestCSRFTokenMiddleware_SafeMethod_SetsCookie(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"GET", http.MethodGet},
		{"HEAD", http.MethodHead},
		{"OPTIONS", http.MethodOptions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			handler := CSRFTokenMiddleware()(next)

			req := httptest.NewRequest(tt.method, "/test", http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if !called {
				t.Error("next handler was not called")
			}
			cookie := findCSRFCookie(rec)
			if cookie == nil {
				t.Error("expected csrf_token cookie to be set")
			}
		})
	}
}

func TestCSRFTokenMiddleware_SafeMethod_ExistingCookie(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRFTokenMiddleware()(next)

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Cookie", csrfCookieName()+"=existing-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler was not called")
	}
	cookie := findCSRFCookie(rec)
	if cookie != nil {
		t.Error("should not set a new cookie when one already exists")
	}
}

func TestCSRFTokenMiddleware_MutatingMethod_ValidToken(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"POST", http.MethodPost},
		{"PUT", http.MethodPut},
		{"PATCH", http.MethodPatch},
		{"DELETE", http.MethodDelete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			handler := CSRFTokenMiddleware()(next)

			req := httptest.NewRequest(tt.method, "/test", http.NoBody)
			req.Header.Set("Cookie", csrfCookieName()+"=valid-token")
			req.Header.Set("X-CSRF-Token", "valid-token")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if !called {
				t.Error("next handler was not called")
			}
			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
		})
	}
}

func TestCSRFTokenMiddleware_MutatingMethod_InvalidToken(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRFTokenMiddleware()(next)

	req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	req.Header.Set("Cookie", csrfCookieName()+"=cookie-token")
	req.Header.Set("X-CSRF-Token", "wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("next handler should not be called with invalid token")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestCSRFTokenMiddleware_InvalidTokenCanonicalEnvelope(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run when the CSRF token does not match")
	})
	handler := CSRFTokenMiddleware()(next)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/users", http.NoBody)
	request.Header.Set("Cookie", csrfCookieName()+"=cookie-token")
	request.Header.Set("X-CSRF-Token", "wrong-token")

	recorder := serveMiddlewareWithRequestID(handler, request, "req-e1b3-csrf")
	assertUnifiedErrorBody(t, recorder, http.StatusForbidden,
		string(apperrors.CodeForbidden), "invalid CSRF token", "req-e1b3-csrf")
}

func TestCSRFTokenMiddleware_MutatingMethod_MissingHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRFTokenMiddleware()(next)

	req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	req.Header.Set("Cookie", csrfCookieName()+"=cookie-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("next handler should not be called without CSRF header")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestCSRFTokenMiddleware_MutatingMethod_MissingCookie(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRFTokenMiddleware()(next)

	req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	req.Header.Set("X-CSRF-Token", "some-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("next handler should not be called without CSRF cookie")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestCSRFTokenMiddleware_ValidatedBearerSessionExempt(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRFTokenMiddleware(func(context.Context) bool { return true })(next)

	req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer validated-by-session-middleware")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("validated bearer session must be exempt, called=%v status=%d", called, rec.Code)
	}
}

func TestCSRFTokenMiddleware_RawCredentialHeadersNotExempt(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		value     string
		validator func(context.Context) bool
	}{
		{name: "authorization without validator", header: "Authorization", value: "Bearer unvalidated"},
		{name: "authorization rejected by validator", header: "Authorization", value: "Bearer invalid", validator: func(context.Context) bool { return false }},
		{name: "access token without validator", header: "X-Access-Token", value: "unvalidated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
			var handler http.Handler
			if tt.validator == nil {
				handler = CSRFTokenMiddleware()(next)
			} else {
				handler = CSRFTokenMiddleware(tt.validator)(next)
			}

			req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
			req.Header.Set(tt.header, tt.value)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if called || rec.Code != http.StatusForbidden {
				t.Fatalf("raw credential header must not bypass CSRF, called=%v status=%d", called, rec.Code)
			}
		})
	}
}

func TestValidateCSRFToken(t *testing.T) {
	tests := []struct {
		name        string
		cookieValue string
		headerValue string
		want        bool
	}{
		{"matching", "token123", "token123", true},
		{"mismatch", "token123", "token456", false},
		{"empty header", "token123", "", false},
		{"whitespace header", "token123", "  ", false},
		{"header with whitespace", "token123", " token123 ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
			if tt.cookieValue != "" {
				req.Header.Set("Cookie", csrfCookieName()+"="+tt.cookieValue)
			}
			req.Header.Set("X-CSRF-Token", tt.headerValue)
			got := validateCSRFToken(req)
			if got != tt.want {
				t.Errorf("validateCSRFToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	token1, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken() error: %v", err)
	}
	if len(token1) != 64 {
		t.Errorf("token length = %d, want 64 (32 bytes hex)", len(token1))
	}
	token2, _ := generateCSRFToken()
	if token1 == token2 {
		t.Error("two generated tokens should not be equal")
	}
}

func findCSRFCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == csrfCookieName() {
			return c
		}
	}
	return nil
}
