package tickets

import (
	"context"

	apptickets "github.com/guliangiam/guliangiam/internal/app/tickets"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service apptickets.Service
}

func NewHandler(s apptickets.Service) Handler {
	return Handler{Service: s}
}

type ListTicketsResponse = presenter.ListResponse[apptickets.TicketListItem]
type TicketResponse = apptickets.TicketDetail

type ListTicketsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         *string
	Assignee       string
	Creator        string
	Priority       string
	Type           string
}

type GetTicketRequest struct {
	ID             string
	OrganizationID string
}

type CreateTicketRequest struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Type           string `json:"type,omitempty"`
	Status         string `json:"status,omitempty"`
	Priority       string `json:"priority,omitempty"`
	Assignee       string `json:"assignee,omitempty"`
	Creator        string `json:"creator,omitempty"`
	Description    string `json:"description,omitempty"`
	Attachment     string `json:"attachment,omitempty"`
}

type PatchTicketRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	Type        *string `json:"type,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *string `json:"priority,omitempty"`
	Assignee    *string `json:"assignee,omitempty"`
	Description *string `json:"description,omitempty"`
	Attachment  *string `json:"attachment,omitempty"`
	Revision    int64   `json:"revision,omitempty"`
}

func (r CreateTicketRequest) toCommand() apptickets.CreateTicketCommand {
	return apptickets.CreateTicketCommand{
		OrganizationID: r.OrganizationID, Name: r.Name, DisplayName: r.DisplayName,
		Type: r.Type, Status: r.Status, Priority: r.Priority, Assignee: r.Assignee,
		Creator: r.Creator, Description: r.Description, Attachment: r.Attachment,
	}
}

func (r PatchTicketRequest) toCommand(id, organizationID string) apptickets.PatchTicketCommand {
	return apptickets.PatchTicketCommand{
		ID: id, OrganizationID: organizationID,
		DisplayName: r.DisplayName, Type: r.Type, Status: r.Status,
		Priority: r.Priority, Assignee: r.Assignee, Description: r.Description,
		Attachment: r.Attachment, Revision: r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListTicketsRequest) (ListTicketsResponse, error) {
	items, total, err := h.Service.ListTickets(ctx, apptickets.ListTicketsQuery{Page: request.Page, PageSize: request.PageSize, OrganizationID: request.OrganizationID, Keyword: request.Keyword, Status: request.Status, Assignee: request.Assignee, Creator: request.Creator, Priority: request.Priority, Type: request.Type})
	if err != nil {
		return ListTicketsResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetTicketRequest) (*TicketResponse, error) {
	response, err := h.Service.GetTicket(ctx, organizationID, request.ID)
	if err != nil {
		return nil, err
	}
	typed := TicketResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateTicketRequest) error {
	return h.Service.CreateTicket(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchTicketRequest) error {
	return h.Service.PatchTicket(ctx, request.toCommand(id, organizationID))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteTicket(ctx, organizationID, id)
}

// AddTicketMessage adds a message to a ticket.
func (h Handler) AddTicketMessage(ctx context.Context, organizationID, ticketID string, author string, content string, attachment string) (*TicketResponse, error) {
	result, err := h.Service.AddTicketMessage(ctx, organizationID, ticketID, author, content, attachment)
	if err != nil {
		return nil, err
	}
	typed := TicketResponse(*result)
	return &typed, nil
}

// ListTicketMessagesResponse is the paginated response for ticket messages.
type ListTicketMessagesResponse = presenter.ListResponse[apptickets.TicketMessage]

// ListTicketMessages returns the messages for a ticket with pagination.
func (h Handler) ListTicketMessages(ctx context.Context, organizationID, ticketID string, page, pageSize int) (ListTicketMessagesResponse, error) {
	messages, total, err := h.Service.ListTicketMessages(ctx, organizationID, ticketID, page, pageSize)
	if err != nil {
		return ListTicketMessagesResponse{}, err
	}
	return presenter.NewListResponse(messages, page, pageSize, total), nil
}
