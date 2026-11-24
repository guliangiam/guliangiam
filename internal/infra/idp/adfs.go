package idp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ADFSProvider implements IdP authentication via Active Directory Federation Services (ADFS).
// ADFS uses standard OAuth2 endpoints under /adfs/oauth2/ on the ADFS server host.
// User info is extracted from the id_token JWT, as the ADFS userinfo endpoint only returns sub.
type ADFSProvider struct {
	BaseOAuthProvider
	host string
}

// NewADFSProvider creates a new ADFS OAuth2 provider.
// The Host field in OAuthConfig specifies the ADFS server URL (e.g., https://adfs.example.com).
func NewADFSProvider(config *OAuthConfig) *ADFSProvider {
	host := config.Host
	if host == "" {
		host = "https://adfs.example.com"
	}
	host = strings.TrimRight(host, "/")

	return &ADFSProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			fmt.Sprintf("%s/adfs/oauth2/authorize", host),
			fmt.Sprintf("%s/adfs/oauth2/token", host),
			fmt.Sprintf("%s/adfs/userinfo", host),
		),
		host: host,
	}
}

// adfsTokenResp represents the ADFS token response.
type adfsTokenResp struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// ExchangeCode exchanges the authorization code for an ADFS token.
func (p *ADFSProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", p.config.ClientID)
	data.Set("client_secret", p.config.ClientSecret)
	data.Set("redirect_uri", p.config.RedirectURI)
	data.Set("resource", p.config.ClientID) // ADFS typically requires resource parameter

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ssrf.DefaultHTTPClient.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "exchange code")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read token response")
	}

	var tokenResp adfsTokenResp
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}

	if tokenResp.Error != "" {
		return nil, apperrors.NewUnauthorizedf("adfs token error: %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return &OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    int64(tokenResp.ExpiresIn),
		Scope:        tokenResp.IDToken, // Store ID token in Scope for GetUserInfo
	}, nil
}

// GetUserInfo retrieves user information from ADFS.
// Since the ADFS userinfo endpoint typically only returns sub,
// we decode the id_token JWT payload to extract user claims.
func (p *ADFSProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	// First try the userinfo endpoint
	info, err := p.getUserInfoFromEndpoint(ctx, token)
	if err == nil && (info.ID != "" || info.DisplayName != "") {
		return info, nil
	}

	// Fall back to decoding the ID token JWT payload
	idToken := token.Scope // ID token stored in Scope during ExchangeCode
	if idToken == "" {
		return nil, apperrors.NewNotFound("could not extract user info from ADFS: no id_token available")
	}

	return p.parseIDTokenClaims(idToken)
}

// getUserInfoFromEndpoint tries to get user info from the ADFS userinfo endpoint.
func (p *ADFSProvider) getUserInfoFromEndpoint(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userURL, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create user info request")
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := ssrf.DefaultHTTPClient.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get user info")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read user info response")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}

	info := &UserInfo{}
	if sub, ok := raw["sub"].(string); ok {
		info.ID = sub
	}
	if name, ok := raw["name"].(string); ok {
		info.DisplayName = name
		info.Username = name
	}
	if email, ok := raw["email"].(string); ok {
		info.Email = email
	}
	if upn, ok := raw["upn"].(string); ok && info.Email == "" {
		info.Email = upn
	}

	return info, nil
}

// parseIDTokenClaims extracts user info from the JWT ID token payload.
// The ID token is a JWT; we decode the payload without signature verification
// (the token was obtained over TLS from a trusted endpoint).
func (p *ADFSProvider) parseIDTokenClaims(idToken string) (*UserInfo, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return nil, apperrors.NewInvalidArgument("invalid id_token format")
	}

	payload, err := decodeBase64URL(parts[1])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode id_token payload")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse id_token claims")
	}

	info := &UserInfo{}

	if sid, ok := claims["sid"].(string); ok {
		info.ID = sid
	}
	if sub, ok := claims["sub"].(string); ok && info.ID == "" {
		info.ID = sub
	}
	if name, ok := claims["unique_name"].(string); ok {
		info.Username = name
		info.DisplayName = name
	}
	if name, ok := claims["name"].(string); ok && info.DisplayName == "" {
		info.DisplayName = name
		info.Username = name
	}
	if upn, ok := claims["upn"].(string); ok {
		info.Email = upn
	}
	if email, ok := claims["email"].(string); ok && info.Email == "" {
		info.Email = email
	}

	return info, nil
}

// decodeBase64URL decodes a base64url-encoded string (no padding).
func decodeBase64URL(s string) ([]byte, error) {
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}
