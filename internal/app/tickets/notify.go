package tickets

import (
	"context"
	"log/slog"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/tickets"
)

// NotificationCommand is the minimal, transport-agnostic notification request
// the ticket use cases emit. The composition root implements Notifier and maps
// it onto the personal notification center, so this package stays independent
// of app/usernotifications.
type NotificationCommand struct {
	OrganizationID    string
	RecipientUserID   string
	RecipientUsername string
	Category          string
	EventType         string
	Title             string
	Content           string
	Link              string
	SourceType        string
	SourceID          string
	AllowReply        bool
}

// Notifier is the optional outbound port for ticket notifications. A nil
// notifier disables notification fan-out without affecting ticket writes.
type Notifier interface {
	Notify(ctx context.Context, command NotificationCommand) error
}

// notifyPatch 在工单更新提交后提醒业务相关人：状态流转提醒创建者，改派提醒
// 新受理人。二者都是「业务提醒」，与 tickets.created/replied 同属 feedback 分类。
func (s Service) notifyPatch(ctx context.Context, t *domain.Ticket, previousStatus, previousAssignee string) {
	if t == nil {
		return
	}
	name := ticketDisplayName(t)
	if t.Status != previousStatus && strings.TrimSpace(t.Creator) != "" {
		s.notify(ctx, NotificationCommand{
			OrganizationID:    t.OrganizationID,
			RecipientUsername: t.Creator,
			Category:          "feedback",
			EventType:         "tickets.status_changed",
			Title:             "工单状态已更新",
			Content:           "工单「" + name + "」状态已变更为「" + ticketStatusLabel(t.Status) + "」，点击查看详情。",
			Link:              "/tickets/" + t.ID,
			SourceType:        "ticket",
			SourceID:          t.ID,
			AllowReply:        true,
		})
	}
	if t.Assignee != previousAssignee {
		s.notifyAssigned(ctx, t)
	}
}

// notifyAssigned tells the assignee a ticket is now theirs. It is a no-op when
// the ticket has no assignee or the assignee is the creator themselves.
func (s Service) notifyAssigned(ctx context.Context, t *domain.Ticket) {
	if t == nil || strings.TrimSpace(t.Assignee) == "" || t.Assignee == t.Creator {
		return
	}
	s.notify(ctx, NotificationCommand{
		OrganizationID:    t.OrganizationID,
		RecipientUsername: t.Assignee,
		Category:          "feedback",
		EventType:         "tickets.assigned",
		Title:             "工单已分配给你",
		Content:           "工单「" + ticketDisplayName(t) + "」已分配给你，请及时跟进处理。",
		Link:              "/tickets/" + t.ID,
		SourceType:        "ticket",
		SourceID:          t.ID,
		AllowReply:        true,
	})
}

func ticketDisplayName(t *domain.Ticket) string {
	if strings.TrimSpace(t.DisplayName) != "" {
		return t.DisplayName
	}
	return t.Name
}

func ticketStatusLabel(status string) string {
	switch status {
	case domain.StatusOpen:
		return "待处理"
	case domain.StatusPending:
		return "处理中"
	case domain.StatusResolved:
		return "已解决"
	case domain.StatusClosed:
		return "已关闭"
	default:
		return status
	}
}

// notify is best-effort: notification failures must never fail or roll back a
// ticket write. The failure is logged with enough context to replay manually.
func (s Service) notify(ctx context.Context, command NotificationCommand) {
	if s.Notifier == nil {
		return
	}
	if strings.TrimSpace(command.RecipientUserID) == "" && strings.TrimSpace(command.RecipientUsername) == "" {
		return
	}
	if err := s.Notifier.Notify(ctx, command); err != nil {
		slog.WarnContext(ctx, "ticket notification failed",
			"organization_id", command.OrganizationID,
			"ticket_id", command.SourceID,
			"event_type", command.EventType,
			"error", err)
	}
}
