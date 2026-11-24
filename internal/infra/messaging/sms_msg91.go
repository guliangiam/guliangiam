package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Msg91Sender sends SMS through the Msg91 flow API.
type Msg91Sender struct {
	AuthKey    string
	SenderID   string
	TemplateID string
	APIURL     string
	client     *http.Client
}

// NewMsg91Sender creates a Msg91 sender.
func NewMsg91Sender(authKey, senderID, templateID string) *Msg91Sender {
	return &Msg91Sender{
		AuthKey:    authKey,
		SenderID:   senderID,
		TemplateID: templateID,
		APIURL:     "https://control.msg91.com/api/v5/flow/",
		client:     ssrf.SafeHTTPClient(10 * time.Second),
	}
}

func (s *Msg91Sender) Send(ctx context.Context, phone, code string) error {
	if s.AuthKey == "" || s.TemplateID == "" {
		return apperrors.NewInvalidArgument("msg91 authKey and templateId are required")
	}

	mobile := strings.TrimPrefix(strings.TrimSpace(phone), "+")
	payload := map[string]any{
		"template_id": s.TemplateID,
		"sender":      s.SenderID,
		"short_url":   "0",
		"mobiles":     mobile,
		fieldCode:     code,
	}

	jsonData, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "msg91: marshal")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.APIURL, jsonReader(jsonData))
	if err != nil {
		return apperrors.WrapInternalf(err, "msg91: create request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("authkey", s.AuthKey)

	response, err := s.client.Do(request)
	if err != nil {
		return apperrors.WrapInternalf(err, "msg91: request")
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		return apperrors.NewInternalf("msg91 send failed: HTTP %d", response.StatusCode)
	}
	return nil
}

var _ SmsSender = (*Msg91Sender)(nil)

// TwilioVerifySender delegates code generation and checking to Twilio Verify.
// Send starts a verification (the locally generated code is unused) and
// CheckCode asks Twilio whether the submitted code is approved.
type TwilioVerifySender struct {
	AccountSID string
	AuthToken  string
	ServiceSID string
	APIURL     string
	client     *http.Client
}

// NewTwilioVerifySender creates a Twilio Verify sender.
func NewTwilioVerifySender(accountSID, authToken, serviceSID string) *TwilioVerifySender {
	return &TwilioVerifySender{
		AccountSID: accountSID,
		AuthToken:  authToken,
		ServiceSID: serviceSID,
		APIURL:     "https://verify.twilio.com/v2/Services/",
		client:     ssrf.SafeHTTPClient(10 * time.Second),
	}
}

func (s *TwilioVerifySender) Send(ctx context.Context, phone, _ string) error {
	if s.AccountSID == "" || s.AuthToken == "" || s.ServiceSID == "" {
		return apperrors.NewInvalidArgument("twilio verify accountSid, authToken and serviceSid are required")
	}

	form := url.Values{}
	form.Set("To", phone)
	form.Set("Channel", "sms")

	statusCode, message, err := s.post(ctx, "Verifications", form)
	if err != nil {
		return err
	}
	if statusCode >= 300 {
		return twilioVerifyError(statusCode, message)
	}
	return nil
}

// CheckCode asks Twilio Verify whether the submitted code is approved.
func (s *TwilioVerifySender) CheckCode(ctx context.Context, phone, code string) (bool, error) {
	form := url.Values{}
	form.Set("To", phone)
	form.Set("Code", code)

	statusCode, message, err := s.post(ctx, "VerificationCheck", form)
	if err != nil {
		return false, err
	}
	if statusCode == http.StatusNotFound {
		return false, nil
	}
	if statusCode >= 300 {
		return false, twilioVerifyError(statusCode, message)
	}
	// The endpoint returns {"status":"approved"} once the code is accepted.
	// A pending status is not an approval, so anything else counts as a wrong
	// code rather than an error.
	return message == "approved", nil
}

func (s *TwilioVerifySender) post(ctx context.Context, path string, form url.Values) (int, string, error) {
	endpoint := s.APIURL + s.ServiceSID + "/" + path
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, "", apperrors.WrapInternalf(err, "twilio verify: create request")
	}
	request.SetBasicAuth(s.AccountSID, s.AuthToken)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := s.client.Do(request)
	if err != nil {
		return 0, "", apperrors.WrapInternalf(err, "twilio verify: request")
	}
	defer response.Body.Close()

	var payload struct {
		Status  string `json:"status"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(response.Body).Decode(&payload)

	message := payload.Status
	if payload.Message != "" {
		message = fmt.Sprintf("%d: %s", payload.Code, payload.Message)
	}
	return response.StatusCode, message, nil
}

func twilioVerifyError(statusCode int, message string) error {
	if message == "" {
		return apperrors.NewInternalf("twilio verify failed: HTTP %d", statusCode)
	}
	return apperrors.NewInternalf("twilio verify failed: %s", message)
}

var _ SmsSender = (*TwilioVerifySender)(nil)
