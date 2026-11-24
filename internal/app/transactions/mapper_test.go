package transactions

import (
	"encoding/json"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	"github.com/stretchr/testify/assert"
)

func TestToListItem_Transaction(t *testing.T) {
	now := time.Now()
	tx := &domain.Transaction{
		ID:              "tx1",
		OrganizationID:  "org1",
		UserID:          "user1",
		OrderID:         "order1",
		TransactionType: "payment",
		Amount:          99.99,
		Currency:        "USD",
		Status:          "completed",
		ProviderType:    "stripe",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	item := toListItem(tx)
	assert.Equal(t, "tx1", item.ID)
	assert.Equal(t, "org1", item.OrganizationID)
	assert.Equal(t, "user1", item.UserID)
	assert.Equal(t, "order1", item.OrderID)
	assert.Equal(t, "payment", item.TransactionType)
	assert.Equal(t, 99.99, item.Amount)
	assert.Equal(t, "USD", item.Currency)
	assert.Equal(t, "completed", item.Status)
	assert.Equal(t, "stripe", item.ProviderType)
}

func TestToDetail_Transaction(t *testing.T) {
	now := time.Now()
	meta := json.RawMessage(`{"key":"value"}`)
	tx := &domain.Transaction{
		ID:              "tx1",
		OrganizationID:  "org1",
		UserID:          "user1",
		OrderID:         "order1",
		TransactionType: "payment",
		Amount:          50.0,
		Currency:        "EUR",
		Status:          "pending",
		ProviderType:    "alipay",
		PaymentID:       "pay1",
		ProviderID:      "prov1",
		Metadata:        meta,
		Revision:        2,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	detail := toDetail(tx)
	assert.Equal(t, "tx1", detail.ID)
	assert.Equal(t, "pay1", detail.PaymentID)
	assert.Equal(t, "prov1", detail.ProviderID)
	assert.Equal(t, int64(2), detail.Revision)
	assert.NotNil(t, detail.Metadata)
}

func TestToDetail_Transaction_NilMetadata(t *testing.T) {
	tx := &domain.Transaction{
		ID:       "tx1",
		Metadata: nil,
	}
	detail := toDetail(tx)
	assert.Nil(t, detail.Metadata)
}
