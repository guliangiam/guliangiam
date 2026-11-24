package idp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// LarkProvider implements IdP authentication via Lark (Feishu) OAuth2.
// It targets the current authen v2 token endpoint
// (https://open.feishu.cn/open-apis/authen/v2/oauth/token, JSON body) and
// the authen v1 user_info endpoint whose payload is wrapped in
// {code, msg, data}. Set OAuthConfig.Host to a Lark international tenant
// (e.g. https://open.larksuite.com) to switch the whole endpoint family.
type LarkProvider struct {
	BaseOAuthProvider

	domain string
}

// NewLarkProvider creates a new Lark OAuth2 provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：authen v1 authorize / v2 token / v1 user_info
// 已按官方文档复核 (https://open.feishu.cn/document/server-docs/authentication-management/login-state-management/obtain-user-token)，
// 国际版通过 Host=open.larksuite.com 切换端点族。
func NewLarkProvider(config *OAuthConfig) *LarkProvider {
	domain := "https://open.feishu.cn"
	if config != nil {
		if h := strings.TrimRight(config.Host, "/"); h != "" {
			if strings.HasPrefix(h, "http://") || strings.HasPrefix(h, "https://") {
				domain = h
			} else {
				domain = "https://" + h
			}
		}
	}
	return &LarkProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			domain+"/open-apis/authen/v1/authorize",
			domain+"/open-apis/authen/v2/oauth/token",
			domain+"/open-apis/authen/v1/user_info",
		),
		domain: domain,
	}
}

// ExchangeCode exchanges the authorization code for a Lark user_access_token.
// The v2 endpoint expects a JSON body and answers with top-level OAuth fields.
func (p *LarkProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	payload, err := json.Marshal(map[string]string{
		"client_id":     p.config.ClientID,
		"client_secret": p.config.ClientSecret,
		"code":          code,
		"grant_type":    "authorization_code",
		"redirect_uri":  p.config.RedirectURI,
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "marshal token request")
	}

	body, status, err := p.requestJSON(ctx, http.MethodPost, p.tokenURL, bytes.NewReader(payload), map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("lark token exchange failed (status %d): %s", status, string(body))
	}
	return decodeTokenPayload(body)
}

// larkUserInfoResponse is the authen v1 user_info answer: {code, msg, data}.
type larkUserInfoResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		UserID    string `json:"user_id"`
		OpenID    string `json:"open_id"`
		UnionID   string `json:"union_id"`
		Name      string `json:"name"`
		EnName    string `json:"en_name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
		Mobile    string `json:"mobile"`
	} `json:"data"`
}

// GetUserInfo retrieves the Lark profile with the user_access_token bearer.
func (p *LarkProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	body, status, err := p.requestJSON(ctx, http.MethodGet, p.userURL, http.NoBody, map[string]string{
		"Authorization": "Bearer " + token.AccessToken,
		"Accept":        "application/json",
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("get user info failed (status %d): %s", status, string(body))
	}

	var resp larkUserInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}
	if resp.Code != 0 {
		return nil, apperrors.NewUnauthorizedf("lark user info error (code %d): %s", resp.Code, resp.Msg)
	}

	id := resp.Data.UnionID
	if id == "" {
		id = resp.Data.OpenID
	}
	if id == "" {
		id = resp.Data.UserID
	}
	if id == "" {
		return nil, apperrors.NewNotFound("lark user info response did not include an identifier")
	}
	return &UserInfo{
		ID:          id,
		Username:    resp.Data.EnName,
		DisplayName: resp.Data.Name,
		AvatarURL:   resp.Data.AvatarURL,
		Email:       resp.Data.Email,
		Phone:       resp.Data.Mobile,
		OpenID:      resp.Data.OpenID,
		UnionID:     resp.Data.UnionID,
	}, nil
}
