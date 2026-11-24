package models

import (
	"context"

	appmodels "github.com/guliangiam/guliangiam/internal/app/models"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct{ Service appmodels.Service }

func NewHandler(s appmodels.Service) Handler { return Handler{Service: s} }

type ListModelsResponse = presenter.ListResponse[appmodels.ModelListItem]
type ModelResponse = appmodels.ModelDetail

type ListModelsRequest struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
}
type CreateModelRequest struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description,omitempty"`
	ModelText      string `json:"modelText,omitempty"`
	// Phase 2 fields
	Owner     string `json:"owner,omitempty"`
	IsEnabled bool   `json:"isEnabled,omitempty"`
}
type PatchModelRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	Description *string `json:"description,omitempty"`
	ModelText   *string `json:"modelText,omitempty"`
	Revision    int64   `json:"revision,omitempty"`
	// Phase 2 fields
	Owner     *string `json:"owner,omitempty"`
	IsEnabled *bool   `json:"isEnabled,omitempty"`
}

func (h Handler) List(ctx context.Context, req ListModelsRequest) (ListModelsResponse, error) {
	items, total, err := h.Service.ListModels(ctx, appmodels.ListModelsQuery(req))
	if err != nil {
		return ListModelsResponse{}, err
	}
	return presenter.NewListResponse(items, req.Page, req.PageSize, total), nil
}
func (h Handler) Get(ctx context.Context, organizationID, id string) (*ModelResponse, error) {
	r, err := h.Service.GetModel(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	t := ModelResponse(*r)
	return &t, nil
}
func (h Handler) Create(ctx context.Context, req CreateModelRequest) error {
	return h.Service.CreateModel(ctx, appmodels.CreateModelCommand{
		OrganizationID: req.OrganizationID, Name: req.Name,
		DisplayName: req.DisplayName, Description: req.Description, ModelText: req.ModelText,
		Owner: req.Owner, IsEnabled: req.IsEnabled,
	})
}
func (h Handler) Patch(ctx context.Context, organizationID, id string, req PatchModelRequest) error {
	return h.Service.PatchModel(ctx, organizationID, appmodels.PatchModelCommand{
		ID: id, DisplayName: req.DisplayName, Description: req.Description, ModelText: req.ModelText, Revision: req.Revision,
		Owner: req.Owner, IsEnabled: req.IsEnabled,
	})
}
func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteModel(ctx, organizationID, id)
}
