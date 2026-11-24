package jwks

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

func testRSAPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal RSA PKCS#8: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func testSM2PrivateKeyPEM(t *testing.T) string {
	t.Helper()
	pemData, err := cryptotest.GenerateSM2PrivateKeyPEM()
	if err != nil {
		t.Fatalf("generate SM2 PKCS#8 PEM: %v", err)
	}
	return pemData
}

func testRingEntry(kid, algorithm, privateKeyPEM string, createdAt time.Time, retiredAt *time.Time) map[string]any {
	return map[string]any{
		"kid":             kid,
		"algorithm":       algorithm,
		"private_key_pem": privateKeyPEM,
		"created_at":      createdAt,
		"retired_at":      retiredAt,
	}
}

func testRingJSON(t *testing.T, overlap string, entries ...map[string]any) []byte {
	t.Helper()
	document := map[string]any{"keys": entries}
	if overlap != "" {
		document["overlap"] = overlap
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal key ring JSON: %v", err)
	}
	return data
}

func TestParseKeyRingAcceptsMultipleKeys(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rsaPEM := testRSAPrivateKeyPEM(t)
	sm2PEM := testSM2PrivateKeyPEM(t)

	retiredRSAAt := now.Add(-2 * time.Hour)
	retiredGMAt := now.Add(-1 * time.Hour)
	ring, err := ParseKeyRing(testRingJSON(t, "",
		testRingEntry("rsa_active", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
		testRingEntry("rsa_retired", SigningAlgorithmRS256, rsaPEM, now.Add(-10*time.Hour), &retiredRSAAt),
		testRingEntry("gm_active", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-30*time.Minute), nil),
		testRingEntry("gm_retired", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-20*time.Hour), &retiredGMAt),
	))
	if err != nil {
		t.Fatalf("ParseKeyRing() error = %v", err)
	}
	if ring.Len() != 4 {
		t.Fatalf("Len() = %d, want 4", ring.Len())
	}
	if ring.Overlap() != DefaultRetirementOverlap {
		t.Fatalf("Overlap() = %s, want %s", ring.Overlap(), DefaultRetirementOverlap)
	}
	active := ring.ActiveKids()
	if len(active) != 2 {
		t.Fatalf("ActiveKids() = %v, want one RSA and one SM2 kid", active)
	}

	manager, err := NewManagerWithKeyRing(ring)
	if err != nil {
		t.Fatalf("NewManagerWithKeyRing() error = %v", err)
	}
	rsaKey, err := manager.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}
	if rsaKey.Kid != "rsa_active" {
		t.Fatalf("signing RSA kid = %q, want rsa_active", rsaKey.Kid)
	}
	gmKey, err := manager.GetGMKeyPair()
	if err != nil {
		t.Fatalf("GetGMKeyPair() error = %v", err)
	}
	if gmKey.Kid != "gm_active" {
		t.Fatalf("signing SM2 kid = %q, want gm_active", gmKey.Kid)
	}
	if _, err := manager.GetKeyPairByID("rsa_retired"); err != nil {
		t.Fatalf("retired RSA kid should verify inside the overlap window: %v", err)
	}
	if _, err := manager.GetGMKeyPairByID("gm_retired"); err != nil {
		t.Fatalf("retired SM2 kid should verify inside the overlap window: %v", err)
	}

	// The full publishable set spans both families (active+retired each), but
	// the standard endpoint only carries the international keys and the GM
	// endpoint only the SM2-SM3 keys.
	all, err := manager.getAllJWKS()
	if err != nil {
		t.Fatalf("getAllJWKS() error = %v", err)
	}
	if len(all.Keys) != 4 {
		t.Fatalf("len(getAllJWKS().Keys) = %d, want 4 (active+retired for both families)", len(all.Keys))
	}
	set, err := manager.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(set.Keys) != 2 {
		t.Fatalf("len(JWKS.Keys) = %d, want 2 (international active+retired)", len(set.Keys))
	}
	gmSet, err := manager.GetGMJWKS()
	if err != nil {
		t.Fatalf("GetGMJWKS() error = %v", err)
	}
	if len(gmSet.Keys) != 2 {
		t.Fatalf("len(GMJWKS.Keys) = %d, want 2 (SM2 active+retired)", len(gmSet.Keys))
	}
}

func TestParseKeyRingSelectsNewestActiveKey(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rsaPEM := testRSAPrivateKeyPEM(t)

	ring, err := ParseKeyRing(testRingJSON(t, "",
		testRingEntry("rsa_old", SigningAlgorithmRS256, rsaPEM, now.Add(-10*time.Hour), nil),
		testRingEntry("rsa_new", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
	))
	if err != nil {
		t.Fatalf("ParseKeyRing() error = %v", err)
	}
	manager, err := NewManagerWithKeyRing(ring)
	if err != nil {
		t.Fatalf("NewManagerWithKeyRing() error = %v", err)
	}

	key, err := manager.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}
	if key.Kid != "rsa_new" {
		t.Fatalf("signing kid = %q, want the newest active key rsa_new", key.Kid)
	}
	// A superseded key without retired_at is still a verification key; the
	// operator retires it explicitly to start the overlap countdown.
	if _, err := manager.GetKeyPairByID("rsa_old"); err != nil {
		t.Fatalf("superseded key should remain usable for verification: %v", err)
	}
	set, err := manager.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(set.Keys) != 2 {
		t.Fatalf("len(JWKS.Keys) = %d, want 2", len(set.Keys))
	}
}

func TestParseKeyRingRejectsInvalidDocuments(t *testing.T) {
	rsaPEM := testRSAPrivateKeyPEM(t)
	sm2PEM := testSM2PrivateKeyPEM(t)
	now := time.Now().UTC().Truncate(time.Second)

	t.Run("duplicate kid", func(t *testing.T) {
		retiredAt := now.Add(-2 * time.Hour)
		_, err := ParseKeyRing(testRingJSON(t, "",
			testRingEntry("rsa_dup", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
			testRingEntry("rsa_dup", SigningAlgorithmRS256, rsaPEM, now.Add(-5*time.Hour), &retiredAt),
		))
		if err == nil || !strings.Contains(err.Error(), "duplicate kid") {
			t.Fatalf("error = %v, want duplicate kid rejection", err)
		}
	})

	t.Run("kid prefix does not match rsa algorithm", func(t *testing.T) {
		_, err := ParseKeyRing(testRingJSON(t, "",
			testRingEntry("gm_not_rsa", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
		))
		if err == nil || !strings.Contains(err.Error(), `must start with "rsa_"`) {
			t.Fatalf("error = %v, want rsa_ prefix rejection", err)
		}
	})

	t.Run("kid prefix does not match gm algorithm", func(t *testing.T) {
		_, err := ParseKeyRing(testRingJSON(t, "",
			testRingEntry("rsa_not_gm", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-time.Hour), nil),
		))
		if err == nil || !strings.Contains(err.Error(), `must start with "gm_"`) {
			t.Fatalf("error = %v, want gm_ prefix rejection", err)
		}
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		_, err := ParseKeyRing(testRingJSON(t, "",
			testRingEntry("rsa_es256", "ES256", rsaPEM, now.Add(-time.Hour), nil),
		))
		if err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("error = %v, want closed-enum rejection", err)
		}
	})

	t.Run("bad rsa pem", func(t *testing.T) {
		_, err := ParseKeyRing(testRingJSON(t, "",
			testRingEntry("rsa_bad", SigningAlgorithmRS256, "not a PEM key", now.Add(-time.Hour), nil),
		))
		if err == nil || !strings.Contains(err.Error(), "parse RSA private key") {
			t.Fatalf("error = %v, want RSA PEM rejection", err)
		}
	})

	t.Run("bad sm2 pem", func(t *testing.T) {
		_, err := ParseKeyRing(testRingJSON(t, "",
			testRingEntry("gm_bad", SigningAlgorithmSM2SM3, rsaPEM, now.Add(-time.Hour), nil),
		))
		if err == nil || !strings.Contains(err.Error(), "parse SM2 private key") {
			t.Fatalf("error = %v, want SM2 PEM rejection", err)
		}
	})

	t.Run("missing created_at", func(t *testing.T) {
		entry := testRingEntry("rsa_no_created", SigningAlgorithmRS256, rsaPEM, now, nil)
		delete(entry, "created_at")
		_, err := ParseKeyRing(testRingJSON(t, "", entry))
		if err == nil || !strings.Contains(err.Error(), "created_at is required") {
			t.Fatalf("error = %v, want created_at rejection", err)
		}
	})

	t.Run("retired_at earlier than created_at", func(t *testing.T) {
		retiredAt := now.Add(-2 * time.Hour)
		_, err := ParseKeyRing(testRingJSON(t, "",
			testRingEntry("rsa_time_order", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), &retiredAt),
		))
		if err == nil || !strings.Contains(err.Error(), "earlier than created_at") {
			t.Fatalf("error = %v, want retired_at ordering rejection", err)
		}
	})

	t.Run("overlap shorter than minimum", func(t *testing.T) {
		_, err := ParseKeyRing(testRingJSON(t, "24h",
			testRingEntry("rsa_short_overlap", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
		))
		if err == nil || !strings.Contains(err.Error(), "shorter than") {
			t.Fatalf("error = %v, want overlap floor rejection", err)
		}
	})

	t.Run("empty keys", func(t *testing.T) {
		_, err := ParseKeyRing(testRingJSON(t, ""))
		if err == nil || !strings.Contains(err.Error(), "at least one key") {
			t.Fatalf("error = %v, want empty-ring rejection", err)
		}
	})

	t.Run("family without active key", func(t *testing.T) {
		retiredAt := now.Add(-time.Hour)
		_, err := ParseKeyRing(testRingJSON(t, "",
			testRingEntry("rsa_all_retired", SigningAlgorithmRS256, rsaPEM, now.Add(-2*time.Hour), &retiredAt),
		))
		if err == nil || !strings.Contains(err.Error(), "no active RS256 key") {
			t.Fatalf("error = %v, want missing-active rejection", err)
		}
	})

	t.Run("ambiguous active keys", func(t *testing.T) {
		createdAt := now.Add(-time.Hour)
		_, err := ParseKeyRing(testRingJSON(t, "",
			testRingEntry("rsa_ambiguous_a", SigningAlgorithmRS256, rsaPEM, createdAt, nil),
			testRingEntry("rsa_ambiguous_b", SigningAlgorithmRS256, rsaPEM, createdAt, nil),
		))
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("error = %v, want ambiguous signer rejection", err)
		}
	})
}

func TestLoadKeyRingReadsFile(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rsaPEM := testRSAPrivateKeyPEM(t)
	path := filepath.Join(t.TempDir(), "jwt-signing-keys.json")
	data := testRingJSON(t, "",
		testRingEntry("rsa_file", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
	)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write key ring: %v", err)
	}

	ring, err := LoadKeyRing(path)
	if err != nil {
		t.Fatalf("LoadKeyRing() error = %v", err)
	}
	if ring.Len() != 1 || len(ring.ActiveKids()) != 1 || ring.ActiveKids()[0] != "rsa_file" {
		t.Fatalf("ring = %+v, want one active rsa_file key", ring)
	}
}

func TestLoadKeyRingRejectsUnreadableFile(t *testing.T) {
	_, err := LoadKeyRing(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil || !strings.Contains(err.Error(), "read JWT signing key ring file") {
		t.Fatalf("error = %v, want unreadable-file rejection", err)
	}
}

func TestNewManagerWithKeyRingRejectsEmptyRing(t *testing.T) {
	if _, err := NewManagerWithKeyRing(nil); err == nil {
		t.Fatal("nil key ring must be rejected")
	}
	if _, err := NewManagerWithKeyRing(&KeyRing{}); err == nil {
		t.Fatal("empty key ring must be rejected")
	}
}
