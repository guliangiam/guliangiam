package messaging

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ResendSender sends emails via the Resend API.
type ResendSender struct {
	APIKey string
	From   string
	client *http.Client
}

// NewResendSender creates a new Resend email sender.
func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{APIKey: apiKey, From: from, client: ssrf.DefaultHTTPClient}
}

func (s *ResendSender) Send(ctx context.Context, to, subject, body string) error {
	payload := map[string]interface{}{
		fieldFrom:    s.From,
		"to":         []string{to},
		fieldSubject: subject,
		fieldText:    body,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "resend marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", strings.NewReader(string(jsonPayload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "resend send email")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("resend send email failed: status %d", resp.StatusCode)
	}
	return nil
}
