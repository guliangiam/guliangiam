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

// SlackNotifier sends notifications via Slack webhook.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：Incoming Webhook
// 接受 JSON {"text"}：https://api.slack.com/messaging/webhooks
type SlackNotifier struct {
	HTTPClient *http.Client
}

// NewSlackNotifier creates a new Slack notifier.
func NewSlackNotifier() *SlackNotifier {
	return &SlackNotifier{
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// slackPayload represents the JSON payload sent to Slack webhook.
type slackPayload struct {
	Text string `json:"text"`
}

func (n *SlackNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("slack notification: webhook URL is required")
	}

	payload := slackPayload{
		Text: notif.Content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "slack notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notif.Recipient, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "slack notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "slack notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("slack notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
