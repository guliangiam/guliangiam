package messaging

import (
	"context"
	"net/http"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func NewCustomHttpSmsSender(apiURL, httpMethod string, headers map[string]string, bodyTemplate string) *CustomHttpSmsSender {
	return &CustomHttpSmsSender{
		APIURL:       apiURL,
		HTTPMethod:   httpMethod,
		Headers:      headers,
		BodyTemplate: bodyTemplate,
		client:       ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send sends an SMS via a custom HTTP endpoint.
func (s *CustomHttpSmsSender) Send(ctx context.Context, phone, code string) error {
	payload := map[string]any{
		"phone":   phone,
		fieldCode: code,
	}

	jsonData, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "custom http sms: marshal")
	}

	method := s.HTTPMethod
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, s.APIURL, jsonReader(jsonData))
	if err != nil {
		return apperrors.WrapInternalf(err, "custom http sms: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "custom http sms: request")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("custom http sms: API error (status %d)", resp.StatusCode)
	}
	return nil
}

// Ensure CustomHttpSmsSender implements SmsSender.
var _ SmsSender = (*CustomHttpSmsSender)(nil)
