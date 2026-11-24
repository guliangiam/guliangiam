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

// OpsGenieNotifier creates alerts through the OpsGenie Alert API.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：
// POST https://api.opsgenie.com/v2/alerts（EU 集群为 api.eu.opsgenie.com），
// 鉴权头 Authorization: GenieKey {ApiKey}；message 必填且最长 130 字符，
// 正文放 description；受理成功返回 202：
// https://docs.opsgenie.com/docs/alert-api
const (
	// OpsGenieAlertsURL is the default (US) cluster create-alert endpoint.
	OpsGenieAlertsURL = "https://api.opsgenie.com/v2/alerts"
	// OpsGenieEUAlertsURL is the EU cluster create-alert endpoint.
	OpsGenieEUAlertsURL = "https://api.eu.opsgenie.com/v2/alerts"
)

type OpsGenieNotifier struct {
	APIKey     string
	APIURL     string
	HTTPClient *http.Client
}

// NewOpsGenieNotifier creates a notifier for the default cluster.
func NewOpsGenieNotifier(apiKey string) *OpsGenieNotifier {
	return NewOpsGenieNotifierWithConfig(apiKey, "")
}

// NewOpsGenieNotifierWithConfig creates a notifier; an empty apiURL falls
// back to the default (US) cluster endpoint.
func NewOpsGenieNotifierWithConfig(apiKey, apiURL string) *OpsGenieNotifier {
	if apiURL == "" {
		apiURL = OpsGenieAlertsURL
	}
	return &OpsGenieNotifier{
		APIKey:     apiKey,
		APIURL:     apiURL,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

func (n *OpsGenieNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.APIKey == "" {
		return apperrors.NewInvalidArgument("opsgenie notification: api key is required")
	}

	message := notif.Subject
	if message == "" {
		message = notif.Content
	}
	if message == "" {
		message = "GuliangIAM notification"
	}
	// The API rejects messages longer than 130 characters.
	message = firstRunes(message, 130)

	payload := map[string]string{
		"message":     message,
		"description": notif.Content,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "opsgenie notification: marshal alert")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.APIURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "opsgenie notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "GenieKey "+n.APIKey)

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "opsgenie notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("opsgenie notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

var _ Provider = (*OpsGenieNotifier)(nil)
