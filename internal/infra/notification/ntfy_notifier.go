package notification

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// NtfyNotifier publishes messages to an ntfy topic (ntfy.sh or self-hosted).
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：发布即
// POST {server}/{topic}，请求体本身就是消息正文；标题走可选 Title 头，私有
// topic 用 Authorization: Bearer {access-token}：https://docs.ntfy.sh/publish/
type NtfyNotifier struct {
	Server      string
	Topic       string
	AccessToken string
	HTTPClient  *http.Client
}

// NewNtfyNotifier creates a new ntfy notifier.
func NewNtfyNotifier(server, topic, accessToken string) *NtfyNotifier {
	return &NtfyNotifier{
		Server:      server,
		Topic:       topic,
		AccessToken: accessToken,
		HTTPClient:  ssrf.DefaultHTTPClient,
	}
}

func (n *NtfyNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.Server == "" {
		return apperrors.NewInvalidArgument("ntfy notification: server URL is required")
	}
	if err := ssrf.ValidateURL(n.Server); err != nil {
		return apperrors.WrapInvalidArgumentf(err, "ntfy notification: server rejected")
	}

	// Providers that store the topic in the recipient column still deliver:
	// the topic falls back to Recipient when the config omits it.
	topic := n.Topic
	if topic == "" {
		topic = notif.Recipient
	}
	if topic == "" {
		return apperrors.NewInvalidArgument("ntfy notification: topic is required")
	}

	endpoint := strings.TrimRight(n.Server, "/") + "/" + topic
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(notif.Content))
	if err != nil {
		return apperrors.WrapInternalf(err, "ntfy notification: create request")
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if notif.Subject != "" {
		req.Header.Set("Title", notif.Subject)
	}
	if n.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.AccessToken)
	}

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "ntfy notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("ntfy notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

var _ Provider = (*NtfyNotifier)(nil)
