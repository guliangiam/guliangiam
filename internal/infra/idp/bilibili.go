package idp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BilibiliProvider implements IdP authentication via Bilibili OAuth2.
// The token/userinfo pair uses the current open-platform endpoints
// (https://open.bilibili.com/doc/4/eaf0e2b5-bde9-b9a0-9be1-019bb455701c):
// POST https://api.bilibili.com/x/account-oauth2/v1/token with the OAuth
// parameters as URL query parameters, and
// GET https://api.bilibili.com/x/account-oauth2/v1/userinfo?access_token=&client_id=.
// Both answers are wrapped in {code, message, data}.
type BilibiliProvider struct {
	BaseOAuthProvider
}

// NewBilibiliProvider creates a new Bilibili OAuth2 provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：端点已按上方 open.bilibili.com 官方文档复核。
func NewBilibiliProvider(config *OAuthConfig) *BilibiliProvider {
	return &BilibiliProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://passport.bilibili.com/oauth2/authorize",
			"https://api.bilibili.com/x/account-oauth2/v1/token",
			"https://api.bilibili.com/x/account-oauth2/v1/userinfo",
		),
	}
}

// bilibiliTokenResponse is the {code, message, data} token answer.
type bilibiliTokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	} `json:"data"`
}

// ExchangeCode exchanges the authorization code for a Bilibili access token.
func (p *BilibiliProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	u, err := url.Parse(p.tokenURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token URL")
	}
	q := u.Query()
	q.Set("client_id", p.config.ClientID)
	q.Set("client_secret", p.config.ClientSecret)
	q.Set("grant_type", "authorization_code")
	u.RawQuery = q.Encode()

	payload, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "marshal token request")
	}

	body, status, err := p.requestJSON(ctx, http.MethodPost, u.String(), bytes.NewReader(payload), map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("bilibili token exchange failed (status %d): %s", status, string(body))
	}

	var resp bilibiliTokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}
	if resp.Code != 0 || resp.Data.AccessToken == "" {
		return nil, apperrors.NewUnauthorizedf("bilibili token error (code %d): %s", resp.Code, resp.Message)
	}

	return &OAuthToken{
		AccessToken:  resp.Data.AccessToken,
		RefreshToken: resp.Data.RefreshToken,
		ExpiresIn:    resp.Data.ExpiresIn,
	}, nil
}

// bilibiliUserResponse is the {code, data} userinfo answer.
type bilibiliUserResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		OpenID string `json:"openid"`
		Name   string `json:"name"`
		Face   string `json:"face"`
	} `json:"data"`
}

// GetUserInfo retrieves the Bilibili profile with query-parameter auth.
func (p *BilibiliProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	u, err := url.Parse(p.userURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info URL")
	}
	q := u.Query()
	q.Set("access_token", token.AccessToken)
	q.Set("client_id", p.config.ClientID)
	u.RawQuery = q.Encode()

	body, status, err := p.requestJSON(ctx, http.MethodGet, u.String(), http.NoBody, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("get user info failed (status %d): %s", status, string(body))
	}

	var resp bilibiliUserResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}
	if resp.Code != 0 || resp.Data.OpenID == "" {
		return nil, apperrors.NewUnauthorizedf("bilibili user info error (code %d): %s", resp.Code, resp.Message)
	}

	return &UserInfo{
		ID:          resp.Data.OpenID,
		Username:    resp.Data.Name,
		DisplayName: resp.Data.Name,
		AvatarURL:   resp.Data.Face,
		OpenID:      resp.Data.OpenID,
	}, nil
}
