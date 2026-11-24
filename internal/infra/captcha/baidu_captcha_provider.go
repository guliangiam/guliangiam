package captcha

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BaiduCaptchaProvider verifies Baidu Security Verification (百度安全验证)
// tokens via the server-side check API.
//
// Official contract (Baidu Security Verification server-side check,
// https://help.baidu.com "安全验证-服务端校验" — the documentation site is
// script-rendered and not machine-readable, so this shape is pinned against the
// official parameter vocabulary mirrored by public SDK samples, verified
// 2026-11; live-credential validation is tracked as an L4 boundary):
//   - POST https://sec-captcha.baidu.com/api/v1/check with a form-urlencoded
//     body of user_ip, user_token, session_id, device_id, app_id, app_name,
//     user_response and signature.
//   - signature = MD5(user_token + user_ip + private_key), 32-char lowercase.
//   - Response {"error_code": int, "result": {"verify_result": bool}}:
//     error_code != 0 is an API-level failure (provider unavailable); a zero
//     error code carries the verdict in result.verify_result.
//
// The widget returns the user_token/user_response/session_id/device_id bundle,
// so the verify token is the URL-encoded query string carrying those four
// fields — the same composite-token shape GeeTest v4 and Tencent CAPTCHA use.
// Credentials map app_id to the site-key slot and private_key to the secret
// slot; the optional app_name travels in the provider config.
type BaiduCaptchaProvider struct {
	AppID      string
	PrivateKey string
	AppName    string

	Endpoint string
	client   *http.Client
}

// NewBaiduCaptchaProvider creates a Baidu Security Verification provider.
func NewBaiduCaptchaProvider(appID, privateKey, appName string) *BaiduCaptchaProvider {
	return &BaiduCaptchaProvider{
		AppID:      appID,
		PrivateKey: privateKey,
		AppName:    appName,
		Endpoint:   "https://sec-captcha.baidu.com/api/v1/check",
		client:     ssrf.DefaultHTTPClient,
	}
}

// Verify performs the server-side check. The token must be the URL-encoded
// "user_token=...&user_response=...&session_id=..." bundle produced by the
// widget; remoteIP is the mandatory user_ip input.
func (p *BaiduCaptchaProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if p.AppID == "" || p.PrivateKey == "" {
		return false, apperrors.NewUnavailable("baidu captcha: credentials not configured")
	}
	if remoteIP == "" {
		return false, apperrors.NewInvalidArgument("baidu captcha: user ip is required")
	}
	bundle, err := parseBaiduCaptchaToken(token)
	if err != nil {
		return false, err
	}

	params := url.Values{}
	params.Set("app_id", p.AppID)
	if p.AppName != "" {
		params.Set("app_name", p.AppName)
	}
	params.Set("user_ip", remoteIP)
	params.Set("user_token", bundle.Get("user_token"))
	params.Set("user_response", bundle.Get("user_response"))
	params.Set("session_id", bundle.Get("session_id"))
	if deviceID := bundle.Get("device_id"); deviceID != "" {
		params.Set("device_id", deviceID)
	}
	params.Set("signature", baiduCaptchaSignature(bundle.Get("user_token"), remoteIP, p.PrivateKey))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "baidu captcha: build request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "baidu captcha verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("baidu captcha verify: unexpected status %d", resp.StatusCode)
	}

	var envelope struct {
		ErrorCode int    `json:"error_code"`
		Message   string `json:"message"`
		Result    struct {
			VerifyResult bool `json:"verify_result"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return false, apperrors.WrapUnavailablef(err, "baidu captcha: decode response")
	}
	if envelope.ErrorCode != 0 {
		return false, apperrors.NewUnavailablef("baidu captcha API error %d: %s", envelope.ErrorCode, envelope.Message)
	}
	return envelope.Result.VerifyResult, nil
}

// parseBaiduCaptchaToken extracts the widget bundle from the composite token.
// user_token and user_response are mandatory inputs; session_id is required by
// the check API, device_id is optional.
func parseBaiduCaptchaToken(token string) (url.Values, error) {
	q, err := url.ParseQuery(token)
	if err != nil || q.Get("user_token") == "" || q.Get("user_response") == "" || q.Get("session_id") == "" {
		return nil, apperrors.NewInvalidArgument(`baidu captcha: token must be the URL-encoded "user_token=...&user_response=...&session_id=..." bundle`)
	}
	return q, nil
}

// baiduCaptchaSignature implements the documented MD5(user_token + user_ip +
// private_key) signing rule.
func baiduCaptchaSignature(userToken, userIP, privateKey string) string {
	sum := md5.Sum([]byte(userToken + userIP + privateKey))
	return hex.EncodeToString(sum[:])
}

var _ Provider = (*BaiduCaptchaProvider)(nil)
