package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// vonageDefaultBaseURL is the Vonage (Nexmo) SMS REST endpoint.
const vonageDefaultBaseURL = "https://rest.nexmo.com"

// VonageSmsSender sends SMS through the Vonage Messages (legacy SMS) API:
// POST /sms/json with api_key/api_secret form credentials.
type VonageSmsSender struct {
	APIKey    string
	APISecret string
	From      string
	BaseURL   string
	client    *http.Client
}

// NewVonageSmsSender creates a Vonage SMS sender.
func NewVonageSmsSender(apiKey, apiSecret, from string) *VonageSmsSender {
	return &VonageSmsSender{
		APIKey:    apiKey,
		APISecret: apiSecret,
		From:      from,
		BaseURL:   vonageDefaultBaseURL,
		client:    ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send posts a verification SMS to rest.nexmo.com/sms/json.
func (s *VonageSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.APIKey == "" || s.APISecret == "" {
		return apperrors.NewInvalidArgument("vonage apiKey and apiSecret are required")
	}

	form := url.Values{}
	form.Set("api_key", s.APIKey)
	form.Set("api_secret", s.APISecret)
	form.Set("to", strings.TrimPrefix(phone, "+"))
	form.Set("from", s.From)
	form.Set("text", "Your verification code is: "+code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/sms/json", strings.NewReader(form.Encode()))
	if err != nil {
		return apperrors.WrapInternalf(err, "vonage: create request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "vonage: request")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapInternalf(err, "vonage: read response")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("vonage: rate limited (429)")
	}

	var result struct {
		Messages []struct {
			Status    string `json:"status"`
			ErrorText string `json:"error-text"`
			MessageID string `json:"message-id"`
		} `json:"messages"`
		ErrorCode string `json:"error-code"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		if resp.StatusCode >= 400 {
			return apperrors.NewInternalf("vonage: API error (status %d)", resp.StatusCode)
		}
		return apperrors.WrapInternalf(err, "vonage: decode response")
	}
	if result.ErrorCode != "" && result.ErrorCode != "200" {
		return apperrors.NewInternalf("vonage: rejected (error-code %s)", result.ErrorCode)
	}
	// Vonage answers HTTP 200 even on failure; status "0" means accepted.
	if len(result.Messages) == 0 {
		if resp.StatusCode >= 400 {
			return apperrors.NewInternalf("vonage: API error (status %d)", resp.StatusCode)
		}
		return apperrors.NewInternalf("vonage: empty response")
	}
	msg := result.Messages[0]
	if msg.Status != "0" {
		return apperrors.NewInternalf("vonage: send failed (status %s, %s)", msg.Status, msg.ErrorText)
	}
	return nil
}

var _ SmsSender = (*VonageSmsSender)(nil)
