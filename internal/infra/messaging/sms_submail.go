package messaging

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func NewSubmailSmsSender(appID, appKey, signName, projectID string) *SubmailSmsSender {
	return &SubmailSmsSender{
		AppID:     appID,
		AppKey:    appKey,
		SignName:  signName,
		ProjectID: projectID,
		client:    ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send sends an SMS via SUBMAIL SMS/XSend API (normal auth mode).
// https://www.mysubmail.com/documents/OOVyh
func (s *SubmailSmsSender) Send(ctx context.Context, phone, code string) error {
	apiURL := "https://api-v4.mysubmail.com/sms/xsend.json"

	vars, err := jsonMarshal(map[string]string{fieldCode: code})
	if err != nil {
		return apperrors.WrapInternalf(err, "submail: marshal vars")
	}

	// Normal mode: signature is the app key itself; sign_type defaults to normal.
	payload := map[string]any{
		"appid":     s.AppID,
		"to":        phone,
		"project":   s.ProjectID,
		"vars":      string(vars),
		"signature": s.AppKey,
	}

	jsonData, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "submail: marshal")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, jsonReader(jsonData))
	if err != nil {
		return apperrors.WrapInternalf(err, "submail: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "submail: request")
	}
	defer resp.Body.Close()

	var result struct {
		Status  string `json:"status"`
		SendID  string `json:"send_id"`
		Code    string `json:"code"`
		Message string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return apperrors.WrapInternalf(err, "submail: decode response")
	}

	// SUBMAIL returns HTTP 200 with {"status":"error","code":...,"msg":...} on failure.
	if result.Status != "success" {
		return apperrors.NewInternalf("submail: API error %s: %s", result.Code, result.Message)
	}
	return nil
}

// Ensure SubmailSmsSender implements SmsSender.
var _ SmsSender = (*SubmailSmsSender)(nil)

// MailtrapSmsSender sends SMS via Mailtrap.
type MailtrapSmsSender struct {
	APIKey     string
	FromNumber string
	client     *http.Client
}

// NewMailtrapSmsSender creates a new Mailtrap SMS sender.
