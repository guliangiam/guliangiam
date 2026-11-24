package captcha

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// geeTestValidateURL is the GeeTest CAPTCHA v4 second-check endpoint. Contract
// verified 2026-11 against https://docs.geetest.com/gt4/deploy/server (the
// legacy /geetest4/captcha_developer/check path now redirects there):
// GET/POST application/x-www-form-urlencoded to gcaptcha4.geetest.com/validate
// with captcha_id in the query string and lot_number/captcha_output/pass_token/
// gen_time/sign_token in the body; sign_token = hex HMAC-SHA256(lot_number,
// captcha_key); status=="error" carries code/msg, otherwise result is
// success/fail with reason.
const geeTestValidateURL = "https://gcaptcha4.geetest.com/validate"

// GeeTestProvider verifies GeeTest CAPTCHA v4 tokens.
type GeeTestProvider struct {
	CaptchaID  string
	CaptchaKey string
	client     *http.Client
}

// NewGeeTestProvider creates a new GeeTest CAPTCHA provider. CaptchaID is the
// public key, CaptchaKey the private key used to sign the validation request.
func NewGeeTestProvider(captchaID, captchaKey string) *GeeTestProvider {
	return &GeeTestProvider{CaptchaID: captchaID, CaptchaKey: captchaKey, client: ssrf.DefaultHTTPClient}
}

// Verify performs the v4 second check. The token must carry the four client
// parameters produced by the GeeTest widget as a URL-encoded query string
// (lot_number/captcha_output/pass_token/gen_time). result=="fail" is a failed
// verification (false, nil); request or protocol errors mean the provider is
// unavailable.
func (p *GeeTestProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	params, err := url.ParseQuery(token)
	if err != nil || params.Get("lot_number") == "" {
		return false, apperrors.NewInvalidArgument("geetest: token must contain lot_number/captcha_output/pass_token/gen_time")
	}

	form := url.Values{}
	form.Set("lot_number", params.Get("lot_number"))
	form.Set("captcha_output", params.Get("captcha_output"))
	form.Set("pass_token", params.Get("pass_token"))
	form.Set("gen_time", params.Get("gen_time"))
	form.Set("sign_token", geeTestSignToken(p.CaptchaKey, params.Get("lot_number")))

	captchaID := p.CaptchaID
	if id := params.Get("captcha_id"); id != "" {
		captchaID = id
	}
	endpoint := geeTestValidateURL + "?captcha_id=" + url.QueryEscape(captchaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "geetest verify request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "geetest verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("geetest verify: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Status string `json:"status"`
		Result string `json:"result"`
		Reason string `json:"reason"`
		Code   string `json:"code"`
		Msg    string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, apperrors.WrapUnavailablef(err, "geetest decode response")
	}

	if result.Status == "error" {
		return false, apperrors.NewUnavailablef("geetest verify error %s: %s", result.Code, result.Msg)
	}
	return result.Result == "success", nil
}

// geeTestSignToken is the documented signature: hex HMAC-SHA256 of lot_number
// keyed with the captcha private key.
func geeTestSignToken(captchaKey, lotNumber string) string {
	mac := hmac.New(sha256.New, []byte(captchaKey))
	mac.Write([]byte(lotNumber))
	return hex.EncodeToString(mac.Sum(nil))
}
