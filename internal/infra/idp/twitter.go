package idp

import (
	"context"
	"encoding/json"
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TwitterProvider implements IdP authentication via the X (Twitter) OAuth 2.0
// authorization-code flow with PKCE. The v2 endpoints are
// https://twitter.com/i/oauth2/authorize, https://api.twitter.com/2/oauth2/token
// and https://api.twitter.com/2/users/me.
type TwitterProvider struct {
	BaseOAuthProvider
}

// NewTwitterProvider creates a new Twitter OAuth2 provider.
func NewTwitterProvider(config *OAuthConfig) *TwitterProvider {
	return &TwitterProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://twitter.com/i/oauth2/authorize",
			"https://api.twitter.com/2/oauth2/token",
			"https://api.twitter.com/2/users/me",
		),
	}
}

// twitterMeResponse is the v2 users/me answer, wrapped in a data object.
type twitterMeResponse struct {
	Data struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		UserName string `json:"username"`
	} `json:"data"`
	Errors []struct {
		Code     int    `json:"code"`
		Message  string `json:"message"`
		Resource string `json:"resource"`
		Field    string `json:"field"`
	} `json:"errors"`
}

// GetUserInfo retrieves the X profile; the v2 API nests the payload under data.
func (p *TwitterProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	body, status, err := p.requestJSON(ctx, http.MethodGet, p.userURL, http.NoBody, map[string]string{
		"Authorization": "Bearer " + token.AccessToken,
		"Accept":        "application/json",
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		var resp twitterMeResponse
		if json.Unmarshal(body, &resp) == nil && len(resp.Errors) > 0 {
			return nil, apperrors.NewUnauthorizedf("twitter user info error (code %d): %s", resp.Errors[0].Code, resp.Errors[0].Message)
		}
		return nil, apperrors.NewUnauthorizedf("twitter user info failed (status %d): %s", status, string(body))
	}

	var resp twitterMeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}
	if resp.Data.ID == "" {
		return nil, apperrors.NewNotFound("twitter user info response did not include a user id")
	}
	return &UserInfo{
		ID:          resp.Data.ID,
		Username:    resp.Data.UserName,
		DisplayName: resp.Data.Name,
	}, nil
}
