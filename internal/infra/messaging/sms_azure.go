package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func NewAzureACSsmsSender(connectionString, fromNumber string) *AzureACSsmsSender {
	return &AzureACSsmsSender{
		ConnectionString: connectionString,
		FromNumber:       fromNumber,
		client:           ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// azureACSConnString parses "endpoint=https://<resource>.communication.azure.com/;accesskey=<base64>".
func azureACSConnString(conn string) (endpoint, accessKey string, err error) {
	for _, part := range strings.Split(conn, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "endpoint":
			endpoint = strings.TrimRight(strings.TrimSpace(value), "/")
		case "accesskey":
			accessKey = strings.TrimSpace(value)
		}
	}
	if endpoint == "" || accessKey == "" {
		return "", "", apperrors.NewInvalidArgument("azure acs sms: connection string must contain endpoint and accesskey")
	}
	return endpoint, accessKey, nil
}

// Send sends an SMS via the Azure Communication Services SMS REST API.
// POST {endpoint}/sms?api-version=2021-03-07 with "Authorization: Basic <accesskey>"
// (the connection-string accesskey is already base64 and is sent verbatim).
// https://learn.microsoft.com/en-us/rest/api/communication/sms/send
func (s *AzureACSsmsSender) Send(ctx context.Context, phone, code string) error {
	endpoint, accessKey, err := azureACSConnString(s.ConnectionString)
	if err != nil {
		return err
	}

	recipient := phone
	if !strings.HasPrefix(phone, "+") {
		recipient = "+86" + phone
	}

	apiURL := fmt.Sprintf("%s/sms?api-version=2021-03-07", endpoint)
	payload := map[string]interface{}{
		"from":    s.FromNumber,
		"message": fmt.Sprintf("Your verification code is: %s", code),
		"smsRecipients": []map[string]interface{}{
			{"to": recipient},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "azure acs sms marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+accessKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "azure acs sms send")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return apperrors.WrapInternalf(err, "azure acs sms read response")
	}

	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("azure acs sms send failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	// 202 Accepted may still carry per-recipient failures in the value list.
	var parsed struct {
		Value []struct {
			Successful   bool   `json:"successful"`
			ErrorMessage string `json:"errorMessage"`
			HTTPStatus   int    `json:"httpStatusCode"`
		} `json:"value"`
	}
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		for _, item := range parsed.Value {
			if !item.Successful {
				return apperrors.NewInternalf("azure acs sms send failed: %s", item.ErrorMessage)
			}
		}
	}
	return nil
}

// Ensure AzureACSsmsSender implements SmsSender.
var _ SmsSender = (*AzureACSsmsSender)(nil)

// InfobipSmsSender sends SMS via Infobip.
type InfobipSmsSender struct {
	BaseURL    string
	APIKey     string
	SenderName string
	client     *http.Client
}

// NewInfobipSmsSender creates a new Infobip SMS sender.
