package jwks

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustManagerFromRing(t *testing.T, overlap string, entries ...map[string]any) *Manager {
	t.Helper()
	ring, err := ParseKeyRing(testRingJSON(t, overlap, entries...))
	if err != nil {
		t.Fatalf("ParseKeyRing() error = %v", err)
	}
	manager, err := NewManagerWithKeyRing(ring)
	if err != nil {
		t.Fatalf("NewManagerWithKeyRing() error = %v", err)
	}
	return manager
}

func TestManagerKeyRingRetiredKeyVerifiesUntilOverlapCloses(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rsaPEM := testRSAPrivateKeyPEM(t)
	retiredAt := now.Add(-59 * 24 * time.Hour) // expiry = now + 1d with the 60d default

	manager := mustManagerFromRing(t, "",
		testRingEntry("rsa_active", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
		testRingEntry("rsa_retired", SigningAlgorithmRS256, rsaPEM, now.Add(-100*24*time.Hour), &retiredAt),
	)

	if _, err := manager.GetKeyPairByID("rsa_retired"); err != nil {
		t.Fatalf("retired key inside the window must verify: %v", err)
	}
	set, err := manager.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(set.Keys) != 2 {
		t.Fatalf("len(JWKS.Keys) = %d, want 2 inside the window", len(set.Keys))
	}

	// The boundary itself is still inside the window.
	manager.now = func() time.Time { return retiredAt.Add(DefaultRetirementOverlap) }
	if _, err := manager.GetKeyPairByID("rsa_retired"); err != nil {
		t.Fatalf("retired key at the exact boundary must verify: %v", err)
	}

	manager.now = func() time.Time { return retiredAt.Add(DefaultRetirementOverlap + time.Second) }
	if _, err := manager.GetKeyPairByID("rsa_retired"); err == nil {
		t.Fatal("retired key outside the window must not verify")
	}
	set, err = manager.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() after window error = %v", err)
	}
	if len(set.Keys) != 1 || set.Keys[0].Kid != "rsa_active" {
		t.Fatalf("JWKS after window = %+v, want only rsa_active", set.Keys)
	}
	if _, err := manager.GetKeyPair(); err != nil {
		t.Fatalf("active signing key must survive retirement expiry: %v", err)
	}
}

func TestManagerKeyRingJWKSPublishesAllWindowKidsWithoutPrivateMaterial(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rsaPEM := testRSAPrivateKeyPEM(t)
	sm2PEM := testSM2PrivateKeyPEM(t)
	rsaRetiredAt := now.Add(-time.Hour)
	gmRetiredAt := now.Add(-time.Hour)

	manager := mustManagerFromRing(t, "",
		testRingEntry("rsa_active", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
		testRingEntry("rsa_retired", SigningAlgorithmRS256, rsaPEM, now.Add(-10*time.Hour), &rsaRetiredAt),
		testRingEntry("gm_active", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-time.Hour), nil),
		testRingEntry("gm_retired", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-10*time.Hour), &gmRetiredAt),
	)

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
	all, err := manager.getAllJWKS()
	if err != nil {
		t.Fatalf("getAllJWKS() error = %v", err)
	}
	if len(all.Keys) != 4 {
		t.Fatalf("len(getAllJWKS().Keys) = %d, want 4", len(all.Keys))
	}
	seen := map[string]bool{}
	for _, key := range all.Keys {
		seen[key.Kid] = true
		raw, err := json.Marshal(key)
		if err != nil {
			t.Fatalf("marshal JWK %q: %v", key.Kid, err)
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatalf("unmarshal JWK %q: %v", key.Kid, err)
		}
		for field := range fields {
			switch field {
			case "kty", "use", "kid", "alg", "n", "e", "crv", "x", "y":
			default:
				t.Fatalf("JWK %q leaked non-public field %q: %s", key.Kid, field, raw)
			}
		}
		if key.Kty == "RSA" && (key.N == "" || key.E == "") {
			t.Fatalf("RSA JWK %q lacks public parameters: %+v", key.Kid, key)
		}
		if key.Kty == "EC" && (key.X == "" || key.Y == "") {
			t.Fatalf("SM2 JWK %q lacks public parameters: %+v", key.Kid, key)
		}
	}
	for _, kid := range []string{"rsa_active", "rsa_retired", "gm_active", "gm_retired"} {
		if !seen[kid] {
			t.Fatalf("JWKS is missing window kid %q: %+v", kid, all.Keys)
		}
	}
	// Each endpoint publishes only its own family's window kids.
	for _, key := range set.Keys {
		if key.Alg != SigningAlgorithmRS256 {
			t.Fatalf("standard JWKS carries non-international key %+v", key)
		}
	}
	for _, kid := range []string{"rsa_active", "rsa_retired"} {
		if !containsKid(set.Keys, kid) {
			t.Fatalf("standard JWKS is missing window kid %q: %+v", kid, set.Keys)
		}
	}
	for _, key := range gmSet.Keys {
		if key.Alg != SigningAlgorithmSM2SM3 {
			t.Fatalf("GM JWKS carries non-SM2 key %+v", key)
		}
	}
	for _, kid := range []string{"gm_active", "gm_retired"} {
		if !containsKid(gmSet.Keys, kid) {
			t.Fatalf("GM JWKS is missing window kid %q: %+v", kid, gmSet.Keys)
		}
	}

	for _, published := range []*JWKS{set, gmSet} {
		rawSet, err := json.Marshal(published)
		if err != nil {
			t.Fatalf("marshal JWKS: %v", err)
		}
		serialized := string(rawSet)
		if strings.Contains(serialized, "PRIVATE KEY") {
			t.Fatalf("JWKS serialization leaked private key material: %s", serialized)
		}
		if strings.Contains(serialized, `"d":`) || strings.Contains(serialized, `"p":`) ||
			strings.Contains(serialized, `"q":`) || strings.Contains(serialized, `"dp":`) ||
			strings.Contains(serialized, `"dq":`) || strings.Contains(serialized, `"qi":`) {
			t.Fatalf("JWKS serialization leaked RSA private parameters: %s", serialized)
		}
	}
}

func containsKid(keys []JWK, kid string) bool {
	for i := range keys {
		if keys[i].Kid == kid {
			return true
		}
	}
	return false
}

func TestManagerKeyRingNeverGeneratesMissingFamily(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rsaPEM := testRSAPrivateKeyPEM(t)
	manager := mustManagerFromRing(t, "",
		testRingEntry("rsa_only", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
	)
	if _, err := manager.GetGMKeyPair(); err == nil ||
		!strings.Contains(err.Error(), "no active SM2-SM3 signing key") {
		t.Fatalf("GetGMKeyPair error = %v, want fail-close for absent SM2 family", err)
	}
	if _, err := manager.GetGMKeyPairByID("gm_unknown"); err == nil {
		t.Fatal("unknown gm_ kid must not resolve through lazy generation")
	}
	set, err := manager.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(set.Keys) != 1 || set.Keys[0].Kid != "rsa_only" {
		t.Fatalf("JWKS = %+v, want only the configured RSA key", set.Keys)
	}
	gmSet, err := manager.GetGMJWKS()
	if err != nil {
		t.Fatalf("GetGMJWKS() error = %v", err)
	}
	if len(gmSet.Keys) != 0 {
		t.Fatalf("GM JWKS = %+v, want empty for an RSA-only ring (no SM2 generation)", gmSet.Keys)
	}

	sm2PEM := testSM2PrivateKeyPEM(t)
	gmManager := mustManagerFromRing(t, "",
		testRingEntry("gm_only", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-time.Hour), nil),
	)
	if _, err := gmManager.GetKeyPair(); err == nil ||
		!strings.Contains(err.Error(), "no active RS256 signing key") {
		t.Fatalf("GetKeyPair error = %v, want fail-close for absent RSA family", err)
	}
	if _, err := gmManager.GetKeyPairByID("rsa_unknown"); err == nil {
		t.Fatal("unknown rsa_ kid must not resolve through lazy generation")
	}
	gmSet, err = gmManager.GetGMJWKS()
	if err != nil {
		t.Fatalf("GetGMJWKS() error = %v", err)
	}
	if len(gmSet.Keys) != 1 || gmSet.Keys[0].Kid != "gm_only" {
		t.Fatalf("GM JWKS = %+v, want only the configured SM2 key", gmSet.Keys)
	}
	set, err = gmManager.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(set.Keys) != 0 {
		t.Fatalf("JWKS = %+v, want empty for a GM-only ring (no RSA generation)", set.Keys)
	}
}

func TestManagerKeyRingCrossAlgorithmLookupFails(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rsaPEM := testRSAPrivateKeyPEM(t)
	sm2PEM := testSM2PrivateKeyPEM(t)
	manager := mustManagerFromRing(t, "",
		testRingEntry("rsa_active", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
		testRingEntry("gm_active", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-time.Hour), nil),
	)
	if _, err := manager.GetGMKeyPairByID("rsa_active"); err == nil {
		t.Fatal("SM2 lookup accepted an rsa_ kid")
	}
	if _, err := manager.GetKeyPairByID("gm_active"); err == nil {
		t.Fatal("RSA lookup accepted a gm_ kid")
	}
}

func TestManagerKeyRingConcurrentReads(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rsaPEM := testRSAPrivateKeyPEM(t)
	sm2PEM := testSM2PrivateKeyPEM(t)
	retiredAt := now.Add(-time.Hour)
	manager := mustManagerFromRing(t, "",
		testRingEntry("rsa_active", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
		testRingEntry("rsa_retired", SigningAlgorithmRS256, rsaPEM, now.Add(-10*time.Hour), &retiredAt),
		testRingEntry("gm_active", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-time.Hour), nil),
	)

	var wait sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := 0; i < 100; i++ {
				if _, err := manager.GetKeyPair(); err != nil {
					t.Errorf("GetKeyPair() error = %v", err)
					return
				}
				if _, err := manager.GetGMKeyPair(); err != nil {
					t.Errorf("GetGMKeyPair() error = %v", err)
					return
				}
				if _, err := manager.GetKeyPairByID("rsa_retired"); err != nil {
					t.Errorf("GetKeyPairByID(retired) error = %v", err)
					return
				}
				if _, err := manager.GetGMKeyPairByID("gm_active"); err != nil {
					t.Errorf("GetGMKeyPairByID(active) error = %v", err)
					return
				}
				set, err := manager.GetJWKS()
				if err != nil {
					t.Errorf("GetJWKS() error = %v", err)
					return
				}
				if len(set.Keys) != 2 {
					t.Errorf("len(JWKS.Keys) = %d, want 2 (rsa_active + rsa_retired)", len(set.Keys))
					return
				}
				gmSet, err := manager.GetGMJWKS()
				if err != nil {
					t.Errorf("GetGMJWKS() error = %v", err)
					return
				}
				if len(gmSet.Keys) != 1 {
					t.Errorf("len(GMJWKS.Keys) = %d, want 1 (gm_active)", len(gmSet.Keys))
					return
				}
				all, err := manager.getAllJWKS()
				if err != nil {
					t.Errorf("getAllJWKS() error = %v", err)
					return
				}
				if len(all.Keys) != 3 {
					t.Errorf("len(getAllJWKS().Keys) = %d, want 3", len(all.Keys))
					return
				}
			}
		}()
	}
	wait.Wait()
}

func TestManagerWithoutKeyRingKeepsEphemeralBehavior(t *testing.T) {
	manager := NewManager(2048, nil)
	first, err := manager.GetKeyPair()
	if err != nil {
		t.Fatalf("first GetKeyPair() error = %v", err)
	}
	second, err := manager.GetKeyPair()
	if err != nil {
		t.Fatalf("second GetKeyPair() error = %v", err)
	}
	if first.Kid != second.Kid {
		t.Fatalf("ephemeral manager changed kid between reads: %q != %q", first.Kid, second.Kid)
	}
	set, err := manager.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(set.Keys) != 1 || set.Keys[0].Alg != SigningAlgorithmRS256 {
		t.Fatalf("JWKS = %+v, want 1 ephemeral RSA key", set.Keys)
	}
	gmSet, err := manager.GetGMJWKS()
	if err != nil {
		t.Fatalf("GetGMJWKS() error = %v", err)
	}
	if len(gmSet.Keys) != 1 || gmSet.Keys[0].Alg != SigningAlgorithmSM2SM3 {
		t.Fatalf("GM JWKS = %+v, want 1 ephemeral SM2 key", gmSet.Keys)
	}
	if manager.keyRing {
		t.Fatal("NewManager must not mark the manager as key-ring backed")
	}
}
