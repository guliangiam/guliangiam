package enforcers

import (
	"context"
	"io"

	appenforcers "github.com/guliangiam/guliangiam/internal/app/enforcers"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct{ Service appenforcers.Service }

func NewHandler(s appenforcers.Service) Handler { return Handler{Service: s} }

// withActor 把可信会话用户注入 ctx，供 app 层审计 actor 归因（BE-D-13）。
func withActor(ctx context.Context) context.Context {
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		return appenforcers.WithActorID(ctx, session.UserID)
	}
	return ctx
}

type ListEnforcersResponse = presenter.ListResponse[appenforcers.EnforcerListItem]
type EnforcerResponse = appenforcers.EnforcerDetail

type ListEnforcersRequest struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
}
type GetEnforcerRequest struct{ ID string }
type CreateEnforcerRequest struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description,omitempty"`
	ModelID        string `json:"modelId,omitempty"`
	AdapterID      string `json:"adapterId,omitempty"`
	IsEnabled      *bool  `json:"isEnabled,omitempty"`
	// Phase 2 fields
	Owner string `json:"owner,omitempty"`
}
type PatchEnforcerRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	Description *string `json:"description,omitempty"`
	ModelID     *string `json:"modelId,omitempty"`
	AdapterID   *string `json:"adapterId,omitempty"`
	IsEnabled   *bool   `json:"isEnabled,omitempty"`
	Revision    int64   `json:"revision,omitempty"`
	// Phase 2 fields
	Owner *string `json:"owner,omitempty"`
}
type EnforceRequest struct {
	PType string `json:"pType"`
	V0    string `json:"v0"`
	V1    string `json:"v1"`
	V2    string `json:"v2,omitempty"`
	V3    string `json:"v3,omitempty"`
	V4    string `json:"v4,omitempty"`
	V5    string `json:"v5,omitempty"`
}
type BatchEnforceRequest struct {
	Requests []EnforceRequest `json:"requests"`
}
type PolicyRequest struct {
	PType string   `json:"pType"`
	Rules []string `json:"rules"`
}

func (h Handler) List(ctx context.Context, req ListEnforcersRequest) (ListEnforcersResponse, error) {
	items, total, err := h.Service.ListEnforcers(ctx, appenforcers.ListEnforcersQuery(req))
	if err != nil {
		return ListEnforcersResponse{}, err
	}
	return presenter.NewListResponse(items, req.Page, req.PageSize, total), nil
}
func (h Handler) Get(ctx context.Context, organizationID string, req GetEnforcerRequest) (*EnforcerResponse, error) {
	r, err := h.Service.GetEnforcer(ctx, organizationID, req.ID)
	if err != nil {
		return nil, err
	}
	t := EnforcerResponse(*r)
	return &t, nil
}
func (h Handler) Create(ctx context.Context, req CreateEnforcerRequest) error {
	return h.Service.CreateEnforcer(withActor(ctx), appenforcers.CreateEnforcerCommand{
		OrganizationID: req.OrganizationID, Name: req.Name, DisplayName: req.DisplayName,
		Description: req.Description, ModelID: req.ModelID, AdapterID: req.AdapterID, IsEnabled: req.IsEnabled,
		Owner: req.Owner,
	})
}
func (h Handler) Patch(ctx context.Context, organizationID, id string, req PatchEnforcerRequest) error {
	return h.Service.PatchEnforcer(withActor(ctx), organizationID, appenforcers.PatchEnforcerCommand{
		ID: id, DisplayName: req.DisplayName, Description: req.Description,
		ModelID: req.ModelID, AdapterID: req.AdapterID, IsEnabled: req.IsEnabled, Revision: req.Revision,
		Owner: req.Owner,
	})
}
func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteEnforcer(withActor(ctx), organizationID, id)
}
func (h Handler) Enforce(ctx context.Context, organizationID, id string, req EnforceRequest) (*appenforcers.EnforceResult, error) {
	return h.Service.Enforce(ctx, organizationID, appenforcers.EnforceCommand{
		ID: id, PType: req.PType, V0: req.V0, V1: req.V1, V2: req.V2, V3: req.V3, V4: req.V4, V5: req.V5,
	})
}
func (h Handler) BatchEnforce(ctx context.Context, organizationID, id string, req BatchEnforceRequest) (*appenforcers.BatchEnforceResult, error) {
	cmds := make([]appenforcers.EnforceCommand, len(req.Requests))
	for i, r := range req.Requests {
		cmds[i] = appenforcers.EnforceCommand{
			ID: id, PType: r.PType, V0: r.V0, V1: r.V1, V2: r.V2, V3: r.V3, V4: r.V4, V5: r.V5,
		}
	}
	return h.Service.BatchEnforce(ctx, organizationID, appenforcers.BatchEnforceCommand{ID: id, Requests: cmds})
}
func (h Handler) AddPolicy(ctx context.Context, organizationID, id string, req PolicyRequest) error {
	return h.Service.AddPolicy(withActor(ctx), organizationID, id, append([]string{req.PType}, req.Rules...))
}
func (h Handler) RemovePolicy(ctx context.Context, organizationID, id string, req PolicyRequest) error {
	return h.Service.RemovePolicy(withActor(ctx), organizationID, id, append([]string{req.PType}, req.Rules...))
}
func (h Handler) GetAllSubjects(ctx context.Context, organizationID, id string) ([]string, error) {
	return h.Service.GetAllSubjects(ctx, organizationID, id)
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
func (h Handler) ReloadPolicy(ctx context.Context, organizationID, id string) error {
	return h.Service.ReloadPolicy(withActor(ctx), organizationID, id)
}
func (h Handler) GetPolicies(ctx context.Context, organizationID, id string) ([][]string, error) {
	return h.Service.GetPolicies(ctx, organizationID, id)
}
func (h Handler) GetFilteredPolicies(ctx context.Context, organizationID, id string, fieldIndex int, fieldValues []string) ([][]string, error) {
	return h.Service.GetFilteredPolicies(ctx, organizationID, id, fieldIndex, fieldValues...)
}

type UpdatePolicyRequest struct {
	OldPolicy []string `json:"oldPolicy"`
	NewPolicy []string `json:"newPolicy"`
}

func (h Handler) UpdatePolicy(ctx context.Context, organizationID, id string, req UpdatePolicyRequest) error {
	return h.Service.UpdatePolicy(withActor(ctx), organizationID, id, req.OldPolicy, req.NewPolicy)
}
func (h Handler) RefreshEngines(ctx context.Context) error {
	return h.Service.RefreshEngines(ctx)
}

func (h Handler) ExportPolicies(ctx context.Context, organizationID, id string) (*appenforcers.PolicyExport, error) {
	return h.Service.ExportPolicies(ctx, organizationID, id)
}

func (h Handler) ImportPolicies(ctx context.Context, organizationID, id string, policies [][]string) error {
	return h.Service.ImportPolicies(withActor(ctx), organizationID, id, policies)
}

// UploadPoliciesExcel handles multipart Excel file upload for bulk policy import.
func (h Handler) UploadPoliciesExcel(ctx context.Context, organizationID, id string, reader io.Reader) (*appenforcers.PolicyUploadResult, error) {
	return h.Service.ImportPoliciesFromExcel(withActor(ctx), organizationID, id, reader)
}
