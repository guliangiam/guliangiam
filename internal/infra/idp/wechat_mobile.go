package idp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WeChatMobileProvider implements WeChat Mobile (in-app browser) OAuth2 login.
// This uses the snsapi_userinfo scope for mobile authorization,
// which allows fetching the user's profile info.
type WeChatMobileProvider struct {
	BaseOAuthProvider
}

// NewWeChatMobileProvider creates a new WeChat Mobile OAuth2 provider.
func NewWeChatMobileProvider(config *OAuthConfig) *WeChatMobileProvider {
	if len(config.Scopes) == 0 {
		config.Scopes = []string{"snsapi_userinfo"}
	}
	return &WeChatMobileProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://open.weixin.qq.com/connect/oauth2/authorize",
			"https://api.weixin.qq.com/sns/oauth2/access_token",
			"https://api.weixin.qq.com/sns/userinfo",
		),
	}
}

// GetRedirectURL returns the WeChat Mobile authorization URL with appid and snsapi_userinfo scope.
func (p *WeChatMobileProvider) GetRedirectURL(state string) (string, error) {
	u, err := url.Parse(p.authURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}

	q := u.Query()
	q.Set("appid", p.config.ClientID)
	q.Set("redirect_uri", p.config.RedirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_userinfo")
	q.Set("state", state)
	u.RawQuery = q.Encode()

	return u.String() + "#wechat_redirect", nil
}

// wechatAccessToken represents the WeChat access token response.
type wechatAccessToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid"`
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
}

// ExchangeCode exchanges the authorization code for a WeChat Mobile token.
// The token endpoint is a GET with appid/secret/code query parameters; the
// openid/unionid it returns are required by the userinfo step.
func (p *WeChatMobileProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	u, err := url.Parse(p.tokenURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token URL")
	}
	q := u.Query()
	q.Set("appid", p.config.ClientID)
	q.Set("secret", p.config.ClientSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")
	u.RawQuery = q.Encode()

	body, status, err := p.requestJSON(ctx, http.MethodGet, u.String(), http.NoBody, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("wechat token exchange failed (status %d): %s", status, string(body))
	}

	var wechatToken wechatAccessToken
	if err := json.Unmarshal(body, &wechatToken); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}

	if wechatToken.ErrCode != 0 {
		return nil, apperrors.NewUnauthorizedf("wechat token error (code %d): %s", wechatToken.ErrCode, wechatToken.ErrMsg)
	}
	if wechatToken.AccessToken == "" {
		return nil, apperrors.NewUnauthorized("wechat token response did not include an access token")
	}

	id := wechatToken.UnionID
	if id == "" {
		id = wechatToken.OpenID
	}
	return &OAuthToken{
		AccessToken:  wechatToken.AccessToken,
		RefreshToken: wechatToken.RefreshToken,
		ExpiresIn:    wechatToken.ExpiresIn,
		Scope:        wechatToken.Scope,
		ID:           id,
		OpenID:       wechatToken.OpenID,
		UnionID:      wechatToken.UnionID,
	}, nil
}

// wechatUserInfo represents the WeChat user info response.
type wechatUserInfo struct {
	OpenID     string   `json:"openid"`
	Nickname   string   `json:"nickname"`
	Sex        int      `json:"sex"`
	Province   string   `json:"province"`
	City       string   `json:"city"`
	Country    string   `json:"country"`
	HeadImgURL string   `json:"headimgurl"`
	Privilege  []string `json:"privilege"`
	UnionID    string   `json:"unionid"`
	ErrCode    int      `json:"errcode"`
	ErrMsg     string   `json:"errmsg"`
}

// GetUserInfo retrieves the user information using the WeChat Mobile access token.
// The userinfo endpoint requires the openid captured at token exchange.
func (p *WeChatMobileProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	openid := token.OpenID
	if openid == "" {
		// Legacy tokens carried the openid inside the scope string.
		openid = extractOpenIDFromToken(token)
	}
	if openid == "" {
		return nil, apperrors.NewUnauthorized("wechat token did not include an openid")
	}

	u, err := url.Parse(p.userURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info URL")
	}
	q := u.Query()
	q.Set("access_token", token.AccessToken)
	q.Set("openid", openid)
	q.Set("lang", "zh_CN")
	u.RawQuery = q.Encode()

	body, status, err := p.requestJSON(ctx, http.MethodGet, u.String(), http.NoBody, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("get user info failed (status %d): %s", status, string(body))
	}

	var wechatUser wechatUserInfo
	if err := json.Unmarshal(body, &wechatUser); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}
	if wechatUser.ErrCode != 0 {
		return nil, apperrors.NewUnauthorizedf("wechat user info error (code %d): %s", wechatUser.ErrCode, wechatUser.ErrMsg)
	}

	// Use UnionID as primary ID if available, otherwise OpenID
	id := wechatUser.UnionID
	if id == "" {
		id = wechatUser.OpenID
	}

	return &UserInfo{
		ID:          id,
		Username:    wechatUser.Nickname,
		DisplayName: wechatUser.Nickname,
		AvatarURL:   wechatUser.HeadImgURL,
		OpenID:      wechatUser.OpenID,
		UnionID:     wechatUser.UnionID,
	}, nil
}

// extractOpenIDFromToken attempts to extract OpenID from token data.
// For WeChat Mobile, the OpenID is returned in the token exchange response.
func extractOpenIDFromToken(token *OAuthToken) string {
	// The OpenID may be encoded in the scope or available via a separate call.
	// In practice, the caller should pass the OpenID alongside the token.
	// For now, we make a best-effort extraction.
	if token.Scope != "" && strings.Contains(token.Scope, "openid") {
		parts := strings.Split(token.Scope, " ")
		if len(parts) > 1 {
			return parts[1]
		}
	}
	return ""
}
