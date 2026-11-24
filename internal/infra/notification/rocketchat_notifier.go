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

// RocketChatNotifier sends notifications via Rocket.Chat webhook.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：Incoming Webhook
// 默认接受 JSON {"text"}：https://docs.rocket.chat/guides/administrator-guides/integrations
type RocketChatNotifier struct {
	HTTPClient *http.Client
}

// NewRocketChatNotifier creates a new Rocket.Chat notifier.
func NewRocketChatNotifier() *RocketChatNotifier {
	return &RocketChatNotifier{
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// rocketChatPayload represents the JSON payload sent to Rocket.Chat webhook.
type rocketChatPayload struct {
	Text string `json:"text"`
}

func (n *RocketChatNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("rocketchat notification: webhook URL is required")
	}

	payload := rocketChatPayload{
		Text: notif.Content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "rocketchat notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notif.Recipient, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "rocketchat notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "rocketchat notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("rocketchat notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
