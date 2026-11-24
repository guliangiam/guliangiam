package idp

import "strings"

// GitLabProvider implements IdP authentication via GitLab OAuth2.
// Supports custom Host for self-hosted GitLab instances.
type GitLabProvider struct {
	BaseOAuthProvider
}

// NewGitLabProvider creates a new GitLab OAuth2 provider.
// If config.Host is set, it will be used instead of gitlab.com.
func NewGitLabProvider(config *OAuthConfig) *GitLabProvider {
	host := "https://gitlab.com"
	if config.Host != "" {
		host = strings.TrimRight(config.Host, "/")
	}

	return &GitLabProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			host+"/oauth/authorize",
			host+"/oauth/token",
			host+"/api/v4/user",
		),
	}
}
