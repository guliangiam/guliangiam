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

// africastalkingDefaultBaseURL is the Africa's Talking SMS host.
const africastalkingDefaultBaseURL = "https://api.africastalking.com"

// AfricastalkingSmsSender sends SMS through the Africa's Talking messaging
// API (POST /version1/messaging with username + apiKey headers).
type AfricastalkingSmsSender struct {
	Username string
	APIKey   string
	From     string
	BaseURL  string
	client   *http.Client
}

// NewAfricastalkingSmsSender creates an Africa's Talking SMS sender.
func NewAfricastalkingSmsSender(username, apiKey, from string) *AfricastalkingSmsSender {
	return &AfricastalkingSmsSender{
		Username: username,
		APIKey:   apiKey,
		From:     from,
		BaseURL:  africastalkingDefaultBaseURL,
		client:   ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send posts a verification SMS to the Africa's Talking messaging endpoint.
func (s *AfricastalkingSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.Username == "" || s.APIKey == "" {
		return apperrors.NewInvalidArgument("africastalking username and apiKey are required")
	}

	form := url.Values{}
	form.Set("to", phone)
	form.Set("message", "Your verification code is: "+code)
	if s.From != "" {
		form.Set(fieldFrom, s.From)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/version1/messaging", strings.NewReader(form.Encode()))
	if err != nil {
		return apperrors.WrapInternalf(err, "africastalking: create request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("username", s.Username)
	req.Header.Set("apiKey", s.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "africastalking: request")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapInternalf(err, "africastalking: read response")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("africastalking: rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("africastalking: API error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		SMSMessageData struct {
			Message    string `json:"Message"`
			Recipients []struct {
				StatusCode string `json:"statusCode"`
				Message    string `json:"message"`
				Number     string `json:"number"`
			} `json:"Recipients"`
		} `json:"SMSMessageData"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return apperrors.WrapInternalf(err, "africastalking: decode response")
	}
	for _, recipient := range result.SMSMessageData.Recipients {
		if recipient.StatusCode != "Success" && recipient.StatusCode != "Queued" {
			return apperrors.NewInternalf("africastalking: send failed (%s: %s)", recipient.StatusCode, recipient.Message)
		}
	}
	if len(result.SMSMessageData.Recipients) == 0 {
		return apperrors.NewInternalf("africastalking: no recipients accepted")
	}
	return nil
}

var _ SmsSender = (*AfricastalkingSmsSender)(nil)
