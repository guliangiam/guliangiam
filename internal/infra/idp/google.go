package idp

// GoogleProvider implements IdP authentication via Google OAuth2.
type GoogleProvider struct {
	BaseOAuthProvider
}

// NewGoogleProvider creates a new Google OAuth2 provider.
func NewGoogleProvider(config *OAuthConfig) *GoogleProvider {
	return &GoogleProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://accounts.google.com/o/oauth2/v2/auth",
			"https://oauth2.googleapis.com/token",
			"https://www.googleapis.com/oauth2/v2/userinfo",
		),
	}
}
