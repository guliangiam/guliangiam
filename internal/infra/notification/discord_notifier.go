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

// DiscordNotifier sends notifications via Discord webhook.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：Webhook 接受
// JSON {"content"}，成功返回 204/200（?wait=true 时 200）：
// https://discord.com/developers/docs/resources/webhook#execute-webhook
type DiscordNotifier struct {
	HTTPClient *http.Client
}

// NewDiscordNotifier creates a new Discord notifier.
func NewDiscordNotifier() *DiscordNotifier {
	return &DiscordNotifier{
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// discordPayload represents the JSON payload sent to Discord webhook.
type discordPayload struct {
	Content string `json:"content"`
}

func (n *DiscordNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("discord notification: webhook URL is required")
	}

	payload := discordPayload{
		Content: notif.Content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "discord notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notif.Recipient, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "discord notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "discord notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("discord notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
