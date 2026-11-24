package auth

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestDetectTokenReuse_RevokedToken(t *testing.T) {
	if !detectTokenReuse("rt-1", true) {
		t.Error("expected reuse detection for revoked token")
	}
}

func TestDetectTokenReuse_ActiveToken(t *testing.T) {
	if detectTokenReuse("rt-1", false) {
		t.Error("expected no reuse detection for active token")
	}
}

func TestCheckTokenReuseAndRevoke_ActiveToken(t *testing.T) {
	err := CheckTokenReuseAndRevoke("family-1", false)
	if err != nil {
		t.Errorf("expected no error for active token, got: %v", err)
	}
}

func TestCheckTokenReuseAndRevoke_RevokedToken(t *testing.T) {
	// Save and restore revokeTokenFamily to avoid interference from WireTokenFamilyRevocation calls in other tests.
	origRevoke := revokeTokenFamily
	revokeTokenFamily = func(familyID string) error { return nil }
	defer func() { revokeTokenFamily = origRevoke }()

	err := CheckTokenReuseAndRevoke("family-1", true)
	if err == nil {
		t.Fatal("expected error for revoked token (reuse detected)")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnauthorized {
		t.Errorf("expected unauthorized, got %s", appErr.Code)
	}
}

func TestCheckTokenReuseAndRevoke_RevokedTokenEmptyFamily(t *testing.T) {
	// Save and restore revokeTokenFamily to avoid interference from WireTokenFamilyRevocation calls in other tests.
	origRevoke := revokeTokenFamily
	revokeTokenFamily = func(familyID string) error { return nil }
	defer func() { revokeTokenFamily = origRevoke }()

	err := CheckTokenReuseAndRevoke("", true)
	if err == nil {
		t.Fatal("expected error for revoked token even with empty familyID")
	}
}

func TestTokenFamily_Fields(t *testing.T) {
	f := tokenFamily{
		FamilyID:       "family-abc",
		AccessTokenID:  "at-123",
		RefreshTokenID: "rt-456",
	}
	if f.FamilyID != "family-abc" {
		t.Errorf("FamilyID = %q, want %q", f.FamilyID, "family-abc")
	}
	if f.AccessTokenID != "at-123" {
		t.Errorf("AccessTokenID = %q, want %q", f.AccessTokenID, "at-123")
	}
	if f.RefreshTokenID != "rt-456" {
		t.Errorf("RefreshTokenID = %q, want %q", f.RefreshTokenID, "rt-456")
	}
}
