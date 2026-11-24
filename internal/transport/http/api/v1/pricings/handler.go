package pricings

import (
	"context"

	apppricings "github.com/guliangiam/guliangiam/internal/app/pricings"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service apppricings.Service
}

func NewHandler(s apppricings.Service) Handler {
	return Handler{Service: s}
}

type ListPricingsResponse = presenter.ListResponse[apppricings.PricingListItem]
type PricingResponse = apppricings.PricingDetail

type ListPricingsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type CreatePricingRequest struct {
	OrganizationID string   `json:"organizationId"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	Description    string   `json:"description,omitempty"`
	Plans          []string `json:"plans,omitempty"`
	IsEnabled      bool     `json:"isEnabled"`
	TrialDuration  int      `json:"trialDuration,omitempty"`
	Application    string   `json:"application,omitempty"`
}

type PatchPricingRequest struct {
	DisplayName   *string  `json:"displayName,omitempty"`
	Description   *string  `json:"description,omitempty"`
	Plans         []string `json:"plans,omitempty"`
	IsEnabled     *bool    `json:"isEnabled,omitempty"`
	TrialDuration *int     `json:"trialDuration,omitempty"`
	Application   *string  `json:"application,omitempty"`
	Revision      int64    `json:"revision,omitempty"`
}

func (r CreatePricingRequest) toCommand() apppricings.CreatePricingCommand {
	return apppricings.CreatePricingCommand{
		OrganizationID: r.OrganizationID, Name: r.Name, DisplayName: r.DisplayName,
		Description: r.Description, Plans: r.Plans, IsEnabled: r.IsEnabled,
		TrialDuration: r.TrialDuration, Application: r.Application,
	}
}

func (r PatchPricingRequest) toCommand(id string) apppricings.PatchPricingCommand {
	return apppricings.PatchPricingCommand{
		ID: id, DisplayName: r.DisplayName, Description: r.Description,
		Plans: r.Plans, IsEnabled: r.IsEnabled, TrialDuration: r.TrialDuration,
		Application: r.Application, Revision: r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListPricingsRequest) (ListPricingsResponse, error) {
	items, total, err := h.Service.ListPricings(ctx, apppricings.ListPricingsQuery(request))
	if err != nil {
		return ListPricingsResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*PricingResponse, error) {
	response, err := h.Service.GetPricing(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	typed := PricingResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreatePricingRequest) error {
	return h.Service.CreatePricing(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchPricingRequest) error {
	return h.Service.PatchPricing(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeletePricing(ctx, organizationID, id)
}
