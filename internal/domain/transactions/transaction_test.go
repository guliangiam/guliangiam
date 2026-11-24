package transactions

import (
	"encoding/json"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestTransaction_Creation(t *testing.T) {
	now := time.Now()
	tx := Transaction{
		ID:              "tx-001",
		OrganizationID:  "org-001",
		UserID:          "user-001",
		OrderID:         "order-001",
		PaymentID:       "pay-001",
		TransactionType: "payment",
		Amount:          99.99,
		Currency:        "USD",
		Status:          "completed",
		ProviderType:    "stripe",
		ProviderID:      "stripe-account-001",
		Metadata:        json.RawMessage(`{"stripeChargeId":"ch_123"}`),
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}

	assert.Equal(t, "tx-001", tx.ID)
	assert.Equal(t, "payment", tx.TransactionType)
	assert.Equal(t, 99.99, tx.Amount)
	assert.Equal(t, "completed", tx.Status)
}

func TestTransaction_Types(t *testing.T) {
	validTypes := map[string]bool{
		"payment": true,
		"refund":  true,
		"topup":   true,
	}

	for txType, valid := range validTypes {
		t.Run("type_"+txType, func(t *testing.T) {
			tx := Transaction{TransactionType: txType}
			_, ok := validTypes[tx.TransactionType]
			assert.Equal(t, valid, ok)
		})
	}
}

func TestTransaction_StatusTransitions(t *testing.T) {
	validStatuses := map[string]bool{
		"pending":   true,
		"completed": true,
		"failed":    true,
	}

	for status, valid := range validStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			tx := Transaction{Status: status}
			_, ok := validStatuses[tx.Status]
			assert.Equal(t, valid, ok)
		})
	}
}

func TestTransaction_Metadata(t *testing.T) {
	tx := Transaction{
		Metadata: json.RawMessage(`{"key":"value"}`),
	}
	assert.NotNil(t, tx.Metadata)

	var meta map[string]string
	err := json.Unmarshal(tx.Metadata, &meta)
	assert.NoError(t, err)
	assert.Equal(t, "value", meta["key"])
}

func TestTransaction_TransitionAnswers422(t *testing.T) {
	txn := &Transaction{Status: StatusCompleted}
	appErr := apperrors.FromError(txn.TransitionTo(StatusPending))
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "status", appErr.Meta["field"])
}

func TestTransaction_ValidateAnswers422(t *testing.T) {
	tx := &Transaction{TransactionType: "weird", Status: StatusCompleted}
	appErr := apperrors.FromError(tx.Validate())
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "type", appErr.Meta["field"])

	bad := &Transaction{TransactionType: TypePayment, Status: "unknown"}
	appErr = apperrors.FromError(bad.Validate())
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "status", appErr.Meta["field"])
}
