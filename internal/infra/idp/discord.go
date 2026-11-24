package idp

// DiscordProvider implements IdP authentication via Discord OAuth2.
type DiscordProvider struct {
	BaseOAuthProvider
}

// NewDiscordProvider creates a new Discord OAuth2 provider.
func NewDiscordProvider(config *OAuthConfig) *DiscordProvider {
	return &DiscordProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://discord.com/api/oauth2/authorize",
			"https://discord.com/api/oauth2/token",
			"https://discord.com/api/users/@me",
		),
	}
}
