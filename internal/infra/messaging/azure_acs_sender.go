package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AzureACSSender sends emails via Azure Communication Services.
type AzureACSSender struct {
	Endpoint string
	APIKey   string
	From     string
	client   *http.Client
}

// NewAzureACSSender creates a new Azure ACS email sender.
func NewAzureACSSender(endpoint, apiKey, from string) *AzureACSSender {
	return &AzureACSSender{Endpoint: endpoint, APIKey: apiKey, From: from, client: ssrf.DefaultHTTPClient}
}

func (s *AzureACSSender) Send(ctx context.Context, to, subject, body string) error {
	apiURL := fmt.Sprintf("%s/emails:send?api-version=2023-03-31", s.Endpoint)

	payload := map[string]interface{}{
		fieldContent: map[string]interface{}{
			fieldSubject: subject,
			"plainText":  body,
		},
		"recipients": map[string]interface{}{
			"to": []map[string]interface{}{
				{"address": to},
			},
		},
		"senderAddress": s.From,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "azure acs marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(string(jsonPayload)))
	if err != nil {
		return err
	}
	// ACS REST API 认证使用 Basic 方案：连接串里的 accesskey 已是 base64，
	// 直接作为 "Authorization: Basic <accesskey>" 发送（官方 SDK 同行为）。
	req.Header.Set("Authorization", "Basic "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "azure acs send email")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("azure acs send email failed: status %d", resp.StatusCode)
	}
	return nil
}
