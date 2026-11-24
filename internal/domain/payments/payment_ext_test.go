package payments_test

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/payments"
)

func TestPayment_PartiallyRefundedTransition(t *testing.T) {
	p := &payments.Payment{
		ID: "pay-1", OrganizationID: "o-1", Price: 100.0,
		State: payments.StateSucceeded,
	}
	// succeeded → partially_refunded
	if err := p.TransitionTo(payments.StatePartiallyRefunded); err != nil {
		t.Fatalf("succeeded → partially_refunded: %v", err)
	}
	if p.State != payments.StatePartiallyRefunded {
		t.Errorf("state = %s, want partially_refunded", p.State)
	}
	// partially_refunded → refunded (all money back)
	if err := p.TransitionTo(payments.StateRefunded); err != nil {
		t.Fatalf("partially_refunded → refunded: %v", err)
	}
	if p.State != payments.StateRefunded {
		t.Errorf("state = %s, want refunded", p.State)
	}
}

func TestPayment_CannotRefundFromCreated(t *testing.T) {
	p := &payments.Payment{State: payments.StateCreated}
	if err := p.TransitionTo(payments.StateRefunded); err == nil {
		t.Error("created → refunded should fail")
	}
}

func TestPayment_AvailableForRefund(t *testing.T) {
	p := &payments.Payment{
		Price: 100.0, RefundedAmount: 20.0, RefundingAmount: 10.0,
	}
	got := p.AvailableForRefund()
	want := 70.0
	if got != want {
		t.Errorf("AvailableForRefund = %v, want %v", got, want)
	}
}

func TestPayment_CanRefund(t *testing.T) {
	// CanRefund means "collected money exists that can be reversed".
	// processing is excluded here even though it can transition to refunded —
	// the refund orchestration layer gate-keeps on IsSettled(), not CanRefund().
	cases := []struct {
		state string
		want  bool
	}{
		{payments.StateSucceeded, true},
		{payments.StatePartiallyRefunded, true},
		{payments.StateCreated, false},
		{payments.StateProcessing, false}, // money not collected yet
		{payments.StateRefunded, false},
		{payments.StateFailed, false},
		{payments.StateCancelled, false},
	}
	for _, c := range cases {
		p := &payments.Payment{State: c.state}
		if got := p.CanRefund(); got != c.want {
			t.Errorf("state %s CanRefund = %v, want %v", c.state, got, c.want)
		}
	}
}

func TestPayment_IsSettledIncludesPartiallyRefunded(t *testing.T) {
	// IsSettled = money collected (no need to re-query channel).
	// refunded is NOT settled — money was fully reversed.
	cases := []struct {
		state string
		want  bool
	}{
		{payments.StateSucceeded, true},
		{payments.StatePartiallyRefunded, true},
		{payments.StateRefunded, false}, // all money reversed
		{payments.StateCreated, false},
		{payments.StateFailed, false},
	}
	for _, c := range cases {
		p := &payments.Payment{State: c.state}
		if got := p.IsSettled(); got != c.want {
			t.Errorf("state %s IsSettled = %v, want %v", c.state, got, c.want)
		}
	}
}
