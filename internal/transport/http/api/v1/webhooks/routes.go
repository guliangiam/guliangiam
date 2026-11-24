package webhooks

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the Casbin permission definitions for webhook endpoints.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/webhooks",
			OperationID: "ListWebhooks",
			Summary:     "List webhooks",
			Permission:  "webhooks:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/webhooks",
			OperationID: "CreateWebhook",
			Summary:     "Create a webhook",
			Permission:  "webhooks:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/webhooks/{id}",
			OperationID: "GetWebhook",
			Summary:     "Get a webhook by ID",
			Permission:  "webhooks:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/webhooks/{id}",
			OperationID: "PatchWebhook",
			Summary:     "Patch a webhook",
			Permission:  "webhooks:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/webhooks/{id}",
			OperationID: "DeleteWebhook",
			Summary:     "Delete a webhook",
			Permission:  "webhooks:admin",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/webhooks/{id}/events",
			OperationID: "ListWebhookEventsForWebhook",
			Summary:     "List events for a webhook",
			Permission:  "webhooks:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/webhook-events/{eventId}",
			OperationID: "GetWebhookEvent",
			Summary:     "Get a webhook event by ID",
			Permission:  "webhooks:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/webhook-events/{eventId}/replay",
			OperationID: "ReplayWebhookEvent",
			Summary:     "Replay a webhook event",
			Permission:  "webhooks:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/webhook-events/{eventId}",
			OperationID: "DeleteWebhookEvent",
			Summary:     "Delete a webhook event",
			Permission:  "webhooks:admin",
			Surface:     route.SurfaceManagement,
		},
	}
}
