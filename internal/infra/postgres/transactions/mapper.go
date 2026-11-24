package transactions

import (
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
)

// scanTransaction scans a full transaction row into a domain Transaction.
func scanTransaction(sc interface{ Scan(...interface{}) error }) (*domain.Transaction, error) {
	var t domain.Transaction
	var metadata []byte
	var userID, orderID, paymentID, providerID *string
	err := sc.Scan(&t.ID, &t.OrganizationID, &userID, &orderID, &paymentID, &t.TransactionType, &t.Amount, &t.Currency, &t.Status, &t.ProviderType, &providerID, &metadata, &t.CreatedAt, &t.UpdatedAt, &t.Revision)
	if err != nil {
		return nil, err
	}
	if userID != nil {
		t.UserID = *userID
	}
	if orderID != nil {
		t.OrderID = *orderID
	}
	if paymentID != nil {
		t.PaymentID = *paymentID
	}
	if providerID != nil {
		t.ProviderID = *providerID
	}
	if len(metadata) > 0 && string(metadata) != "null" {
		t.Metadata = json.RawMessage(metadata)
	}
	return &t, nil
}

// nilIfEmpty returns nil for empty strings, otherwise a pointer to the string.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
