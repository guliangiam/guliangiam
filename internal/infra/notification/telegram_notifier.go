package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TelegramNotifier sends notifications via Telegram Bot API.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：
// POST https://api.telegram.org/bot{token}/sendMessage（JSON chat_id/text）：
// https://core.telegram.org/bots/api#sendmessage
type TelegramNotifier struct {
	BotToken   string
	HTTPClient *http.Client
}

// NewTelegramNotifier creates a new Telegram notifier.
func NewTelegramNotifier(botToken string) *TelegramNotifier {
	return &TelegramNotifier{
		BotToken:   botToken,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// telegramPayload represents the JSON payload sent to Telegram Bot API.
type telegramPayload struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

func (n *TelegramNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.BotToken == "" {
		return apperrors.NewInvalidArgument("telegram notification: bot token is required")
	}
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("telegram notification: chat ID is required")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.BotToken)

	payload := telegramPayload{
		ChatID: notif.Recipient,
		Text:   notif.Content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "telegram notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "telegram notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "telegram notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("telegram notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
