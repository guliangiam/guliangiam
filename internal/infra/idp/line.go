package idp

// LineProvider implements IdP authentication via LINE OAuth2.
type LineProvider struct {
	BaseOAuthProvider
}

// NewLineProvider creates a new LINE OAuth2 provider.
func NewLineProvider(config *OAuthConfig) *LineProvider {
	return &LineProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://access.line.me/oauth2/v2.1/authorize",
			"https://api.line.me/oauth2/v2.1/token",
			"https://api.line.me/v2/profile",
		),
	}
}
