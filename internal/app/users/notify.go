package users

import (
	"context"
	"log/slog"
	"strings"
)

// NotificationCommand is the transport-agnostic notification request users
// use cases emit for account-security events. app/users stays independent of
// app/usernotifications; the composition root adapts the port onto the personal
// notification center.
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

// Notifier is the optional outbound port for account-security notifications.
// A nil notifier disables fan-out without affecting the committed write.
type Notifier interface {
	Notify(ctx context.Context, command NotificationCommand) error
}

// notifyAdminPasswordReset informs the target user that an administrator reset
// their password. It is best-effort: the reset (and its audit record) is
// already committed, so a notification failure is logged rather than surfaced
// as a failed reset.
func (s *Service) notifyAdminPasswordReset(ctx context.Context, organizationID, userID, username string) {
	if s.Notifier == nil {
		return
	}
	command := adminPasswordResetNotification(organizationID, userID, username)
	if err := s.Notifier.Notify(ctx, command); err != nil {
		slog.WarnContext(ctx, "administrator password reset notification failed",
			"organization_id", organizationID,
			"user_id", userID,
			"error", err)
	}
}

func adminPasswordResetNotification(organizationID, userID, username string) NotificationCommand {
	name := strings.TrimSpace(username)
	if name == "" {
		name = "当前账号"
	} else {
		name = "账号 " + name
	}
	return NotificationCommand{
		OrganizationID: organizationID,
		UserID:         userID,
		Category:       "security",
		EventType:      "user.password.reset",
		Title:          "密码已重置",
		Content:        "你的" + name + "密码已被管理员重置，请在下次登录时按提示设置新密码。如非本人操作，请立即联系管理员。",
		Link:           "/account",
		SourceType:     "user",
		SourceID:       userID,
	}
}
