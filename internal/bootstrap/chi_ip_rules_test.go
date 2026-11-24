package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	httpmiddleware "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
)

type stubIPRuleService struct {
	entries []httpmiddleware.RuleEntry
	err     error
}

func (s stubIPRuleService) ListEnabledByType(_ context.Context, _ string) ([]httpmiddleware.RuleEntry, error) {
	return s.entries, s.err
}

// BE-E-04：IPRulesMiddleware 必须真实挂载到 root 链——Block 命中 403、
// Allow 优先放行、未装配（合成 Runtime）不 panic。
func TestNewChiRouterMountsIPRules(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")

	blockRule := httpmiddleware.RuleEntry{
		Name: "block-test-net", Action: "Block", IsEnabled: true,
		Conditions: []map[string]any{{"field": "ip", "operator": "cidr", "value": "203.0.113.0/24"}},
	}
	allowRule := httpmiddleware.RuleEntry{
		Name: "allow-host", Action: "Allow", IsEnabled: true,
		Conditions: []map[string]any{{"field": "ip", "operator": "eq", "value": "203.0.113.7"}},
	}

	cases := []struct {
		name    string
		entries []httpmiddleware.RuleEntry
		ip      string
		want    int
	}{
		{"blocked IP denied", []httpmiddleware.RuleEntry{blockRule}, "203.0.113.9:1234", http.StatusForbidden},
		{"unmatched IP passes", []httpmiddleware.RuleEntry{blockRule}, "198.51.100.10:1234", http.StatusNoContent},
		{"allow overrides block", []httpmiddleware.RuleEntry{blockRule, allowRule}, "203.0.113.7:1234", http.StatusNoContent},
		{"no rules passes", nil, "203.0.113.9:1234", http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := NewChiRouter(&Runtime{Middleware: MiddlewareConfig{
				WAFMiddleware:     &httpmiddleware.WAFMiddleware{},
				ServiceIPLimiter:  &httpmiddleware.ServiceIPLimiter{},
				IPRulesMiddleware: httpmiddleware.NewIPRulesMiddleware(stubIPRuleService{entries: tc.entries}),
			}})
			router.Get("/_test/ip-rules", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/_test/ip-rules", http.NoBody)
			req.RemoteAddr = tc.ip
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestNewChiRouterNilIPRulesPassthrough(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	router := NewChiRouter(&Runtime{Middleware: MiddlewareConfig{
		WAFMiddleware:    &httpmiddleware.WAFMiddleware{},
		ServiceIPLimiter: &httpmiddleware.ServiceIPLimiter{},
	}})
	router.Get("/_test/ip-rules-nil", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/_test/ip-rules-nil", http.NoBody)
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
