package aigateway_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

func TestLlmApiKey_GenerateAndVerify(t *testing.T) {
	k := &aigateway.LlmApiKey{
		ID: "k-1", OrganizationID: "o-1", Name: "test-key",
		Status: aigateway.APIKeyStatusActive,
	}
	plain, err := k.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if plain == "" {
		t.Fatal("plaintext must not be empty")
	}
	if len(k.KeyPrefix) < 10 {
		t.Errorf("KeyPrefix too short: %s", k.KeyPrefix)
	}
	if k.KeyHash == "" {
		t.Error("KeyHash must be populated")
	}
	// Verify round-trip
	if !k.VerifyKey(plain) {
		t.Error("VerifyKey should return true for original plaintext")
	}
	if k.VerifyKey(plain + "tampered") {
		t.Error("VerifyKey should return false for tampered plaintext")
	}
}

func TestLlmApiKey_VerifyKeyMatchesSHA256(t *testing.T) {
	k := &aigateway.LlmApiKey{
		ID: "k-1", OrganizationID: "o-1", Name: "k",
		Status: aigateway.APIKeyStatusActive,
	}
	plain, _ := k.GenerateKey()
	sum := sha256.Sum256([]byte(plain))
	expected := hex.EncodeToString(sum[:])
	if k.KeyHash != expected {
		t.Errorf("KeyHash = %s, want sha256(plaintext) = %s", k.KeyHash, expected)
	}
}

func TestLlmApiKey_Revoke(t *testing.T) {
	k := &aigateway.LlmApiKey{
		ID: "k-1", OrganizationID: "o-1", Name: "k",
		Status: aigateway.APIKeyStatusActive,
	}
	if err := k.Revoke(); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if k.Status != aigateway.APIKeyStatusRevoked {
		t.Errorf("status = %s, want revoked", k.Status)
	}
	// Double-revoke is no-op
	if err := k.Revoke(); err != nil {
		t.Errorf("double Revoke should be nil, got %v", err)
	}
}

func TestLlmApiKey_RevokeExpiredFails(t *testing.T) {
	k := &aigateway.LlmApiKey{
		ID: "k-1", OrganizationID: "o-1", Name: "k",
		Status: aigateway.APIKeyStatusExpired,
	}
	if err := k.Revoke(); err == nil {
		t.Error("revoking an expired key should fail")
	}
}

func TestLlmApiKey_IsActive(t *testing.T) {
	k := &aigateway.LlmApiKey{
		Status: aigateway.APIKeyStatusActive,
	}
	if !k.IsActive(time.Now()) {
		t.Error("active key should be usable")
	}
	k.Status = aigateway.APIKeyStatusRevoked
	if k.IsActive(time.Now()) {
		t.Error("revoked key should NOT be usable")
	}
	k.Status = aigateway.APIKeyStatusActive
	expired := time.Now().Add(-time.Hour)
	k.ExpiresAt = &expired
	if k.IsActive(time.Now()) {
		t.Error("expired key should NOT be usable")
	}
}

func TestLlmApiKey_Validate(t *testing.T) {
	k := &aigateway.LlmApiKey{
		ID: "k-1", OrganizationID: "o-1", Name: "k",
		Status: aigateway.APIKeyStatusActive, Currency: "USD",
	}
	if err := k.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := &aigateway.LlmApiKey{ID: "", OrganizationID: "", Name: "k", Status: "active"}
	if err := bad.Validate(); err == nil {
		t.Error("missing fields should fail Validate")
	}
	bad2 := &aigateway.LlmApiKey{ID: "1", OrganizationID: "2", Name: "3", Status: "weird"}
	if err := bad2.Validate(); err == nil {
		t.Error("invalid status should fail Validate")
	}
}
