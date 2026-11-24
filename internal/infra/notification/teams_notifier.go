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

// TeamsNotifier sends notifications via Microsoft Teams webhook.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：payload 为
// Workflows 入站 webhook 的 Adaptive Card 形态（type=message +
// application/vnd.microsoft.card.adaptive），legacy connector MessageCard 已随
// 2025 年 connector webhook 退役被 Workflows 取代：
// https://learn.microsoft.com/en-us/power-platform/power-automate/send-message-adaptive-card-to-chat-using-workflows
type TeamsNotifier struct {
	HTTPClient *http.Client
}

// NewTeamsNotifier creates a new Microsoft Teams notifier.
func NewTeamsNotifier() *TeamsNotifier {
	return &TeamsNotifier{
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// teamsPayload represents the Adaptive Card JSON payload sent to Teams webhook.
type teamsPayload struct {
	Type        string            `json:"type"`
	Attachments []teamsAttachment `json:"attachments"`
}

// teamsAttachment represents an attachment in a Teams message.
type teamsAttachment struct {
	ContentType string    `json:"contentType"`
	ContentURL  string    `json:"contentUrl,omitempty"`
	Content     teamsCard `json:"content"`
}

// teamsCard represents an Adaptive Card.
type teamsCard struct {
	Schema  string      `json:"$schema"`
	Type    string      `json:"type"`
	Version string      `json:"version"`
	Body    []teamsBody `json:"body"`
}

// teamsBody represents a body element in an Adaptive Card.
type teamsBody struct {
	Type   string `json:"type"`
	Size   string `json:"size,omitempty"`
	Text   string `json:"text,omitempty"`
	Weight string `json:"weight,omitempty"`
	Wrap   bool   `json:"wrap"`
}

func (n *TeamsNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("teams notification: webhook URL is required")
	}

	card := teamsPayload{
		Type: "message",
		Attachments: []teamsAttachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				Content: teamsCard{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: "1.4",
					Body: []teamsBody{
						{
							Type:   "TextBlock",
							Text:   notif.Subject,
							Size:   "Medium",
							Weight: "Bolder",
							Wrap:   true,
						},
						{
							Type: "TextBlock",
							Text: notif.Content,
							Wrap: true,
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(card)
	if err != nil {
		return apperrors.WrapInternalf(err, "teams notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notif.Recipient, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "teams notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "teams notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("teams notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
