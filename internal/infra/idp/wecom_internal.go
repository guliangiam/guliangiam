package idp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WeComInternalProvider implements WeCom (WeChat Work) Internal application login.
// Internal applications use a two-step process:
// 1. Get access token using corpid/corpsecret
// 2. Use the code from OAuth redirect to get the user's UserId, then fetch user details
type WeComInternalProvider struct {
	BaseOAuthProvider
}

// NewWeComInternalProvider creates a new WeCom Internal application provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：端点已按官方开发者文档复核
// (https://developer.work.weixin.qq.com — 网页扫码登录 qrConnect、
// gettoken、getuserinfo、user/get)；qrConnect 必填 corpid+agentid，
// AgentID 缺失在 GetRedirectURL 即 fail（见 provider_contract_test.go）。
func NewWeComInternalProvider(config *OAuthConfig) *WeComInternalProvider {
	return &WeComInternalProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://open.work.weixin.qq.com/wwopen/sso/qrConnect",
			"https://qyapi.weixin.qq.com/cgi-bin/gettoken",
			"https://qyapi.weixin.qq.com/cgi-bin/user/get",
		),
	}
}

// GetRedirectURL returns the WeCom Internal authorization URL.
func (p *WeComInternalProvider) GetRedirectURL(state string) (string, error) {
	u, err := url.Parse(p.authURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}

	q := u.Query()
	q.Set("appid", p.config.ClientID) // corpid
	// agentid is the application AgentID; the corp secret must never be
	// placed in this browser-visible URL.
	if p.config.AgentID == "" {
		return "", apperrors.NewInvalidArgument("wecom internal provider requires the AgentID configuration")
	}
	q.Set("agentid", p.config.AgentID)
	q.Set("redirect_uri", p.config.RedirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// weComAccessToken represents the WeCom access token response.
type weComAccessToken struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// weComInternalUserResp represents the response from getting user info by code.
type weComInternalUserResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	UserID  string `json:"UserId"`
	OpenID  string `json:"OpenId"`
}

// weComInternalUserInfo represents the full user info from WeCom.
type weComInternalUserInfo struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Avatar  string `json:"avatar"`
	UserID  string `json:"userid"`
	OpenID  string `json:"open_userid"`
}

// ExchangeCode exchanges the authorization code for an access token.
// For WeCom Internal, the token is obtained using corpid/corpsecret (not the code).
// The code is saved for the GetUserInfo step.
func (p *WeComInternalProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	u, err := url.Parse(p.tokenURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token URL")
	}
	q := u.Query()
	q.Set("corpid", p.config.ClientID)
	q.Set("corpsecret", p.config.ClientSecret)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create token request")
	}

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(sanitizeURLError(err), "get access token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read token response")
	}

	var tokenResp weComAccessToken
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}

	if tokenResp.ErrCode != 0 {
		return nil, apperrors.NewUnauthorizedf("wecom token error (code %d): %s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}

	return &OAuthToken{
		AccessToken: tokenResp.AccessToken,
		ExpiresIn:   int64(tokenResp.ExpiresIn),
		Scope:       code, // Store code in Scope for GetUserInfo to use
	}, nil
}

// GetUserInfo retrieves user information using the access token and code.
// WeCom Internal requires a two-step process:
// 1. Get UserId from the code
// 2. Get full user info using the UserId
func (p *WeComInternalProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	code := token.Scope // Code stored in Scope field during ExchangeCode

	// Step 1: Get UserId from code
	userInfoURL := "https://qyapi.weixin.qq.com/cgi-bin/user/getuserinfo"
	u, err := url.Parse(userInfoURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user id URL")
	}
	q := u.Query()
	q.Set("access_token", token.AccessToken)
	q.Set("code", code)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create user info request")
	}

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(sanitizeURLError(err), "get user id")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read user id response")
	}

	var userResp weComInternalUserResp
	if err := json.Unmarshal(body, &userResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user id response")
	}

	if userResp.ErrCode != 0 {
		return nil, apperrors.NewUnauthorizedf("wecom user id error (code %d): %s", userResp.ErrCode, userResp.ErrMsg)
	}

	// If OpenID is present, this is not an internal user
	if userResp.OpenID != "" {
		return nil, apperrors.NewUnauthorized("not an internal user (has OpenID, expected UserId only)")
	}

	// Step 2: Get full user info using UserId
	detailURL, err := url.Parse(p.userURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user detail URL")
	}
	detailQuery := detailURL.Query()
	detailQuery.Set("access_token", token.AccessToken)
	detailQuery.Set("userid", userResp.UserID)
	detailURL.RawQuery = detailQuery.Encode()

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, detailURL.String(), http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create user detail request")
	}

	resp, err = p.httpClient().Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(sanitizeURLError(err), "get user detail")
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read user detail response")
	}

	var infoResp weComInternalUserInfo
	if err := json.Unmarshal(body, &infoResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user detail response")
	}

	if infoResp.ErrCode != 0 {
		return nil, apperrors.NewUnauthorizedf("wecom user detail error (code %d): %s", infoResp.ErrCode, infoResp.ErrMsg)
	}

	id := infoResp.UserID
	if id == "" {
		id = infoResp.OpenID
	}

	return &UserInfo{
		ID:          id,
		Username:    infoResp.UserID,
		DisplayName: infoResp.Name,
		Email:       infoResp.Email,
		AvatarURL:   infoResp.Avatar,
	}, nil
}
