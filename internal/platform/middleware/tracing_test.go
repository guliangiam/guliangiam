package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTracingMiddleware_CallsNext(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := TracingMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTracingMiddleware_PreservesMethodAndPath(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"GET root", http.MethodGet, "/"},
		{"POST api", http.MethodPost, "/api/users"},
		{"PUT with id", http.MethodPut, "/api/users/123"},
		{"DELETE", http.MethodDelete, "/api/items/456"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedMethod string
			var receivedPath string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				receivedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
			})

			handler := TracingMiddleware(next)
			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if receivedMethod != tt.method {
				t.Errorf("method = %q, want %q", receivedMethod, tt.method)
			}
			if receivedPath != tt.path {
				t.Errorf("path = %q, want %q", receivedPath, tt.path)
			}
		})
	}
}

func TestTracingMiddleware_SetsStatusOK(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := TracingMiddleware(next)
	req := httptest.NewRequest(http.MethodPost, "/api/resource", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestTracingMiddleware_MultipleRequests(t *testing.T) {
	var callCount int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	handler := TracingMiddleware(next)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}
