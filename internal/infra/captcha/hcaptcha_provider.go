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

// hCaptchaVerifyURL is the official server-side verification endpoint
// (https://docs.hcaptcha.com/#verify-the-user-response-server-side:
// https://api.hcaptcha.com/siteverify, form-urlencoded secret/response/
// remoteip, error-codes mirror Google's siteverify vocabulary; verified 2026-11).
const hCaptchaVerifyURL = "https://api.hcaptcha.com/siteverify"

// HCaptchaProvider verifies hCaptcha tokens.
type HCaptchaProvider struct {
	SecretKey string
	client    *http.Client
}

// NewHCaptchaProvider creates a new hCaptcha provider.
func NewHCaptchaProvider(secretKey string) *HCaptchaProvider {
	return &HCaptchaProvider{SecretKey: secretKey, client: ssrf.DefaultHTTPClient}
}

// Verify posts secret/response/remoteip as a form-urlencoded body, which is
// what the endpoint expects. success=false is a failed verification;
// transport/HTTP/decode problems mean the provider is unavailable.
func (p *HCaptchaProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	data := url.Values{}
	data.Set("secret", p.SecretKey)
	data.Set("response", token)
	if remoteIP != "" {
		data.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hCaptchaVerifyURL, strings.NewReader(data.Encode()))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "hcaptcha verify request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "hcaptcha verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("hcaptcha verify: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, apperrors.WrapUnavailablef(err, "hcaptcha decode response")
	}

	return result.Success, nil
}
