package idp

// GitHubProvider implements IdP authentication via GitHub OAuth2.
type GitHubProvider struct {
	BaseOAuthProvider
}

// NewGitHubProvider creates a new GitHub OAuth2 provider.
func NewGitHubProvider(config *OAuthConfig) *GitHubProvider {
	return &GitHubProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://github.com/login/oauth/authorize",
			"https://github.com/login/oauth/access_token",
			"https://api.github.com/user",
		),
	}
}
