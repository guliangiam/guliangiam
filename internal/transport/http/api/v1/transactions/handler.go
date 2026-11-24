package transactions

import (
	"context"
	"encoding/json"

	apptransactions "github.com/guliangiam/guliangiam/internal/app/transactions"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service apptransactions.Service
}

func NewHandler(s apptransactions.Service) Handler {
	return Handler{Service: s}
}

type ListTransactionsResponse = presenter.ListResponse[apptransactions.TransactionListItem]
type TransactionResponse = apptransactions.TransactionDetail

type ListTransactionsRequest struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	UserID, OrderID         string
	Status, TransactionType *string
}

type CreateTransactionRequest struct {
	OrganizationID  string          `json:"organizationId"`
	UserID          string          `json:"userId,omitempty"`
	OrderID         string          `json:"orderId,omitempty"`
	PaymentID       string          `json:"paymentId,omitempty"`
	TransactionType string          `json:"transactionType"`
	Amount          float64         `json:"amount"`
	Currency        string          `json:"currency,omitempty"`
	Status          string          `json:"status,omitempty"`
	ProviderType    string          `json:"providerType,omitempty"`
	ProviderID      string          `json:"providerId,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
}

type PatchTransactionRequest struct {
	Status   *string         `json:"status,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
	Revision int64           `json:"revision,omitempty"`
}

func (h Handler) List(ctx context.Context, request ListTransactionsRequest) (ListTransactionsResponse, error) {
	items, total, err := h.Service.ListTransactions(ctx, apptransactions.ListTransactionsQuery{
		Page: request.Page, PageSize: request.PageSize, OrganizationID: request.OrganizationID,
		Keyword: request.Keyword, UserID: request.UserID, OrderID: request.OrderID,
		Status: request.Status, TransactionType: request.TransactionType,
	})
	if err != nil {
		return ListTransactionsResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*TransactionResponse, error) {
	result, err := h.Service.GetTransaction(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	typed := TransactionResponse(*result)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateTransactionRequest) error {
	return h.Service.CreateTransaction(ctx, apptransactions.CreateTransactionCommand{
		OrganizationID: request.OrganizationID, UserID: request.UserID, OrderID: request.OrderID,
		PaymentID: request.PaymentID, TransactionType: request.TransactionType,
		Amount: request.Amount, Currency: request.Currency, Status: request.Status,
		ProviderType: request.ProviderType, ProviderID: request.ProviderID, Metadata: request.Metadata,
	})
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchTransactionRequest) error {
	return h.Service.PatchTransaction(ctx, organizationID, apptransactions.PatchTransactionCommand{
		ID: id, Status: request.Status, Metadata: request.Metadata, Revision: request.Revision,
	})
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteTransaction(ctx, organizationID, id)
}
