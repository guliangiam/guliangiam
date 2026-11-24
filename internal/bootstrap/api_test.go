package bootstrap

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/guliangiam/guliangiam/cred"
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appgroups "github.com/guliangiam/guliangiam/internal/app/groups"
	apporders "github.com/guliangiam/guliangiam/internal/app/orders"
	apporganizations "github.com/guliangiam/guliangiam/internal/app/organizations"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	approles "github.com/guliangiam/guliangiam/internal/app/roles"
	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	apptokens "github.com/guliangiam/guliangiam/internal/app/tokens"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	actionsdomain "github.com/guliangiam/guliangiam/internal/domain/actions"
	adapterdomain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	agentdomain "github.com/guliangiam/guliangiam/internal/domain/agents"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	captchaproviderdomain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/domain/castickets"
	certdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	coupondomain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	entrydomain "github.com/guliangiam/guliangiam/internal/domain/entries"
	faceidproviderdomain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	formdomain "github.com/guliangiam/guliangiam/internal/domain/forms"
	gatewaydomain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	groupsdomain "github.com/guliangiam/guliangiam/internal/domain/groups"
	idvdomain "github.com/guliangiam/guliangiam/internal/domain/idv"
	igadomain "github.com/guliangiam/guliangiam/internal/domain/iga"
	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	keysdomain "github.com/guliangiam/guliangiam/internal/domain/keys"
	ldapdomain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	mcpserverdomain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	meteringdomain "github.com/guliangiam/guliangiam/internal/domain/metering"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	notificationproviderdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	notificationsdomain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	orderdomain "github.com/guliangiam/guliangiam/internal/domain/orders"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	paymentdomain "github.com/guliangiam/guliangiam/internal/domain/payments"
	permissiondomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	plandomain "github.com/guliangiam/guliangiam/internal/domain/plans"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	pricingdomain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	radiusaccountingdomain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	radiusconfigdomain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	rebac "github.com/guliangiam/guliangiam/internal/domain/rebac"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	resourcedomain "github.com/guliangiam/guliangiam/internal/domain/resources"
	riskdomain "github.com/guliangiam/guliangiam/internal/domain/risk"
	roledomain "github.com/guliangiam/guliangiam/internal/domain/roles"
	ruledomain "github.com/guliangiam/guliangiam/internal/domain/rules"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	subscriptiondomain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	syncerdomain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	tenantidentities "github.com/guliangiam/guliangiam/internal/domain/tenant_identities"
	tenantinstances "github.com/guliangiam/guliangiam/internal/domain/tenant_instances"
	"github.com/guliangiam/guliangiam/internal/domain/tenants"
	ticketdomain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	transactiondomain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	trusteddevicesdomain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/domain/web3identity"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webhookeventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	webhookdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	workloaddomain "github.com/guliangiam/guliangiam/internal/domain/workload"
	jwtinfra "github.com/guliangiam/guliangiam/internal/infra/jwt"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	applicationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/applications"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	groupstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/groups"
	organizationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/organizations"
	permissiontransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/permissions"
	providertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/providers"
	roletransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/roles"
	sessiontransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/sessions"
	tokentransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/tokens"
	usertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/users"
	httpmiddleware "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// wafMiddlewareForTest creates a WAF middleware for tests.
// Uses nil ruleSvc (no-op mode) which is valid and does not fail.
func wafMiddlewareForTest() *httpmiddleware.WAFMiddleware {
	m, err := httpmiddleware.NewWAFMiddleware(nil, nil)
	if err != nil {
		panic("WAF middleware for test should never fail: " + err.Error())
	}
	return m
}

func TestV1ApplicationsRoutes(t *testing.T) {
	router, _, _, applicationRepo, _, _, _, _ := newTestV1Router(t)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/applications?page=1&pageSize=10", nil)
	listRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(listRecorder, listRequest)
	assertStatus(t, listRecorder, http.StatusOK)

	getBySlugRecorder := httptest.NewRecorder()
	getBySlugRequest := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/by-slug/built-in", nil)
	getBySlugRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(getBySlugRecorder, getBySlugRequest)
	assertStatus(t, getBySlugRecorder, http.StatusOK)

	createBody := map[string]any{
		"organizationId":          "org-1",
		"slug":                    "console",
		"displayName":             "Console",
		"category":                "admin",
		"appType":                 "web",
		"homepageUrl":             "https://console.example.com",
		"domain":                  "console.example.com",
		"status":                  "active",
		"clientId":                "console-client",
		"clientSecret":            "console-secret-0123456789abcdef",
		"tokenEndpointAuthMethod": "client_secret_basic",
		"redirectUris":            []string{"https://console.example.com/callback"},
		"grantTypes":              []string{"authorization_code"},
		"customScopes": []map[string]any{{
			"scope":       "profile",
			"displayName": "Profile",
			"description": "Read profile data",
		}},
	}
	createRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/applications", createBody)
	assertStatus(t, createRecorder, http.StatusCreated)
	var createResponse struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("json.Unmarshal(application create response) error = %v", err)
	}
	if createResponse.ID != "00000000-0000-0000-0000-000000000301" {
		t.Fatalf("application create response id = %q, want %q", createResponse.ID, "00000000-0000-0000-0000-000000000301")
	}
	if createResponse.Status != "created" {
		t.Fatalf("application create response status = %q, want %q", createResponse.Status, "created")
	}
	if _, ok := applicationRepo.items["00000000-0000-0000-0000-000000000301"]; !ok {
		t.Fatalf("created application not found in fake repository")
	}
	if got := len(applicationRepo.items["00000000-0000-0000-0000-000000000301"].CustomScopes); got != 1 {
		t.Fatalf("created application customScopes len = %d, want %d", got, 1)
	}
	if got := applicationRepo.items["00000000-0000-0000-0000-000000000301"].ClientID; got != "console-client" {
		t.Fatalf("created application clientId = %q, want %q", got, "console-client")
	}
	// 新表是唯一写路径：legacy 列必须保持空，secret 行写入 app_client_secrets。
	if got := applicationRepo.items["00000000-0000-0000-0000-000000000301"].ClientSecretHash; got != "" {
		t.Fatalf("legacy clientSecretHash = %q, want empty (new table is the only write path)", got)
	}
	secrets := fakeBootstrapClientSecrets[fakeBootstrapClientSecretKey("org-1", "00000000-0000-0000-0000-000000000301")]
	if len(secrets) == 0 || secrets[0].SecretHash == "" {
		t.Fatalf("created application has no app_client_secrets row with a hash")
	}
	if secrets[0].SecretPrefix == "" {
		t.Fatalf("created application secret row has empty prefix")
	}
	if got := applicationRepo.items["00000000-0000-0000-0000-000000000301"].TokenEndpointAuthMethod; got != applicationdomain.TokenEndpointAuthMethodClientSecretBasic {
		t.Fatalf("created application tokenEndpointAuthMethod = %q, want %q", got, applicationdomain.TokenEndpointAuthMethodClientSecretBasic)
	}

	invalidCreateRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/applications", map[string]any{
		"organizationId": "org-1",
		"slug":           "console-bad",
		"displayName":    "Console",
		"homepageUrl":    "bad-url",
		"status":         "active",
	})
	assertStatus(t, invalidCreateRecorder, http.StatusUnprocessableEntity)

	duplicateCreateRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/applications", map[string]any{
		"organizationId": "org-1",
		"slug":           "portal",
		"displayName":    "Portal Duplicate",
		"status":         "active",
		// 默认 none 认证方式要求 authorization_code，否则请求先被 422 拒绝，
		// 覆盖不到本用例要验证的 slug 冲突路径。
		"grantTypes": []string{"authorization_code"},
	})
	assertStatus(t, duplicateCreateRecorder, http.StatusConflict)

	deleteRecorder := performJSONRequest(t, router, http.MethodDelete, "/api/v1/applications/00000000-0000-0000-0000-000000000301", nil)
	assertStatus(t, deleteRecorder, http.StatusOK)
	if _, ok := applicationRepo.items["00000000-0000-0000-0000-000000000301"]; ok {
		t.Fatalf("deleted application still exists in fake repository")
	}

	deleteMissingRecorder := performJSONRequest(t, router, http.MethodDelete, "/api/v1/applications/missing", nil)
	assertStatus(t, deleteMissingRecorder, http.StatusNotFound)

	patchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"slug":        "portal-updated",
		"displayName": "Portal Updated",
		// clientId is immutable (RFC 6749) — must not appear in PATCH requests.
		"clientSecret":            "portal-secret-updated",
		"tokenEndpointAuthMethod": "client_secret_post",
		"redirectUris": []string{
			"https://portal.example.com/callback",
			"https://portal.example.com/oidc/callback",
		},
		"grantTypes": []string{"authorization_code", "urn:ietf:params:oauth:grant-type:device_code"},
		"customScopes": []map[string]any{{
			"scope":       "email",
			"displayName": "Email",
			"description": "Read email",
		}},
		"revision": float64(1),
	})
	assertStatus(t, patchRecorder, http.StatusOK)
	if got := applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"].Slug; got != "portal-updated" {
		t.Fatalf("application slug = %q, want %q", got, "portal-updated")
	}
	if got := applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"].DisplayName; got != "Portal Updated" {
		t.Fatalf("application displayName = %q, want %q", got, "Portal Updated")
	}
	if got := len(applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"].CustomScopes); got != 1 || applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"].CustomScopes[0].Scope != "email" {
		t.Fatalf("application customScopes = %+v, want email scope", applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"].CustomScopes)
	}
	// clientId is immutable (RFC 6749) — PATCH must not change it.
	// This assert verifies the seed value was preserved (not mutated to "portal-client-updated").
	clientID := applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"].ClientID
	if clientID == "" {
		t.Fatalf("application clientId is empty, expected seed value")
	}
	if clientID == "portal-client-updated" {
		t.Fatalf("application clientId was mutated by PATCH — immutable contract violated")
	}
	// 新表是唯一写路径：legacy 列保持空，secret 行同事务写入 app_client_secrets。
	if got := applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"].ClientSecretHash; got != "" {
		t.Fatalf("legacy clientSecretHash = %q, want empty (new table is the only write path)", got)
	}
	if rows := fakeBootstrapClientSecrets[fakeBootstrapClientSecretKey("org-1", "01a00000-0000-7000-8000-0000000000a1")]; len(rows) == 0 || rows[len(rows)-1].SecretHash == "" {
		t.Fatalf("application has no app_client_secrets row with a hash after update")
	}
	if got := applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"].TokenEndpointAuthMethod; got != applicationdomain.TokenEndpointAuthMethodClientSecretPost {
		t.Fatalf("application tokenEndpointAuthMethod = %q, want %q", got, applicationdomain.TokenEndpointAuthMethodClientSecretPost)
	}

	invalidPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"displayName": "   ",
		"revision":    float64(2),
	})
	assertStatus(t, invalidPatchRecorder, http.StatusBadRequest)

	invalidSlugPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"slug":     "Portal Updated",
		"revision": float64(2),
	})
	assertStatus(t, invalidSlugPatchRecorder, http.StatusUnprocessableEntity)

	duplicateSlugPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"slug":     "confidential-portal",
		"revision": float64(2),
	})
	assertStatus(t, duplicateSlugPatchRecorder, http.StatusConflict)

	// Immutable guard: clientId MUST NOT appear in PATCH — RFC 6749 discipline.
	immutableClientIDPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"clientId": "gl_cid_immutable-violation",
		"revision": float64(2),
	})
	assertStatus(t, immutableClientIDPatchRecorder, http.StatusUnprocessableEntity)

	// Also verify empty-string clientId is rejected — guard is strict, no "skip" mode.
	immutableEmptyClientIDPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"clientId": "",
		"revision": float64(2),
	})
	assertStatus(t, immutableEmptyClientIDPatchRecorder, http.StatusUnprocessableEntity)

	invalidCustomScopeRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"customScopes": []map[string]any{{"scope": "email"}, {"scope": "email"}},
		"revision":     float64(2),
	})
	assertStatus(t, invalidCustomScopeRecorder, http.StatusUnprocessableEntity)

	invalidOAuthSettingsRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"clientId":     "portal-client-updated",
		"redirectUris": []string{"bad-uri"},
		"revision":     float64(2),
	})
	assertStatus(t, invalidOAuthSettingsRecorder, http.StatusUnprocessableEntity)

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", nil)
	getRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(getRecorder, getRequest)
	assertStatus(t, getRecorder, http.StatusOK)
	var getResponse struct {
		Slug                    string   `json:"slug"`
		ClientID                string   `json:"clientId"`
		TokenEndpointAuthMethod string   `json:"tokenEndpointAuthMethod"`
		RedirectURIs            []string `json:"redirectUris"`
		GrantTypes              []string `json:"grantTypes"`
		CustomScopes            []struct {
			Scope string `json:"scope"`
		} `json:"customScopes"`
	}
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &getResponse); err != nil {
		t.Fatalf("json.Unmarshal(application detail response) error = %v", err)
	}
	if len(getResponse.CustomScopes) != 1 || getResponse.CustomScopes[0].Scope != "email" {
		t.Fatalf("unexpected application customScopes response: %+v", getResponse.CustomScopes)
	}
	if getResponse.Slug != "portal-updated" {
		t.Fatalf("unexpected application slug response: %+v", getResponse)
	}
	if getResponse.ClientID == "" || getResponse.ClientID == "portal-client-updated" {
		t.Fatalf("unexpected application oauth settings response (clientId must be immutable): %+v", getResponse)
	}
	if getResponse.TokenEndpointAuthMethod != applicationdomain.TokenEndpointAuthMethodClientSecretPost || len(getResponse.RedirectURIs) != 2 || len(getResponse.GrantTypes) != 2 {
		t.Fatalf("unexpected application oauth settings response: %+v", getResponse)
	}

	grantSummaryConsentRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/grant", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","grantedScopes":["email"]}`)))
	grantSummaryConsentRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	grantSummaryConsentRequest.Header.Set("Content-Type", "application/json")
	grantSummaryConsentRecorder := httptest.NewRecorder()
	router.ServeHTTP(grantSummaryConsentRecorder, grantSummaryConsentRequest)
	assertStatus(t, grantSummaryConsentRecorder, http.StatusOK)

	deleteSummaryRecorder := httptest.NewRecorder()
	deleteSummaryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1/delete-summary", nil)
	deleteSummaryRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(deleteSummaryRecorder, deleteSummaryRequest)
	assertStatus(t, deleteSummaryRecorder, http.StatusOK)
	var deleteSummaryResponse struct {
		ApplicationID        string `json:"applicationId"`
		ProviderBindingCount int64  `json:"providerBindingCount"`
		ActiveTokenCount     int64  `json:"activeTokenCount"`
		ActiveConsentCount   int64  `json:"activeConsentCount"`
	}
	if err := json.Unmarshal(deleteSummaryRecorder.Body.Bytes(), &deleteSummaryResponse); err != nil {
		t.Fatalf("json.Unmarshal(application delete summary response) error = %v", err)
	}
	if deleteSummaryResponse.ApplicationID != "01a00000-0000-7000-8000-0000000000a1" || deleteSummaryResponse.ProviderBindingCount != 1 || deleteSummaryResponse.ActiveTokenCount != 2 || deleteSummaryResponse.ActiveConsentCount != 1 {
		t.Fatalf("unexpected application delete summary response: %+v", deleteSummaryResponse)
	}

	bindingRecorder := httptest.NewRecorder()
	bindingRequest := httptest.NewRequest(http.MethodGet, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1/provider-bindings", nil)
	bindingRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(bindingRecorder, bindingRequest)
	assertStatus(t, bindingRecorder, http.StatusOK)

	replaceBindingsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1/provider-bindings", map[string]any{
		"items": []map[string]any{{"providerId": "provider-1"}, {"providerId": "provider-2"}},
	})
	assertStatus(t, replaceBindingsRecorder, http.StatusOK)
	if got := len(applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"].ProviderBindings); got != 2 {
		t.Fatalf("application providerBindings len = %d, want %d", got, 2)
	}

	invalidBindingsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1/provider-bindings", map[string]any{
		"items": []map[string]any{{"providerId": "provider-1"}, {"providerId": "provider-1"}},
	})
	assertStatus(t, invalidBindingsRecorder, http.StatusUnprocessableEntity)

	notFoundRecorder := httptest.NewRecorder()
	notFoundRequest := httptest.NewRequest(http.MethodGet, "/api/v1/applications/missing", nil)
	notFoundRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(notFoundRecorder, notFoundRequest)
	assertStatus(t, notFoundRecorder, http.StatusNotFound)

	invalidRecorder := performRawJSONRequest(router, http.MethodPost, "/api/v1/applications", []byte(`{"organizationId":"org-1","slug":"app","displayName":"App","status":"active","unknown":true}`))
	assertStatus(t, invalidRecorder, http.StatusBadRequest)
}

func TestV1UserMfaSetupRoutes(t *testing.T) {
	router, orgRepo, userRepo, _, _, _, _, _ := newTestV1Router(t)
	orgRepo.items["org-1"].Security.SessionPolicy = map[string]any{"mfaRememberHours": 24}

	initiateRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/users/user-1/mfa-setup/initiate", map[string]any{
		"mfaType": "app",
	})
	assertStatus(t, initiateRecorder, http.StatusOK)

	var initiateResponse struct {
		Secret             string   `json:"secret"`
		URL                string   `json:"url"`
		RecoveryCodes      []string `json:"recoveryCodes"`
		MfaRememberInHours int64    `json:"mfaRememberInHours"`
	}
	if err := json.Unmarshal(initiateRecorder.Body.Bytes(), &initiateResponse); err != nil {
		t.Fatalf("json.Unmarshal(initiate user mfa setup response) error = %v", err)
	}
	if initiateResponse.Secret == "" || initiateResponse.URL == "" {
		t.Fatalf("expected initiate response to contain secret and url, got %+v", initiateResponse)
	}
	if len(initiateResponse.RecoveryCodes) != 1 {
		t.Fatalf("expected initiate response to contain one recovery code, got %+v", initiateResponse.RecoveryCodes)
	}
	if initiateResponse.MfaRememberInHours != 24 {
		t.Fatalf("initiate response remember hours = %d, want %d", initiateResponse.MfaRememberInHours, 24)
	}

	verifyRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/users/user-1/mfa-setup/verify", map[string]any{
		"mfaType":  "app",
		"passcode": mustGenerateTotpCode(t, initiateResponse.Secret, time.Now()),
		"secret":   initiateResponse.Secret,
	})
	assertStatus(t, verifyRecorder, http.StatusOK)

	enableRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/users/user-1/mfa-setup/enable", map[string]any{
		"mfaType":       "app",
		"secret":        initiateResponse.Secret,
		"passcode":      mustGenerateTotpCode(t, initiateResponse.Secret, fixedNow()),
		"recoveryCodes": initiateResponse.RecoveryCodes,
	})
	assertStatus(t, enableRecorder, http.StatusOK)

	if got := userRepo.items["user-1"].TotpSecret; got != initiateResponse.Secret {
		t.Fatalf("user totp secret = %q, want %q", got, initiateResponse.Secret)
	}
	if got := userRepo.items["user-1"].PreferredMfaType; got != "totp" {
		t.Fatalf("user preferredMfaType after setup = %q, want %q", got, "totp")
	}
	codeHashes := userRepo.items["user-1"].RecoveryCodeHashes
	if len(codeHashes) != 1 {
		t.Fatalf("unexpected recovery code hashes after setup: %+v", codeHashes)
	}
	if ok, err := platformcrypto.VerifyClientSecret(initiateResponse.RecoveryCodes[0], codeHashes[0]); err != nil || !ok {
		t.Fatalf("stored recovery code hash does not verify (err=%v)", err)
	}
	if len(userRepo.items["user-1"].MfaMethods) != 1 {
		t.Fatalf("user mfa method count after setup = %d, want %d", len(userRepo.items["user-1"].MfaMethods), 1)
	}
	if got := userRepo.items["user-1"].MfaMethods[0].MethodType; got != "totp" {
		t.Fatalf("user mfa method type after setup = %q, want %q", got, "totp")
	}
	if !userRepo.items["user-1"].MfaMethods[0].Enabled || !userRepo.items["user-1"].MfaMethods[0].Verified {
		t.Fatalf("expected user mfa method to be enabled and verified after setup, got %+v", userRepo.items["user-1"].MfaMethods[0])
	}
}

func TestV1ApplicationsPatchKeepsExistingClientSecretOnPartialUpdate(t *testing.T) {
	router, _, _, applicationRepo, _, _, _, _ := newTestV1Router(t)

	makeConfidentialRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		// clientId is immutable (RFC 6749) — must not appear in PATCH requests.
		"clientSecret":            "portal-secret-updated",
		"tokenEndpointAuthMethod": "client_secret_post",
		"redirectUris":            []string{"https://portal.example.com/callback"},
		"grantTypes":              []string{"authorization_code"},
		"revision":                float64(1),
	})
	assertStatus(t, makeConfidentialRecorder, http.StatusOK)

	partialPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"description": "Portal after confidential update",
		"revision":    float64(2),
	})
	assertStatus(t, partialPatchRecorder, http.StatusOK)

	application := applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"]
	if got := application.Description; got != "Portal after confidential update" {
		t.Fatalf("application description = %q, want %q", got, "Portal after confidential update")
	}
	if got := application.ClientSecretHash; got != "" {
		t.Fatalf("legacy clientSecretHash = %q, want empty (new table is the only write path)", got)
	}
	secrets := fakeBootstrapClientSecrets[fakeBootstrapClientSecretKey("org-1", "01a00000-0000-7000-8000-0000000000a1")]
	if len(secrets) == 0 {
		t.Fatalf("application has no app_client_secrets row after secret update")
	}
	if last := secrets[len(secrets)-1]; last.SecretHash == "" || last.Status != applicationdomain.ClientSecretStatusActive {
		t.Fatalf("last secret row hash=%q status=%q, want non-empty hash and active", last.SecretHash, last.Status)
	}
	if got := application.TokenEndpointAuthMethod; got != applicationdomain.TokenEndpointAuthMethodClientSecretPost {
		t.Fatalf("application tokenEndpointAuthMethod = %q, want %q", got, applicationdomain.TokenEndpointAuthMethodClientSecretPost)
	}

	// Regression test for "unknown field \"logo\"" bug — FE payload sends logo,
	// expireInHours, isVisible, etc. PatchApplicationRequest must accept ALL of them
	// (DecideUnknownFields on global route decoder). Core assertion: no decoder error.
	logoPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/applications/01a00000-0000-7000-8000-0000000000a1", map[string]any{
		"logo":          "https://cdn.example.com/new-logo.png",
		"isVisible":     true,
		"isPortal":      false,
		"expireInHours": 48,
		"tokenFormat":   "JWT",
		"revision":      float64(3),
	})
	assertStatus(t, logoPatchRecorder, http.StatusOK)
	if body := logoPatchRecorder.Body.String(); containsSubstring(body, "unknown field") {
		t.Fatalf("PATCH with logo/expireInHours still hits DecideUnknownFields: %s", body)
	}
	// Logo + expireInHours should be persisted (patch logic added with the fix).
	application = applicationRepo.items["01a00000-0000-7000-8000-0000000000a1"]
	if got := application.Logo; got != "https://cdn.example.com/new-logo.png" {
		t.Fatalf("application logo = %q, want new-logo.png", got)
	}
	if got := application.ExpireInHours; got != 48 {
		t.Fatalf("application expireInHours = %d, want 48", got)
	}
	if got := application.TokenFormat; got != "JWT" {
		t.Fatalf("application tokenFormat = %q, want JWT", got)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (len(s) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestV1AuthAccountRoute(t *testing.T) {
	router, _, _, _, _, _, _, _ := newTestV1Router(t)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/account", nil)
	request.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	request.Header.Set("Authorization", "Bearer token-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assertStatus(t, recorder, http.StatusOK)

	var response struct {
		Account struct {
			Owner       string `json:"owner"`
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			IsAdmin     bool   `json:"isAdmin"`
		} `json:"account"`
		Organization struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"organization"`
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Account.Owner != "built-in" || response.Account.Name != "jane" {
		t.Fatalf("unexpected account response: %+v", response.Account)
	}
	if response.Organization.ID != "org-1" || response.AccessToken != "" {
		t.Fatalf("unexpected organization/access token response: %+v", response)
	}

	unauthenticatedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/account", nil)
	unauthenticatedRecorder := httptest.NewRecorder()
	router.ServeHTTP(unauthenticatedRecorder, unauthenticatedRequest)
	assertStatus(t, unauthenticatedRecorder, http.StatusUnauthorized)

	// 历史 HMAC 签名 Cookie（不落库、明文内嵌 AccessToken）不再能建立会话：解码兼容已随
	// OPEN-API-19 收口删除，会话只来自仓库。这里把它固定成回归断言——若哪天又有人把签名
	// Cookie 接回会话解析，这条用例会立刻变红。
	signedCookieRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/account", nil)
	signedCookieRequest.AddCookie(&http.Cookie{
		Name: authtransport.SessionCookieName,
		Value: mustEncodeV1SessionCookie(t, map[string]string{
			"organizationSlug": "built-in",
			"username":         "jane",
			"accessToken":      "cookie-token-456",
		}),
	})
	signedCookieRecorder := httptest.NewRecorder()
	router.ServeHTTP(signedCookieRecorder, signedCookieRequest)
	assertStatus(t, signedCookieRecorder, http.StatusUnauthorized)

	repositoryCookieRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/account", nil)
	repositoryCookieRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	repositoryCookieRecorder := httptest.NewRecorder()
	router.ServeHTTP(repositoryCookieRecorder, repositoryCookieRequest)
	assertStatus(t, repositoryCookieRecorder, http.StatusOK)

	if err := json.Unmarshal(repositoryCookieRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() repository cookie response error = %v", err)
	}
	if response.Account.Name != "jane" || response.Organization.Name != "built-in" {
		t.Fatalf("unexpected repository cookie account response: %+v", response)
	}

	staleRepositoryCookieRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/account", nil)
	staleRepositoryCookieRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-6"})
	staleRepositoryCookieRecorder := httptest.NewRecorder()
	router.ServeHTTP(staleRepositoryCookieRecorder, staleRepositoryCookieRequest)
	assertStatus(t, staleRepositoryCookieRecorder, http.StatusOK)

	idleExpiredRepositoryCookieRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/account", nil)
	idleExpiredRepositoryCookieRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-5"})
	idleExpiredRepositoryCookieRecorder := httptest.NewRecorder()
	router.ServeHTTP(idleExpiredRepositoryCookieRecorder, idleExpiredRepositoryCookieRequest)
	assertStatus(t, idleExpiredRepositoryCookieRecorder, http.StatusUnauthorized)
}

func TestV1AuthLoginRoute(t *testing.T) {
	router, _, userRepo, _, _, _, _, _ := newTestV1Router(t)

	loginRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "Password!2026",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
	})
	assertStatus(t, loginRecorder, http.StatusOK)

	var response struct {
		Status  string `json:"status"`
		Account struct {
			Name string `json:"name"`
		} `json:"account"`
	}
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(login response) error = %v", err)
	}
	if response.Status != "success" {
		t.Fatalf("unexpected login status: %q", response.Status)
	}
	if response.Account.Name != "jane" {
		t.Fatalf("unexpected login account response: %+v", response)
	}

	cookies := loginRecorder.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != authtransport.SessionCookieName || cookies[0].Value == "" {
		t.Fatalf("expected login response to set session cookie")
	}

	if userRepo.items["user-1"].LastSigninAt == nil {
		t.Fatalf("expected login to update last signin timestamp")
	}

	invalidRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "wrong-password",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
	})
	assertStatus(t, invalidRecorder, http.StatusUnauthorized)
}

func TestV1AuthLoginRequiresMfaRoute(t *testing.T) {
	router, _, userRepo, _, _, _, _, _ := newTestV1Router(t)
	userRepo.items["user-1"].MfaMethods = []userdomain.UserMfaMethod{
		{
			ID:          "mfa-1",
			UserID:      "user-1",
			MethodType:  "totp",
			Enabled:     true,
			Verified:    true,
			DisplayName: "Authenticator",
		},
	}
	userRepo.items["user-1"].PreferredMfaType = "totp"
	userRepo.items["user-1"].TotpSecret = "JBSWY3DPEHPK3PXP"

	loginRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "Password!2026",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
	})
	assertStatus(t, loginRecorder, http.StatusOK)

	var response struct {
		Status              string   `json:"status"`
		ChallengeID         string   `json:"challengeId"`
		ChallengeExpiresAt  string   `json:"challengeExpiresAt"`
		NextMfa             string   `json:"nextMfa"`
		RequiredMfa         []string `json:"requiredMfa"`
		AvailableMfaMethods []struct {
			ID         string `json:"id"`
			MethodType string `json:"methodType"`
		} `json:"availableMfaMethods"`
	}
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(mfa login response) error = %v", err)
	}
	if response.Status != "mfa_required" {
		t.Fatalf("unexpected login status: %q", response.Status)
	}
	if response.NextMfa != "totp" {
		t.Fatalf("unexpected nextMfa: %q", response.NextMfa)
	}
	if len(response.RequiredMfa) != 1 || response.RequiredMfa[0] != "totp" {
		t.Fatalf("unexpected requiredMfa: %+v", response.RequiredMfa)
	}
	if len(response.AvailableMfaMethods) != 1 || response.AvailableMfaMethods[0].MethodType != "totp" {
		t.Fatalf("unexpected availableMfaMethods: %+v", response.AvailableMfaMethods)
	}
	if response.ChallengeID == "" || response.ChallengeExpiresAt == "" {
		t.Fatalf("expected MFA-required response to return challenge metadata: %+v", response)
	}

	if cookies := loginRecorder.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("expected MFA-required response to skip session cookie, got %+v", cookies)
	}
	if userRepo.items["user-1"].LastSigninAt != nil {
		t.Fatalf("expected MFA-required login to avoid updating last signin timestamp")
	}

	verificationRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"challengeId":       response.ChallengeID,
		"mfaType":           "totp",
		"passcode":          mustGenerateTotpCode(t, "JBSWY3DPEHPK3PXP", fixedNow()),
		"enableMfaRemember": true,
	})
	assertStatus(t, verificationRecorder, http.StatusOK)

	var successResponse struct {
		Status  string `json:"status"`
		Account struct {
			Name string `json:"name"`
		} `json:"account"`
	}
	if err := json.Unmarshal(verificationRecorder.Body.Bytes(), &successResponse); err != nil {
		t.Fatalf("json.Unmarshal(totp verification response) error = %v", err)
	}
	if successResponse.Status != "success" || successResponse.Account.Name != "jane" {
		t.Fatalf("unexpected totp verification response: %+v", successResponse)
	}
	if cookies := verificationRecorder.Result().Cookies(); len(cookies) == 0 || cookies[0].Value == "" {
		t.Fatalf("expected successful MFA verification to issue session cookie")
	}
	if userRepo.items["user-1"].MfaRememberDeadline == nil {
		t.Fatalf("expected successful MFA verification to set remember deadline")
	}
	if userRepo.items["user-1"].MfaMethods[0].LastUsedAt == nil {
		t.Fatalf("expected successful MFA verification to mark MFA method as used")
	}

	rememberedLoginRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "Password!2026",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
	})
	assertStatus(t, rememberedLoginRecorder, http.StatusOK)

	if err := json.Unmarshal(rememberedLoginRecorder.Body.Bytes(), &successResponse); err != nil {
		t.Fatalf("json.Unmarshal(remembered login response) error = %v", err)
	}
	if successResponse.Status != "success" {
		t.Fatalf("unexpected remembered login status: %q", successResponse.Status)
	}
}

func TestV1AuthLoginRequiresMfaSetupRoute(t *testing.T) {
	router, orgRepo, userRepo, _, _, _, _, _ := newTestV1Router(t)
	orgRepo.items["org-1"].Security.MfaPolicy = map[string]any{"required": true}
	userRepo.items["user-1"].PreferredMfaType = "totp"

	loginRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "Password!2026",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
	})
	assertStatus(t, loginRecorder, http.StatusOK)

	var response struct {
		Status             string   `json:"status"`
		ChallengeID        string   `json:"challengeId"`
		ChallengeExpiresAt string   `json:"challengeExpiresAt"`
		RequiredMfa        []string `json:"requiredMfa"`
	}
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(required setup login response) error = %v", err)
	}
	if response.Status != "required_mfa_setup" {
		t.Fatalf("unexpected login status: %q", response.Status)
	}
	if len(response.RequiredMfa) != 1 || response.RequiredMfa[0] != "app" {
		t.Fatalf("unexpected requiredMfa: %+v", response.RequiredMfa)
	}
	if response.ChallengeID == "" || response.ChallengeExpiresAt == "" {
		t.Fatalf("expected setup-required response to include challenge metadata, got %+v", response)
	}
	if cookies := loginRecorder.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("expected setup-required response to skip session cookie, got %+v", cookies)
	}
	if userRepo.items["user-1"].LastSigninAt != nil {
		t.Fatalf("expected setup-required login to avoid updating last signin timestamp")
	}
	if len(userRepo.items["user-1"].MfaMethods) != 0 {
		t.Fatalf("expected setup-required login to avoid mutating MFA methods, got %+v", userRepo.items["user-1"].MfaMethods)
	}

	userRepo.items["user-1"].MfaMethods = []userdomain.UserMfaMethod{
		{
			ID:          "mfa-setup-1",
			UserID:      "user-1",
			MethodType:  "totp",
			Enabled:     true,
			Verified:    true,
			DisplayName: "Authenticator",
		},
	}
	userRepo.items["user-1"].TotpSecret = "JBSWY3DPEHPK3PXP"

	verificationRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"challengeId": response.ChallengeID,
		"mfaType":     "totp",
		"passcode":    mustGenerateTotpCode(t, "JBSWY3DPEHPK3PXP", fixedNow()),
	})
	assertStatus(t, verificationRecorder, http.StatusOK)

	var successResponse struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(verificationRecorder.Body.Bytes(), &successResponse); err != nil {
		t.Fatalf("json.Unmarshal(required setup verification response) error = %v", err)
	}
	if successResponse.Status != "success" {
		t.Fatalf("unexpected required setup verification status: %q", successResponse.Status)
	}
}

func TestV1AuthLoginRequiresEmailMfaRoute(t *testing.T) {
	router, _, userRepo, _, _, _, _, _ := newTestV1Router(t)
	userRepo.items["user-1"].MfaMethods = []userdomain.UserMfaMethod{
		{
			ID:          "mfa-email-1",
			UserID:      "user-1",
			MethodType:  "email",
			Enabled:     true,
			Verified:    true,
			DisplayName: "Email OTP",
		},
	}
	userRepo.items["user-1"].PreferredMfaType = "email"
	userRepo.items["user-1"].Profile.Email = "jane@example.com"

	loginRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "Password!2026",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
	})
	assertStatus(t, loginRecorder, http.StatusOK)

	var response struct {
		Status              string `json:"status"`
		NextMfa             string `json:"nextMfa"`
		AvailableMfaMethods []struct {
			MethodType  string `json:"methodType"`
			Target      string `json:"target"`
			CountryCode string `json:"countryCode"`
		} `json:"availableMfaMethods"`
	}
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(email mfa login response) error = %v", err)
	}
	if response.Status != "mfa_required" || response.NextMfa != "email" {
		t.Fatalf("unexpected email mfa login response: %+v", response)
	}
	if len(response.AvailableMfaMethods) != 1 {
		t.Fatalf("unexpected availableMfaMethods len: %+v", response.AvailableMfaMethods)
	}
	if response.AvailableMfaMethods[0].MethodType != "email" || response.AvailableMfaMethods[0].Target != "jane@example.com" {
		t.Fatalf("unexpected email availableMfaMethod payload: %+v", response.AvailableMfaMethods[0])
	}
}

func TestV1AuthLoginRequiresRadiusMfaRoute(t *testing.T) {
	router, _, userRepo, _, _, _, _, _ := newTestV1Router(t)
	userRepo.items["user-1"].MfaMethods = []userdomain.UserMfaMethod{
		{
			ID:          "mfa-radius-1",
			UserID:      "user-1",
			MethodType:  "radius",
			Enabled:     true,
			Verified:    true,
			DisplayName: "Corporate RADIUS",
		},
	}
	userRepo.items["user-1"].PreferredMfaType = "radius"
	userRepo.items["user-1"].MfaRadiusUsername = "jane-radius"
	userRepo.items["user-1"].MfaRadiusProvider = "provider-radius-1"

	loginRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "Password!2026",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
	})
	assertStatus(t, loginRecorder, http.StatusOK)

	var response struct {
		Status              string `json:"status"`
		NextMfa             string `json:"nextMfa"`
		AvailableMfaMethods []struct {
			MethodType string `json:"methodType"`
			Target     string `json:"target"`
		} `json:"availableMfaMethods"`
	}
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(radius mfa login response) error = %v", err)
	}
	if response.Status != "mfa_required" || response.NextMfa != "radius" {
		t.Fatalf("unexpected radius mfa login response: %+v", response)
	}
	if len(response.AvailableMfaMethods) != 1 {
		t.Fatalf("unexpected availableMfaMethods len: %+v", response.AvailableMfaMethods)
	}
	if response.AvailableMfaMethods[0].MethodType != "radius" || response.AvailableMfaMethods[0].Target != "jane-radius" {
		t.Fatalf("unexpected radius availableMfaMethod payload: %+v", response.AvailableMfaMethods[0])
	}
}

func TestV1AuthLoginRequiresPushMfaRoute(t *testing.T) {
	originalSendPushAuthentication := appauth.SendPushAuthenticationForTest()
	appauth.SetSendPushAuthenticationForTest(func(ctx context.Context, config appauth.PushProviderConfigForTest, receiver string, title string, message string) error {
		if receiver != "device-token-1" {
			return apperrors.NewInvalidArgument("unexpected push receiver")
		}
		return nil
	})
	defer appauth.SetSendPushAuthenticationForTest(originalSendPushAuthentication)

	router, _, userRepo, _, providerRepo, _, _, _ := newTestV1Router(t)
	userRepo.items["user-1"].MfaMethods = []userdomain.UserMfaMethod{
		{
			ID:          "mfa-push-1",
			UserID:      "user-1",
			MethodType:  "push",
			Enabled:     true,
			Verified:    true,
			DisplayName: "Push Approval",
		},
	}
	userRepo.items["user-1"].PreferredMfaType = "push"
	userRepo.items["user-1"].MfaPushReceiver = "device-token-1"
	userRepo.items["user-1"].MfaPushProvider = "provider-push-1"
	providerRepo.items["provider-push-1"] = &providerdomain.Provider{
		ID:             "provider-push-1",
		OrganizationID: "org-1",
		Name:           "pushbullet",
		DisplayName:    "Pushbullet",
		Category:       "Notification",
		ProviderType:   "Pushbullet",
		ClientSecret:   "push-secret",
		CreatedAt:      fixedNow(),
		UpdatedAt:      fixedNow(),
		Revision:       1,
	}
	providerRepo.configs["provider-push-1"] = &providerdomain.ProviderConfig{
		ProviderID: "provider-push-1",
		ConfigType: "notification",
		Config: map[string]any{
			"type":         "Pushbullet",
			"clientSecret": "push-secret",
		},
		UpdatedAt: fixedNow(),
	}

	loginRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "Password!2026",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
	})
	assertStatus(t, loginRecorder, http.StatusOK)

	var response struct {
		Status              string `json:"status"`
		NextMfa             string `json:"nextMfa"`
		ChallengeID         string `json:"challengeId"`
		ChallengeState      string `json:"challengeState"`
		AvailableMfaMethods []struct {
			MethodType string `json:"methodType"`
			Target     string `json:"target"`
		} `json:"availableMfaMethods"`
	}
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(push mfa login response) error = %v", err)
	}
	if response.Status != "mfa_required" || response.NextMfa != "push" {
		t.Fatalf("unexpected push mfa login response: %+v", response)
	}
	if response.ChallengeID == "" || response.ChallengeState != mfachallengedomain.StatePending {
		t.Fatalf("unexpected push challenge response: %+v", response)
	}
	if len(response.AvailableMfaMethods) != 1 {
		t.Fatalf("unexpected availableMfaMethods len: %+v", response.AvailableMfaMethods)
	}
	if response.AvailableMfaMethods[0].MethodType != "push" || response.AvailableMfaMethods[0].Target != "device-token-1" {
		t.Fatalf("unexpected push availableMfaMethod payload: %+v", response.AvailableMfaMethods[0])
	}

	pushTriggerRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"challengeId": response.ChallengeID,
		"mfaType":     "push",
	})
	assertStatus(t, pushTriggerRecorder, http.StatusOK)

	if err := json.Unmarshal(pushTriggerRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(push trigger response) error = %v", err)
	}
	if response.Status != "mfa_required" || response.ChallengeState != mfachallengedomain.StatePushPending {
		t.Fatalf("unexpected push trigger response: %+v", response)
	}

	challengeRecorder := httptest.NewRecorder()
	challengeRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login-challenges/"+response.ChallengeID, nil)
	challengeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(challengeRecorder, challengeRequest)
	assertStatus(t, challengeRecorder, http.StatusOK)

	var challengeResponse struct {
		ID         string `json:"id"`
		State      string `json:"state"`
		MethodType string `json:"methodType"`
	}
	if err := json.Unmarshal(challengeRecorder.Body.Bytes(), &challengeResponse); err != nil {
		t.Fatalf("json.Unmarshal(push challenge response) error = %v", err)
	}
	if challengeResponse.ID != response.ChallengeID || challengeResponse.State != mfachallengedomain.StatePushPending || challengeResponse.MethodType != "push" {
		t.Fatalf("unexpected push challenge state response: %+v", challengeResponse)
	}

	approveRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login-challenges/"+response.ChallengeID+"/approve", map[string]any{})
	assertStatus(t, approveRecorder, http.StatusOK)

	verificationRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"challengeId":       response.ChallengeID,
		"mfaType":           "push",
		"enableMfaRemember": true,
	})
	assertStatus(t, verificationRecorder, http.StatusOK)

	var verificationResponse struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(verificationRecorder.Body.Bytes(), &verificationResponse); err != nil {
		t.Fatalf("json.Unmarshal(push verification response) error = %v", err)
	}
	if verificationResponse.Status != "success" {
		t.Fatalf("unexpected push verification response: %+v", verificationResponse)
	}
	if userRepo.items["user-1"].MfaRememberDeadline == nil {
		t.Fatalf("expected push approval login to set MFA remember deadline")
	}
}

func TestV1AuthLoginWithRecoveryCodeRoute(t *testing.T) {
	router, _, userRepo, _, _, _, _, _ := newTestV1Router(t)
	userRepo.items["user-1"].MfaMethods = []userdomain.UserMfaMethod{
		{
			ID:          "mfa-1",
			UserID:      "user-1",
			MethodType:  "totp",
			Enabled:     true,
			Verified:    true,
			DisplayName: "Authenticator",
		},
	}
	userRepo.items["user-1"].RecoveryCodeHashes = []string{mustHashRecoveryCode(t, "recovery-1"), mustHashRecoveryCode(t, "recovery-2")}

	loginRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "Password!2026",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
	})
	assertStatus(t, loginRecorder, http.StatusOK)

	var response struct {
		Status      string `json:"status"`
		ChallengeID string `json:"challengeId"`
	}
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(recovery login response) error = %v", err)
	}
	if response.Status != "mfa_required" || response.ChallengeID == "" {
		t.Fatalf("unexpected recovery login response: %+v", response)
	}

	recoveryRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"challengeId":  response.ChallengeID,
		"recoveryCode": "recovery-1",
	})
	assertStatus(t, recoveryRecorder, http.StatusOK)

	if err := json.Unmarshal(recoveryRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(recovery verification response) error = %v", err)
	}
	if response.Status != "success" {
		t.Fatalf("unexpected recovery verification status: %q", response.Status)
	}
	remaining := userRepo.items["user-1"].RecoveryCodeHashes
	if len(remaining) != 1 {
		t.Fatalf("expected used recovery code to be consumed, got %+v", remaining)
	}
	if ok, err := platformcrypto.VerifyClientSecret("recovery-2", remaining[0]); err != nil || !ok {
		t.Fatalf("remaining hash should verify against recovery-2 (err=%v)", err)
	}
	if ok, _ := platformcrypto.VerifyClientSecret("recovery-1", remaining[0]); ok {
		t.Fatalf("consumed recovery code still verifies")
	}

	invalidRecoveryRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"challengeId":  response.ChallengeID,
		"recoveryCode": "missing-code",
	})
	assertStatus(t, invalidRecoveryRecorder, http.StatusUnauthorized)
}

func TestV1AuthLogoutRoute(t *testing.T) {
	router, _, _, _, _, _, _, _ := newTestV1Router(t)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assertStatus(t, recorder, http.StatusOK)

	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response["status"] != "logged_out" {
		t.Fatalf("logout status = %q, want %q", response["status"], "logged_out")
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected logout response to clear session cookie")
	}
	if cookies[0].Name != authtransport.SessionCookieName || cookies[0].MaxAge != -1 {
		t.Fatalf("unexpected logout cookie: %+v", cookies[0])
	}

	reusedSessionRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/account", nil)
	reusedSessionRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	reusedSessionRecorder := httptest.NewRecorder()
	router.ServeHTTP(reusedSessionRecorder, reusedSessionRequest)
	assertStatus(t, reusedSessionRecorder, http.StatusUnauthorized)
}

func TestV1AuthConsentRoutes(t *testing.T) {
	router, _, _, _, _, _, _, _ := newTestV1Router(t)

	checkRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/check", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","requestedScopes":["openid","profile","email"]}`)))
	checkRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	checkRequest.Header.Set("Content-Type", "application/json")
	checkRecorder := httptest.NewRecorder()
	router.ServeHTTP(checkRecorder, checkRequest)
	assertStatus(t, checkRecorder, http.StatusOK)

	var checkResponse struct {
		Status     string `json:"status"`
		Evaluation struct {
			Required          bool     `json:"required"`
			RequestedScopes   []string `json:"requestedScopes"`
			ConsentableScopes []string `json:"consentableScopes"`
			GrantedScopes     []string `json:"grantedScopes"`
			MissingScopes     []string `json:"missingScopes"`
		} `json:"evaluation"`
	}
	if err := json.Unmarshal(checkRecorder.Body.Bytes(), &checkResponse); err != nil {
		t.Fatalf("json.Unmarshal(consent check response) error = %v", err)
	}
	if checkResponse.Status != "evaluated" || !checkResponse.Evaluation.Required || len(checkResponse.Evaluation.ConsentableScopes) != 2 || len(checkResponse.Evaluation.GrantedScopes) != 0 || len(checkResponse.Evaluation.MissingScopes) != 2 {
		t.Fatalf("unexpected consent check response: %+v", checkResponse)
	}

	grantRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/grant", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","grantedScopes":["profile","email","email"]}`)))
	grantRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	grantRequest.Header.Set("Content-Type", "application/json")
	grantRecorder := httptest.NewRecorder()
	router.ServeHTTP(grantRecorder, grantRequest)
	assertStatus(t, grantRecorder, http.StatusOK)

	var grantResponse struct {
		Status  string `json:"status"`
		Consent struct {
			ApplicationID string   `json:"applicationId"`
			GrantedScopes []string `json:"grantedScopes"`
			RevokedAt     *string  `json:"revokedAt"`
		} `json:"consent"`
	}
	if err := json.Unmarshal(grantRecorder.Body.Bytes(), &grantResponse); err != nil {
		t.Fatalf("json.Unmarshal(consent grant response) error = %v", err)
	}
	if grantResponse.Status != "granted" || grantResponse.Consent.ApplicationID != "01a00000-0000-7000-8000-0000000000a1" || len(grantResponse.Consent.GrantedScopes) != 2 || grantResponse.Consent.RevokedAt != nil {
		t.Fatalf("unexpected consent grant response: %+v", grantResponse)
	}

	checkedAfterGrantRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/check", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","requestedScopes":["openid","profile","email"]}`)))
	checkedAfterGrantRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	checkedAfterGrantRequest.Header.Set("Content-Type", "application/json")
	checkedAfterGrantRecorder := httptest.NewRecorder()
	router.ServeHTTP(checkedAfterGrantRecorder, checkedAfterGrantRequest)
	assertStatus(t, checkedAfterGrantRecorder, http.StatusOK)

	if err := json.Unmarshal(checkedAfterGrantRecorder.Body.Bytes(), &checkResponse); err != nil {
		t.Fatalf("json.Unmarshal(consent check after grant response) error = %v", err)
	}
	if checkResponse.Evaluation.Required || len(checkResponse.Evaluation.GrantedScopes) != 2 || len(checkResponse.Evaluation.MissingScopes) != 0 {
		t.Fatalf("unexpected consent check after grant response: %+v", checkResponse)
	}

	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/revoke", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","grantedScopes":["email"]}`)))
	revokeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	revokeRequest.Header.Set("Content-Type", "application/json")
	revokeRecorder := httptest.NewRecorder()
	router.ServeHTTP(revokeRecorder, revokeRequest)
	assertStatus(t, revokeRecorder, http.StatusOK)

	var revokeResponse struct {
		Status  string `json:"status"`
		Consent struct {
			GrantedScopes []string `json:"grantedScopes"`
			RevokedAt     *string  `json:"revokedAt"`
		} `json:"consent"`
	}
	if err := json.Unmarshal(revokeRecorder.Body.Bytes(), &revokeResponse); err != nil {
		t.Fatalf("json.Unmarshal(consent revoke response) error = %v", err)
	}
	if revokeResponse.Status != "revoked" || len(revokeResponse.Consent.GrantedScopes) != 1 || revokeResponse.Consent.GrantedScopes[0] != "profile" || revokeResponse.Consent.RevokedAt != nil {
		t.Fatalf("unexpected consent revoke response: %+v", revokeResponse)
	}

	checkAfterRevokeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/check", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","requestedScopes":["profile","email"]}`)))
	checkAfterRevokeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	checkAfterRevokeRequest.Header.Set("Content-Type", "application/json")
	checkAfterRevokeRecorder := httptest.NewRecorder()
	router.ServeHTTP(checkAfterRevokeRecorder, checkAfterRevokeRequest)
	assertStatus(t, checkAfterRevokeRecorder, http.StatusOK)

	if err := json.Unmarshal(checkAfterRevokeRecorder.Body.Bytes(), &checkResponse); err != nil {
		t.Fatalf("json.Unmarshal(consent check after revoke response) error = %v", err)
	}
	if !checkResponse.Evaluation.Required || len(checkResponse.Evaluation.GrantedScopes) != 1 || len(checkResponse.Evaluation.MissingScopes) != 1 || checkResponse.Evaluation.MissingScopes[0] != "email" {
		t.Fatalf("unexpected consent check after revoke response: %+v", checkResponse)
	}

	revokeAllRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/revoke", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","grantedScopes":["profile"]}`)))
	revokeAllRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	revokeAllRequest.Header.Set("Content-Type", "application/json")
	revokeAllRecorder := httptest.NewRecorder()
	router.ServeHTTP(revokeAllRecorder, revokeAllRequest)
	assertStatus(t, revokeAllRecorder, http.StatusOK)

	var revokeAllResponse struct {
		Status  string `json:"status"`
		Consent struct {
			GrantedScopes []string `json:"grantedScopes"`
			RevokedAt     *string  `json:"revokedAt"`
		} `json:"consent"`
	}
	if err := json.Unmarshal(revokeAllRecorder.Body.Bytes(), &revokeAllResponse); err != nil {
		t.Fatalf("json.Unmarshal(consent revoke all response) error = %v", err)
	}
	if revokeAllResponse.Status != "revoked" || len(revokeAllResponse.Consent.GrantedScopes) != 0 || revokeAllResponse.Consent.RevokedAt == nil {
		t.Fatalf("unexpected consent revoke all response: %+v", revokeAllResponse)
	}

	invalidGrantRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/grant", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","grantedScopes":["openid"]}`)))
	invalidGrantRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	invalidGrantRequest.Header.Set("Content-Type", "application/json")
	invalidGrantRecorder := httptest.NewRecorder()
	router.ServeHTTP(invalidGrantRecorder, invalidGrantRequest)
	assertStatus(t, invalidGrantRecorder, http.StatusBadRequest)
}

func TestV1AuthAuthorizeRoutes(t *testing.T) {
	router, _, _, _, _, _, _, _ := newTestV1Router(t)

	consentRequiredRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/authorize", bytes.NewReader([]byte(`{"clientId":"portal-client","redirectUri":"https://portal.example.com/callback","responseType":"code","requestedScopes":["openid","profile"],"state":"state-1","codeChallenge":"pkce-challenge","codeChallengeMethod":"S256"}`)))
	consentRequiredRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	consentRequiredRequest.Header.Set("Content-Type", "application/json")
	consentRequiredRecorder := httptest.NewRecorder()
	router.ServeHTTP(consentRequiredRecorder, consentRequiredRequest)
	assertStatus(t, consentRequiredRecorder, http.StatusOK)

	var consentRequiredResponse struct {
		Status     string `json:"status"`
		Evaluation struct {
			Required      bool     `json:"required"`
			MissingScopes []string `json:"missingScopes"`
		} `json:"evaluation"`
	}
	if err := json.Unmarshal(consentRequiredRecorder.Body.Bytes(), &consentRequiredResponse); err != nil {
		t.Fatalf("json.Unmarshal(authorize consent_required response) error = %v", err)
	}
	if consentRequiredResponse.Status != "consent_required" || !consentRequiredResponse.Evaluation.Required || len(consentRequiredResponse.Evaluation.MissingScopes) != 1 || consentRequiredResponse.Evaluation.MissingScopes[0] != "profile" {
		t.Fatalf("unexpected authorize consent_required response: %+v", consentRequiredResponse)
	}

	grantRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/grant", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","grantedScopes":["profile"]}`)))
	grantRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	grantRequest.Header.Set("Content-Type", "application/json")
	grantRecorder := httptest.NewRecorder()
	router.ServeHTTP(grantRecorder, grantRequest)
	assertStatus(t, grantRecorder, http.StatusOK)

	authorizeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/authorize", bytes.NewReader([]byte(`{"clientId":"portal-client","redirectUri":"https://portal.example.com/callback","responseType":"code","requestedScopes":["openid","profile"],"state":"state-1","codeChallenge":"pkce-challenge","codeChallengeMethod":"S256"}`)))
	authorizeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	authorizeRequest.Header.Set("Content-Type", "application/json")
	authorizeRecorder := httptest.NewRecorder()
	router.ServeHTTP(authorizeRecorder, authorizeRequest)
	assertStatus(t, authorizeRecorder, http.StatusOK)

	var authorizeResponse struct {
		Status string `json:"status"`
		Code   struct {
			Code        string `json:"code"`
			RedirectURI string `json:"redirectUri"`
			RedirectTo  string `json:"redirectTo"`
			State       string `json:"state"`
		} `json:"code"`
	}
	if err := json.Unmarshal(authorizeRecorder.Body.Bytes(), &authorizeResponse); err != nil {
		t.Fatalf("json.Unmarshal(authorize response) error = %v", err)
	}
	if authorizeResponse.Status != "authorized" || authorizeResponse.Code.Code == "" || authorizeResponse.Code.RedirectURI != "https://portal.example.com/callback" || authorizeResponse.Code.State != "state-1" || authorizeResponse.Code.RedirectTo == "" {
		t.Fatalf("unexpected authorize response: %+v", authorizeResponse)
	}

	invalidRedirectRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/authorize", bytes.NewReader([]byte(`{"clientId":"portal-client","redirectUri":"https://portal.example.com/unknown","responseType":"code"}`)))
	invalidRedirectRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	invalidRedirectRequest.Header.Set("Content-Type", "application/json")
	invalidRedirectRecorder := httptest.NewRecorder()
	router.ServeHTTP(invalidRedirectRecorder, invalidRedirectRequest)
	assertStatus(t, invalidRedirectRecorder, http.StatusBadRequest)
}

func TestV1AuthTokenExchangeRoutes(t *testing.T) {
	router, _, _, _, _, _, _, tokenRepo := newTestV1Router(t)
	initialAccessTokenCount := len(tokenRepo.items)
	initialRefreshTokenCount := len(tokenRepo.refreshItems)

	grantRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/grant", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","grantedScopes":["profile"]}`)))
	grantRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	grantRequest.Header.Set("Content-Type", "application/json")
	grantRecorder := httptest.NewRecorder()
	router.ServeHTTP(grantRecorder, grantRequest)
	assertStatus(t, grantRecorder, http.StatusOK)

	verifier := "pkce-verifier-1"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	authorizeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/authorize", bytes.NewReader([]byte(`{"clientId":"portal-client","redirectUri":"https://portal.example.com/callback","responseType":"code","requestedScopes":["openid","profile","offline_access"],"state":"state-2","codeChallenge":"`+challenge+`","codeChallengeMethod":"S256","resource":"https://api.example.com"}`)))
	authorizeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	authorizeRequest.Header.Set("Content-Type", "application/json")
	authorizeRecorder := httptest.NewRecorder()
	router.ServeHTTP(authorizeRecorder, authorizeRequest)
	assertStatus(t, authorizeRecorder, http.StatusOK)

	var authorizeResponse struct {
		Code struct {
			Code string `json:"code"`
		} `json:"code"`
	}
	if err := json.Unmarshal(authorizeRecorder.Body.Bytes(), &authorizeResponse); err != nil {
		t.Fatalf("json.Unmarshal(authorize response) error = %v", err)
	}
	if authorizeResponse.Code.Code == "" {
		t.Fatalf("expected authorization code in authorize response: %s", authorizeRecorder.Body.String())
	}

	// portal-client is a public client (token_endpoint_auth_method=none), so it
	// identifies itself with the body client_id and presents no credentials
	// (RFC 6749 §2.3.1). A Basic header would be a different auth method than
	// the registered one and is rejected.
	form := "grant_type=authorization_code&client_id=portal-client&code=" + authorizeResponse.Code.Code + "&redirect_uri=https%3A%2F%2Fportal.example.com%2Fcallback&code_verifier=" + verifier + "&resource=https%3A%2F%2Fapi.example.com"
	exchangeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", strings.NewReader(form))
	exchangeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	exchangeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	exchangeRecorder := httptest.NewRecorder()
	router.ServeHTTP(exchangeRecorder, exchangeRequest)
	assertStatus(t, exchangeRecorder, http.StatusOK)

	var exchangeResponse struct {
		Status       string `json:"status"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		TokenType    string `json:"tokenType"`
		ExpiresIn    int64  `json:"expiresIn"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(exchangeRecorder.Body.Bytes(), &exchangeResponse); err != nil {
		t.Fatalf("json.Unmarshal(exchange response) error = %v", err)
	}
	if exchangeResponse.Status != "issued" || !strings.HasPrefix(exchangeResponse.AccessToken, "gl_pat_") || !strings.HasPrefix(exchangeResponse.RefreshToken, "gl_prt_") || exchangeResponse.TokenType != "Bearer" || exchangeResponse.ExpiresIn != 3600 || exchangeResponse.Scope != "openid profile offline_access" {
		t.Fatalf("unexpected exchange response: %+v", exchangeResponse)
	}
	if len(tokenRepo.items) != initialAccessTokenCount+1 {
		t.Fatalf("expected exchanged access token to be persisted, access token count = %d, want %d", len(tokenRepo.items), initialAccessTokenCount+1)
	}
	if len(tokenRepo.refreshItems) != initialRefreshTokenCount+1 {
		t.Fatalf("expected exchanged refresh token to be persisted, refresh token count = %d, want %d", len(tokenRepo.refreshItems), initialRefreshTokenCount+1)
	}

	accessTokenDigest := sha256.Sum256([]byte(exchangeResponse.AccessToken))
	accessTokenHash := hex.EncodeToString(accessTokenDigest[:])
	refreshTokenDigest := sha256.Sum256([]byte(exchangeResponse.RefreshToken))
	refreshTokenHash := hex.EncodeToString(refreshTokenDigest[:])
	accessTokenPersisted := false
	for _, item := range tokenRepo.items {
		if item.TokenHash == accessTokenHash {
			accessTokenPersisted = true
			break
		}
	}
	if !accessTokenPersisted {
		t.Fatalf("expected exchanged access token hash to be persisted")
	}
	refreshTokenPersisted := false
	for _, item := range tokenRepo.refreshItems {
		if item.RefreshTokenHash == refreshTokenHash {
			refreshTokenPersisted = true
			break
		}
	}
	if !refreshTokenPersisted {
		t.Fatalf("expected exchanged refresh token hash to be persisted")
	}

	reuseRecorder := httptest.NewRecorder()
	reuseRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", strings.NewReader(form))
	reuseRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	reuseRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(reuseRecorder, reuseRequest)
	assertStatus(t, reuseRecorder, http.StatusBadRequest)
}

func TestV1AuthTokenExchangeRequiresClientSecretForConfidentialClient(t *testing.T) {
	router, _, _, _, _, _, _, _ := newTestV1Router(t)

	grantRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/grant", bytes.NewReader([]byte(`{"applicationId":"01a00000-0000-7000-8000-0000000000a2","grantedScopes":["profile"]}`)))
	grantRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	grantRequest.Header.Set("Content-Type", "application/json")
	grantRecorder := httptest.NewRecorder()
	router.ServeHTTP(grantRecorder, grantRequest)
	assertStatus(t, grantRecorder, http.StatusOK)

	verifier := "confidential-verifier-1"
	verifierSum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierSum[:])
	authorizeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/authorize", bytes.NewReader([]byte(`{"clientId":"confidential-client","redirectUri":"https://confidential.example.com/callback","responseType":"code","requestedScopes":["openid","profile"],"codeChallenge":"`+challenge+`","codeChallengeMethod":"S256"}`)))
	authorizeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	authorizeRequest.Header.Set("Content-Type", "application/json")
	authorizeRecorder := httptest.NewRecorder()
	router.ServeHTTP(authorizeRecorder, authorizeRequest)
	assertStatus(t, authorizeRecorder, http.StatusOK)

	var authorizeResponse struct {
		Code struct {
			Code string `json:"code"`
		} `json:"code"`
	}
	if err := json.Unmarshal(authorizeRecorder.Body.Bytes(), &authorizeResponse); err != nil {
		t.Fatalf("json.Unmarshal(confidential authorize response) error = %v", err)
	}

	missingSecretRecorder := httptest.NewRecorder()
	missingSecretRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", strings.NewReader("grant_type=authorization_code&client_id=confidential-client&code="+authorizeResponse.Code.Code+"&redirect_uri=https%3A%2F%2Fconfidential.example.com%2Fcallback"))
	missingSecretRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	missingSecretRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(missingSecretRecorder, missingSecretRequest)
	assertStatus(t, missingSecretRecorder, http.StatusUnauthorized)

	// confidential-client registers client_secret_basic, so the remaining
	// attempts must use Basic. Mixing Basic with a body client_secret is a
	// different-auth-method violation and is rejected before verification.
	wrongSecretRecorder := httptest.NewRecorder()
	wrongSecretRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", strings.NewReader("grant_type=authorization_code&client_id=confidential-client&code="+authorizeResponse.Code.Code+"&redirect_uri=https%3A%2F%2Fconfidential.example.com%2Fcallback"))
	wrongSecretRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	wrongSecretRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wrongSecretRequest.SetBasicAuth("confidential-client", "wrong-secret")
	router.ServeHTTP(wrongSecretRecorder, wrongSecretRequest)
	assertStatus(t, wrongSecretRecorder, http.StatusUnauthorized)

	successRecorder := httptest.NewRecorder()
	successRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", strings.NewReader("grant_type=authorization_code&code="+authorizeResponse.Code.Code+"&redirect_uri=https%3A%2F%2Fconfidential.example.com%2Fcallback&code_verifier="+verifier))
	successRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	successRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	successRequest.SetBasicAuth("confidential-client", "confidential-secret")
	router.ServeHTTP(successRecorder, successRequest)
	assertStatus(t, successRecorder, http.StatusOK)
}

func TestV1ProvidersRoutes(t *testing.T) {
	router, _, _, _, providerRepo, _, _, _ := newTestV1Router(t)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/providers?page=1&pageSize=10", nil)
	listRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(listRecorder, listRequest)
	assertStatus(t, listRecorder, http.StatusOK)

	createBody := map[string]any{
		"organizationId": "org-1",
		"name":           "github",
		"displayName":    "GitHub",
		"category":       "OAuth",
		"providerType":   "GitHub",
		"clientId":       "github-client-id",
		"clientSecret":   "github-client-secret",
		"providerUrl":    "https://github.com",
	}
	createRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/providers", createBody)
	assertStatus(t, createRecorder, http.StatusCreated)
	if _, ok := providerRepo.items["00000000-0000-0000-0000-000000000401"]; !ok {
		t.Fatalf("created provider not found in fake repository")
	}

	invalidCreateRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/providers", map[string]any{
		"organizationId": "org-1",
		"name":           "github-bad",
		"displayName":    "GitHub",
		"category":       "OAuth",
		"providerType":   "GitHub",
		"providerUrl":    "bad-url",
	})
	// A syntactically invalid providerUrl parsed and bound, so it is reported as
	// 422 with field metadata rather than a malformed-body 400 (手册 4.14.1).
	assertStatus(t, invalidCreateRecorder, http.StatusUnprocessableEntity)

	patchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/providers/provider-1", map[string]any{
		"displayName": "GitHub Updated",
		"revision":    float64(1),
	})
	assertStatus(t, patchRecorder, http.StatusOK)
	if got := providerRepo.items["provider-1"].DisplayName; got != "GitHub Updated" {
		t.Fatalf("provider displayName = %q, want %q", got, "GitHub Updated")
	}

	invalidPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/providers/provider-1", map[string]any{
		"displayName": "   ",
		"revision":    float64(2),
	})
	assertStatus(t, invalidPatchRecorder, http.StatusBadRequest)

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/providers/provider-1", nil)
	getRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(getRecorder, getRequest)
	assertStatus(t, getRecorder, http.StatusOK)

	configRecorder := httptest.NewRecorder()
	configRequest := httptest.NewRequest(http.MethodGet, "/api/v1/providers/provider-1/config", nil)
	configRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(configRecorder, configRequest)
	assertStatus(t, configRecorder, http.StatusOK)

	updateConfigRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/providers/provider-1/config", map[string]any{
		"configType": "oauth",
		"config": map[string]any{
			"authUrl":  "https://github.com/login/oauth/authorize",
			"tokenUrl": "https://github.com/login/oauth/access_token",
		},
	})
	assertStatus(t, updateConfigRecorder, http.StatusOK)
	if got := providerRepo.configs["provider-1"].ConfigType; got != "oauth" {
		t.Fatalf("provider configType = %q, want %q", got, "oauth")
	}

	invalidConfigRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/providers/provider-1/config", map[string]any{
		"configType": "   ",
		"config":     map[string]any{"authUrl": "https://github.com/login/oauth/authorize"},
	})
	assertStatus(t, invalidConfigRecorder, http.StatusBadRequest)

	secretStatusRecorder := httptest.NewRecorder()
	secretStatusRequest := httptest.NewRequest(http.MethodGet, "/api/v1/providers/provider-1/secret-status", nil)
	secretStatusRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(secretStatusRecorder, secretStatusRequest)
	assertStatus(t, secretStatusRecorder, http.StatusOK)

	notFoundRecorder := httptest.NewRecorder()
	notFoundRequest := httptest.NewRequest(http.MethodGet, "/api/v1/providers/missing", nil)
	notFoundRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(notFoundRecorder, notFoundRequest)
	assertStatus(t, notFoundRecorder, http.StatusNotFound)
}

func TestV1SessionsRoutes(t *testing.T) {
	router, _, _, _, _, _, _, _ := newTestV1Router(t)

	currentRecorder := httptest.NewRecorder()
	currentRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/current", nil)
	currentRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(currentRecorder, currentRequest)
	assertStatus(t, currentRecorder, http.StatusOK)

	var currentResponse struct {
		ID         string `json:"id"`
		LastSeenAt string `json:"lastSeenAt"`
	}
	if err := json.Unmarshal(currentRecorder.Body.Bytes(), &currentResponse); err != nil {
		t.Fatalf("json.Unmarshal(current session response) error = %v", err)
	}
	if currentResponse.ID != "session-1" || currentResponse.LastSeenAt != fixedNow().Format(time.RFC3339) {
		t.Fatalf("unexpected current session response: %+v", currentResponse)
	}

	staleCurrentRecorder := httptest.NewRecorder()
	staleCurrentRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/current", nil)
	staleCurrentRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-6"})
	router.ServeHTTP(staleCurrentRecorder, staleCurrentRequest)
	assertStatus(t, staleCurrentRecorder, http.StatusOK)

	var staleCurrentResponse struct {
		ID         string `json:"id"`
		LastSeenAt string `json:"lastSeenAt"`
		UpdatedAt  string `json:"updatedAt"`
	}
	if err := json.Unmarshal(staleCurrentRecorder.Body.Bytes(), &staleCurrentResponse); err != nil {
		t.Fatalf("json.Unmarshal(stale current session response) error = %v", err)
	}
	if staleCurrentResponse.ID != "session-6" || staleCurrentResponse.LastSeenAt != fixedNow().Format(time.RFC3339) || staleCurrentResponse.UpdatedAt != fixedNow().Format(time.RFC3339) {
		t.Fatalf("unexpected stale current session response: %+v", staleCurrentResponse)
	}

	expiredCurrentRecorder := httptest.NewRecorder()
	expiredCurrentRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/current", nil)
	expiredCurrentRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-4"})
	router.ServeHTTP(expiredCurrentRecorder, expiredCurrentRequest)
	assertStatus(t, expiredCurrentRecorder, http.StatusUnauthorized)

	idleExpiredCurrentRecorder := httptest.NewRecorder()
	idleExpiredCurrentRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/current", nil)
	idleExpiredCurrentRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-5"})
	router.ServeHTTP(idleExpiredCurrentRecorder, idleExpiredCurrentRequest)
	assertStatus(t, idleExpiredCurrentRecorder, http.StatusUnauthorized)

	expiredRevokeOthersRecorder := httptest.NewRecorder()
	expiredRevokeOthersRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/current/revoke-others", nil)
	expiredRevokeOthersRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-4"})
	router.ServeHTTP(expiredRevokeOthersRecorder, expiredRevokeOthersRequest)
	assertStatus(t, expiredRevokeOthersRecorder, http.StatusUnauthorized)

	idleExpiredRevokeOthersRecorder := httptest.NewRecorder()
	idleExpiredRevokeOthersRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/current/revoke-others", nil)
	idleExpiredRevokeOthersRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-5"})
	router.ServeHTTP(idleExpiredRevokeOthersRecorder, idleExpiredRevokeOthersRequest)
	assertStatus(t, idleExpiredRevokeOthersRecorder, http.StatusUnauthorized)

	revokeOthersRecorder := httptest.NewRecorder()
	revokeOthersRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/current/revoke-others", nil)
	revokeOthersRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(revokeOthersRecorder, revokeOthersRequest)
	assertStatus(t, revokeOthersRecorder, http.StatusOK)

	var revokeOthersResponse struct {
		Status       string `json:"status"`
		RevokedCount int64  `json:"revokedCount"`
	}
	if err := json.Unmarshal(revokeOthersRecorder.Body.Bytes(), &revokeOthersResponse); err != nil {
		t.Fatalf("json.Unmarshal(revoke other sessions response) error = %v", err)
	}
	if revokeOthersResponse.Status != "revoked" || revokeOthersResponse.RevokedCount != 3 {
		t.Fatalf("unexpected revoke others response: %+v", revokeOthersResponse)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?page=1&pageSize=10&activeOnly=true", nil)
	listRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(listRecorder, listRequest)
	assertStatus(t, listRecorder, http.StatusOK)

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1", nil)
	getRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(getRecorder, getRequest)
	assertStatus(t, getRecorder, http.StatusOK)

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/session-2", nil)
	deleteRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(deleteRecorder, deleteRequest)
	assertStatus(t, deleteRecorder, http.StatusOK)

	notFoundRecorder := httptest.NewRecorder()
	notFoundRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-2", nil)
	notFoundRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(notFoundRecorder, notFoundRequest)
	assertStatus(t, notFoundRecorder, http.StatusNotFound)

	revokeRecorder := httptest.NewRecorder()
	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-1/revoke", nil)
	revokeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(revokeRecorder, revokeRequest)
	assertStatus(t, revokeRecorder, http.StatusOK)

	revokedGetRecorder := httptest.NewRecorder()
	revokedGetRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1", nil)
	revokedGetRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-7"})
	router.ServeHTTP(revokedGetRecorder, revokedGetRequest)
	assertStatus(t, revokedGetRecorder, http.StatusOK)

	var revokedResponse struct {
		LastSeenAt   string  `json:"lastSeenAt"`
		RevokedAt    *string `json:"revokedAt"`
		SessionState string  `json:"sessionState"`
	}
	if err := json.Unmarshal(revokedGetRecorder.Body.Bytes(), &revokedResponse); err != nil {
		t.Fatalf("json.Unmarshal(revoked session response) error = %v", err)
	}
	if revokedResponse.SessionState != "revoked" || revokedResponse.RevokedAt == nil || *revokedResponse.RevokedAt == "" || revokedResponse.LastSeenAt != fixedNow().Format(time.RFC3339) {
		t.Fatalf("unexpected revoked session response: %+v", revokedResponse)
	}
}

func TestV1RolesRoutes(t *testing.T) {
	router, _, _, _, _, roleRepo, _, _ := newTestV1Router(t)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/roles?page=1&pageSize=10", nil)
	listRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(listRecorder, listRequest)
	assertStatus(t, listRecorder, http.StatusOK)

	createRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/roles", map[string]any{
		"organizationId": "org-1",
		"name":           "auditor",
		"displayName":    "Auditor",
		"description":    "Read-only audit role",
		"groupIds":       []string{"group-1"},
		"roleIds":        []string{"role-1"},
		"domains":        []string{"console"},
		"isEnabled":      true,
	})
	assertStatus(t, createRecorder, http.StatusCreated)
	if _, ok := roleRepo.items["00000000-0000-0000-0000-000000000501"]; !ok {
		t.Fatalf("created role not found in fake repository")
	}

	invalidCreateRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/roles", map[string]any{
		"organizationId": "org-1",
		"name":           "auditor-2",
		"displayName":    "Auditor",
		"groupIds":       []string{"group-1", "group-1"},
	})
	assertStatus(t, invalidCreateRecorder, http.StatusUnprocessableEntity)

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/roles/role-1", nil)
	getRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(getRecorder, getRequest)
	assertStatus(t, getRecorder, http.StatusOK)

	patchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/roles/role-1", map[string]any{
		"displayName": "Admin Updated",
		"description": "Updated admin role",
		"domains":     []string{"console", "api"},
		"isEnabled":   false,
		"revision":    float64(1),
	})
	assertStatus(t, patchRecorder, http.StatusOK)
	if got := roleRepo.items["role-1"].DisplayName; got != "Admin Updated" {
		t.Fatalf("role displayName = %q, want %q", got, "Admin Updated")
	}
	if got := roleRepo.items["role-1"].IsEnabled; got != false {
		t.Fatalf("role isEnabled = %v, want false", got)
	}

	invalidPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/roles/role-1", map[string]any{
		"displayName": "   ",
		"revision":    float64(2),
	})
	assertStatus(t, invalidPatchRecorder, http.StatusBadRequest)

	notFoundRecorder := httptest.NewRecorder()
	notFoundRequest := httptest.NewRequest(http.MethodGet, "/api/v1/roles/missing", nil)
	notFoundRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(notFoundRecorder, notFoundRequest)
	assertStatus(t, notFoundRecorder, http.StatusNotFound)

	invalidRecorder := performRawJSONRequest(router, http.MethodPost, "/api/v1/roles", []byte(`{"organizationId":"org-1","name":"x","displayName":"X","unknown":true}`))
	assertStatus(t, invalidRecorder, http.StatusBadRequest)
}

func TestV1PermissionsRoutes(t *testing.T) {
	router, _, _, _, _, _, permissionRepo, _ := newTestV1Router(t)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/permissions?page=1&pageSize=10&state=active", nil)
	listRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(listRecorder, listRequest)
	assertStatus(t, listRecorder, http.StatusOK)

	createRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/permissions", map[string]any{
		"organizationId": "org-1",
		"name":           "portal-read-2",
		"displayName":    "Portal Read 2",
		"description":    "Read portal users",
		"roles":          []string{"role-1"},
		"domains":        []string{"console"},
		"resourceType":   "Application",
		"resources":      []string{"01a00000-0000-7000-8000-0000000000a1"},
		"actions":        []string{"read"},
		"effect":         "allow",
		"isEnabled":      true,
		"submitter":      "user-1",
		"state":          "active",
	})
	assertStatus(t, createRecorder, http.StatusCreated)
	if _, ok := permissionRepo.items["00000000-0000-0000-0000-000000000601"]; !ok {
		t.Fatalf("created permission not found in fake repository")
	}

	invalidCreateRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/permissions", map[string]any{
		"organizationId": "org-1",
		"name":           "portal-read-3",
		"displayName":    "Portal Read 3",
		"resourceType":   "Application",
		"resources":      []string{"01a00000-0000-7000-8000-0000000000a1", "01a00000-0000-7000-8000-0000000000a1"},
	})
	assertStatus(t, invalidCreateRecorder, http.StatusUnprocessableEntity)

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/permissions/permission-1", nil)
	getRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(getRecorder, getRequest)
	assertStatus(t, getRecorder, http.StatusOK)

	patchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/permissions/permission-1", map[string]any{
		"displayName": "Portal Read Updated",
		"description": "Updated description",
		"actions":     []string{"read", "export"},
		"effect":      "deny",
		"state":       "approved",
		"revision":    float64(1),
	})
	assertStatus(t, patchRecorder, http.StatusOK)
	if got := permissionRepo.items["permission-1"].DisplayName; got != "Portal Read Updated" {
		t.Fatalf("permission displayName = %q, want %q", got, "Portal Read Updated")
	}
	if got := permissionRepo.items["permission-1"].Effect; got != "deny" {
		t.Fatalf("permission effect = %q, want %q", got, "deny")
	}

	invalidPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/permissions/permission-1", map[string]any{
		"effect":   "block",
		"revision": float64(2),
	})
	assertStatus(t, invalidPatchRecorder, http.StatusUnprocessableEntity)

	notFoundRecorder := httptest.NewRecorder()
	notFoundRequest := httptest.NewRequest(http.MethodGet, "/api/v1/permissions/missing", nil)
	notFoundRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(notFoundRecorder, notFoundRequest)
	assertStatus(t, notFoundRecorder, http.StatusNotFound)

	invalidRecorder := performRawJSONRequest(router, http.MethodPost, "/api/v1/permissions", []byte(`{"organizationId":"org-1","name":"portal-read","displayName":"Portal Read","resourceType":"Application","unknown":true}`))
	assertStatus(t, invalidRecorder, http.StatusBadRequest)
}

func TestV1OrganizationsRoutes(t *testing.T) {
	router, orgRepo, _, _, _, _, _, _ := newTestV1Router(t)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/organizations?page=1&pageSize=10", nil)
	listRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(listRecorder, listRequest)
	assertStatus(t, listRecorder, http.StatusOK)

	createBody := map[string]any{
		"slug":        "new-org",
		"displayName": "New Org",
		"websiteUrl":  "https://new.example.com",
		"status":      "active",
		"languages":   []string{"en", "zh-CN"},
	}
	createRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/organizations", createBody)
	assertStatus(t, createRecorder, http.StatusCreated)
	createdOrg, ok := orgRepo.items["00000000-0000-0000-0000-000000000101"]
	if !ok {
		t.Fatalf("created organization not found in fake repository")
	}
	if got := createdOrg.Settings.Languages; len(got) != 2 || got[0] != "en" || got[1] != "zh-CN" {
		t.Fatalf("created organization languages = %v, want [en zh-CN]", got)
	}

	invalidCreateRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/organizations", map[string]any{
		"slug":        " ",
		"displayName": "New Org",
		"status":      "active",
	})
	assertStatus(t, invalidCreateRecorder, http.StatusBadRequest)

	invalidCreateURLRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/organizations", map[string]any{
		"slug":        "new-org-bad-url",
		"displayName": "New Org",
		"status":      "active",
		"websiteUrl":  "not-a-url",
	})
	assertStatus(t, invalidCreateURLRecorder, http.StatusUnprocessableEntity)

	patchBody := map[string]any{
		"displayName": "Built-in Updated",
		"revision":    float64(1),
	}
	patchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/organizations/org-1", patchBody)
	assertStatus(t, patchRecorder, http.StatusOK)
	if got := orgRepo.items["org-1"].DisplayName; got != "Built-in Updated" {
		t.Fatalf("organization displayName = %q, want %q", got, "Built-in Updated")
	}

	invalidPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/organizations/org-1", map[string]any{
		"displayName": "   ",
		"revision":    float64(2),
	})
	assertStatus(t, invalidPatchRecorder, http.StatusBadRequest)

	invalidPatchURLRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/organizations/org-1", map[string]any{
		"websiteUrl": "bad-url",
		"revision":   float64(2),
	})
	assertStatus(t, invalidPatchURLRecorder, http.StatusUnprocessableEntity)

	settingsBody := map[string]any{
		"languages":            []string{"en", "zh-CN"},
		"defaultApplicationId": "01a00000-0000-7000-8000-0000000000a2",
		"accountItems": []map[string]any{{
			"name":    "Email",
			"visible": true,
			"tab":     "basic",
		}},
		"featureFlags": map[string]any{"iam": true},
		"userTypes":    []string{"employee"},
	}
	settingsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/organizations/org-1/settings", settingsBody)
	assertStatus(t, settingsRecorder, http.StatusOK)
	if got := orgRepo.items["org-1"].Settings.DefaultApplicationID; got != "01a00000-0000-7000-8000-0000000000a2" {
		t.Fatalf("organization defaultApplicationId = %q, want %q", got, "01a00000-0000-7000-8000-0000000000a2")
	}

	invalidSettingsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/organizations/org-1/settings", map[string]any{
		"languages":    []string{"en", "en"},
		"accountItems": []map[string]any{{"name": "Email", "visible": true}, {"name": "Email", "visible": true}},
	})
	assertStatus(t, invalidSettingsRecorder, http.StatusUnprocessableEntity)

	// Bare "zh" is ambiguous; the embedded translation packages only back "zh-CN"/"zh-TW".
	unsupportedLanguageRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/organizations/org-1/settings", map[string]any{
		"languages": []string{"zh"},
	})
	assertStatus(t, unsupportedLanguageRecorder, http.StatusUnprocessableEntity)
	var unsupportedLanguageResponse struct {
		Error struct {
			Meta struct {
				Allowed []string `json:"allowed"`
			} `json:"meta"`
		} `json:"error"`
	}
	if err := json.Unmarshal(unsupportedLanguageRecorder.Body.Bytes(), &unsupportedLanguageResponse); err != nil {
		t.Fatalf("json.Unmarshal(unsupported language response) error = %v", err)
	}
	if len(unsupportedLanguageResponse.Error.Meta.Allowed) == 0 {
		t.Fatalf("meta.allowed missing, body = %s", unsupportedLanguageRecorder.Body.String())
	}

	themeBody := map[string]any{
		"themeName": "enterprise",
		"tokens":    map[string]any{"colorPrimary": "#0057b8"},
		"customCss": ".header { font-weight: 700; }",
		"assets":    map[string]any{"logo": "https://cdn.example.com/logo-enterprise.png"},
	}
	themeRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/organizations/org-1/theme", themeBody)
	assertStatus(t, themeRecorder, http.StatusOK)
	if got := orgRepo.items["org-1"].Theme.ThemeName; got != "enterprise" {
		t.Fatalf("organization themeName = %q, want %q", got, "enterprise")
	}

	invalidThemeRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/organizations/org-1/theme", map[string]any{
		"themeName": "   ",
	})
	assertStatus(t, invalidThemeRecorder, http.StatusBadRequest)

	securityBody := map[string]any{
		"passwordPolicy": map[string]any{"minLength": float64(14)},
		"mfaPolicy":      map[string]any{"required": true},
		"sessionPolicy":  map[string]any{"idleTimeoutMinutes": float64(30)},
	}
	securityRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/organizations/org-1/security-policy", securityBody)
	assertStatus(t, securityRecorder, http.StatusOK)
	if got := orgRepo.items["org-1"].Security.PasswordPolicy["minLength"]; got != float64(14) {
		t.Fatalf("organization password minLength = %#v, want %#v", got, float64(14))
	}

	invalidSecurityRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/organizations/org-1/security-policy", map[string]any{
		"passwordPolicy": map[string]any{"minLength": float64(0)},
		"sessionPolicy":  map[string]any{"idleTimeoutMinutes": float64(-1)},
	})
	assertStatus(t, invalidSecurityRecorder, http.StatusUnprocessableEntity)

	notFoundRecorder := httptest.NewRecorder()
	notFoundRequest := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/missing", nil)
	notFoundRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(notFoundRecorder, notFoundRequest)
	assertStatus(t, notFoundRecorder, http.StatusNotFound)

	notFoundBySlugRecorder := httptest.NewRecorder()
	notFoundBySlugRequest := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/by-slug/missing", nil)
	notFoundBySlugRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(notFoundBySlugRecorder, notFoundBySlugRequest)
	assertStatus(t, notFoundBySlugRecorder, http.StatusNotFound)

	invalidRecorder := performRawJSONRequest(router, http.MethodPost, "/api/v1/organizations", []byte(`{"slug":"x","displayName":"X","status":"active","unknown":true}`))
	assertStatus(t, invalidRecorder, http.StatusBadRequest)
}

func TestV1UsersRoutes(t *testing.T) {
	router, _, userRepo, _, _, _, _, tokenRepo := newTestV1Router(t)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users?page=1&pageSize=10", nil)
	listRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(listRecorder, listRequest)
	assertStatus(t, listRecorder, http.StatusOK)

	createBody := map[string]any{
		"organizationId": "built-in",
		"username":       "alice",
		"userType":       "standard",
		"status":         "active",
		"isAdmin":        false,
		"profile": map[string]any{
			"displayName": "Alice",
			"email":       "alice@example.com",
		},
		"roleBindingIds": []string{"role-1"},
		"groupIds":       []string{"group-1"},
	}
	// The body names the built-in organization by slug. Only a platform-admin
	// scope keeps the requested tenant (TrustedOrganizationID); the ordinary
	// session scope is pinned to session.OrganizationID. The bootstrap test
	// router does not mount TenantFilterMiddleware, so the platform-admin mode
	// production derives from session.PrincipalType must be injected here. This
	// exercises the legitimate cross-tenant target instead of weakening the
	// trusted-tenant convergence.
	createRecorder := performJSONRequestAsPlatformAdmin(t, router, http.MethodPost, "/api/v1/users", createBody)
	assertStatus(t, createRecorder, http.StatusCreated)
	createdUser := userRepo.items["00000000-0000-0000-0000-000000000201"]
	if createdUser == nil {
		t.Fatalf("created user not found in fake repository")
	}
	if createdUser.OrganizationID != "org-1" {
		t.Fatalf("created user organization = %q, want the built-in slug resolved to %q", createdUser.OrganizationID, "org-1")
	}
	if createdUser.Profile.Email != "alice@example.com" {
		t.Fatalf("created user email = %q, want %q", createdUser.Profile.Email, "alice@example.com")
	}

	invalidCreateRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/users", map[string]any{
		"organizationId": "org-1",
		"username":       "alice-2",
		"userType":       "standard",
		"status":         "active",
		"isAdmin":        false,
		"profile": map[string]any{
			"displayName": "Alice 2",
			"email":       "not-an-email",
		},
		"roleBindingIds": []string{"role-1", "role-1"},
	})
	assertStatus(t, invalidCreateRecorder, http.StatusUnprocessableEntity)

	invalidCreateAvatarRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/users", map[string]any{
		"organizationId": "org-1",
		"username":       "alice-3",
		"userType":       "standard",
		"status":         "active",
		"isAdmin":        false,
		"profile": map[string]any{
			"displayName": "Alice 3",
			"avatarUrl":   "bad-url",
		},
	})
	assertStatus(t, invalidCreateAvatarRecorder, http.StatusUnprocessableEntity)

	userRepo.items["user-1"].PreferredMfaType = "radius"
	userRepo.items["user-1"].RecoveryCodeHashes = []string{mustHashRecoveryCode(t, "recovery-1"), mustHashRecoveryCode(t, "recovery-2")}
	userRepo.items["user-1"].MfaMethods = []userdomain.UserMfaMethod{{
		ID:          "mfa-radius-1",
		UserID:      "user-1",
		MethodType:  "radius",
		Enabled:     true,
		Verified:    true,
		DisplayName: "RADIUS",
	}}
	userRepo.items["user-1"].MfaRadiusUsername = "jane-radius"
	userRepo.items["user-1"].MfaRadiusProvider = "radius-main"

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1", nil)
	getRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(getRecorder, getRequest)
	assertStatus(t, getRecorder, http.StatusOK)

	var getResponse map[string]any
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &getResponse); err != nil {
		t.Fatalf("json.Unmarshal(user detail response) error = %v", err)
	}
	mfaSetupSummary, ok := getResponse["mfaSetupSummary"].([]any)
	if !ok || len(mfaSetupSummary) == 0 {
		t.Fatalf("expected mfaSetupSummary in user detail response, got %s", getRecorder.Body.String())
	}
	radiusFound := false
	for _, entry := range mfaSetupSummary {
		typed, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if typed["mfaType"] == "radius" {
			radiusFound = true
			if typed["isPreferred"] != true {
				t.Fatalf("expected radius setup summary to be preferred, got %+v", typed)
			}
			if typed["enabled"] != true {
				t.Fatalf("expected radius setup summary to be enabled, got %+v", typed)
			}
			if typed["url"] != "radius-main" {
				t.Fatalf("expected radius setup summary url, got %+v", typed)
			}
		}
	}
	if !radiusFound {
		t.Fatalf("expected radius entry in mfaSetupSummary, got %+v", mfaSetupSummary)
	}
	recoveryCodes, ok := getResponse["recoveryCodes"].([]any)
	if !ok || len(recoveryCodes) != 2 {
		t.Fatalf("expected recoveryCodes in user detail response, got %s", getRecorder.Body.String())
	}

	patchBody := map[string]any{
		"status":      "disabled",
		"isForbidden": true,
		"revision":    float64(1),
	}
	patchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/users/user-1", patchBody)
	assertStatus(t, patchRecorder, http.StatusOK)
	if got := userRepo.items["user-1"].Status; got != "disabled" {
		t.Fatalf("user status = %q, want %q", got, "disabled")
	}
	if !userRepo.items["user-1"].IsForbidden {
		t.Fatalf("user isForbidden = false, want true")
	}

	invalidPatchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/users/user-1", map[string]any{
		"status":   "   ",
		"revision": float64(2),
	})
	// 规范 4.14.1：可解析但语义非法（空白串）→ 422，而非畸形报文的 400。
	assertStatus(t, invalidPatchRecorder, http.StatusUnprocessableEntity)

	profileBody := map[string]any{
		"displayName": "Jane Updated",
		"email":       "jane.updated@example.com",
		"region":      "CN-ZJ",
		"properties":  map[string]any{"department": "engineering"},
	}
	profileRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/profile", profileBody)
	assertStatus(t, profileRecorder, http.StatusOK)
	if got := userRepo.items["user-1"].Profile.Email; got != "jane.updated@example.com" {
		t.Fatalf("user profile email = %q, want %q", got, "jane.updated@example.com")
	}

	invalidProfileRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/profile", map[string]any{
		"email": "bad-email",
	})
	assertStatus(t, invalidProfileRecorder, http.StatusUnprocessableEntity)

	invalidProfileAvatarRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/profile", map[string]any{
		"avatarUrl": "bad-url",
	})
	assertStatus(t, invalidProfileAvatarRecorder, http.StatusUnprocessableEntity)

	roleBindingsBody := map[string]any{
		"items": []map[string]any{{"roleId": "role-admin"}, {"roleId": "role-auditor"}},
	}
	roleBindingsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/role-bindings", roleBindingsBody)
	assertStatus(t, roleBindingsRecorder, http.StatusOK)
	if got := len(userRepo.items["user-1"].RoleBindings); got != 2 {
		t.Fatalf("user role binding count = %d, want %d", got, 2)
	}

	invalidRoleBindingsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/role-bindings", map[string]any{
		"items": []map[string]any{{"roleId": "role-admin"}, {"roleId": "role-admin"}},
	})
	assertStatus(t, invalidRoleBindingsRecorder, http.StatusUnprocessableEntity)

	groupMembershipsBody := map[string]any{
		"items": []map[string]any{{"groupId": "group-finance"}},
	}
	groupMembershipsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/group-memberships", groupMembershipsBody)
	assertStatus(t, groupMembershipsRecorder, http.StatusOK)
	if got := len(userRepo.items["user-1"].GroupMemberships); got != 1 {
		t.Fatalf("user group membership count = %d, want %d", got, 1)
	}

	invalidGroupMembershipsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/group-memberships", map[string]any{
		"items": []map[string]any{{"groupId": "group-finance"}, {"groupId": "group-finance"}},
	})
	assertStatus(t, invalidGroupMembershipsRecorder, http.StatusUnprocessableEntity)

	// A new identity carries no client-supplied id: the use case rejects ids
	// that are not already bound to this user, then generates the id itself.
	identitiesBody := map[string]any{
		"items": []map[string]any{{
			"providerId":      "provider-github",
			"identityType":    "oauth",
			"subject":         "github|12345",
			"providerAccount": "octocat",
			"profileSnapshot": map[string]any{"login": "octocat"},
		}},
	}
	identitiesRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/identities", identitiesBody)
	assertStatus(t, identitiesRecorder, http.StatusOK)
	if got := len(userRepo.items["user-1"].Identities); got != 1 {
		t.Fatalf("user identity count = %d, want %d", got, 1)
	}
	if got := userRepo.items["user-1"].Identities[0].ProviderID; got != "provider-github" {
		t.Fatalf("user identity providerId = %q, want %q", got, "provider-github")
	}

	invalidIdentityRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/identities", map[string]any{
		"items": []map[string]any{{
			"identityType": "oauth",
			"subject":      "github|missing-provider",
		}},
	})
	assertStatus(t, invalidIdentityRecorder, http.StatusBadRequest)

	userRepo.items["user-1"].MfaMethods = append(userRepo.items["user-1"].MfaMethods, userdomain.UserMfaMethod{
		ID:          "mfa-totp-1",
		UserID:      "user-1",
		MethodType:  "totp",
		Enabled:     true,
		Verified:    true,
		DisplayName: "Primary TOTP",
	})
	mfaMethodsBody := map[string]any{
		"revision": float64(userRepo.items["user-1"].Revision),
		"items": []map[string]any{{
			"id":          "mfa-totp-1",
			"methodType":  "totp",
			"enabled":     true,
			"displayName": "Primary TOTP",
		}},
	}
	mfaMethodsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/mfa-methods", mfaMethodsBody)
	assertStatus(t, mfaMethodsRecorder, http.StatusOK)
	if got := len(userRepo.items["user-1"].MfaMethods); got != 1 {
		t.Fatalf("user mfa method count = %d, want %d", got, 1)
	}
	if got := userRepo.items["user-1"].MfaMethods[0].MethodType; got != "totp" {
		t.Fatalf("user mfa method type = %q, want %q", got, "totp")
	}

	preferredMfaRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/preferred-mfa", map[string]any{
		"mfaType":  "totp",
		"revision": float64(userRepo.items["user-1"].Revision),
	})
	assertStatus(t, preferredMfaRecorder, http.StatusOK)
	if got := userRepo.items["user-1"].PreferredMfaType; got != "totp" {
		t.Fatalf("user preferredMfaType = %q, want %q", got, "totp")
	}

	disableAllMfaRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/mfa-methods", map[string]any{
		"revision": float64(userRepo.items["user-1"].Revision),
		"items": []map[string]any{{
			"id":         userRepo.items["user-1"].MfaMethods[0].ID,
			"methodType": "totp",
			"enabled":    false,
		}},
	})
	assertStatus(t, disableAllMfaRecorder, http.StatusOK)
	if got := userRepo.items["user-1"].PreferredMfaType; got != "" {
		t.Fatalf("user preferredMfaType after disabling MFA = %q, want empty", got)
	}

	invalidPreferredMfaRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/preferred-mfa", map[string]any{
		"mfaType":  "email",
		"revision": float64(userRepo.items["user-1"].Revision),
	})
	assertStatus(t, invalidPreferredMfaRecorder, http.StatusUnprocessableEntity)

	invalidMFARecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/mfa-methods", map[string]any{
		"revision": float64(userRepo.items["user-1"].Revision),
		"items": []map[string]any{
			{"methodType": "totp", "enabled": false},
			{"methodType": "totp", "enabled": false},
		},
	})
	assertStatus(t, invalidMFARecorder, http.StatusUnprocessableEntity)

	credentialsBody := map[string]any{
		"rotateAccessSecret": true,
		"needUpdatePassword": true,
	}
	credentialsRecorder := performJSONRequest(t, router, http.MethodPut, "/api/v1/users/user-1/credentials", credentialsBody)
	assertStatus(t, credentialsRecorder, http.StatusOK)
	// PUT 会级联撤销当前会话，因此口令端点改用平台管理员租户 scope 发送。
	// 目标必须是与操作者不同的用户：session-1 解析为 user-1，管理员重置自己
	// 的口令会被应用层拒绝（422），因此目标改用本测试创建的 alice 用户。
	setPasswordRecorder := performJSONRequestAsPlatformAdmin(t, router, http.MethodPost, "/api/v1/users/00000000-0000-0000-0000-000000000201/set-password?organizationId=org-1", map[string]any{
		"password": "NewPassword!2026",
	})
	assertStatus(t, setPasswordRecorder, http.StatusOK)
	// 自改密保护是故意的生产行为：操作者（user-1）对自己重置口令 → 422。
	selfResetRecorder := performJSONRequestAsPlatformAdmin(t, router, http.MethodPost, "/api/v1/users/user-1/set-password?organizationId=org-1", map[string]any{
		"password": "NewPassword!2026",
	})
	assertStatus(t, selfResetRecorder, http.StatusUnprocessableEntity)
	var credentialsResponse map[string]any
	if err := json.Unmarshal(credentialsRecorder.Body.Bytes(), &credentialsResponse); err != nil {
		t.Fatalf("json.Unmarshal(credentials response) error = %v", err)
	}
	if credentialsResponse["status"] != "updated" {
		t.Fatalf("credentials status = %#v, want %#v", credentialsResponse["status"], "updated")
	}
	if credentialsResponse["accessKey"] == "" || credentialsResponse["accessSecret"] == "" {
		t.Fatalf("credentials response missing rotated secret: %s", credentialsRecorder.Body.String())
	}
	if userRepo.items["user-1"].Credentials.PasswordHash == "" {
		t.Fatalf("user credentials password hash not updated")
	}
	if !userRepo.items["user-1"].Credentials.NeedUpdatePassword {
		t.Fatalf("user credentials needUpdatePassword = false, want true")
	}

	invalidCredentialsRecorder := performJSONRequestWithSession(t, router, http.MethodPut, "/api/v1/users/user-1/credentials", map[string]any{}, "session-7")
	assertStatus(t, invalidCredentialsRecorder, http.StatusBadRequest)

	weakPasswordRecorder := performJSONRequestWithSession(t, router, http.MethodPut, "/api/v1/users/user-1/credentials", map[string]any{
		"password": "short123",
	}, "session-7")
	assertStatus(t, weakPasswordRecorder, http.StatusUnprocessableEntity)

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/users/user-1", nil)
	deleteRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-7"})
	router.ServeHTTP(deleteRecorder, deleteRequest)
	assertStatus(t, deleteRecorder, http.StatusOK)
	if userRepo.items["user-1"].DeletedAt == nil {
		t.Fatalf("expected user deletedAt to be set after delete")
	}
	if tokenRepo.items["token-1"].RevokedAt == nil {
		t.Fatalf("expected user tokens to be revoked after delete")
	}

	deletedGetRecorder := httptest.NewRecorder()
	deletedGetRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1", nil)
	deletedGetRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-7"})
	router.ServeHTTP(deletedGetRecorder, deletedGetRequest)
	assertStatus(t, deletedGetRecorder, http.StatusNotFound)

	notFoundRecorder := httptest.NewRecorder()
	notFoundRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users/missing", nil)
	notFoundRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-7"})
	router.ServeHTTP(notFoundRecorder, notFoundRequest)
	assertStatus(t, notFoundRecorder, http.StatusNotFound)

	invalidRecorder := performRawJSONRequestWithSession(router, http.MethodPost, "/api/v1/users", []byte(`{"organizationId":"org-1","username":"bob","userType":"standard","status":"active","isAdmin":false,"profile":{},"unknown":true}`), "session-7")
	assertStatus(t, invalidRecorder, http.StatusBadRequest)

	openapiRecorder := httptest.NewRecorder()
	openapiRequest := httptest.NewRequest(http.MethodGet, "/openapi/openapi.yaml", nil)
	openapiRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(openapiRecorder, openapiRequest)
	assertStatus(t, openapiRecorder, http.StatusOK)

	openapiJSONRecorder := httptest.NewRecorder()
	openapiJSONRequest := httptest.NewRequest(http.MethodGet, "/openapi/openapi.json", nil)
	openapiJSONRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(openapiJSONRecorder, openapiJSONRequest)
	assertStatus(t, openapiJSONRecorder, http.StatusOK)

	openapiETag := openapiRecorder.Header().Get("ETag")
	if openapiETag == "" {
		t.Fatalf("expected ETag on openapi yaml response")
	}

	openapiJSONETag := openapiJSONRecorder.Header().Get("ETag")
	if openapiJSONETag == "" {
		t.Fatalf("expected ETag on openapi json response")
	}

	openapiJSONNotModifiedRecorder := httptest.NewRecorder()
	openapiJSONNotModifiedRequest := httptest.NewRequest(http.MethodGet, "/openapi/openapi.json", nil)
	openapiJSONNotModifiedRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	openapiJSONNotModifiedRequest.Header.Set("If-None-Match", openapiJSONETag)
	router.ServeHTTP(openapiJSONNotModifiedRecorder, openapiJSONNotModifiedRequest)
	assertStatus(t, openapiJSONNotModifiedRecorder, http.StatusNotModified)
}

func TestV1TokensRoutes(t *testing.T) {
	router, _, _, _, _, _, _, tokenRepo := newTestV1Router(t)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tokens?page=1&pageSize=10", nil)
	listRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(listRecorder, listRequest)
	assertStatus(t, listRecorder, http.StatusOK)

	createRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/tokens", map[string]any{
		"organizationId":          "org-1",
		"userId":                  "user-1",
		"applicationId":           "01a00000-0000-7000-8000-0000000000a1",
		"scopes":                  []string{"read", "write"},
		"audience":                []string{"portal"},
		"metadata":                map[string]any{"source": "auth-ui"},
		"expiresInSeconds":        float64(3600),
		"issueRefreshToken":       true,
		"refreshExpiresInSeconds": float64(7200),
	})
	assertStatus(t, createRecorder, http.StatusCreated)
	if _, ok := tokenRepo.items["00000000-0000-0000-0000-000000000701"]; !ok {
		t.Fatalf("created token not found in fake repository")
	}
	if len(tokenRepo.refreshItems) == 0 {
		t.Fatalf("expected create token to issue refresh token when requested")
	}

	revokeManyRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/tokens/revoke", map[string]any{
		"organizationId": "org-1",
	})
	assertStatus(t, revokeManyRecorder, http.StatusOK)
	if tokenRepo.items["token-1"].RevokedAt == nil || tokenRepo.items["token-2"].RevokedAt == nil {
		t.Fatalf("expected organization scoped revoke to revoke active tokens")
	}

	invalidRevokeManyRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/tokens/revoke", map[string]any{})
	assertStatus(t, invalidRevokeManyRecorder, http.StatusBadRequest)

	tokenRepo.items["token-1"].RevokedAt = nil
	tokenRepo.items["token-1"].UpdatedAt = fixedNow()
	tokenRepo.items["token-2"].RevokedAt = nil
	tokenRepo.items["token-2"].UpdatedAt = fixedNow()

	introspectRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/tokens/introspect", map[string]any{
		"token":         "gl_pat_token1",
		"tokenTypeHint": "access_token",
	})
	assertStatus(t, introspectRecorder, http.StatusOK)
	var introspectResponse struct {
		Active    bool     `json:"active"`
		Scope     string   `json:"scope"`
		Username  string   `json:"username"`
		TokenType string   `json:"tokenType"`
		Sub       string   `json:"sub"`
		Aud       []string `json:"aud"`
		Jti       string   `json:"jti"`
	}
	if err := json.Unmarshal(introspectRecorder.Body.Bytes(), &introspectResponse); err != nil {
		t.Fatalf("json.Unmarshal(introspect response) error = %v", err)
	}
	if !introspectResponse.Active || introspectResponse.Scope != "read write" || introspectResponse.Username != "jane" || introspectResponse.TokenType != "access_token" || introspectResponse.Sub != "user-1" || introspectResponse.Jti != "token-1" {
		t.Fatalf("unexpected introspection response: %+v", introspectResponse)
	}
	if len(introspectResponse.Aud) != 1 || introspectResponse.Aud[0] != "portal" {
		t.Fatalf("unexpected introspection audience: %+v", introspectResponse)
	}

	inactiveIntrospectRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/tokens/introspect", map[string]any{
		"token":         "gl_pat_missing",
		"tokenTypeHint": "access_token",
	})
	assertStatus(t, inactiveIntrospectRecorder, http.StatusOK)
	var inactiveIntrospectResponse struct {
		Active bool `json:"active"`
	}
	if err := json.Unmarshal(inactiveIntrospectRecorder.Body.Bytes(), &inactiveIntrospectResponse); err != nil {
		t.Fatalf("json.Unmarshal(inactive introspect response) error = %v", err)
	}
	if inactiveIntrospectResponse.Active {
		t.Fatalf("expected inactive introspection response")
	}

	refreshRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/tokens/refresh", map[string]any{
		"refreshToken": "gl_prt_token1",
	})
	assertStatus(t, refreshRecorder, http.StatusOK)
	var refreshResponse struct {
		Status       string `json:"status"`
		TokenValue   string `json:"tokenValue"`
		RefreshToken string `json:"refreshToken"`
		Token        struct {
			ID string `json:"id"`
		} `json:"token"`
	}
	if err := json.Unmarshal(refreshRecorder.Body.Bytes(), &refreshResponse); err != nil {
		t.Fatalf("json.Unmarshal(refresh response) error = %v", err)
	}
	// Secrets are crypto/rand base64url values (BE-D-19): assert prefix and
	// entropy length instead of the old UUID-derived deterministic values.
	if refreshResponse.Status != "refreshed" || refreshResponse.Token.ID != "00000000-0000-0000-0000-000000000801" ||
		!strings.HasPrefix(refreshResponse.TokenValue, "gl_pat_") || len(refreshResponse.TokenValue) != len("gl_pat_")+43 ||
		!strings.HasPrefix(refreshResponse.RefreshToken, "gl_prt_") || len(refreshResponse.RefreshToken) != len("gl_prt_")+43 {
		t.Fatalf("unexpected refresh response: %+v", refreshResponse)
	}
	if tokenRepo.items["token-1"].RevokedAt == nil {
		t.Fatalf("expected refresh to revoke previous access token")
	}
	if tokenRepo.refreshItems["refresh-1"].RevokedAt == nil {
		t.Fatalf("expected refresh to revoke previous refresh token")
	}

	invalidCreateRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/tokens", map[string]any{
		"organizationId":   "org-1",
		"userId":           "user-1",
		"applicationId":    "01a00000-0000-7000-8000-0000000000a1",
		"scopes":           []string{},
		"expiresInSeconds": float64(0),
	})
	assertStatus(t, invalidCreateRecorder, http.StatusUnprocessableEntity)

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tokens/token-1", nil)
	getRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(getRecorder, getRequest)
	assertStatus(t, getRecorder, http.StatusOK)

	tokenRepo.items["token-1"].RevokedAt = nil
	tokenRepo.items["token-1"].UpdatedAt = fixedNow()

	revokeRecorder := httptest.NewRecorder()
	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tokens/token-1/revoke", nil)
	revokeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(revokeRecorder, revokeRequest)
	assertStatus(t, revokeRecorder, http.StatusOK)
	if tokenRepo.items["token-1"].RevokedAt == nil {
		t.Fatalf("revoked token still has nil revokedAt")
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/tokens/token-2", nil)
	deleteRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(deleteRecorder, deleteRequest)
	assertStatus(t, deleteRecorder, http.StatusOK)
	if _, ok := tokenRepo.items["token-2"]; ok {
		t.Fatalf("deleted token still exists in fake repository")
	}

	notFoundRecorder := httptest.NewRecorder()
	notFoundRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tokens/missing", nil)
	notFoundRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(notFoundRecorder, notFoundRequest)
	assertStatus(t, notFoundRecorder, http.StatusNotFound)
}

func newTestV1RouterWithSessions(t *testing.T) (chi.Router, *fakeOrganizationRepository, *fakeUserRepository, *fakeApplicationRepository, *fakeProviderRepository, *fakeRoleRepository, *fakePermissionRepository, *fakeTokenRepository, *fakeSessionRepository) {
	t.Helper()
	t.Setenv("RATE_LIMIT_AUTH_RATE", "10000")
	t.Setenv("RATE_LIMIT_AUTH_BURST", "10000")
	t.Setenv("RATE_LIMIT_SIGNUP_RATE", "10000")
	t.Setenv("RATE_LIMIT_SIGNUP_BURST", "10000")
	t.Setenv("RATE_LIMIT_TOKEN_RATE", "10000")
	t.Setenv("RATE_LIMIT_TOKEN_BURST", "10000")
	mw.InitRateLimitsFromEnv()
	enforcerRepo := &fakeEnforcerRepository{items: map[string]*enforcerdomain.Enforcer{
		"01a00000-0000-7000-8000-0000000000a1": {
			ID: "01a00000-0000-7000-8000-0000000000a1", OrganizationID: "org-1",
			Name: "portal", DisplayName: "Portal", ModelID: "01a00000-0000-7000-8000-0000000000b1",
		},
	}}
	modelRepo := &fakeModelRepository{items: map[string]*modeldomain.Model{
		"01a00000-0000-7000-8000-0000000000b1": {
			ID: "01a00000-0000-7000-8000-0000000000b1", OrganizationID: "org-1",
			Name: "rbac", DisplayName: "RBAC", ModelText: "[request_definition] r = sub, obj, act",
		},
	}}
	permissionRepo := &fakePermissionRepository{
		items: map[string]*permissiondomain.Permission{
			"permission-1": {
				ID:             "permission-1",
				OrganizationID: "org-1",
				Name:           "portal-read",
				DisplayName:    "Portal Read",
				Description:    "Read portal users",
				Users:          []string{"user-1"},
				Groups:         []string{"group-1"},
				Roles:          []string{"role-1"},
				Domains:        []string{"console"},
				ResourceType:   "Application",
				Resources:      []string{"01a00000-0000-7000-8000-0000000000a1"},
				Actions:        []string{"read"},
				Effect:         "allow",
				IsEnabled:      true,
				Submitter:      "user-1",
				Approver:       "admin",
				State:          "active",
				CreatedAt:      fixedNow(),
				UpdatedAt:      fixedNow(),
				Revision:       1,
			},
		},
	}
	roleRepo := &fakeRoleRepository{
		items: map[string]*roledomain.Role{
			"role-1": {
				ID:             "role-1",
				OrganizationID: "org-1",
				Name:           "admin",
				DisplayName:    "Admin",
				Description:    "Administrator role",
				GroupIDs:       []string{"group-1"},
				RoleIDs:        []string{},
				Domains:        []string{"console"},
				IsEnabled:      true,
				CreatedAt:      fixedNow(),
				UpdatedAt:      fixedNow(),
				Revision:       1,
			},
			// Relation-write routes bind role ids through the trusted tenant, so
			// the fixture must carry the role rows a user write references.
			"role-admin": {
				ID:             "role-admin",
				OrganizationID: "org-1",
				Name:           "admin",
				DisplayName:    "Admin",
				Domains:        []string{"console"},
				IsEnabled:      true,
				CreatedAt:      fixedNow(),
				UpdatedAt:      fixedNow(),
				Revision:       1,
			},
			"role-auditor": {
				ID:             "role-auditor",
				OrganizationID: "org-1",
				Name:           "auditor",
				DisplayName:    "Auditor",
				Domains:        []string{"console"},
				IsEnabled:      true,
				CreatedAt:      fixedNow(),
				UpdatedAt:      fixedNow(),
				Revision:       1,
			},
		},
	}
	providerRepo := &fakeProviderRepository{
		items: map[string]*providerdomain.Provider{
			"provider-1": {
				ID:             "provider-1",
				OrganizationID: "org-1",
				Name:           "github",
				DisplayName:    "GitHub",
				Category:       "OAuth",
				ProviderType:   "GitHub",
				ClientID:       "github-client-id",
				ClientSecret:   "github-client-secret",
				ProviderURL:    "https://github.com",
				CreatedAt:      fixedNow(),
				UpdatedAt:      fixedNow(),
				Revision:       1,
			},
			// provider-2 exists so the provider-bindings PUT exercises a
			// successful batch; the app layer now rejects unknown providerIds.
			"provider-2": {
				ID:             "provider-2",
				OrganizationID: "org-1",
				Name:           "google",
				DisplayName:    "Google",
				Category:       "OAuth",
				ProviderType:   "Google",
				CreatedAt:      fixedNow(),
				UpdatedAt:      fixedNow(),
				Revision:       1,
			},
		},
		configs: map[string]*providerdomain.ProviderConfig{
			"provider-1": {
				ProviderID: "provider-1",
				ConfigType: "oauth",
				Config: map[string]any{
					"authUrl": "https://github.com/login/oauth/authorize",
				},
				UpdatedAt: fixedNow(),
			},
		},
		secretStatuses: map[string][]providerdomain.ProviderSecretStatus{
			"provider-1": {
				{
					ID:            "provider-secret-1",
					ProviderID:    "provider-1",
					SecretType:    "client_secret",
					Status:        "active",
					LastRotatedAt: fixedNow(),
					ExpiresAt:     fixedNow().Add(24 * time.Hour),
					UpdatedAt:     fixedNow(),
					Version:       1,
				},
			},
		},
	}
	applicationRepo := &fakeApplicationRepository{
		items: map[string]*applicationdomain.Application{
			"01a00000-0000-7000-8000-0000000000a1": {
				ID:             "01a00000-0000-7000-8000-0000000000a1",
				OrganizationID: "org-1",
				Slug:           "portal",
				DisplayName:    "Portal",
				Category:       "customer",
				AppType:        "web",
				Description:    "Default portal",
				HomepageURL:    "https://portal.example.com",
				Domain:         "portal.example.com",
				Status:         "active",
				ClientID:       "portal-client",
				RedirectURIs:   []string{"https://portal.example.com/callback"},
				// Direct fixture (not created through the API): keeps refresh-token
				// issuance covered. The console closed set no longer offers it until
				// OPEN-API-63 implements redemption.
				GrantTypes: []string{"authorization_code", "refresh_token"},
				CustomScopes: []applicationdomain.ApplicationCustomScope{{
					Scope:       "profile",
					DisplayName: "Profile",
					Description: "Profile scope",
				}, {
					Scope:       "email",
					DisplayName: "Email",
					Description: "Email scope",
				}},
				CreatedAt:        fixedNow(),
				UpdatedAt:        fixedNow(),
				Revision:         1,
				ProviderBindings: []applicationdomain.ApplicationProviderBinding{{ApplicationID: "01a00000-0000-7000-8000-0000000000a1", ProviderID: "provider-1"}},
			},
			"01a00000-0000-7000-8000-0000000000a2": {
				ID:                      "01a00000-0000-7000-8000-0000000000a2",
				OrganizationID:          "org-1",
				Slug:                    "confidential-portal",
				DisplayName:             "Confidential Portal",
				Category:                "customer",
				AppType:                 "web",
				Description:             "Confidential portal",
				HomepageURL:             "https://confidential.example.com",
				Domain:                  "confidential.example.com",
				Status:                  "active",
				ClientID:                "confidential-client",
				ClientSecretHash:        func() string { h, _ := platformcrypto.HashClientSecret("confidential-secret"); return h }(),
				TokenEndpointAuthMethod: applicationdomain.TokenEndpointAuthMethodClientSecretBasic,
				RedirectURIs:            []string{"https://confidential.example.com/callback"},
				GrantTypes:              []string{"authorization_code"},
				CustomScopes: []applicationdomain.ApplicationCustomScope{{
					Scope:       "profile",
					DisplayName: "Profile",
					Description: "Profile scope",
				}},
				CreatedAt: fixedNow(),
				UpdatedAt: fixedNow(),
				Revision:  1,
			},
		},
	}
	orgRepo := &fakeOrganizationRepository{
		items: map[string]*orgdomain.Organization{
			"org-1": {
				ID:          "org-1",
				Slug:        "built-in",
				DisplayName: "Built-in",
				WebsiteURL:  "https://built-in.example.com",
				LogoURL:     "https://cdn.example.com/logo.png",
				Status:      "active",
				CreatedAt:   fixedNow(),
				UpdatedAt:   fixedNow(),
				Revision:    1,
				Settings: orgdomain.OrganizationSettings{
					OrganizationID:       "org-1",
					Languages:            []string{"en"},
					DefaultApplicationID: "01a00000-0000-7000-8000-0000000000a1",
				},
				Security: orgdomain.OrganizationSecurityPolicy{
					OrganizationID: "org-1",
					SessionPolicy:  map[string]any{"idleTimeoutMinutes": 30, "mfaRememberHours": 24},
				},
			},
		},
	}
	userRepo := &fakeUserRepository{
		items: map[string]*userdomain.User{
			"user-1": {
				ID:             "user-1",
				OrganizationID: "org-1",
				Username:       "jane",
				UserType:       "standard",
				Status:         "active",
				CreatedAt:      fixedNow(),
				UpdatedAt:      fixedNow(),
				Revision:       1,
				Profile: userdomain.UserProfile{
					UserID:      "user-1",
					DisplayName: "Jane",
					Email:       "jane@example.com",
				},
				Credentials: userdomain.UserCredentials{
					UserID:       "user-1",
					HasPassword:  true,
					PasswordHash: mustHashPassword(t, "Password!2026"),
					PasswordAlgo: "argon2id",
				},
			},
		},
	}
	sessionRepo := &fakeSessionRepository{
		items: map[string]*sessiondomain.Session{
			"session-1": {
				ID:               "session-1",
				UserID:           "user-1",
				Username:         "jane",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",

				ApplicationSlug: "portal",
				SessionState:    "active",
				DeviceInfo:      map[string]any{"platform": "macOS"},
				IPAddress:       "127.0.0.1",
				UserAgent:       "test-agent",
				ExpiresAt:       fixedNow().Add(14 * 24 * time.Hour),
				CreatedAt:       fixedNow().Add(-1 * time.Hour),
				LastSeenAt:      fixedNow(),
				UpdatedAt:       fixedNow(),
				Revision:        1,
			},
			"session-2": {
				ID:               "session-2",
				UserID:           "user-1",
				Username:         "jane",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",

				ApplicationSlug: "portal",
				SessionState:    "revoked",
				DeviceInfo:      map[string]any{"platform": "iOS"},
				IPAddress:       "10.0.0.2",
				UserAgent:       "mobile-agent",
				ExpiresAt:       fixedNow().Add(-1 * time.Hour),
				RevokedAt:       timePtr(fixedNow().Add(-30 * time.Minute)),
				CreatedAt:       fixedNow().Add(-2 * time.Hour),
				LastSeenAt:      fixedNow().Add(-45 * time.Minute),
				UpdatedAt:       fixedNow().Add(-30 * time.Minute),
				Revision:        2,
			},
			"session-3": {
				ID:               "session-3",
				UserID:           "user-1",
				Username:         "jane",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",

				ApplicationSlug: "portal",
				SessionState:    "active",
				DeviceInfo:      map[string]any{"platform": "Windows"},
				IPAddress:       "10.0.0.3",
				UserAgent:       "desktop-agent",
				ExpiresAt:       fixedNow().Add(20 * 24 * time.Hour),
				CreatedAt:       fixedNow().Add(-90 * time.Minute),
				LastSeenAt:      fixedNow(),
				UpdatedAt:       fixedNow(),
				Revision:        1,
			},
			"session-4": {
				ID:               "session-4",
				UserID:           "user-1",
				Username:         "jane",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",

				ApplicationSlug: "portal",
				SessionState:    "active",
				DeviceInfo:      map[string]any{"platform": "Android"},
				IPAddress:       "10.0.0.4",
				UserAgent:       "expired-agent",
				ExpiresAt:       fixedNow().Add(-5 * time.Minute),
				CreatedAt:       fixedNow().Add(-3 * time.Hour),
				LastSeenAt:      fixedNow().Add(-10 * time.Minute),
				UpdatedAt:       fixedNow().Add(-10 * time.Minute),
				Revision:        1,
			},
			"session-5": {
				ID:               "session-5",
				UserID:           "user-1",
				Username:         "jane",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",

				ApplicationSlug: "portal",
				SessionState:    "active",
				DeviceInfo:      map[string]any{"platform": "Linux"},
				IPAddress:       "10.0.0.5",
				UserAgent:       "idle-agent",
				ExpiresAt:       fixedNow().Add(24 * time.Hour),
				CreatedAt:       fixedNow().Add(-4 * time.Hour),
				LastSeenAt:      fixedNow().Add(-2 * time.Hour),
				UpdatedAt:       fixedNow().Add(-2 * time.Hour),
				Revision:        1,
			},
			"session-6": {
				ID:               "session-6",
				UserID:           "user-1",
				Username:         "jane",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",

				ApplicationSlug: "portal",
				SessionState:    "active",
				DeviceInfo:      map[string]any{"platform": "ChromeOS"},
				IPAddress:       "10.0.0.6",
				UserAgent:       "stale-agent",
				ExpiresAt:       fixedNow().Add(24 * time.Hour),
				CreatedAt:       fixedNow().Add(-4 * time.Hour),
				LastSeenAt:      fixedNow().Add(-10 * time.Minute),
				UpdatedAt:       fixedNow().Add(-10 * time.Minute),
				Revision:        1,
			},
			// Stable caller for management requests that outlive the target user's
			// sessions (credential rotation and user deletion revoke user-1).
			"session-7": {
				ID:               "session-7",
				UserID:           "user-2",
				Username:         "operator",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",
				ApplicationSlug:  "portal",
				SessionState:     "active",
				DeviceInfo:       map[string]any{"platform": "macOS"},
				IPAddress:        "127.0.0.1",
				UserAgent:        "test-agent",
				ExpiresAt:        fixedNow().Add(14 * 24 * time.Hour),
				CreatedAt:        fixedNow().Add(-30 * time.Minute),
				LastSeenAt:       fixedNow(),
				UpdatedAt:        fixedNow(),
				Revision:         1,
			},
		},
	}
	tokenRepo := &fakeTokenRepository{
		items: map[string]*tokendomain.Token{
			"token-1": {
				ID:               "token-1",
				UserID:           "user-1",
				Username:         "jane",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",

				ApplicationSlug: "portal",
				TokenPrefix:     "gl_tok_abc",
				TokenHash:       hashTokenValueForTest("gl_pat_token1"),
				Scopes:          []string{"read", "write"},
				Audience:        []string{"portal"},
				Metadata:        map[string]any{"source": "legacy-bridge"},
				ExpiresAt:       fixedNow().Add(2 * time.Hour),
				CreatedAt:       fixedNow().Add(-1 * time.Hour),
				UpdatedAt:       fixedNow(),
				Revision:        1,
			},
			"token-2": {
				ID:               "token-2",
				UserID:           "user-1",
				Username:         "jane",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",

				ApplicationSlug: "portal",
				TokenPrefix:     "gl_tok_xyz",
				TokenHash:       hashTokenValueForTest("gl_pat_token2"),
				Scopes:          []string{"read"},
				Audience:        []string{"console"},
				Metadata:        map[string]any{"source": "auth"},
				ExpiresAt:       fixedNow().Add(4 * time.Hour),
				CreatedAt:       fixedNow().Add(-90 * time.Minute),
				UpdatedAt:       fixedNow(),
				Revision:        2,
			},
		},
		refreshItems: map[string]*tokendomain.RefreshToken{
			"refresh-1": {
				ID:               "refresh-1",
				AccessTokenID:    "token-1",
				UserID:           "user-1",
				Username:         "jane",
				OrganizationID:   "org-1",
				OrganizationSlug: "built-in",
				ApplicationID:    "01a00000-0000-7000-8000-0000000000a1",

				ApplicationSlug:  "portal",
				RefreshTokenHash: hashTokenValueForTest("gl_prt_token1"),
				Scopes:           []string{"read", "write"},
				Audience:         []string{"portal"},
				ExpiresAt:        fixedNow().Add(24 * time.Hour),
				CreatedAt:        fixedNow().Add(-2 * time.Hour),
				UpdatedAt:        fixedNow(),
				Revision:         1,
			},
		},
	}
	consentRepo := &fakeConsentRepository{items: map[string]*consentdomain.Consent{}}
	challengeRepo := &fakeMfaChallengeRepository{items: map[string]*mfachallengedomain.Challenge{}}
	authorizationCodeRepo := &fakeAuthorizationCodeRepository{items: map[string]*authorizationcodedomain.AuthorizationCode{}}
	groupRepo := &fakeGroupRepository{items: map[string]*groupsdomain.Group{
		"group-1": {
			ID:             "group-1",
			OrganizationID: "org-1",
			Name:           "ops",
			DisplayName:    "Operations",
			Type:           "department",
			IsEnabled:      true,
			CreatedAt:      fixedNow(),
			UpdatedAt:      fixedNow(),
			Revision:       1,
		},
		// Group membership replacement resolves each group id inside the trusted
		// tenant, so the fixture must carry the referenced group rows.
		"group-finance": {
			ID:             "group-finance",
			OrganizationID: "org-1",
			Name:           "finance",
			DisplayName:    "Finance",
			Type:           "department",
			IsEnabled:      true,
			CreatedAt:      fixedNow(),
			UpdatedAt:      fixedNow(),
			Revision:       1,
		},
	}}
	recordRepo := &fakeRecordRepository{items: map[string]*recordsdomain.Record{}}
	verificationRepo := &fakeVerificationRepository{items: map[string]*verificationsdomain.Verification{}}
	lockoutRepo := &fakeLockoutAttemptRepository{items: map[string]*lockoutattempts.Attempt{}}
	uow := &fakeUnitOfWork{applications: applicationRepo, authorizationCodes: authorizationCodeRepo, consents: consentRepo, groups: groupRepo, organizations: orgRepo, permissions: permissionRepo, providers: providerRepo, roles: roleRepo, sessions: sessionRepo, tokens: tokenRepo, mfaChallenges: challengeRepo, users: userRepo, records: recordRepo, verifications: verificationRepo, lockoutAttempts: lockoutRepo, enforcers: enforcerRepo, models: modelRepo}
	applicationService := appapplications.NewService(fixedClock{}, &fixedUUID{values: []string{"00000000-0000-0000-0000-000000000301"}}, uow)
	orgService := apporganizations.NewService(fixedClock{}, &fixedUUID{values: []string{"00000000-0000-0000-0000-000000000101"}}, uow)
	permissionService := apppermissions.NewService(fixedClock{}, &fixedUUID{values: []string{"00000000-0000-0000-0000-000000000601"}}, uow, nil)
	providerService := appproviders.NewService(fixedClock{}, &fixedUUID{values: []string{"00000000-0000-0000-0000-000000000401"}}, uow)
	roleService := approles.NewService(fixedClock{}, &fixedUUID{values: []string{"00000000-0000-0000-0000-000000000501"}}, uow, nil)
	sessionService := appsessions.NewService(fixedClock{}, uow)
	tokenService := apptokens.NewService(fixedClock{}, &fixedUUID{values: []string{"00000000-0000-0000-0000-000000000701", "00000000-0000-0000-0000-000000000702", "00000000-0000-0000-0000-000000000801", "00000000-0000-0000-0000-000000000901"}}, uow)
	userService := appusers.NewService(fixedClock{}, &fixedUUID{values: []string{"00000000-0000-0000-0000-000000000201"}}, uow, nil)
	orderService := apporders.NewService(fixedClock{}, &fixedUUID{values: []string{"00000000-0000-0000-0000-000000000901"}}, uow)
	paymentService := apppayments.NewService(fixedClock{}, &fixedUUID{values: []string{"00000000-0000-0000-0000-000000001001"}}, uow)
	authService := appauth.NewServiceWithDependencies(fixedClock{}, &fixedUUID{values: []string{"mfa-challenge-1", "session-login-1", "mfa-challenge-2", "session-login-2", "auth-code-id-1", "auth-code-value-1", "secret-token-auth-1", "access-token-id-auth-1", "secret-refresh-auth-1", "refresh-token-id-auth-1"}}, uow)
	// The authorization-code exchange signs an OIDC ID token when the granted
	// scopes contain openid, so the test service needs a real issuer exactly like
	// the production runtime does.
	apiTestJWKS := platformjwks.NewManager(2048, nil)
	authService.TokenIssuer = jwtinfra.NewIssuer(apiTestJWKS)
	groupService := appgroups.Service{Clock: fixedClock{}, UUID: &fixedUUID{values: []string{"00000000-0000-0000-0000-000000000001"}}, UOW: uow}
	runtime := &Runtime{
		Services: AppServices{
			AuthService:         authService,
			ApplicationService:  applicationService,
			GroupService:        groupService,
			OrganizationService: orgService,
			PermissionService:   permissionService,
			ProviderService:     providerService,
			RoleService:         roleService,
			SessionService:      sessionService,
			TokenService:        tokenService,
			UserService:         userService,
			OrderService:        orderService,
			PaymentService:      paymentService,
		},
		Handlers: TransportHandlers{
			AuthHandler:         authtransport.NewHandler(&authService, nil),
			ApplicationHandler:  applicationtransport.NewHandler(applicationService, nil),
			GroupHandler:        groupstransport.Handler{Service: groupService},
			OrganizationHandler: organizationtransport.NewHandler(orgService, applicationService),
			ProviderHandler:     providertransport.NewHandler(providerService),
			PermissionHandler:   permissiontransport.NewHandler(permissionService),
			RoleHandler:         roletransport.NewHandler(roleService),
			SessionHandler:      sessiontransport.NewHandler(sessionService),
			TokenHandler:        tokentransport.NewHandler(tokenService),
			UserHandler:         usertransport.NewHandler(userService, orderService, paymentService, sessionService, nil),
		},
		Middleware: MiddlewareConfig{
			ServiceIPLimiter: httpmiddleware.NewServiceIPLimiter(httpmiddleware.DefaultServiceIPLimitConfigs),
			WAFMiddleware:    wafMiddlewareForTest(),
		},
	}
	router := newTestChiRouter(runtime)
	registerSystemRoutes(router)
	return router, orgRepo, userRepo, applicationRepo, providerRepo, roleRepo, permissionRepo, tokenRepo, sessionRepo
}

func newTestV1Router(t *testing.T) (chi.Router, *fakeOrganizationRepository, *fakeUserRepository, *fakeApplicationRepository, *fakeProviderRepository, *fakeRoleRepository, *fakePermissionRepository, *fakeTokenRepository) {
	router, orgRepo, userRepo, applicationRepo, providerRepo, roleRepo, permissionRepo, tokenRepo, _ := newTestV1RouterWithSessions(t)
	return router, orgRepo, userRepo, applicationRepo, providerRepo, roleRepo, permissionRepo, tokenRepo
}

func newTestChiRouter(runtime *Runtime) chi.Router {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(mw.TracingMiddleware)
	router.Use(mw.PrometheusMiddleware)
	router.Use(mw.RateLimit(mw.RateLimiterConfig{Rate: 10000, Burst: 10000, CleanupInterval: 5 * time.Minute, MaxAge: 10 * time.Minute}))
	router.Use(runtime.Middleware.ServiceIPLimiter.Middleware)
	router.Use(httpmiddleware.SecurityHeaders(httpmiddleware.DefaultSecurityHeadersConfig()))
	router.Use(runtime.Middleware.WAFMiddleware.Middleware)
	router.Use(authtransport.SessionContextMiddleware(runtime.Handlers.AuthHandler, runtime.Infra.JWTIssuer))
	// APIAuthorizationMiddleware: fixture CasbinManager is nil; skip enforcement.
	// Production code keeps fail-close — a nil manager always 503s.
	if runtime.Infra.CasbinManager != nil {
		router.Use(authtransport.APIAuthorizationMiddleware(runtime.Infra.CasbinManager, runtime.Infra.RebacEngine, nil))
	}
	router.Use(route.RequestMetadataMiddleware())
	RegisterPublicRoutes(router, runtime)
	protocol := router.Group(nil)
	protocol.Use(route.WithSurface(route.SurfaceProtocol))
	RegisterProtocolRoutes(protocol, runtime)
	management := router.Group(nil)
	management.Use(route.WithSurface(route.SurfaceManagement))
	authtransport.RegisterChiRoutes(management, runtime.Handlers.AuthHandler)
	RegisterManagementRoutes(management, runtime)
	return router
}

func performJSONRequest(t *testing.T, router http.Handler, method string, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return performRawJSONRequest(router, method, target, encoded)
}

func performRawJSONRequest(router http.Handler, method string, target string, body []byte) *httptest.ResponseRecorder {
	return performRawJSONRequestWithSession(router, method, target, body, "session-1")
}

func performRawJSONRequestWithSession(router http.Handler, method string, target string, body []byte, sessionID string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	request.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: sessionID})
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(recorder, request)
	return recorder
}

func performJSONRequestWithSession(t *testing.T, router http.Handler, method string, target string, body any, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return performRawJSONRequestWithSession(router, method, target, encoded, sessionID)
}

// performJSONRequestAsPlatformAdmin sends a management request with the tenant
// scope production gives a platform administrator (TenantFilterMiddleware maps
// session.PrincipalType == "platform_admin" to TenantFilterSuperAdmin). The
// bootstrap test router omits TenantFilterMiddleware, so the super-admin scope
// is injected here; without it TrustedOrganizationID pins writes and reads to
// the session's organization. The fixture keeps non-UUID IDs ("org-1") for
// readability, so a pinned non-UUID target would be misread by the app as an
// unresolved slug - an artifact of the fixture, not of the trusted-tenant rule.
func performJSONRequestAsPlatformAdmin(t *testing.T, router http.Handler, method string, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	request := httptest.NewRequest(method, target, bytes.NewReader(encoded))
	request.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer test-token")
	request = request.WithContext(tenantctx.WithTenantScope(request.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func mustEncodeV1SessionCookie(t *testing.T, payload map[string]string) string {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(encoded)
	mac := hmac.New(sha256.New, testSessionHMACKey())
	mac.Write([]byte(payloadB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payloadB64 + "." + sigB64
}

func testSessionHMACKey() []byte {
	if key := os.Getenv("SESSION_HMAC_KEY"); key != "" {
		return []byte(key)
	}
	return []byte("guliangiam-session-hmac-default-key")
}

func assertStatus(t *testing.T, recorder *httptest.ResponseRecorder, want int) {
	t.Helper()
	if recorder.Code != want {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, want, recorder.Body.String())
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
}

func timePtr(value time.Time) *time.Time {
	return &value
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return fixedNow()
}

type fixedUUID struct {
	values []string
	index  int
}

func (g *fixedUUID) NewString() (string, error) {
	if g.index >= len(g.values) {
		return "fixed-uuid", nil
	}
	value := g.values[g.index]
	g.index++
	return value, nil
}

type fakeUnitOfWork struct {
	applications       *fakeApplicationRepository
	authorizationCodes *fakeAuthorizationCodeRepository
	consents           *fakeConsentRepository
	gateway            *fakeGatewayRepository
	groups             *fakeGroupRepository
	invitations        *fakeInvitationRepository
	organizations      *fakeOrganizationRepository
	permissions        *fakePermissionRepository
	providers          *fakeProviderRepository
	roles              *fakeRoleRepository
	sessions           *fakeSessionRepository
	tokens             *fakeTokenRepository
	mfaChallenges      *fakeMfaChallengeRepository
	users              *fakeUserRepository
	records            *fakeRecordRepository
	verifications      *fakeVerificationRepository
	lockoutAttempts    *fakeLockoutAttemptRepository
	enforcers          *fakeEnforcerRepository
	models             *fakeModelRepository
}

func (u fakeUnitOfWork) IDV() idvdomain.Repository { return nil }
func (u fakeUnitOfWork) IGA() igadomain.Repository { return nil }

func (u fakeUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u fakeUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u fakeUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u *fakeUnitOfWork) Applications() applicationdomain.Repository {
	return u.applications
}

func (u *fakeUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository {
	return u.authorizationCodes
}

func (u *fakeUnitOfWork) Adapters() adapterdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Agents() agentdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Enforcers() enforcerdomain.Repository {
	if u.enforcers == nil {
		return nil
	}
	return u.enforcers
}

func (u *fakeUnitOfWork) Models() modeldomain.Repository {
	if u.models == nil {
		return nil
	}
	return u.models
}

func (u *fakeUnitOfWork) Syncers() syncerdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Webhooks() webhookdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Ldaps() ldapdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Certs() certdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) NotificationProviders() notificationproviderdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Notifications() notificationsdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Forms() formdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Orders() orderdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Payments() paymentdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Plans() plandomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository {
	// 空 map → GetByUserID 返回 NotFound，满足 set-password 的平台账号保护分支。
	return fakePlatformAdminRepo{}
}

// TrustedDevices 提供最小替身：RevokeByUser 恒返回 0，不产生额外副作用。
func (u *fakeUnitOfWork) TrustedDevices() trusteddevicesdomain.Repository {
	return fakeTrustedDeviceRepository{}
}

func (u *fakeUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return nil
}
func (u *fakeUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository { return nil }

func (u *fakeUnitOfWork) Pricings() pricingdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Products() productdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Subscriptions() subscriptiondomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Tickets() ticketdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Resources() resourcedomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Consents() consentdomain.Repository {
	return u.consents
}

func (u *fakeUnitOfWork) Gateway() gatewaydomain.Repository {
	return u.gateway
}

func (u *fakeUnitOfWork) Groups() groupsdomain.Repository {
	return u.groups
}

func (u *fakeUnitOfWork) Invitations() invitationsdomain.Repository {
	return u.invitations
}

func (u *fakeUnitOfWork) MfaChallenges() mfachallengedomain.Repository {
	return u.mfaChallenges
}

func (u *fakeUnitOfWork) Organizations() orgdomain.Repository {
	return u.organizations
}

func (u *fakeUnitOfWork) Permissions() permissiondomain.Repository {
	return u.permissions
}

func (u *fakeUnitOfWork) Providers() providerdomain.Repository {
	return u.providers
}

func (u *fakeUnitOfWork) Roles() roledomain.Repository {
	return u.roles
}

func (u *fakeUnitOfWork) Sessions() sessiondomain.Repository {
	return u.sessions
}

func (u *fakeUnitOfWork) Tokens() tokendomain.Repository {
	return u.tokens
}

func (u *fakeUnitOfWork) Users() userdomain.Repository {
	return u.users
}

func (u *fakeUnitOfWork) Records() recordsdomain.Repository {
	return u.records
}

func (u *fakeUnitOfWork) Coupons() coupondomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository {
	return nil
}

func (u *fakeUnitOfWork) Entries() entrydomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) McpServers() mcpserverdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Verifications() verificationsdomain.Repository {
	return u.verifications
}

// stubWebAuthnCredentials 让 GetUser 的 webauthnCredentialCount 回填可走通；
// 返回空列表（计数 0）。
type stubWebAuthnCredentials struct{ webauthndomain.Repository }

func (stubWebAuthnCredentials) ListByUserID(context.Context, string) ([]*webauthndomain.Credential, error) {
	return nil, nil
}

func (u *fakeUnitOfWork) WebAuthnCredentials() webauthndomain.Repository {
	return stubWebAuthnCredentials{}
}

func (u *fakeUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Rules() ruledomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Keys() keysdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) Transactions() transactiondomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) WebhookEvents() webhookeventdomain.Repository {
	return nil
}

func (u *fakeUnitOfWork) QRCodeSessions() qrcodesessions.Repository {
	return nil
}

func (u *fakeUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository {
	return nil
}

func (u *fakeUnitOfWork) Web3Identities() web3identity.Repository {
	return nil
}

func (u *fakeUnitOfWork) FaceIDSessions() faceidsessions.Repository {
	return nil
}

func (u *fakeUnitOfWork) CASTickets() castickets.Repository {
	return nil
}

func (u *fakeUnitOfWork) KerberosTickets() kerberostickets.Repository {
	return nil
}

func (u *fakeUnitOfWork) LockoutAttempts() lockoutattempts.Repository {
	return u.lockoutAttempts
}

func (u *fakeUnitOfWork) Actions() actionsdomain.Repository { return nil }
func (u fakeUnitOfWork) Outbox() domainoutbox.Repository    { return nil }

func (u *fakeUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u *fakeUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *fakeUnitOfWork) Tenants() tenants.Repository { return fakeTenantPort{} }

// fakeTenantPort 只服务于创建组织路由的接线断言；不参与身份/拓扑语义测试。
type fakeTenantPort struct{}

func (fakeTenantPort) CreateIdentity(context.Context, *tenantidentities.TenantIdentity) error {
	return nil
}

func (fakeTenantPort) GetIdentity(context.Context, string) (*tenantidentities.TenantIdentity, error) {
	return nil, apperrors.NewNotFound("tenant identity not found")
}

func (fakeTenantPort) DecommissionIdentity(context.Context, string, time.Time) error { return nil }

func (fakeTenantPort) CreatePrimaryTopology(context.Context, *tenantinstances.TenantInstance) error {
	return nil
}

func (fakeTenantPort) GetPrimaryTopology(context.Context, string) (*tenantinstances.TenantInstance, error) {
	return nil, apperrors.NewNotFound("tenant primary topology not found")
}

func (fakeTenantPort) DecommissionTopologies(context.Context, string, time.Time) error { return nil }

func (fakeTenantPort) PrimaryRegion(context.Context) (*tenantinstances.Region, error) {
	return &tenantinstances.Region{ID: "region-1", RegionCode: "cn-north", CountryCode: "CN"}, nil
}

type fakeApplicationRepository struct {
	items map[string]*applicationdomain.Application
}

type fakeConsentRepository struct {
	items map[string]*consentdomain.Consent
}

type fakeAuthorizationCodeRepository struct {
	items map[string]*authorizationcodedomain.AuthorizationCode
}

func (r *fakeConsentRepository) GetByUserAndApplication(ctx context.Context, userID string, applicationID string) (*consentdomain.Consent, error) {
	for _, consent := range r.items {
		if consent.UserID == userID && consent.ApplicationID == applicationID {
			return consent, nil
		}
	}
	return nil, apperrors.NewNotFound("consent not found")
}

func (r *fakeConsentRepository) CountActiveByApplication(ctx context.Context, applicationID string) (int64, error) {
	var count int64
	for _, consent := range r.items {
		if consent.ApplicationID == applicationID && consent.RevokedAt == nil {
			count++
		}
	}

	return count, nil
}

func (r *fakeConsentRepository) Upsert(ctx context.Context, consent *consentdomain.Consent) error {
	r.items[consent.ID] = consent
	return nil
}

func (r *fakeConsentRepository) Revoke(ctx context.Context, userID string, applicationID string) error {
	for _, consent := range r.items {
		if consent.UserID == userID && consent.ApplicationID == applicationID {
			now := time.Now()
			consent.RevokedAt = &now
			return nil
		}
	}
	return apperrors.NewNotFound("consent not found")
}

func (r *fakeApplicationRepository) GetByID(ctx context.Context, id string) (*applicationdomain.Application, error) {
	application, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("application not found")
	}
	return application, nil
}

func (r *fakeApplicationRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*applicationdomain.Application, error) {
	application, ok := r.items[id]
	if !ok || application.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("application not found")
	}
	return application, nil
}

func (r *fakeApplicationRepository) GetByClientID(ctx context.Context, clientID string) (*applicationdomain.Application, error) {
	for _, application := range r.items {
		if application.ClientID == clientID {
			return application, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r *fakeApplicationRepository) GetByOrganizationAndSlug(ctx context.Context, organizationID string, slug string) (*applicationdomain.Application, error) {
	for _, application := range r.items {
		if application.OrganizationID == organizationID && application.Slug == slug {
			return application, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r *fakeAuthorizationCodeRepository) GetByCode(ctx context.Context, code string) (*authorizationcodedomain.AuthorizationCode, error) {
	for _, item := range r.items {
		if item.Code == code {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("authorization code not found")
}

func (r *fakeAuthorizationCodeRepository) Create(ctx context.Context, authorizationCode *authorizationcodedomain.AuthorizationCode) error {
	r.items[authorizationCode.ID] = authorizationCode
	return nil
}

func (r *fakeAuthorizationCodeRepository) MarkUsed(ctx context.Context, id string, usedAt time.Time) error {
	item, ok := r.items[id]
	if !ok {
		return apperrors.NewNotFound("authorization code not found")
	}
	// Mirrors the real repository: the single-use guard is a conditional update,
	// so a second redemption is a conflict rather than a silent no-op (手册 19.2
	// 一次性消费).
	if item.UsedAt != nil {
		return apperrors.NewConflict("authorization code already used")
	}
	item.UsedAt = timePtr(usedAt)
	return nil
}

func (r *fakeApplicationRepository) List(ctx context.Context, query applicationdomain.ListQuery) ([]*applicationdomain.Application, int64, error) {
	items := make([]*applicationdomain.Application, 0, len(r.items))
	for _, application := range r.items {
		if query.OrganizationID != "" && application.OrganizationID != query.OrganizationID {
			continue
		}
		if query.Status != "" && application.Status != query.Status {
			continue
		}
		items = append(items, application)
	}
	return items, int64(len(items)), nil
}

func (r *fakeApplicationRepository) Create(ctx context.Context, application *applicationdomain.Application) error {
	r.items[application.ID] = application
	return nil
}

func (r *fakeApplicationRepository) Delete(ctx context.Context, id, organizationID string) error {
	application, ok := r.items[id]
	if !ok || application.OrganizationID != organizationID {
		return apperrors.NewNotFound("application not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeApplicationRepository) Update(ctx context.Context, application *applicationdomain.Application) error {
	if _, ok := r.items[application.ID]; !ok {
		return apperrors.NewNotFound("application not found")
	}
	r.items[application.ID] = application
	return nil
}

func (r *fakeApplicationRepository) ReplaceProviderBindings(ctx context.Context, applicationID string, bindings []applicationdomain.ApplicationProviderBinding) error {
	application, ok := r.items[applicationID]
	if !ok {
		return apperrors.NewNotFound("application not found")
	}
	application.ProviderBindings = bindings
	return nil
}

func (r *fakeApplicationRepository) HasDeletedWithOrgAndSlug(_ context.Context, _ string, _ string) (bool, error) {
	return false, nil
}

func (r *fakeApplicationRepository) FindHostConflicts(_ context.Context, _, _ string, _ []string) ([]applicationdomain.HostConflict, error) {
	return nil, nil
}

func (r *fakeApplicationRepository) SetRegistrationAccessToken(_ context.Context, _ string, _ string) error {
	return nil
}

func (r *fakeApplicationRepository) GetRegistrationAccessTokenHash(_ context.Context, _ string) (string, error) {
	return "", nil
}

const fakeProviderRegistryConfigType = "_provider_registry_v1"

type fakeProviderRepository struct {
	items           map[string]*providerdomain.Provider
	configs         map[string]*providerdomain.ProviderConfig
	registryConfigs map[string]*providerdomain.ProviderConfig
	secretStatuses  map[string][]providerdomain.ProviderSecretStatus
}

type fakePermissionRepository struct {
	items map[string]*permissiondomain.Permission
}

type fakeSessionRepository struct {
	items map[string]*sessiondomain.Session
}

type fakeTokenRepository struct {
	items        map[string]*tokendomain.Token
	refreshItems map[string]*tokendomain.RefreshToken
}

type fakeMfaChallengeRepository struct {
	items map[string]*mfachallengedomain.Challenge
}

func (r *fakeMfaChallengeRepository) GetByID(ctx context.Context, id string) (*mfachallengedomain.Challenge, error) {
	challenge, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("login challenge not found")
	}
	return challenge, nil
}

func (r *fakeMfaChallengeRepository) Create(ctx context.Context, challenge *mfachallengedomain.Challenge) error {
	r.items[challenge.ID] = challenge
	return nil
}

func (r *fakeMfaChallengeRepository) Update(ctx context.Context, challenge *mfachallengedomain.Challenge) error {
	if _, ok := r.items[challenge.ID]; !ok {
		return apperrors.NewNotFound("login challenge not found")
	}
	r.items[challenge.ID] = challenge
	return nil
}

func (r *fakeMfaChallengeRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return apperrors.NewNotFound("login challenge not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeSessionRepository) GetByID(ctx context.Context, id string) (*sessiondomain.Session, error) {
	session, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("session not found")
	}
	return session, nil
}

func (r *fakeSessionRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*sessiondomain.Session, error) {
	session, ok := r.items[id]
	if !ok || session.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("session not found")
	}
	return session, nil
}

func (r *fakeSessionRepository) List(ctx context.Context, query sessiondomain.ListQuery) ([]*sessiondomain.Session, int64, error) {
	items := make([]*sessiondomain.Session, 0, len(r.items))
	for _, session := range r.items {
		if query.OrganizationID != "" && session.OrganizationID != query.OrganizationID {
			continue
		}
		if query.UserID != "" && session.UserID != query.UserID {
			continue
		}
		if query.ApplicationID != "" && session.ApplicationID != query.ApplicationID {
			continue
		}
		if query.State != "" && string(session.SessionState) != query.State {
			continue
		}
		if query.ActiveOnly != nil {
			isActive := session.RevokedAt == nil && session.ExpiresAt.After(fixedNow())
			if isActive != *query.ActiveOnly {
				continue
			}
		}
		items = append(items, session)
	}
	return items, int64(len(items)), nil
}

func (r *fakeSessionRepository) Create(ctx context.Context, session *sessiondomain.Session) error {
	r.items[session.ID] = session
	return nil
}

func (r *fakeSessionRepository) Update(ctx context.Context, session *sessiondomain.Session) error {
	return nil
}

func (r *fakeSessionRepository) Touch(ctx context.Context, id string, touchedAt time.Time) error {
	item, ok := r.items[id]
	if !ok {
		return apperrors.NewNotFound("session not found")
	}
	if item.RevokedAt != nil || !item.ExpiresAt.After(touchedAt) {
		return nil
	}

	item.LastSeenAt = touchedAt
	item.UpdatedAt = touchedAt
	item.Revision++
	return nil
}

func (r *fakeSessionRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	item, ok := r.items[id]
	if !ok {
		return apperrors.NewNotFound("session not found")
	}

	if item.RevokedAt != nil {
		return nil
	}

	item.SessionState = "revoked"
	item.RevokedAt = timePtr(revokedAt)
	item.UpdatedAt = revokedAt
	item.Revision++
	return nil
}

func (r *fakeSessionRepository) RevokeForOrganization(ctx context.Context, id, organizationID string, revokedAt time.Time) error {
	item, ok := r.items[id]
	if !ok || item.OrganizationID != organizationID {
		return apperrors.NewNotFound("session not found")
	}

	if item.RevokedAt != nil {
		return nil
	}

	item.SessionState = "revoked"
	item.RevokedAt = timePtr(revokedAt)
	item.UpdatedAt = revokedAt
	item.Revision++
	return nil
}

func (r *fakeSessionRepository) RevokeByUserExcept(ctx context.Context, userID string, excludeSessionID string, revokedAt time.Time) (int64, error) {
	var count int64
	for _, item := range r.items {
		if item.UserID != userID || item.ID == excludeSessionID {
			continue
		}
		if item.RevokedAt != nil || !item.ExpiresAt.After(revokedAt) {
			continue
		}

		item.SessionState = "revoked"
		item.RevokedAt = timePtr(revokedAt)
		item.UpdatedAt = revokedAt
		item.Revision++
		count++
	}

	return count, nil
}

func (r *fakeSessionRepository) RevokeByUserAndApp(ctx context.Context, userID string, applicationID string, excludeSessionID string, revokedAt time.Time) (int64, error) {
	var count int64
	for _, item := range r.items {
		if item.UserID != userID || item.ApplicationID != applicationID || item.ID == excludeSessionID {
			continue
		}
		if item.RevokedAt != nil || !item.ExpiresAt.After(revokedAt) {
			continue
		}

		item.SessionState = "revoked"
		item.RevokedAt = timePtr(revokedAt)
		item.UpdatedAt = revokedAt
		item.Revision++
		count++
	}

	return count, nil
}

func (r *fakeSessionRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return apperrors.NewNotFound("session not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeSessionRepository) DeleteForOrganization(ctx context.Context, id, organizationID string) error {
	item, ok := r.items[id]
	if !ok || item.OrganizationID != organizationID {
		return apperrors.NewNotFound("session not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeSessionRepository) CleanupExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (r *fakeSessionRepository) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	var count int64
	for _, item := range r.items {
		if item.OrganizationID != organizationID {
			continue
		}
		if item.RevokedAt != nil || !item.ExpiresAt.After(revokedAt) {
			continue
		}
		item.SessionState = "revoked"
		item.RevokedAt = timePtr(revokedAt)
		item.UpdatedAt = revokedAt
		item.Revision++
		count++
	}
	return count, nil
}

func (r *fakeSessionRepository) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	var count int64
	for _, item := range r.items {
		if item.UserID != userID {
			continue
		}
		if item.RevokedAt != nil || !item.ExpiresAt.After(revokedAt) {
			continue
		}
		item.SessionState = "revoked"
		item.RevokedAt = timePtr(revokedAt)
		item.UpdatedAt = revokedAt
		item.Revision++
		count++
	}
	return count, nil
}

func (r *fakeTokenRepository) GetByID(ctx context.Context, id string) (*tokendomain.Token, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("token not found")
	}
	return item, nil
}

func (r *fakeTokenRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*tokendomain.Token, error) {
	item, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("token not found")
	}
	return item, nil
}

func (r *fakeTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*tokendomain.Token, error) {
	for _, item := range r.items {
		if item.TokenHash == tokenHash {
			return item, nil
		}
	}

	return nil, apperrors.NewNotFound("token not found")
}

func (r *fakeTokenRepository) GetByDeviceSecretHash(ctx context.Context, deviceSecretHash string) (*tokendomain.Token, error) {
	for _, item := range r.items {
		if item.DeviceSecretHash == deviceSecretHash {
			return item, nil
		}
	}

	return nil, apperrors.NewNotFound("token not found")
}

func (r *fakeTokenRepository) GetRefreshByTokenHash(ctx context.Context, tokenHash string) (*tokendomain.RefreshToken, error) {
	for _, item := range r.refreshItems {
		if item.RefreshTokenHash == tokenHash {
			return item, nil
		}
	}

	return nil, apperrors.NewNotFound("refresh token not found")
}

func (r *fakeTokenRepository) List(ctx context.Context, query tokendomain.ListQuery) ([]*tokendomain.Token, int64, error) {
	items := make([]*tokendomain.Token, 0, len(r.items))
	for _, item := range r.items {
		if query.OrganizationID != "" && item.OrganizationID != query.OrganizationID {
			continue
		}
		if query.UserID != "" && item.UserID != query.UserID {
			continue
		}
		if query.ApplicationID != "" && item.ApplicationID != query.ApplicationID {
			continue
		}
		if query.ActiveOnly != nil {
			isActive := item.RevokedAt == nil && item.ExpiresAt.After(fixedNow())
			if isActive != *query.ActiveOnly {
				continue
			}
		}
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (r *fakeTokenRepository) Create(ctx context.Context, token *tokendomain.Token) error {
	r.items[token.ID] = token
	return nil
}

func (r *fakeTokenRepository) UpdateDeviceSecret(ctx context.Context, tokenID, deviceSecretHash string, expiresIn int) error {
	return nil
}
func (r *fakeTokenRepository) CreateRefresh(ctx context.Context, token *tokendomain.RefreshToken) error {
	r.refreshItems[token.ID] = token
	return nil
}

func (r *fakeTokenRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	item, ok := r.items[id]
	if !ok {
		return apperrors.NewNotFound("token not found")
	}
	if item.RevokedAt != nil {
		return nil
	}
	item.RevokedAt = timePtr(revokedAt)
	item.UpdatedAt = revokedAt
	item.Revision++
	return nil
}

func (r *fakeTokenRepository) RevokeRefresh(ctx context.Context, id string, revokedAt time.Time) error {
	item, ok := r.refreshItems[id]
	if !ok {
		return apperrors.NewNotFound("refresh token not found")
	}
	if item.RevokedAt != nil {
		return nil
	}
	item.RevokedAt = timePtr(revokedAt)
	item.UpdatedAt = revokedAt
	item.Revision++
	return nil
}

func (r *fakeTokenRepository) RevokeByQuery(ctx context.Context, query tokendomain.RevokeQuery, revokedAt time.Time) (int64, error) {
	var count int64
	for _, item := range r.items {
		if query.OrganizationID != "" && item.OrganizationID != query.OrganizationID {
			continue
		}
		if query.UserID != "" && item.UserID != query.UserID {
			continue
		}
		if query.ApplicationID != "" && item.ApplicationID != query.ApplicationID {
			continue
		}
		if item.RevokedAt != nil || !item.ExpiresAt.After(revokedAt) {
			continue
		}

		item.RevokedAt = timePtr(revokedAt)
		item.UpdatedAt = revokedAt
		item.Revision++
		count++
	}

	return count, nil
}

func (r *fakeTokenRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return apperrors.NewNotFound("token not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeTokenRepository) RevokeByFamilyID(ctx context.Context, familyID string, revokedAt time.Time) (int64, error) {
	var count int64
	for _, item := range r.refreshItems {
		if item.FamilyID == familyID && item.RevokedAt == nil {
			item.RevokedAt = &revokedAt
			count++
		}
	}
	return count, nil
}

func (r *fakeTokenRepository) RevokeRefreshByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *fakeTokenRepository) RevokeByAuthorizationCodeID(ctx context.Context, authorizationCodeID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	manager := cred.GetCredManager("argon2id")
	if manager == nil {
		t.Fatalf("argon2id credential manager unavailable")
	}
	hash := manager.GetHashedPassword(password, "")
	if hash == "" {
		t.Fatalf("failed to hash password")
	}
	return hash
}

func mustGenerateTotpCode(t *testing.T, secret string, now time.Time) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(secret, now.UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}
	return code
}

func hashTokenValueForTest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (r *fakePermissionRepository) GetByID(ctx context.Context, id string) (*permissiondomain.Permission, error) {
	permission, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("permission not found")
	}
	return permission, nil
}

func (r *fakePermissionRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*permissiondomain.Permission, error) {
	permission, ok := r.items[id]
	if !ok || permission.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("permission not found")
	}
	return permission, nil
}

func (r *fakePermissionRepository) List(ctx context.Context, query permissiondomain.ListQuery) ([]*permissiondomain.Permission, int64, error) {
	items := make([]*permissiondomain.Permission, 0, len(r.items))
	for _, permission := range r.items {
		if query.OrganizationID != "" && permission.OrganizationID != query.OrganizationID {
			continue
		}
		if query.State != "" && permission.State != query.State {
			continue
		}
		if query.ResourceType != "" && permission.ResourceType != query.ResourceType {
			continue
		}
		if query.IsEnabled != nil && permission.IsEnabled != *query.IsEnabled {
			continue
		}
		items = append(items, permission)
	}
	return items, int64(len(items)), nil
}

func (r *fakePermissionRepository) Create(ctx context.Context, permission *permissiondomain.Permission) error {
	r.items[permission.ID] = permission
	return nil
}

func (r *fakePermissionRepository) Update(ctx context.Context, permission *permissiondomain.Permission) error {
	if _, ok := r.items[permission.ID]; !ok {
		return apperrors.NewNotFound("permission not found")
	}
	r.items[permission.ID] = permission
	return nil
}

func (r *fakePermissionRepository) Delete(ctx context.Context, id, organizationID string) error {
	permission, ok := r.items[id]
	if !ok || permission.OrganizationID != organizationID {
		return apperrors.NewNotFound("permission not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeProviderRepository) GetByID(ctx context.Context, id string) (*providerdomain.Provider, error) {
	provider, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("provider not found")
	}
	return provider, nil
}

func (r *fakeProviderRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*providerdomain.Provider, error) {
	provider, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if provider.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("provider not found")
	}
	return provider, nil
}

func (r *fakeProviderRepository) List(ctx context.Context, query providerdomain.ListQuery) ([]*providerdomain.Provider, int64, error) {
	items := make([]*providerdomain.Provider, 0, len(r.items))
	for _, provider := range r.items {
		if query.OrganizationID != "" && provider.OrganizationID != query.OrganizationID {
			continue
		}
		if query.Category != "" && provider.Category != providerdomain.ProviderCategory(query.Category) {
			continue
		}
		if query.ProviderType != "" && provider.ProviderType != providerdomain.ProviderType(query.ProviderType) {
			continue
		}
		items = append(items, provider)
	}
	return items, int64(len(items)), nil
}

func (r *fakeProviderRepository) Create(ctx context.Context, provider *providerdomain.Provider) error {
	r.items[provider.ID] = provider
	return nil
}

func (r *fakeProviderRepository) Update(ctx context.Context, provider *providerdomain.Provider) error {
	if _, ok := r.items[provider.ID]; !ok {
		return apperrors.NewNotFound("provider not found")
	}
	r.items[provider.ID] = provider
	return nil
}

func (r *fakeProviderRepository) GetConfig(ctx context.Context, providerID string) (*providerdomain.ProviderConfig, error) {
	config, ok := r.configs[providerID]
	if !ok {
		return &providerdomain.ProviderConfig{ProviderID: providerID, Config: map[string]any{}}, nil
	}
	return config, nil
}

func (r *fakeProviderRepository) GetConfigByType(ctx context.Context, providerID, configType string) (*providerdomain.ProviderConfig, error) {
	if configType == fakeProviderRegistryConfigType {
		config, ok := r.registryConfigs[providerID]
		if !ok {
			return nil, apperrors.NewNotFound("provider config not found")
		}
		return config, nil
	}
	config, ok := r.configs[providerID]
	if !ok || config.ConfigType != configType {
		return nil, apperrors.NewNotFound("provider config not found")
	}
	return config, nil
}

func (r *fakeProviderRepository) ListConfigsByType(ctx context.Context, providerIDs []string, configType string) ([]providerdomain.ProviderConfig, error) {
	items := make([]providerdomain.ProviderConfig, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		config, err := r.GetConfigByType(ctx, providerID, configType)
		if err != nil {
			continue
		}
		items = append(items, *config)
	}
	return items, nil
}

func (r *fakeProviderRepository) UpdateConfig(ctx context.Context, config *providerdomain.ProviderConfig) error {
	if _, ok := r.items[config.ProviderID]; !ok {
		return apperrors.NewNotFound("provider not found")
	}
	if config.ConfigType == fakeProviderRegistryConfigType {
		if r.registryConfigs == nil {
			r.registryConfigs = map[string]*providerdomain.ProviderConfig{}
		}
		r.registryConfigs[config.ProviderID] = config
		return nil
	}
	r.configs[config.ProviderID] = config
	return nil
}

func (r *fakeProviderRepository) ListSecretStatuses(ctx context.Context, providerID string) ([]providerdomain.ProviderSecretStatus, error) {
	items := r.secretStatuses[providerID]
	result := make([]providerdomain.ProviderSecretStatus, 0, len(items))
	result = append(result, items...)
	return result, nil
}

func (r *fakeProviderRepository) RotateSecretStatus(ctx context.Context, current providerdomain.ProviderSecretStatus, previousStatus string, previousExpiresAt time.Time) error {
	items := r.secretStatuses[current.ProviderID]
	for i := range items {
		if items[i].SecretType == current.SecretType && items[i].Version < current.Version && (items[i].Status == "active" || items[i].Status == "grace") {
			items[i].Status = previousStatus
			items[i].ExpiresAt = previousExpiresAt
			items[i].UpdatedAt = current.UpdatedAt
		}
	}
	r.secretStatuses[current.ProviderID] = append(items, current)
	return nil
}

func (r *fakeProviderRepository) Delete(ctx context.Context, id string) error {
	delete(r.items, id)
	return nil
}

type fakeOrganizationRepository struct {
	items map[string]*orgdomain.Organization
}

type fakeRoleRepository struct {
	items map[string]*roledomain.Role
}

func (r *fakeRoleRepository) GetByID(ctx context.Context, id string) (*roledomain.Role, error) {
	role, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("role not found")
	}
	return role, nil
}

func (r *fakeRoleRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*roledomain.Role, error) {
	role, ok := r.items[id]
	if !ok || role.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("role not found")
	}
	return role, nil
}

func (r *fakeRoleRepository) List(ctx context.Context, query roledomain.ListQuery) ([]*roledomain.Role, int64, error) {
	items := make([]*roledomain.Role, 0, len(r.items))
	for _, role := range r.items {
		if query.OrganizationID != "" && role.OrganizationID != query.OrganizationID {
			continue
		}
		if query.IsEnabled != nil && role.IsEnabled != *query.IsEnabled {
			continue
		}
		items = append(items, role)
	}
	return items, int64(len(items)), nil
}

func (r *fakeRoleRepository) Create(ctx context.Context, role *roledomain.Role) error {
	r.items[role.ID] = role
	return nil
}

func (r *fakeRoleRepository) Update(ctx context.Context, role *roledomain.Role) error {
	if _, ok := r.items[role.ID]; !ok {
		return apperrors.NewNotFound("role not found")
	}
	r.items[role.ID] = role
	return nil
}

func (r *fakeRoleRepository) Delete(ctx context.Context, id, organizationID string) error {
	role, ok := r.items[id]
	if !ok || role.OrganizationID != organizationID {
		return apperrors.NewNotFound("role not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeOrganizationRepository) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	organization, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("organization not found")
	}
	return organization, nil
}

func (r *fakeOrganizationRepository) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	for _, organization := range r.items {
		if organization.Slug == slug {
			return organization, nil
		}
	}
	return nil, apperrors.NewNotFound("organization not found")
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (r *fakeOrganizationRepository) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return r.GetBySlug(ctx, slug)
}

func (r *fakeOrganizationRepository) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	items := make([]*orgdomain.Organization, 0, len(r.items))
	for _, organization := range r.items {
		items = append(items, organization)
	}
	return items, int64(len(items)), nil
}

func (r *fakeOrganizationRepository) Create(ctx context.Context, organization *orgdomain.Organization) error {
	r.items[organization.ID] = organization
	return nil
}

func (r *fakeOrganizationRepository) Update(ctx context.Context, organization *orgdomain.Organization) error {
	r.items[organization.ID] = organization
	return nil
}

func (r *fakeOrganizationRepository) UpdateSettings(ctx context.Context, settings *orgdomain.OrganizationSettings) error {
	organization, err := r.GetByID(ctx, settings.OrganizationID)
	if err != nil {
		return err
	}
	organization.Settings = *settings
	return nil
}

func (r *fakeOrganizationRepository) UpdateTheme(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error {
	organization, err := r.GetByID(ctx, theme.OrganizationID)
	if err != nil {
		return err
	}
	organization.Theme = *theme
	return nil
}

func (r *fakeOrganizationRepository) UpdateSecurityPolicy(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error {
	organization, err := r.GetByID(ctx, policy.OrganizationID)
	if err != nil {
		return err
	}
	organization.Security = *policy
	return nil
}

func (r *fakeOrganizationRepository) Delete(_ context.Context, _ string) error {
	return apperrors.NewUnimplemented("not implemented")
}

func (r *fakeOrganizationRepository) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type fakeUserRepository struct {
	items map[string]*userdomain.User
}

// LockByID 是会话记账用的行级锁；本测试的假仓储没有事务，占位即可。
func (r *fakeUserRepository) LockByID(context.Context, string) error { return nil }

func (r *fakeUserRepository) GetByID(ctx context.Context, id string) (*userdomain.User, error) {
	user, ok := r.items[id]
	if !ok || user.DeletedAt != nil {
		return nil, apperrors.NewNotFound("user not found")
	}
	return user, nil
}

func (r *fakeUserRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*userdomain.User, error) {
	user, ok := r.items[id]
	if !ok || user.DeletedAt != nil || user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return user, nil
}

func (r *fakeUserRepository) GetByUsername(ctx context.Context, organizationID string, username string) (*userdomain.User, error) {
	for _, user := range r.items {
		if user.DeletedAt == nil && user.OrganizationID == organizationID && user.Username == username {
			return user, nil
		}
	}
	return nil, apperrors.NewNotFound("user not found")
}

// GetByLoginIdentifier 只在密码登录入口使用；引导测试委托用户名读路径即可。
func (r *fakeUserRepository) GetByLoginIdentifier(ctx context.Context, organizationID string, identifier string) (*userdomain.User, error) {
	return r.GetByUsername(ctx, organizationID, identifier)
}

func (r *fakeUserRepository) List(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	items := make([]*userdomain.User, 0, len(r.items))
	for _, user := range r.items {
		if user.DeletedAt == nil {
			items = append(items, user)
		}
	}
	return items, int64(len(items)), nil
}

func (r *fakeUserRepository) Create(ctx context.Context, user *userdomain.User) error {
	r.items[user.ID] = user
	return nil
}

func (r *fakeUserRepository) Update(ctx context.Context, user *userdomain.User) error {
	r.items[user.ID] = user
	return nil
}

func (r *fakeUserRepository) UpdateForOrganization(ctx context.Context, user *userdomain.User, organizationID string) error {
	current, ok := r.items[user.ID]
	if !ok || current.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	r.items[user.ID] = user
	return nil
}

func (r *fakeUserRepository) UpdateProfile(ctx context.Context, profile *userdomain.UserProfile) error {
	user, err := r.GetByID(ctx, profile.UserID)
	if err != nil {
		return err
	}
	user.Profile = *profile
	return nil
}

func (r *fakeUserRepository) SetContactVerifiedForOrganization(ctx context.Context, userID, organizationID string, emailVerified, phoneVerified bool) error {
	user, err := r.GetByIDForOrganization(ctx, userID, organizationID)
	if err != nil {
		return err
	}
	user.Profile.EmailVerified = emailVerified
	user.Profile.PhoneVerified = phoneVerified
	return nil
}

func (r *fakeUserRepository) UpdateCollectionsForOrganization(ctx context.Context, userID, organizationID string, collections userdomain.UserCollections) error {
	user, err := r.GetByIDForOrganization(ctx, userID, organizationID)
	if err != nil {
		return err
	}
	user.Addresses = collections.Addresses
	user.ManagedAccounts = collections.ManagedAccounts
	user.FaceIds = collections.FaceIds
	user.Cart = collections.Cart
	user.MfaItems = collections.MfaItems
	user.MfaAccounts = collections.MfaAccounts
	user.ApplicationScopes = collections.ApplicationScopes
	return nil
}

func (r *fakeUserRepository) UpdateCredentials(ctx context.Context, credentials *userdomain.UserCredentials) error {
	user, err := r.GetByID(ctx, credentials.UserID)
	if err != nil {
		return err
	}
	user.Credentials = *credentials
	return nil
}

func (r *fakeUserRepository) UpdatePasswordCredential(ctx context.Context, credentials *userdomain.UserCredentials) error {
	user, err := r.GetByID(ctx, credentials.UserID)
	if err != nil {
		return err
	}
	user.Credentials.HasPassword = credentials.HasPassword
	user.Credentials.PasswordHash = credentials.PasswordHash
	user.Credentials.PasswordAlgo = credentials.PasswordAlgo
	user.Credentials.PasswordUpdatedAt = credentials.PasswordUpdatedAt
	user.Credentials.NeedUpdatePassword = credentials.NeedUpdatePassword
	return nil
}

func (r *fakeUserRepository) ReplaceIdentities(ctx context.Context, userID string, identities []userdomain.UserIdentity) error {
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	user.Identities = identities
	return nil
}

func (r *fakeUserRepository) CreateIdentity(ctx context.Context, identity *userdomain.UserIdentity) error {
	return nil
}

func (r *fakeUserRepository) DeleteIdentity(ctx context.Context, userID string, identityID string) error {
	return nil
}

func (r *fakeUserRepository) ReplaceMfaMethods(ctx context.Context, userID string, methods []userdomain.UserMfaMethod) error {
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	user.MfaMethods = methods
	return nil
}

func (r *fakeUserRepository) ReplaceRoleBindings(ctx context.Context, userID string, bindings []userdomain.UserRoleBinding) error {
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	user.RoleBindings = bindings
	return nil
}

func (r *fakeUserRepository) ListRoleNames(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}

func (r *fakeUserRepository) ReplaceGroupMemberships(ctx context.Context, userID string, memberships []userdomain.UserGroupMembership) error {
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	user.GroupMemberships = memberships
	return nil
}

func (r *fakeUserRepository) Delete(ctx context.Context, id, organizationID string) error {
	user, ok := r.items[id]
	if !ok || user.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeUserRepository) GetPasswordHistory(ctx context.Context, userID string, limit int) ([]userdomain.PasswordHistoryEntry, error) {
	return nil, nil
}

func (r *fakeUserRepository) CreatePasswordHistory(ctx context.Context, entry *userdomain.PasswordHistoryEntry) error {
	return nil
}

func (r *fakeUserRepository) DeletePasswordHistoryBeyond(ctx context.Context, userID string, keepCount int) error {
	return nil
}

func (r *fakeUserRepository) GetByProviderIdentity(ctx context.Context, providerID string, subject string) (*userdomain.User, error) {
	return nil, nil
}

func (r *fakeUserRepository) Count(ctx context.Context, organizationID string) (int64, error) {
	return 0, nil
}

func (r *fakeUserRepository) HasDeletedWithUsername(_ context.Context, _ string, _ string) (bool, error) {
	return false, nil
}

type fakeGroupRepository struct {
	items map[string]*groupsdomain.Group
}

func (r *fakeGroupRepository) GetByID(ctx context.Context, id string) (*groupsdomain.Group, error) {
	group, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("group not found")
	}
	return group, nil
}

func (r *fakeGroupRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*groupsdomain.Group, error) {
	group, ok := r.items[id]
	if !ok || group.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("group not found")
	}
	return group, nil
}

func (r *fakeGroupRepository) List(ctx context.Context, query groupsdomain.ListQuery) ([]*groupsdomain.Group, int64, error) {
	items := make([]*groupsdomain.Group, 0)
	for _, group := range r.items {
		if query.OrganizationID != "" && group.OrganizationID != query.OrganizationID {
			continue
		}
		if query.Keyword != "" && !strings.Contains(group.Name, query.Keyword) && !strings.Contains(group.DisplayName, query.Keyword) {
			continue
		}
		if query.Status == "active" && !group.IsEnabled {
			continue
		}
		if query.Status == "inactive" && group.IsEnabled {
			continue
		}
		items = append(items, group)
	}
	return items, int64(len(items)), nil
}

func (r *fakeGroupRepository) Create(ctx context.Context, group *groupsdomain.Group) error {
	r.items[group.ID] = group
	return nil
}

func (r *fakeGroupRepository) Update(ctx context.Context, group *groupsdomain.Group) error {
	r.items[group.ID] = group
	return nil
}

func (r *fakeGroupRepository) Delete(ctx context.Context, id, organizationID string) error {
	group, ok := r.items[id]
	if !ok || group.OrganizationID != organizationID {
		return apperrors.NewNotFound("group not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeGroupRepository) CountChildrenByParentID(ctx context.Context, organizationID string, parentID string) (int64, error) {
	var count int64
	for _, group := range r.items {
		if group.OrganizationID == organizationID && group.ParentID == parentID {
			count++
		}
	}
	return count, nil
}

type fakeInvitationRepository struct {
	items map[string]*invitationsdomain.Invitation
}

func (r *fakeInvitationRepository) GetByID(ctx context.Context, id string) (*invitationsdomain.Invitation, error) {
	invitation, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("invitation not found")
	}
	return invitation, nil
}

func (r *fakeInvitationRepository) List(ctx context.Context, query invitationsdomain.ListQuery) ([]*invitationsdomain.Invitation, int64, error) {
	items := make([]*invitationsdomain.Invitation, 0)
	for _, invitation := range r.items {
		if query.OrganizationID != "" && invitation.OrganizationID != query.OrganizationID {
			continue
		}
		if query.Keyword != "" && !strings.Contains(invitation.Email, query.Keyword) {
			continue
		}
		if query.Status != "" && invitation.Status != query.Status {
			continue
		}
		items = append(items, invitation)
	}
	return items, int64(len(items)), nil
}

func (r *fakeInvitationRepository) Create(ctx context.Context, invitation *invitationsdomain.Invitation) error {
	r.items[invitation.ID] = invitation
	return nil
}

func (r *fakeInvitationRepository) Update(ctx context.Context, invitation *invitationsdomain.Invitation) error {
	r.items[invitation.ID] = invitation
	return nil
}

func (r *fakeInvitationRepository) Delete(ctx context.Context, id string) error {
	delete(r.items, id)
	return nil
}

func (r *fakeInvitationRepository) GetByCode(ctx context.Context, code string) (*invitationsdomain.Invitation, error) {
	for _, inv := range r.items {
		if inv.Code == code {
			return inv, nil
		}
	}
	return nil, apperrors.NewNotFound("invitation not found by code")
}

func (r *fakeInvitationRepository) GetByEmail(ctx context.Context, orgID string, email string) (*invitationsdomain.Invitation, error) {
	for _, inv := range r.items {
		if inv.Email == email && inv.OrganizationID == orgID {
			return inv, nil
		}
	}
	return nil, apperrors.NewNotFound("invitation not found by email")
}

type fakeGatewayRepository struct{}

func (r *fakeGatewayRepository) GetServerByID(ctx context.Context, id string) (*gatewaydomain.Server, error) {
	return nil, apperrors.NewNotFound("server not found")
}

func (r *fakeGatewayRepository) GetServerByIDForOrganization(ctx context.Context, id, organizationID string) (*gatewaydomain.Server, error) {
	return nil, apperrors.NewNotFound("server not found")
}

func (r *fakeGatewayRepository) ListServers(ctx context.Context, query gatewaydomain.ListServersQuery) ([]*gatewaydomain.Server, int64, error) {
	return nil, 0, nil
}

func (r *fakeGatewayRepository) CreateServer(ctx context.Context, server *gatewaydomain.Server) error {
	return nil
}

func (r *fakeGatewayRepository) UpdateServer(ctx context.Context, server *gatewaydomain.Server) error {
	return nil
}

func (r *fakeGatewayRepository) DeleteServer(ctx context.Context, id string) error {
	return nil
}

func (r *fakeGatewayRepository) GetSiteByID(ctx context.Context, id string) (*gatewaydomain.Site, error) {
	return nil, apperrors.NewNotFound("site not found")
}

func (r *fakeGatewayRepository) GetSiteByIDForOrganization(ctx context.Context, id, organizationID string) (*gatewaydomain.Site, error) {
	return nil, apperrors.NewNotFound("site not found")
}

func (r *fakeGatewayRepository) ListSites(ctx context.Context, query gatewaydomain.ListSitesQuery) ([]*gatewaydomain.Site, int64, error) {
	return nil, 0, nil
}

func (r *fakeGatewayRepository) CreateSite(ctx context.Context, site *gatewaydomain.Site) error {
	return nil
}

func (r *fakeGatewayRepository) UpdateSite(ctx context.Context, site *gatewaydomain.Site) error {
	return nil
}

func (r *fakeGatewayRepository) DeleteSite(ctx context.Context, id string) error {
	return nil
}

type fakeRecordRepository struct {
	items map[string]*recordsdomain.Record
}

func (r *fakeRecordRepository) Create(ctx context.Context, record *recordsdomain.Record) error {
	r.items[record.ID] = record
	return nil
}

func (r *fakeRecordRepository) List(ctx context.Context, query recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *fakeRecordRepository) Search(ctx context.Context, criteria recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *fakeRecordRepository) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}
func (r *fakeRecordRepository) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (r *fakeRecordRepository) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}
func (r *fakeRecordRepository) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

type fakeVerificationRepository struct {
	items map[string]*verificationsdomain.Verification
}

func (r *fakeVerificationRepository) GetByID(ctx context.Context, id string) (*verificationsdomain.Verification, error) {
	return nil, apperrors.NewNotFound("verification not found")
}

func (r *fakeVerificationRepository) List(ctx context.Context, query verificationsdomain.ListQuery) ([]*verificationsdomain.Verification, int64, error) {
	return nil, 0, nil
}

func (r *fakeVerificationRepository) Create(ctx context.Context, verification *verificationsdomain.Verification) error {
	return nil
}

func (r *fakeVerificationRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	return nil
}

func (r *fakeVerificationRepository) GetActiveByRecipientAndCode(ctx context.Context, recipient string, code string, verificationType string) (*verificationsdomain.Verification, error) {
	return nil, apperrors.NewNotFound("verification not found")
}

func (r *fakeVerificationRepository) GetActiveByRecipient(ctx context.Context, recipient string, verificationType string) (*verificationsdomain.Verification, error) {
	return nil, apperrors.NewNotFound("verification not found")
}

func (r *fakeVerificationRepository) UpdateCode(ctx context.Context, id string, code string) error {
	return nil
}

type fakeLockoutAttemptRepository struct {
	items map[string]*lockoutattempts.Attempt
}

func (r *fakeLockoutAttemptRepository) GetByUserID(ctx context.Context, userID string) (*lockoutattempts.Attempt, error) {
	if a, ok := r.items[userID]; ok {
		return a, nil
	}
	return nil, nil
}

func (r *fakeLockoutAttemptRepository) Create(ctx context.Context, attempt *lockoutattempts.Attempt) error {
	r.items[attempt.UserID] = attempt
	return nil
}

func (r *fakeLockoutAttemptRepository) Update(ctx context.Context, attempt *lockoutattempts.Attempt) error {
	r.items[attempt.UserID] = attempt
	return nil
}

func (r *fakeLockoutAttemptRepository) DeleteByUserID(ctx context.Context, userID string) error {
	delete(r.items, userID)
	return nil
}

func (r *fakeLockoutAttemptRepository) DeleteExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

// RecordFailure mirrors the lockout state transition for bootstrap tests.
func (r *fakeLockoutAttemptRepository) RecordFailure(ctx context.Context, userID string, maxAttempts int, lockDurationMinutes int, now time.Time) (*lockoutattempts.Attempt, error) {
	if r.items == nil {
		r.items = map[string]*lockoutattempts.Attempt{}
	}

	attempt, ok := r.items[userID]
	if !ok {
		attempt = &lockoutattempts.Attempt{
			UserID:          userID,
			CreatedAt:       now,
			WindowStartedAt: now,
			UpdatedAt:       now,
		}
		r.items[userID] = attempt
	}
	if attempt.WindowStartedAt.IsZero() {
		attempt.WindowStartedAt = attempt.CreatedAt
		if attempt.WindowStartedAt.IsZero() {
			attempt.WindowStartedAt = now
		}
	}
	attempt.RecordFailure(now, maxAttempts, time.Duration(lockDurationMinutes)*time.Minute)
	attempt.UpdatedAt = now
	return attempt, nil
}

// fakeTrustedDeviceRepository 是最小替身，仅保证凭据级联路径不会因为缺少
// 仓储能力而返回 internal error。
type fakeTrustedDeviceRepository struct{}

func (fakeTrustedDeviceRepository) Create(context.Context, *trusteddevicesdomain.TrustedDevice) error {
	return nil
}

func (fakeTrustedDeviceRepository) GetByID(context.Context, string) (*trusteddevicesdomain.TrustedDevice, error) {
	return nil, apperrors.NewNotFound("trusted device not found")
}

func (fakeTrustedDeviceRepository) ListByUser(context.Context, string, string) ([]*trusteddevicesdomain.TrustedDevice, error) {
	return nil, nil
}

func (fakeTrustedDeviceRepository) FindActiveByCredential(context.Context, string, string, string, string, time.Time) (*trusteddevicesdomain.TrustedDevice, error) {
	return nil, apperrors.NewNotFound("trusted device not found")
}

func (fakeTrustedDeviceRepository) Revoke(context.Context, string, string, time.Time) error {
	return nil
}

func (fakeTrustedDeviceRepository) TouchLastSeen(context.Context, string, time.Time) error {
	return nil
}

func (fakeTrustedDeviceRepository) RevokeByCredential(context.Context, string, string, string, time.Time) error {
	return nil
}

func (fakeTrustedDeviceRepository) RevokeByUser(context.Context, string, string, time.Time) (int64, error) {
	return 0, nil
}

func mustHashRecoveryCode(t *testing.T, code string) string {
	t.Helper()
	hash, err := platformcrypto.HashClientSecret(code)
	if err != nil {
		t.Fatalf("hash recovery code: %v", err)
	}
	return hash
}

type fakeEnforcerRepository struct {
	items map[string]*enforcerdomain.Enforcer
}

func (r *fakeEnforcerRepository) GetByID(_ context.Context, id string) (*enforcerdomain.Enforcer, error) {
	if item, ok := r.items[id]; ok {
		return item, nil
	}
	return nil, apperrors.NewNotFound("enforcer not found")
}

func (r *fakeEnforcerRepository) GetByIDForOrganization(_ context.Context, id, organizationID string) (*enforcerdomain.Enforcer, error) {
	if item, ok := r.items[id]; ok && item.OrganizationID == organizationID {
		return item, nil
	}
	return nil, apperrors.NewNotFound("enforcer not found")
}

func (r *fakeEnforcerRepository) List(_ context.Context, _ enforcerdomain.ListQuery) ([]*enforcerdomain.Enforcer, int64, error) {
	items := make([]*enforcerdomain.Enforcer, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (r *fakeEnforcerRepository) Create(_ context.Context, enforcer *enforcerdomain.Enforcer) error {
	r.items[enforcer.ID] = enforcer
	return nil
}

func (r *fakeEnforcerRepository) Update(_ context.Context, enforcer *enforcerdomain.Enforcer) error {
	r.items[enforcer.ID] = enforcer
	return nil
}

func (r *fakeEnforcerRepository) Delete(_ context.Context, id, _ string) error {
	delete(r.items, id)
	return nil
}

type fakeModelRepository struct {
	items map[string]*modeldomain.Model
}

func (r *fakeModelRepository) GetByID(_ context.Context, id string) (*modeldomain.Model, error) {
	if item, ok := r.items[id]; ok {
		return item, nil
	}
	return nil, apperrors.NewNotFound("model not found")
}

func (r *fakeModelRepository) GetByIDForOrganization(_ context.Context, id, organizationID string) (*modeldomain.Model, error) {
	if item, ok := r.items[id]; ok && item.OrganizationID == organizationID {
		return item, nil
	}
	return nil, apperrors.NewNotFound("model not found")
}

func (r *fakeModelRepository) List(_ context.Context, _ modeldomain.ListQuery) ([]*modeldomain.Model, int64, error) {
	items := make([]*modeldomain.Model, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (r *fakeModelRepository) Create(_ context.Context, model *modeldomain.Model) error {
	r.items[model.ID] = model
	return nil
}

func (r *fakeModelRepository) Update(_ context.Context, model *modeldomain.Model) error {
	r.items[model.ID] = model
	return nil
}

func (r *fakeModelRepository) Delete(_ context.Context, id, _ string) error {
	delete(r.items, id)
	return nil
}

func TestV1AuthLoginResolvesOrganizationDefaultApplicationRoute(t *testing.T) {
	router, _, _, _, _, _, _, _, sessions := newTestV1RouterWithSessions(t)

	loginRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"organizationSlug": "built-in",
		"username":         "jane",
		"password":         "Password!2026",
	})
	assertStatus(t, loginRecorder, http.StatusOK)

	createdSession := ""
	for id, session := range sessions.items {
		if id != "session-1" && session.UserID == "user-1" {
			createdSession = id
		}
	}
	if createdSession == "" {
		t.Fatalf("expected login to create a new session")
	}
	if got := sessions.items[createdSession].ApplicationID; got != "01a00000-0000-7000-8000-0000000000a1" {
		t.Fatalf("session applicationId = %q, want configured organization default application", got)
	}
}
