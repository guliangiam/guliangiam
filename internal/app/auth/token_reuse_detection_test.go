package auth

import (
	"testing"
)

func TestDetectTokenReuse_NotRevoked(t *testing.T) {
	if detectTokenReuse("token1", false) {
		t.Error("expected no reuse detection for non-revoked token")
	}
}

func TestDetectTokenReuse_Revoked(t *testing.T) {
	if !detectTokenReuse("token1", true) {
		t.Error("expected reuse detection for revoked token")
	}
}

func TestCheckTokenReuseAndRevoke_NoReuse(t *testing.T) {
	err := CheckTokenReuseAndRevoke("family1", false)
	if err != nil {
		t.Errorf("expected no error for non-revoked token, got %v", err)
	}
}

func TestCheckTokenReuseAndRevoke_ReuseDetected(t *testing.T) {
	// Save and restore revokeTokenFamily to avoid interference from WireTokenFamilyRevocation calls in other tests.
	origRevoke := revokeTokenFamily
	revokeTokenFamily = func(familyID string) error { return nil }
	defer func() { revokeTokenFamily = origRevoke }()

	err := CheckTokenReuseAndRevoke("family1", true)
	if err == nil {
		t.Error("expected error for reused token")
	}
}
