package idp

import "strings"

// OktaProvider implements IdP authentication via Okta OAuth2.
// Uses config.Host to construct the OAuth endpoints for the Okta domain.
type OktaProvider struct {
	BaseOAuthProvider
}

// NewOktaProvider creates a new Okta OAuth2 provider.
// The Host field in config must be set to the Okta domain (e.g. "https://dev-123456.okta.com").
func NewOktaProvider(config *OAuthConfig) *OktaProvider {
	host := strings.TrimRight(config.Host, "/")

	return &OktaProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			host+"/oauth2/v1/authorize",
			host+"/oauth2/v1/token",
			host+"/oauth2/v1/userinfo",
		),
	}
}
