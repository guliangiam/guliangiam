package notification

import (
	"context"
	"encoding/json"
	"strconv"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Notification represents a notification to be sent.
type Notification struct {
	Type           string                 // "email", "sms", "webhook", "notification", etc.
	OrganizationID string                 // target organization for push notifications
	Recipient      string                 // email address, phone number, webhook URL, etc.
	Subject        string                 // notification subject
	Content        string                 // notification body
	Metadata       map[string]interface{} // additional provider-specific metadata
}

// Provider is the interface for notification delivery backends.
type Provider interface {
	// Send sends a notification.
	Send(ctx context.Context, n *Notification) error
}

// MetadataString returns a metadata value as a non-empty string, tolerating
// json.Number/float64 values that come back through the provider config merge.
func (n *Notification) MetadataString(key string) string {
	if n == nil || n.Metadata == nil {
		return ""
	}
	value, ok := n.Metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

// MetadataTemplateData flattens metadata["template_data"] into string values for
// providers whose payload keys are dictated by an external template.
func (n *Notification) MetadataTemplateData() map[string]string {
	if n == nil || n.Metadata == nil {
		return nil
	}
	raw, ok := n.Metadata["template_data"]
	if !ok {
		return nil
	}
	result := map[string]string{}
	switch typed := raw.(type) {
	case map[string]string:
		for key, value := range typed {
			result[key] = value
		}
	case map[string]any:
		for key, value := range typed {
			switch text := value.(type) {
			case string:
				result[key] = text
			case json.Number:
				result[key] = text.String()
			case float64:
				result[key] = strconv.FormatFloat(text, 'f', -1, 64)
			}
		}
	}
	return result
}

// providerCodeResponse captures the in-body result codes used by DingTalk
// (errcode), Lark (code), WeCom (errcode) and Viber (status). These robots
// answer HTTP 200 even on business failures, so the body code is the real
// success signal per each official spec.
type providerCodeResponse struct {
	Errcode       *int   `json:"errcode"`
	Code          *int   `json:"code"`
	Status        *int   `json:"status"`
	Errmsg        string `json:"errmsg"`
	Msg           string `json:"msg"`
	StatusMessage string `json:"status_message"`
}

// checkProviderCodeBody parses a 2xx response body and fails when a provider
// result code field is present and non-zero. Non-JSON or code-free bodies
// (e.g. Slack "ok") are treated as success.
func checkProviderCodeBody(provider, body string) error {
	if body == "" {
		return nil
	}
	var r providerCodeResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		return nil
	}
	for _, pair := range []struct {
		name  string
		value *int
		msg   string
	}{
		{"errcode", r.Errcode, r.Errmsg},
		{"code", r.Code, r.Msg},
		{"status", r.Status, r.StatusMessage},
	} {
		if pair.value != nil && *pair.value != 0 {
			return apperrors.NewInternalf("%s notification: provider rejected request (%s=%d: %s)", provider, pair.name, *pair.value, pair.msg)
		}
	}
	return nil
}
