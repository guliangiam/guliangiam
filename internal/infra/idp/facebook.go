package idp

// FacebookProvider implements IdP authentication via Facebook OAuth2.
type FacebookProvider struct {
	BaseOAuthProvider
}

// NewFacebookProvider creates a new Facebook OAuth2 provider.
func NewFacebookProvider(config *OAuthConfig) *FacebookProvider {
	return &FacebookProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://www.facebook.com/v21.0/dialog/oauth",
			"https://graph.facebook.com/v21.0/oauth/access_token",
			"https://graph.facebook.com/me?fields=id,name,email,picture",
		),
	}
}
