package orders

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// isolateSharedAdapters snapshots and restores the package-level adapter
// holder so tests never leak wiring into each other (BE-C-21).
func isolateSharedAdapters(t *testing.T) {
	t.Helper()
	previous := sharedPaymentAdapters.Load()
	sharedPaymentAdapters.Store(nil)
	t.Cleanup(func() { sharedPaymentAdapters.Store(previous) })
}

// TestService_PaymentAdapters_FieldInjectionWins pins BE-C-21: per-instance
// injection takes precedence over the bootstrap-level shared wiring, so
// tests and multi-gateway assemblies never mutate package state.
func TestService_PaymentAdapters_FieldInjectionWins(t *testing.T) {
	isolateSharedAdapters(t)
	SetPaymentAdapters(
		func(context.Context, string, float64, string, string, string, string) (string, string, error) {
			return "shared-url", "shared-trade", nil
		},
		func(context.Context, string) (payments.Outcome, error) {
			return payments.OutcomeFailed, nil
		},
	)
	svc := Service{
		CreatePayment: func(context.Context, string, float64, string, string, string, string) (string, string, error) {
			return "injected-url", "injected-trade", nil
		},
		QueryPayment: func(context.Context, string) (payments.Outcome, error) {
			return payments.OutcomeSucceeded, nil
		},
	}

	url, trade, err := svc.createPayment(context.Background(), "o-1", 10, "USD", "subject", "", "")
	if err != nil || url != "injected-url" || trade != "injected-trade" {
		t.Fatalf("createPayment = (%q, %q, %v), want the injected adapter", url, trade, err)
	}
	outcome, err := svc.queryPayment(context.Background(), "trade-1")
	if err != nil || outcome != payments.OutcomeSucceeded {
		t.Fatalf("queryPayment = (%v, %v), want the injected adapter", outcome, err)
	}
}

// TestService_PaymentAdapters_SharedFallback: without field injection the
// bootstrap wiring via SetPaymentAdapters still applies.
func TestService_PaymentAdapters_SharedFallback(t *testing.T) {
	isolateSharedAdapters(t)
	SetPaymentAdapters(
		func(context.Context, string, float64, string, string, string, string) (string, string, error) {
			return "shared-url", "shared-trade", nil
		},
		func(context.Context, string) (payments.Outcome, error) {
			return payments.OutcomeSucceeded, nil
		},
	)
	svc := Service{}
	url, trade, err := svc.createPayment(context.Background(), "o-1", 10, "USD", "subject", "", "")
	if err != nil || url != "shared-url" || trade != "shared-trade" {
		t.Fatalf("createPayment = (%q, %q, %v), want the shared adapter", url, trade, err)
	}
	outcome, err := svc.queryPayment(context.Background(), "trade-1")
	if err != nil || outcome != payments.OutcomeSucceeded {
		t.Fatalf("queryPayment = (%v, %v), want the shared adapter", outcome, err)
	}
}

// TestService_PaymentAdapters_FailClosedByDefault: an unwired service must
// refuse to invent a trade instead of silently succeeding.
func TestService_PaymentAdapters_FailClosedByDefault(t *testing.T) {
	isolateSharedAdapters(t)
	svc := Service{}
	_, _, err := svc.createPayment(context.Background(), "o-1", 10, "USD", "subject", "", "")
	if err == nil {
		t.Fatal("createPayment() error = nil, want fail-closed default")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnavailable {
		t.Fatalf("error code = %s, want %s", code, apperrors.CodeUnavailable)
	}
	_, err = svc.queryPayment(context.Background(), "trade-1")
	if err == nil {
		t.Fatal("queryPayment() error = nil, want fail-closed default")
	}
}
