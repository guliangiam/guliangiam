package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// clickatellDefaultBaseURL is the Clickatell Messages API host.
const clickatellDefaultBaseURL = "https://api.clickatell.com"

// ClickatellSmsSender sends SMS through the Clickatell Messages API
// (POST /messages with a Bearer API key).
type ClickatellSmsSender struct {
	APIKey  string
	From    string
	BaseURL string
	client  *http.Client
}

// NewClickatellSmsSender creates a Clickatell SMS sender.
func NewClickatellSmsSender(apiKey, from string) *ClickatellSmsSender {
	return &ClickatellSmsSender{
		APIKey:  apiKey,
		From:    from,
		BaseURL: clickatellDefaultBaseURL,
		client:  ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send posts a verification SMS to the Clickatell Messages API.
func (s *ClickatellSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.APIKey == "" {
		return apperrors.NewInvalidArgument("clickatell apiKey is required")
	}

	payload := map[string]any{
		"to":      phone,
		fieldText: "Your verification code is: " + code,
	}
	if s.From != "" {
		payload[fieldFrom] = s.From
	}
	body, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "clickatell: marshal")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/messages", jsonReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "clickatell: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "clickatell: request")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapInternalf(err, "clickatell: read response")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("clickatell: rate limited (429)")
	}

	var result struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		if resp.StatusCode >= 400 {
			return apperrors.NewInternalf("clickatell: API error (status %d)", resp.StatusCode)
		}
		return apperrors.WrapInternalf(err, "clickatell: decode response")
	}
	if len(result.Errors) > 0 {
		e := result.Errors[0]
		return apperrors.NewInternalf("clickatell: send failed (code %s, %s)", e.Code, e.Message)
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("clickatell: API error (status %d)", resp.StatusCode)
	}
	return nil
}

var _ SmsSender = (*ClickatellSmsSender)(nil)
