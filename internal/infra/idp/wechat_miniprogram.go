package idp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WeChatMiniProgramProvider implements WeChat MiniProgram login (code2session flow).
// Unlike standard OAuth2, MiniProgram login uses wx.login() to obtain a js_code,
// which the backend exchanges for session data via the jscode2session API.
type WeChatMiniProgramProvider struct {
	BaseOAuthProvider
}

// NewWeChatMiniProgramProvider creates a new WeChat MiniProgram provider.
// RedirectURI is not used for MiniProgram login.
func NewWeChatMiniProgramProvider(config *OAuthConfig) *WeChatMiniProgramProvider {
	return &WeChatMiniProgramProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"", // No auth URL - MiniProgram uses wx.login() on the client
			"https://api.weixin.qq.com/sns/jscode2session",
			"", // No user info URL - user info is derived from session data
		),
	}
}

// WeChatMiniProgramSession contains the session data returned by jscode2session.
type WeChatMiniProgramSession struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// GetRedirectURL returns an error for MiniProgram - this provider does not use redirect-based OAuth.
func (p *WeChatMiniProgramProvider) GetRedirectURL(state string) (string, error) {
	return "", apperrors.NewUnimplemented("wechat miniprogram does not support redirect-based OAuth; use GetSessionByCode instead")
}

// ExchangeCode exchanges a js_code for an OAuth token containing the OpenID.
// The AccessToken field carries the session_key (for data decryption),
// and the ID field is populated with the OpenID for user identification.
func (p *WeChatMiniProgramProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	session, err := p.GetSessionByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// Use UnionID as the primary ID if available (cross-app identity),
	// otherwise fall back to OpenID (app-specific identity).
	userID := session.UnionID
	if userID == "" {
		userID = session.OpenID
	}

	return &OAuthToken{
		AccessToken: session.SessionKey,
		ID:          userID,
		OpenID:      session.OpenID,
		UnionID:     session.UnionID,
		Scope:       "snsapi_miniprogram",
	}, nil
}

// GetUserInfo returns user info derived from the session data.
// For MiniProgram, the user identity is the OpenID (or UnionID if available).
// Applications should use wx.getUserProfile() on the client side for richer profile info.
func (p *WeChatMiniProgramProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	// The user ID was set during ExchangeCode.
	// UnionID is preferred for cross-app identity; OpenID is app-specific.
	userID := token.ID
	if userID == "" {
		userID = token.OpenID
	}

	return &UserInfo{
		ID:      userID,
		UnionID: token.UnionID,
		OpenID:  token.OpenID,
	}, nil
}

// GetSessionByCode exchanges a js_code (from wx.login) for session data.
// This is the primary method for WeChat MiniProgram authentication.
func (p *WeChatMiniProgramProvider) GetSessionByCode(ctx context.Context, jsCode string) (*WeChatMiniProgramSession, error) {
	// Use parameterized URL construction to prevent injection.
	sessionURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		p.tokenURL,
		url.QueryEscape(p.config.ClientID),
		url.QueryEscape(p.config.ClientSecret),
		url.QueryEscape(jsCode))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionURL, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create session request")
	}

	client := p.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get session by code")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read session response")
	}

	var session WeChatMiniProgramSession
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse session response")
	}

	if session.ErrCode != 0 {
		return nil, apperrors.NewUnauthorizedf("wechat miniprogram error (code %d): %s", session.ErrCode, session.ErrMsg)
	}

	return &session, nil
}
