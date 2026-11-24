package usernotifications

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BroadcastCommand sends one administrator-authored message to a set of users.
// Each recipient gets an independent inbox row (and its own external-channel
// fan-out), so read state, replies and deletion stay per-user.
type BroadcastCommand struct {
	OrganizationID string
	UserIDs        []string
	Category       string
	Title          string
	Content        string
	Link           string
	AllowReply     bool
}

// BroadcastResult reports per-recipient outcomes. Partial failure is not an
// error: the caller sees exactly which recipients were skipped.
type BroadcastResult struct {
	Delivered int      `json:"delivered"`
	Failed    int      `json:"failed"`
	FailedIDs []string `json:"failedUserIds,omitempty"`
}

const broadcastMaxRecipients = 500

// Broadcast validates the message once and publishes it to every recipient.
func (s Service) Broadcast(ctx context.Context, command BroadcastCommand) (BroadcastResult, error) {
	if strings.TrimSpace(command.Title) == "" {
		return BroadcastResult{}, apperrors.NewInvalidArgument("title is required")
	}
	if !isKnownCategory(command.Category) {
		return BroadcastResult{}, apperrors.NewInvalidArgument("unknown notification category")
	}
	recipients := dedupeUserIDs(command.UserIDs)
	if len(recipients) == 0 {
		return BroadcastResult{}, apperrors.NewInvalidArgument("at least one userId is required")
	}
	if len(recipients) > broadcastMaxRecipients {
		return BroadcastResult{}, apperrors.NewInvalidArgument("too many recipients (max 500)")
	}

	result := BroadcastResult{}
	for _, userID := range recipients {
		_, err := s.Publish(ctx, PublishCommand{
			OrganizationID: command.OrganizationID,
			UserID:         userID,
			Category:       command.Category,
			EventType:      "notifications.broadcast.sent",
			Title:          command.Title,
			Content:        command.Content,
			Link:           command.Link,
			SourceType:     "broadcast",
			AllowReply:     command.AllowReply,
		})
		if err != nil {
			result.Failed++
			result.FailedIDs = append(result.FailedIDs, userID)
			continue
		}
		result.Delivered++
	}
	return result, nil
}

func isKnownCategory(category string) bool {
	switch domain.Category(category) {
	case domain.CategorySystem, domain.CategorySecurity, domain.CategoryBusiness,
		domain.CategoryFeedback, domain.CategoryAnnouncement:
		return true
	default:
		return false
	}
}

func dedupeUserIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
