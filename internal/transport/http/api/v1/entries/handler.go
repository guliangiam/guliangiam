package entries

import (
	"context"

	appentries "github.com/guliangiam/guliangiam/internal/app/entries"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appentries.Service
}

func NewHandler(s appentries.Service) Handler {
	return Handler{Service: s}
}

type ListEntriesResponse = presenter.ListResponse[appentries.EntryListItem]
type EntryResponse = appentries.EntryDetail

type ListEntriesRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Type           string
	Application    string
}

type GetEntryRequest struct {
	ID             string
	OrganizationID string
}

// EntryTranscriptResponse is the transcript read page. It is a sequence read,
// not a collection read: it carries an opaque forward cursor rather than page
// numbers, because the stream grows while the reader is looking at it (43.2.6).
type EntryTranscriptResponse = appentries.TranscriptPage

// GetEntryTranscriptRequest reads one page of an entry's message sequence.
type GetEntryTranscriptRequest struct {
	ID string
	// OrganizationID is the caller's tenant scope. A mismatch is answered with
	// 404, never 403, so the response cannot be used to probe for entries of
	// other tenants (43.5.2).
	OrganizationID string
	// Cursor is the opaque value from the previous page's nextCursor.
	Cursor string
	// Limit is the page size. Zero means the server default.
	Limit int
}

type CreateEntryRequest struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Provider       string `json:"provider,omitempty"`
	Application    string `json:"application,omitempty"`
	Type           string `json:"type,omitempty"`
	ClientIP       string `json:"clientIp,omitempty"`
	UserAgent      string `json:"userAgent,omitempty"`
	Message        string `json:"message,omitempty"`
}

type PatchEntryRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	Message     *string `json:"message,omitempty"`
	Revision    int64   `json:"revision,omitempty"`
}

func (r CreateEntryRequest) toCommand() appentries.CreateEntryCommand {
	return appentries.CreateEntryCommand{
		OrganizationID: r.OrganizationID, Name: r.Name, DisplayName: r.DisplayName,
		Provider: r.Provider, Application: r.Application, Type: r.Type,
		ClientIP: r.ClientIP, UserAgent: r.UserAgent, Message: r.Message,
	}
}

func (r PatchEntryRequest) toCommand(id, organizationID string) appentries.PatchEntryCommand {
	return appentries.PatchEntryCommand{
		ID: id, OrganizationID: organizationID,
		DisplayName: r.DisplayName, Message: r.Message, Revision: r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListEntriesRequest) (ListEntriesResponse, error) {
	items, total, err := h.Service.ListEntries(ctx, appentries.ListEntriesQuery{Page: request.Page, PageSize: request.PageSize, OrganizationID: request.OrganizationID, Keyword: request.Keyword, Type: request.Type, Application: request.Application})
	if err != nil {
		return ListEntriesResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetEntryRequest) (*EntryResponse, error) {
	response, err := h.Service.GetEntry(ctx, organizationID, request.ID)
	if err != nil {
		return nil, err
	}
	typed := EntryResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateEntryRequest) error {
	return h.Service.CreateEntry(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchEntryRequest) error {
	return h.Service.PatchEntry(ctx, request.toCommand(id, organizationID))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteEntry(ctx, organizationID, id)
}
