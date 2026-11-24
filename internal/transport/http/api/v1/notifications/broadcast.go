package notifications

import (
	"context"

	appusernotifications "github.com/guliangiam/guliangiam/internal/app/usernotifications"
)

// BroadcastNotificationRequest sends an administrator message to many users.
// OrganizationID is overwritten by the trusted tenant resolution in chi.go;
// the body value is only honoured for platform administrators.
type BroadcastNotificationRequest struct {
	OrganizationID string   `json:"organizationId"`
	UserIDs        []string `json:"userIds"`
	Category       string   `json:"category"`
	Title          string   `json:"title"`
	Content        string   `json:"content"`
	Link           string   `json:"link"`
	AllowReply     bool     `json:"allowReply"`
}

// Broadcast handles POST /api/v1/notifications/broadcast.
func (h Handler) Broadcast(ctx context.Context, request BroadcastNotificationRequest) (appusernotifications.BroadcastResult, error) {
	return h.Inbox.Broadcast(ctx, appusernotifications.BroadcastCommand{
		OrganizationID: request.OrganizationID,
		UserIDs:        request.UserIDs,
		Category:       request.Category,
		Title:          request.Title,
		Content:        request.Content,
		Link:           request.Link,
		AllowReply:     request.AllowReply,
	})
}
