package messaging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func NewInfobipSmsSender(baseURL, apiKey, senderName string) *InfobipSmsSender {
	return &InfobipSmsSender{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		SenderName: senderName,
		client:     ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send sends an SMS via Infobip SMS API.
func (s *InfobipSmsSender) Send(ctx context.Context, phone, code string) error {
	apiURL := fmt.Sprintf("%s/sms/2/text/advanced", s.BaseURL)

	payload := map[string]any{
		"messages": []map[string]any{
			{
				fieldFrom: s.SenderName,
				"destinations": []map[string]any{
					{"to": phone},
				},
				fieldText: fmt.Sprintf("Your verification code is: %s", code),
			},
		},
	}

	jsonData, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "infobip: marshal")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, jsonReader(jsonData))
	if err != nil {
		return apperrors.WrapInternalf(err, "infobip: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "App "+s.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "infobip: request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("infobip: rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("infobip: API error (status %d)", resp.StatusCode)
	}
	return nil
}

// Ensure InfobipSmsSender implements SmsSender.
var _ SmsSender = (*InfobipSmsSender)(nil)

// UCloudSmsSender sends SMS via UCLOUD (优刻得).
type UCloudSmsSender struct {
	PublicKey  string
	PrivateKey string
	ProjectID  string
	SignName   string
	TemplateID string
	client     *http.Client
}

// NewUCloudSmsSender creates a new UCLOUD SMS sender.
