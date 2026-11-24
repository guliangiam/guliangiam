package idp

import "strings"

// CasdoorProvider implements OAuth2 login against another Casdoor instance.
// Endpoints are derived from the issuer/host, and explicit auth/token/userinfo
// URLs win when the admin configured them.
type CasdoorProvider struct {
	BaseOAuthProvider
}

func NewCasdoorProvider(config *OAuthConfig) *CasdoorProvider {
	if config == nil {
		config = &OAuthConfig{}
	}
	host := strings.TrimRight(config.Host, "/")
	return &CasdoorProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			firstNonEmpty(config.AuthURL, joinHostPath(host, "/login/oauth/authorize")),
			firstNonEmpty(config.TokenURL, joinHostPath(host, "/api/login/oauth/access_token")),
			firstNonEmpty(config.UserInfoURL, joinHostPath(host, "/api/userinfo")),
		),
	}
}

func joinHostPath(host, path string) string {
	if host == "" {
		return ""
	}
	return host + path
}
