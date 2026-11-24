package idp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// KwaiProvider implements IdP authentication via Kwai (快手) OAuth2.
// Kwai uses app_id/app_secret as parameter names instead of client_id/client_secret.
// Endpoints follow https://open.kuaishou.com/docs/develop/source/authorization_process.html
type KwaiProvider struct {
	BaseOAuthProvider
}

// NewKwaiProvider creates a new Kwai OAuth2 provider.
func NewKwaiProvider(config *OAuthConfig) *KwaiProvider {
	if len(config.Scopes) == 0 {
		config.Scopes = []string{"user_info"}
	}
	return &KwaiProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://open.kuaishou.com/oauth2/authorize",
			"https://open.kuaishou.com/oauth2/access_token",
			"https://open.kuaishou.com/openapi/user_info",
		),
	}
}

// kwaiTokenResp represents the Kwai token response.
type kwaiTokenResp struct {
	Result       int    `json:"result"`
	ErrorMsg     string `json:"error_msg"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"open_id"`
}

// kwaiUserInfoResp represents the Kwai user info response.
type kwaiUserInfoResp struct {
	Result   int    `json:"result"`
	ErrorMsg string `json:"error_msg"`
	UserInfo struct {
		Head string `json:"head"`
		Name string `json:"name"`
		Sex  string `json:"sex"`
		City string `json:"city"`
	} `json:"user_info"`
}

// ExchangeCode exchanges the authorization code for a Kwai access token.
// Kwai uses app_id/app_secret as query parameters instead of standard OAuth2 form body.
func (p *KwaiProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	u, err := url.Parse(p.tokenURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token URL")
	}
	q := u.Query()
	q.Set("app_id", p.config.ClientID)
	q.Set("app_secret", p.config.ClientSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")
	u.RawQuery = q.Encode()

	body, status, err := p.requestJSON(ctx, http.MethodGet, u.String(), http.NoBody, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("kwai token exchange failed (status %d): %s", status, string(body))
	}

	var tokenResp kwaiTokenResp
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}

	if tokenResp.Result != 1 || tokenResp.AccessToken == "" {
		return nil, apperrors.NewUnauthorizedf("kwai token error: %s", tokenResp.ErrorMsg)
	}

	return &OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    int64(tokenResp.ExpiresIn),
		ID:           tokenResp.OpenID,
		OpenID:       tokenResp.OpenID,
	}, nil
}

// GetUserInfo retrieves user information from Kwai.
func (p *KwaiProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	u, err := url.Parse(p.userURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info URL")
	}
	q := u.Query()
	q.Set("app_id", p.config.ClientID)
	q.Set("access_token", token.AccessToken)
	u.RawQuery = q.Encode()

	body, status, err := p.requestJSON(ctx, http.MethodGet, u.String(), http.NoBody, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("get user info failed (status %d): %s", status, string(body))
	}

	var kwaiUser kwaiUserInfoResp
	if err := json.Unmarshal(body, &kwaiUser); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}

	if kwaiUser.Result != 1 {
		return nil, apperrors.NewUnauthorizedf("kwai user info error: %s", kwaiUser.ErrorMsg)
	}

	return &UserInfo{
		ID:          token.OpenID,
		Username:    kwaiUser.UserInfo.Name,
		DisplayName: kwaiUser.UserInfo.Name,
		AvatarURL:   kwaiUser.UserInfo.Head,
		OpenID:      token.OpenID,
	}, nil
}

// GetRedirectURL returns the Kwai authorization URL with app_id instead of client_id.
func (p *KwaiProvider) GetRedirectURL(state string) (string, error) {
	u, err := url.Parse(p.authURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}

	q := u.Query()
	q.Set("app_id", p.config.ClientID)
	q.Set("redirect_uri", p.config.RedirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	if len(p.config.Scopes) > 0 {
		q.Set("scope", p.config.Scopes[0])
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}
