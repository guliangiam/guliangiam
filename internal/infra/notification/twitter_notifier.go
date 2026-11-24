package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TwitterNotifier sends notifications via Twitter/X Direct Messages API v2.
// Requires OAuth 2.0 credentials configured via TWITTER_OAUTH2_TOKEN env var.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：X API v2 私信
// 端点 POST /2/dm_conversations/with/{participant_id}/messages（OAuth 2.0 Bearer、
// JSON {"text"}、成功 201）；v1.1 direct_messages/events/new 已退役：
// https://docs.x.com/x-api/direct-messages/introduce-direct-messages
type TwitterNotifier struct {
	OAuth2Token string
	APIKey      string
	APISecret   string
	HTTPClient  *http.Client
}

// NewTwitterNotifier creates a new Twitter/X notifier without credentials.
func NewTwitterNotifier() *TwitterNotifier {
	return &TwitterNotifier{HTTPClient: ssrf.DefaultHTTPClient}
}

// NewTwitterNotifierWithConfig creates a new Twitter/X notifier with OAuth 2.0 credentials.
func NewTwitterNotifierWithConfig(oauth2Token, apiKey, apiSecret string) *TwitterNotifier {
	return &TwitterNotifier{
		OAuth2Token: oauth2Token,
		APIKey:      apiKey,
		APISecret:   apiSecret,
		HTTPClient:  ssrf.DefaultHTTPClient,
	}
}

// twitterDMPayload represents the JSON body of the X API v2
// POST /2/dm_conversations/with/{participant_id}/messages endpoint.
type twitterDMPayload struct {
	Text string `json:"text"`
}

func (n *TwitterNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.OAuth2Token == "" {
		return apperrors.NewUnavailable("twitter notification: OAuth2 token is not configured")
	}
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("twitter notification: recipient user ID is required")
	}

	// The v1.1 direct_messages/events/new endpoint is retired; X API v2 sends
	// DMs through the conversation-with-participant route.
	apiURL := "https://api.twitter.com/2/dm_conversations/with/" + url.PathEscape(notif.Recipient) + "/messages"

	payload := twitterDMPayload{
		Text: notif.Content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "twitter notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "twitter notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.OAuth2Token)

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "twitter notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("twitter notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
