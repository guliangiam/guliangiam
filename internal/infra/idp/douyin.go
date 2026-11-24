package idp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DouyinProvider implements IdP authentication via Douyin OAuth2.
// The endpoints follow the open.douyin.com documentation: the token request
// uses client_key (not client_id), and both token and userinfo answers are
// wrapped in {data, message, error_code}. The userinfo call is a GET with a
// JSON body carrying access_token/open_id plus an access-token header.
// https://open.douyin.com/platform/doc/oauth2-web
type DouyinProvider struct {
	BaseOAuthProvider
}

// NewDouyinProvider creates a new Douyin OAuth2 provider.
func NewDouyinProvider(config *OAuthConfig) *DouyinProvider {
	if config != nil && len(config.Scopes) == 0 {
		config.Scopes = []string{"user_info"}
	}
	return &DouyinProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://open.douyin.com/platform/oauth/connect",
			"https://open.douyin.com/oauth/access_token",
			"https://open.douyin.com/oauth/userinfo/",
		),
	}
}

// GetRedirectURL uses client_key instead of client_id.
func (p *DouyinProvider) GetRedirectURL(state string) (string, error) {
	u, err := url.Parse(p.authURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}
	q := u.Query()
	q.Set("client_key", p.config.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(p.config.Scopes, ","))
	q.Set("redirect_uri", p.config.RedirectURI)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// douyinTokenResponse is the {data:{...}, message, error_code} token answer.
type douyinTokenResponse struct {
	Data struct {
		ErrorCode    int    `json:"error_code"`
		Description  string `json:"description"`
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		OpenID       string `json:"open_id"`
		Scope        string `json:"scope"`
	} `json:"data"`
	Message string `json:"message"`
}

// ExchangeCode exchanges the authorization code for a Douyin access token.
func (p *DouyinProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("client_key", p.config.ClientID)
	form.Set("client_secret", p.config.ClientSecret)

	body, status, err := p.requestJSON(ctx, http.MethodPost, p.tokenURL,
		strings.NewReader(form.Encode()),
		map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
			"Accept":       "application/json",
		})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("douyin token exchange failed (status %d): %s", status, string(body))
	}

	var resp douyinTokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}
	if resp.Data.ErrorCode != 0 || resp.Data.AccessToken == "" {
		return nil, apperrors.NewUnauthorizedf("douyin token error (code %d): %s", resp.Data.ErrorCode, resp.Data.Description)
	}

	return &OAuthToken{
		AccessToken:  resp.Data.AccessToken,
		RefreshToken: resp.Data.RefreshToken,
		ExpiresIn:    resp.Data.ExpiresIn,
		Scope:        resp.Data.Scope,
		ID:           resp.Data.OpenID,
		OpenID:       resp.Data.OpenID,
	}, nil
}

// douyinUserResponse is the {data:{...}} userinfo answer.
type douyinUserResponse struct {
	Data struct {
		OpenID   string `json:"open_id"`
		UnionID  string `json:"union_id"`
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
	} `json:"data"`
	Message string `json:"message"`
}

// GetUserInfo retrieves the Douyin profile. The endpoint is a GET that still
// requires a JSON body with access_token/open_id and an access-token header.
func (p *DouyinProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	openID := token.OpenID
	if openID == "" {
		openID = token.ID
	}
	if openID == "" {
		return nil, apperrors.NewUnauthorized("douyin token did not include an open_id")
	}

	payload, err := json.Marshal(map[string]string{
		"access_token": token.AccessToken,
		"open_id":      openID,
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "marshal user info request")
	}

	body, status, err := p.requestJSON(ctx, http.MethodGet, p.userURL, bytes.NewReader(payload), map[string]string{
		"Content-Type": "application/json",
		"access-token": token.AccessToken,
		"Accept":       "application/json",
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("get user info failed (status %d): %s", status, string(body))
	}

	var resp douyinUserResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}
	if resp.Data.OpenID == "" {
		return nil, apperrors.NewNotFound("douyin user info response did not include an open_id: " + resp.Message)
	}

	return &UserInfo{
		ID:          resp.Data.OpenID,
		Username:    resp.Data.Nickname,
		DisplayName: resp.Data.Nickname,
		AvatarURL:   resp.Data.Avatar,
		OpenID:      resp.Data.OpenID,
		UnionID:     resp.Data.UnionID,
	}, nil
}
