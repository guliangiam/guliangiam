package idp

// GiteeProvider implements IdP authentication via Gitee OAuth2.
type GiteeProvider struct {
	BaseOAuthProvider
}

// NewGiteeProvider creates a new Gitee OAuth2 provider.
func NewGiteeProvider(config *OAuthConfig) *GiteeProvider {
	return &GiteeProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://gitee.com/oauth/authorize",
			"https://gitee.com/oauth/token",
			"https://gitee.com/api/v5/user",
		),
	}
}
