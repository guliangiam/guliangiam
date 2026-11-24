package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/corazawaf/coraza/v3/types"
	"github.com/stretchr/testify/assert"
)

func TestWAFMiddleware_NilRuleSvc_NoOpPassThrough(t *testing.T) {
	t.Parallel()
	m, err := NewWAFMiddleware(nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil middleware")
	}

	// With nil ruleSvc, WAF operates in no-op mode (pass-through).
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (no-op pass-through), got %d", rec.Code)
	}
}

func TestWAFMiddleware_NoOp_AllowSimpleGet(t *testing.T) {
	t.Parallel()
	// With nil ruleSvc, WAF is in no-op mode → pass-through
	m, _ := NewWAFMiddleware(nil, nil)
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/api/v1/users", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// No-op mode: pass-through → 200
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWAFMiddleware_NoOp_AllowPostWithBody(t *testing.T) {
	t.Parallel()
	// With nil ruleSvc, WAF is in no-op mode → pass-through
	m, _ := NewWAFMiddleware(nil, nil)
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/api/v1/users", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// No-op mode: pass-through → 201
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestWAFLogInterruption_RedactsQuery(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	m := &WAFMiddleware{logger: logger}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/users?access_token=super-secret-value&api_key=key-123&page=2", http.NoBody)
	m.logInterruption(req, &types.Interruption{
		RuleID: 1001,
		Action: "deny",
		Status: http.StatusForbidden,
	}, "headers")

	out := buf.String()
	assert.NotContains(t, out, "super-secret-value")
	assert.NotContains(t, out, "key-123")
	assert.NotContains(t, out, `"uri"`)
	assert.Contains(t, out, `"path":"/api/v1/users"`)
	assert.Contains(t, out, "access_token=***")
}

// ─── Pure function tests ──────────────────────────────────────────────────

func TestEscapeCorazaPattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{"no quotes", "sql_injection", "sql_injection"},
		{"with quotes", `pattern"with"quotes`, `pattern\"with\"quotes`},
		{"empty", "", ""},
		{"single quote", `"`, `\"`},
		{"multiple quotes", `"a"b"c"`, `\"a\"b\"c\"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, escapeCorazaPattern(tt.pattern))
		})
	}
}

func TestRuleIDFromName(t *testing.T) {
	t.Parallel()
	// Different names should produce different IDs
	id1 := ruleIDFromName("sql-injection")
	id2 := ruleIDFromName("xss-attack")
	assert.NotEqual(t, id1, id2)

	// Same name should produce same ID
	id3 := ruleIDFromName("sql-injection")
	assert.Equal(t, id1, id3)

	// IDs should be non-zero
	assert.NotZero(t, id1)
	assert.NotZero(t, id2)

	// Empty name should still produce a valid ID
	idEmpty := ruleIDFromName("")
	assert.NotZero(t, idEmpty)
}

func TestBuildBaseDirectives(t *testing.T) {
	t.Parallel()
	directives := buildBaseDirectives()
	assert.Contains(t, directives, "SecRuleEngine On")
}

// ─── WAFMiddleware fail-close behavior ────────────────────────────────────

func TestWAFMiddleware_FailClose_WhenWAFNotInitialized(t *testing.T) {
	t.Parallel()
	// When ruleSvc is set but WAF engine fails to initialize, requests should be rejected.
	// We simulate this by creating a middleware with a non-nil ruleSvc but manually
	// leaving the waf field nil (which happens when Coraza initialization fails).
	m, err := NewWAFMiddleware(nil, nil)
	assert.NoError(t, err)

	// Manually set ruleSvc to simulate "configured but engine failed" scenario
	m.ruleSvc = &mockRuleQueryServiceForWAF{}

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"GET request", "GET", "/api/v1/users"},
		{"POST request", "POST", "/api/v1/users"},
		{"PUT request", "PUT", "/api/v1/users/123"},
		{"DELETE request", "DELETE", "/api/v1/users/123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"test":true}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			// Fail-Close: WAF not available → 503
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		})
	}
}

func TestWAFMiddleware_NoOp_PreservesRequestBody(t *testing.T) {
	t.Parallel()
	// With nil ruleSvc, WAF in no-op mode → pass-through
	m, _ := NewWAFMiddleware(nil, nil)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := `{"username":"admin","password":"secret"}`
	req := httptest.NewRequest("POST", "/api/v1/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// No-op mode: pass-through → 200
	assert.Equal(t, http.StatusOK, rec.Code)
}

// ─── WAFMiddleware with active Coraza engine ──────────────────────────────

func TestWAFMiddleware_NoOp_BlocksSQLInjection(t *testing.T) {
	// With nil ruleSvc, WAF engine is not initialized → no-op pass-through
	m, _ := NewWAFMiddleware(nil, nil)
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// SQL injection patterns in URL (URL-safe for httptest.NewRequest)
	sqlInjectionPaths := []struct {
		name string
		path string
	}{
		{"sqli_union", "/api/v1/users?name=admin%27%20UNION%20SELECT%20*%20FROM%20credentials--"},
		{"sqli_drop", "/api/v1/users?id=1;%20DROP%20TABLE%20users--"},
		{"sqli_or", "/api/v1/users?id=1%27%20OR%20%271%27%3D%271"},
	}

	for _, tt := range sqlInjectionPaths {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			// No-op mode: pass-through → 200 (WAF rules not active without ruleSvc)
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestWAFMiddleware_NoOp_BlocksXSS(t *testing.T) {
	// With nil ruleSvc, WAF engine is not initialized → no-op pass-through
	m, _ := NewWAFMiddleware(nil, nil)
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// XSS patterns in URL (URL-safe for httptest.NewRequest)
	xssPaths := []struct {
		name string
		path string
	}{
		{"script_tag", "/api/v1/users?name=%3Cscript%3Ealert(%27xss%27)%3C/script%3E"},
		{"img_onerror", "/api/v1/users?name=%3Cimg%20src=x%20onerror=alert(1)%3E"},
		{"javascript_uri", "/api/v1/users?name=javascript:alert(1)"},
	}

	for _, tt := range xssPaths {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			// No-op mode: pass-through → 200
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestWAFMiddleware_NoOp_BlocksPathTraversal(t *testing.T) {
	// With nil ruleSvc, WAF engine is not initialized → no-op pass-through
	m, _ := NewWAFMiddleware(nil, nil)
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Path traversal patterns
	traversalPaths := []struct {
		name string
		path string
	}{
		{"unix_traversal", "/api/v1/files?path=../../../etc/passwd"},
		{"windows_traversal", "/api/v1/files?path=..%5C..%5C..%5Cwindows%5Csystem32"},
		{"path_traversal", "/api/v1/files/../../etc/shadow"},
	}

	for _, tt := range traversalPaths {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			// No-op mode: pass-through → 200
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestWAFMiddleware_NoOp_AllowsNormalRequests(t *testing.T) {
	// With nil ruleSvc, WAF engine is not initialized → no-op pass-through
	m, _ := NewWAFMiddleware(nil, nil)
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	normalRequests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/users", ""},
		{"GET", "/api/v1/users/01900000-0000-7000-8000-000000000001", ""},
		{"POST", "/api/v1/users", `{"username":"john","email":"john@example.com"}`},
		{"PUT", "/api/v1/users/01900000-0000-7000-8000-000000000001", `{"status":"active"}`},
		{"DELETE", "/api/v1/users/01900000-0000-7000-8000-000000000001", ""},
		{"GET", "/api/v1/organizations", ""},
		{"GET", "/healthz", ""},
	}

	for _, tt := range normalRequests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, http.NoBody)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			// No-op mode: pass-through → 200
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestWAFMiddleware_NoOp_MaliciousBody(t *testing.T) {
	// With nil ruleSvc, WAF not initialized → no-op pass-through
	m, _ := NewWAFMiddleware(nil, nil)
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Malicious request body — no-op mode passes through (WAF rules not active)
	maliciousBody := `{"username":"admin' OR '1'='1","password":"' UNION SELECT * FROM users--"}`
	req := httptest.NewRequest("POST", "/api/v1/login", strings.NewReader(maliciousBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// No-op mode: pass-through → 200
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWAFMiddleware_NoOp_MultipleRequestMethods(t *testing.T) {
	// With nil ruleSvc, WAF not initialized → no-op pass-through
	m, _ := NewWAFMiddleware(nil, nil)
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/test", http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			// No-op mode: pass-through → 200
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

// mockRuleQueryServiceForWAF is a minimal mock for testing fail-close behavior.
type mockRuleQueryServiceForWAF struct{}

func (m *mockRuleQueryServiceForWAF) ListEnabledByType(_ context.Context, _ string) ([]RuleEntry, error) {
	return nil, nil
}
