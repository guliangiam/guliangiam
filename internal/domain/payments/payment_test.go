package payments

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestPayment_Creation(t *testing.T) {
	now := time.Now()
	p := Payment{
		ID:             "pay-001",
		OrganizationID: "org-001",
		Name:           "test-payment",
		DisplayName:    "Test Payment",
		Provider:       "stripe",
		Type:           "payment",
		Currency:       "USD",
		Price:          29.99,
		State:          StateCreated,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	assert.Equal(t, "pay-001", p.ID)
	assert.Equal(t, "stripe", p.Provider)
	assert.Equal(t, "USD", p.Currency)
	assert.Equal(t, 29.99, p.Price)
	assert.Equal(t, StateCreated, p.State)
}

func TestPayment_Validate(t *testing.T) {
	t.Run("valid_payment", func(t *testing.T) {
		p := Payment{ID: "pay-001", OrganizationID: "org-001", State: StateCreated, Price: 99.99}
		assert.NoError(t, p.Validate())
	})

	t.Run("missing_id", func(t *testing.T) {
		p := Payment{OrganizationID: "org-001", State: StateCreated}
		assert.Error(t, p.Validate())
	})

	t.Run("missing_organizationId", func(t *testing.T) {
		p := Payment{ID: "pay-001", State: StateCreated}
		assert.Error(t, p.Validate())
	})

	t.Run("invalid_state", func(t *testing.T) {
		p := Payment{ID: "pay-001", OrganizationID: "org-001", State: "unknown"}
		assert.Error(t, p.Validate())
	})

	t.Run("negative_price", func(t *testing.T) {
		p := Payment{ID: "pay-001", OrganizationID: "org-001", State: StateCreated, Price: -1.0}
		assert.Error(t, p.Validate())
	})

	t.Run("zero_price_is_valid", func(t *testing.T) {
		p := Payment{ID: "pay-001", OrganizationID: "org-001", State: StateCreated, Price: 0}
		assert.NoError(t, p.Validate())
	})

	t.Run("all_valid_states", func(t *testing.T) {
		states := []string{StateCreated, StateProcessing, StateSucceeded, StateFailed, StateCancelled, StateRefunded}
		for _, state := range states {
			p := Payment{ID: "pay-001", OrganizationID: "org-001", State: state}
			assert.NoError(t, p.Validate(), "state %s should be valid", state)
		}
	})
}

func TestPayment_IsSettled(t *testing.T) {
	p := Payment{State: StateSucceeded}
	assert.True(t, p.IsSettled())

	p.State = StateCreated
	assert.False(t, p.IsSettled())
}

func TestPayment_CanTransitionTo(t *testing.T) {
	// §1.2: only a collected payment can be reversed.
	p := Payment{State: StateCreated}
	assert.True(t, p.CanTransitionTo(StateSucceeded))
	assert.False(t, p.CanTransitionTo(StateRefunded))

	p.State = StateSucceeded
	assert.True(t, p.CanTransitionTo(StateRefunded))

	p.State = StateCancelled
	assert.False(t, p.CanTransitionTo(StateSucceeded))
}

func TestPayment_Validate_RejectsUnknownStateWith422(t *testing.T) {
	p := Payment{ID: "pay-1", OrganizationID: "org-1", State: "unknown"}
	err := p.Validate()
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity || appErr.HTTPStatus() != 422 {
		t.Fatalf("expected 422 unprocessable_entity, got %v", err)
	}
	if appErr.Meta["field"] != "state" || appErr.Meta["value"] != "unknown" {
		t.Fatalf("expected field/value metadata, got %v", appErr.Meta)
	}
}

func TestPayment_Validate_RejectsNegativePriceWith422(t *testing.T) {
	p := Payment{ID: "pay-1", OrganizationID: "org-1", Price: -1}
	err := p.Validate()
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected 422 unprocessable_entity, got %v", err)
	}
	if appErr.Meta["field"] != "price" || appErr.Meta["min"] != 0 {
		t.Fatalf("expected price/min metadata, got %v", appErr.Meta)
	}
}

func TestPayment_TransitionTo_RejectsIllegalEdgeWith422(t *testing.T) {
	p := Payment{State: StateSucceeded}
	err := p.TransitionTo(StateFailed)
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected 422 unprocessable_entity, got %v", err)
	}
	if appErr.Meta["field"] != "state" || appErr.Meta["value"] != StateSucceeded ||
		appErr.Meta["transition"] != StateFailed {
		t.Fatalf("expected state/transition metadata, got %v", appErr.Meta)
	}
	allowed, ok := appErr.Meta["allowed"].([]string)
	if !ok || len(allowed) != 2 || allowed[0] != StateRefunded || allowed[1] != StatePartiallyRefunded {
		t.Fatalf("expected allowed=[refunded partially_refunded], got %v", appErr.Meta["allowed"])
	}
}

func TestPayment_TransitionTo_RejectsUnknownStateWith422(t *testing.T) {
	p := Payment{State: StateCreated}
	err := p.TransitionTo("unknown")
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected 422 unprocessable_entity, got %v", err)
	}
	if appErr.Meta["set"] != "paymentsState" {
		t.Fatalf("expected paymentsState metadata, got %v", appErr.Meta)
	}
}
