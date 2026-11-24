package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// BrevoEmailSender sends transactional email through Brevo (ex-Sendinblue)
// (POST https://api.brevo.com/v3/smtp/email, header api-key).
// https://developers.brevo.com/reference/send-transac-email
type BrevoEmailSender struct {
	APIKey string
	From   string
	Name   string // sender display name
	client *http.Client
}

// NewBrevoEmailSender creates a Brevo sender.
func NewBrevoEmailSender(apiKey, from, name string) *BrevoEmailSender {
	return &BrevoEmailSender{
		APIKey: apiKey,
		From:   from,
		Name:   name,
		client: ssrf.DefaultHTTPClient,
	}
}

// Send delivers one message via POST /v3/smtp/email (JSON, textContent).
func (s *BrevoEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.APIKey == "" {
		return apperrors.NewInvalidArgument("brevo API key is required")
	}
	if s.From == "" {
		return apperrors.NewInvalidArgument("brevo sender email is required")
	}
	sender := map[string]string{"email": s.From}
	if s.Name != "" {
		sender["name"] = s.Name
	}
	payload, err := json.Marshal(map[string]any{
		"sender":      sender,
		"to":          []map[string]string{{"email": to}},
		"subject":     subject,
		"textContent": body,
	})
	if err != nil {
		return apperrors.WrapInternalf(err, "brevo marshal payload")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", s.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "brevo send")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapUnavailablef(err, "brevo read response")
	}
	if resp.StatusCode >= 300 {
		var parsed struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &parsed)
		return apperrors.NewInternalf("brevo send failed: %s %s", parsed.Code, parsed.Message)
	}
	return nil
}

var _ EmailSender = (*BrevoEmailSender)(nil)
