package applications

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// Sentinels planted on application columns that the anonymous by-client-id
// discovery route must never expose: SAML material, callback allowlists,
// transport/certificate configuration and client secret hashes.
const (
	publicAppCertificateSentinel  = "SENTINEL-app-certificate"
	publicAppSamlMetadataSentinel = "SENTINEL-app-saml-metadata"
	publicAppSamlReplySentinel    = "SENTINEL-app-saml-reply"
	publicAppRedirectSentinel     = "SENTINEL-app-redirect-uri"
	publicAppIPWhitelistSentinel  = "SENTINEL-app-ip-whitelist"
	publicAppSSLModeSentinel      = "SENTINEL-app-ssl-mode"
	publicAppSSLCertSentinel      = "SENTINEL-app-ssl-cert"
	publicAppClientCertSentinel   = "SENTINEL-app-client-cert"
	publicAppSecretHashSentinel   = "SENTINEL-app-client-secret-hash"
)

var publicAppForbiddenKeys = []string{
	"certificate",
	"samlMetadata",
	"samlReplyUrl",
	"redirectUris",
	"grantTypes",
	"tokenSigningMethod",
	"samlNameIdFormat",
	"samlAttributes",
	"ipWhitelist",
	"ipBlacklist",
	"corsAllowedOrigins",
	"sslMode",
	"sslCert",
	"clientCert",
	"upstreamHost",
	"otherDomains",
	"clientSecretHash",
	"tokenFields",
	"tokenAttributes",
	"customScopes",
	"signinUrl",
	"logoutUrl",
	"logoutRedirectUrl",
	"promptUrl",
	"backchannelLogoutUri",
	"frontchannelLogoutUri",
	"defaultGroup",
	"clientJwks",
	"revision",
	"tokenEndpointAuthMethod",
}

var publicAppForbiddenValues = []string{
	publicAppCertificateSentinel,
	publicAppSamlMetadataSentinel,
	publicAppSamlReplySentinel,
	publicAppRedirectSentinel,
	publicAppIPWhitelistSentinel,
	publicAppSSLModeSentinel,
	publicAppSSLCertSentinel,
	publicAppClientCertSentinel,
	publicAppSecretHashSentinel,
}

func publicProjectionApplication(organizationID, clientID, slug, marker string) *domain.Application {
	return &domain.Application{
		ID:                      "app-" + marker,
		OrganizationID:          organizationID,
		Slug:                    slug,
		DisplayName:             marker + " application",
		Description:             marker + " description",
		HomepageURL:             "https://" + marker + ".example.com",
		ClientID:                clientID,
		Logo:                    "https://" + marker + ".example.com/logo.png",
		Title:                   marker + " title",
		Favicon:                 "https://" + marker + ".example.com/favicon.ico",
		IsShared:                true,
		EnableAutoSignin:        true,
		EnableSignup:            true,
		ForcedRedirectOrigin:    "https://" + marker + ".example.com",
		SignupUrl:               "/signup/" + marker,
		ForgetUrl:               "/forgot-password/" + marker,
		TermsOfUse:              marker + " terms of use",
		PrivacyPolicy:           marker + " privacy policy",
		OrgChoiceMode:           "search",
		SigninHtml:              "<div>" + marker + " signin html</div>",
		SignupHtml:              "<div>" + marker + " signup html</div>",
		HeaderHtml:              "<header>" + marker + " header</header>",
		FooterHtml:              "<footer>" + marker + " footer</footer>",
		CustomCss:               "." + marker + "-custom { color: green; }",
		CustomHeader:            `<meta name="description" content="` + marker + `">`,
		FormCss:                 "." + marker + " { color: red; }",
		FormCssMobile:           "." + marker + "-mobile { color: blue; }",
		FormSideHtml:            "<aside>" + marker + " side</aside>",
		FormBackgroundUrl:       "https://" + marker + ".example.com/bg.png",
		FormBackgroundUrlMobile: "https://" + marker + ".example.com/bg-mobile.png",
		FormOffset:              "4",
		SigninMethods:           []string{"password", "ldap"},
		SigninItems: []domain.SigninItemConfig{
			{Name: "Logo", Visible: true, Rule: "Always"},
		},
		SignupItems: []domain.SignupItem{
			{Name: "Username", Visible: true, Required: true},
		},

		Certificate:        publicAppCertificateSentinel,
		SamlMetadata:       publicAppSamlMetadataSentinel,
		SamlReplyUrl:       publicAppSamlReplySentinel,
		RedirectURIs:       []string{publicAppRedirectSentinel},
		GrantTypes:         []string{"authorization_code"},
		TokenSigningMethod: "RS256",
		SamlNameIdFormat:   "emailAddress",
		SamlAttributes: []domain.SamlAttribute{
			{Name: "mail", Value: "saml-attribute-sentinel"},
		},
		IpWhitelist:        publicAppIPWhitelistSentinel,
		IpBlacklist:        "SENTINEL-app-ip-blacklist",
		CorsAllowedOrigins: "SENTINEL-app-cors-origin",
		SslMode:            publicAppSSLModeSentinel,
		SslCert:            publicAppSSLCertSentinel,
		ClientCert:         publicAppClientCertSentinel,
		UpstreamHost:       "SENTINEL-app-upstream-host",
		OtherDomains:       []string{"SENTINEL-app-other-domain"},
		ClientSecretHash:   publicAppSecretHashSentinel,
		TokenFields:        "SENTINEL-app-token-fields",
		TokenAttributes: []domain.TokenAttribute{
			{Name: "sentinel-token-attribute", SourceField: "sentinel"},
		},
		CustomScopes: []domain.ApplicationCustomScope{
			{Scope: "SENTINEL-app-scope", DisplayName: "sentinel", Description: "sentinel"},
		},
		SigninUrl:             "SENTINEL-app-signin-url",
		LogoutUrl:             "SENTINEL-app-logout-url",
		LogoutRedirectUrl:     "SENTINEL-app-logout-redirect",
		PromptUrl:             "SENTINEL-app-prompt-url",
		BackchannelLogoutUri:  "SENTINEL-app-backchannel-logout",
		FrontchannelLogoutUri: "SENTINEL-app-frontchannel-logout",
		DefaultGroup:          "SENTINEL-app-default-group",
	}
}

func assertNoPublicApplicationLeaks(t *testing.T, body string) {
	t.Helper()
	for _, key := range publicAppForbiddenKeys {
		assert.NotContains(t, body, `"`+key+`"`, "public application response exposes %q", key)
	}
	for _, value := range publicAppForbiddenValues {
		assert.NotContains(t, body, value, "public application response exposes sentinel %q", value)
	}
}

func assertPublicApplicationConsumptionFields(t *testing.T, body string) {
	t.Helper()
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(body), &payload))

	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	assert.ElementsMatch(t, []string{
		"id",
		"organizationId",
		"slug",
		"name",
		"displayName",
		"description",
		"homepageUrl",
		"clientId",
		"logo",
		"title",
		"favicon",
		"isShared",
		"enableAutoSignin",
		"enableSignup",
		"forcedRedirectOrigin",
		"signupUrl",
		"forgetUrl",
		"termsOfUse",
		"privacyPolicy",
		"orgChoiceMode",
		"signinHtml",
		"signupHtml",
		"headerHtml",
		"footerHtml",
		"customCss",
		"customHeader",
		"formCss",
		"formCssMobile",
		"formSideHtml",
		"formBackgroundUrl",
		"formBackgroundUrlMobile",
		"formOffset",
		"signinMethods",
		"signinItems",
		"signupItems",
	}, keys, "public application projection must stay minimal")

	assert.Contains(t, body, `"clientId":"sso-client"`)
	assert.Contains(t, body, `"slug":"sso-app"`)
	assert.Contains(t, body, `"name":"sso-app"`)
	assert.Contains(t, body, `"displayName":"sso application"`)
	assert.Contains(t, body, `"title":"sso title"`)
	assert.Contains(t, body, `"signinMethods":["password","ldap"]`)
	assert.Contains(t, body, `"formCss":".sso { color: red; }"`)
}

// TestPublicApplicationProjection_ByClientID is the B3 regression for the
// anonymous GET /api/v1/applications/by-client-id/{clientId} discovery route.
// It pins two properties at once: the response no longer carries SAML
// certificates/metadata, redirect allowlists or transport secrets, and two
// applications in different tenants are resolved strictly by client ID (the
// endpoint is tenant-free by design, so the selector is the client ID itself).
func TestPublicApplicationProjection_ByClientID(t *testing.T) {
	repo := &byIDApplicationRepo{apps: []*domain.Application{
		publicProjectionApplication(byIDForeignOrg, "foreign-client", "foreign-app", "foreign"),
		publicProjectionApplication(byIDSessionOrg, "sso-client", "sso-app", "sso"),
	}}
	router, _ := newByIDRouter(repo)

	recorder := serveByID(router, httptest.NewRequest(http.MethodGet,
		"/api/v1/applications/by-client-id/sso-client", nil))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := recorder.Body.String()
	assertNoPublicApplicationLeaks(t, body)
	assertPublicApplicationConsumptionFields(t, body)
	assert.NotContains(t, body, "foreign-client")
	assert.NotContains(t, body, "foreign-app")
}

// resolve 与 by-client-id 同为匿名发现路由，必须钉死同一公开投影：
// 管理详情的证书/SAML/回调/传输字段与 clientJwks/revision 不得出现。
func TestPublicApplicationProjection_Resolve(t *testing.T) {
	repo := &byIDApplicationRepo{apps: []*domain.Application{
		publicProjectionApplication(byIDSessionOrg, "sso-client", "sso-app", "sso"),
	}}
	router, _ := newByIDRouter(repo)

	recorder := serveByID(router, httptest.NewRequest(http.MethodGet,
		"/api/v1/applications/resolve?ownerSlug=session-org&appSlug=sso-app", nil))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := recorder.Body.String()
	assertNoPublicApplicationLeaks(t, body)
	assertPublicApplicationConsumptionFields(t, body)
}

func TestPublicApplicationProjection_Resolve_NotFound(t *testing.T) {
	repo := &byIDApplicationRepo{apps: []*domain.Application{
		publicProjectionApplication(byIDSessionOrg, "sso-client", "sso-app", "sso"),
	}}
	router, _ := newByIDRouter(repo)

	recorder := serveByID(router, httptest.NewRequest(http.MethodGet,
		"/api/v1/applications/resolve?ownerSlug=session-org&appSlug=missing", nil))
	require.Equal(t, http.StatusNotFound, recorder.Code)

	recorder = serveByID(router, httptest.NewRequest(http.MethodGet,
		"/api/v1/applications/resolve?ownerSlug=missing-org&appSlug=sso-app", nil))
	require.Equal(t, http.StatusNotFound, recorder.Code)
}
