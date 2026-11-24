package idp

// BitbucketProvider implements IdP authentication via Bitbucket OAuth2.
type BitbucketProvider struct {
	BaseOAuthProvider
}

// NewBitbucketProvider creates a new Bitbucket OAuth2 provider.
func NewBitbucketProvider(config *OAuthConfig) *BitbucketProvider {
	return &BitbucketProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://bitbucket.org/site/oauth2/authorize",
			"https://bitbucket.org/site/oauth2/access_token",
			"https://api.bitbucket.org/2.0/user",
		),
	}
}
