package orders

import (
	"context"

	apporders "github.com/guliangiam/guliangiam/internal/app/orders"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service apporders.Service
}

func NewHandler(s apporders.Service) Handler {
	return Handler{Service: s}
}

type ListOrdersResponse = presenter.ListResponse[apporders.OrderListItem]
type OrderResponse = apporders.OrderDetail

type ListOrdersRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	User           string
	State          *string
}

type GetOrderRequest struct {
	ID string
}

type CreateOrderRequest struct {
	OrganizationID string   `json:"organizationId"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	User           string   `json:"user"`
	Products       []string `json:"products"`
	Payment        string   `json:"payment"`
	Price          float64  `json:"price"`
	Currency       string   `json:"currency"`
	CouponName     string   `json:"couponName"`
	CouponDiscount float64  `json:"couponDiscount"`
	NotifyURL      string   `json:"notifyUrl"`
	ReturnURL      string   `json:"returnUrl"`
}

type PatchOrderRequest struct {
	DisplayName    *string  `json:"displayName"`
	Payment        *string  `json:"payment"`
	Message        *string  `json:"message"`
	Price          *float64 `json:"price"`
	Currency       *string  `json:"currency"`
	CouponName     *string  `json:"couponName"`
	CouponDiscount *float64 `json:"couponDiscount"`
	State          *string  `json:"state"`
	Revision       int64    `json:"revision"`
}

func (r CreateOrderRequest) toCommand() apporders.CreateOrderCommand {
	return apporders.CreateOrderCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		DisplayName:    r.DisplayName,
		User:           r.User,
		Products:       r.Products,
		Payment:        r.Payment,
		Price:          r.Price,
		Currency:       r.Currency,
		CouponName:     r.CouponName,
		CouponDiscount: r.CouponDiscount,
		NotifyURL:      r.NotifyURL,
		ReturnURL:      r.ReturnURL,
	}
}

func (r PatchOrderRequest) toCommand(id string) apporders.PatchOrderCommand {
	return apporders.PatchOrderCommand{
		ID:             id,
		DisplayName:    r.DisplayName,
		Payment:        r.Payment,
		Message:        r.Message,
		Price:          r.Price,
		Currency:       r.Currency,
		CouponName:     r.CouponName,
		CouponDiscount: r.CouponDiscount,
		State:          r.State,
		Revision:       r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListOrdersRequest) (ListOrdersResponse, error) {
	items, total, err := h.Service.ListOrders(ctx, apporders.ListOrdersQuery(request))
	if err != nil {
		return ListOrdersResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetOrderRequest) (*OrderResponse, error) {
	response, err := h.Service.GetOrder(ctx, organizationID, request.ID)
	if err != nil {
		return nil, err
	}
	typed := OrderResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateOrderRequest) error {
	return h.Service.CreateOrder(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchOrderRequest) error {
	return h.Service.PatchOrder(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteOrder(ctx, organizationID, id)
}

// PlaceOrder creates an order with business validation (product/pricing/coupon).
func (h Handler) PlaceOrder(ctx context.Context, request CreateOrderRequest) (*OrderResponse, error) {
	result, err := h.Service.PlaceOrder(ctx, request.toCommand())
	if err != nil {
		return nil, err
	}
	typed := OrderResponse(*result)
	return &typed, nil
}

// CancelOrder cancels an order that is in a cancellable state.
func (h Handler) CancelOrder(ctx context.Context, organizationID, id string, revision int64) error {
	return h.Service.CancelOrder(ctx, organizationID, id, revision)
}

// PayOrder marks an order as paid with a payment reference.
func (h Handler) PayOrder(ctx context.Context, organizationID, id, paymentRef string) error {
	return h.Service.PayOrder(ctx, organizationID, id, paymentRef)
}

// GetUserOrders lists orders for a specific user. The caller identity comes
// from the session, never from the request, so the app layer can reject
// cross-user reads without trusting a forged user/query value.
func (h Handler) GetUserOrders(ctx context.Context, orgID, user string, page, pageSize int) (ListOrdersResponse, error) {
	items, total, err := h.Service.GetUserOrders(ctx, orgID, user, sessionUserID(ctx), page, pageSize)
	if err != nil {
		return ListOrdersResponse{}, err
	}
	return presenter.NewListResponse(items, page, pageSize, total), nil
}

// sessionUserID returns the authenticated session's user id, or "" when the
// request has no session (the app layer then denies cross-user reads).
func sessionUserID(ctx context.Context) string {
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		return session.UserID
	}
	return ""
}

// GetOrderPayUrl returns the payment URL for an order.
func (h Handler) GetOrderPayUrl(ctx context.Context, organizationID, id string) (*OrderPayUrlResponse, error) {
	detail, err := h.Service.GetOrder(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	return &OrderPayUrlResponse{
		PayURL: detail.PaymentURL,
		State:  detail.State,
	}, nil
}

// OrderPayUrlResponse contains the payment URL and current order state.
type OrderPayUrlResponse struct {
	PayURL string `json:"payUrl"`
	State  string `json:"state"`
}

// NotifyOrderPaid confirms an order as paid after external payment notification.
// It shares PayOrder's scoped use case so both surfaces answer the same way:
// the channel decides the outcome, the trusted tenant decides visibility.
func (h Handler) NotifyOrderPaid(ctx context.Context, organizationID, id, paymentID string) error {
	return h.Service.PayOrder(ctx, organizationID, id, paymentID)
}
