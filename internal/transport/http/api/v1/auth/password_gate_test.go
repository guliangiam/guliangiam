package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

type fakePasswordGateClock struct {
	now time.Time
}

func (f fakePasswordGateClock) Now() time.Time { return f.now }

type fakePasswordGateService struct {
	state         appauth.PasswordState
	stateErr      error
	impersonation *appauth.ImpersonationInfo
	audits        []appauth.PasswordGateBypassInput
	stateCalls    int
	lastScope     tenantctx.TenantScope
}

func (f *fakePasswordGateService) GetPasswordState(ctx context.Context, _ appauth.PasswordStateQuery) (appauth.PasswordState, error) {
	f.stateCalls++
	f.lastScope = tenantctx.TenantScopeFromContext(ctx)
	return f.state, f.stateErr
}

func (f *fakePasswordGateService) RecordPasswordGateBypass(_ context.Context, input appauth.PasswordGateBypassInput) error {
	f.audits = append(f.audits, input)
	return nil
}

func (f *fakePasswordGateService) ResolveImpersonation(_ context.Context, _ string) (*appauth.ImpersonationInfo, error) {
	return f.impersonation, nil
}

func newGateTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

func runPasswordGate(t *testing.T, gate *PasswordGate, method, path string, session *SessionContext) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if session != nil {
		req = req.WithContext(WithSessionContext(req.Context(), *session))
	}
	rec := httptest.NewRecorder()
	gate.Middleware(newGateTestHandler()).ServeHTTP(rec, req)
	return rec
}

func decodeGateError(t *testing.T, rec *httptest.ResponseRecorder) (string, map[string]any) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (body=%s)", err, rec.Body.String())
	}
	return body.Error.Code, body.Error.Details
}

func repositorySession() SessionContext {
	return SessionContext{
		ID:             "session-1",
		UserID:         "user-1",
		OrganizationID: "org-1",
		Source:         "repository",
		PrincipalType:  "user",
	}
}

func TestPasswordGateEnforceBlocksNonExemptRoute(t *testing.T) {
	service := &fakePasswordGateService{state: appauth.PasswordState{
		MustChange: true,
		Reason:     appauth.PasswordChangeReasonNeedUpdatePassword,
	}}
	session := repositorySession()
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodGet, "/api/v1/users", &session)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	code, details := decodeGateError(t, rec)
	if code != "password_change_required" {
		t.Fatalf("code = %q, want password_change_required", code)
	}
	if details["reason"] != appauth.PasswordChangeReasonNeedUpdatePassword {
		t.Fatalf("details.reason = %v", details["reason"])
	}
	if details["changePasswordUrl"] != "/account" {
		t.Fatalf("details.changePasswordUrl = %v", details["changePasswordUrl"])
	}
}

// TestPasswordGateBlocksAuthorizeForGatedSession pins where a gated user is
// stopped in the authorization-code + PKCE flow: at /authorize, the
// user-mediated step that mints the code — not at the client-authenticated
// token exchange, which the client reaches only after holding a code.
func TestPasswordGateBlocksAuthorizeForGatedSession(t *testing.T) {
	service := &fakePasswordGateService{state: appauth.PasswordState{
		MustChange: true,
		Reason:     appauth.PasswordChangeReasonNeedUpdatePassword,
	}}
	session := repositorySession()
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodPost, "/api/v1/auth/authorize", &session)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	code, details := decodeGateError(t, rec)
	if code != "password_change_required" {
		t.Fatalf("code = %q, want password_change_required", code)
	}
	if details["changePasswordUrl"] != passwordChangeURL {
		t.Fatalf("details.changePasswordUrl = %v", details["changePasswordUrl"])
	}
}

// TestPasswordGateSkipsUserinfoForBearerRequest documents the other half of the
// contract: /oauth2/userinfo is served on the public route group and a bearer
// call carries no session id, so the gate has nothing to decide and never
// consults credential state.
func TestPasswordGateSkipsUserinfoForBearerRequest(t *testing.T) {
	service := &fakePasswordGateService{}
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodGet, "/oauth2/userinfo", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.stateCalls != 0 {
		t.Fatalf("credential state reads = %d, want 0 for a session-less bearer call", service.stateCalls)
	}
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("read counter: %v", err)
	}
	return m.GetCounter().GetValue()
}

func TestPasswordGateFailOpenIsCounted(t *testing.T) {
	before := counterValue(t, passwordGateFailOpenTotal)
	service := &fakePasswordGateService{stateErr: apperrors.NewInternal("credential store unavailable")}
	session := repositorySession()
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodGet, "/api/v1/users", &session)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (fail-open must not block)", rec.Code, http.StatusOK)
	}
	if delta := counterValue(t, passwordGateFailOpenTotal) - before; delta != 1 {
		t.Fatalf("password_gate_fail_open_total delta = %v, want 1", delta)
	}
}

func TestPasswordGatePinsSessionTenantScope(t *testing.T) {
	service := &fakePasswordGateService{state: appauth.PasswordState{
		MustChange: true,
		Reason:     appauth.PasswordChangeReasonNeedUpdatePassword,
	}}
	session := repositorySession()
	session.OrganizationSlug = "built-in"
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodGet, "/api/v1/users", &session)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	// 门禁前移到 TenantFilter 之前：必须自己用会话组织钉住 RLS 作用域，
	// 否则凭据行被 RLS 过滤，门禁 fail-open 变成永远放行。
	if service.lastScope.OrganizationID != "org-1" {
		t.Fatalf("scope organization id = %q, want org-1", service.lastScope.OrganizationID)
	}
	if service.lastScope.OrganizationName != "built-in" {
		t.Fatalf("scope organization name = %q, want built-in", service.lastScope.OrganizationName)
	}
	if service.lastScope.Mode != tenantctx.TenantFilterOptional {
		t.Fatalf("scope mode = %q, want optional", service.lastScope.Mode)
	}
}

func TestPasswordGateKeepsExistingTenantScope(t *testing.T) {
	service := &fakePasswordGateService{state: appauth.PasswordState{MustChange: false}}
	session := repositorySession()
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req = req.WithContext(WithSessionContext(
		tenantctx.WithTenantScope(req.Context(), "org-existing", "existing", tenantctx.TenantFilterOptional),
		session,
	))
	rec := httptest.NewRecorder()
	gate.Middleware(newGateTestHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.lastScope.OrganizationID != "org-existing" {
		t.Fatalf("scope organization id = %q, want org-existing", service.lastScope.OrganizationID)
	}
}

func TestPasswordGateEnforceMaxAgeReturnsPasswordExpired(t *testing.T) {
	service := &fakePasswordGateService{state: appauth.PasswordState{
		MustChange: true,
		Reason:     appauth.PasswordChangeReasonMaxAgeExceeded,
	}}
	session := repositorySession()
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodGet, "/api/v1/users", &session)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	code, details := decodeGateError(t, rec)
	if code != "password_expired" {
		t.Fatalf("code = %q, want password_expired", code)
	}
	if details["reason"] != appauth.PasswordChangeReasonMaxAgeExceeded {
		t.Fatalf("details.reason = %v", details["reason"])
	}
	if details["changePasswordUrl"] != "/account" {
		t.Fatalf("details.changePasswordUrl = %v", details["changePasswordUrl"])
	}
}

func TestPasswordGateExemptRouteAllowed(t *testing.T) {
	for _, reason := range []string{
		appauth.PasswordChangeReasonNeedUpdatePassword,
		appauth.PasswordChangeReasonMaxAgeExceeded,
	} {
		t.Run(reason, func(t *testing.T) {
			service := &fakePasswordGateService{state: appauth.PasswordState{
				MustChange: true,
				Reason:     reason,
			}}
			session := repositorySession()
			gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

			rec := runPasswordGate(t, gate, http.MethodGet, "/api/v1/auth/account", &session)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if service.stateCalls != 0 {
				t.Fatalf("state lookups = %d, want 0 for an exempt route", service.stateCalls)
			}
		})
	}
}

func TestPasswordGateAllowsWhenFlagCleared(t *testing.T) {
	service := &fakePasswordGateService{state: appauth.PasswordState{MustChange: false}}
	session := repositorySession()
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodGet, "/api/v1/users", &session)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPasswordGateWarnModeAllows(t *testing.T) {
	service := &fakePasswordGateService{state: appauth.PasswordState{
		MustChange: true,
		Reason:     appauth.PasswordChangeReasonNeedUpdatePassword,
	}}
	session := repositorySession()
	gate := NewPasswordGate(service, service, "warn", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodGet, "/api/v1/users", &session)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPasswordGateOffModeIsByteIdentical(t *testing.T) {
	service := &fakePasswordGateService{state: appauth.PasswordState{
		MustChange: true,
		Reason:     appauth.PasswordChangeReasonNeedUpdatePassword,
	}}
	session := repositorySession()
	gate := NewPasswordGate(service, service, "off", fakePasswordGateClock{now: time.Now()})

	plain := httptest.NewRecorder()
	newGateTestHandler().ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))

	gated := runPasswordGate(t, gate, http.MethodGet, "/api/v1/users", &session)

	if gated.Code != plain.Code {
		t.Fatalf("off status = %d, want %d", gated.Code, plain.Code)
	}
	if gated.Body.String() != plain.Body.String() {
		t.Fatalf("off body = %q, want %q", gated.Body.String(), plain.Body.String())
	}
}

func TestPasswordGateImpersonationBypassIsAudited(t *testing.T) {
	service := &fakePasswordGateService{
		state: appauth.PasswordState{MustChange: true, Reason: appauth.PasswordChangeReasonNeedUpdatePassword},
		impersonation: &appauth.ImpersonationInfo{
			SubjectUserID:  "target-user",
			ImpersonatorID: "admin-1",
			OrganizationID: "org-1",
		},
	}
	session := SessionContext{
		ID:             "",
		UserID:         "target-user",
		OrganizationID: "org-1",
		AccessToken:    "opaque-impersonation-token",
		Source:         "header",
		PrincipalType:  "user",
	}
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodGet, "/api/v1/users", &session)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(service.audits) != 1 {
		t.Fatalf("audit calls = %d, want 1", len(service.audits))
	}
	audit := service.audits[0]
	if audit.SubjectUserID != "target-user" || audit.ImpersonatorID != "admin-1" {
		t.Fatalf("audit subject=%q impersonator=%q", audit.SubjectUserID, audit.ImpersonatorID)
	}
	if audit.Method != http.MethodGet || audit.Path != "/api/v1/users" {
		t.Fatalf("audit method=%q path=%q", audit.Method, audit.Path)
	}
}

func TestPasswordGateOptionsAlwaysAllowed(t *testing.T) {
	service := &fakePasswordGateService{state: appauth.PasswordState{
		MustChange: true,
		Reason:     appauth.PasswordChangeReasonNeedUpdatePassword,
	}}
	gate := NewPasswordGate(service, service, "enforce", fakePasswordGateClock{now: time.Now()})

	rec := runPasswordGate(t, gate, http.MethodOptions, "/api/v1/users", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
