package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	actionsapp "github.com/guliangiam/guliangiam/internal/app/actions"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	deviceauthcodesdomain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type fakeActionsCall struct {
	hook  string
	orgID string
	raw   json.RawMessage
	input map[string]any
}

type fakeActionsDispatcher struct {
	results map[string]*actionsapp.DispatchResult
	errs    map[string]error
	calls   []fakeActionsCall
}

func (f *fakeActionsDispatcher) record(hook, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error) {
	parsed := map[string]any{}
	_ = json.Unmarshal(input, &parsed)
	f.calls = append(f.calls, fakeActionsCall{hook: hook, orgID: orgID, raw: input, input: parsed})
	if err := f.errs[hook]; err != nil {
		return nil, err
	}
	if result, ok := f.results[hook]; ok {
		return result, nil
	}
	return &actionsapp.DispatchResult{Allow: true}, nil
}

func (f *fakeActionsDispatcher) DispatchPreLogin(_ context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error) {
	return f.record("pre_login", orgID, input)
}
func (f *fakeActionsDispatcher) DispatchPreToken(_ context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error) {
	return f.record("pre_token", orgID, input)
}
func (f *fakeActionsDispatcher) DispatchPostRegistration(_ context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error) {
	return f.record("post_registration", orgID, input)
}
func (f *fakeActionsDispatcher) DispatchConsent(_ context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error) {
	return f.record("consent", orgID, input)
}
func (f *fakeActionsDispatcher) DispatchMFA(_ context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error) {
	return f.record("mfa", orgID, input)
}

func (f *fakeActionsDispatcher) callsFor(hook string) []fakeActionsCall {
	var out []fakeActionsCall
	for _, call := range f.calls {
		if call.hook == hook {
			out = append(out, call)
		}
	}
	return out
}

func denyResult(reason string) *actionsapp.DispatchResult {
	return &actionsapp.DispatchResult{Allow: false, Reasons: []string{reason}, ActionIDs: []string{"action-1"}}
}

func actionsAuditUow() (loginTestUnitOfWork, *authTestOutboxRepository) {
	uow := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	outbox := &authTestOutboxRepository{}
	uow.recordsRepo = &authTestRecordsRepository{}
	uow.outboxRepo = outbox
	return uow, outbox
}

func assertActionsDenyAudit(t *testing.T, outbox *authTestOutboxRepository, event string) map[string]any {
	t.Helper()
	for _, ev := range outbox.events {
		if ev.EventType != event {
			continue
		}
		var payload map[string]any
		require.NoError(t, json.Unmarshal(ev.Payload, &payload))
		return payload
	}
	t.Fatalf("expected audit outbox event %q, got %d events", event, len(outbox.events))
	return nil
}

// ── pre_login ────────────────────────────────────────────────────────────

func TestActionsPreLogin_DenyBlocksLogin(t *testing.T) {
	uow, outbox := actionsAuditUow()
	svc := newTestService(uow)
	dispatcher := &fakeActionsDispatcher{results: map[string]*actionsapp.DispatchResult{
		"pre_login": denyResult("blocked by tenant policy secret-reason"),
	}}
	svc.Actions = dispatcher

	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	require.Error(t, err)
	var typed *apperrors.Error
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeUnauthorized, typed.Code)
	assert.Equal(t, "login denied by policy", typed.Message)
	assert.NotContains(t, err.Error(), "secret-reason")
	assert.Empty(t, uow.sessRepo.sessions, "deny must short-circuit before any session is created")

	payload := assertActionsDenyAudit(t, outbox, "actions.pre_login.denied")
	assert.Equal(t, "denied", payload["outcome"])
	assert.Contains(t, payload["reasons"], "blocked by tenant policy secret-reason")
}

func TestActionsPreLogin_DispatchErrorFailsClosed(t *testing.T) {
	uow, outbox := actionsAuditUow()
	svc := newTestService(uow)
	svc.Actions = &fakeActionsDispatcher{errs: map[string]error{
		"pre_login": errors.New("wasm runtime crashed"),
	}}

	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	require.Error(t, err)
	var typed *apperrors.Error
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeUnavailable, typed.Code)
	assert.Empty(t, uow.sessRepo.sessions)

	payload := assertActionsDenyAudit(t, outbox, "actions.pre_login.denied")
	assert.Equal(t, "error", payload["outcome"])
}

func TestActionsPreLogin_AllowKeepsLoginAndInputIsMetadataOnly(t *testing.T) {
	uow := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	svc := newTestService(uow)
	dispatcher := &fakeActionsDispatcher{}
	svc.Actions = dispatcher

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
		IPAddress:        "203.0.113.9",
		UserAgent:        "actions-test-agent",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, LoginStatusSuccess, result.Response.Status)

	calls := dispatcher.callsFor("pre_login")
	require.Len(t, calls, 1)
	call := calls[0]
	assert.Equal(t, testOrgID, call.orgID)
	assert.Equal(t, testOrgID, call.input["organization_id"])
	assert.Equal(t, testUserID, call.input["user_id"])
	assert.Equal(t, testUsername, call.input["username"])
	assert.Equal(t, testAppID, call.input["application_id"])
	assert.Equal(t, "203.0.113.9", call.input["ip"])
	assert.Equal(t, "actions-test-agent", call.input["user_agent"])
	assert.NotEmpty(t, call.input["channel"])
	// 安全红线：凭据绝不进入 input（execution_logs.Input 落库）。
	raw := string(call.raw)
	assert.NotContains(t, raw, testPassword)
	assert.NotContains(t, raw, testPasswordHash)
	// channel:"password" 是合法元数据；红线扫描针对凭据字段键与凭据值。
	for _, forbidden := range []string{`"password":`, `"passcode":`, `"token":`, `"assertion":`, `"credential":`, `"device_credential_id":`} {
		assert.NotContains(t, strings.ToLower(raw), forbidden)
	}
}

func TestActionsPreLogin_NilDispatcherKeepsLegacyLogin(t *testing.T) {
	uow := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	svc := newTestService(uow)
	svc.Actions = nil

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	require.NoError(t, err)
	assert.Equal(t, LoginStatusSuccess, result.Response.Status)
}

// ── mfa ──────────────────────────────────────────────────────────────────

func TestActionsMFA_DenyBlocksVerifiedChallenge(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	user := newTestUser(func(u *userdomain.User) {
		u.TotpSecret = secret
		u.PreferredMfaType = mfaTypeTotp
		u.MfaMethods = []userdomain.UserMfaMethod{{
			ID:         "mfa-totp",
			UserID:     testUserID,
			MethodType: mfaTypeTotp,
			Enabled:    true,
			Verified:   true,
		}}
	})
	uow, outbox := actionsAuditUow()
	uow.userRepo.users[user.ID] = user
	uow.userRepo.usersByOrg[testOrgID][user.Username] = user
	svc := newTestService(uow)
	dispatcher := &fakeActionsDispatcher{results: map[string]*actionsapp.DispatchResult{
		"mfa": denyResult("mfa blocked secret-reason"),
	}}
	svc.Actions = dispatcher

	first, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, first.Response.ChallengeID)

	code, err := totp.GenerateCode(secret, svc.Clock.Now())
	require.NoError(t, err)
	_, err = svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		ChallengeID:      first.Response.ChallengeID,
		MfaType:          mfaTypeTotp,
		Passcode:         code,
	})
	require.Error(t, err)
	var typed *apperrors.Error
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeUnauthorized, typed.Code)
	assert.NotContains(t, err.Error(), "secret-reason")
	assert.Empty(t, uow.sessRepo.sessions, "MFA deny must not create a session")

	calls := dispatcher.callsFor("mfa")
	require.Len(t, calls, 1)
	assert.Equal(t, mfaTypeTotp, calls[0].input["mfa_type"])
	assert.NotContains(t, string(calls[0].raw), code)
	assert.NotContains(t, string(calls[0].raw), secret)
	assertActionsDenyAudit(t, outbox, "actions.mfa.denied")
}

// ── consent ──────────────────────────────────────────────────────────────

func consentActionsFixture(t *testing.T) (*Service, *fakeActionsDispatcher, *authTestOutboxRepository) {
	t.Helper()
	app := newTestApp(func(a *applicationdomain.Application) {
		a.CustomScopes = []applicationdomain.ApplicationCustomScope{{Scope: "read"}}
	})
	base := newServiceTestUow(newTestOrg(), newTestUser(), app)
	records := &authTestRecordsRepository{}
	outbox := &authTestOutboxRepository{}
	base.recordsRepo = records
	base.outboxRepo = outbox
	svc := newServiceTestService(base)
	dispatcher := &fakeActionsDispatcher{}
	svc.Actions = dispatcher
	return &svc, dispatcher, outbox
}

func TestActionsConsent_DenyRenders403BeforeWrite(t *testing.T) {
	svc, dispatcher, outbox := consentActionsFixture(t)
	dispatcher.results = map[string]*actionsapp.DispatchResult{
		"consent": denyResult("consent blocked secret-reason"),
	}

	_, err := svc.GrantConsent(context.Background(), GrantConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"read"},
	})
	require.Error(t, err)
	var typed *apperrors.Error
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeForbidden, typed.Code)
	assert.NotContains(t, err.Error(), "secret-reason")
	require.Len(t, dispatcher.callsFor("consent"), 1)

	payload := assertActionsDenyAudit(t, outbox, "actions.consent.denied")
	assert.Equal(t, "denied", payload["outcome"])
}

func TestActionsConsent_DispatchErrorFailsClosed(t *testing.T) {
	svc, dispatcher, _ := consentActionsFixture(t)
	dispatcher.errs = map[string]error{"consent": errors.New("engine down")}

	_, err := svc.GrantConsent(context.Background(), GrantConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"read"},
	})
	require.Error(t, err)
	var typed *apperrors.Error
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeUnavailable, typed.Code)
}

func TestActionsConsent_AllowGrants(t *testing.T) {
	svc, dispatcher, _ := consentActionsFixture(t)

	response, err := svc.GrantConsent(context.Background(), GrantConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"read"},
	})
	require.NoError(t, err)
	assert.Equal(t, "granted", response.Status)
	require.Len(t, dispatcher.callsFor("consent"), 1)
	assert.Equal(t, []any{"read"}, dispatcher.callsFor("consent")[0].input["scopes"])
}

// ── pre_token: refresh grant ─────────────────────────────────────────────

func TestActionsPreToken_RefreshDenyIsInvalidGrant(t *testing.T) {
	exchanger := &fakeRefreshExchanger{}
	svc := refreshGrantFixture(t, []string{grantAuthorizationCode, oauthRefreshToken}, exchanger)
	dispatcher := &fakeActionsDispatcher{results: map[string]*actionsapp.DispatchResult{
		"pre_token": denyResult("refresh blocked"),
	}}
	svc.Actions = dispatcher

	_, err := svc.ExchangeRefreshToken(context.Background(), ExchangeRefreshTokenCommand{
		RefreshToken: "gl_prt_old",
		ClientID:     "refresh-client",
		Scope:        "openid profile",
	})
	require.Error(t, err)
	var protocolErr apperrors.ProtocolCodeError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, "invalid_grant", protocolErr.ProtocolCode())
	assert.Empty(t, exchanger.requests, "deny must short-circuit before token issuance")

	calls := dispatcher.callsFor("pre_token")
	require.Len(t, calls, 1)
	assert.Equal(t, oauthRefreshToken, calls[0].input["grant_type"])
	assert.Equal(t, testOrgID, calls[0].orgID)
	assert.NotContains(t, string(calls[0].raw), "gl_prt_old")
}

func TestActionsPreToken_RefreshDispatchErrorFailsClosed(t *testing.T) {
	exchanger := &fakeRefreshExchanger{}
	svc := refreshGrantFixture(t, []string{grantAuthorizationCode, oauthRefreshToken}, exchanger)
	svc.Actions = &fakeActionsDispatcher{errs: map[string]error{"pre_token": errors.New("engine down")}}

	_, err := svc.ExchangeRefreshToken(context.Background(), ExchangeRefreshTokenCommand{
		RefreshToken: "gl_prt_old",
		ClientID:     "refresh-client",
	})
	require.Error(t, err)
	assert.Empty(t, exchanger.requests)
}

func TestActionsPreToken_RefreshNilDispatcherKeepsLegacy(t *testing.T) {
	exchanger := &fakeRefreshExchanger{result: RefreshTokenExchangeResult{
		AccessToken: "gl_pat_new", RefreshToken: "gl_prt_new", TokenType: "Bearer",
	}}
	svc := refreshGrantFixture(t, []string{grantAuthorizationCode, oauthRefreshToken}, exchanger)
	svc.Actions = nil

	_, err := svc.ExchangeRefreshToken(context.Background(), ExchangeRefreshTokenCommand{
		RefreshToken: "gl_prt_old",
		ClientID:     "refresh-client",
	})
	require.NoError(t, err)
	require.Len(t, exchanger.requests, 1)
}

// ── pre_token: device flow ───────────────────────────────────────────────

func deviceActionsFixture(t *testing.T) (*Service, *fakeActionsDispatcher) {
	t.Helper()
	app := newTestApp()
	entry := deviceTestEntry(nil)
	entry.Verified = true
	entry.UserID = testUserID
	entry.OrganizationID = testOrgID
	repo := &testDeviceAuthCodeRepository{
		entries: map[string]*deviceauthcodesdomain.DeviceAuthCode{entry.DeviceCode: entry},
	}
	base := newTestUow(newTestOrg(), newTestUser(), app)
	base.appRepo.byClientID = map[string]*applicationdomain.Application{entry.ClientID: app}
	uow := serviceTestUnitOfWork{loginTestUnitOfWork: base, deviceRepo: repo}
	svc := newTestService(base)
	svc.UOW = uow
	dispatcher := &fakeActionsDispatcher{}
	svc.Actions = dispatcher
	return &svc, dispatcher
}

func TestActionsPreToken_DeviceDenyKeepsAccessDeniedProtocolError(t *testing.T) {
	svc, dispatcher := deviceActionsFixture(t)
	dispatcher.results = map[string]*actionsapp.DispatchResult{
		"pre_token": denyResult("device blocked"),
	}

	_, err := svc.DeviceToken(context.Background(), DeviceTokenCommand{
		DeviceCode: "device-code-1",
		ClientID:   "device-client",
	})
	require.Error(t, err)
	// 门禁错误必须穿透外层 WrapInternal，保持 RFC 8628 协议错误类型。
	assert.Equal(t, deviceErrorAccessDenied, deviceProtocolCode(t, err))

	calls := dispatcher.callsFor("pre_token")
	require.Len(t, calls, 1)
	assert.Equal(t, "urn:ietf:params:oauth:grant-type:device_code", calls[0].input["grant_type"])
	assert.NotContains(t, string(calls[0].raw), "device-code-1")
	require.Len(t, dispatcher.callsFor("pre_login"), 1)
	assert.Equal(t, "device_flow", dispatcher.callsFor("pre_login")[0].input["channel"])
}

func TestActionsPreLogin_DeviceDenyRenders401(t *testing.T) {
	svc, dispatcher := deviceActionsFixture(t)
	dispatcher.results = map[string]*actionsapp.DispatchResult{
		"pre_login": denyResult("device login blocked"),
	}

	_, err := svc.DeviceToken(context.Background(), DeviceTokenCommand{
		DeviceCode: "device-code-1",
		ClientID:   "device-client",
	})
	require.Error(t, err)
	var typed *apperrors.Error
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeUnauthorized, typed.Code)
	assert.Empty(t, dispatcher.callsFor("pre_token"), "pre_login deny must short-circuit before pre_token")
}

// ── post_registration（fail-open, EXT-004）────────────────────────────────

func TestActionsPostRegistration_DispatchErrorDoesNotBlockSignup(t *testing.T) {
	uow := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	svc := newTestService(uow)
	dispatcher := &fakeActionsDispatcher{errs: map[string]error{
		"post_registration": errors.New("webhook target unreachable"),
	}}
	svc.Actions = dispatcher

	result, err := svc.Signup(context.Background(), SignupCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "actions-new-user",
		Password:         testPassword,
		SignupAppID:      testAppID,
	})
	require.NoError(t, err, "post_registration is fail-open: dispatch error must not block signup")
	require.NotNil(t, result)

	calls := dispatcher.callsFor("post_registration")
	require.Len(t, calls, 1)
	assert.Equal(t, "Application Signup", calls[0].input["register_type"])
	assert.Equal(t, testOrgID, calls[0].orgID)
	assert.NotContains(t, string(calls[0].raw), testPassword)
}

func TestActionsPostRegistration_DenyDoesNotBlockSignup(t *testing.T) {
	uow, outbox := actionsAuditUow()
	svc := newTestService(uow)
	svc.Actions = &fakeActionsDispatcher{results: map[string]*actionsapp.DispatchResult{
		"post_registration": denyResult("notify target rejected"),
	}}

	_, err := svc.Signup(context.Background(), SignupCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "actions-new-user",
		Password:         testPassword,
		SignupAppID:      testAppID,
	})
	require.NoError(t, err, "post_registration deny is advisory only (EXT-004)")

	payload := assertActionsDenyAudit(t, outbox, "actions.post_registration.failed")
	assert.Equal(t, "denied", payload["outcome"])
}

func TestActionsPostRegistration_NilDispatcherSkips(t *testing.T) {
	uow := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	svc := newTestService(uow)
	svc.Actions = nil

	_, err := svc.Signup(context.Background(), SignupCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "actions-new-user",
		Password:         testPassword,
		SignupAppID:      testAppID,
	})
	require.NoError(t, err)
}

func TestActionsPostRegistration_AllowWritesNoFailureAudit(t *testing.T) {
	uow, outbox := actionsAuditUow()
	svc := newTestService(uow)
	dispatcher := &fakeActionsDispatcher{}
	svc.Actions = dispatcher

	svc.dispatchPostRegistration(context.Background(), testOrgID, testUserID, testUsername, testAppID, "Application Signup")
	require.Len(t, dispatcher.callsFor("post_registration"), 1)
	for _, ev := range outbox.events {
		assert.NotEqual(t, "actions.post_registration.failed", ev.EventType)
	}
}

// ── helper 级断言（mfa / pre_token 输入红线）───────────────────────────────

func TestActionsMFA_DenyAndErrorSemantics(t *testing.T) {
	uow, outbox := actionsAuditUow()
	svc := newTestService(uow)
	dispatcher := &fakeActionsDispatcher{results: map[string]*actionsapp.DispatchResult{
		"mfa": denyResult("mfa blocked"),
	}}
	svc.Actions = dispatcher

	err := svc.enforceActionsMFA(context.Background(), testOrgID, testUserID, testUsername, testAppID, mfaTypeTotp)
	require.Error(t, err)
	var typed *apperrors.Error
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeUnauthorized, typed.Code)
	assert.Equal(t, "login denied by policy", typed.Message)
	require.Len(t, dispatcher.callsFor("mfa"), 1)
	assert.Equal(t, mfaTypeTotp, dispatcher.callsFor("mfa")[0].input["mfa_type"])
	assertActionsDenyAudit(t, outbox, "actions.mfa.denied")

	dispatcher.errs = map[string]error{"mfa": errors.New("engine down")}
	err = svc.enforceActionsMFA(context.Background(), testOrgID, testUserID, testUsername, testAppID, mfaTypeTotp)
	require.Error(t, err)
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeUnavailable, typed.Code)

	svc.Actions = nil
	assert.NoError(t, svc.enforceActionsMFA(context.Background(), testOrgID, testUserID, testUsername, testAppID, mfaTypeTotp))
}

func TestActionsPreToken_InputCarriesOnlyMetadata(t *testing.T) {
	uow := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	svc := newTestService(uow)
	dispatcher := &fakeActionsDispatcher{}
	svc.Actions = dispatcher

	err := svc.enforceActionsPreToken(context.Background(), testOrgID, testUserID, testUsername, testAppID,
		grantAuthorizationCode, []string{"openid", "profile"}, newInvalidGrantError("token issuance denied by policy"))
	require.NoError(t, err)
	calls := dispatcher.callsFor("pre_token")
	require.Len(t, calls, 1)
	assert.Equal(t, grantAuthorizationCode, calls[0].input["grant_type"])
	assert.Equal(t, []any{"openid", "profile"}, calls[0].input["scopes"])
	assert.Equal(t, testUserID, calls[0].input["user_id"])

	dispatcher.results = map[string]*actionsapp.DispatchResult{"pre_token": denyResult("blocked")}
	err = svc.enforceActionsPreToken(context.Background(), testOrgID, testUserID, testUsername, testAppID,
		grantAuthorizationCode, nil, newInvalidGrantError("token issuance denied by policy"))
	require.Error(t, err)
	var protocolErr apperrors.ProtocolCodeError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, "invalid_grant", protocolErr.ProtocolCode())
}
