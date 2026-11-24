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

// MailerSendEmailSender sends email through MailerSend's Transactional API
// (POST https://api.mailersend.com/v1/email, header Authorization: Bearer).
// https://developers.mailersend.com/api/v1/email
type MailerSendEmailSender struct {
	APIToken string
	From     string
	FromName string
	Endpoint string // default https://api.mailersend.com/v1/email
	client   *http.Client
}

// NewMailerSendEmailSender creates a MailerSend sender.
func NewMailerSendEmailSender(apiToken, from, fromName, endpoint string) *MailerSendEmailSender {
	if endpoint == "" {
		endpoint = "https://api.mailersend.com/v1/email"
	}
	return &MailerSendEmailSender{
		APIToken: apiToken,
		From:     from,
		FromName: fromName,
		Endpoint: endpoint,
		client:   ssrf.DefaultHTTPClient,
	}
}

// Send delivers one message via POST /v1/email (JSON, text body).
func (s *MailerSendEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.APIToken == "" {
		return apperrors.NewInvalidArgument("mailersend API token is required")
	}
	if s.From == "" {
		return apperrors.NewInvalidArgument("mailersend from address is required")
	}
	from := map[string]string{"email": s.From}
	if s.FromName != "" {
		from["name"] = s.FromName
	}
	payload, err := json.Marshal(map[string]any{
		"from":    from,
		"to":      []map[string]string{{"email": to}},
		"subject": subject,
		"text":    body,
	})
	if err != nil {
		return apperrors.WrapInternalf(err, "mailersend marshal payload")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "mailersend send")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapUnavailablef(err, "mailersend read response")
	}
	if resp.StatusCode >= 300 {
		var parsed struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &parsed)
		return apperrors.NewInternalf("mailersend send failed: %s", parsed.Message)
	}
	return nil
}

var _ EmailSender = (*MailerSendEmailSender)(nil)
