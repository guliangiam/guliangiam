package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mutableRuleSvc is a thread-safe RuleQueryService whose rule set (and error
// behaviour) can be changed at runtime to exercise the WAF reload loop.
type mutableRuleSvc struct {
	mu    sync.Mutex
	rules []RuleEntry
	err   error
	calls int
}

func (s *mutableRuleSvc) ListEnabledByType(_ context.Context, _ string) ([]RuleEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	out := make([]RuleEntry, len(s.rules))
	copy(out, s.rules)
	return out, nil
}

func (s *mutableRuleSvc) setRules(rules []RuleEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = rules
}

func (s *mutableRuleSvc) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func (s *mutableRuleSvc) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func blockBodyRule(pattern string) RuleEntry {
	return RuleEntry{
		Name:      "block-" + pattern,
		Action:    "Block",
		IsEnabled: true,
		Conditions: []map[string]any{
			// JSON bodies are parsed into ARGS by the JSON body processor;
			// REQUEST_BODY covers RAW (non-JSON, non-form) bodies.
			{"pattern": pattern, "target": "ARGS|REQUEST_BODY"},
		},
	}
}

// BE-E-03: chunked requests (ContentLength == -1) must be inspected too.
func TestWAFMiddleware_ChunkedAttackBodyBlocked(t *testing.T) {
	svc := &mutableRuleSvc{rules: []RuleEntry{blockBodyRule("evilpayload")}}
	m, err := NewWAFMiddleware(svc, nil, WithWAFReloadInterval(0))
	require.NoError(t, err)
	t.Cleanup(m.Stop)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/users", io.NopCloser(strings.NewReader(`{"data":"evilpayload"}`)))
	req.ContentLength = -1 // chunked / unknown length
	req.TransferEncoding = []string{"chunked"}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestWAFMiddleware_ChunkedCleanBodyPassesIntact(t *testing.T) {
	svc := &mutableRuleSvc{rules: []RuleEntry{blockBodyRule("evilpayload")}}
	m, err := NewWAFMiddleware(svc, nil, WithWAFReloadInterval(0))
	require.NoError(t, err)
	t.Cleanup(m.Stop)

	payload := `{"data":"benign"}`
	var seen string
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/users", io.NopCloser(strings.NewReader(payload)))
	req.ContentLength = -1
	req.TransferEncoding = []string{"chunked"}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, payload, seen, "body must be restored intact for downstream handlers")
}

// Known-length attack body is still blocked (behaviour unchanged).
func TestWAFMiddleware_KnownLengthAttackBodyBlocked(t *testing.T) {
	svc := &mutableRuleSvc{rules: []RuleEntry{blockBodyRule("evilpayload")}}
	m, err := NewWAFMiddleware(svc, nil, WithWAFReloadInterval(0))
	require.NoError(t, err)
	t.Cleanup(m.Stop)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("POST", "/api/v1/users", strings.NewReader(`{"data":"evilpayload"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// BE-E-17: coraza passes backslash sequences through to the regex engine
// verbatim except `\"`; only end-of-pattern and pre-quote backslashes are
// doubled so regex escapes such as \d keep their semantics.
func TestEscapeCorazaPattern_Backslash(t *testing.T) {
	assert.Equal(t, `a\b`, escapeCorazaPattern(`a\b`))
	assert.Equal(t, `foo\\`, escapeCorazaPattern(`foo\`))
	assert.Equal(t, `\\\"`, escapeCorazaPattern(`\"`))
	assert.Equal(t, `\d+`, escapeCorazaPattern(`\d+`))
}

// BE-E-17 regression: a regex pattern containing backslashes must survive
// directive generation exactly once-escaped (the old %q double-escaped it).
func TestWAFMiddleware_BackslashRegexRuleWorks(t *testing.T) {
	svc := &mutableRuleSvc{rules: []RuleEntry{{
		Name:      "digits",
		Action:    "Block",
		IsEnabled: true,
		Conditions: []map[string]any{
			{"pattern": `attack\d+`, "target": "ARGS"},
		},
	}}}
	m, err := NewWAFMiddleware(svc, nil, WithWAFReloadInterval(0))
	require.NoError(t, err)
	t.Cleanup(m.Stop)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/users?q=attack123", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code, `\d must keep regex semantics after escaping`)

	req = httptest.NewRequest("GET", "/api/v1/users?q=attackXYZ", http.NoBody)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// A quote in the pattern must not break out of the SecRule argument.
// text/plain body → RAW processor → REQUEST_BODY holds the raw payload.
func TestWAFMiddleware_QuotePatternDoesNotInject(t *testing.T) {
	svc := &mutableRuleSvc{rules: []RuleEntry{blockBodyRule(`evil",pass"`)}}
	m, err := NewWAFMiddleware(svc, nil, WithWAFReloadInterval(0))
	require.NoError(t, err, "pattern with quotes must compile into a valid rule")
	t.Cleanup(m.Stop)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("POST", "/api/v1/users", strings.NewReader(`evil",pass"`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// REQUEST_BODY-targeted rules inspect non-JSON bodies via the RAW processor.
func TestWAFMiddleware_RawBodyRuleBlocks(t *testing.T) {
	svc := &mutableRuleSvc{rules: []RuleEntry{{
		Name:      "raw-block",
		Action:    "Block",
		IsEnabled: true,
		Conditions: []map[string]any{
			{"pattern": "evilpayload", "target": "REQUEST_BODY"},
		},
	}}}
	m, err := NewWAFMiddleware(svc, nil, WithWAFReloadInterval(0))
	require.NoError(t, err)
	t.Cleanup(m.Stop)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("POST", "/api/v1/users", strings.NewReader("evilpayload"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// BE-E-17: runtime rule changes take effect on the reload tick.
func TestWAFMiddleware_ReloadPicksUpNewRules(t *testing.T) {
	svc := &mutableRuleSvc{}
	m, err := NewWAFMiddleware(svc, nil, WithWAFReloadInterval(10*time.Millisecond))
	require.NoError(t, err)
	t.Cleanup(m.Stop)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	do := func() int {
		req := httptest.NewRequest("POST", "/api/v1/users", strings.NewReader(`{"data":"evilpayload"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, do(), "no rules yet → pass")

	svc.setRules([]RuleEntry{blockBodyRule("evilpayload")})
	require.Eventually(t, func() bool { return do() == http.StatusForbidden },
		2*time.Second, 20*time.Millisecond, "new rule must take effect after a reload tick")

	// Reload failure keeps the previous engine (fail-closed, never fail-open).
	svc.setErr(assert.AnError)
	base := svc.callCount()
	require.Eventually(t, func() bool { return svc.callCount() > base+1 },
		2*time.Second, 20*time.Millisecond)
	assert.Equal(t, http.StatusForbidden, do(), "failed reload must keep blocking engine")

	// Rule removal takes effect too.
	svc.setErr(nil)
	svc.setRules(nil)
	require.Eventually(t, func() bool { return do() == http.StatusOK },
		2*time.Second, 20*time.Millisecond, "removed rule must stop blocking after reload")
}

func TestWAFMiddleware_StopIdempotent(t *testing.T) {
	svc := &mutableRuleSvc{}
	m, err := NewWAFMiddleware(svc, nil, WithWAFReloadInterval(10*time.Millisecond))
	require.NoError(t, err)
	m.Stop()
	m.Stop() // must not panic

	// No-op instance: Stop is also safe.
	noop, err := NewWAFMiddleware(nil, nil)
	require.NoError(t, err)
	noop.Stop()
	noop.Stop()
}
