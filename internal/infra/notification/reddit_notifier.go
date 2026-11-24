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

// RedditNotifier sends notifications via Reddit Private Messages API.
// Requires OAuth2 credentials configured via REDDIT_ACCESS_TOKEN env var.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：
// POST https://oauth.reddit.com/api/compose（OAuth2 Bearer + privatemessages scope，
// User-Agent 为 Reddit API 强制要求）：https://www.reddit.com/dev/api/#POST_api_compose
type RedditNotifier struct {
	AccessToken  string
	ClientID     string
	ClientSecret string
	Username     string
	HTTPClient   *http.Client
}

// NewRedditNotifier creates a new Reddit notifier without credentials.
func NewRedditNotifier() *RedditNotifier {
	return &RedditNotifier{HTTPClient: ssrf.DefaultHTTPClient}
}

// NewRedditNotifierWithConfig creates a new Reddit notifier with OAuth2 credentials.
func NewRedditNotifierWithConfig(accessToken, clientID, clientSecret, username string) *RedditNotifier {
	return &RedditNotifier{
		AccessToken:  accessToken,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Username:     username,
		HTTPClient:   ssrf.DefaultHTTPClient,
	}
}

// redditComposePayload represents the form data for Reddit compose message API.
type redditComposePayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

func (n *RedditNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.AccessToken == "" {
		return apperrors.NewUnavailable("reddit notification: access token is not configured")
	}
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("reddit notification: recipient username is required")
	}

	apiURL := "https://oauth.reddit.com/api/compose"

	payload := redditComposePayload{
		To:      notif.Recipient,
		Subject: notif.Subject,
		Text:    notif.Content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "reddit notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "reddit notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.AccessToken)
	req.Header.Set("User-Agent", "GuliangIAM/1.0 by "+n.Username)

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "reddit notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("reddit notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
