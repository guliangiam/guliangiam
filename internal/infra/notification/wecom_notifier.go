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

// WeComNotifier sends notifications via WeCom/Enterprise WeChat robot webhook.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：群机器人
// webhook 以 key 参数鉴权、无加签（加签属自建应用服务端 API，不适用于本端点），
// POST JSON msgtype=text：https://developer.work.weixin.qq.com/document/path/91770
type WeComNotifier struct {
	HTTPClient *http.Client
}

// NewWeComNotifier creates a new WeCom notifier.
func NewWeComNotifier() *WeComNotifier {
	return &WeComNotifier{
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// weComPayload represents the JSON payload sent to WeCom webhook.
type weComPayload struct {
	MsgType string    `json:"msgtype"`
	Text    weComText `json:"text"`
}

// weComText represents the text content in a WeCom message.
type weComText struct {
	Content string `json:"content"`
}

func (n *WeComNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("wecom notification: webhook URL is required")
	}

	payload := weComPayload{
		MsgType: messageTypeText,
		Text: weComText{
			Content: notif.Content,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "wecom notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notif.Recipient, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "wecom notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "wecom notification: send request")
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return apperrors.NewInternalf("wecom notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	// WeCom group robot answers HTTP 200 even for rejected messages; the body
	// errcode is the authoritative result per the webhook/send spec.
	return checkProviderCodeBody("wecom", string(respBody))
}
