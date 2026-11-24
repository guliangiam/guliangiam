package idp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// QQProvider implements IdP authentication via QQ Connect OAuth2.
// The flow is non-standard: the token endpoint answers with a
// form-encoded text body, the openid is only available from the JSONP
// /oauth2.0/me endpoint, and get_user_info requires oauth_consumer_key
// plus openid as query parameters.
// https://wiki.connect.qq.com/使用authorization_code获取access_token
type QQProvider struct {
	BaseOAuthProvider
}

// NewQQProvider creates a new QQ OAuth2 provider.
func NewQQProvider(config *OAuthConfig) *QQProvider {
	return &QQProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://graph.qq.com/oauth2.0/authorize",
			"https://graph.qq.com/oauth2.0/token",
			"https://graph.qq.com/user/get_user_info",
		),
	}
}

// ExchangeCode exchanges the authorization code for a QQ access token.
// The response body is `access_token=...&expires_in=...` text, not JSON.
func (p *QQProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	u, err := url.Parse(p.tokenURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token URL")
	}
	q := u.Query()
	q.Set("grant_type", "authorization_code")
	q.Set("client_id", p.config.ClientID)
	q.Set("client_secret", p.config.ClientSecret)
	q.Set("code", code)
	q.Set("redirect_uri", p.config.RedirectURI)
	u.RawQuery = q.Encode()

	body, status, err := p.requestJSON(ctx, http.MethodGet, u.String(), http.NoBody, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("qq token exchange failed (status %d): %s", status, string(body))
	}

	values, err := url.ParseQuery(string(body))
	if err != nil || values.Get("access_token") == "" {
		if oauthErr := values.Get("error"); oauthErr != "" {
			return nil, apperrors.NewUnauthorizedf("qq token error (%s): %s", oauthErr, values.Get("error_description"))
		}
		return nil, apperrors.NewUnauthorized("qq token response did not include an access token")
	}

	var expiresIn int64
	if v := values.Get("expires_in"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			expiresIn = n
		}
	}
	return &OAuthToken{
		AccessToken: values.Get("access_token"),
		ExpiresIn:   expiresIn,
	}, nil
}

// qqOpenIDRegexp extracts the openid from the JSONP /oauth2.0/me answer,
// e.g. callback( {"client_id":"...","openid":"..."} );
var qqOpenIDRegexp = regexp.MustCompile(`"openid"\s*:\s*"([^"]+)"`)

// qqMeResponse is the JSON payload embedded in the /oauth2.0/me JSONP answer.
type qqMeResponse struct {
	ClientID string `json:"client_id"`
	OpenID   string `json:"openid"`
}

// qqUserInfoResponse is the get_user_info answer.
type qqUserInfoResponse struct {
	Ret               int    `json:"ret"`
	Msg               string `json:"msg"`
	Nickname          string `json:"nickname"`
	Figureurl         string `json:"figureurl"`
	Figureurl1        string `json:"figureurl_1"`
	Figureurl2        string `json:"figureurl_2"`
	FigureurlQQ1      string `json:"figureurl_qq_1"`
	FigureurlQQ2      string `json:"figureurl_qq_2"`
	Gender            string `json:"gender"`
	Province          string `json:"province"`
	City              string `json:"city"`
	IsYearMember      string `json:"is_year_member"`
	YearMemberFlagExp string `json:"year_member_flag_expire"`
}

// GetUserInfo resolves the openid via /oauth2.0/me (JSONP) and then calls
// get_user_info with oauth_consumer_key and openid.
func (p *QQProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	openid := token.OpenID
	if openid == "" {
		meURL := p.tokenURL // graph.qq.com/oauth2.0/token
		if u, err := url.Parse(meURL); err == nil {
			u.Path = "/oauth2.0/me"
			meURL = u.String()
		}
		meURL = meURL + "?access_token=" + url.QueryEscape(token.AccessToken)

		body, status, err := p.requestJSON(ctx, http.MethodGet, meURL, http.NoBody, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, apperrors.NewUnauthorizedf("qq me failed (status %d): %s", status, string(body))
		}
		// Strip the JSONP wrapper when present and try a strict decode first.
		raw := string(body)
		if start := strings.Index(raw, "("); start >= 0 {
			if end := strings.LastIndex(raw, ")"); end > start {
				raw = raw[start+1 : end]
			}
		}
		var me qqMeResponse
		if err := json.Unmarshal([]byte(raw), &me); err != nil {
			if m := qqOpenIDRegexp.FindSubmatch(body); m != nil {
				openid = string(m[1])
			}
		} else {
			openid = me.OpenID
		}
		if openid == "" {
			return nil, apperrors.NewUnauthorized("qq did not return an openid")
		}
	}

	u, err := url.Parse(p.userURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info URL")
	}
	q := u.Query()
	q.Set("access_token", token.AccessToken)
	q.Set("oauth_consumer_key", p.config.ClientID)
	q.Set("openid", openid)
	u.RawQuery = q.Encode()

	body, status, err := p.requestJSON(ctx, http.MethodGet, u.String(), http.NoBody, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("get user info failed (status %d): %s", status, string(body))
	}

	var user qqUserInfoResponse
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}
	if user.Ret != 0 {
		return nil, apperrors.NewUnauthorizedf("qq user info error (ret %d): %s", user.Ret, user.Msg)
	}

	avatar := user.FigureurlQQ1
	if avatar == "" {
		avatar = user.Figureurl2
	}
	if avatar == "" {
		avatar = user.Figureurl
	}
	return &UserInfo{
		ID:          openid,
		Username:    user.Nickname,
		DisplayName: user.Nickname,
		AvatarURL:   avatar,
		OpenID:      openid,
	}, nil
}
