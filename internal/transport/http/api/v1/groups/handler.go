package groups

import (
	"context"
	"io"

	appgroups "github.com/guliangiam/guliangiam/internal/app/groups"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appgroups.Service
}

func (h Handler) List(ctx context.Context, request ListGroupsRequest) (ListGroupsResponse, error) {
	items, total, err := h.Service.ListGroups(ctx, appgroups.ListGroupsQuery(request))
	if err != nil {
		return ListGroupsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetGroupRequest) (*GroupResponse, error) {
	response, err := h.Service.GetGroup(ctx, organizationID, request.ID)
	if err != nil {
		return nil, err
	}

	typed := GroupResponse(*response)
	return &typed, nil
}

// operatorID returns the authenticated session's user id, or "" for system
// paths that carry no session. Callers write it into the command so the audit
// provider can attribute the write to the operator.
func operatorID(ctx context.Context) string {
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		return session.UserID
	}
	return ""
}

func (h Handler) Create(ctx context.Context, request CreateGroupRequest) (*CreateGroupResponse, error) {
	command := request.toCommand()
	command.ActorID = operatorID(ctx)
	group, err := h.Service.CreateGroup(ctx, command)
	if err != nil {
		return nil, err
	}

	return &CreateGroupResponse{ID: group.ID, Status: "created"}, nil
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchGroupRequest) error {
	command := request.toCommand(id)
	command.ActorID = operatorID(ctx)
	return h.Service.PatchGroup(ctx, organizationID, command)
}

func (h Handler) Delete(ctx context.Context, organizationID string, request GetGroupRequest) error {
	return h.Service.DeleteGroup(ctx, organizationID, appgroups.DeleteGroupCommand{
		ID:      request.ID,
		ActorID: operatorID(ctx),
	})
}

// BulkImportRequest represents a request to import multiple groups.
type BulkImportRequest struct {
	OrganizationID string                  `json:"organizationId"`
	Groups         []appgroups.ImportGroup `json:"groups"`
}

// BulkImport imports multiple groups from a JSON payload.
func (h Handler) BulkImport(ctx context.Context, request BulkImportRequest) (*appgroups.BulkImportResult, error) {
	return h.Service.BulkImportGroups(ctx, request.OrganizationID, request.Groups)
}

// UploadExcel handles multipart Excel file upload for bulk group import.
func (h Handler) UploadExcel(ctx context.Context, reader io.Reader, organizationID string) (*appgroups.BulkImportResult, error) {
	return h.Service.ImportGroupsFromExcel(ctx, reader, organizationID)
}
