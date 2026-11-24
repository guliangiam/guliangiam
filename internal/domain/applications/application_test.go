package applications

import (
	"testing"
)

func TestApplication_AppTypeValues(t *testing.T) {
	types := []string{"web", "native", "spa", "machine", "saml", "cas"}
	for _, typ := range types {
		app := Application{AppType: typ}
		if app.AppType != typ {
			t.Errorf("AppType = %q, want %q", app.AppType, typ)
		}
	}
}

func TestApplication_GrantTypes(t *testing.T) {
	app := Application{
		ID:         "app-1",
		GrantTypes: []string{"authorization_code", "refresh_token", "client_credentials"},
	}
	if len(app.GrantTypes) != 3 {
		t.Errorf("len(GrantTypes) = %d, want 3", len(app.GrantTypes))
	}
}

func TestApplication_RedirectURIs(t *testing.T) {
	app := Application{
		ID:           "app-1",
		RedirectURIs: []string{"https://example.com/callback", "http://localhost:3000/callback"},
	}
	if len(app.RedirectURIs) != 2 {
		t.Errorf("len(RedirectURIs) = %d, want 2", len(app.RedirectURIs))
	}
}

func TestApplication_TokenAuthMethod(t *testing.T) {
	app := Application{
		ID:                      "app-1",
		TokenEndpointAuthMethod: TokenEndpointAuthMethodClientSecretBasic,
	}
	if app.TokenEndpointAuthMethod != "client_secret_basic" {
		t.Errorf("TokenEndpointAuthMethod = %q, want %q", app.TokenEndpointAuthMethod, "client_secret_basic")
	}
}

func TestApplication_DisableSignin(t *testing.T) {
	app := Application{ID: "app-1", DisableSignin: true}
	if !app.DisableSignin {
		t.Error("DisableSignin should be true")
	}
	app.DisableSignin = false
	if app.DisableSignin {
		t.Error("DisableSignin should be false")
	}
}

func TestApplication_EffectiveTokenEndpointAuthMethod(t *testing.T) {
	tests := []struct {
		name     string
		app      Application
		expected string
	}{
		{"explicit none", Application{TokenEndpointAuthMethod: "none"}, TokenEndpointAuthMethodNone},
		{"explicit client_secret_basic", Application{TokenEndpointAuthMethod: "client_secret_basic"}, TokenEndpointAuthMethodClientSecretBasic},
		{"explicit client_secret_post", Application{TokenEndpointAuthMethod: "client_secret_post"}, TokenEndpointAuthMethodClientSecretPost},
		{"empty method with secret defaults to basic", Application{TokenEndpointAuthMethod: "", ClientSecretHash: "hash123"}, TokenEndpointAuthMethodClientSecretBasic},
		{"empty method no secret defaults to none", Application{TokenEndpointAuthMethod: "", ClientSecretHash: ""}, TokenEndpointAuthMethodNone},
		{"case insensitive", Application{TokenEndpointAuthMethod: "NONE"}, TokenEndpointAuthMethodNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.app.EffectiveTokenEndpointAuthMethod()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestApplication_IsConfidential(t *testing.T) {
	confidential := Application{TokenEndpointAuthMethod: "client_secret_basic"}
	if !confidential.IsConfidential() {
		t.Error("expected confidential app")
	}
	public := Application{TokenEndpointAuthMethod: "none"}
	if public.IsConfidential() {
		t.Error("expected public app")
	}
}

func TestApplication_RequiresClientSecret(t *testing.T) {
	app := Application{TokenEndpointAuthMethod: "client_secret_post"}
	if !app.RequiresClientSecret() {
		t.Error("expected to require client secret")
	}
	app = Application{TokenEndpointAuthMethod: "none"}
	if app.RequiresClientSecret() {
		t.Error("expected not to require client secret")
	}
}

// ---------------------------------------------------------------------------
// Extended validation and boundary tests
// ---------------------------------------------------------------------------

func TestApplication_ClientIDFormat(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
	}{
		{"standard UUID", "01923e51-7b68-7df3-8e94-0a1b2c3d4e5f"},
		{"short alphanumeric", "app-abc123"},
		{"empty string", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := Application{ID: "app-1", ClientID: tt.clientID}
			if app.ClientID != tt.clientID {
				t.Errorf("ClientID = %q, want %q", app.ClientID, tt.clientID)
			}
		})
	}
}

func TestApplication_RedirectURI_Boundary(t *testing.T) {
	tests := []struct {
		name    string
		uris    []string
		wantLen int
	}{
		{"empty slice", []string{}, 0},
		{"nil slice", nil, 0},
		{"single HTTPS URI", []string{"https://example.com/callback"}, 1},
		{"multiple URIs", []string{"https://example.com/cb", "http://localhost:3000/cb"}, 2},
		{"URI with query params", []string{"https://example.com/cb?state=abc"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := Application{ID: "app-1", RedirectURIs: tt.uris}
			if len(app.RedirectURIs) != tt.wantLen {
				t.Errorf("len(RedirectURIs) = %d, want %d", len(app.RedirectURIs), tt.wantLen)
			}
		})
	}
}

func TestApplication_GrantTypes_Boundary(t *testing.T) {
	validGrantTypes := map[string]bool{
		"authorization_code": true,
		"implicit":           true,
		"client_credentials": true,
		"refresh_token":      true,
		"password":           true,
		"urn:ietf:params:oauth:grant-type:device_code":    true,
		"urn:ietf:params:oauth:grant-type:token-exchange": true,
	}

	tests := []struct {
		name       string
		grantTypes []string
	}{
		{"single authorization_code", []string{"authorization_code"}},
		{"code + refresh", []string{"authorization_code", "refresh_token"}},
		{"client_credentials only", []string{"client_credentials"}},
		{"empty grant types", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := Application{ID: "app-1", GrantTypes: tt.grantTypes}
			for _, gt := range app.GrantTypes {
				if !validGrantTypes[gt] {
					t.Errorf("unexpected grant type: %q", gt)
				}
			}
		})
	}
}

func TestApplication_EffectiveTokenEndpointAuthMethod_Boundary(t *testing.T) {
	tests := []struct {
		name     string
		app      Application
		expected string
	}{
		{"whitespace-only method with secret", Application{TokenEndpointAuthMethod: "  ", ClientSecretHash: "hash"}, TokenEndpointAuthMethodClientSecretBasic},
		{"mixed-case Client_Secret_Basic", Application{TokenEndpointAuthMethod: "Client_Secret_Basic"}, TokenEndpointAuthMethodClientSecretBasic},
		{"method with surrounding whitespace", Application{TokenEndpointAuthMethod: " none "}, TokenEndpointAuthMethodNone},
		{"unknown method defaults based on secret", Application{TokenEndpointAuthMethod: "unknown", ClientSecretHash: ""}, TokenEndpointAuthMethodNone},
		{"unknown method with secret defaults to basic", Application{TokenEndpointAuthMethod: "unknown", ClientSecretHash: "abc"}, TokenEndpointAuthMethodClientSecretBasic},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.app.EffectiveTokenEndpointAuthMethod()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestApplication_SignupItems(t *testing.T) {
	app := Application{
		ID: "app-1",
		SignupItems: []SignupItem{
			{Name: "email", Visible: true, Required: true},
			{Name: "phone", Visible: true, Required: false, Regex: `^\+?\d{10,15}$`},
			{Name: "username", Visible: false, Prompted: true},
		},
	}
	if len(app.SignupItems) != 3 {
		t.Errorf("len(SignupItems) = %d, want 3", len(app.SignupItems))
	}
	if !app.SignupItems[0].Required {
		t.Error("first SignupItem should be required")
	}
	if app.SignupItems[1].Regex == "" {
		t.Error("second SignupItem should have regex")
	}
}

func TestApplication_ProviderItems(t *testing.T) {
	app := Application{
		ID: "app-1",
		ProviderItems: []ProviderItem{
			{Provider: "github", CanSignUp: true, CanSignIn: true, CanUnlink: true},
			{Provider: "google", CanSignUp: true, CanSignIn: false, CanUnlink: false},
		},
	}
	if len(app.ProviderItems) != 2 {
		t.Errorf("len(ProviderItems) = %d, want 2", len(app.ProviderItems))
	}
	if app.ProviderItems[0].Provider != "github" {
		t.Errorf("ProviderItems[0].Provider = %q, want %q", app.ProviderItems[0].Provider, "github")
	}
}

func TestApplication_TokenExpiry(t *testing.T) {
	tests := []struct {
		name             string
		expireInHours    int
		refreshExpireHrs int
	}{
		{"default expiry", 24, 720},
		{"custom expiry", 1, 48},
		{"zero expiry", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := Application{
				ID:                        "app-1",
				ExpireInHours:             tt.expireInHours,
				RefreshTokenExpireInHours: tt.refreshExpireHrs,
			}
			if app.ExpireInHours != tt.expireInHours {
				t.Errorf("ExpireInHours = %d, want %d", app.ExpireInHours, tt.expireInHours)
			}
			if app.RefreshTokenExpireInHours != tt.refreshExpireHrs {
				t.Errorf("RefreshTokenExpireInHours = %d, want %d", app.RefreshTokenExpireInHours, tt.refreshExpireHrs)
			}
		})
	}
}

func TestApplication_CustomScopes(t *testing.T) {
	app := Application{
		ID: "app-1",
		CustomScopes: []ApplicationCustomScope{
			{Scope: "reports:read", DisplayName: "Read Reports", Description: "Access to read reports"},
			{Scope: "reports:write", DisplayName: "Write Reports", Description: "Access to write reports"},
		},
	}
	if len(app.CustomScopes) != 2 {
		t.Errorf("len(CustomScopes) = %d, want 2", len(app.CustomScopes))
	}
	if app.CustomScopes[0].Scope != "reports:read" {
		t.Errorf("CustomScopes[0].Scope = %q, want %q", app.CustomScopes[0].Scope, "reports:read")
	}
}

func TestApplication_ProviderBindings(t *testing.T) {
	app := Application{
		ID: "app-1",
		ProviderBindings: []ApplicationProviderBinding{
			{ApplicationID: "app-1", ProviderID: "prov-github"},
			{ApplicationID: "app-1", ProviderID: "prov-google"},
		},
	}
	if len(app.ProviderBindings) != 2 {
		t.Errorf("len(ProviderBindings) = %d, want 2", len(app.ProviderBindings))
	}
}

func TestApplication_SigninMethods(t *testing.T) {
	app := Application{
		ID: "app-1",
		SigninMethods: []string{
			"password",
			"otp",
		},
	}
	if len(app.SigninMethods) != 2 {
		t.Errorf("len(SigninMethods) = %d, want 2", len(app.SigninMethods))
	}
}

func TestApplication_SigninItemConfigs(t *testing.T) {
	app := Application{
		ID: "app-1",
		SigninItems: []SigninItemConfig{
			{Name: "username", Visible: true, Label: "Username", Placeholder: "Enter username"},
			{Name: "password", Visible: true, Label: "Password", Rule: "required"},
		},
	}
	if len(app.SigninItems) != 2 {
		t.Errorf("len(SigninItems) = %d, want 2", len(app.SigninItems))
	}
}
