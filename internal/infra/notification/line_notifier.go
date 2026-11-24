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

// LineNotifier sends notifications via the LINE Messaging API push endpoint.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：使用
// Messaging API POST https://api.line.me/v2/bot/message/push（channel access
// token Bearer 鉴权）；LINE Notify 服务已停止，不作为实现路径：
// https://developers.line.biz/en/reference/messaging-api/#send-push-message
// The legacy LINE Notify service (notify-api.line.me) was discontinued on
// 2025-03-31; the official successor is POST /v2/bot/message/push, where the
// recipient is the target user's LINE userId.
type LineNotifier struct {
	AccessToken string
	HTTPClient  *http.Client
}

// NewLineNotifier creates a new LINE Messaging API notifier.
func NewLineNotifier(accessToken string) *LineNotifier {
	return &LineNotifier{
		AccessToken: accessToken,
		HTTPClient:  ssrf.DefaultHTTPClient,
	}
}

// linePayload represents the JSON payload sent to the Messaging API push endpoint.
type linePayload struct {
	To       string        `json:"to"`
	Messages []lineMessage `json:"messages"`
}

type lineMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (n *LineNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.AccessToken == "" {
		return apperrors.NewInvalidArgument("line notification: channel access token is required")
	}
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("line notification: recipient user ID is required")
	}

	payload := linePayload{
		To: notif.Recipient,
		Messages: []lineMessage{
			{Type: messageTypeText, Text: notif.Content},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "line notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.line.me/v2/bot/message/push", bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "line notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.AccessToken)

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "line notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("line notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
