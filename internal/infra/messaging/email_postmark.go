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

// PostmarkEmailSender sends email through Postmark's Message API
// (POST https://api.postmarkapp.com/email, header X-Postmark-Server-Token).
// https://postmarkapp.com/developer/user-guide/send-email-with-api
type PostmarkEmailSender struct {
	ServerToken string
	From        string
	Endpoint    string // default https://api.postmarkapp.com/email
	client      *http.Client
}

// NewPostmarkEmailSender creates a Postmark sender.
func NewPostmarkEmailSender(serverToken, from, endpoint string) *PostmarkEmailSender {
	if endpoint == "" {
		endpoint = "https://api.postmarkapp.com/email"
	}
	return &PostmarkEmailSender{
		ServerToken: serverToken,
		From:        from,
		Endpoint:    endpoint,
		client:      ssrf.DefaultHTTPClient,
	}
}

// Send delivers one message as TextBody (JSON payload).
func (s *PostmarkEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.ServerToken == "" {
		return apperrors.NewInvalidArgument("postmark server token is required")
	}
	if s.From == "" {
		return apperrors.NewInvalidArgument("postmark from address is required")
	}
	payload, err := json.Marshal(map[string]string{
		"From":     s.From,
		"To":       to,
		"Subject":  subject,
		"TextBody": body,
	})
	if err != nil {
		return apperrors.WrapInternalf(err, "postmark marshal payload")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Postmark-Server-Token", s.ServerToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "postmark send")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapUnavailablef(err, "postmark read response")
	}
	if resp.StatusCode >= 300 {
		var parsed struct {
			Message string `json:"Message"`
		}
		_ = json.Unmarshal(raw, &parsed)
		return apperrors.NewInternalf("postmark send failed: %s", parsed.Message)
	}
	// 2xx with an "ErrorCode" in the body still means rejection.
	var envelope struct {
		ErrorCode *int   `json:"ErrorCode"`
		Message   string `json:"Message"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.ErrorCode != nil {
		return apperrors.NewInternalf("postmark send failed: %d %s", *envelope.ErrorCode, envelope.Message)
	}
	return nil
}

var _ EmailSender = (*PostmarkEmailSender)(nil)
