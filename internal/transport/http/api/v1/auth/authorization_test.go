package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apprebac "github.com/guliangiam/guliangiam/internal/app/rebac"
	domainrebac "github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/stretchr/testify/assert"
)

func TestDefaultRBACModel(t *testing.T) {
	model := defaultRBACModel()
	assert.Contains(t, model, "request_definition")
	assert.Contains(t, model, "policy_definition")
	assert.Contains(t, model, "role_definition")
	assert.Contains(t, model, "policy_effect")
	assert.Contains(t, model, "matchers")
}

func TestResourceFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		// ── Generic modules ──────────────────────────────────────────
		{"users_path", "/api/v1/users/123", "users"},
		{"organizations_path", "/api/v1/organizations", "organizations"},
		{"global_resource", "/api/v1/global/permissions", "permissions"},
		{"global_with_colon", "/api/v1/global/users:import", "users"},
		{"resource_with_colon", "/api/v1/users:import", "users"},
		{"empty_after_prefix", "/api/v1/", ""},
		{"root_path", "/", ""},
		{"no_api_prefix", "/other/path", ""},

		// ── Payments reconciliation / channels / adapters / i18n ─────────
		// routes.go paths that derive their Casbin resource from the first
		// segment; the colon action suffixes are stripped.
		{"pay_recon_run", "/api/v1/pay-recon:run", "pay-recon"},
		{"pay_recon_item_resolve", "/api/v1/pay-recon-items/550e8400-0000-4000-8000-000000000001:resolve", "pay-recon-items"},
		{"pay_recon_batches", "/api/v1/pay-recon-batches", "pay-recon-batches"},
		{"pay_channel_configs", "/api/v1/pay-channel-configs", "pay-channel-configs"},
		{"pay_channel_config_transition", "/api/v1/pay-channel-configs/550e8400-0000-4000-8000-000000000001:transition", "pay-channel-configs"},
		{"pay_channels", "/api/v1/pay-channels", "pay-channels"},
		{"adapters", "/api/v1/adapters/550e8400-0000-4000-8000-000000000001", "adapters"},
		{"adapter_policies", "/api/v1/adapters/550e8400-0000-4000-8000-000000000001/policies", "adapters"},
		{"i18n_translations", "/api/v1/i18n/translations/en", "i18n"},
		{"i18n_translate", "/api/v1/i18n/translate", "i18n"},

		// ── AI Gateway: Management surface (admin_chi.go + routes.go) ────
		{"ag_list_providers", "/api/v1/ai/providers", "aigateway:providers"},
		{"ag_create_provider", "/api/v1/ai/providers", "aigateway:providers"},
		{"ag_get_provider", "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001", "aigateway:providers"},
		{"ag_update_provider", "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001", "aigateway:providers"},
		{"ag_delete_provider", "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001", "aigateway:providers"},
		{"ag_test_provider", "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001/test", "aigateway:providers"},
		{"ag_list_models", "/api/v1/ai/models", "aigateway:models"},
		{"ag_create_model", "/api/v1/ai/models", "aigateway:models"},
		{"ag_get_model", "/api/v1/ai/models/mid", "aigateway:models"},
		{"ag_update_model", "/api/v1/ai/models/mid", "aigateway:models"},
		{"ag_delete_model", "/api/v1/ai/models/mid", "aigateway:models"},
		{"ag_list_aliases", "/api/v1/ai/aliases", "aigateway:aliases"},
		{"ag_create_alias", "/api/v1/ai/aliases", "aigateway:aliases"},
		{"ag_get_alias", "/api/v1/ai/aliases/aid", "aigateway:aliases"},
		{"ag_update_alias", "/api/v1/ai/aliases/aid", "aigateway:aliases"},
		{"ag_delete_alias", "/api/v1/ai/aliases/aid", "aigateway:aliases"},
		{"ag_list_routes", "/api/v1/ai/routes", "aigateway:routes"},
		{"ag_create_route", "/api/v1/ai/routes", "aigateway:routes"},
		{"ag_get_route", "/api/v1/ai/routes/rid", "aigateway:routes"},
		{"ag_update_route", "/api/v1/ai/routes/rid", "aigateway:routes"},
		{"ag_delete_route", "/api/v1/ai/routes/rid", "aigateway:routes"},
		{"ag_list_api_keys", "/api/v1/ai/api-keys", "aigateway:api_keys"},
		{"ag_create_api_key", "/api/v1/ai/api-keys", "aigateway:api_keys"},
		{"ag_get_api_key", "/api/v1/ai/api-keys/kid", "aigateway:api_keys"},
		{"ag_update_api_key", "/api/v1/ai/api-keys/kid", "aigateway:api_keys"},
		{"ag_delete_api_key", "/api/v1/ai/api-keys/kid", "aigateway:api_keys"},
		{"ag_revoke_api_key", "/api/v1/ai/api-keys/kid/revoke", "aigateway:api_keys"},
		{"ag_list_quotas", "/api/v1/ai/quotas", "aigateway:quotas"},
		{"ag_create_quota", "/api/v1/ai/quotas", "aigateway:quotas"},
		{"ag_get_quota", "/api/v1/ai/quotas/qid", "aigateway:quotas"},
		{"ag_update_quota", "/api/v1/ai/quotas/qid", "aigateway:quotas"},
		{"ag_delete_quota", "/api/v1/ai/quotas/qid", "aigateway:quotas"},
		{"ag_list_usage", "/api/v1/ai/usage", "aigateway:usage"},
		{"ag_usage_stats", "/api/v1/ai/usage/stats", "aigateway:usage"},
		{"ag_list_prices", "/api/v1/ai/prices", "aigateway:prices"},
		{"ag_create_price", "/api/v1/ai/prices", "aigateway:prices"},
		{"ag_get_price", "/api/v1/ai/prices/pid", "aigateway:prices"},
		{"ag_update_price", "/api/v1/ai/prices/pid", "aigateway:prices"},
		{"ag_delete_price", "/api/v1/ai/prices/pid", "aigateway:prices"},
		{"ag_list_health", "/api/v1/ai/health", "aigateway:health"},
		{"ag_get_health", "/api/v1/ai/health/hid", "aigateway:health"},

		// ── AI Gateway: Data-plane (protocol faces) ────────────────────
		// routes.go declares Permission: "aigateway:chat" for all faces.
		// resourceFromPath must map /api/v1/ai/faces/* → "aigateway:chat".
		{"ag_face_openai_chat", "/api/v1/ai/faces/openai/chat/completions", "aigateway:chat"},
		{"ag_face_openai_embeddings", "/api/v1/ai/faces/openai/embeddings", "aigateway:chat"},
		{"ag_face_openai_models", "/api/v1/ai/faces/openai/models", "aigateway:chat"},
		{"ag_face_anthropic_chat", "/api/v1/ai/faces/anthropic/chat/completions", "aigateway:chat"},
		{"ag_face_anthropic_embeddings", "/api/v1/ai/faces/anthropic/embeddings", "aigateway:chat"},
		{"ag_face_anthropic_models", "/api/v1/ai/faces/anthropic/models", "aigateway:chat"},
		{"ag_face_gemini_chat", "/api/v1/ai/faces/gemini/chat/completions", "aigateway:chat"},
		{"ag_face_gemini_embeddings", "/api/v1/ai/faces/gemini/embeddings", "aigateway:chat"},
		{"ag_face_gemini_models", "/api/v1/ai/faces/gemini/models", "aigateway:chat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resourceFromPath(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsTrulyPublicEndpoint(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// Infrastructure — 完全公开
		{"well_known", "/.well-known/openid-configuration", true},
		{"healthz", "/healthz", true},
		{"openapi", "/openapi/swagger.json", true},
		{"uploads", "/uploads/logo.png", true},
		// Protocol endpoints — 自身认证
		{"cas", "/cas/login", true},
		{"oauth2", "/oauth2/token", true},
		{"kerberos", "/kerberos/login", true},
		{"scim", "/scim/v2/Users", true},
		// SAML — 协议握手必须公开（metadata 拉取、SSO/SSLO 发起、IdP 回调）
		{"saml_idp_metadata", "/api/v1/saml/metadata/app", true},
		{"saml_sso", "/api/v1/saml/sso/app", true},
		{"saml_slo", "/api/v1/saml/slo", true},
		{"saml_auth_metadata", "/api/v1/auth/saml/metadata/org/app", true},
		{"saml_acs", "/api/v1/auth/saml/acs", true},
		// Auth **login flow** — 公开
		{"auth_login", "/api/v1/auth/login", true},
		{"auth_signup", "/api/v1/auth/signup", true},
		{"auth_social_callback", "/api/v1/auth/social/callback", true},
		// QR code scan login — 创建/扫码/轮询允许匿名，确认与换取需要会话。
		// 轮询同时锁两种形状：运行期真实路径与 routes.go 模式串。
		{"qrcode_create", "/api/v1/auth/qrcode", true},
		{"qrcode_scan", "/api/v1/auth/qrcode/scan", true},
		{"qrcode_status_pattern", "/api/v1/auth/qrcode/{codeId}/status", true},
		{"qrcode_status_concrete", "/api/v1/auth/qrcode/01924f00-0000-7000-8000-000000000000/status", true},
		{"qrcode_confirm_not_public", "/api/v1/auth/qrcode/confirm", false},
		// 会话由 exchange 签发（对齐 /faceid/complete），故它属于登录握手：
		// 要求调用方先有会话等于要求「先登录才能登录」。
		{"qrcode_exchange", "/api/v1/auth/qrcode/exchange", true},
		// WebAuthn（Passkey）登录：begin/finish 是同一断言握手的两个半边，会话在
		// finish 签发 → 匿名可达；凭证注册需要已认证会话 → 不公开。
		{"webauthn_authentication_begin", "/api/v1/auth/webauthn/authentication/begin", true},
		{"webauthn_authentication_finish", "/api/v1/auth/webauthn/authentication/finish", true},
		{"webauthn_registration_begin_not_public", "/api/v1/auth/webauthn/registration/begin", false},
		{"webauthn_registration_finish_not_public", "/api/v1/auth/webauthn/registration/finish", false},
		// DCR / Application discovery
		{"dcr", "/api/v1/dcr/register", true},
		{"app_resolve", "/api/v1/applications/resolve", true},
		{"app_by_client_id", "/api/v1/applications/by-client-id/test", true},
		// Captcha provider administration — never anonymous, including reads.
		// Login reads configuration via /api/v1/auth/captcha-status.
		{"captcha_providers_not_public", "/api/v1/captcha-providers/generate", false},
		{"captcha_provider_detail_not_public", "/api/v1/captcha-providers/abc", false},
		{"invitation_verify", "/api/v1/invitations/verify", true},
		{"system_health", "/api/v1/system/health", true},
		{"payment_notify", "/api/v1/payments/notify/wechat", true},
		// Auth 会话端点 — **不是** truly public，走 isAuthRequiringSession
		{"auth_account_not_public", "/api/v1/auth/account", false},
		{"auth_logout_not_public", "/api/v1/auth/logout", false},
		{"auth_consents_not_public", "/api/v1/auth/consents/check", false},
		{"auth_mfa_push_initiate_not_public", "/api/v1/auth/mfa/push/initiate", false},
		{"auth_mfa_push_verify_not_public", "/api/v1/auth/mfa/push/verify", false},
		// Private CRUD — 正常 Casbin 流程
		{"private_users", "/api/v1/users", false},
		{"private_orgs", "/api/v1/organizations/org-1", false},
		{"private_roles", "/api/v1/roles", false},
		{"private_apps", "/api/v1/applications/app-1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTrulyPublicEndpoint(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsTrulyPublicRequest_CaptchaProvidersDeniedForEveryMethod locks the
// P0-2 whitelist shape: the decision key is method + path, and no verb on the
// captcha-provider namespace is anonymous. The login page's actual pre-login
// captcha reads stay public.
func TestIsTrulyPublicRequest_CaptchaProvidersDeniedForEveryMethod(t *testing.T) {
	for _, method := range []string{
		http.MethodGet, http.MethodHead, http.MethodOptions,
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	} {
		for _, path := range []string{
			"/api/v1/captcha-providers",
			"/api/v1/captcha-providers/abc",
		} {
			assert.False(t, isTrulyPublicRequest(method, path),
				"%s %s must not be anonymous", method, path)
		}
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/auth/captcha-status"},
		{http.MethodGet, "/api/v1/auth/captcha"},
		{http.MethodPost, "/api/v1/auth/captcha/verify"},
	} {
		assert.True(t, isTrulyPublicRequest(tc.method, tc.path),
			"%s %s is a login-flow captcha read and must stay public", tc.method, tc.path)
	}
}

func TestIsAuthRequiringSession(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// 需要 session 的 auth 端点
		{"account", "/api/v1/auth/account", true},
		// logout 是幂等端点：允许无 session 调用（session 过期时必须也能 ClearSessionCookie）
		{"logout_not_session", "/api/v1/auth/logout", false},
		{"permissions", "/api/v1/auth/permissions", true},
		{"password_check", "/api/v1/auth/password/check", true},
		{"password_change", "/api/v1/auth/password/change", true},
		{"verify_code", "/api/v1/auth/verify-code", true},
		{"consents_check", "/api/v1/auth/consents/check", true},
		{"consents_grant", "/api/v1/auth/consents/grant", true},
		{"consents_revoke", "/api/v1/auth/consents/revoke", true},
		// Token 端点由客户端自证（secret / assertion / mTLS），不属于「操作当前
		// session」——匿名可达是 OAuth2 客户端的唯一合法调用方式。
		{"tokens_exchange", "/api/v1/auth/tokens/exchange", false},
		{"tokens_jwt_bearer", "/api/v1/auth/tokens/jwt-bearer", false},
		{"tokens_token_exchange", "/api/v1/auth/tokens/token-exchange", false},
		{"tokens_introspect", "/api/v1/auth/tokens/introspect", false},
		{"tokens_client_credentials", "/api/v1/auth/tokens/client-credentials", false},
		// revoke/introspect 与 token 端点同属客户端认证协议面（RFC 7009/7662），
		// 调用方以客户端凭据自证；/oauth2/ 下其余端点仍需 session。
		{"oauth2_revoke_not_session", "/api/v1/auth/oauth2/revoke", false},
		{"oauth2_introspect_not_session", "/api/v1/auth/oauth2/introspect", false},
		{"oauth2_userinfo_session", "/api/v1/auth/oauth2/userinfo", true},
		{"authorize", "/api/v1/auth/authorize", true},
		{"mfa_push_enable", "/api/v1/auth/mfa/push/enable", true},
		{"mfa_push_disable", "/api/v1/auth/mfa/push/disable", true},
		{"mfa_push_initiate", "/api/v1/auth/mfa/push/initiate", true},
		{"mfa_push_verify", "/api/v1/auth/mfa/push/verify", true},
		{"challenge_approve", "/api/v1/auth/login-challenges/abc/approve", true},
		{"challenge_reject", "/api/v1/auth/login-challenges/abc/reject", true},
		{"qrcode_confirm", "/api/v1/auth/qrcode/confirm", true},
		{"qrcode_exchange_not_session", "/api/v1/auth/qrcode/exchange", false},
		// WebAuthn 注册凭证是自服务动作（对象是调用者自己的凭证），需要 session
		// 但跳过 Casbin；认证面属登录握手，必须匿名可达。
		{"webauthn_registration_begin", "/api/v1/auth/webauthn/registration/begin", true},
		{"webauthn_registration_finish", "/api/v1/auth/webauthn/registration/finish", true},
		{"webauthn_authentication_begin_not_session", "/api/v1/auth/webauthn/authentication/begin", false},
		{"webauthn_authentication_finish_not_session", "/api/v1/auth/webauthn/authentication/finish", false},
		{"qrcode_status_not_session", "/api/v1/auth/qrcode/01924f00-0000-7000-8000-000000000000/status", false},
		// login flow — 不是 require session（truly public）
		{"login_not_session", "/api/v1/auth/login", false},
		{"signup_not_session", "/api/v1/auth/signup", false},
		{"social_callback_not_session", "/api/v1/auth/social/callback", false},
		{"qrcode_initiate_not_session", "/api/v1/auth/qrcode", false},
		{"faceid_init_not_session", "/api/v1/auth/faceid", false},
		// Account 自助档案/实名/FaceID — 需要 session 但跳过 Casbin；faceids
		// （复数）是会话自助面，绝不能串进公开 faceid（单数）登录握手。
		{"self_profile", "/api/v1/auth/profile", true},
		{"self_verify_identification", "/api/v1/auth/verify-identification", true},
		{"self_faceids", "/api/v1/auth/faceids", true},
		{"faceid_verify_not_session", "/api/v1/auth/faceid/verify", false},
		{"self_profile_subpath_not_session", "/api/v1/auth/profile/extra", false},
		{"self_faceids_subpath_not_session", "/api/v1/auth/faceids/extra", false},
		// 非 auth 端点 — 一律 false
		{"private_users", "/api/v1/users", false},
		{"private_orgs", "/api/v1/organizations", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAuthRequiringSession(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPEPFailClose_NoSessionOnSessionAuthEndpoint — 安全测试：
// 攻击者不带 session cookie + 伪造 X-Username Header → 必须返回 401
// 此测试锁定 isAuthRequiringSession + requestFromHTTPRequest 的 fail-close 行为。
//
// 注意：/auth/logout 是例外 — 它被特殊放行以便无 session 时也能 ClearSessionCookie
// （session 过期时若被拦截 → HttpOnly cookie 残留 → 下次 bootstrap 自动重登）。
func TestPEPFailClose_NoSessionOnSessionAuthEndpoint(t *testing.T) {
	paths := []string{
		"/api/v1/auth/account",
		"/api/v1/auth/consents/check",
		"/api/v1/auth/authorize",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			assert.True(t, isAuthRequiringSession(p),
				"path must be classified as session-requiring before Casbin")
			assert.False(t, isTrulyPublicEndpoint(p),
				"path must NOT be truly public — needs session context")
		})
	}
}

type baseAuthorizationPDPStub struct {
	allowed      bool
	enforceCalls int
}

func (s *baseAuthorizationPDPStub) ResolveEnforcerUUID(context.Context, string) (string, error) {
	return "enforcer-1", nil
}

func (s *baseAuthorizationPDPStub) Enforce(context.Context, string, string, ...interface{}) (bool, error) {
	s.enforceCalls++
	return s.allowed, nil
}

type failingReBACTupleStore struct{}

func (failingReBACTupleStore) FetchAllTuples(context.Context, string, string, string) ([]*domainrebac.Tuple, error) {
	return nil, errors.New("tuple store unavailable")
}

func (failingReBACTupleStore) GetLatestZookie(context.Context, string, string) (string, error) {
	return "", errors.New("zookie store unavailable")
}

type fixedNamespaceResolver string

func (r fixedNamespaceResolver) Resolve(context.Context, string) (string, error) {
	return string(r), nil
}

type failingNamespaceResolver struct{}

func (failingNamespaceResolver) Resolve(context.Context, string) (string, error) {
	return "", errors.New("namespace store unavailable")
}

func testAPIAuthorizationMiddleware(pdp AuthorizationPDP) func(http.Handler) http.Handler {
	return APIAuthorizationMiddleware(pdp, apprebac.NewEngine(failingReBACTupleStore{}), fixedNamespaceResolver(""))
}

func testRequirePermission(permission string, pdp AuthorizationPDP, cfg CasbinPermissionConfig) func(http.Handler) http.Handler {
	return RequirePermission(permission, pdp, apprebac.NewEngine(failingReBACTupleStore{}), cfg, fixedNamespaceResolver(""))
}

func TestAPIAuthorizationMiddleware_ReBACFailureReturnsUnavailable(t *testing.T) {
	nextCalls := 0
	pdp := &baseAuthorizationPDPStub{allowed: true}
	engine := apprebac.NewEngine(failingReBACTupleStore{})
	handler := APIAuthorizationMiddleware(pdp, engine, fixedNamespaceResolver("namespace-1"))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			nextCalls++
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req = req.WithContext(WithSessionContext(req.Context(), SessionContext{
		OrganizationID:   "org-uuid",
		OrganizationSlug: "acme",
		Username:         "alice",
		Source:           "repository",
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Zero(t, nextCalls, "ReBAC dependency failures must not reach the protected handler")
}

func TestRequirePermission_ReBACFailureReturnsUnavailable(t *testing.T) {
	nextCalls := 0
	pdp := &baseAuthorizationPDPStub{allowed: true}
	engine := apprebac.NewEngine(failingReBACTupleStore{})
	handler := RequirePermission("orders:read", pdp, engine,
		CasbinPermissionConfig{EnforcerID: "enforcer-1", ModelText: "model"},
		fixedNamespaceResolver("namespace-1"),
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req = req.WithContext(WithSessionContext(req.Context(), SessionContext{
		OrganizationID:   "org-uuid",
		OrganizationSlug: "acme",
		Username:         "alice",
		Source:           "repository",
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Zero(t, nextCalls, "ReBAC dependency failures must not reach the protected handler")
}

func TestAPIAuthorizationMiddleware_NamespaceResolutionFailureReturnsUnavailable(t *testing.T) {
	nextCalls := 0
	pdp := &baseAuthorizationPDPStub{allowed: true}
	engine := apprebac.NewEngine(failingReBACTupleStore{})
	handler := APIAuthorizationMiddleware(pdp, engine, failingNamespaceResolver{})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			nextCalls++
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req = req.WithContext(WithSessionContext(req.Context(), SessionContext{
		OrganizationID:   "org-uuid",
		OrganizationSlug: "acme",
		Username:         "alice",
		Source:           "repository",
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Zero(t, nextCalls, "namespace resolution failures must not reach the protected handler")
}

func TestAPIAuthorizationMiddleware_MissingReBACEngineReturnsUnavailable(t *testing.T) {
	nextCalls := 0
	handler := APIAuthorizationMiddleware(&baseAuthorizationPDPStub{allowed: true}, nil, nil)(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalls++ }),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req = req.WithContext(WithSessionContext(req.Context(), SessionContext{
		OrganizationID:   "org-uuid",
		OrganizationSlug: "acme",
		Username:         "alice",
		Source:           "repository",
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Zero(t, nextCalls)
}

func TestRequirePermission_NamespaceResolutionFailureReturnsUnavailable(t *testing.T) {
	nextCalls := 0
	pdp := &baseAuthorizationPDPStub{allowed: true}
	engine := apprebac.NewEngine(failingReBACTupleStore{})
	handler := RequirePermission("orders:read", pdp, engine,
		CasbinPermissionConfig{EnforcerID: "enforcer-1", ModelText: "model"},
		failingNamespaceResolver{},
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req = req.WithContext(WithSessionContext(req.Context(), SessionContext{
		OrganizationID:   "org-uuid",
		OrganizationSlug: "acme",
		Username:         "alice",
		Source:           "repository",
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Zero(t, nextCalls, "namespace resolution failures must not reach the protected handler")
}

func TestRequirePermission_MissingReBACEngineReturnsUnavailable(t *testing.T) {
	nextCalls := 0
	handler := RequirePermission("orders:read", &baseAuthorizationPDPStub{allowed: true}, nil,
		CasbinPermissionConfig{EnforcerID: "enforcer-1", ModelText: "model"}, nil,
	)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalls++ }))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req = req.WithContext(WithSessionContext(req.Context(), SessionContext{
		OrganizationID:   "org-uuid",
		OrganizationSlug: "acme",
		Username:         "alice",
		Source:           "repository",
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Zero(t, nextCalls)
}

type privilegedAuthorizationPDPStub struct {
	baseAuthorizationPDPStub
	requiredLifecycle string
	privilegedCalls   int
	gotLifecycle      string
	gotRvals          []interface{}
}

func (s *privilegedAuthorizationPDPStub) EnforcePrivileged(_ context.Context, _, _ string, lifecycleID string, rvals ...interface{}) (bool, error) {
	s.privilegedCalls++
	s.gotLifecycle = lifecycleID
	s.gotRvals = rvals
	return s.requiredLifecycle != "" && lifecycleID == s.requiredLifecycle, nil
}

func runAPIAuthorizationForTest(pdp AuthorizationPDP, session SessionContext) (int, int) {
	nextCalls := 0
	handler := testAPIAuthorizationMiddleware(pdp)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	session.OrganizationID = "org-uuid"
	session.OrganizationSlug = "acme"
	session.Username = "alice"
	session.Source = "repository"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("X-Lifecycle-Id", "lifecycle-a")
	req = req.WithContext(WithSessionContext(req.Context(), session))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Code, nextCalls
}

func TestAPIAuthorizationMiddleware_OrdinarySessionCannotUseJIT(t *testing.T) {
	pdp := &privilegedAuthorizationPDPStub{requiredLifecycle: "lifecycle-a"}
	status, nextCalls := runAPIAuthorizationForTest(pdp, SessionContext{LifecycleID: "lifecycle-a"})
	assert.Equal(t, http.StatusForbidden, status)
	assert.Zero(t, nextCalls)
	// Verb and CRUD alias are both evaluated before default-deny.
	assert.Equal(t, 2, pdp.enforceCalls)
	assert.Zero(t, pdp.privilegedCalls)
}

func TestAPIAuthorizationMiddleware_PrivilegedSessionUsesBoundLifecycle(t *testing.T) {
	for _, tt := range []struct {
		name        string
		lifecycleID string
		wantStatus  int
	}{
		{name: "same lifecycle and time window allowed", lifecycleID: "lifecycle-a", wantStatus: http.StatusNoContent},
		{name: "another lifecycle denied despite request header", lifecycleID: "lifecycle-b", wantStatus: http.StatusForbidden},
		{name: "empty lifecycle denied", lifecycleID: "", wantStatus: http.StatusForbidden},
		{name: "blank lifecycle denied", lifecycleID: "  ", wantStatus: http.StatusForbidden},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pdp := &privilegedAuthorizationPDPStub{requiredLifecycle: "lifecycle-a"}
			status, nextCalls := runAPIAuthorizationForTest(pdp, SessionContext{IsPrivileged: true, LifecycleID: tt.lifecycleID})
			assert.Equal(t, tt.wantStatus, status)
			// Denied requests try both action candidates (verb then CRUD alias).
			wantCalls, wantAction := 1, "get"
			if tt.wantStatus != http.StatusNoContent {
				wantCalls, wantAction = 2, "read"
			}
			assert.Equal(t, wantCalls, pdp.privilegedCalls)
			assert.Zero(t, pdp.enforceCalls)
			assert.Equal(t, tt.lifecycleID, pdp.gotLifecycle)
			assert.Equal(t, []interface{}{"alice", "acme", "orders", wantAction}, pdp.gotRvals)
			if tt.wantStatus == http.StatusNoContent {
				assert.Equal(t, 1, nextCalls)
			} else {
				assert.Zero(t, nextCalls)
			}
		})
	}
}

func TestAPIAuthorizationMiddleware_PrivilegedSessionFailsClosedWithoutCapability(t *testing.T) {
	pdp := &baseAuthorizationPDPStub{allowed: true}
	status, nextCalls := runAPIAuthorizationForTest(pdp, SessionContext{IsPrivileged: true, LifecycleID: "lifecycle-a"})
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Zero(t, nextCalls)
	assert.Zero(t, pdp.enforceCalls)
}

func TestRequirePermission_PrivilegedSessionUsesPrivilegedPDP(t *testing.T) {
	pdp := &privilegedAuthorizationPDPStub{requiredLifecycle: "lifecycle-a"}
	handler := testRequirePermission("orders:read", pdp, CasbinPermissionConfig{EnforcerID: "enforcer-1", ModelText: "model"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	session := SessionContext{OrganizationSlug: "acme", Username: "alice", Source: "repository", IsPrivileged: true, LifecycleID: "lifecycle-a"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req = req.WithContext(WithSessionContext(req.Context(), session))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, 1, pdp.privilegedCalls)
	assert.Zero(t, pdp.enforceCalls)
	assert.Equal(t, []interface{}{"alice", "acme", "orders", "read"}, pdp.gotRvals)
}

// OPEN-API-72：路由 Permission 必须归一化成 Casbin 策略使用的 (resource, CRUD act)，
// 否则只有 */* 通配策略能命中，普通角色按权限点授权实际失效。
func TestCasbinObjectAndAction(t *testing.T) {
	cases := []struct {
		permission string
		method     string
		object     string
		action     string
	}{
		{"webauthn:write", http.MethodPost, "webauthn", "write"},
		{"webauthn:read", http.MethodGet, "webauthn", "read"},
		{"users:read", http.MethodGet, "users", "read"},
		{"users:write", http.MethodPatch, "users", "write"},
		{"users:password:reset", http.MethodPost, "users", "reset"},
		{"aigateway:providers", http.MethodGet, "aigateway", "read"},
		{"aigateway:providers", http.MethodPost, "aigateway", "write"},
		{"rebac:check", http.MethodPost, "rebac", "check"},
		{"public", http.MethodGet, "public", "read"},
	}
	for _, testCase := range cases {
		object, action := casbinObjectAndAction(testCase.permission, testCase.method)
		if object != testCase.object || action != testCase.action {
			t.Fatalf("%s %s => (%s,%s), want (%s,%s)", testCase.permission, testCase.method, object, action, testCase.object, testCase.action)
		}
	}
}
