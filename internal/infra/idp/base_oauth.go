package idp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BaseOAuthProvider provides a generic OAuth2 implementation that specific
// providers can embed and customize by overriding methods.
type BaseOAuthProvider struct {
	config   *OAuthConfig
	authURL  string
	tokenURL string
	userURL  string
}

func newBaseOAuth(config *OAuthConfig, authURL, tokenURL, userURL string) BaseOAuthProvider {
	return BaseOAuthProvider{
		config:   config,
		authURL:  authURL,
		tokenURL: tokenURL,
		userURL:  userURL,
	}
}

// httpClient returns the provider-specific client when the row opted into
// proxy/headers, otherwise the SSRF-guarded package default.
func (p *BaseOAuthProvider) httpClient() *http.Client {
	if p.config != nil && p.config.HTTPClient != nil {
		return p.config.HTTPClient
	}
	return ssrf.DefaultHTTPClient
}

func (p *BaseOAuthProvider) applyHeaders(req *http.Request) {
	if p.config == nil {
		return
	}
	for key, value := range p.config.HTTPHeaders {
		req.Header.Set(key, value)
	}
}

// sanitizeURLError strips the query string (and userinfo) from a *url.Error
// before it is embedded in a wrapped error. Several Chinese IdPs carry secrets
// (corpsecret, appsecret, access_token) in the query string and the Go HTTP
// client echoes the full URL in transport errors; the redacted form keeps
// scheme://host/path plus a literal "?redacted" marker and preserves the
// unwrap chain.
func sanitizeURLError(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) || urlErr.URL == "" {
		return err
	}
	redacted := *urlErr
	if parsed, parseErr := url.Parse(urlErr.URL); parseErr == nil {
		parsed.User = nil
		parsed.Fragment = ""
		parsed.RawQuery = "redacted"
		redacted.URL = parsed.String()
	} else {
		redacted.URL = "redacted"
	}
	return &redacted
}

// requestJSON issues an HTTP request with the provider's client and headers
// and returns the raw response body plus status. Vendor-specific error
// mapping stays in each adapter because several Chinese IdPs answer 200 with
// errcode-style payloads.
func (p *BaseOAuthProvider) requestJSON(ctx context.Context, method, rawURL string, body io.Reader, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "create request")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	p.applyHeaders(req)

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(sanitizeURLError(err), "provider request")
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, apperrors.WrapInternalf(err, "read response")
	}
	return data, resp.StatusCode, nil
}

func (p *BaseOAuthProvider) GetRedirectURL(state string) (string, error) {
	return p.GetRedirectURLWithOptions(state, AuthorizationRequestOptions{})
}

// GetRedirectURLWithOptions adds the nonce and S256 PKCE parameters when they
// are supplied. Empty options produce exactly the same URL as GetRedirectURL,
// so named OAuth2 providers keep their previous behaviour.
func (p *BaseOAuthProvider) GetRedirectURLWithOptions(state string, options AuthorizationRequestOptions) (string, error) {
	if strings.TrimSpace(p.authURL) == "" {
		return "", apperrors.NewInvalidArgument("auth URL is not configured for this provider")
	}
	u, err := url.Parse(p.authURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}

	q := u.Query()
	q.Set("client_id", p.config.ClientID)
	q.Set("redirect_uri", p.config.RedirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	if len(p.config.Scopes) > 0 {
		q.Set("scope", strings.Join(p.config.Scopes, " "))
	}
	if nonce := strings.TrimSpace(options.Nonce); nonce != "" {
		q.Set("nonce", nonce)
	}
	if challenge := strings.TrimSpace(options.CodeChallenge); challenge != "" {
		method := strings.TrimSpace(options.CodeChallengeMethod)
		if method == "" {
			method = "S256"
		}
		if method != "S256" {
			return "", apperrors.NewInvalidArgument("code_challenge_method must be S256")
		}
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", method)
	} else if strings.TrimSpace(options.CodeChallengeMethod) != "" {
		return "", apperrors.NewInvalidArgument("code_challenge is required when code_challenge_method is set")
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (p *BaseOAuthProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	if strings.TrimSpace(p.tokenURL) == "" {
		return nil, apperrors.NewInvalidArgument("token URL is not configured for this provider")
	}
	data := url.Values{}
	data.Set("client_id", p.config.ClientID)
	data.Set("client_secret", p.config.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", p.config.RedirectURI)
	data.Set("grant_type", "authorization_code")
	if p.config.CodeVerifier != "" {
		data.Set("code_verifier", p.config.CodeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	p.applyHeaders(req)

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(sanitizeURLError(err), "exchange code")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read token response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	// OAuth token endpoints use snake_case (RFC 6749); a few vendors emit
	// camelCase. Decode both instead of losing the access token silently.
	token, err := decodeTokenPayload(body)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func decodeTokenPayload(body []byte) (*OAuthToken, error) {
	var payload struct {
		AccessToken       string `json:"access_token"`
		AccessTokenCamel  string `json:"accessToken"`
		TokenType         string `json:"token_type"`
		TokenTypeCamel    string `json:"tokenType"`
		RefreshToken      string `json:"refresh_token"`
		RefreshTokenCamel string `json:"refreshToken"`
		ExpiresIn         int64  `json:"expires_in"`
		ExpiresInCamel    int64  `json:"expiresIn"`
		Scope             string `json:"scope"`
		ID                string `json:"id"`
		OpenID            string `json:"open_id"`
		OpenIDCamel       string `json:"openId"`
		UnionID           string `json:"union_id"`
		UnionIDCamel      string `json:"unionId"`
		IDToken           string `json:"id_token"`
		IDTokenCamel      string `json:"idToken"`
		Error             string `json:"error"`
		ErrorDescription  string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}
	if payload.Error != "" {
		return nil, apperrors.NewUnauthorizedf("token exchange rejected: %s %s", payload.Error, payload.ErrorDescription)
	}

	token := &OAuthToken{
		AccessToken:  firstNonEmptyString(payload.AccessToken, payload.AccessTokenCamel),
		TokenType:    firstNonEmptyString(payload.TokenType, payload.TokenTypeCamel),
		RefreshToken: firstNonEmptyString(payload.RefreshToken, payload.RefreshTokenCamel),
		Scope:        payload.Scope,
		ID:           payload.ID,
		OpenID:       firstNonEmptyString(payload.OpenID, payload.OpenIDCamel),
		UnionID:      firstNonEmptyString(payload.UnionID, payload.UnionIDCamel),
		IDToken:      firstNonEmptyString(payload.IDToken, payload.IDTokenCamel),
	}
	if payload.ExpiresIn > 0 {
		token.ExpiresIn = payload.ExpiresIn
	} else {
		token.ExpiresIn = payload.ExpiresInCamel
	}
	if token.AccessToken == "" {
		return nil, apperrors.NewUnauthorized("token response did not include an access token")
	}

	return token, nil
}

func (p *BaseOAuthProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	if strings.TrimSpace(p.userURL) == "" {
		return nil, apperrors.NewInvalidArgument("user info URL is not configured for this provider")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")
	p.applyHeaders(req)

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(sanitizeURLError(err), "get user info")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read user info response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("get user info failed (status %d): %s", resp.StatusCode, string(body))
	}

	return p.parseUserInfo(body)
}

func (p *BaseOAuthProvider) parseUserInfo(body []byte) (*UserInfo, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}

	info := &UserInfo{}
	switch id := raw["id"].(type) {
	case float64:
		info.ID = fmt.Sprintf("%.0f", id)
	case string:
		info.ID = id
	}
	// OIDC-style endpoints (Casdoor, Microsoft, Auth0) report the subject as
	// `sub`; a few vendors use user_id/userid/openid instead of `id`.
	if info.ID == "" {
		for _, key := range []string{"sub", "user_id", "userid", "openid", "open_id"} {
			if v, ok := raw[key].(string); ok && v != "" {
				info.ID = v
				break
			}
		}
	}
	if v, ok := raw["login"].(string); ok {
		info.Username = v
	}
	if info.Username == "" {
		if v, ok := raw["preferred_username"].(string); ok {
			info.Username = v
		}
	}
	if v, ok := raw["name"].(string); ok {
		info.DisplayName = v
	}
	if v, ok := raw["email"].(string); ok {
		info.Email = v
	}
	if v, ok := raw["avatar_url"].(string); ok {
		info.AvatarURL = v
	} else if v, ok := raw["picture"].(string); ok {
		info.AvatarURL = v
	}
	if v, ok := raw["phone_number"].(string); ok {
		info.Phone = v
	} else if v, ok := raw["phone"].(string); ok {
		info.Phone = v
	}

	return info, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
