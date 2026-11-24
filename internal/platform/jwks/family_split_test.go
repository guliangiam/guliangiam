package jwks

import (
	"strings"
	"testing"
	"time"
)

// assertNoGMKeys pins the reason the standard endpoint exists: a strict JOSE
// library (Nimbus/Spring) aborts the whole key set on the unknown SM2 curve, so
// GetJWKS must never carry an SM2-SM3 key, whatever the manager's configuration.
func assertNoGMKeys(t *testing.T, set *JWKS) {
	t.Helper()
	for i := range set.Keys {
		key := &set.Keys[i]
		if key.Alg == SigningAlgorithmSM2SM3 || key.Crv == "SM2" || strings.HasPrefix(key.Kid, KidPrefixGM) {
			t.Fatalf("standard JWKS leaked a GM key: %+v", *key)
		}
	}
}

// assertAllGMKeys pins the GM endpoint contract: every published key is an
// SM2-SM3 EC key with the gm_ kid prefix a GM-capable RP selects on.
func assertAllGMKeys(t *testing.T, set *JWKS) {
	t.Helper()
	for i := range set.Keys {
		key := &set.Keys[i]
		if key.Alg != SigningAlgorithmSM2SM3 || key.Crv != "SM2" || key.Kty != "EC" ||
			!strings.HasPrefix(key.Kid, KidPrefixGM) {
			t.Fatalf("GM JWKS carries a non-SM2 key: %+v", *key)
		}
	}
}

// assertSplitCoversAll checks the two family documents partition getAllJWKS:
// no key is dropped or duplicated by the split.
func assertSplitCoversAll(t *testing.T, manager *Manager) {
	t.Helper()
	all, err := manager.getAllJWKS()
	if err != nil {
		t.Fatalf("getAllJWKS() error = %v", err)
	}
	standard, err := manager.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	gm, err := manager.GetGMJWKS()
	if err != nil {
		t.Fatalf("GetGMJWKS() error = %v", err)
	}
	if len(standard.Keys)+len(gm.Keys) != len(all.Keys) {
		t.Fatalf("split lost keys: standard=%d gm=%d all=%d",
			len(standard.Keys), len(gm.Keys), len(all.Keys))
	}
	seen := map[string]bool{}
	combined := append(append([]JWK(nil), standard.Keys...), gm.Keys...)
	for i := range combined {
		kid := combined[i].Kid
		if seen[kid] {
			t.Fatalf("kid %q published by both family endpoints", kid)
		}
		seen[kid] = true
	}
	for i := range all.Keys {
		kid := all.Keys[i].Kid
		if !seen[kid] {
			t.Fatalf("kid %q missing from both family endpoints", kid)
		}
	}
}

func TestGetJWKSExcludesGMFamily(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rsaPEM := testRSAPrivateKeyPEM(t)
	sm2PEM := testSM2PrivateKeyPEM(t)
	retiredAt := now.Add(-time.Hour)

	t.Run("legacy ephemeral manager", func(t *testing.T) {
		manager := NewManager(2048, nil)
		standard, err := manager.GetJWKS()
		if err != nil {
			t.Fatalf("GetJWKS() error = %v", err)
		}
		assertNoGMKeys(t, standard)
		gm, err := manager.GetGMJWKS()
		if err != nil {
			t.Fatalf("GetGMJWKS() error = %v", err)
		}
		assertAllGMKeys(t, gm)
		assertSplitCoversAll(t, manager)
	})

	t.Run("rsa-only key ring", func(t *testing.T) {
		manager := mustManagerFromRing(t, "",
			testRingEntry("rsa_only", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
		)
		standard, err := manager.GetJWKS()
		if err != nil {
			t.Fatalf("GetJWKS() error = %v", err)
		}
		assertNoGMKeys(t, standard)
		if len(standard.Keys) != 1 {
			t.Fatalf("len(JWKS.Keys) = %d, want 1", len(standard.Keys))
		}
		gm, err := manager.GetGMJWKS()
		if err != nil {
			t.Fatalf("GetGMJWKS() error = %v", err)
		}
		if len(gm.Keys) != 0 {
			t.Fatalf("GM JWKS = %+v, want empty for an RSA-only ring", gm.Keys)
		}
		assertSplitCoversAll(t, manager)
	})

	t.Run("gm-only key ring", func(t *testing.T) {
		manager := mustManagerFromRing(t, "",
			testRingEntry("gm_only", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-time.Hour), nil),
		)
		standard, err := manager.GetJWKS()
		if err != nil {
			t.Fatalf("GetJWKS() error = %v", err)
		}
		if len(standard.Keys) != 0 {
			t.Fatalf("JWKS = %+v, want empty for a GM-only ring", standard.Keys)
		}
		assertNoGMKeys(t, standard)
		gm, err := manager.GetGMJWKS()
		if err != nil {
			t.Fatalf("GetGMJWKS() error = %v", err)
		}
		if len(gm.Keys) != 1 {
			t.Fatalf("len(GMJWKS.Keys) = %d, want 1", len(gm.Keys))
		}
		assertAllGMKeys(t, gm)
		assertSplitCoversAll(t, manager)
	})

	t.Run("both families with retired keys", func(t *testing.T) {
		manager := mustManagerFromRing(t, "",
			testRingEntry("rsa_active", SigningAlgorithmRS256, rsaPEM, now.Add(-time.Hour), nil),
			testRingEntry("rsa_retired", SigningAlgorithmRS256, rsaPEM, now.Add(-10*time.Hour), &retiredAt),
			testRingEntry("gm_active", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-time.Hour), nil),
			testRingEntry("gm_retired", SigningAlgorithmSM2SM3, sm2PEM, now.Add(-10*time.Hour), &retiredAt),
		)
		standard, err := manager.GetJWKS()
		if err != nil {
			t.Fatalf("GetJWKS() error = %v", err)
		}
		assertNoGMKeys(t, standard)
		if len(standard.Keys) != 2 {
			t.Fatalf("len(JWKS.Keys) = %d, want 2 (active+retired RSA)", len(standard.Keys))
		}
		gm, err := manager.GetGMJWKS()
		if err != nil {
			t.Fatalf("GetGMJWKS() error = %v", err)
		}
		assertAllGMKeys(t, gm)
		if len(gm.Keys) != 2 {
			t.Fatalf("len(GMJWKS.Keys) = %d, want 2 (active+retired SM2)", len(gm.Keys))
		}
		assertSplitCoversAll(t, manager)
	})
}
