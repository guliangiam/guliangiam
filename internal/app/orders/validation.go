package orders

import (
	"fmt"

	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// These guards answer 422: the basket or callback parsed and bound, but a
// business rule rejected it, and the caller succeeds by changing one field
// (手册 4.14.1 跨字段矛盾 / 状态机非法转移). The metadata names the field and
// the accepted value, so the console can point at the input.

var payableStates = []string{domain.StateCreated, domain.StatePaymentPending, domain.StateFailed}

func validateSellableProduct(product *productdomain.Product) error {
	if product.State == productdomain.StateActive {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"product "+product.ID+" is not available for purchase",
		map[string]any{
			"field":   "productId",
			"value":   product.ID,
			"allowed": []string{productdomain.StateActive},
		},
	)
}

func validateSingleCurrency(current, next string) error {
	if current == "" || current == next {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"all products in one order must share a currency",
		map[string]any{"field": "currency", "value": next, "expected": current},
	)
}

func validateClaimedCurrency(claimed, catalog string) error {
	if claimed == "" || catalog == "" || claimed == catalog {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		fmt.Sprintf("currency %s does not match the catalog currency %s", claimed, catalog),
		map[string]any{"field": "currency", "value": claimed, "expected": catalog},
	)
}

func validateClaimedPrice(claimed, total float64) error {
	if claimed == 0 {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		fmt.Sprintf("price %g does not match the catalog price %g", claimed, total),
		map[string]any{"field": "price", "value": claimed, "expected": total},
	)
}

func validatePayableState(state string) error {
	for _, allowed := range payableStates {
		if state == allowed {
			return nil
		}
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"order cannot be paid in current state: "+state,
		map[string]any{
			"field":   "state",
			"value":   state,
			"allowed": append([]string{}, payableStates...),
		},
	)
}

func validateCallbackAmount(actual, expected float64) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		fmt.Sprintf("payment callback amount %.2f does not match order amount %.2f", actual, expected),
		map[string]any{"field": "amount", "value": actual, "expected": expected},
	)
}

func validateCallbackCurrency(actual, expected string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		fmt.Sprintf("payment callback currency %s does not match order currency %s", actual, expected),
		map[string]any{"field": "currency", "value": actual, "expected": expected},
	)
}
