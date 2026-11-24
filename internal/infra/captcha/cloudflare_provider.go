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

// CloudflareTurnstileProvider verifies Cloudflare Turnstile tokens via
// https://challenges.cloudflare.com/turnstile/v0/siteverify (form-urlencoded
// secret/response/remoteip; success=false plus error-codes such as
// "invalid-input-response" or "timeout-or-duplicate" is a failed verification —
// tokens are single-use and expire after 300s per
// https://developers.cloudflare.com/turnstile/get-started/server-side-validation/,
// verified 2026-11).
type CloudflareTurnstileProvider struct {
	SecretKey string
	client    *http.Client
}

// NewCloudflareTurnstileProvider creates a new Cloudflare Turnstile provider.
func NewCloudflareTurnstileProvider(secretKey string) *CloudflareTurnstileProvider {
	return &CloudflareTurnstileProvider{SecretKey: secretKey, client: ssrf.DefaultHTTPClient}
}

// Verify posts to the official siteverify endpoint with a form-urlencoded
// body. success=false is a failed verification (false, nil); transport/HTTP/
// decode problems mean the provider is unavailable.
func (p *CloudflareTurnstileProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	data := url.Values{}
	data.Set("secret", p.SecretKey)
	data.Set("response", token)
	if remoteIP != "" {
		data.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(data.Encode()))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "turnstile verify request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "turnstile verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("turnstile verify: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, apperrors.WrapUnavailablef(err, "turnstile decode response")
	}

	return result.Success, nil
}
