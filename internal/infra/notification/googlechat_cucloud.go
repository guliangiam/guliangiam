package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// GoogleChatNotifier sends notifications to Google Chat (Hangouts) via webhook.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：Chat 入站
// webhook 的 legacy 纯文本形态 {"text": "..."} 仍受支持（富文本用 cardsV2，
// 非必需）：https://developers.google.com/workspace/chat/authenticate-authorize-chat-webhook
type GoogleChatNotifier struct {
	webhookURL string
	HTTPClient *http.Client
}

// NewGoogleChatNotifier creates a new Google Chat notifier.
func NewGoogleChatNotifier(webhookURL string) *GoogleChatNotifier {
	return &GoogleChatNotifier{
		webhookURL: webhookURL,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

func (n *GoogleChatNotifier) Send(ctx context.Context, notification *Notification) error {
	message := map[string]interface{}{
		messageTypeText: formatGoogleChatMessage(notification),
	}
	body, err := json.Marshal(message)
	if err != nil {
		return apperrors.WrapInternalf(err, "marshal google chat message")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "create google chat request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "send google chat notification")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return apperrors.NewInternalf("google chat notification failed (status %d)", resp.StatusCode)
	}

	return nil
}

func formatGoogleChatMessage(n *Notification) string {
	if n.Subject != "" {
		return fmt.Sprintf("*%s*\n%s", n.Subject, n.Content)
	}
	return n.Content
}

// CUCloudNotifier sends notifications via CUCloud (天翼云) messaging service.
//
// Calibration (OPEN-API-75 P6, 2026-11-14)：天翼云 oos-api 通知接口无公开规范，
// 端点与 X-API-Key 头承袭现状、未验证（真实凭据联调属 L4 边界）。
type CUCloudNotifier struct {
	appID  string
	apiKey string
}

// NewCUCloudNotifier creates a new CUCloud notifier.
func NewCUCloudNotifier(appID, apiKey string) *CUCloudNotifier {
	return &CUCloudNotifier{
		appID:  appID,
		apiKey: apiKey,
	}
}

func (n *CUCloudNotifier) Send(ctx context.Context, notification *Notification) error {
	// CUCloud notification API (similar to Alibaba Cloud SMS pattern)
	payload := map[string]interface{}{
		"app_id":  n.appID,
		"to":      notification.Recipient,
		"subject": notification.Subject,
		"content": notification.Content,
		"type":    notification.Type,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "marshal cucloud message")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oos-api.ctyunapi.cn/api/v1/notify", bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "create cucloud request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", n.apiKey)

	resp, err := ssrf.DefaultHTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "send cucloud notification")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return apperrors.NewInternalf("cucloud notification failed (status %d)", resp.StatusCode)
	}

	return nil
}

// Ensure implementations satisfy the Provider interface
var _ Provider = (*GoogleChatNotifier)(nil)
var _ Provider = (*CUCloudNotifier)(nil)
