package applications

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

func publicApplicationAggregate() *domain.Application {
	now := time.Now().UTC()
	return &domain.Application{
		ID:                      "app-public-1",
		OrganizationID:          "org-public-1",
		Slug:                    "public-app",
		DisplayName:             "Public App",
		Description:             "Public description",
		HomepageURL:             "https://public.example.com",
		ClientID:                "public-client",
		Logo:                    "https://public.example.com/logo.png",
		Title:                   "Public title",
		Favicon:                 "https://public.example.com/favicon.ico",
		IsShared:                true,
		EnableAutoSignin:        true,
		EnableSignup:            true,
		ForcedRedirectOrigin:    "https://public.example.com",
		SignupUrl:               "/signup/public",
		ForgetUrl:               "/forgot-password/public",
		TermsOfUse:              "public terms",
		PrivacyPolicy:           "public privacy policy",
		OrgChoiceMode:           "search",
		SigninHtml:              "<div>public signin</div>",
		SignupHtml:              "<div>public signup</div>",
		HeaderHtml:              "<header>public header</header>",
		FooterHtml:              "<footer>public footer</footer>",
		CustomCss:               ".public-custom { color: green; }",
		CustomHeader:            `<meta name="description" content="public">`,
		FormCss:                 ".public { color: red; }",
		FormCssMobile:           ".public-mobile { color: blue; }",
		FormSideHtml:            "<aside>public side</aside>",
		FormBackgroundUrl:       "https://public.example.com/bg.png",
		FormBackgroundUrlMobile: "https://public.example.com/bg-mobile.png",
		FormOffset:              "4",
		SigninMethods:           []string{"password", "ldap"},
		SigninItems: []domain.SigninItemConfig{
			{Name: "Logo", Visible: true, Rule: "Always"},
		},
		SignupItems: []domain.SignupItem{
			{Name: "Username", Visible: true, Required: true},
		},

		// Sensitive columns: none of these may be mapped by toPublicDetail.
		Certificate:      "SENTINEL-service-certificate",
		SamlMetadata:     "SENTINEL-service-saml-metadata",
		SamlReplyUrl:     "SENTINEL-service-saml-reply",
		RedirectURIs:     []string{"SENTINEL-service-redirect-uri"},
		GrantTypes:       []string{"authorization_code"},
		IpWhitelist:      "SENTINEL-service-ip-whitelist",
		SslCert:          "SENTINEL-service-ssl-cert",
		ClientCert:       "SENTINEL-service-client-cert",
		ClientSecretHash: "SENTINEL-service-client-secret-hash",
		TokenAttributes: []domain.TokenAttribute{
			{Name: "SENTINEL-service-token-attribute", SourceField: "sentinel"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestService_GetApplicationPublicByClientID_ProjectsPublicFields(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{appRepo: &mockApplicationRepo{
		apps: []*domain.Application{publicApplicationAggregate()},
	}}}

	detail, err := svc.GetApplicationPublicByClientID(context.Background(), "public-client")
	require.NoError(t, err)
	require.NotNil(t, detail)

	assert.Equal(t, "app-public-1", detail.ID)
	assert.Equal(t, "org-public-1", detail.OrganizationID)
	assert.Equal(t, "public-app", detail.Slug)
	assert.Equal(t, "public-app", detail.Name)
	assert.Equal(t, "public-client", detail.ClientID)
	assert.Equal(t, "Public App", detail.DisplayName)
	assert.Equal(t, "Public title", detail.Title)
	assert.Equal(t, "https://public.example.com/favicon.ico", detail.Favicon)
	assert.Equal(t, []string{"password", "ldap"}, detail.SigninMethods)
	assert.Equal(t, ".public { color: red; }", detail.FormCSS)
	assert.Equal(t, ".public-custom { color: green; }", detail.CustomCSS)
	assert.Equal(t, `<meta name="description" content="public">`, detail.CustomHeader)
	assert.Equal(t, "<header>public header</header>", detail.HeaderHTML)
	assert.Equal(t, "<footer>public footer</footer>", detail.FooterHTML)
	assert.Equal(t, "<div>public signup</div>", detail.SignupHTML)
	assert.Equal(t, "public privacy policy", detail.PrivacyPolicy)
	assert.Equal(t, "search", detail.OrgChoiceMode)
	assert.True(t, detail.IsShared)
	assert.True(t, detail.EnableAutoSignin)
	assert.True(t, detail.EnableSignup)

	payload, err := json.Marshal(detail)
	require.NoError(t, err)
	body := string(payload)
	for _, leaked := range []string{
		"SENTINEL-service-certificate",
		"SENTINEL-service-saml-metadata",
		"SENTINEL-service-saml-reply",
		"SENTINEL-service-redirect-uri",
		"SENTINEL-service-ip-whitelist",
		"SENTINEL-service-ssl-cert",
		"SENTINEL-service-client-cert",
		"SENTINEL-service-client-secret-hash",
		"SENTINEL-service-token-attribute",
	} {
		assert.NotContains(t, body, leaked)
	}
	for _, key := range []string{
		`"certificate"`,
		`"samlMetadata"`,
		`"samlReplyUrl"`,
		`"redirectUris"`,
		`"grantTypes"`,
		`"ipWhitelist"`,
		`"sslCert"`,
		`"clientCert"`,
		`"clientSecretHash"`,
		`"tokenAttributes"`,
	} {
		assert.NotContains(t, body, key)
	}
}

func TestService_GetApplicationPublicByClientID_NotFound(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{appRepo: &mockApplicationRepo{}}}

	_, err := svc.GetApplicationPublicByClientID(context.Background(), "missing-client")
	require.Error(t, err)
}

// resolve（ownerSlug/appSlug）与 by-client-id 同为匿名预登录发现，
// 必须返回同一公开投影：管理详情里的证书/SAML/回调/传输配置不得泄漏。
func TestService_GetApplicationByOrgAndSlug_ProjectsPublicFields(t *testing.T) {
	svc := Service{UOW: appTestUOW{
		appRepo: &mockApplicationRepo{apps: []*domain.Application{publicApplicationAggregate()}},
		orgRepo: &appTestOrgRepo{orgs: []*orgdomain.Organization{
			{ID: "org-public-1", Slug: "public-org"},
		}},
	}}

	detail, err := svc.GetApplicationByOrgAndSlug(context.Background(), "public-org", "public-app")
	require.NoError(t, err)
	require.NotNil(t, detail)

	assert.Equal(t, "app-public-1", detail.ID)
	assert.Equal(t, "public-app", detail.Slug)
	assert.Equal(t, ".public-custom { color: green; }", detail.CustomCSS)
	assert.Equal(t, "<footer>public footer</footer>", detail.FooterHTML)
	assert.True(t, detail.EnableSignup)

	payload, err := json.Marshal(detail)
	require.NoError(t, err)
	body := string(payload)
	for _, leaked := range []string{
		"SENTINEL-service-certificate",
		"SENTINEL-service-saml-metadata",
		"SENTINEL-service-saml-reply",
		"SENTINEL-service-redirect-uri",
		"SENTINEL-service-ip-whitelist",
		"SENTINEL-service-ssl-cert",
		"SENTINEL-service-client-cert",
		"SENTINEL-service-client-secret-hash",
		"SENTINEL-service-token-attribute",
	} {
		assert.NotContains(t, body, leaked)
	}
	for _, key := range []string{
		`"certificate"`,
		`"samlMetadata"`,
		`"redirectUris"`,
		`"clientJwks"`,
		`"revision"`,
		`"tokenEndpointAuthMethod"`,
	} {
		assert.NotContains(t, body, key)
	}
}

func TestService_GetApplicationByOrgAndSlug_NotFound(t *testing.T) {
	svc := Service{UOW: appTestUOW{
		appRepo: &mockApplicationRepo{},
		orgRepo: &appTestOrgRepo{orgs: []*orgdomain.Organization{
			{ID: "org-public-1", Slug: "public-org"},
		}},
	}}

	_, err := svc.GetApplicationByOrgAndSlug(context.Background(), "public-org", "missing-app")
	require.Error(t, err)

	_, err = svc.GetApplicationByOrgAndSlug(context.Background(), "missing-org", "public-app")
	require.Error(t, err)
}
