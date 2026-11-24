package applications

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMax_Applications(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    int
		fallback int
		want     int
	}{
		{"positive_value", 5, 10, 5},
		{"zero_uses_fallback", 0, 10, 10},
		{"negative_uses_fallback", -1, 20, 20},
		{"both_positive", 15, 10, 15},
		{"same_values", 10, 10, 10},
		{"large_page_size", 500, 20, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxInt(tt.value, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDerefString_Boundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input *string
		want  string
	}{
		{"nil_pointer", nil, ""},
		{"empty_string", strPtr(""), ""},
		{"normal_string", strPtr("hello"), "hello"},
		{"uuid_string", strPtr("01900000-0000-7000-8000-000000000001"), "01900000-0000-7000-8000-000000000001"},
		{"special_chars", strPtr("O'Brien"), "O'Brien"},
		{"unicode", strPtr("应用"), "应用"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := derefString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestApplicationRow_ClientIDUniqueness(t *testing.T) {
	t.Parallel()

	t.Run("client_id_populated", func(t *testing.T) {
		clientID := "gl_client_abc123"
		row := applicationRow{
			ID:               "app-001",
			OrganizationID:   "org-001",
			Slug:             "my-app",
			DisplayName:      "My App",
			ClientID:         &clientID,
			ClientSecretHash: strPtr("sha256hash"),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
			Revision:         1,
		}

		require.NotNil(t, row.ClientID)
		assert.Equal(t, "gl_client_abc123", *row.ClientID)
	})

	t.Run("client_id_nil", func(t *testing.T) {
		row := applicationRow{
			ID:               "app-002",
			OrganizationID:   "org-001",
			Slug:             "no-client",
			DisplayName:      "No Client App",
			ClientID:         nil,
			ClientSecretHash: nil,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		assert.Nil(t, row.ClientID)
	})

	t.Run("different_client_ids", func(t *testing.T) {
		clientID1 := "gl_client_app1"
		clientID2 := "gl_client_app2"
		row1 := applicationRow{ID: "app-1", ClientID: &clientID1}
		row2 := applicationRow{ID: "app-2", ClientID: &clientID2}

		assert.NotEqual(t, *row1.ClientID, *row2.ClientID)
	})
}

func TestApplicationRow_OrganizationFilter(t *testing.T) {
	t.Parallel()

	t.Run("same_org_different_apps", func(t *testing.T) {
		orgID := "org-001"
		row1 := applicationRow{
			ID:             "app-001",
			OrganizationID: orgID,
			Slug:           "app-a",
			DisplayName:    "App A",
		}
		row2 := applicationRow{
			ID:             "app-002",
			OrganizationID: orgID,
			Slug:           "app-b",
			DisplayName:    "App B",
		}

		assert.Equal(t, row1.OrganizationID, row2.OrganizationID)
		assert.NotEqual(t, row1.Slug, row2.Slug)
	})

	t.Run("different_orgs", func(t *testing.T) {
		row1 := applicationRow{
			ID:             "app-001",
			OrganizationID: "org-001",
		}
		row2 := applicationRow{
			ID:             "app-002",
			OrganizationID: "org-002",
		}

		assert.NotEqual(t, row1.OrganizationID, row2.OrganizationID)
	})
}

func TestApplicationRow_AllFieldsAccessible(t *testing.T) {
	t.Parallel()

	now := time.Now()
	desc := "Full Application"
	homepage := "https://app.example.com"
	domain := "app.example.com"
	clientID := "gl_client_full"
	clientSecretHash := "sha256full"

	row := applicationRow{
		ID:                        "app-full",
		OrganizationID:            "org-full",
		Slug:                      "full-app",
		DisplayName:               "Full Application",
		Category:                  "web",
		AppType:                   "native",
		Description:               &desc,
		HomepageURL:               &homepage,
		Domain:                    &domain,
		Status:                    "active",
		ClientID:                  &clientID,
		ClientSecretHash:          &clientSecretHash,
		TokenEndpointAuthMethod:   "client_secret_basic",
		RedirectURIs:              []string{"https://app.example.com/callback"},
		GrantTypes:                []string{"authorization_code", "refresh_token"},
		DisableSignin:             false,
		Tags:                      []string{"production"},
		EnableExclusiveSignin:     false,
		EnableSigninSession:       true,
		Logo:                      "logo.png",
		EnableSamlCompress:        false,
		TokenFormat:               "JWT",
		ExpireInHours:             24,
		RefreshTokenExpireInHours: 168,
		TokenSigningMethod:        "RS256",
		IsPortal:                  false,
		IsVisible:                 true,
		ConsentMode:               "explicit",
		DpopRequired:              false,
		PkceRequired:              true,
		IpWhitelist:               "",
		IpBlacklist:               "",
		FailedSigninLimit:         5,
		FailedSigninFrozenTime:    15,
		CodeResendTimeout:         60,
		CorsAllowedOrigins:        "*",
		EnableBetaOidc:            false,
		DefaultGroup:              "",
		OrgChoiceMode:             "single",
		CreatedAt:                 now,
		UpdatedAt:                 now,
		Revision:                  3,
	}

	assert.Equal(t, "app-full", row.ID)
	assert.Equal(t, "org-full", row.OrganizationID)
	assert.Equal(t, "full-app", row.Slug)
	assert.Equal(t, "Full Application", row.DisplayName)
	assert.Equal(t, "web", row.Category)
	assert.Equal(t, "native", row.AppType)
	assert.Equal(t, "active", row.Status)
	assert.Equal(t, "gl_client_full", *row.ClientID)
	assert.Equal(t, []string{"authorization_code", "refresh_token"}, row.GrantTypes)
	assert.Equal(t, []string{"https://app.example.com/callback"}, row.RedirectURIs)
	assert.Equal(t, true, row.PkceRequired)
	assert.Equal(t, 24, row.ExpireInHours)
	assert.Equal(t, 168, row.RefreshTokenExpireInHours)
	assert.Equal(t, int64(3), row.Revision)
}

func TestApplicationRow_Phase2Fields(t *testing.T) {
	t.Parallel()

	t.Run("oidc_fields", func(t *testing.T) {
		row := applicationRow{
			ID:                   "app-oidc",
			ConsentMode:          "explicit",
			DpopRequired:         true,
			PkceRequired:         true,
			CorsAllowedOrigins:   "https://app1.com,https://app2.com",
			EnableBetaOidc:       true,
			BackchannelLogoutUri: "https://app.example.com/backchannel",
		}

		assert.Equal(t, "explicit", row.ConsentMode)
		assert.True(t, row.DpopRequired)
		assert.True(t, row.PkceRequired)
		assert.True(t, row.EnableBetaOidc)
	})

	t.Run("saml_fields", func(t *testing.T) {
		row := applicationRow{
			ID:                           "app-saml",
			SamlReplyUrl:                 "https://app.example.com/saml/acs",
			SamlMetadata:                 "<EntityDescriptor>...</EntityDescriptor>",
			SamlNameIdFormat:             "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
			EnableSamlPostBinding:        true,
			EnableSamlAssertionSignature: true,
			EnableSamlC14n10:             false,
			DisableSamlAttributes:        false,
			UseEmailAsSamlNameId:         true,
		}

		assert.Equal(t, "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress", row.SamlNameIdFormat)
		assert.True(t, row.EnableSamlPostBinding)
		assert.True(t, row.EnableSamlAssertionSignature)
		assert.True(t, row.UseEmailAsSamlNameId)
	})

	t.Run("signin_customization_fields", func(t *testing.T) {
		row := applicationRow{
			ID:            "app-signin",
			FormCss:       ".form { color: blue; }",
			FormCssMobile: ".form { color: red; }",
			FormSideHtml:  "<div>Side</div>",
			HeaderHtml:    "<header>App</header>",
			FooterHtml:    "<footer>App</footer>",
			SigninHtml:    "<div>Signin</div>",
			SignupHtml:    "<div>Signup</div>",
			TermsOfUse:    "https://app.example.com/terms",
			PrivacyPolicy: "https://app.example.com/privacy",
		}

		assert.Contains(t, row.FormCss, "blue")
		assert.Contains(t, row.HeaderHtml, "App")
		assert.NotEmpty(t, row.TermsOfUse)
		assert.NotEmpty(t, row.PrivacyPolicy)
	})
}

func TestToApplication_Boundary(t *testing.T) {
	t.Parallel()

	t.Run("minimal_app", func(t *testing.T) {
		row := applicationRow{
			ID:             "app-min",
			OrganizationID: "org-001",
			Slug:           "min",
			DisplayName:    "Min",
		}

		app, err := toApplication(row)
		require.NoError(t, err)
		assert.Equal(t, "app-min", app.ID)
		assert.Equal(t, "", app.Description)
		assert.Equal(t, "", app.ClientID)
	})

	t.Run("full_app_with_jsonb", func(t *testing.T) {
		row := applicationRow{
			ID:              "app-jsonb",
			OrganizationID:  "org-001",
			Slug:            "jsonb-app",
			DisplayName:     "JSONB App",
			CustomScopes:    []byte(`[{"name":"custom","displayName":"Custom"}]`),
			SignupItems:     []byte(`[{"name":"username","visible":true}]`),
			ProviderItems:   []byte(`[{"name":"github","canSignUp":true}]`),
			TokenAttributes: []byte(`[{"name":"attr1","sourceField":"email"}]`),
			SamlAttributes:  []byte(`[{"name":"email","nameFormat":"urn:oasis"}]`),
			SigninMethods:   []byte(`["password"]`),
			SigninItems:     []byte(`[{"name":"username","visible":true}]`),
		}

		app, err := toApplication(row)
		require.NoError(t, err)
		assert.Equal(t, "app-jsonb", app.ID)
	})
}

// strPtr is a helper to create a string pointer from a literal.
func strPtr(s string) *string {
	return &s
}
