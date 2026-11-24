package products

import (
	"context"

	appproducts "github.com/guliangiam/guliangiam/internal/app/products"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appproducts.Service
}

func NewHandler(s appproducts.Service) Handler {
	return Handler{Service: s}
}

type ListProductsResponse = presenter.ListResponse[appproducts.ProductListItem]
type ProductResponse = appproducts.ProductDetail

type ListProductsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	State          *string
	IsRecharge     *bool
}

type CreateProductRequest struct {
	OrganizationID        string    `json:"organizationId"`
	Name                  string    `json:"name"`
	DisplayName           string    `json:"displayName"`
	Image                 string    `json:"image"`
	Detail                string    `json:"detail"`
	Description           string    `json:"description"`
	Tag                   string    `json:"tag"`
	Currency              string    `json:"currency"`
	Price                 float64   `json:"price"`
	Quantity              int       `json:"quantity"`
	Sold                  int       `json:"sold"`
	IsRecharge            bool      `json:"isRecharge"`
	RechargeOptions       []float64 `json:"rechargeOptions"`
	DisableCustomRecharge bool      `json:"disableCustomRecharge"`
	Providers             []string  `json:"providers"`
	SuccessUrl            string    `json:"successUrl"`
	State                 string    `json:"state"`
}

type PatchProductRequest struct {
	DisplayName           *string   `json:"displayName,omitempty"`
	Image                 *string   `json:"image,omitempty"`
	Detail                *string   `json:"detail,omitempty"`
	Description           *string   `json:"description,omitempty"`
	Tag                   *string   `json:"tag,omitempty"`
	Currency              *string   `json:"currency,omitempty"`
	Price                 *float64  `json:"price,omitempty"`
	Quantity              *int      `json:"quantity,omitempty"`
	Sold                  *int      `json:"sold,omitempty"`
	IsRecharge            *bool     `json:"isRecharge,omitempty"`
	RechargeOptions       []float64 `json:"rechargeOptions,omitempty"`
	DisableCustomRecharge *bool     `json:"disableCustomRecharge,omitempty"`
	Providers             []string  `json:"providers,omitempty"`
	SuccessUrl            *string   `json:"successUrl,omitempty"`
	State                 *string   `json:"state,omitempty"`
	Revision              int64     `json:"revision,omitempty"`
}

func (r CreateProductRequest) toCommand() appproducts.CreateProductCommand {
	return appproducts.CreateProductCommand{
		OrganizationID:        r.OrganizationID,
		Name:                  r.Name,
		DisplayName:           r.DisplayName,
		Image:                 r.Image,
		Detail:                r.Detail,
		Description:           r.Description,
		Tag:                   r.Tag,
		Currency:              r.Currency,
		Price:                 r.Price,
		Quantity:              r.Quantity,
		Sold:                  r.Sold,
		IsRecharge:            r.IsRecharge,
		RechargeOptions:       r.RechargeOptions,
		DisableCustomRecharge: r.DisableCustomRecharge,
		Providers:             r.Providers,
		SuccessUrl:            r.SuccessUrl,
		State:                 r.State,
	}
}

func (r PatchProductRequest) toCommand(id string) appproducts.PatchProductCommand {
	return appproducts.PatchProductCommand{
		ID:                    id,
		DisplayName:           r.DisplayName,
		Image:                 r.Image,
		Detail:                r.Detail,
		Description:           r.Description,
		Tag:                   r.Tag,
		Currency:              r.Currency,
		Price:                 r.Price,
		Quantity:              r.Quantity,
		Sold:                  r.Sold,
		IsRecharge:            r.IsRecharge,
		RechargeOptions:       r.RechargeOptions,
		DisableCustomRecharge: r.DisableCustomRecharge,
		Providers:             r.Providers,
		SuccessUrl:            r.SuccessUrl,
		State:                 r.State,
		Revision:              r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListProductsRequest) (ListProductsResponse, error) {
	items, total, err := h.Service.ListProducts(ctx, appproducts.ListProductsQuery(request))
	if err != nil {
		return ListProductsResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*ProductResponse, error) {
	response, err := h.Service.GetProduct(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	typed := ProductResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateProductRequest) error {
	return h.Service.CreateProduct(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchProductRequest) error {
	return h.Service.PatchProduct(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteProduct(ctx, organizationID, id)
}

// BuyProduct validates a product inside the trusted tenant and returns
// purchase info.
func (h Handler) BuyProduct(ctx context.Context, organizationID, id, user, pricingName, planName string, quantity int) (*ProductResponse, error) {
	result, err := h.Service.BuyProduct(ctx, organizationID, id, user, pricingName, planName, quantity)
	if err != nil {
		return nil, err
	}
	typed := ProductResponse(*result)
	return &typed, nil
}
