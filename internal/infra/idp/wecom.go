package idp

// WeComProvider implements IdP authentication via WeCom (WeChat Work) OAuth2.
type WeComProvider struct {
	BaseOAuthProvider
}

// NewWeComProvider creates a new WeCom OAuth2 provider.
func NewWeComProvider(config *OAuthConfig) *WeComProvider {
	return &WeComProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://open.work.weixin.qq.com/wwopen/sso/3rd_qrConnect",
			"https://qyapi.weixin.qq.com/cgi-bin/service/get_provider_token",
			"https://qyapi.weixin.qq.com/cgi-bin/service/get_login_info",
		),
	}
}
