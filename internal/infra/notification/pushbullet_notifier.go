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

// PushbulletNotifier sends notifications via Pushbullet API.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：
// POST https://api.pushbullet.com/v2/pushes，Access-Token 头为官方文档允许的
// 简单鉴权方式之一：https://docs.pushbullet.com/#create-push
type PushbulletNotifier struct {
	AccessToken string
	HTTPClient  *http.Client
}

// NewPushbulletNotifier creates a new Pushbullet notifier.
func NewPushbulletNotifier(accessToken string) *PushbulletNotifier {
	return &PushbulletNotifier{
		AccessToken: accessToken,
		HTTPClient:  ssrf.DefaultHTTPClient,
	}
}

// pushbulletPayload represents the JSON payload sent to Pushbullet API.
type pushbulletPayload struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Email string `json:"email,omitempty"`
}

func (n *PushbulletNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.AccessToken == "" {
		return apperrors.NewInvalidArgument("pushbullet notification: access token is required")
	}

	payload := pushbulletPayload{
		Type:  "note",
		Title: notif.Subject,
		Body:  notif.Content,
		Email: notif.Recipient,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "pushbullet notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.pushbullet.com/v2/pushes", bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "pushbullet notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Access-Token", n.AccessToken)

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "pushbullet notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("pushbullet notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
