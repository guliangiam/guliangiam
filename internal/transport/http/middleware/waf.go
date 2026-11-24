package middleware

import (
	"bytes"
	"context"

	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/types"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// DefaultWAFReloadInterval is how often the WAF rule set is re-read from the
// rules repository (BE-E-17). Rules added/removed at runtime take effect on
// the next tick instead of only after a process restart.
const DefaultWAFReloadInterval = 60 * time.Second

// WAFMiddleware inspects HTTP requests against WAF rules using the Coraza engine.
// It loads WAF-type rules from the rules repository and converts them into
// Coraza SecRule directives. On initialization failure, it returns an error
// (fail-close). At runtime, if the WAF engine is unavailable, requests are
// rejected with 503 Service Unavailable.
//
// The engine is double-buffered (BE-E-17): a background ticker rebuilds a new
// Coraza WAF from the current rule rows and swaps it under mu only on success;
// a failed rebuild keeps the previous engine (fail-closed, never fail-open).
type WAFMiddleware struct {
	ruleSvc RuleQueryService
	mu      sync.RWMutex
	waf     coraza.WAF
	logger  *slog.Logger

	reloadInterval time.Duration
	stopCh         chan struct{}
	stopOnce       sync.Once
}

// WAFOption customizes a WAFMiddleware.
type WAFOption func(*WAFMiddleware)

// WithWAFReloadInterval overrides the rule reload tick. d <= 0 disables
// periodic reload (startup-only load, the pre-BE-E-17 behaviour).
func WithWAFReloadInterval(d time.Duration) WAFOption {
	return func(m *WAFMiddleware) { m.reloadInterval = d }
}

// NewWAFMiddleware creates a new WAF middleware. If ruleSvc is nil, the
// middleware operates in no-op mode (WAF disabled but explicitly configured).
// If Coraza initialization fails, an error is returned (fail-close strategy).
func NewWAFMiddleware(ruleSvc RuleQueryService, logger *slog.Logger, opts ...WAFOption) (*WAFMiddleware, error) {
	m := &WAFMiddleware{
		ruleSvc:        ruleSvc,
		logger:         logger,
		reloadInterval: DefaultWAFReloadInterval,
		stopCh:         make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}

	if ruleSvc == nil {
		if logger != nil {
			logger.Info("WAF middleware initialized in no-op mode (ruleSvc is nil)")
		}
		return m, nil
	}

	waf, err := m.buildEngine(context.Background())
	if err != nil {
		if logger != nil {
			logger.Error("failed to initialize Coraza WAF, failing closed",
				"error", err,
			)
		}
		return nil, apperrors.WrapInternal(err, "coraza WAF initialization failed")
	}

	m.waf = waf
	if m.reloadInterval > 0 {
		recover.SafeGo("waf-rule-reload", m.reloadLoop)
	}
	return m, nil
}

// buildEngine compiles the base directives plus the current DB rule rows into
// a fresh Coraza WAF with request body access enabled.
func (m *WAFMiddleware) buildEngine(ctx context.Context) (coraza.WAF, error) {
	ruleDirectives, err := m.loadRuleDirectives(ctx)
	if err != nil {
		return nil, err
	}
	directives := buildBaseDirectives() + ruleDirectives
	return coraza.NewWAF(
		coraza.NewWAFConfig().
			WithRequestBodyAccess().
			WithRequestBodyLimit(13107200).
			WithRequestBodyInMemoryLimit(131072).
			WithDirectives(directives),
	)
}

// currentWAF reads the active engine under the swap lock.
func (m *WAFMiddleware) currentWAF() coraza.WAF {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.waf
}

func (m *WAFMiddleware) reloadLoop() {
	ticker := time.NewTicker(m.reloadInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.reload()
		case <-m.stopCh:
			return
		}
	}
}

// reload rebuilds the engine from current rules and swaps it in on success.
// On any failure the previous engine stays active (fail-closed).
func (m *WAFMiddleware) reload() {
	waf, err := m.buildEngine(context.Background())
	if err != nil {
		if m.logger != nil {
			m.logger.Error("WAF rule reload failed; keeping previous engine", "error", err)
		}
		return
	}
	m.mu.Lock()
	m.waf = waf
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("WAF rules reloaded")
	}
}

// Stop terminates the background reload goroutine. Safe to call multiple
// times and on no-op instances.
func (m *WAFMiddleware) Stop() {
	if m.stopCh == nil {
		return
	}
	m.stopOnce.Do(func() { close(m.stopCh) })
}

func buildBaseDirectives() string {
	// Coraza only auto-selects URLENCODED/MULTIPART body processors from
	// Content-Type; without these ctl rules a JSON or plain-text body is never
	// parsed, so ARGS/REQUEST_BODY stay empty and every body rule silently
	// no-ops (BE-E-03). JSON bodies populate ARGS; other non-form bodies fall
	// back to RAW which populates REQUEST_BODY. IDs sit outside the
	// ruleIDFromName range (100000..1000099999) to avoid collisions.
	return "SecRuleEngine On\n" +
		"SecRule REQUEST_HEADERS:Content-Type \"@rx ^application/json\" \"id:2000000001,phase:1,pass,t:none,ctl:requestBodyProcessor=JSON\"\n" +
		"SecRule REQUEST_HEADERS:Content-Type \"!@rx ^(application/x-www-form-urlencoded|multipart/form-data|application/json)\" \"id:2000000002,phase:1,pass,t:none,ctl:requestBodyProcessor=RAW\"\n"
}

// loadRuleDirectives loads WAF-type rules from the database and converts them to SecRule directives.
// A repository error is propagated so reload keeps the previous engine
// (fail-closed) instead of silently swapping in a rule-less one.
func (m *WAFMiddleware) loadRuleDirectives(ctx context.Context) (string, error) {
	wafRules, err := m.ruleSvc.ListEnabledByType(ctx, "WAF")
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to load WAF rules from database",
				"error", err,
			)
		}
		return "", apperrors.WrapInternal(err, "load WAF rules")
	}

	var b strings.Builder
	for _, rule := range wafRules {
		if !rule.IsEnabled {
			continue
		}

		for _, cond := range rule.Conditions {
			pattern, _ := cond["pattern"].(string)
			if pattern == "" {
				continue
			}

			// Determine target from condition, default to common targets
			target, _ := cond["target"].(string)
			if target == "" {
				target = "ARGS|URI|REQUEST_BODY"
			}

			// Action: deny for Block, pass for Allow
			action := "deny,status:403"
			if rule.Action == "Allow" {
				action = "pass"
			}

			// Use a unique rule ID based on rule name hash
			ruleID := ruleIDFromName(rule.Name)

			// Quote manually: escapeCorazaPattern already performs SecRule
			// escaping; %q would escape a second time and corrupt regex
			// patterns containing backslashes (BE-E-17).
			fmt.Fprintf(&b, "SecRule %s \"%s\" \"id:%d,%s\"\n",
				target,
				escapeCorazaPattern(pattern),
				ruleID,
				action,
			)
		}
	}

	return b.String(), nil
}

// Middleware returns an HTTP middleware that inspects requests using the Coraza WAF engine.
func (m *WAFMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No-op mode: if ruleSvc is nil, WAF is explicitly disabled; pass through.
		if m.ruleSvc == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Fail-Close: if WAF engine is not available (but ruleSvc was configured), reject the request.
		// This prevents attackers from bypassing WAF when the engine is down.
		waf := m.currentWAF()
		if waf == nil {
			route.WriteError(w, r, apperrors.NewUnavailable("WAF not available, request rejected for security"))
			return
		}

		// Create a new Coraza transaction
		tx := waf.NewTransaction()
		defer tx.Close()

		// Process connection info
		tx.ProcessConnection(r.RemoteAddr, 0, "", 0)
		tx.ProcessURI(r.URL.RequestURI(), r.Method, r.Proto)

		// Process request headers
		for k, vals := range r.Header {
			for _, v := range vals {
				tx.AddRequestHeader(k, v)
			}
		}
		// Add Host header (Go http.Request doesn't include it in Header map)
		if r.Host != "" {
			tx.AddRequestHeader("Host", r.Host)
		}

		// Phase 1-2: Check request headers
		if it := tx.ProcessRequestHeaders(); it != nil {
			m.logInterruption(r, it, "headers")
			route.WriteError(w, r, apperrors.NewForbidden("request blocked by WAF"))
			return
		}

		// Process request body. ContentLength == -1 means chunked/unknown length
		// (BE-E-03): those bodies must be inspected too — the bootstrap
		// MaxBytesReader wall keeps the read bounded. Only a declared-empty
		// body (0) skips inspection.
		if r.Body != nil && r.ContentLength != 0 {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				if m.logger != nil {
					m.logger.ErrorContext(r.Context(), "WAF: failed to read request body", "error", redact.WrapError(err))
				}
				// Fail-Close: if we cannot read the body for WAF inspection, reject the request
				route.WriteError(w, r, apperrors.NewUnavailable("WAF unable to inspect request body, request rejected for security"))
				return
			}
			// Restore body for downstream handlers
			r.Body = io.NopCloser(bytes.NewReader(body))

			it, _, err := tx.WriteRequestBody(body)
			if err != nil {
				if m.logger != nil {
					m.logger.WarnContext(r.Context(), "WAF: failed to process request body", "error", redact.WrapError(err))
				}
			}
			if it != nil {
				m.logInterruption(r, it, "body-write")
				route.WriteError(w, r, apperrors.NewForbidden("request blocked by WAF"))
				return
			}
		}

		// Phase 4: Process request body
		it, err := tx.ProcessRequestBody()
		if err != nil {
			if m.logger != nil {
				m.logger.WarnContext(r.Context(), "WAF: error processing request body", "error", redact.WrapError(err))
			}
		}
		if it != nil {
			m.logInterruption(r, it, "body")
			route.WriteError(w, r, apperrors.NewForbidden("request blocked by WAF"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *WAFMiddleware) logInterruption(r *http.Request, it *types.Interruption, phase string) {
	if m.logger == nil {
		return
	}
	m.logger.InfoContext(r.Context(), "WAF: request blocked",
		"phase", phase,
		"method", r.Method,
		"path", redact.SanitizeText(r.URL.Path),
		"query", redact.SanitizeQuery(r.URL.RawQuery),
		"remote_addr", r.RemoteAddr,
		"rule_id", it.RuleID,
		"action", it.Action,
		"status", it.Status,
	)
}

// escapeCorazaPattern makes a regex pattern safe to embed in a quoted SecRule
// operator. Coraza's directive parser only treats `\"` as an escaped quote;
// every other backslash sequence is passed through to the regex engine
// verbatim (verified against coraza v3.8.1). Therefore regex escapes such as
// `\d` must NOT be doubled, while a backslash at end-of-pattern or directly
// before a quote must be doubled so it cannot swallow the closing quote or
// the quote escape (BE-E-17).
func escapeCorazaPattern(pattern string) string {
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' && (i+1 == len(pattern) || pattern[i+1] == '"') {
			b.WriteByte('\\')
			b.WriteByte('\\')
			continue
		}
		if c == '"' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}

// ruleIDFromName generates a numeric rule ID from a rule name for Coraza.
// Coraza requires unique numeric IDs for each SecRule. FNV-1a (BE-E-17)
// replaces the old multiply-add hash whose collisions were trivial to
// construct (anagram names summed identically); the offset keeps IDs clear
// of the CRS range.
func ruleIDFromName(name string) int {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return 100000 + int(h.Sum64()%1_000_000_000)
}
