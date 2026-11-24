package providers

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestProvider_Validate(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid OAuth provider",
			provider: Provider{
				Name:         "github",
				Category:     CategoryOAuth,
				ProviderType: ProviderGitHub,
				ClientID:     "gh-client-id",
				AuthUrl:      "https://github.com/login/oauth/authorize",
				TokenUrl:     "https://github.com/login/oauth/access_token",
			},
			wantErr: false,
		},
		{
			name: "valid OIDC provider",
			provider: Provider{
				Name:         "okta",
				Category:     CategoryOIDC,
				ProviderType: ProviderOkta,
				ClientID:     "okta-client-id",
				AuthUrl:      "https://okta.example.com/oauth2/v1/authorize",
				TokenUrl:     "https://okta.example.com/oauth2/v1/token",
			},
			wantErr: false,
		},
		{
			name: "valid generic OIDC provider discovered from issuer",
			provider: Provider{
				Name:         "oidc",
				Category:     CategoryOIDC,
				ProviderType: ProviderOIDC,
				ClientID:     "oidc-client-id",
				IssuerUrl:    "https://sso.example.com",
			},
			wantErr: false,
		},
		{
			name: "generic OIDC without issuer still requires authUrl",
			provider: Provider{
				Name:         "oidc",
				Category:     CategoryOIDC,
				ProviderType: ProviderOIDC,
				ClientID:     "oidc-client-id",
			},
			wantErr: true,
			errMsg:  "authUrl is required for OAuth/OIDC providers",
		},
		{
			name: "valid SAML provider",
			provider: Provider{
				Name:         "saml-idp",
				Category:     CategoryEnterprise,
				ProviderType: ProviderSAML,
				IdPEntityId:  "https://idp.example.com/entity",
			},
			wantErr: false,
		},
		{
			name: "valid LDAP provider",
			provider: Provider{
				Name:         "ldap-corp",
				Category:     CategoryEnterprise,
				ProviderType: ProviderLDAP,
				Host:         "ldap.example.com",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			provider: Provider{
				Category:     CategoryOAuth,
				ProviderType: ProviderGitHub,
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "category type mismatch",
			provider: Provider{
				Name:         "mismatch",
				Category:     CategorySAML,
				ProviderType: ProviderGitHub, // GitHub maps to OAuth
			},
			wantErr: true,
			errMsg:  "does not match type",
		},
		{
			name: "OAuth missing clientId",
			provider: Provider{
				Name:         "github",
				Category:     CategoryOAuth,
				ProviderType: ProviderGitHub,
				AuthUrl:      "https://github.com/login/oauth/authorize",
				TokenUrl:     "https://github.com/login/oauth/access_token",
			},
			wantErr: true,
			errMsg:  "clientId is required for OAuth/OIDC providers",
		},
		{
			name: "OAuth missing authUrl",
			provider: Provider{
				Name:         "github",
				Category:     CategoryOAuth,
				ProviderType: ProviderGitHub,
				ClientID:     "gh-client-id",
				TokenUrl:     "https://github.com/login/oauth/access_token",
			},
			wantErr: true,
			errMsg:  "authUrl is required for OAuth/OIDC providers",
		},
		{
			name: "OAuth missing tokenUrl",
			provider: Provider{
				Name:         "github",
				Category:     CategoryOAuth,
				ProviderType: ProviderGitHub,
				ClientID:     "gh-client-id",
				AuthUrl:      "https://github.com/login/oauth/authorize",
			},
			wantErr: true,
			errMsg:  "tokenUrl is required for OAuth/OIDC providers",
		},
		{
			name: "SAML with empty Category skips consistency and field checks",
			provider: Provider{
				Name:         "saml-idp",
				Category:     "",
				ProviderType: ProviderSAML,
			},
			wantErr: false,
		},
		{
			name: "LDAP with Enterprise category - no LDAP-specific field check triggered",
			provider: Provider{
				Name:         "ldap-corp",
				Category:     CategoryEnterprise,
				ProviderType: ProviderLDAP,
				Host:         "",
			},
			wantErr: false, // Validate() only checks Host under CategoryLDAP, not CategoryEnterprise
		},
		{
			name: "empty category skips category check",
			provider: Provider{
				Name:         "no-category",
				ProviderType: ProviderGitHub,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if err.Error() == "" {
					t.Errorf("Validate() error message is empty")
				}
			}
		})
	}
}

func TestGetProviderCategory(t *testing.T) {
	tests := []struct {
		kind Kind
		want ProviderCategory
	}{
		{Kind("github"), CategoryOAuth},
		{Kind("google"), CategoryOAuth},
		{Kind("azure_ad"), CategoryOAuth},
		{Kind("okta"), CategoryOAuth},
		{Kind("oidc"), CategoryOIDC},
		{Kind("wechat"), CategoryOAuth},
		{Kind("dingtalk"), CategoryOAuth},
		{Kind("ldap"), CategoryLDAP},
		{Kind("saml"), CategorySAML},
		{Kind("metamask"), CategoryWeb3},
		{Kind("alipay"), CategoryOAuth},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			got, err := GetProviderCategory(tt.kind)
			if err != nil {
				t.Fatalf("GetProviderCategory(%q) error = %v", tt.kind, err)
			}
			if got != tt.want {
				t.Errorf("GetProviderCategory(%q) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}

	if _, err := GetProviderCategory(Kind("unknown")); err == nil {
		t.Error("GetProviderCategory(unknown) must return an error instead of defaulting to OAuth")
	}
}

func TestGetSupportedProviderTypes(t *testing.T) {
	types := GetSupportedProviderTypes()
	if len(types) < 30 {
		t.Errorf("len(GetSupportedProviderTypes()) = %d, want at least 30", len(types))
	}
}

func TestProviderType_Values(t *testing.T) {
	// Verify specific type values
	if ProviderGitHub != "github" {
		t.Errorf("ProviderGitHub = %q, want %q", ProviderGitHub, "github")
	}
	if ProviderWeChat != "wechat" {
		t.Errorf("ProviderWeChat = %q, want %q", ProviderWeChat, "wechat")
	}
	if ProviderLDAP != "ldap" {
		t.Errorf("ProviderLDAP = %q, want %q", ProviderLDAP, "ldap")
	}
	if ProviderMetamask != "metamask" {
		t.Errorf("ProviderMetamask = %q, want %q", ProviderMetamask, "metamask")
	}
}

func TestProviderCategory_Values(t *testing.T) {
	if CategoryOAuth != "OAuth" {
		t.Errorf("CategoryOAuth = %q, want %q", CategoryOAuth, "OAuth")
	}
	if CategoryOIDC != "OIDC" {
		t.Errorf("CategoryOIDC = %q, want %q", CategoryOIDC, "OIDC")
	}
	if CategorySAML != "SAML" {
		t.Errorf("CategorySAML = %q, want %q", CategorySAML, "SAML")
	}
	if CategoryWeb3 != "Web3" {
		t.Errorf("CategoryWeb3 = %q, want %q", CategoryWeb3, "Web3")
	}
}

func TestProvider_Fields(t *testing.T) {
	now := time.Now()
	provider := Provider{
		ID:             "019test-provider-id",
		OrganizationID: "019test-org-id",
		Name:           "github",
		DisplayName:    "GitHub",
		Category:       CategoryOAuth,
		ProviderType:   ProviderGitHub,
		ClientID:       "gh-client-id",
		ClientSecret:   "gh-client-secret",
		Scopes:         []string{"user:email", "read:org"},
		Enabled:        true,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	if !provider.Enabled {
		t.Error("Enabled should be true")
	}
	if len(provider.Scopes) != 2 {
		t.Errorf("len(Scopes) = %d, want 2", len(provider.Scopes))
	}
	if provider.Revision != 1 {
		t.Errorf("Revision = %d, want 1", provider.Revision)
	}
}

// ---------------------------------------------------------------------------
// Extended boundary and validation tests
// ---------------------------------------------------------------------------

func TestProvider_Validate_EmailRegex(t *testing.T) {
	valid := Provider{
		Name:         "github",
		Category:     CategoryOAuth,
		ProviderType: ProviderGitHub,
		ClientID:     "gh-client-id",
		AuthUrl:      "https://github.com/login/oauth/authorize",
		TokenUrl:     "https://github.com/login/oauth/access_token",
		EmailRegex:   `^[^@]+@example\.com$`,
	}
	assert.NoError(t, valid.Validate())

	invalid := valid
	invalid.EmailRegex = "["
	err := invalid.Validate()
	appErr, ok := err.(*apperrors.Error)
	assert.True(t, ok)
	assert.Equal(t, 400, appErr.HTTPStatus())
	assert.Contains(t, err.Error(), "emailRegex")
}

func TestProvider_Validate_Boundary(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  bool
	}{
		{"valid OAuth with all fields", Provider{
			Name: "github", Category: CategoryOAuth, ProviderType: ProviderGitHub,
			ClientID: "id", AuthUrl: "https://auth", TokenUrl: "https://token",
		}, false},
		{"valid OIDC provider", Provider{
			Name: "okta", Category: CategoryOIDC, ProviderType: ProviderOkta,
			ClientID: "id", AuthUrl: "https://okta/auth", TokenUrl: "https://okta/token",
		}, false},
		{"valid SAML provider", Provider{
			Name: "saml", Category: CategoryEnterprise, ProviderType: ProviderSAML,
			IdPEntityId: "https://idp.example.com",
		}, false},
		{"valid LDAP provider", Provider{
			Name: "ldap", Category: CategoryEnterprise, ProviderType: ProviderLDAP,
			Host: "ldap.example.com",
		}, false},
		{"valid Web3 provider", Provider{
			Name: "metamask", Category: CategoryWeb3, ProviderType: ProviderMetamask,
		}, false},
		{"valid Payment provider", Provider{
			Name: "alipay", Category: CategoryPayment, ProviderType: ProviderAlipay,
		}, false},
		{"missing name", Provider{Category: CategoryOAuth, ProviderType: ProviderGitHub}, true},
		{"category type mismatch", Provider{
			Name: "mismatch", Category: CategorySAML, ProviderType: ProviderGitHub,
		}, true},
		{"OAuth missing clientId", Provider{
			Name: "gh", Category: CategoryOAuth, ProviderType: ProviderGitHub,
			AuthUrl: "https://auth", TokenUrl: "https://token",
		}, true},
		{"OAuth missing authUrl", Provider{
			Name: "gh", Category: CategoryOAuth, ProviderType: ProviderGitHub,
			ClientID: "id", TokenUrl: "https://token",
		}, true},
		{"OAuth missing tokenUrl", Provider{
			Name: "gh", Category: CategoryOAuth, ProviderType: ProviderGitHub,
			ClientID: "id", AuthUrl: "https://auth",
		}, true},
		{"SAML missing idpEntityId with CategorySAML", Provider{
			Name: "saml", Category: CategorySAML, ProviderType: ProviderSAML,
		}, true},
		{"LDAP missing host with CategoryLDAP", Provider{
			Name: "ldap", Category: CategoryLDAP, ProviderType: ProviderLDAP,
		}, true},
		{"LDAP with Enterprise category - no LDAP-specific field check", Provider{
			Name: "ldap", Category: CategoryEnterprise, ProviderType: ProviderLDAP,
			Host: "",
		}, false},
		{"empty category skips consistency check", Provider{
			Name: "any", ProviderType: ProviderGitHub,
		}, false},
		{"Chinese social provider WeChat", Provider{
			Name: "wechat", Category: CategorySocial, ProviderType: ProviderWeChat,
		}, false},
		{"Chinese social provider DingTalk", Provider{
			Name: "dingtalk", Category: CategorySocial, ProviderType: ProviderDingTalk,
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetProviderCategory_AllTypes(t *testing.T) {
	tests := []struct {
		kind Kind
		want ProviderCategory
	}{
		// OAuth
		{Kind("github"), CategoryOAuth},
		{Kind("google"), CategoryOAuth},
		{Kind("facebook"), CategoryOAuth},
		{Kind("twitter"), CategoryOAuth},
		{Kind("gitlab"), CategoryOAuth},
		{Kind("gitee"), CategoryOAuth},
		{Kind("linkedin"), CategoryOAuth},
		// OIDC
		{Kind("azure_ad"), CategoryOAuth},
		{Kind("azure_ad_b2c"), CategoryOAuth},
		{Kind("okta"), CategoryOAuth},
		{Kind("oidc"), CategoryOIDC},
		// Social
		{Kind("wechat"), CategoryOAuth},
		{Kind("wechat_mobile"), CategoryOAuth},
		{Kind("wechat_miniprogram"), CategoryOAuth},
		{Kind("wecom_internal"), CategoryOAuth},
		{Kind("wecom_third"), CategoryOAuth},
		{Kind("dingtalk"), CategoryOAuth},
		{Kind("lark"), CategoryOAuth},
		{Kind("feishu"), CategoryOAuth},
		{Kind("qq"), CategoryOAuth},
		{Kind("weibo"), CategoryOAuth},
		{Kind("baidu"), CategoryOAuth},
		{Kind("douyin"), CategoryOAuth},
		{Kind("kwai"), CategoryOAuth},
		{Kind("bilibili"), CategoryOAuth},
		{Kind("infoflow_internal"), CategoryOAuth},
		{Kind("infoflow_third"), CategoryOAuth},
		{Kind("telegram"), CategoryOAuth},
		{Kind("goth"), CategoryOAuth},
		// Enterprise
		{Kind("ldap"), CategoryLDAP},
		{Kind("adfs"), CategoryOAuth},
		{Kind("saml"), CategorySAML},
		{Kind("custom"), CategoryOAuth}, // Custom accepts every declared category
		// Web3
		{Kind("metamask"), CategoryWeb3},
		{Kind("web3onboard"), CategoryWeb3},
		// Payment (identity login rows keep OAuth as canonical)
		{Kind("alipay"), CategoryOAuth},
		{Kind("wechat_pay"), CategoryOAuth},
		// Email / SMS / MFA / Scan / IDV / Log
		{Kind("sendgrid"), CategoryEmail},
		{Kind("alibaba_sms"), CategorySMS},
		{Kind("totp"), CategoryMFA},
		{Kind("wechat_scan"), CategoryScan},
		{Kind("jumio"), CategoryIDVerification},
		{Kind("elasticsearch"), CategoryLog},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			got, err := GetProviderCategory(tt.kind)
			if err != nil {
				t.Fatalf("GetProviderCategory(%q) error = %v", tt.kind, err)
			}
			if got != tt.want {
				t.Errorf("GetProviderCategory(%q) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestProvider_SecretRotation(t *testing.T) {
	p := Provider{
		Name:          "github",
		Category:      CategoryOAuth,
		ProviderType:  ProviderGitHub,
		ClientID:      "id-1",
		ClientSecret:  "secret-1",
		ClientID2:     "id-2",
		ClientSecret2: "secret-2",
	}
	assert.Equal(t, "id-1", p.ClientID)
	assert.Equal(t, "secret-1", p.ClientSecret)
	assert.Equal(t, "id-2", p.ClientID2)
	assert.Equal(t, "secret-2", p.ClientSecret2)
}

func TestProvider_SAML_Fields(t *testing.T) {
	p := Provider{
		Name:         "saml-idp",
		Category:     CategoryEnterprise,
		ProviderType: ProviderSAML,
		IdPEntityId:  "https://idp.example.com/entity",
		SsoUrl:       "https://idp.example.com/sso",
		SloUrl:       "https://idp.example.com/slo",
		Metadata:     "<EntityDescriptor>...</EntityDescriptor>",
		SigningAlg:   "SHA256",
	}
	assert.Equal(t, "https://idp.example.com/entity", p.IdPEntityId)
	assert.NotEmpty(t, p.SsoUrl)
	assert.NotEmpty(t, p.SloUrl)
}

func TestProvider_LDAP_Fields(t *testing.T) {
	p := Provider{
		Name:         "ldap-corp",
		Category:     CategoryLDAP,
		ProviderType: ProviderLDAP,
		Host:         "ldap.example.com",
		Port:         389,
		Admin:        "cn=admin,dc=example,dc=com",
		Password:     "admin-password",
	}
	assert.Equal(t, "ldap.example.com", p.Host)
	assert.Equal(t, 389, p.Port)
}

func TestProvider_EnabledDisabled(t *testing.T) {
	p := Provider{Name: "test", Enabled: true}
	assert.True(t, p.Enabled)
	p.Enabled = false
	assert.False(t, p.Enabled)
}

func TestProviderConfig_Fields(t *testing.T) {
	now := time.Now()
	config := ProviderConfig{
		ProviderID: "prov-1",
		ConfigType: "oauth",
		Config:     map[string]any{"clientId": "abc", "clientSecret": "xyz"},
		UpdatedAt:  now,
	}
	assert.Equal(t, "prov-1", config.ProviderID)
	assert.Len(t, config.Config, 2)
}

func TestProvider_Validate_CategoryAnswers422(t *testing.T) {
	p := &Provider{Name: "prov", ProviderType: ProviderType("github"), Category: CategorySAML}
	appErr := apperrors.FromError(p.Validate())
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "category", appErr.Meta["field"])
}
