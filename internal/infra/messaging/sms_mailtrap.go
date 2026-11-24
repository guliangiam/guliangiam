package messaging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func NewMailtrapSmsSender(apiKey, fromNumber string) *MailtrapSmsSender {
	return &MailtrapSmsSender{
		APIKey:     apiKey,
		FromNumber: fromNumber,
		client:     ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send sends an SMS via Mailtrap SMS API.
func (s *MailtrapSmsSender) Send(ctx context.Context, phone, code string) error {
	apiURL := "https://sms.api.mailtrap.io/api/v1/sms"

	payload := map[string]any{
		fieldFrom: s.FromNumber,
		"to":      phone,
		fieldText: fmt.Sprintf("Your verification code is: %s", code),
	}

	jsonData, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "mailtrap: marshal")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, jsonReader(jsonData))
	if err != nil {
		return apperrors.WrapInternalf(err, "mailtrap: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "mailtrap: request")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("mailtrap: API error (status %d)", resp.StatusCode)
	}
	return nil
}

// Ensure MailtrapSmsSender implements SmsSender.
var _ SmsSender = (*MailtrapSmsSender)(nil)

// CustomHttpSmsSender sends SMS via a custom HTTP webhook.
type CustomHttpSmsSender struct {
	APIURL       string
	HTTPMethod   string
	Headers      map[string]string
	BodyTemplate string
	client       *http.Client
}

// NewCustomHttpSmsSender creates a new custom HTTP SMS sender.
