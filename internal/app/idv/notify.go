package idv

import (
	"context"
	"log/slog"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
)

// NotificationCommand is the transport-agnostic notification request emitted
// when an identity verification reaches a new terminal state. app/idv stays
// independent of app/usernotifications; the composition root adapts the port
// onto the personal notification center.
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

// Notifier is the optional outbound port for verification-result notifications.
// A nil notifier disables fan-out without affecting verification writes.
type Notifier interface {
	Notify(ctx context.Context, command NotificationCommand) error
}

// verificationNotification builds the user-facing message for a terminal
// transition. It returns ok=false for rows without a subject user, for
// non-terminal states, and for duplicate observations of the same status, so
// callers skip publishing without re-deriving domain rules.
//
// The content is deliberately generic: provider reasons and raw results can
// carry identity data and must never be copied into an inbox row.
func verificationNotification(verification *domain.Verification, previous domain.VerificationStatus) (NotificationCommand, bool) {
	if verification == nil || verification.UserID == nil {
		return NotificationCommand{}, false
	}
	userID := strings.TrimSpace(*verification.UserID)
	if userID == "" {
		return NotificationCommand{}, false
	}
	if !verification.Status.IsTerminal() || verification.Status == previous {
		return NotificationCommand{}, false
	}

	title := "身份认证已通过"
	content := "您的身份认证已通过，实名信息已更新。"
	switch verification.Status {
	case domain.StatusRejected:
		title = "身份认证未通过"
		content = "您的身份认证未通过，请核对证件信息后重试，或联系管理员。"
	case domain.StatusExpired:
		title = "身份认证已过期"
		content = "本次身份认证会话已过期，请重新发起认证。"
	}

	return NotificationCommand{
		OrganizationID: verification.OrganizationID,
		UserID:         userID,
		Category:       "security",
		EventType:      "idv.verification.completed",
		Title:          title,
		Content:        content,
		Link:           "/idv",
		SourceType:     "identity_verification",
		SourceID:       verification.ID,
	}, true
}

// notifyVerificationResult is best-effort: the verification outcome is already
// committed, so a notification failure is logged instead of turned into a
// callback error (which would make the provider retry the same result).
func (s Service) notifyVerificationResult(ctx context.Context, verification *domain.Verification, previous domain.VerificationStatus) {
	if s.Notifier == nil {
		return
	}
	command, ok := verificationNotification(verification, previous)
	if !ok {
		return
	}
	if err := s.Notifier.Notify(ctx, command); err != nil {
		slog.WarnContext(ctx, "identity verification notification failed",
			"organization_id", command.OrganizationID,
			"verification_id", command.SourceID,
			"status", string(verification.Status),
			"error", err)
	}
}
