package tickets

import (
	"context"
	"fmt"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// DESIGN NOTE (M4-long-term): Messages are serialized into ticket.Description using "---"
// as delimiter, with escapeMessageContent / unescapeMessageContent guarding against
// user-supplied "---" breaking parsing. This is safe for current scale, but the
// production-grade solution is a dedicated ticket_messages table with proper
// columns (id, ticket_id, author, content, attachment, created_at) + a migration
// that extracts embedded messages into the new table. Scheduled M4 milestone.

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.CommerceUoW
	// Notifier 可选：注入后工单创建/回复会同步写入个人通知中心。
	Notifier Notifier
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.CommerceUoW) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

type ListTicketsQuery struct {
	Page, PageSize                             int
	OrganizationID, Keyword, Assignee, Creator string
	Status                                     *string
	Priority                                   string
	Type                                       string
}
type CreateTicketCommand struct {
	OrganizationID, Name, DisplayName, Type, Status, Priority, Assignee, Creator, Description, Attachment string
}
type PatchTicketCommand struct {
	ID                                                                     string
	OrganizationID                                                         string
	DisplayName, Type, Status, Priority, Assignee, Description, Attachment *string
	Revision                                                               int64
}

type TicketListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Type           string    `json:"type,omitempty"`
	Status         string    `json:"status"`
	Priority       string    `json:"priority,omitempty"`
	Assignee       string    `json:"assignee,omitempty"`
	Creator        string    `json:"creator,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type TicketDetail struct {
	TicketListItem
	Description string     `json:"description,omitempty"`
	Attachment  string     `json:"attachment,omitempty"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
	Revision    int64      `json:"revision"`
}

func toListItem(t *domain.Ticket) TicketListItem {
	return TicketListItem{ID: t.ID, OrganizationID: t.OrganizationID, Name: t.Name, DisplayName: t.DisplayName,
		Type: t.Type, Status: t.Status, Priority: t.Priority, Assignee: t.Assignee, Creator: t.Creator,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}
func toDetail(t *domain.Ticket) TicketDetail {
	return TicketDetail{TicketListItem: toListItem(t), Description: t.Description,
		Attachment: t.Attachment, ResolvedAt: t.ResolvedAt, Revision: t.Revision}
}

func (s Service) ListTickets(ctx context.Context, q ListTicketsQuery) ([]TicketListItem, int64, error) {
	items, total, err := s.UOW.Tickets().List(ctx, domain.ListQuery{Page: q.Page, PageSize: q.PageSize, OrganizationID: q.OrganizationID, Keyword: q.Keyword, Status: q.Status, Assignee: q.Assignee, Creator: q.Creator, Priority: q.Priority, Type: q.Type})
	if err != nil {
		return nil, 0, err
	}
	r := make([]TicketListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

// requireTicketTenant keeps the tenant invariant at the app boundary. The
// repository already filters by organization_id, but a dialect whose predicate
// regresses must not turn a foreign row into a successful read or write.
func requireTicketTenant(t *domain.Ticket, organizationID string) error {
	if t == nil || t.OrganizationID != organizationID {
		return apperrors.NewNotFound("ticket not found")
	}
	return nil
}

func (s Service) GetTicket(ctx context.Context, organizationID, id string) (*TicketDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// 租户谓词在仓储层显式生效；MySQL 无 RLS，不能依赖数据库兜底。
	t, err := s.UOW.Tickets().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if err := requireTicketTenant(t, organizationID); err != nil {
		return nil, err
	}
	d := toDetail(t)
	return &d, nil
}
func (s Service) CreateTicket(ctx context.Context, c CreateTicketCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	status := domain.StatusOpen
	if c.Status != "" {
		status = c.Status
	}
	t := &domain.Ticket{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Type: c.Type, Status: status, Priority: c.Priority, Assignee: c.Assignee, Creator: c.Creator,
		Description: c.Description, Attachment: c.Attachment, CreatedAt: now, UpdatedAt: now, Revision: 1}
	if err := t.Validate(); err != nil {
		return err
	}
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Tickets().Create(ctx, t); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			t.OrganizationID,
			"tickets.create",
			t.ID,
			"tickets.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.notify(ctx, NotificationCommand{
		OrganizationID:    t.OrganizationID,
		RecipientUsername: t.Creator,
		Category:          "feedback",
		EventType:         "tickets.created",
		Title:             "工单已创建",
		Content:           "你的工单「" + t.Name + "」已提交，我们会尽快处理，进展会同步到通知中心。",
		Link:              "/tickets/" + t.ID,
		SourceType:        "ticket",
		SourceID:          t.ID,
		AllowReply:        true,
	})
	s.notifyAssigned(ctx, t)
	return nil
}

func (s Service) PatchTicket(ctx context.Context, c PatchTicketCommand) error {
	if err := requireOrganizationID(c.OrganizationID); err != nil {
		return err
	}
	var (
		patched          *domain.Ticket
		previousStatus   string
		previousAssignee string
	)
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		t, err := tx.Tickets().GetByID(ctx, c.ID, c.OrganizationID)
		if err != nil {
			return err
		}
		if err := requireTicketTenant(t, c.OrganizationID); err != nil {
			return err
		}
		previousStatus = t.Status
		previousAssignee = t.Assignee
		if c.Revision > 0 && t.Revision != c.Revision {
			return apperrors.NewConflict("ticket revision mismatch")
		}
		if c.DisplayName != nil {
			t.DisplayName = *c.DisplayName
		}
		if c.Type != nil {
			// The whitelist guards new writes only; historical rows may keep
			// a legacy free-text type and stay readable.
			if *c.Type != "" && !domain.IsKnownType(*c.Type) {
				return apperrors.NewWithMeta(
					apperrors.CodeUnprocessableEntity,
					fmt.Sprintf("invalid ticket type: %s", *c.Type),
					map[string]any{"field": "type", "value": *c.Type, "allowed": domain.Types.SortedValues()},
				)
			}
			t.Type = *c.Type
		}
		if c.Status != nil {
			// A status change is a lifecycle edge, so it goes through the
			// domain transition table rather than being written straight
			// through: the registry (§1.5) allows `pending ⇄ open/resolved`
			// but never a move out of the terminal `closed`.
			if err := t.TransitionTo(*c.Status); err != nil {
				return err
			}
			if domain.ResolvesWork(*c.Status) {
				now := s.Clock.Now()
				t.ResolvedAt = &now
			}
		}
		if c.Priority != nil {
			t.Priority = *c.Priority
		}
		if c.Assignee != nil {
			t.Assignee = *c.Assignee
		}
		if c.Description != nil {
			t.Description = *c.Description
		}
		if c.Attachment != nil {
			t.Attachment = *c.Attachment
		}
		t.UpdatedAt = s.Clock.Now()
		t.Revision++
		if err := tx.Tickets().Update(ctx, t); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			t.OrganizationID,
			"tickets.update",
			t.ID,
			"tickets.update",
			map[string]any{},
		); err != nil {
			return err
		}
		if previousStatus != t.Status {
			if err := s.embedAuditEvent(
				tx,
				ctx,
				t.OrganizationID,
				"tickets.update",
				t.ID,
				"tickets.status_changed",
				map[string]any{"from": previousStatus, "to": t.Status},
			); err != nil {
				return err
			}
		}
		if previousAssignee != t.Assignee {
			if err := s.embedAuditEvent(
				tx,
				ctx,
				t.OrganizationID,
				"tickets.update",
				t.ID,
				"tickets.assigned",
				map[string]any{"from": previousAssignee, "to": t.Assignee},
			); err != nil {
				return err
			}
		}
		patched = t
		return nil
	})
	if err != nil {
		return err
	}
	// 状态流转/改派属于业务提醒：提交后尽量通知创建者与新受理人，失败不回滚。
	s.notifyPatch(ctx, patched, previousStatus, previousAssignee)
	return nil
}
func (s Service) DeleteTicket(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		t, err := tx.Tickets().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := requireTicketTenant(t, organizationID); err != nil {
			return err
		}
		if err := tx.Tickets().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			t.OrganizationID,
			"tickets.delete",
			id,
			"tickets.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// TicketMessage represents a message attached to a ticket.
type TicketMessage struct {
	Author     string    `json:"author"`
	Content    string    `json:"content"`
	Attachment string    `json:"attachment,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// AddTicketMessage appends a message to a ticket by updating its description
// and transitioning status appropriately.
func (s Service) AddTicketMessage(ctx context.Context, organizationID, ticketID, author, content, attachment string) (*TicketDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return nil, apperrors.NewInvalidArgument("message content is required")
	}
	var detail *TicketDetail
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		t, err := tx.Tickets().GetByID(ctx, ticketID, organizationID)
		if err != nil {
			return err
		}
		if err := requireTicketTenant(t, organizationID); err != nil {
			return err
		}
		if domain.IsTerminal(t.Status) {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "cannot add message to a closed ticket", map[string]any{"field": "status", "value": string(t.Status)})
		}
		now := s.Clock.Now()
		msg := TicketMessage{Author: author, Content: content, Attachment: attachment, CreatedAt: now}
		// Append message as structured text in description
		if t.Description != "" {
			t.Description += "\n\n"
		}
		t.Description += formatMessage(msg)
		// First reply from someone other than the reporter moves the ticket
		// from `open` to `pending` (§1.5).
		if t.Status == domain.StatusOpen && author != t.Creator {
			if err := t.TransitionTo(domain.StatusPending); err != nil {
				return err
			}
		}
		t.UpdatedAt = now
		t.Revision++
		if err := tx.Tickets().Update(ctx, t); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			t.OrganizationID,
			"tickets.update",
			t.ID,
			"tickets.update",
			map[string]any{},
		); err != nil {
			return err
		}
		d := toDetail(t)
		detail = &d
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 通知对手方：客服回复提醒创建者，创建者补充提醒受理人。
	recipient := detail.Creator
	if author == detail.Creator {
		recipient = detail.Assignee
	}
	if recipient != "" && recipient != author {
		s.notify(ctx, NotificationCommand{
			OrganizationID:    organizationID,
			RecipientUsername: recipient,
			Category:          "feedback",
			EventType:         "tickets.replied",
			Title:             "工单有新回复",
			Content:           "工单「" + detail.Name + "」有新的回复，点击查看详情。",
			Link:              "/tickets/" + detail.ID,
			SourceType:        "ticket",
			SourceID:          detail.ID,
			AllowReply:        true,
		})
	}
	return detail, nil
}

// messageDelimiter is the separator used between messages in the description field.
const messageDelimiter = "---"

// escapeMessageContent replaces the message delimiter in user-supplied content
// so that it does not interfere with message parsing.
func escapeMessageContent(s string) string {
	return strings.ReplaceAll(s, messageDelimiter, "\\-\\-\\-")
}

// unescapeMessageContent reverses the escaping applied by escapeMessageContent.
func unescapeMessageContent(s string) string {
	return strings.ReplaceAll(s, "\\-\\-\\-", messageDelimiter)
}

func formatMessage(m TicketMessage) string {
	s := messageDelimiter + "\n"
	s += "Author: " + m.Author + "\n"
	s += "Time: " + m.CreatedAt.Format(time.RFC3339) + "\n"
	if m.Attachment != "" {
		s += "Attachment: " + m.Attachment + "\n"
	}
	s += escapeMessageContent(m.Content)
	return s
}

// ListTicketMessages returns the messages for a ticket, parsed from its description.
// Messages are separated by "---" delimiters within the description field.
func (s Service) ListTicketMessages(ctx context.Context, organizationID, ticketID string, page, pageSize int) ([]TicketMessage, int64, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, 0, err
	}
	t, err := s.UOW.Tickets().GetByID(ctx, ticketID, organizationID)
	if err != nil {
		return nil, 0, err
	}
	if err := requireTicketTenant(t, organizationID); err != nil {
		return nil, 0, err
	}

	messages := parseMessages(t)
	total := int64(len(messages))

	start, end, ok := ticketMessageBounds(page, pageSize, len(messages))
	if !ok {
		return []TicketMessage{}, total, nil
	}
	return messages[start:end], total, nil
}

// ticketMessageBounds maps a 1-based page onto a half-open slice range. The
// page size is clamped before both the offset and the end are computed, so the
// two use the same width and a hostile page cannot wrap the product negative.
func ticketMessageBounds(page, pageSize, total int) (start, end int, ok bool) {
	pageSize = platformpagination.ClampPageSize(pageSize)
	start = platformpagination.Offset(page, pageSize)
	if start >= total {
		return 0, 0, false
	}
	return start, min(start+pageSize, total), true
}

// parseMessages extracts TicketMessage entries from the ticket description.
// The description format uses "---" as a delimiter between messages.
func parseMessages(t *domain.Ticket) []TicketMessage {
	if t.Description == "" {
		return nil
	}

	var messages []TicketMessage
	parts := strings.Split(t.Description, messageDelimiter)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		msg := TicketMessage{}
		lines := strings.Split(part, "\n")
		contentLines := make([]string, 0)

		for _, line := range lines {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "Author: "):
				msg.Author = strings.TrimPrefix(line, "Author: ")
			case strings.HasPrefix(line, "Time: "):
				timeStr := strings.TrimPrefix(line, "Time: ")
				if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
					msg.CreatedAt = parsed
				}
			case strings.HasPrefix(line, "Attachment: "):
				msg.Attachment = strings.TrimPrefix(line, "Attachment: ")
			default:
				contentLines = append(contentLines, line)
			}
		}

		msg.Content = unescapeMessageContent(strings.Join(contentLines, "\n"))
		if msg.Content != "" || msg.Author != "" {
			messages = append(messages, msg)
		}
	}

	return messages
}
