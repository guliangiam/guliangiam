package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	platformclock "github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/logging"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	httputil "github.com/guliangiam/guliangiam/internal/transport/http/util"
)

const (
	passwordChangeURL    = "/account"
	passwordGateCacheTTL = 30 * time.Second
)

var passwordChangeWarnTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "password_change_warn_total",
	Help: "Requests that would be blocked by the password-change gate in warn mode.",
})

// passwordGateFailOpenTotal is the alerting surface for the gate's single
// fail-open branch: the credential state could not be read (infrastructure
// failure). It must not be treated as normal traffic.
var passwordGateFailOpenTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "password_gate_fail_open_total",
	Help: "Requests allowed because the password gate could not read credential state (infrastructure failure).",
})

// PasswordGateMode is the AUTH_PASSWORD_CHANGE_ENFORCEMENT setting.
type PasswordGateMode string

const (
	PasswordGateOff     PasswordGateMode = "off"
	PasswordGateWarn    PasswordGateMode = "warn"
	PasswordGateEnforce PasswordGateMode = "enforce"
)

// DefaultPasswordGateMode applies when AUTH_PASSWORD_CHANGE_ENFORCEMENT is
// unset (and when an unrecognised value is supplied). Product contract: seeds
// ship need_update_password=true and the reset UI promises the target user must
// set a new password at next sign-in — an unconfigured deployment must therefore
// enforce the change, or that promise is false.
const DefaultPasswordGateMode = PasswordGateEnforce

// ParsePasswordGateMode normalises the raw setting. Unset and invalid values
// resolve to the default (enforce); the second return value is false only for
// invalid values so the caller can log a warning.
func ParsePasswordGateMode(raw string) (PasswordGateMode, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "warn":
		return PasswordGateWarn, true
	case "enforce":
		return PasswordGateEnforce, true
	case "off":
		return PasswordGateOff, true
	case "":
		return DefaultPasswordGateMode, true
	default:
		return DefaultPasswordGateMode, false
	}
}

// PasswordStateProvider is the app-layer read/audit surface used by the gate.
type PasswordStateProvider interface {
	GetPasswordState(ctx context.Context, query appauth.PasswordStateQuery) (appauth.PasswordState, error)
	RecordPasswordGateBypass(ctx context.Context, input appauth.PasswordGateBypassInput) error
}

// ImpersonationResolver resolves impersonation metadata from an opaque token.
type ImpersonationResolver interface {
	ResolveImpersonation(ctx context.Context, accessToken string) (*appauth.ImpersonationInfo, error)
}

type passwordGateCacheEntry struct {
	state     appauth.PasswordState
	expiresAt time.Time
}

// PasswordGate blocks non-exempt routes while a password change is required.
// The password state is cached per session for 30s; the accepted fail-open
// window after an administrator forces the flag is bounded by that TTL.
type PasswordGate struct {
	mode          PasswordGateMode
	states        PasswordStateProvider
	impersonation ImpersonationResolver
	clock         platformclock.Clock
	ttl           time.Duration

	mu    sync.Mutex
	cache map[string]passwordGateCacheEntry
}

// NewPasswordGate builds the middleware. rawMode is the raw env value.
func NewPasswordGate(states PasswordStateProvider, impersonation ImpersonationResolver, rawMode string, clk platformclock.Clock) *PasswordGate {
	mode, valid := ParsePasswordGateMode(rawMode)
	if !valid {
		slog.Warn("invalid AUTH_PASSWORD_CHANGE_ENFORCEMENT; falling back to default", "value", rawMode, "default", string(DefaultPasswordGateMode))
	}
	if clk == nil {
		clk = platformclock.SystemClock{}
	}
	return &PasswordGate{
		mode:          mode,
		states:        states,
		impersonation: impersonation,
		clock:         clk,
		ttl:           passwordGateCacheTTL,
		cache:         make(map[string]passwordGateCacheEntry),
	}
}

// Middleware implements the gate. It never changes existing handlers and is
// a strict no-op when the mode is off.
func (g *PasswordGate) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || g.mode == PasswordGateOff {
			next.ServeHTTP(w, r)
			return
		}

		if PasswordChangeExempt(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		session, ok := SessionContextFromContext(r.Context())
		if !ok {
			g.recordImpersonationBypass(r, SessionContext{}, nil)
			next.ServeHTTP(w, r)
			return
		}

		// 门禁排在 TenantFilterMiddleware 之前，此时请求上下文还没有 RLS 租户
		// 作用域。门禁只读当前会话用户自己的凭据，用会话所属组织钉住作用域即可；
		// 缺作用域时 RLS 会把凭据行过滤成 NotFound，门禁会 fail-open 放行。
		gateCtx := withGateTenantScope(r.Context(), session)

		if isPasswordGateExemptPrincipal(session.PrincipalType) {
			next.ServeHTTP(w, r)
			return
		}

		if info := g.resolveImpersonation(gateCtx, session.AccessToken); info != nil {
			g.recordImpersonationBypass(r.WithContext(gateCtx), session, info)
			next.ServeHTTP(w, r)
			return
		}

		// Bearer/API-token principals have no session row, so there is no
		// session-keyed cache; the contract exempts them from the gate.
		if session.ID == "" || session.UserID == "" || session.OrganizationID == "" {
			next.ServeHTTP(w, r)
			return
		}

		state, err := g.passwordState(gateCtx, session)
		if err != nil {
			// Fail open, deliberately: only an infrastructure read failure reaches
			// here. The query inputs are the server-side session identities, never
			// request content, so a caller cannot induce this branch. Availability
			// wins over enforcement, but the event must stay alertable.
			passwordGateFailOpenTotal.Inc()
			slog.ErrorContext(r.Context(), "password gate state lookup failed; allowing request",
				"event", "password_gate_fail_open",
				"error", err, "session_id", session.ID, "user_id", session.UserID)
			next.ServeHTTP(w, r)
			return
		}
		if !state.MustChange {
			next.ServeHTTP(w, r)
			return
		}

		switch g.mode {
		case PasswordGateWarn:
			passwordChangeWarnTotal.Inc()
			slog.WarnContext(r.Context(), "password change required (warn mode)",
				"reason", state.Reason, "session_id", session.ID, "user_id", session.UserID,
				"method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r)
		case PasswordGateEnforce:
			code, message := passwordGateError(state.Reason)
			route.WriteError(w, r, apperrors.NewWithMeta(
				code,
				message,
				map[string]any{
					"reason":            state.Reason,
					"changePasswordUrl": passwordChangeURL,
				},
			))
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func (g *PasswordGate) passwordState(ctx context.Context, session SessionContext) (appauth.PasswordState, error) {
	now := g.clock.Now()

	g.mu.Lock()
	if entry, ok := g.cache[session.ID]; ok && now.Before(entry.expiresAt) {
		g.mu.Unlock()
		return entry.state, nil
	}
	g.mu.Unlock()

	state, err := g.states.GetPasswordState(ctx, appauth.PasswordStateQuery{
		UserID:         session.UserID,
		OrganizationID: session.OrganizationID,
	})
	if err != nil {
		return appauth.PasswordState{}, err
	}

	g.mu.Lock()
	if len(g.cache) > 1024 {
		for key, entry := range g.cache {
			if !now.Before(entry.expiresAt) {
				delete(g.cache, key)
			}
		}
	}
	g.cache[session.ID] = passwordGateCacheEntry{state: state, expiresAt: now.Add(g.ttl)}
	g.mu.Unlock()

	return state, nil
}

func (g *PasswordGate) resolveImpersonation(ctx context.Context, accessToken string) *appauth.ImpersonationInfo {
	if g.impersonation == nil || strings.TrimSpace(accessToken) == "" {
		return nil
	}
	info, err := g.impersonation.ResolveImpersonation(ctx, accessToken)
	if err != nil {
		slog.WarnContext(ctx, "password gate impersonation lookup failed; treating as non-impersonation", "error", err)
		return nil
	}
	return info
}

func (g *PasswordGate) recordImpersonationBypass(r *http.Request, session SessionContext, info *appauth.ImpersonationInfo) {
	if info == nil {
		return
	}
	ids := logging.RequestIDsFrom(r.Context())
	err := g.states.RecordPasswordGateBypass(r.Context(), appauth.PasswordGateBypassInput{
		OrganizationID: firstNonEmpty(info.OrganizationID, session.OrganizationID),
		SubjectUserID:  firstNonEmpty(info.SubjectUserID, session.UserID),
		ImpersonatorID: info.ImpersonatorID,
		SessionID:      session.ID,
		Method:         r.Method,
		Path:           r.URL.Path,
		RequestID:      ids.RequestID,
		TraceID:        ids.TraceID,
		IPAddress:      httputil.ClientIP(r),
		UserAgent:      r.UserAgent(),
	})
	if err != nil {
		slog.WarnContext(r.Context(), "password gate bypass audit write failed", "error", err)
	}
}

// passwordGateError maps the evaluation reason to its distinct 403 code and
// message. The two codes are never interchangeable: need_update_password means
// an administrator forced the change, max_age_exceeded means the policy aged
// out. Messages stay distinct as well so the wire body matches the code.
func passwordGateError(reason string) (apperrors.Code, string) {
	if reason == appauth.PasswordChangeReasonMaxAgeExceeded {
		return apperrors.CodePasswordExpired, "password expired"
	}
	return apperrors.CodePasswordChangeRequired, "password change required"
}

func isPasswordGateExemptPrincipal(principalType string) bool {
	switch strings.ToLower(strings.TrimSpace(principalType)) {
	case "service_account", "system":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// withGateTenantScope pins the RLS tenant scope for the gate's own reads.
//
// The gate runs before TenantFilterMiddleware, so the request context carries no
// tenant scope yet. Every read it performs is about the signed-in user's own
// credentials, so the session organization is the correct scope. Without it the
// forced-tenant RLS policies filter the credential rows out and the gate would
// silently fail open.
func withGateTenantScope(ctx context.Context, session SessionContext) context.Context {
	if existing := tenantctx.TenantScopeFromContext(ctx); existing.OrganizationID != "" ||
		existing.OrganizationName != "" ||
		existing.Mode == tenantctx.TenantFilterSystem ||
		existing.Mode == tenantctx.TenantFilterSuperAdmin {
		return ctx
	}
	if session.OrganizationID == "" && session.OrganizationSlug == "" {
		return ctx
	}
	return tenantctx.WithTenantScope(ctx, session.OrganizationID, session.OrganizationSlug, tenantctx.TenantFilterOptional)
}
