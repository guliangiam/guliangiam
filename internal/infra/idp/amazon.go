package idp

// AmazonProvider implements IdP authentication via Amazon OAuth2.
type AmazonProvider struct {
	BaseOAuthProvider
}

// NewAmazonProvider creates a new Amazon OAuth2 provider.
func NewAmazonProvider(config *OAuthConfig) *AmazonProvider {
	return &AmazonProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://www.amazon.com/ap/oa",
			"https://api.amazon.com/auth/o2/token",
			"https://api.amazon.com/user/profile",
		),
	}
}
