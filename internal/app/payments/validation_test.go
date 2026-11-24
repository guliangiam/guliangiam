package payments

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wantPaymentErrorCode(t *testing.T, err error, code apperrors.Code, status int) *apperrors.Error {
	t.Helper()
	require.Error(t, err)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok, "guard must return *apperrors.Error, got %T", err)
	assert.Equal(t, code, appErr.Code)
	assert.Equal(t, status, appErr.HTTPStatus())
	return appErr
}

func wantPaymentError(t *testing.T, err error) *apperrors.Error {
	return wantPaymentErrorCode(t, err, apperrors.CodeUnprocessableEntity, 422)
}

func wantInvalidTransition(t *testing.T, err error) *apperrors.Error {
	return wantPaymentErrorCode(t, err, apperrors.CodePaymentInvalidTransition, 400)
}

func TestValidateChargeState(t *testing.T) {
	payment := &domain.Payment{ID: "pay-1", State: domain.StateCreated}
	require.NoError(t, validateChargeState(payment))

	for _, state := range []string{
		domain.StateProcessing, domain.StateSucceeded, domain.StateFailed,
		domain.StateCancelled, domain.StateRefunded, domain.StatePartiallyRefunded,
	} {
		payment.State = state
		appErr := wantInvalidTransition(t, validateChargeState(payment))
		assert.Equal(t, map[string]any{
			"field":   "state",
			"value":   state,
			"allowed": []string{domain.StateCreated},
		}, appErr.Meta)
	}
}

func TestValidateRefundState(t *testing.T) {
	require.NoError(t, validateRefundState("pay-1", domain.StateSucceeded))
	require.NoError(t, validateRefundState("pay-1", domain.StatePartiallyRefunded))

	for _, state := range []string{
		domain.StateCreated, domain.StateProcessing, domain.StateFailed,
		domain.StateCancelled, domain.StateRefunded,
	} {
		appErr := wantInvalidTransition(t, validateRefundState("pay-1", state))
		assert.Equal(t, map[string]any{
			"field":   "state",
			"value":   state,
			"allowed": []string{domain.StateSucceeded, domain.StatePartiallyRefunded},
		}, appErr.Meta)
	}
}

func TestValidateRefundAmount(t *testing.T) {
	require.NoError(t, validateRefundAmount(0.01))

	for _, amount := range []float64{0, -1, -0.01} {
		appErr := wantPaymentError(t, validateRefundAmount(amount))
		assert.Equal(t, "amount", appErr.Meta["field"])
		assert.Equal(t, amount, appErr.Meta["value"])
		assert.Equal(t, 0, appErr.Meta["min"])
	}
}

func TestValidateRefundAvailable(t *testing.T) {
	require.NoError(t, validateRefundAvailable(10, 10))
	// The float tolerance keeps an honest request at the ceiling passing.
	require.NoError(t, validateRefundAvailable(10.00005, 10))

	err := validateRefundAvailable(10.01, 10)
	require.Error(t, err)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok, "guard must return *apperrors.Error, got %T", err)
	assert.Equal(t, apperrors.CodePaymentRefundExceedsBalance, appErr.Code)
	assert.Equal(t, 422, appErr.HTTPStatus())
	assert.Equal(t, "amount", appErr.Meta["field"])
	assert.Equal(t, 10.01, appErr.Meta["value"])
	assert.Equal(t, 10.0, appErr.Meta["max"])
}

func TestValidatePaymentTransition(t *testing.T) {
	payment := &domain.Payment{ID: "pay-1", State: domain.StateProcessing}
	require.NoError(t, validatePaymentTransition(payment, domain.StateSucceeded))

	// processing → created is not a declared edge.
	appErr := wantInvalidTransition(t, validatePaymentTransition(payment, domain.StateCreated))
	assert.Equal(t, map[string]any{
		"field":      "state",
		"value":      domain.StateProcessing,
		"transition": domain.StateCreated,
		"allowed":    payment.AllowedTransitions(),
	}, appErr.Meta)
	assert.Equal(t, []string{
		domain.StateSucceeded, domain.StateFailed, domain.StateCancelled,
		domain.StateRefunded, domain.StatePartiallyRefunded,
	}, appErr.Meta["allowed"])
}
