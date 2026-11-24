package idp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DingTalkProvider implements IdP authentication via DingTalk OAuth2.
// It uses the current-generation endpoints documented at
// https://open.dingtalk.com/document/orgapp/obtain-user-token :
// the token exchange is a JSON POST (clientId/clientSecret/code/grantType)
// and the profile endpoint authenticates with the
// x-acs-dingtalk-access-token header instead of Authorization: Bearer.
type DingTalkProvider struct {
	BaseOAuthProvider
}

// NewDingTalkProvider creates a new DingTalk OAuth2 provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：三端点与 x-acs-dingtalk-access-token 头已按上方官方文档复核。
func NewDingTalkProvider(config *OAuthConfig) *DingTalkProvider {
	if config != nil && len(config.Scopes) == 0 {
		// openid is required to obtain userAccessToken; openid+corpid additionally yields corpId.
		config.Scopes = []string{"openid"}
	}
	return &DingTalkProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://login.dingtalk.com/oauth2/auth",
			"https://api.dingtalk.com/v1.0/oauth2/userAccessToken",
			"https://api.dingtalk.com/v1.0/contact/users/me",
		),
	}
}

// dingtalkTokenResponse is the JSON body of /v1.0/oauth2/userAccessToken.
type dingtalkTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpireIn     int64  `json:"expireIn"`
	CorpID       string `json:"corpId"`
	// Error fields used by the API gateway for non-2xx answers.
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ExchangeCode exchanges the authorization code for a DingTalk user access token.
func (p *DingTalkProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	payload, err := json.Marshal(map[string]string{
		"clientId":     p.config.ClientID,
		"clientSecret": p.config.ClientSecret,
		"code":         code,
		"grantType":    "authorization_code",
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

	var tokenResp dingtalkTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}
	if status != http.StatusOK || tokenResp.AccessToken == "" {
		if tokenResp.Message != "" {
			return nil, apperrors.NewUnauthorizedf("dingtalk token error (%s): %s", tokenResp.Code, tokenResp.Message)
		}
		return nil, apperrors.NewUnauthorizedf("dingtalk token exchange failed (status %d): %s", status, string(body))
	}

	return &OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpireIn,
	}, nil
}

// dingtalkUserResponse is the JSON body of /v1.0/contact/users/me.
type dingtalkUserResponse struct {
	Nick      string `json:"nick"`
	AvatarURL string `json:"avatarUrl"`
	Mobile    string `json:"mobile"`
	OpenID    string `json:"openId"`
	UnionID   string `json:"unionId"`
	Email     string `json:"email"`
	StateCode string `json:"stateCode"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// GetUserInfo retrieves the DingTalk profile via the x-acs-dingtalk-access-token header.
func (p *DingTalkProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	body, status, err := p.requestJSON(ctx, http.MethodGet, p.userURL, http.NoBody, map[string]string{
		"x-acs-dingtalk-access-token": token.AccessToken,
		"Accept":                      "application/json",
	})
	if err != nil {
		return nil, err
	}

	var userResp dingtalkUserResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}
	if status != http.StatusOK || (userResp.OpenID == "" && userResp.UnionID == "") {
		if userResp.Message != "" {
			return nil, apperrors.NewUnauthorizedf("dingtalk user info error (%s): %s", userResp.Code, userResp.Message)
		}
		return nil, apperrors.NewUnauthorizedf("dingtalk user info failed (status %d): %s", status, string(body))
	}

	id := userResp.OpenID
	if id == "" {
		id = userResp.UnionID
	}
	return &UserInfo{
		ID:          id,
		Username:    userResp.Nick,
		DisplayName: userResp.Nick,
		AvatarURL:   userResp.AvatarURL,
		Email:       userResp.Email,
		Phone:       userResp.Mobile,
		OpenID:      userResp.OpenID,
		UnionID:     userResp.UnionID,
	}, nil
}
