package auth

import (
	"context"
	"log/slog"
)

// NotificationCommand is the minimal notification request auth flows emit for
// security alerts (password reset, new-device sign-in). The composition root
// maps it onto the personal notification center.
type NotificationCommand struct {
	OrganizationID string
	UserID         string
	Category       string
	EventType      string
	Title          string
	Content        string
	Link           string
	SourceType     string
	SourceID       string
}

// NotificationPublisher is the optional port to the personal inbox. nil keeps
// auth flows silent (tests, dev).
type NotificationPublisher interface {
	Publish(ctx context.Context, command NotificationCommand) error
}

// notifyUser is best-effort: a notification failure never fails an auth flow.
func (s *Service) notifyUser(ctx context.Context, command NotificationCommand) {
	if s.Notifications == nil || command.UserID == "" || command.OrganizationID == "" {
		return
	}
	if err := s.Notifications.Publish(ctx, command); err != nil {
		slog.WarnContext(ctx, "auth notification failed",
			"organization_id", command.OrganizationID,
			"user_id", command.UserID,
			"event_type", command.EventType,
			"error", err)
	}
}
