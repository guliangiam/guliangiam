package auth

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// The SMS verify path used to build `countryCode + phone` verbatim, turning
// `profile.CountryCode = "CN"` into the recipient `CN13800138000`. It must
// reject ISO alpha-2 before touching the repository.
func TestVerifySmsCodeWithRepoRejectsISOAsDialCode(t *testing.T) {
	err := verifySmsCodeWithRepo(context.Background(), nil, "13800138000", "CN", "123456")
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument || appErr.Meta["field"] != "countryCode" {
		t.Fatalf("code/meta = %v/%v, want invalid_argument countryCode", appErr.Code, appErr.Meta)
	}
}

func TestVerifySmsCodeWithRepoRejectsMissingPhone(t *testing.T) {
	err := verifySmsCodeWithRepo(context.Background(), nil, "", "86", "123456")
	if err == nil {
		t.Fatal("expected error")
	}
}
