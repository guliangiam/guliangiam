package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// reCAPTCHAV2VerifyURL is Google's official siteverify endpoint, shared by v2 and v3.
// Contract verified 2026-11 against https://developers.google.com/recaptcha/docs/verify:
// POST application/x-www-form-urlencoded with secret (required), response (required),
// remoteip (optional); success=false plus error-codes (missing-input-secret,
// invalid-input-secret, missing-input-response, invalid-input-response, bad-request,
// timeout-or-duplicate) is a failed verification, not a transport error.
const reCAPTCHAV2VerifyURL = "https://www.google.com/recaptcha/api/siteverify"

// ReCAPTCHAV2Provider verifies Google reCAPTCHA v2 tokens.
type ReCAPTCHAV2Provider struct {
	SecretKey string
	client    *http.Client
}

// NewReCAPTCHAV2Provider creates a new reCAPTCHA v2 provider.
func NewReCAPTCHAV2Provider(secretKey string) *ReCAPTCHAV2Provider {
	return &ReCAPTCHAV2Provider{SecretKey: secretKey, client: ssrf.DefaultHTTPClient}
}

// Verify calls the official siteverify endpoint. Per Google's docs the
// parameters go in the POST body (application/x-www-form-urlencoded), not the
// query string. A success=false response is a failed verification (false, nil);
// transport/HTTP/decode problems mean the provider is unavailable.
func (p *ReCAPTCHAV2Provider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	data := url.Values{}
	data.Set("secret", p.SecretKey)
	data.Set("response", token)
	if remoteIP != "" {
		data.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reCAPTCHAV2VerifyURL, strings.NewReader(data.Encode()))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "recaptcha verify request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "recaptcha verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("recaptcha verify: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, apperrors.WrapUnavailablef(err, "recaptcha decode response")
	}

	return result.Success, nil
}
