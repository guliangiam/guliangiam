package transactions

import (
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
)

func toListItem(t *domain.Transaction) TransactionListItem {
	return TransactionListItem{
		ID: t.ID, OrganizationID: t.OrganizationID, UserID: t.UserID,
		OrderID: t.OrderID, TransactionType: t.TransactionType,
		Amount: t.Amount, Currency: t.Currency, Status: t.Status,
		ProviderType: t.ProviderType, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func toDetail(t *domain.Transaction) TransactionDetail {
	var metadata json.RawMessage
	if t.Metadata != nil {
		metadata = make(json.RawMessage, len(t.Metadata))
		copy(metadata, t.Metadata)
	}
	return TransactionDetail{
		TransactionListItem: toListItem(t),
		PaymentID:           t.PaymentID, ProviderID: t.ProviderID,
		Metadata: metadata, Revision: t.Revision,
	}
}
