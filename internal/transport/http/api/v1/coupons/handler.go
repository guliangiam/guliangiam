package coupons

import (
	"context"

	appcoupons "github.com/guliangiam/guliangiam/internal/app/coupons"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appcoupons.Service
}

func NewHandler(s appcoupons.Service) Handler {
	return Handler{Service: s}
}

type ListCouponsResponse = presenter.ListResponse[appcoupons.CouponListItem]
type CouponResponse = appcoupons.CouponDetail

type ListCouponsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	State          *string
	DiscountType   string
}

type CreateCouponRequest struct {
	OrganizationID  string   `json:"organizationId"`
	Name            string   `json:"name"`
	DisplayName     string   `json:"displayName"`
	Description     string   `json:"description,omitempty"`
	Code            string   `json:"code"`
	DiscountType    string   `json:"discountType"`
	Discount        float64  `json:"discount"`
	MaxDiscount     float64  `json:"maxDiscount,omitempty"`
	Scope           string   `json:"scope,omitempty"`
	Products        []string `json:"products,omitempty"`
	Users           []string `json:"users,omitempty"`
	Quantity        int      `json:"quantity,omitempty"`
	MaxUsagePerUser int      `json:"maxUsagePerUser,omitempty"`
	StartTime       string   `json:"startTime,omitempty"`
	ExpireTime      string   `json:"expireTime,omitempty"`
	MinOrderAmount  float64  `json:"minOrderAmount,omitempty"`
	Currency        string   `json:"currency,omitempty"`
	State           string   `json:"state,omitempty"`
}

type PatchCouponRequest struct {
	DisplayName     *string  `json:"displayName,omitempty"`
	Description     *string  `json:"description,omitempty"`
	Scope           *string  `json:"scope,omitempty"`
	StartTime       *string  `json:"startTime,omitempty"`
	ExpireTime      *string  `json:"expireTime,omitempty"`
	Currency        *string  `json:"currency,omitempty"`
	State           *string  `json:"state,omitempty"`
	Discount        *float64 `json:"discount,omitempty"`
	MaxDiscount     *float64 `json:"maxDiscount,omitempty"`
	MinOrderAmount  *float64 `json:"minOrderAmount,omitempty"`
	Products        []string `json:"products,omitempty"`
	Users           []string `json:"users,omitempty"`
	Quantity        *int     `json:"quantity,omitempty"`
	MaxUsagePerUser *int     `json:"maxUsagePerUser,omitempty"`
	Revision        int64    `json:"revision,omitempty"`
}

func (r CreateCouponRequest) toCommand() appcoupons.CreateCouponCommand {
	return appcoupons.CreateCouponCommand{
		OrganizationID: r.OrganizationID, Name: r.Name, DisplayName: r.DisplayName,
		Description: r.Description, Code: r.Code, DiscountType: r.DiscountType,
		Discount: r.Discount, MaxDiscount: r.MaxDiscount, Scope: r.Scope,
		Products: r.Products, Users: r.Users, Quantity: r.Quantity,
		MaxUsagePerUser: r.MaxUsagePerUser, StartTime: r.StartTime, ExpireTime: r.ExpireTime,
		MinOrderAmount: r.MinOrderAmount, Currency: r.Currency, State: r.State,
	}
}

func (r PatchCouponRequest) toCommand(id string) appcoupons.PatchCouponCommand {
	return appcoupons.PatchCouponCommand{
		ID: id, DisplayName: r.DisplayName, Description: r.Description,
		Scope: r.Scope, StartTime: r.StartTime, ExpireTime: r.ExpireTime,
		Currency: r.Currency, State: r.State, Discount: r.Discount,
		MaxDiscount: r.MaxDiscount, MinOrderAmount: r.MinOrderAmount,
		Products: r.Products, Users: r.Users, Quantity: r.Quantity,
		MaxUsagePerUser: r.MaxUsagePerUser, Revision: r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListCouponsRequest) (ListCouponsResponse, error) {
	items, total, err := h.Service.ListCoupons(ctx, appcoupons.ListCouponsQuery{Page: request.Page, PageSize: request.PageSize, OrganizationID: request.OrganizationID, Keyword: request.Keyword, State: request.State, DiscountType: request.DiscountType})
	if err != nil {
		return ListCouponsResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*CouponResponse, error) {
	response, err := h.Service.GetCoupon(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	typed := CouponResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateCouponRequest) error {
	return h.Service.CreateCoupon(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchCouponRequest) error {
	return h.Service.PatchCoupon(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteCoupon(ctx, organizationID, id)
}

// ValidateCoupon validates a coupon code and returns the result.
func (h Handler) ValidateCoupon(ctx context.Context, orgID string, code string, user string, productNames []string, orderAmount float64) (*appcoupons.ValidateCouponResult, error) {
	return h.Service.ValidateCoupon(ctx, orgID, code, user, productNames, orderAmount)
}
