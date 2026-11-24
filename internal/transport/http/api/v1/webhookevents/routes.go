package webhookevents

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/webhook-events",
			OperationID: "ListWebhookEvents",
			Summary:     "List webhook delivery events across all webhooks (admin audit)",
			Permission:  "webhookevents:read",
			Surface:     route.SurfaceManagement,
		},
	}
}
