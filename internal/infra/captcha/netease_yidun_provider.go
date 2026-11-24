package captcha

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// NetEaseYidunProvider verifies NetEase Yidun (网易易盾) CAPTCHA tokens via the
// server-side second-check API.
//
// Official contract (verified 2026-11 against the Yidun CAPTCHA v2 server-side
// check documentation and the official PHP SDK at
// https://github.com/yidun/captcha-php-demo):
//   - POST https://c.dun.163.com/api/v2/verify with a form-urlencoded body of
//     captchaId, validate, user (optional), secretId, version="v2", timestamp
//     (milliseconds), nonce and signature.
//   - signature = MD5 over every request parameter (excluding signature) sorted
//     by ascending parameter name, concatenated as key+value with no separator,
//     then the business secret appended, rendered as 32-char lowercase hex.
//   - Response {"result": bool, "error": int}: result=true means the second
//     check passed; a transport/HTTP/decode failure means the provider is
//     unavailable, not a failed verification.
//
// The widget hands the client a `validate` token, so the verify token is that
// string verbatim. Three credentials are required (captchaId, secretId and the
// business secret); the two-slot unified mapping keeps captchaId/secret and the
// secretId travels in the provider config (see NewProviderFromRow).
type NetEaseYidunProvider struct {
	CaptchaID string
	SecretID  string
	SecretKey string

	Endpoint string
	client   *http.Client
}

// NewNetEaseYidunProvider creates a NetEase Yidun CAPTCHA provider.
func NewNetEaseYidunProvider(captchaID, secretID, secretKey string) *NetEaseYidunProvider {
	return &NetEaseYidunProvider{
		CaptchaID: captchaID,
		SecretID:  secretID,
		SecretKey: secretKey,
		Endpoint:  "https://c.dun.163.com/api/v2/verify",
		client:    ssrf.DefaultHTTPClient,
	}
}

// Verify submits the widget's validate token for the server-side second check.
func (p *NetEaseYidunProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if p.CaptchaID == "" || p.SecretID == "" || p.SecretKey == "" {
		return false, apperrors.NewUnavailable("netease yidun captcha: credentials not configured")
	}
	if token == "" {
		return false, apperrors.NewInvalidArgument("netease yidun captcha: validate token is required")
	}

	params := url.Values{}
	params.Set("captchaId", p.CaptchaID)
	params.Set("validate", token)
	params.Set("secretId", p.SecretID)
	params.Set("version", "v2")
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("nonce", strconv.FormatInt(time.Now().UnixNano()%1e9, 10))
	params.Set("signature", yidunSignature(p.SecretKey, params))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "netease yidun captcha: build request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "netease yidun captcha verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("netease yidun captcha verify: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Result bool  `json:"result"`
		Error  int64 `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, apperrors.WrapUnavailablef(err, "netease yidun captcha: decode response")
	}
	if result.Error != 0 && !result.Result {
		// Non-zero error codes with result=false are API-level rejections of the
		// request (bad signature, expired token); the documented pass signal is
		// result=true, so anything else is a failed verification.
		return false, nil
	}
	return result.Result, nil
}

// yidunSignature implements the documented signing rule: sort the parameters by
// name, concatenate key+value pairs, append the business secret, MD5 lowercase.
func yidunSignature(secretKey string, params url.Values) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, k := range keys {
		builder.WriteString(k)
		builder.WriteString(params.Get(k))
	}
	builder.WriteString(secretKey)

	sum := md5.Sum([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

var _ Provider = (*NetEaseYidunProvider)(nil)
