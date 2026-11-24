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

// ViberNotifier sends notifications via Viber Bot API.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：
// https://chatapi.viber.com/pa/send_message + X-Viber-Auth-Token 头，
// sender.type=text：https://developers.viber.com/docs/api/rest-bot-api/#send-message
// Requires a Viber Bot account with auth token configured via VIBER_AUTH_TOKEN env var.
type ViberNotifier struct {
	AuthToken  string
	Name       string
	HTTPClient *http.Client
}

// NewViberNotifier creates a new Viber notifier without credentials.
func NewViberNotifier() *ViberNotifier {
	return &ViberNotifier{HTTPClient: ssrf.DefaultHTTPClient}
}

// NewViberNotifierWithConfig creates a new Viber notifier with Bot API credentials.
func NewViberNotifierWithConfig(authToken, name string) *ViberNotifier {
	return &ViberNotifier{
		AuthToken:  authToken,
		Name:       name,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// viberSendPayload represents the JSON payload sent to Viber Bot API send_message endpoint.
type viberSendPayload struct {
	Receiver      string       `json:"receiver"`
	MinAPIVersion int          `json:"min_api_version"`
	Sender        viberSender  `json:"sender"`
	Message       viberMessage `json:"message"`
}

type viberSender struct {
	Name string `json:"name"`
}

type viberMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (n *ViberNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.AuthToken == "" {
		return apperrors.NewUnavailable("viber notification: bot auth token is not configured")
	}
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("viber notification: receiver ID is required")
	}

	apiURL := "https://chatapi.viber.com/pa/send_message"

	name := n.Name
	if name == "" {
		name = "GuliangIAM"
	}

	payload := viberSendPayload{
		Receiver:      notif.Recipient,
		MinAPIVersion: 7,
		Sender:        viberSender{Name: name},
		Message: viberMessage{
			Type: messageTypeText,
			Text: notif.Content,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "viber notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "viber notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Viber-Auth-Token", n.AuthToken)

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "viber notification: send request")
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return apperrors.NewInternalf("viber notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	// Viber Bot API answers HTTP 200 with a body status code; status 0 is the
	// only success signal per the send_message spec.
	return checkProviderCodeBody("viber", string(respBody))
}
