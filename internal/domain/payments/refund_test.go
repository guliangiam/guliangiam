package payments_test

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRefund_Validate(t *testing.T) {
	tests := []struct {
		name    string
		r       payments.Refund
		wantErr bool
	}{
		{
			name: "valid",
			r: payments.Refund{
				ID: "r-1", OrganizationID: "o-1", PaymentID: "pay-1",
				OutRefundNo: "rf-1", Channel: "alipay", Amount: 50.0,
				Currency: "CNY",
			},
			wantErr: false,
		},
		{
			name: "missing id",
			r: payments.Refund{
				OrganizationID: "o-1", PaymentID: "p", OutRefundNo: "n",
				Channel: "alipay", Amount: 10,
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			r: payments.Refund{
				ID: "r-1", OrganizationID: "o-1", PaymentID: "p",
				OutRefundNo: "n", Channel: "alipay", Amount: 0,
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			r: payments.Refund{
				ID: "r-1", OrganizationID: "o-1", PaymentID: "p",
				OutRefundNo: "n", Channel: "alipay", Amount: -1,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.r.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRefund_StateTransitions(t *testing.T) {
	r := &payments.Refund{State: payments.RefundStateRequested}

	// requested → processing
	if err := r.TransitionTo(payments.RefundStateProcessing); err != nil {
		t.Fatalf("requested → processing: %v", err)
	}
	if r.State != payments.RefundStateProcessing {
		t.Error("should be processing")
	}
	// processing → succeeded
	if err := r.TransitionTo(payments.RefundStateSucceeded); err != nil {
		t.Fatalf("processing → succeeded: %v", err)
	}
	if r.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
	// terminal → anything should fail
	if err := r.TransitionTo(payments.RefundStateRequested); err == nil {
		t.Error("terminal state should not transition back")
	}
}

func TestRefund_CalculateNewRefundedAmount(t *testing.T) {
	r := &payments.Refund{Amount: 30.0}
	got := r.CalculateNewRefundedAmount(20.0, 5.0)
	want := 20.0 + 5.0 + 30.0
	if got != want {
		t.Errorf("new refunded = %v, want %v", got, want)
	}
}

func TestRefund_SelfTransition(t *testing.T) {
	r := &payments.Refund{State: payments.RefundStateRequested}
	if err := r.TransitionTo(payments.RefundStateRequested); err != nil {
		t.Error("self-transition should be no-op")
	}
}

func TestPaymentsSemanticRules_Answer422(t *testing.T) {
	refund := &payments.Refund{ID: "r-1", OrganizationID: "org-1", PaymentID: "pay-1", OutRefundNo: "r-2", Amount: 0}
	if code := apperrors.FromError(refund.Validate()).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("refund amount: code = %q, want 422", code)
	}

	receipt := &payments.CallbackReceipt{ID: "c-1", OrganizationID: "org-1", Channel: "wechat", PayloadSHA256: "hash"}
	if code := apperrors.FromError(receipt.Validate()).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("receipt payload: code = %q, want 422", code)
	}

	receiptTransition := &payments.CallbackReceipt{ProcessState: payments.ReceiptStateAccepted}
	if code := apperrors.FromError(receiptTransition.TransitionTo(payments.ReceiptStateReceived)).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("receipt transition: code = %q, want 422", code)
	}

	channel := &payments.PayChannelConfig{Status: "bogus"}
	if code := apperrors.FromError(channel.TransitionTo(payments.ChannelConfigStatusActive)).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("channel transition: code = %q, want 422", code)
	}

	refundTransition := &payments.Refund{State: payments.RefundStateSucceeded}
	if code := apperrors.FromError(refundTransition.TransitionTo(payments.RefundStateRequested)).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("refund transition: code = %q, want 422", code)
	}
}
