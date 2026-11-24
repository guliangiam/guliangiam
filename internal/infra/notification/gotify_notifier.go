package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// GotifyNotifier publishes messages to a self-hosted Gotify server.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：
// POST {base}/message，JSON 体 {message（必填）, title, priority（1-11，默认
// 5）}；应用 token 经 X-Gotify-Key 头鉴权：
// https://gotify.net/api-docs#/message/createMessage
type GotifyNotifier struct {
	BaseURL    string
	Token      string
	Priority   int
	HTTPClient *http.Client
}

// NewGotifyNotifier creates a notifier with the default priority (5).
func NewGotifyNotifier(baseURL, token string) *GotifyNotifier {
	return NewGotifyNotifierWithConfig(baseURL, token, 0)
}

// NewGotifyNotifierWithConfig creates a notifier; a non-positive priority
// falls back to Gotify's default of 5.
func NewGotifyNotifierWithConfig(baseURL, token string, priority int) *GotifyNotifier {
	if priority <= 0 {
		priority = 5
	}
	return &GotifyNotifier{
		BaseURL:    baseURL,
		Token:      token,
		Priority:   priority,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

func (n *GotifyNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.BaseURL == "" {
		return apperrors.NewInvalidArgument("gotify notification: server URL is required")
	}
	if err := ssrf.ValidateURL(n.BaseURL); err != nil {
		return apperrors.WrapInvalidArgumentf(err, "gotify notification: server rejected")
	}

	endpoint := strings.TrimRight(n.BaseURL, "/") + "/message"
	payload := struct {
		Message  string `json:"message"`
		Title    string `json:"title,omitempty"`
		Priority int    `json:"priority"`
	}{
		Message:  notif.Content,
		Title:    notif.Subject,
		Priority: n.Priority,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "gotify notification: marshal message")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "gotify notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	if n.Token != "" {
		req.Header.Set("X-Gotify-Key", n.Token)
	}

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "gotify notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("gotify notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

var _ Provider = (*GotifyNotifier)(nil)
