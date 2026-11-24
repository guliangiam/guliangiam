package messaging

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// telnyxDefaultBaseURL is the Telnyx Messaging API host.
const telnyxDefaultBaseURL = "https://api.telnyx.com"

// TelnyxSmsSender sends SMS through the Telnyx Messages API
// (POST /v2/messages with a Bearer API key).
type TelnyxSmsSender struct {
	APIKey  string
	From    string
	BaseURL string
	client  *http.Client
}

// NewTelnyxSmsSender creates a Telnyx SMS sender.
func NewTelnyxSmsSender(apiKey, from string) *TelnyxSmsSender {
	return &TelnyxSmsSender{
		APIKey:  apiKey,
		From:    from,
		BaseURL: telnyxDefaultBaseURL,
		client:  ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send posts a verification SMS to the Telnyx Messages API.
func (s *TelnyxSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.APIKey == "" {
		return apperrors.NewInvalidArgument("telnyx apiKey is required")
	}

	payload := map[string]any{
		fieldFrom: s.From,
		"to":      phone,
		fieldText: "Your verification code is: " + code,
	}
	body, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "telnyx: marshal")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/v2/messages", jsonReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "telnyx: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "telnyx: request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("telnyx: rate limited (429)")
	}

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
		Errors []struct {
			Code   string `json:"code"`
			Title  string `json:"title"`
			Detail string `json:"detail"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if resp.StatusCode >= 400 {
			return apperrors.NewInternalf("telnyx: API error (status %d)", resp.StatusCode)
		}
		return apperrors.WrapInternalf(err, "telnyx: decode response")
	}
	if len(result.Errors) > 0 {
		e := result.Errors[0]
		return apperrors.NewInternalf("telnyx: send failed (code %s, %s: %s)", e.Code, e.Title, e.Detail)
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("telnyx: API error (status %d)", resp.StatusCode)
	}
	if result.Data.ID == "" {
		return apperrors.NewInternalf("telnyx: missing message id in response")
	}
	return nil
}

var _ SmsSender = (*TelnyxSmsSender)(nil)
