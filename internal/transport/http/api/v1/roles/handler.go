package roles

import (
	"context"
	"io"

	approles "github.com/guliangiam/guliangiam/internal/app/roles"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service approles.Service
}

func (h Handler) List(ctx context.Context, request ListRolesRequest) (ListRolesResponse, error) {
	items, total, err := h.Service.ListRoles(ctx, approles.ListRolesQuery(request))
	if err != nil {
		return ListRolesResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetRoleRequest) (*RoleResponse, error) {
	response, err := h.Service.GetRole(ctx, organizationID, request.ID)
	if err != nil {
		return nil, err
	}

	typed := RoleResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateRoleRequest) error {
	return h.Service.CreateRole(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchRoleRequest) error {
	command := request.toCommand(id)
	// The session user, not the target role, is the audit actor.
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		command.ActorID = session.UserID
	}
	return h.Service.PatchRole(ctx, organizationID, command)
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteRole(ctx, organizationID, id)
}

// BulkImportRequest represents a request to import multiple roles.
type BulkImportRequest struct {
	OrganizationID string                `json:"organizationId"`
	Roles          []approles.ImportRole `json:"roles"`
}

// BulkImport imports multiple roles from a JSON payload.
func (h Handler) BulkImport(ctx context.Context, request BulkImportRequest) (*approles.BulkImportResult, error) {
	return h.Service.BulkImportRoles(ctx, request.OrganizationID, request.Roles)
}

// UploadExcel handles multipart Excel file upload for bulk role import.
func (h Handler) UploadExcel(ctx context.Context, reader io.Reader, organizationID string) (*approles.BulkImportResult, error) {
	return h.Service.ImportRolesFromExcel(ctx, reader, organizationID)
}
