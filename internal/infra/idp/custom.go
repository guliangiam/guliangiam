package idp

// CustomOAuthProvider implements IdP authentication via a user-defined OAuth2 provider.
// It uses the AuthURL, TokenURL, and UserInfoURL from the config directly.
type CustomOAuthProvider struct {
	BaseOAuthProvider
}

// NewCustomOAuthProvider creates a new custom OAuth2 provider using config URLs.
func NewCustomOAuthProvider(config *OAuthConfig) *CustomOAuthProvider {
	return &CustomOAuthProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			config.AuthURL,
			config.TokenURL,
			config.UserInfoURL,
		),
	}
}
