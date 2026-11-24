package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders_DefaultConfig(t *testing.T) {
	t.Setenv("CSP_REPORT_TO_URL", "")
	cfg := DefaultSecurityHeadersConfig()
	handler := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if v := rec.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", v, "nosniff")
	}
	if v := rec.Header().Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("X-Frame-Options = %q, want %q", v, "DENY")
	}
	if v := rec.Header().Get("X-XSS-Protection"); v != "0" {
		t.Errorf("X-XSS-Protection = %q, want %q", v, "0")
	}
	if v := rec.Header().Get("Referrer-Policy"); v != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want %q", v, "strict-origin-when-cross-origin")
	}
	if v := rec.Header().Get("Permissions-Policy"); v != DefaultPermissionsPolicy {
		t.Errorf("Permissions-Policy = %q, want %q", v, DefaultPermissionsPolicy)
	}
	// The default policy must stay report-only: enforcing it breaks the SAML
	// POST binding, whose auto-submit form relies on an inline event handler.
	// See SecurityHeadersConfig.CSPReportOnly.
	if v := rec.Header().Get("Content-Security-Policy-Report-Only"); v != cfg.CSPPolicy {
		t.Errorf("Content-Security-Policy-Report-Only = %q, want %q", v, cfg.CSPPolicy)
	}
	if v := rec.Header().Get("Content-Security-Policy"); v != "" {
		t.Errorf("Content-Security-Policy = %q, want empty (default is report-only)", v)
	}
}

func TestDefaultSecurityHeadersConfig_ReadsCSPReportToURL(t *testing.T) {
	t.Setenv("CSP_REPORT_TO_URL", "https://internal.example/csp-report")
	cfg := DefaultSecurityHeadersConfig()
	if cfg.ReportToURL != "https://internal.example/csp-report" {
		t.Fatalf("ReportToURL = %q, want the CSP_REPORT_TO_URL value", cfg.ReportToURL)
	}
}

func TestSecurityHeaders_CSPReportTo(t *testing.T) {
	t.Setenv("CSP_REPORT_TO_URL", "")
	const reportURL = "https://internal.example/csp-report"
	cfg := SecurityHeadersConfig{CSPPolicy: "default-src 'self'", ReportToURL: reportURL}
	handler := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	want := "default-src 'self'; report-uri " + reportURL + "; report-to csp-endpoint"
	if v := rec.Header().Get("Content-Security-Policy"); v != want {
		t.Errorf("Content-Security-Policy = %q, want %q", v, want)
	}
	reportTo := rec.Header().Get("Report-To")
	if !strings.Contains(reportTo, `"group":"csp-endpoint"`) {
		t.Errorf("Report-To = %q, want group csp-endpoint", reportTo)
	}
	if !strings.Contains(reportTo, `"url":"`+reportURL+`"`) {
		t.Errorf("Report-To = %q, want endpoint %s", reportTo, reportURL)
	}
}

func TestSecurityHeaders_CSPEnforcedWhenNotReportOnly(t *testing.T) {
	cfg := SecurityHeadersConfig{CSPPolicy: "default-src 'none'"}
	handler := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if v := rec.Header().Get("Content-Security-Policy"); v != "default-src 'none'" {
		t.Errorf("Content-Security-Policy = %q, want %q", v, "default-src 'none'")
	}
	if v := rec.Header().Get("Content-Security-Policy-Report-Only"); v != "" {
		t.Errorf("Content-Security-Policy-Report-Only = %q, want empty when enforcing", v)
	}
}

func TestSecurityHeaders_PermissionsPolicyOmittedWhenEmpty(t *testing.T) {
	cfg := SecurityHeadersConfig{PermissionsPolicy: ""}
	handler := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if v := rec.Header().Get("Permissions-Policy"); v != "" {
		t.Errorf("Permissions-Policy = %q, want empty when unset", v)
	}
}

func TestSecurityHeaders_CustomCSP(t *testing.T) {
	cfg := SecurityHeadersConfig{
		ContentTypeNosniff: true,
		FrameDeny:          true,
		CSPPolicy:          "default-src 'none'",
		XSSProtection:      true,
		ReferrerPolicy:     "no-referrer",
	}
	handler := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if v := rec.Header().Get("Content-Security-Policy"); v != "default-src 'none'" {
		t.Errorf("Content-Security-Policy = %q, want %q", v, "default-src 'none'")
	}
	if v := rec.Header().Get("Referrer-Policy"); v != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want %q", v, "no-referrer")
	}
}

func TestSecurityHeaders_DisabledOptions(t *testing.T) {
	cfg := SecurityHeadersConfig{}
	handler := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if v := rec.Header().Get("X-Content-Type-Options"); v != "" {
		t.Errorf("X-Content-Type-Options should be empty when disabled, got %q", v)
	}
	if v := rec.Header().Get("X-Frame-Options"); v != "" {
		t.Errorf("X-Frame-Options should be empty when disabled, got %q", v)
	}
	if v := rec.Header().Get("Content-Security-Policy"); v != "" {
		t.Errorf("Content-Security-Policy should be empty when disabled, got %q", v)
	}
	if v := rec.Header().Get("X-XSS-Protection"); v != "" {
		t.Errorf("X-XSS-Protection should be empty when disabled, got %q", v)
	}
}

func TestSecurityHeaders_HSTS_OverTLS(t *testing.T) {
	cfg := SecurityHeadersConfig{
		HSTSMaxAge:         86400,
		HSTSIncludeSubs:    true,
		HSTSPreload:        true,
		ContentTypeNosniff: true,
	}
	handler := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if v := rec.Header().Get("Strict-Transport-Security"); v != "max-age=86400; includeSubDomains; preload" {
		t.Errorf("Strict-Transport-Security = %q, want %q", v, "max-age=86400; includeSubDomains; preload")
	}
}

func TestSecurityHeaders_HSTS_NoPreload(t *testing.T) {
	cfg := SecurityHeadersConfig{
		HSTSMaxAge:         86400,
		HSTSIncludeSubs:    true,
		HSTSPreload:        false,
		ContentTypeNosniff: true,
	}
	handler := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if v := rec.Header().Get("Strict-Transport-Security"); v != "max-age=86400; includeSubDomains" {
		t.Errorf("Strict-Transport-Security = %q, want %q", v, "max-age=86400; includeSubDomains")
	}
}

func TestSecurityHeaders_HSTS_NoTLS(t *testing.T) {
	cfg := SecurityHeadersConfig{
		HSTSMaxAge:         86400,
		HSTSIncludeSubs:    true,
		ContentTypeNosniff: true,
	}
	handler := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if v := rec.Header().Get("Strict-Transport-Security"); v != "" {
		t.Errorf("Strict-Transport-Security should be empty over plain HTTP, got %q", v)
	}
}
