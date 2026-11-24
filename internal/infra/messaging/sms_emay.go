package messaging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func NewEmaySmsSender(apiURL, serialNumber, password, signName string) *EmaySmsSender {
	return &EmaySmsSender{
		APIURL:       apiURL,
		SerialNumber: serialNumber,
		Password:     password,
		SignName:     signName,
		client:       ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send sends an SMS via Emay SMS API.
func (s *EmaySmsSender) Send(ctx context.Context, phone, code string) error {
	payload := map[string]any{
		"serialNumber": s.SerialNumber,
		"password":     s.Password,
		"phone":        phone,
		fieldContent:   fmt.Sprintf("[%s] Your verification code is: %s, please use within 10 minutes.", s.SignName, code),
	}

	jsonData, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "emay: marshal")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.APIURL, jsonReader(jsonData))
	if err != nil {
		return apperrors.WrapInternalf(err, "emay: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "emay: request")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("emay: API error (status %d)", resp.StatusCode)
	}
	return nil
}

// Ensure EmaySmsSender implements SmsSender.
var _ SmsSender = (*EmaySmsSender)(nil)

// MockSmsSender is a test SMS sender that only logs messages.
type MockSmsSender struct{}

// NewMockSmsSender creates a new mock SMS sender.
