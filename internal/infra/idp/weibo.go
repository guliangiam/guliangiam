package idp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WeiboProvider implements IdP authentication via Weibo OAuth2.
// The token endpoint is standard form-encoded but answers with `uid` next to
// the access token, and users/show.json requires access_token plus uid as
// query parameters (no Bearer header).
// https://open.weibo.com/wiki/Oauth2/access_token
type WeiboProvider struct {
	BaseOAuthProvider
}

// NewWeiboProvider creates a new Weibo OAuth2 provider.
func NewWeiboProvider(config *OAuthConfig) *WeiboProvider {
	return &WeiboProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://api.weibo.com/oauth2/authorize",
			"https://api.weibo.com/oauth2/access_token",
			"https://api.weibo.com/2/users/show.json",
		),
	}
}

// weiboTokenResponse carries the extra uid field of the Weibo token answer.
type weiboTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	RemindIn    string `json:"remind_in"`
	UID         string `json:"uid"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

// ExchangeCode exchanges the authorization code and captures the uid, which
// the profile endpoint requires.
func (p *WeiboProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", p.config.ClientID)
	form.Set("client_secret", p.config.ClientSecret)
	form.Set("redirect_uri", p.config.RedirectURI)

	body, status, err := p.requestJSON(ctx, http.MethodPost, p.tokenURL,
		strings.NewReader(form.Encode()),
		map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
			"Accept":       "application/json",
		})
	if err != nil {
		return nil, err
	}

	var tokenResp weiboTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperrors.NewUnauthorizedf("weibo token exchange failed (status %d): %s", status, string(body))
	}
	if status != http.StatusOK || tokenResp.AccessToken == "" {
		if tokenResp.Error != "" {
			return nil, apperrors.NewUnauthorizedf("weibo token error (%s): %s", tokenResp.Error, tokenResp.Description)
		}
		return nil, apperrors.NewUnauthorizedf("weibo token exchange failed (status %d): %s", status, string(body))
	}

	return &OAuthToken{
		AccessToken: tokenResp.AccessToken,
		ExpiresIn:   tokenResp.ExpiresIn,
		ID:          tokenResp.UID,
	}, nil
}

// weiboUserResponse is the users/show.json answer.
type weiboUserResponse struct {
	ID          int64  `json:"id"`
	IDStr       string `json:"idstr"`
	ScreenName  string `json:"screen_name"`
	Name        string `json:"name"`
	AvatarLarge string `json:"avatar_large"`
	AvatarHD    string `json:"avatar_hd"`
	ErrorCode   int    `json:"error_code"`
	Error       string `json:"error"`
}

// GetUserInfo fetches the profile with access_token and uid query parameters.
func (p *WeiboProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	uid := token.ID
	if uid == "" {
		return nil, apperrors.NewUnauthorized("weibo token did not include a uid")
	}

	u, err := url.Parse(p.userURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info URL")
	}
	q := u.Query()
	q.Set("access_token", token.AccessToken)
	q.Set("uid", uid)
	u.RawQuery = q.Encode()

	body, status, err := p.requestJSON(ctx, http.MethodGet, u.String(), http.NoBody, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("get user info failed (status %d): %s", status, string(body))
	}

	var user weiboUserResponse
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}
	if user.ID == 0 && user.IDStr == "" {
		if user.Error != "" {
			return nil, apperrors.NewUnauthorizedf("weibo user info error (%d): %s", user.ErrorCode, user.Error)
		}
		return nil, apperrors.NewNotFound("weibo user info response did not include a user id")
	}

	id := user.IDStr
	if id == "" {
		id = strconv.FormatInt(user.ID, 10)
	}
	displayName := user.ScreenName
	if displayName == "" {
		displayName = user.Name
	}
	avatar := user.AvatarHD
	if avatar == "" {
		avatar = user.AvatarLarge
	}
	return &UserInfo{
		ID:          id,
		Username:    user.ScreenName,
		DisplayName: displayName,
		AvatarURL:   avatar,
	}, nil
}
