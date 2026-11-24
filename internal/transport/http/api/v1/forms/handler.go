package forms

import (
	"context"

	appforms "github.com/guliangiam/guliangiam/internal/app/forms"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appforms.Service
}

func NewHandler(s appforms.Service) Handler {
	return Handler{Service: s}
}

type ListFormsResponse = presenter.ListResponse[appforms.FormListItem]
type FormResponse = appforms.FormDetail

type ListFormsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Type           string
	IsEnabled      *bool
}

type CreateFormRequest struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description,omitempty"`
	Type           string `json:"type,omitempty"`
	Properties     string `json:"properties,omitempty"`
	IsEnabled      bool   `json:"isEnabled"`
}

type PatchFormRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	Description *string `json:"description,omitempty"`
	Type        *string `json:"type,omitempty"`
	Properties  *string `json:"properties,omitempty"`
	IsEnabled   *bool   `json:"isEnabled,omitempty"`
	Revision    int64   `json:"revision,omitempty"`
}

func (r CreateFormRequest) toCommand() appforms.CreateFormCommand {
	return appforms.CreateFormCommand{
		OrganizationID: r.OrganizationID, Name: r.Name, DisplayName: r.DisplayName,
		Description: r.Description, Type: r.Type, Properties: r.Properties, IsEnabled: r.IsEnabled,
	}
}

func (r PatchFormRequest) toCommand(id string) appforms.PatchFormCommand {
	return appforms.PatchFormCommand{
		ID: id, DisplayName: r.DisplayName, Description: r.Description, Type: r.Type,
		Properties: r.Properties, IsEnabled: r.IsEnabled, Revision: r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListFormsRequest) (ListFormsResponse, error) {
	items, total, err := h.Service.ListForms(ctx, appforms.ListFormsQuery(request))
	if err != nil {
		return ListFormsResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*FormResponse, error) {
	response, err := h.Service.GetForm(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	typed := FormResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateFormRequest) error {
	return h.Service.CreateForm(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchFormRequest) error {
	return h.Service.PatchForm(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteForm(ctx, organizationID, id)
}
