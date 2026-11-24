package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// PushoverNotifier sends notifications via Pushover API.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：
// POST https://api.pushover.net/1/messages.json，接受 JSON 体（token/user/message/title）：
// https://pushover.net/api
type PushoverNotifier struct {
	AppToken   string
	HTTPClient *http.Client
}

// NewPushoverNotifier creates a new Pushover notifier.
func NewPushoverNotifier(appToken string) *PushoverNotifier {
	return &PushoverNotifier{
		AppToken:   appToken,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// pushoverPayload represents the JSON payload sent to Pushover API.
type pushoverPayload struct {
	Token   string `json:"token"`
	User    string `json:"user"`
	Message string `json:"message"`
	Title   string `json:"title,omitempty"`
}

func (n *PushoverNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.AppToken == "" {
		return apperrors.NewInvalidArgument("pushover notification: app token is required")
	}
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("pushover notification: user key is required")
	}

	payload := pushoverPayload{
		Token:   n.AppToken,
		User:    notif.Recipient,
		Message: notif.Content,
		Title:   notif.Subject,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "pushover notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.pushover.net/1/messages.json", bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "pushover notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "pushover notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("pushover notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
