package idp

// LinkedInProvider implements IdP authentication via LinkedIn OAuth2.
type LinkedInProvider struct {
	BaseOAuthProvider
}

// NewLinkedInProvider creates a new LinkedIn OAuth2 provider.
func NewLinkedInProvider(config *OAuthConfig) *LinkedInProvider {
	return &LinkedInProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://www.linkedin.com/oauth/v2/authorization",
			"https://www.linkedin.com/oauth/v2/accessToken",
			"https://api.linkedin.com/v2/userinfo",
		),
	}
}
