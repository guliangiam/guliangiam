package idp

// AzureADProvider implements IdP authentication via Azure Active Directory OAuth2.
type AzureADProvider struct {
	BaseOAuthProvider
}

// NewAzureADProvider creates a new Azure AD OAuth2 provider.
func NewAzureADProvider(config *OAuthConfig) *AzureADProvider {
	return &AzureADProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			"https://login.microsoftonline.com/common/oauth2/v2.0/token",
			"https://graph.microsoft.com/oidc/userinfo",
		),
	}
}
