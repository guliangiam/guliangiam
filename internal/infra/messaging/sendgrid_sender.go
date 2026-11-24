package messaging

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SendGridSender sends emails via the SendGrid v3 Mail Send API.
type SendGridSender struct {
	APIKey string
	From   string
	client *http.Client
}

// NewSendGridSender creates a new SendGrid email sender.
func NewSendGridSender(apiKey, from string) *SendGridSender {
	return &SendGridSender{APIKey: apiKey, From: from, client: ssrf.DefaultHTTPClient}
}

func (s *SendGridSender) Send(ctx context.Context, to, subject, body string) error {
	payload := map[string]interface{}{
		"personalizations": []map[string]interface{}{
			{
				"to": []map[string]string{{"email": to}},
			},
		},
		fieldFrom:    map[string]string{"email": s.From},
		fieldSubject: subject,
		fieldContent: []map[string]string{
			{"type": "text/plain", "value": body},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "sendgrid marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.sendgrid.com/v3/mail/send", strings.NewReader(string(jsonPayload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "sendgrid send email")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("sendgrid send email failed: status %d", resp.StatusCode)
	}
	return nil
}
