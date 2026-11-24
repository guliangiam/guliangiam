package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SubmailEmailSender sends transactional email through Submail's mail API.
type SubmailEmailSender struct {
	AppID  string
	AppKey string
	From   string
	APIURL string
	client *http.Client
}

// NewSubmailEmailSender creates a Submail email sender.
func NewSubmailEmailSender(appID, appKey, from string) *SubmailEmailSender {
	return &SubmailEmailSender{
		AppID:  appID,
		AppKey: appKey,
		From:   from,
		APIURL: "https://api-v4.mysubmail.com/mail/send.json",
		client: ssrf.SafeHTTPClient(10 * time.Second),
	}
}

func (s *SubmailEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.AppID == "" || s.AppKey == "" {
		return apperrors.NewInvalidArgument("submail appId and appKey are required")
	}

	form := url.Values{}
	form.Set("appid", s.AppID)
	form.Set("to", to)
	form.Set(fieldSubject, subject)
	form.Set(fieldText, body)
	form.Set("signature", s.AppKey)
	if s.From != "" {
		form.Set(fieldFrom, s.From)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.APIURL, strings.NewReader(form.Encode()))
	if err != nil {
		return apperrors.WrapInternalf(err, "submail email: create request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "submail email: request")
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return apperrors.WrapInternalf(err, "submail email: read response")
	}

	if response.StatusCode >= 300 {
		return apperrors.NewInternalf("submail email failed: HTTP %d", response.StatusCode)
	}
	var result struct {
		Status string `json:"status"`
		Code   any    `json:"code"`
		Msg    string `json:"msg"`
	}
	_ = json.Unmarshal(payload, &result)
	if result.Status != "" && result.Status != "success" {
		return apperrors.NewInternalf("submail email failed: %v %s", result.Code, result.Msg)
	}
	return nil
}

var _ EmailSender = (*SubmailEmailSender)(nil)

// MailtrapEmailSender sends email through Mailtrap's sending API.
type MailtrapEmailSender struct {
	APIToken string
	From     string
	APIURL   string
	client   *http.Client
}

// NewMailtrapEmailSender creates a Mailtrap email sender.
func NewMailtrapEmailSender(apiToken, from string) *MailtrapEmailSender {
	return &MailtrapEmailSender{
		APIToken: apiToken,
		From:     from,
		APIURL:   "https://send.api.mailtrap.io/api/send",
		client:   ssrf.SafeHTTPClient(10 * time.Second),
	}
}

func (s *MailtrapEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.APIToken == "" {
		return apperrors.NewInvalidArgument("mailtrap apiToken is required")
	}
	if s.From == "" {
		return apperrors.NewInvalidArgument("mailtrap from address is required")
	}

	payload := map[string]any{
		fieldFrom:    map[string]string{"email": s.From},
		"to":         []map[string]string{{"email": to}},
		fieldSubject: subject,
		fieldText:    body,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "mailtrap: marshal")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.APIURL, strings.NewReader(string(jsonPayload)))
	if err != nil {
		return apperrors.WrapInternalf(err, "mailtrap: create request")
	}
	request.Header.Set("Api-Token", s.APIToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "mailtrap: request")
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		return apperrors.NewInternalf("mailtrap send failed: HTTP %d", response.StatusCode)
	}
	return nil
}

var _ EmailSender = (*MailtrapEmailSender)(nil)
