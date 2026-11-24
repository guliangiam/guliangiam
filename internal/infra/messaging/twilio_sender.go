package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TwilioSmsSender sends SMS via Twilio.
type TwilioSmsSender struct {
	AccountSID string
	AuthToken  string
	FromNumber string
	client     *http.Client
}

// NewTwilioSmsSender creates a new Twilio SMS sender.
func NewTwilioSmsSender(accountSID, authToken, fromNumber string) *TwilioSmsSender {
	return &TwilioSmsSender{AccountSID: accountSID, AuthToken: authToken, FromNumber: fromNumber, client: ssrf.DefaultHTTPClient}
}

func (s *TwilioSmsSender) Send(ctx context.Context, phone, code string) error {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.AccountSID)

	data := url.Values{}
	data.Set("To", phone)
	data.Set("From", s.FromNumber)
	data.Set("Body", fmt.Sprintf("Your verification code is: %s", code))

	// Twilio REST requires POST parameters in an application/x-www-form-urlencoded body.
	// https://www.twilio.com/docs/messaging/api/message-resource
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.AccountSID, s.AuthToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "twilio send sms")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apperrors.WrapInternalf(err, "twilio read response")
	}

	if resp.StatusCode >= 300 {
		var apiErr struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		}
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Message != "" {
			return apperrors.NewInternalf("twilio send sms failed: %s (code %d)", apiErr.Message, apiErr.Code)
		}
		return apperrors.NewInternalf("twilio send sms failed: status %d", resp.StatusCode)
	}
	return nil
}
