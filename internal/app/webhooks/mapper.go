package webhooks

import (
	"encoding/json"

	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
)

func toListItem(w *domain.Webhook) WebhookListItem {
	return WebhookListItem{ID: w.ID, OrganizationID: w.OrganizationID, Name: w.Name, DisplayName: w.DisplayName,
		URL: w.URL, Method: w.Method, IsEnabled: w.IsEnabled,
		Organization: w.Organization, ContentType: w.ContentType, CertID: w.CertID,
		IsError: w.IsError, SingleOrgOnly: w.SingleOrgOnly,
		CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt}
}
func toDetail(w *domain.Webhook) WebhookDetail {
	return WebhookDetail{WebhookListItem: toListItem(w), Events: append([]string(nil), w.Events...),
		Headers:    w.Headers,
		RetryCount: w.RetryCount, TimeoutMs: w.TimeoutMs,
		CustomBody: w.CustomBody, CustomHeaders: w.CustomHeaders, ErrorText: w.ErrorText,
		TokenFields: append([]string(nil), w.TokenFields...), ObjectFields: append([]string(nil), w.ObjectFields...),
		IsUserExtended: w.IsUserExtended, MaxRetries: w.MaxRetries, RetryInterval: w.RetryInterval,
		UseExponentialBackoff: w.UseExponentialBackoff,
		SecretSet:             w.Secret != "", SecretMasked: maskSecret(w.Secret),
		Revision: w.Revision}
}

// maskSecret returns first 2 chars + *** + last 2 chars, or empty string.
// Short secrets (< 6 chars) return fully masked "******". Mirrors the
// radiusconfigs/certs read-projection convention.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) < 6 {
		return "******"
	}
	return s[:2] + "***" + s[len(s)-2:]
}

func toEventListItem(e *eventdomain.WebhookEvent) WebhookEventListItem {
	return WebhookEventListItem{
		ID: e.ID, WebhookID: e.WebhookID, EventType: e.EventType,
		Status: e.Status, RetryCount: e.RetryCount, ResponseCode: e.ResponseCode,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func toEventDetail(e *eventdomain.WebhookEvent) WebhookEventDetail {
	var payload json.RawMessage
	if e.Payload != nil {
		payload = make(json.RawMessage, len(e.Payload))
		copy(payload, e.Payload)
	}
	return WebhookEventDetail{
		WebhookEventListItem: toEventListItem(e),
		Payload:              payload,
		ResponseBody:         e.ResponseBody,
	}
}
