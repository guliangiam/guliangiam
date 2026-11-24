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

// SendCloudEmailSender sends email through SendCloud's Web API v2
// (POST https://api.sendcloud.net/apiv2/mail/send, form-encoded apiUser/apiKey
// credentials).
// https://www.sendcloud.net/doc/email_v2/send_email/
type SendCloudEmailSender struct {
	APIUser  string
	APIKey   string
	From     string // verified sending address (from)
	FromName string
	Endpoint string // default https://api.sendcloud.net/apiv2/mail/send
	client   *http.Client
}

// NewSendCloudEmailSender creates a SendCloud sender.
func NewSendCloudEmailSender(apiUser, apiKey, from, fromName, endpoint string) *SendCloudEmailSender {
	if endpoint == "" {
		endpoint = "https://api.sendcloud.net/apiv2/mail/send"
	}
	return &SendCloudEmailSender{
		APIUser:  apiUser,
		APIKey:   apiKey,
		From:     from,
		FromName: fromName,
		Endpoint: endpoint,
		client:   ssrf.DefaultHTTPClient,
	}
}

// Send delivers one email as text/plain (SendCloud "plain" field).
func (s *SendCloudEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.APIUser == "" || s.APIKey == "" {
		return apperrors.NewInvalidArgument("sendcloud apiUser/apiKey are required")
	}
	if s.From == "" {
		return apperrors.NewInvalidArgument("sendcloud from address is required")
	}
	form := url.Values{}
	form.Set("apiUser", s.APIUser)
	form.Set("apiKey", s.APIKey)
	form.Set("from", s.From)
	if s.FromName != "" {
		form.Set("fromName", s.FromName)
	}
	form.Set("to", to) // multiple recipients are ';'-separated per vendor spec
	form.Set("subject", subject)
	form.Set("plain", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "sendcloud send")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapUnavailablef(err, "sendcloud read response")
	}
	// SendCloud reports success as statusCode 200 inside the JSON body
	// (the field appears as a number in vendor examples, a string elsewhere).
	var parsed struct {
		StatusCode json.RawMessage `json:"statusCode"`
		Message    string          `json:"message"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return apperrors.NewInternalf("sendcloud send failed: status %d", resp.StatusCode)
	}
	code := strings.Trim(string(parsed.StatusCode), `"`)
	if code != "" && code != "200" {
		return apperrors.NewInternalf("sendcloud send failed: %s %s", code, parsed.Message)
	}
	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("sendcloud send failed: status %d", resp.StatusCode)
	}
	return nil
}

var _ EmailSender = (*SendCloudEmailSender)(nil)
