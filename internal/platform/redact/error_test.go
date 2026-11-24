package redact

import (
	"errors"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestErrorCategory_Nil(t *testing.T) {
	if got := ErrorCategory(nil); got != "" {
		t.Fatalf("ErrorCategory(nil) = %q, want empty", got)
	}
}

func TestErrorCategory_PlatformCode(t *testing.T) {
	err := apperrors.Wrap(
		apperrors.CodeConflict,
		errors.New(`duplicate key value violates unique constraint "uq_users_org_username" (email)=(a@b.c)`),
		"resource conflicts",
	)
	if got := ErrorCategory(err); got != string(apperrors.CodeConflict) {
		t.Fatalf("ErrorCategory() = %q, want %q", got, apperrors.CodeConflict)
	}
}

func TestErrorCategory_NeverEchoesMessage(t *testing.T) {
	const secret = "duplicate key value ... (email)=(a@b.c)"
	got := ErrorCategory(errors.New(secret))
	if strings.Contains(got, secret) || strings.Contains(got, "a@b.c") {
		t.Fatalf("ErrorCategory leaked the message: %q", got)
	}
	if got == "" {
		t.Fatal("ErrorCategory() returned empty for a non-nil error")
	}
}
