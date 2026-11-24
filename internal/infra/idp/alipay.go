package idp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AlipayProvider implements IdP authentication via Alipay OAuth.
// Alipay is not a plain OAuth2 server: the authorization page is
// https://openauth.alipay.com/oauth2/publicAppAuthorize.htm (app_id/scope/
// redirect_uri), while token exchange and profile lookup go through the
// signed OpenAPI gateway https://openapi.alipay.com/gateway.do using
// alipay.system.oauth.token and alipay.user.info.share with RSA2
// (SHA-256 + PKCS1v15) request signatures.
// https://opendocs.alipay.com/open/02e7gq (授权登录)
//
// Credential semantics (OPEN-API-75 gap ③): OAuthConfig.ClientSecret carries
// the 支付宝 RSA2 应用私钥 (application private key, PKCS#1/PKCS#8 PEM) used to
// sign gateway requests — it is NOT an OAuth client_secret. The UI field label
// in Provider Center mirrors this semantics.
type AlipayProvider struct {
	BaseOAuthProvider
}

// NewAlipayProvider creates a new Alipay OAuth provider.
func NewAlipayProvider(config *OAuthConfig) *AlipayProvider {
	if config != nil && len(config.Scopes) == 0 {
		// auth_user asks the user to authorize profile access; auth_base is
		// silent and cannot feed alipay.user.info.share.
		config.Scopes = []string{"auth_user"}
	}
	return &AlipayProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://openauth.alipay.com/oauth2/publicAppAuthorize.htm",
			"https://openapi.alipay.com/gateway.do",
			"https://openapi.alipay.com/gateway.do",
		),
	}
}

// GetRedirectURL returns the Alipay authorization page with app_id instead of client_id.
func (p *AlipayProvider) GetRedirectURL(state string) (string, error) {
	u, err := url.Parse(p.authURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}
	q := u.Query()
	q.Set("app_id", p.config.ClientID)
	q.Set("scope", strings.Join(p.config.Scopes, ","))
	q.Set("redirect_uri", p.config.RedirectURI)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// alipayPrivateKey decodes the application private key (PKCS#1 or PKCS#8 PEM)
// stored in ClientSecret.
func alipayPrivateKey(secret string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(secret)))
	if block == nil {
		return nil, apperrors.NewInvalidArgument("alipay provider requires the application private key (PEM) as client secret")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, apperrors.NewInvalidArgument("alipay private key is not an RSA key")
		}
		return rsaKey, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, apperrors.NewInvalidArgument("failed to parse the alipay private key")
}

// alipaySignString builds the sorted k=v&k=v string that must be signed.
func alipaySignString(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "sign_type" || params[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b bytes.Buffer
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	return b.String()
}

// alipayRequest signs and POSTs an OpenAPI gateway request, returning the
// decoded JSON body.
func (p *AlipayProvider) alipayRequest(ctx context.Context, method string, appParams map[string]string) (map[string]json.RawMessage, error) {
	key, err := alipayPrivateKey(p.config.ClientSecret)
	if err != nil {
		return nil, err
	}

	// Alipay expects timestamps in GMT+8.
	timestamp := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
	params := map[string]string{
		"app_id":    p.config.ClientID,
		"method":    method,
		"format":    "json",
		"charset":   "utf-8",
		"sign_type": "RSA2",
		"timestamp": timestamp,
		"version":   "1.0",
	}
	for k, v := range appParams {
		params[k] = v
	}

	digest := sha256.Sum256([]byte(alipaySignString(params)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "sign request")
	}
	params["sign"] = base64.StdEncoding.EncodeToString(signature)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	body, status, err := p.requestJSON(ctx, http.MethodPost, p.tokenURL,
		strings.NewReader(form.Encode()),
		map[string]string{
			"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
			"Accept":       "application/json",
		})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apperrors.NewUnauthorizedf("alipay gateway failed (status %d): %s", status, string(body))
	}

	var resp map[string]json.RawMessage
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse gateway response")
	}
	if errBody, ok := resp["error_response"]; ok {
		var apiErr struct {
			Code    int    `json:"code"`
			Msg     string `json:"msg"`
			SubCode string `json:"sub_code"`
			SubMsg  string `json:"sub_msg"`
		}
		_ = json.Unmarshal(errBody, &apiErr)
		return nil, apperrors.NewUnauthorizedf("alipay error (%d %s): %s %s", apiErr.Code, apiErr.Msg, apiErr.SubCode, apiErr.SubMsg)
	}
	return resp, nil
}

// alipayTokenResponse is the alipay.system.oauth.token business payload.
type alipayTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	ReExpiresIn      int64  `json:"re_expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	OpenID           string `json:"open_id"`
	AlipayUserID     string `json:"alipay_user_id"`
	UserID           string `json:"user_id"`
}

// ExchangeCode exchanges the authorization code via alipay.system.oauth.token.
func (p *AlipayProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	resp, err := p.alipayRequest(ctx, "alipay.system.oauth.token", map[string]string{
		"grant_type": "authorization_code",
		"code":       code,
	})
	if err != nil {
		return nil, err
	}

	raw, ok := resp["alipay_system_oauth_token_response"]
	if !ok {
		return nil, apperrors.NewUnauthorized("alipay token response is missing alipay_system_oauth_token_response")
	}
	var tokenResp alipayTokenResponse
	if err := json.Unmarshal(raw, &tokenResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse token response")
	}
	if tokenResp.AccessToken == "" {
		return nil, apperrors.NewUnauthorized("alipay token response did not include an access token")
	}

	id := firstNonEmptyString(tokenResp.OpenID, tokenResp.UserID, tokenResp.AlipayUserID)
	return &OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
		ID:           id,
		OpenID:       id,
	}, nil
}

// alipayUserInfoResponse is the alipay.user.info.share business payload.
type alipayUserInfoResponse struct {
	Code     string `json:"code"`
	Msg      string `json:"msg"`
	SubCode  string `json:"sub_code"`
	SubMsg   string `json:"sub_msg"`
	OpenID   string `json:"open_id"`
	UserID   string `json:"user_id"`
	NickName string `json:"nick_name"`
	Avatar   string `json:"avatar"`
	Province string `json:"province"`
	City     string `json:"city"`
}

// GetUserInfo retrieves the profile via alipay.user.info.share using the
// OAuth token as auth_token.
func (p *AlipayProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	resp, err := p.alipayRequest(ctx, "alipay.user.info.share", map[string]string{
		"auth_token": token.AccessToken,
	})
	if err != nil {
		return nil, err
	}

	raw, ok := resp["alipay_user_info_share_response"]
	if !ok {
		return nil, apperrors.NewUnauthorized("alipay user info response is missing alipay_user_info_share_response")
	}
	var userResp alipayUserInfoResponse
	if err := json.Unmarshal(raw, &userResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse user info")
	}
	if userResp.Code != "10000" {
		return nil, apperrors.NewUnauthorizedf("alipay user info error (%s %s): %s %s", userResp.Code, userResp.Msg, userResp.SubCode, userResp.SubMsg)
	}

	id := firstNonEmptyString(userResp.OpenID, userResp.UserID)
	if id == "" {
		id = token.ID
	}
	return &UserInfo{
		ID:          id,
		Username:    userResp.NickName,
		DisplayName: userResp.NickName,
		AvatarURL:   userResp.Avatar,
		OpenID:      id,
	}, nil
}
