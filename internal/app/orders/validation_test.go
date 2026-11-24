package orders

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wantOrderError(t *testing.T, err error) *apperrors.Error {
	t.Helper()
	require.Error(t, err)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok, "guard must return *apperrors.Error, got %T", err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, 422, appErr.HTTPStatus())
	return appErr
}

func TestValidateSellableProduct(t *testing.T) {
	product := &productdomain.Product{ID: "prod-1", State: productdomain.StateActive}
	require.NoError(t, validateSellableProduct(product))

	for _, state := range []string{
		productdomain.StateDraft, productdomain.StateDiscontinued, "",
	} {
		product.State = state
		appErr := wantOrderError(t, validateSellableProduct(product))
		assert.Equal(t, map[string]any{
			"field":   "productId",
			"value":   "prod-1",
			"allowed": []string{productdomain.StateActive},
		}, appErr.Meta)
	}
}

func TestValidateSingleCurrency(t *testing.T) {
	require.NoError(t, validateSingleCurrency("", "CNY"))
	require.NoError(t, validateSingleCurrency("CNY", "CNY"))

	appErr := wantOrderError(t, validateSingleCurrency("CNY", "USD"))
	assert.Equal(t, map[string]any{
		"field":    "currency",
		"value":    "USD",
		"expected": "CNY",
	}, appErr.Meta)
}

func TestValidateClaimedCurrency(t *testing.T) {
	require.NoError(t, validateClaimedCurrency("", "CNY"))
	require.NoError(t, validateClaimedCurrency("CNY", ""))
	require.NoError(t, validateClaimedCurrency("CNY", "CNY"))

	appErr := wantOrderError(t, validateClaimedCurrency("USD", "CNY"))
	assert.Equal(t, map[string]any{
		"field":    "currency",
		"value":    "USD",
		"expected": "CNY",
	}, appErr.Meta)
}

func TestValidateClaimedPrice(t *testing.T) {
	require.NoError(t, validateClaimedPrice(0, 100))

	appErr := wantOrderError(t, validateClaimedPrice(99, 100))
	assert.Equal(t, map[string]any{
		"field":    "price",
		"value":    99.0,
		"expected": 100.0,
	}, appErr.Meta)
}

func TestValidatePayableState(t *testing.T) {
	for _, state := range []string{
		domain.StateCreated, domain.StatePaymentPending, domain.StateFailed,
	} {
		require.NoError(t, validatePayableState(state))
	}

	for _, state := range []string{domain.StatePaid, domain.StateCompleted, domain.StateCancelled} {
		appErr := wantOrderError(t, validatePayableState(state))
		assert.Equal(t, map[string]any{
			"field":   "state",
			"value":   state,
			"allowed": []string{domain.StateCreated, domain.StatePaymentPending, domain.StateFailed},
		}, appErr.Meta)
	}
}

func TestValidateCallbackAmount(t *testing.T) {
	appErr := wantOrderError(t, validateCallbackAmount(99, 100))
	assert.Equal(t, map[string]any{
		"field":    "amount",
		"value":    99.0,
		"expected": 100.0,
	}, appErr.Meta)
}

func TestValidateCallbackCurrency(t *testing.T) {
	appErr := wantOrderError(t, validateCallbackCurrency("usd", "CNY"))
	assert.Equal(t, map[string]any{
		"field":    "currency",
		"value":    "usd",
		"expected": "CNY",
	}, appErr.Meta)
}
