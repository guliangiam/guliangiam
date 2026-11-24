package idp

import "fmt"

// AzureADB2CProvider implements IdP authentication via Azure AD B2C.
// Unlike regular Azure AD which uses /common/ endpoints, Azure AD B2C uses
// tenant-specific endpoints with user flow (policy) paths.
type AzureADB2CProvider struct {
	BaseOAuthProvider
	tenant   string
	userFlow string
}

// NewAzureADB2CProvider creates a new Azure AD B2C OAuth2 provider.
// The Host field in OAuthConfig is used as the tenant name.
// The Scopes field is used to pass the user flow (policy) name as the first element.
func NewAzureADB2CProvider(config *OAuthConfig) *AzureADB2CProvider {
	tenant := config.Host
	userFlow := "B2C_1_signupsignin" // default user flow
	if len(config.Scopes) > 0 {
		userFlow = config.Scopes[0]
		config.Scopes = []string{"openid", "email"}
	}

	authURL := fmt.Sprintf("https://%s.b2clogin.com/%s.onmicrosoft.com/%s/oauth2/v2.0/authorize",
		tenant, tenant, userFlow)
	tokenURL := fmt.Sprintf("https://%s.b2clogin.com/%s.onmicrosoft.com/%s/oauth2/v2.0/token",
		tenant, tenant, userFlow)
	userInfoURL := fmt.Sprintf("https://%s.b2clogin.com/%s.onmicrosoft.com/%s/openid/v2.0/userinfo",
		tenant, tenant, userFlow)

	return &AzureADB2CProvider{
		BaseOAuthProvider: newBaseOAuth(config, authURL, tokenURL, userInfoURL),
		tenant:            tenant,
		userFlow:          userFlow,
	}
}
