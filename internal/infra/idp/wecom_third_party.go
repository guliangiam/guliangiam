package idp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WeComThirdPartyProvider implements WeCom Third-party (service provider) login.
// Third-party providers use a different flow from internal applications:
// 1. Get provider_access_token using corpid/provider_secret via POST
// 2. Get login info using provider_access_token and auth_code via POST
type WeComThirdPartyProvider struct {
	BaseOAuthProvider
}

// NewWeComThirdPartyProvider creates a new WeCom Third-party service provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：端点已按官方开发者文档复核
// (https://developer.work.weixin.qq.com — 代开发/第三方应用 3rd_qrConnect、
// service/get_provider_token、service/get_login_info)。
func NewWeComThirdPartyProvider(config *OAuthConfig) *WeComThirdPartyProvider {
	return &WeComThirdPartyProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://open.work.weixin.qq.com/wwopen/sso/3rd_qrConnect",
			"https://qyapi.weixin.qq.com/cgi-bin/service/get_provider_token",
			"https://qyapi.weixin.qq.com/cgi-bin/service/get_login_info",
		),
	}
}

// GetRedirectURL returns the WeCom Third-party authorization URL.
func (p *WeComThirdPartyProvider) GetRedirectURL(state string) (string, error) {
	u, err := url.Parse(p.authURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}

	q := u.Query()
	q.Set("appid", p.config.ClientID)
	q.Set("redirect_uri", p.config.RedirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// weComProviderToken represents the WeCom provider token response.
type weComProviderToken struct {
	ErrCode             int    `json:"errcode"`
	ErrMsg              string `json:"errmsg"`
	ProviderAccessToken string `json:"provider_access_token"`
	ExpiresIn           int    `json:"expires_in"`
}

// weComLoginInfo represents the WeCom login info response.
type weComLoginInfo struct {
	ErrCode  int    `json:"errcode"`
	ErrMsg   string `json:"errmsg"`
	UserType int    `json:"usertype"`
	UserInfo struct {
		UserID     string `json:"userid"`
		OpenUserID string `json:"open_userid"`
		Name       string `json:"name"`
		Avatar     string `json:"avatar"`
	} `json:"user_info"`
	CorpInfo struct {
		CorpID string `json:"corpid"`
	} `json:"corp_info"`
}

// ExchangeCode exchanges the authorization code for a provider access token.
// For WeCom Third-party, this uses POST with corpid/provider_secret.
func (p *WeComThirdPartyProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	reqBody := map[string]string{
		"corpid":          p.config.ClientID,
		"provider_secret": p.config.ClientSecret,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "marshal request body")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create token request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get provider token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read token response")
	}

	var tokenResp weComProviderToken
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}

	if tokenResp.ErrCode != 0 {
		return nil, apperrors.NewUnauthorizedf("wecom provider token error (code %d): %s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}

	return &OAuthToken{
		AccessToken: tokenResp.ProviderAccessToken,
		ExpiresIn:   int64(tokenResp.ExpiresIn),
		Scope:       code, // Store code in Scope for GetUserInfo to use
	}, nil
}

// GetUserInfo retrieves user information using the provider access token and auth code.
// For WeCom Third-party, this is a single POST request to get_login_info.
func (p *WeComThirdPartyProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	authCode := token.Scope // Code stored in Scope field during ExchangeCode

	reqBody := map[string]string{
		"auth_code": authCode,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "marshal request body")
	}

	loginInfoURL := fmt.Sprintf("%s?access_token=%s", p.userURL, token.AccessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginInfoURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create login info request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get login info")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read login info response")
	}

	var loginInfo weComLoginInfo
	if err := json.Unmarshal(body, &loginInfo); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse login info response")
	}

	if loginInfo.ErrCode != 0 {
		return nil, apperrors.NewUnauthorizedf("wecom login info error (code %d): %s", loginInfo.ErrCode, loginInfo.ErrMsg)
	}

	id := loginInfo.UserInfo.OpenUserID
	if id == "" {
		id = loginInfo.UserInfo.UserID
	}

	return &UserInfo{
		ID:          id,
		Username:    loginInfo.UserInfo.Name,
		DisplayName: loginInfo.UserInfo.Name,
		AvatarURL:   loginInfo.UserInfo.Avatar,
	}, nil
}
