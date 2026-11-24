package messaging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func NewOsonSmsSender(apiURL, apiKey, senderName string) *OsonSmsSender {
	return &OsonSmsSender{
		APIURL:     apiURL,
		APIKey:     apiKey,
		SenderName: senderName,
		client:     ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send sends an SMS via OSON SMS API.
func (s *OsonSmsSender) Send(ctx context.Context, phone, code string) error {
	payload := map[string]any{
		fieldFrom: s.SenderName,
		"to":      phone,
		fieldText: fmt.Sprintf("Your verification code is: %s", code),
	}

	jsonData, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "oson: marshal")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.APIURL, jsonReader(jsonData))
	if err != nil {
		return apperrors.WrapInternalf(err, "oson: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "oson: request")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("oson: API error (status %d)", resp.StatusCode)
	}
	return nil
}

// Ensure OsonSmsSender implements SmsSender.
var _ SmsSender = (*OsonSmsSender)(nil)

// BaiduSmsSender sends SMS via Baidu Cloud SMS.
type BaiduSmsSender struct {
	AccessKey  string
	SecretKey  string
	SignID     string
	TemplateID string
	InvokeID   string
	client     *http.Client
}

// NewBaiduSmsSender creates a new Baidu Cloud SMS sender.
