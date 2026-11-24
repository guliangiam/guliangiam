package idp

// SlackProvider implements IdP authentication via Slack OAuth2.
type SlackProvider struct {
	BaseOAuthProvider
}

// NewSlackProvider creates a new Slack OAuth2 provider.
func NewSlackProvider(config *OAuthConfig) *SlackProvider {
	return &SlackProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://slack.com/oauth/v2/authorize",
			"https://slack.com/api/oauth.v2.access",
			"https://slack.com/api/auth.test",
		),
	}
}
