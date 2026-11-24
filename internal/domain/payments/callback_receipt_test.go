package payments_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/payments"
)

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestCallbackReceipt_Validate(t *testing.T) {
	tests := []struct {
		name    string
		r       payments.CallbackReceipt
		wantErr bool
	}{
		{
			name: "valid received",
			r: payments.CallbackReceipt{
				ID: "c-1", OrganizationID: "o-1", Channel: "alipay",
				PayloadSHA256: sha256hex("hello"),
				Payload:       []byte("hello"), ReceivedAt: time.Now(),
				ProcessState: payments.ReceiptStateReceived,
			},
			wantErr: false,
		},
		{
			name: "missing payload",
			r: payments.CallbackReceipt{
				ID: "c-1", OrganizationID: "o-1", Channel: "alipay",
				PayloadSHA256: "abc", ReceivedAt: time.Now(),
				ProcessState: payments.ReceiptStateReceived,
			},
			wantErr: true,
		},
		{
			name: "invalid process state",
			r: payments.CallbackReceipt{
				ID: "c-1", OrganizationID: "o-1", Channel: "alipay",
				PayloadSHA256: "abc", Payload: []byte("x"),
				ReceivedAt: time.Now(), ProcessState: "murky",
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

func TestCallbackReceipt_StateMachine(t *testing.T) {
	r := &payments.CallbackReceipt{ProcessState: payments.ReceiptStateReceived}
	// 验签与归一化在 ParseCallback 内同一步完成：received → normalized 直达。
	stages := []string{
		payments.ReceiptStateNormalized,
		payments.ReceiptStateAccepted,
	}
	for _, next := range stages {
		if err := r.TransitionTo(next); err != nil {
			t.Fatalf("%s → %s: %v", r.ProcessState, next, err)
		}
	}
	// Accepted is terminal
	if err := r.TransitionTo(payments.ReceiptStateVerified); err == nil {
		t.Error("accepted → verified should fail")
	}
}

func TestCallbackReceipt_RejectedBranch(t *testing.T) {
	r := &payments.CallbackReceipt{ProcessState: payments.ReceiptStateReceived}
	if err := r.TransitionTo(payments.ReceiptStateRejected); err != nil {
		t.Fatalf("received → rejected: %v", err)
	}
	if err := r.TransitionTo(payments.ReceiptStateVerified); err == nil {
		t.Error("rejected → verified should fail (terminal)")
	}
}

func TestCallbackReceipt_ReceivedCannotSkipNormalization(t *testing.T) {
	r := &payments.CallbackReceipt{ProcessState: payments.ReceiptStateReceived}
	if err := r.TransitionTo(payments.ReceiptStateAccepted); err == nil {
		t.Error("received → accepted should fail (must pass normalized first)")
	}
	if err := r.TransitionTo(payments.ReceiptStateDuplicated); err == nil {
		t.Error("received → duplicated should fail (must pass normalized first)")
	}
}

func TestCallbackReceipt_DuplicatedBranch(t *testing.T) {
	r := &payments.CallbackReceipt{ProcessState: payments.ReceiptStateNormalized}
	if err := r.TransitionTo(payments.ReceiptStateDuplicated); err != nil {
		t.Fatalf("normalized → duplicated: %v", err)
	}
	if err := r.TransitionTo(payments.ReceiptStateAccepted); err == nil {
		t.Error("duplicated → accepted should fail (terminal)")
	}
}

func TestCallbackReceipt_LegacyVerifiedState(t *testing.T) {
	// verified 为历史兼容态：现行管线不产生，但存量行仍可前进或拒绝。
	r := &payments.CallbackReceipt{ProcessState: payments.ReceiptStateVerified}
	if err := r.TransitionTo(payments.ReceiptStateNormalized); err != nil {
		t.Fatalf("verified → normalized: %v", err)
	}
	v := &payments.CallbackReceipt{ProcessState: payments.ReceiptStateVerified}
	if err := v.TransitionTo(payments.ReceiptStateRejected); err != nil {
		t.Fatalf("verified → rejected: %v", err)
	}
}
