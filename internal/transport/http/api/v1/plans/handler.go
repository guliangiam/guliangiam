package plans

import (
	"context"

	appplans "github.com/guliangiam/guliangiam/internal/app/plans"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appplans.Service
}

func NewHandler(s appplans.Service) Handler {
	return Handler{Service: s}
}

type ListPlansResponse = presenter.ListResponse[appplans.PlanListItem]
type PlanResponse = appplans.PlanDetail

type ListPlansRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Period         string
	State          string
	IsExclusive    *bool
}

type CreatePlanRequest struct {
	OrganizationID   string   `json:"organizationId"`
	Name             string   `json:"name"`
	DisplayName      string   `json:"displayName"`
	Description      string   `json:"description"`
	Price            float64  `json:"price"`
	Currency         string   `json:"currency"`
	Period           string   `json:"period"`
	Product          string   `json:"product"`
	PaymentProviders []string `json:"paymentProviders"`
	State            string   `json:"state"`
	IsExclusive      bool     `json:"isExclusive"`
	Role             string   `json:"role"`
}

type PatchPlanRequest struct {
	DisplayName      *string  `json:"displayName"`
	Description      *string  `json:"description"`
	Currency         *string  `json:"currency"`
	Period           *string  `json:"period"`
	Product          *string  `json:"product"`
	Role             *string  `json:"role"`
	Price            *float64 `json:"price"`
	PaymentProviders []string `json:"paymentProviders"`
	State            *string  `json:"state"`
	IsExclusive      *bool    `json:"isExclusive"`
	Revision         int64    `json:"revision"`
}

func (r CreatePlanRequest) toCommand() appplans.CreatePlanCommand {
	return appplans.CreatePlanCommand{
		OrganizationID:   r.OrganizationID,
		Name:             r.Name,
		DisplayName:      r.DisplayName,
		Description:      r.Description,
		Price:            r.Price,
		Currency:         r.Currency,
		Period:           r.Period,
		Product:          r.Product,
		PaymentProviders: r.PaymentProviders,
		State:            r.State,
		IsExclusive:      r.IsExclusive,
		Role:             r.Role,
	}
}

func (r PatchPlanRequest) toCommand(id string) appplans.PatchPlanCommand {
	return appplans.PatchPlanCommand{
		ID:               id,
		DisplayName:      r.DisplayName,
		Description:      r.Description,
		Currency:         r.Currency,
		Period:           r.Period,
		Product:          r.Product,
		Role:             r.Role,
		Price:            r.Price,
		PaymentProviders: r.PaymentProviders,
		State:            r.State,
		IsExclusive:      r.IsExclusive,
		Revision:         r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListPlansRequest) (ListPlansResponse, error) {
	items, total, err := h.Service.ListPlans(ctx, appplans.ListPlansQuery(request))
	if err != nil {
		return ListPlansResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*PlanResponse, error) {
	response, err := h.Service.GetPlan(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	typed := PlanResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreatePlanRequest) error {
	return h.Service.CreatePlan(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchPlanRequest) error {
	return h.Service.PatchPlan(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeletePlan(ctx, organizationID, id)
}
