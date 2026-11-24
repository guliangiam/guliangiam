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

// PagerDutyNotifier triggers incidents through the PagerDuty Events API v2.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：
// POST https://events.pagerduty.com/v2/enqueue，体为
// {routing_key, event_action:"trigger", payload:{summary, source,
// severity∈info|warning|error|critical, custom_details}}；受理成功返回
// HTTP 202：
// https://developer.pagerduty.com/docs/ZG9jOjExMDI5NTgx-send-an-alert-event
type PagerDutyNotifier struct {
	RoutingKey string
	Severity   string
	Source     string
	HTTPClient *http.Client
}

const pagerDutyEnqueueURL = "https://events.pagerduty.com/v2/enqueue"

// pagerDutySeverities is the closed severity set of Events API v2 payloads.
var pagerDutySeverities = map[string]struct{}{
	"info":     {},
	"warning":  {},
	"error":    {},
	"critical": {},
}

// NewPagerDutyNotifier creates a notifier with the default severity/source.
func NewPagerDutyNotifier(routingKey string) *PagerDutyNotifier {
	return NewPagerDutyNotifierWithConfig(routingKey, "", "")
}

// NewPagerDutyNotifierWithConfig creates a notifier, falling back to
// "error"/"GuliangIAM" for an unknown severity or an empty source.
func NewPagerDutyNotifierWithConfig(routingKey, severity, source string) *PagerDutyNotifier {
	if _, ok := pagerDutySeverities[severity]; !ok {
		severity = "error"
	}
	if source == "" {
		source = "GuliangIAM"
	}
	return &PagerDutyNotifier{
		RoutingKey: routingKey,
		Severity:   severity,
		Source:     source,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// pagerDutyEvent mirrors the Events API v2 trigger envelope.
type pagerDutyEvent struct {
	RoutingKey  string `json:"routing_key"`
	EventAction string `json:"event_action"`
	Payload     struct {
		Summary       string            `json:"summary"`
		Source        string            `json:"source"`
		Severity      string            `json:"severity"`
		CustomDetails map[string]string `json:"custom_details,omitempty"`
	} `json:"payload"`
}

func (n *PagerDutyNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.RoutingKey == "" {
		return apperrors.NewInvalidArgument("pagerduty notification: routing key is required")
	}

	summary := notif.Subject
	if summary == "" {
		summary = notif.Content
	}
	if summary == "" {
		summary = "GuliangIAM notification"
	}

	event := pagerDutyEvent{
		RoutingKey:  n.RoutingKey,
		EventAction: "trigger",
	}
	event.Payload.Summary = summary
	event.Payload.Source = n.Source
	event.Payload.Severity = n.Severity
	if notif.Content != "" && notif.Content != summary {
		event.Payload.CustomDetails = map[string]string{"content": notif.Content}
	}

	body, err := json.Marshal(event)
	if err != nil {
		return apperrors.WrapInternalf(err, "pagerduty notification: marshal event")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pagerDutyEnqueueURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "pagerduty notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "pagerduty notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("pagerduty notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

var _ Provider = (*PagerDutyNotifier)(nil)
