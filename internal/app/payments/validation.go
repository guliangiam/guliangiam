package payments

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 状态机非法迁移使用 payment_invalid_transition（400，见 design/commerce/payment.md §12.1
// 与 errors.yaml）；元数据给出字段、当前值与合法边，控制台无需解析文案即可解释拒绝原因。

var chargeableStates = []string{domain.StateCreated}

var refundableStates = []string{domain.StateSucceeded, domain.StatePartiallyRefunded}

func validateChargeState(payment *domain.Payment) error {
	for _, state := range chargeableStates {
		if payment.State == state {
			return nil
		}
	}
	return apperrors.NewWithMeta(
		apperrors.CodePaymentInvalidTransition,
		"payment "+payment.ID+" is in state "+payment.State+", cannot charge",
		map[string]any{
			"field":   "state",
			"value":   payment.State,
			"allowed": append([]string{}, chargeableStates...),
		},
	)
}

func validateRefundState(paymentID, state string) error {
	for _, allowed := range refundableStates {
		if state == allowed {
			return nil
		}
	}
	return apperrors.NewWithMeta(
		apperrors.CodePaymentInvalidTransition,
		"payment "+paymentID+" in state "+state+", cannot refund",
		map[string]any{
			"field":   "state",
			"value":   state,
			"allowed": append([]string{}, refundableStates...),
		},
	)
}

func validateRefundAmount(amount float64) error {
	if amount > 0 {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"refund amount must be positive",
		map[string]any{"field": "amount", "value": amount, "min": 0},
	)
}

// refundTolerance absorbs binary rounding between the DECIMAL amounts stored in
// the ledger and the float64 amounts carried by the API; anything larger is a
// real request for more money than the payment still holds.
const refundTolerance = 0.0001

func validateRefundAvailable(amount, available float64) error {
	if amount <= available+refundTolerance {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodePaymentRefundExceedsBalance,
		"refund amount exceeds available balance",
		map[string]any{"field": "amount", "value": amount, "max": available},
	)
}

func validatePaymentTransition(payment *domain.Payment, target string) error {
	if payment.CanTransitionTo(target) {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodePaymentInvalidTransition,
		"cannot advance payment "+payment.ID+" from "+payment.State+" to "+target,
		map[string]any{
			"field":      "state",
			"value":      payment.State,
			"transition": target,
			"allowed":    payment.AllowedTransitions(),
		},
	)
}
