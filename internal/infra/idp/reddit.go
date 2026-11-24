package idp

// RedditProvider implements IdP authentication via Reddit OAuth2.
type RedditProvider struct {
	BaseOAuthProvider
}

// NewRedditProvider creates a new Reddit OAuth2 provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：三个端点已按官方文档复核
// (https://www.reddit.com/dev/api/oauth/) — authorize 走 www.reddit.com，
// token 需 Basic 认证，userinfo 必须走 oauth.reddit.com 域。
func NewRedditProvider(config *OAuthConfig) *RedditProvider {
	return &RedditProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://www.reddit.com/api/v1/authorize",
			"https://www.reddit.com/api/v1/access_token",
			"https://oauth.reddit.com/api/v1/me",
		),
	}
}
