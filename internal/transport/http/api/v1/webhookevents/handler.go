package webhookevents

import (
	"context"

	appwh "github.com/guliangiam/guliangiam/internal/app/webhookevents"
)

// Handler exposes the webhook_event app service to HTTP transport.
type Handler struct {
	Service appwh.Service
}

// ─── List ─────────────────────────────────────────────────────────────────

type ListQuery struct {
	WebhookID      string
	EventType      string
	Status         *string
	OrganizationID string
	Page           int
	PageSize       int
}

func (h Handler) List(ctx context.Context, q ListQuery) ([]appwh.WebhookEventDTO, int64, error) {
	return h.Service.List(ctx, appwh.ListQuery{
		WebhookID:      q.WebhookID,
		EventType:      q.EventType,
		Status:         q.Status,
		OrganizationID: q.OrganizationID,
		Page:           q.Page,
		PageSize:       q.PageSize,
	})
}
