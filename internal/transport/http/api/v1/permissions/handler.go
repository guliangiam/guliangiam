package permissions

import (
	"context"
	"io"

	apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service apppermissions.Service
}

func (h Handler) List(ctx context.Context, request ListPermissionsRequest) (ListPermissionsResponse, error) {
	items, total, err := h.Service.ListPermissions(ctx, apppermissions.ListPermissionsQuery(request))
	if err != nil {
		return ListPermissionsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetPermissionRequest) (*PermissionResponse, error) {
	response, err := h.Service.GetPermission(ctx, organizationID, request.ID)
	if err != nil {
		return nil, err
	}

	typed := PermissionResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreatePermissionRequest) error {
	return h.Service.CreatePermission(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchPermissionRequest) error {
	return h.Service.PatchPermission(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeletePermission(ctx, organizationID, id)
}

// ListUserEffectivePermissions expands the user's role bindings into concrete
// resource/action grants. The user id is always scoped to the trusted tenant.
func (h Handler) ListUserEffectivePermissions(ctx context.Context, organizationID, userID string) ([]apppermissions.EffectivePermission, error) {
	return h.Service.ListUserEffectivePermissions(ctx, organizationID, userID)
}

func (h Handler) Enforce(ctx context.Context, organizationID string, request EnforceRequest) (*apppermissions.EnforceResult, error) {
	allowed, err := h.Service.Enforce(ctx, organizationID, request.EnforcerID, apppermissions.EnforceRequest{
		Sub: request.Sub,
		Dom: request.Dom,
		Obj: request.Obj,
		Act: request.Act,
	})
	if err != nil {
		return nil, err
	}
	return &apppermissions.EnforceResult{Allowed: allowed}, nil
}

func (h Handler) BatchEnforce(ctx context.Context, organizationID string, request BatchEnforceRequest) (*apppermissions.BatchEnforceResult, error) {
	requests := make([]apppermissions.EnforceRequest, len(request.Requests))
	for i, r := range request.Requests {
		requests[i] = apppermissions.EnforceRequest{
			Sub: r.Sub,
			Dom: r.Dom,
			Obj: r.Obj,
			Act: r.Act,
		}
	}
	results, err := h.Service.BatchEnforce(ctx, organizationID, request.EnforcerID, requests)
	if err != nil {
		return nil, err
	}
	return &apppermissions.BatchEnforceResult{Results: results}, nil
}

func (h Handler) GetPolicies(ctx context.Context, organizationID, id string) ([][]string, error) {
	return h.Service.GetPolicies(ctx, organizationID, id)
}

func (h Handler) UpdatePolicies(ctx context.Context, organizationID, id string, request UpdatePolicyRequest) error {
	policies := make([]apppermissions.PolicyItem, len(request.Policies))
	for i, p := range request.Policies {
		policies[i] = apppermissions.PolicyItem{
			PType: p.PType,
			Rules: p.Rules,
		}
	}
	return h.Service.UpdatePolicies(ctx, organizationID, id, policies)
}

func (h Handler) GetAllObjects(ctx context.Context, organizationID, id string) ([]string, error) {
	return h.Service.GetAllObjects(ctx, organizationID, id)
}

func (h Handler) GetAllActions(ctx context.Context, organizationID, id string) ([]string, error) {
	return h.Service.GetAllActions(ctx, organizationID, id)
}

func (h Handler) GetAllRoles(ctx context.Context, organizationID, id string) ([]string, error) {
	return h.Service.GetAllRoles(ctx, organizationID, id)
}

// BulkImportRequest represents a request to import multiple permissions.
type BulkImportRequest struct {
	OrganizationID string                            `json:"organizationId"`
	Permissions    []apppermissions.ImportPermission `json:"permissions"`
}

// BulkImport imports multiple permissions from a JSON payload.
func (h Handler) BulkImport(ctx context.Context, request BulkImportRequest) (*apppermissions.BulkImportResult, error) {
	return h.Service.BulkImportPermissions(ctx, request.OrganizationID, request.Permissions)
}

// UploadExcel handles multipart Excel file upload for bulk permission import.
func (h Handler) UploadExcel(ctx context.Context, reader io.Reader, organizationID string) (*apppermissions.BulkImportResult, error) {
	return h.Service.ImportPermissionsFromExcel(ctx, reader, organizationID)
}
