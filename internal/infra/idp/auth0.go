package idp

import "strings"

// Auth0Provider implements IdP authentication via Auth0 OAuth2.
// Uses config.Host to construct the OAuth endpoints for the Auth0 tenant.
type Auth0Provider struct {
	BaseOAuthProvider
}

// NewAuth0Provider creates a new Auth0 OAuth2 provider.
// The Host field in config must be set to the Auth0 tenant domain (e.g. "https://dev-123456.us.auth0.com").
func NewAuth0Provider(config *OAuthConfig) *Auth0Provider {
	host := strings.TrimRight(config.Host, "/")

	return &Auth0Provider{
		BaseOAuthProvider: newBaseOAuth(config,
			host+"/authorize",
			host+"/oauth/token",
			host+"/userinfo",
		),
	}
}
