package idp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BaiduProvider implements IdP authentication via Baidu OAuth2.
type BaiduProvider struct {
	BaseOAuthProvider
}

// NewBaiduProvider creates a new Baidu OAuth2 provider.
func NewBaiduProvider(config *OAuthConfig) *BaiduProvider {
	if len(config.Scopes) == 0 {
		config.Scopes = []string{"basic"}
	}
	return &BaiduProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://openapi.baidu.com/oauth/2.0/authorize",
			"https://openapi.baidu.com/oauth/2.0/token",
			"https://openapi.baidu.com/rest/2.0/passport/users/getInfo",
		),
	}
}

// baiduUserInfo represents the Baidu user info response.
type baiduUserInfo struct {
	OpenID   string `json:"openid"`
	Username string `json:"username"`
	Portrait string `json:"portrait"`
}

// GetUserInfo retrieves user information from Baidu.
// Baidu uses a query parameter for the access token instead of the Authorization header.
func (p *BaiduProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	u, err := url.Parse(p.userURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info URL")
	}
	q := u.Query()
	q.Set("access_token", token.AccessToken)
	u.RawQuery = q.Encode()

	body, status, err := p.requestJSON(ctx, http.MethodGet, u.String(), http.NoBody, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("get user info failed (status %d): %s", status, string(body))
	}

	var baiduUser baiduUserInfo
	if err := json.Unmarshal(body, &baiduUser); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}

	avatarURL := ""
	if baiduUser.Portrait != "" {
		avatarURL = fmt.Sprintf("https://himg.bdimg.com/sys/portrait/item/%s", baiduUser.Portrait)
	}

	return &UserInfo{
		ID:          baiduUser.OpenID,
		Username:    baiduUser.Username,
		DisplayName: baiduUser.Username,
		AvatarURL:   avatarURL,
	}, nil
}
