package idp

// InfoFlowProvider implements IdP authentication via InfoFlow (QD) OAuth2.
// Similar to WeCom but uses different endpoint URLs.
type InfoFlowProvider struct {
	BaseOAuthProvider
}

// NewInfoFlowProvider creates a new InfoFlow OAuth2 provider.
//
// 2026-11-14 OPEN-API-75 P4 裁定：InfoFlow（腾讯资讯流企业微信代接，qy.qq.com）
// 无公开官方开发者文档，端点承袭 Casdoor 历史实现，无法按 URL 复核；维持现状，
// 真实租户接入前需在开放项登记复核证据（见 open-items.yaml OPEN-API-75 裁定记录）。
func NewInfoFlowProvider(config *OAuthConfig) *InfoFlowProvider {
	return &InfoFlowProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://qy.qq.com/open/wework/3rd_qr_connect",
			"https://qy.qq.com/cgi-bin/service/get_provider_token",
			"https://qy.qq.com/cgi-bin/service/get_login_info",
		),
	}
}
