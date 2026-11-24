package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func TestSM3KnownVector(t *testing.T) {
	got := hex.EncodeToString(SM3Sum([]byte("abc")))
	if got != sm3ABCVector {
		t.Fatalf("SM3(abc) = %s, want %s", got, sm3ABCVector)
	}
}

func TestSM4GCMRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 16)
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("rand nonce: %v", err)
	}
	suite := NewCryptoSuite()

	sealed, err := suite.SM4GCMSeal(key, nonce, []byte("guliangiam-sm4"), []byte("aad"))
	if err != nil {
		t.Fatalf("SM4GCMSeal: %v", err)
	}
	opened, err := suite.SM4GCMOpen(key, nonce, sealed, []byte("aad"))
	if err != nil {
		t.Fatalf("SM4GCMOpen: %v", err)
	}
	if string(opened) != "guliangiam-sm4" {
		t.Fatalf("roundtrip = %q", opened)
	}
}

func TestSM4GCMRejectsWrongKeyLength(t *testing.T) {
	suite := NewCryptoSuite()
	if _, err := suite.SM4GCMSeal(make([]byte, 15), make([]byte, 12), []byte("x"), nil); err == nil {
		t.Fatal("expected error for a 15-byte SM4 key")
	}
}

func TestSM2SignVerifyRoundTrip(t *testing.T) {
	suite := NewCryptoSuite()
	privateKey, err := GenerateSM2Key()
	if err != nil {
		t.Fatalf("GenerateSM2Key: %v", err)
	}
	digest := SM3Sum([]byte("sm2-sign"))
	signature, err := suite.SM2Sign(privateKey, digest)
	if err != nil {
		t.Fatalf("SM2Sign: %v", err)
	}
	if !suite.SM2Verify(privateKey.Public(), digest, signature) {
		t.Fatal("SM2Verify rejected a valid signature")
	}
	if suite.SM2Verify(privateKey.Public(), SM3Sum([]byte("tampered")), signature) {
		t.Fatal("SM2Verify accepted a signature for a different digest")
	}
}

func TestSM2EncryptDecryptRoundTrip(t *testing.T) {
	suite := NewCryptoSuite()
	privateKey, err := GenerateSM2Key()
	if err != nil {
		t.Fatalf("GenerateSM2Key: %v", err)
	}
	ciphertext, err := suite.SM2Encrypt(privateKey.Public(), []byte("sm2-encrypt"))
	if err != nil {
		t.Fatalf("SM2Encrypt: %v", err)
	}
	plaintext, err := suite.SM2Decrypt(privateKey, ciphertext)
	if err != nil {
		t.Fatalf("SM2Decrypt: %v", err)
	}
	if string(plaintext) != "sm2-encrypt" {
		t.Fatalf("roundtrip = %q", plaintext)
	}
}

func TestDetectCapabilities(t *testing.T) {
	detected := DetectCapabilities()
	if len(detected) != len(registryCapabilityOrder) {
		t.Fatalf("detected %d capabilities, want %d", len(detected), len(registryCapabilityOrder))
	}
	for i, capability := range detected {
		if capability.ID != registryCapabilityOrder[i] {
			t.Fatalf("capability[%d] = %s, want %s", i, capability.ID, registryCapabilityOrder[i])
		}
		if capability.ID != CapabilityGMTLS && !capability.Available {
			t.Fatalf("%s self-test failed: %s", capability.ID, capability.Detail)
		}
	}
}

func capabilityStatuses(missing ...string) []CapabilityStatus {
	missingSet := make(map[string]bool, len(missing))
	for _, id := range missing {
		missingSet[id] = true
	}
	out := make([]CapabilityStatus, 0, len(registryCapabilityOrder))
	for _, id := range registryCapabilityOrder {
		out = append(out, CapabilityStatus{ID: id, Available: !missingSet[id]})
	}
	return out
}

func TestAssessProfile(t *testing.T) {
	tests := []struct {
		name          string
		profile       string
		capabilities  []CapabilityStatus
		wantErr       bool
		wantMissing   []string
		wantDegraded  []string
		wantAvailable bool
	}{
		{
			name:          "default profile ignores missing gm capabilities",
			profile:       "default",
			capabilities:  capabilityStatuses(CapabilitySM2Sign, CapabilitySM3),
			wantAvailable: true,
		},
		{
			name:          "gm profile with all capabilities",
			profile:       "gm",
			capabilities:  capabilityStatuses(),
			wantAvailable: true,
		},
		{
			name:          "gm profile refuses when sm3 is missing",
			profile:       "gm",
			capabilities:  capabilityStatuses(CapabilitySM3),
			wantErr:       true,
			wantMissing:   []string{CapabilitySM3},
			wantAvailable: false,
		},
		{
			name:          "gm profile refuses when any required capability is missing",
			profile:       "gm",
			capabilities:  capabilityStatuses(CapabilitySM2Sign, CapabilitySM2Encrypt, CapabilitySM3, CapabilitySM4),
			wantErr:       true,
			wantMissing:   []string{CapabilitySM2Sign, CapabilitySM2Encrypt, CapabilitySM3, CapabilitySM4},
			wantAvailable: false,
		},
		{
			name:          "gm_tls missing only degrades",
			profile:       "gm",
			capabilities:  capabilityStatuses(CapabilityGMTLS),
			wantAvailable: true,
			wantDegraded:  []string{CapabilityGMTLS},
		},
		{
			name:          "empty profile defaults to default",
			profile:       "",
			capabilities:  capabilityStatuses(),
			wantAvailable: true,
		},
		{
			name:          "unknown profile is rejected",
			profile:       "gmv2",
			capabilities:  capabilityStatuses(),
			wantErr:       true,
			wantAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := AssessProfile(tt.profile, tt.capabilities)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if report.Available != tt.wantAvailable {
				t.Fatalf("Available = %v, want %v", report.Available, tt.wantAvailable)
			}
			if len(report.Missing) != len(tt.wantMissing) {
				t.Fatalf("Missing = %v, want %v", report.Missing, tt.wantMissing)
			}
			for i, id := range tt.wantMissing {
				if report.Missing[i] != id {
					t.Fatalf("Missing[%d] = %s, want %s", i, report.Missing[i], id)
				}
			}
			if len(report.Degraded) != len(tt.wantDegraded) {
				t.Fatalf("Degraded = %v, want %v", report.Degraded, tt.wantDegraded)
			}
		})
	}
}

func TestHealthReport(t *testing.T) {
	t.Cleanup(func() {
		SetRuntimeReport(ProfileReport{Profile: string(ProfileDefault), Available: true})
	})

	SetRuntimeReport(ProfileReport{Profile: string(ProfileGM), Available: true})
	if got := HealthReport(); got.Status != HealthStatusOK || got.Profile != string(ProfileGM) {
		t.Fatalf("health = %+v, want ok/gm", got)
	}

	SetRuntimeReport(ProfileReport{Profile: string(ProfileGM), Available: true, Degraded: []string{CapabilityGMTLS}})
	got := HealthReport()
	if got.Status != HealthStatusDegraded || len(got.Degraded) != 1 || got.Degraded[0] != CapabilityGMTLS {
		t.Fatalf("health = %+v, want degraded/gm_tls", got)
	}

	SetRuntimeReport(ProfileReport{Profile: string(ProfileGM), Available: false, Missing: []string{CapabilitySM3}})
	got = HealthReport()
	if got.Status != HealthStatusUnavailable || len(got.Missing) != 1 || got.Missing[0] != CapabilitySM3 {
		t.Fatalf("health = %+v, want unavailable/sm3", got)
	}
}

func TestValidateRuntimeProfileHonorsEnv(t *testing.T) {
	t.Setenv("CRYPTO_PROFILE", "gm")
	t.Setenv("GM_TLS_ENABLED", "false")

	report, err := ValidateRuntimeProfile()
	if err != nil {
		t.Fatalf("ValidateRuntimeProfile: %v", err)
	}
	if report.Profile != string(ProfileGM) || !report.Available {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Degraded) != 1 || report.Degraded[0] != CapabilityGMTLS {
		t.Fatalf("Degraded = %v, want [gm_tls]", report.Degraded)
	}
}
