package idp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WeChatProvider implements IdP authentication via WeChat OAuth2.
// WeChat uses "appid" instead of "client_id" in its OAuth flow.
type WeChatProvider struct {
	BaseOAuthProvider
}

// NewWeChatProvider creates a new WeChat OAuth2 provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：qrconnect/sns 端点与 appid 参数名已按官方文档复核
// (https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html)。
func NewWeChatProvider(config *OAuthConfig) *WeChatProvider {
	if config != nil && len(config.Scopes) == 0 {
		config.Scopes = []string{"snsapi_login"}
	}
	return &WeChatProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://open.weixin.qq.com/connect/qrconnect",
			"https://api.weixin.qq.com/sns/oauth2/access_token",
			"https://api.weixin.qq.com/sns/userinfo",
		),
	}
}

// GetRedirectURL returns the WeChat authorization URL with appid instead of client_id.
func (p *WeChatProvider) GetRedirectURL(state string) (string, error) {
	u, err := url.Parse(p.authURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}

	q := u.Query()
	q.Set("appid", p.config.ClientID)
	q.Set("redirect_uri", p.config.RedirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	if len(p.config.Scopes) > 0 {
		q.Set("scope", strings.Join(p.config.Scopes, ","))
	}
	u.RawQuery = q.Encode()

	// WeChat requires appending #wechat_redirect to the URL
	return u.String() + "#wechat_redirect", nil
}

// ExchangeCode exchanges the authorization code for a WeChat token.
// The token endpoint is a GET with appid/secret/code query parameters and
// answers with openid/unionid plus errcode/errmsg instead of OAuth error
// fields.
// https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html
func (p *WeChatProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	u, err := url.Parse(p.tokenURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token URL")
	}

	q := u.Query()
	q.Set("grant_type", "authorization_code")
	q.Set("appid", p.config.ClientID)
	q.Set("secret", p.config.ClientSecret)
	q.Set("code", code)
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

// GetUserInfo fetches the profile with the openid captured at token exchange.
// The sns/userinfo endpoint requires access_token and openid as query
// parameters and reports failures through errcode in a 200 body.
func (p *WeChatProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	openid := token.OpenID
	if openid == "" {
		openid = token.ID
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
