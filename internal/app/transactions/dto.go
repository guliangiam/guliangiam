package transactions

import (
	"encoding/json"
	"time"
)

type ListTransactionsQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	UserID, OrderID         string
	Status, TransactionType *string
}

type CreateTransactionCommand struct {
	OrganizationID  string
	UserID          string
	OrderID         string
	PaymentID       string
	TransactionType string
	Amount          float64
	Currency        string
	Status          string
	ProviderType    string
	ProviderID      string
	Metadata        json.RawMessage
}

type PatchTransactionCommand struct {
	ID       string
	Status   *string
	Metadata json.RawMessage
	Revision int64
}

type TransactionListItem struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organizationId"`
	UserID          string    `json:"userId,omitempty"`
	OrderID         string    `json:"orderId,omitempty"`
	TransactionType string    `json:"transactionType"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency,omitempty"`
	Status          string    `json:"status"`
	ProviderType    string    `json:"providerType,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type TransactionDetail struct {
	TransactionListItem
	PaymentID  string          `json:"paymentId,omitempty"`
	ProviderID string          `json:"providerId,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	Revision   int64           `json:"revision"`
}
