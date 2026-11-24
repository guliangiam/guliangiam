package payments

import (
	"context"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// TokenVerifier 是本包对 JWT 验证能力的本地接口依赖。
// 由 bootstrap/wire_transport.go 注入 infra/jwt.Issuer 具体实现。
type TokenVerifier interface {
	ValidateToken(tokenString string) (map[string]interface{}, error)
}

type Handler struct {
	Service   apppayments.Service
	JWTIssuer TokenVerifier
	// Charge 发起支付编排（OPEN-API-71 运行期接线）；nil 时 charge 端点返回不可用。
	Charge *apppayments.ChargeOrchestrator
	// Refund 发起退款编排（BE-C-09 运行期接线）；nil 时 refund 端点返回不可用。
	Refund apppayments.RefundExecutor
}

func NewHandler(s apppayments.Service, jwtIssuer TokenVerifier) Handler {
	return Handler{Service: s, JWTIssuer: jwtIssuer}
}

// ChargePayment 发起一次支付：状态门禁与渠道激活校验由 ChargeOrchestrator 负责
// （AC-PAY-006 draft 配置 → payment_channel_not_activated 422），成功后回读最新支付详情。
func (h Handler) ChargePayment(ctx context.Context, organizationID, id string) (*apppayments.PaymentDetail, error) {
	if h.Charge == nil {
		return nil, apperrors.NewUnimplemented("payment charge orchestration is not wired")
	}
	if _, err := h.Charge.Execute(ctx, id, organizationID); err != nil {
		return nil, err
	}
	return h.Service.GetPayment(ctx, organizationID, id)
}

// RefundPayment 发起一次退款：金额/状态/可退额度门禁与 org 归属断言由
// RefundOrchestrator 负责（BE-C-09），成功后返回退款单详情。
func (h Handler) RefundPayment(ctx context.Context, organizationID, id string, amount float64, reason string) (*apppayments.RefundDetail, error) {
	if h.Refund == nil {
		return nil, apperrors.NewUnimplemented("payment refund orchestration is not wired")
	}
	refund, err := h.Refund.ExecuteRefund(ctx, id, organizationID, amount, reason)
	if err != nil {
		return nil, err
	}
	return apppayments.NewRefundDetail(refund), nil
}

// RefundPaymentRequest 是 POST /payments/{id}/refund 的请求体。
type RefundPaymentRequest struct {
	OrganizationID string  `json:"organizationId"`
	Amount         float64 `json:"amount"`
	Reason         string  `json:"reason"`
}

type ListPaymentsResponse = presenter.ListResponse[apppayments.PaymentListItem]
type PaymentResponse = apppayments.PaymentDetail

type ListPaymentsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	User           string
	State          *string
}

type GetPaymentRequest struct {
	ID string
}

type CreatePaymentRequest struct {
	OrganizationID     string   `json:"organizationId"`
	Name               string   `json:"name"`
	DisplayName        string   `json:"displayName"`
	Provider           string   `json:"provider"`
	Type               string   `json:"type"`
	Products           []string `json:"products"`
	ProductName        string   `json:"productName"`
	ProductDisplayName string   `json:"productDisplayName"`
	Detail             string   `json:"detail"`
	Currency           string   `json:"currency"`
	Price              float64  `json:"price"`
	User               string   `json:"user"`
	PersonName         string   `json:"personName"`
	PersonIdCard       string   `json:"personIdCard"`
	PersonEmail        string   `json:"personEmail"`
	PersonPhone        string   `json:"personPhone"`
	InvoiceType        string   `json:"invoiceType"`
	InvoiceTitle       string   `json:"invoiceTitle"`
	InvoiceTaxId       string   `json:"invoiceTaxId"`
	InvoiceRemark      string   `json:"invoiceRemark"`
	InvoiceUrl         string   `json:"invoiceUrl"`
	Order              string   `json:"order"`
	OutOrderId         string   `json:"outOrderId"`
	PayUrl             string   `json:"payUrl"`
	SuccessUrl         string   `json:"successUrl"`
	State              string   `json:"state"`
	Message            string   `json:"message"`
}

type PatchPaymentRequest struct {
	DisplayName *string `json:"displayName"`
	Provider    *string `json:"provider"`
	Type        *string `json:"type"`
	State       *string `json:"state"`
	Message     *string `json:"message"`
	PayUrl      *string `json:"payUrl"`
	SuccessUrl  *string `json:"successUrl"`
	OutOrderId  *string `json:"outOrderId"`
	InvoiceUrl  *string `json:"invoiceUrl"`
	Revision    int64   `json:"revision"`
}

func (r CreatePaymentRequest) toCommand() apppayments.CreatePaymentCommand {
	return apppayments.CreatePaymentCommand{
		OrganizationID:     r.OrganizationID,
		Name:               r.Name,
		DisplayName:        r.DisplayName,
		Provider:           r.Provider,
		Type:               r.Type,
		Products:           r.Products,
		ProductName:        r.ProductName,
		ProductDisplayName: r.ProductDisplayName,
		Detail:             r.Detail,
		Currency:           r.Currency,
		Price:              r.Price,
		User:               r.User,
		PersonName:         r.PersonName,
		PersonIdCard:       r.PersonIdCard,
		PersonEmail:        r.PersonEmail,
		PersonPhone:        r.PersonPhone,
		InvoiceType:        r.InvoiceType,
		InvoiceTitle:       r.InvoiceTitle,
		InvoiceTaxId:       r.InvoiceTaxId,
		InvoiceRemark:      r.InvoiceRemark,
		InvoiceUrl:         r.InvoiceUrl,
		Order:              r.Order,
		OutOrderId:         r.OutOrderId,
		PayUrl:             r.PayUrl,
		SuccessUrl:         r.SuccessUrl,
		State:              r.State,
		Message:            r.Message,
	}
}

func (r PatchPaymentRequest) toCommand(id string) apppayments.PatchPaymentCommand {
	return apppayments.PatchPaymentCommand{
		ID:          id,
		DisplayName: r.DisplayName,
		Provider:    r.Provider,
		Type:        r.Type,
		State:       r.State,
		Message:     r.Message,
		PayUrl:      r.PayUrl,
		SuccessUrl:  r.SuccessUrl,
		OutOrderId:  r.OutOrderId,
		InvoiceUrl:  r.InvoiceUrl,
		Revision:    r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListPaymentsRequest) (ListPaymentsResponse, error) {
	q := apppayments.ListPaymentsQuery{
		Page:           request.Page,
		PageSize:       request.PageSize,
		OrganizationID: request.OrganizationID,
		Keyword:        request.Keyword,
		User:           request.User,
		State:          request.State,
	}
	items, total, err := h.Service.ListPayments(ctx, q)
	if err != nil {
		return ListPaymentsResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetPaymentRequest) (*PaymentResponse, error) {
	response, err := h.Service.GetPayment(ctx, organizationID, request.ID)
	if err != nil {
		return nil, err
	}
	typed := PaymentResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreatePaymentRequest) error {
	return h.Service.CreatePayment(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchPaymentRequest) error {
	return h.Service.PatchPayment(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeletePayment(ctx, organizationID, id)
}

// InvoicePaymentRequest represents a request to invoice a payment.
type InvoicePaymentRequest struct {
	InvoiceType   string `json:"invoiceType"`
	InvoiceTitle  string `json:"invoiceTitle"`
	InvoiceTaxId  string `json:"invoiceTaxId"`
	InvoiceRemark string `json:"invoiceRemark"`
	PersonName    string `json:"personName"`
	PersonIdCard  string `json:"personIdCard"`
	PersonEmail   string `json:"personEmail"`
	PersonPhone   string `json:"personPhone"`
	Revision      int64  `json:"revision"`
}

// InvoicePayment updates the invoice information for a payment.
func (h Handler) InvoicePayment(ctx context.Context, organizationID, id string, request InvoicePaymentRequest) error {
	return h.Service.InvoicePayment(ctx, organizationID, apppayments.InvoicePaymentCommand{
		ID:            id,
		InvoiceType:   request.InvoiceType,
		InvoiceTitle:  request.InvoiceTitle,
		InvoiceTaxId:  request.InvoiceTaxId,
		InvoiceRemark: request.InvoiceRemark,
		PersonName:    request.PersonName,
		PersonIdCard:  request.PersonIdCard,
		PersonEmail:   request.PersonEmail,
		PersonPhone:   request.PersonPhone,
		Revision:      request.Revision,
	})
}

// GetPaymentResult returns the payment result for user-facing queries.
func (h Handler) GetPaymentResult(ctx context.Context, organizationID, id string) (*apppayments.PaymentResult, error) {
	return h.Service.GetPaymentResult(ctx, organizationID, id)
}
