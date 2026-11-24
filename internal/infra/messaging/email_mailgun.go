package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// MailgunEmailSender sends email through Mailgun's Messages API
// (POST https://api.mailgun.net/v3/{domain}/messages, HTTP Basic auth with
// username "api" and the private API key).
// https://documentation.mailgun.com/docs/mailgun/api-reference/send/mailgun/messages
type MailgunEmailSender struct {
	APIKey  string
	Domain  string // sending domain (sandbox or verified)
	From    string
	BaseURL string // default https://api.mailgun.net (EU: https://api.eu.mailgun.net)
	client  *http.Client
}

// NewMailgunEmailSender creates a Mailgun sender.
func NewMailgunEmailSender(apiKey, domain, from, baseURL string) *MailgunEmailSender {
	if baseURL == "" {
		baseURL = "https://api.mailgun.net"
	}
	return &MailgunEmailSender{
		APIKey:  apiKey,
		Domain:  domain,
		From:    from,
		BaseURL: strings.TrimRight(baseURL, "/"),
		client:  ssrf.DefaultHTTPClient,
	}
}

// Send delivers one message via POST /v3/{domain}/messages (form-encoded).
func (s *MailgunEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.APIKey == "" {
		return apperrors.NewInvalidArgument("mailgun private API key is required")
	}
	if s.Domain == "" {
		return apperrors.NewInvalidArgument("mailgun sending domain is required")
	}
	from := s.From
	if from == "" {
		from = "postmaster@" + s.Domain
	}
	form := url.Values{}
	form.Set("from", from)
	form.Set("to", to)
	form.Set("subject", subject)
	form.Set("text", body)

	endpoint := s.BaseURL + "/v3/" + url.PathEscape(s.Domain) + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("api", s.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "mailgun send")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapUnavailablef(err, "mailgun read response")
	}
	if resp.StatusCode >= 300 {
		var parsed struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &parsed)
		return apperrors.NewInternalf("mailgun send failed: %s", parsed.Message)
	}
	return nil
}

var _ EmailSender = (*MailgunEmailSender)(nil)
