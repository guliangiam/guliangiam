package jwks

import (
	"bytes"
	"encoding/base64"
	"testing"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// sm2CoordinateBytes is the SM2 (sm2p256v1) field element size. RFC 7518
// §6.2.1.2 requires an EC JWK coordinate octet string to be exactly this long.
const sm2CoordinateBytes = 32

func TestManager_GetGMKeyPair_SignVerify(t *testing.T) {
	mgr := NewManager(2048, nil)
	kp, err := mgr.GetGMKeyPair()
	if err != nil {
		t.Fatalf("GetGMKeyPair() error = %v", err)
	}
	if len(kp.Kid) <= len(KidPrefixGM) || kp.Kid[:len(KidPrefixGM)] != KidPrefixGM {
		t.Fatalf("GM kid = %q, want %s prefix", kp.Kid, KidPrefixGM)
	}
	if kp.Algorithm != SigningAlgorithmSM2SM3 {
		t.Fatalf("GM algorithm = %q", kp.Algorithm)
	}

	digest := platformcrypto.SM3Sum([]byte("jwt-signing-input"))
	signature, err := platformcrypto.NewCryptoSuite().SM2Sign(kp.PrivateKey, digest)
	if err != nil {
		t.Fatalf("SM2Sign: %v", err)
	}
	if !platformcrypto.NewCryptoSuite().SM2Verify(kp.PublicKey, digest, signature) {
		t.Fatal("SM2Verify rejected the JWKS GM key signature")
	}
}

func TestManager_CrossAlgorithmKidLookupFails(t *testing.T) {
	mgr := NewManager(2048, nil)
	rsaKP, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair: %v", err)
	}
	gmKP, err := mgr.GetGMKeyPair()
	if err != nil {
		t.Fatalf("GetGMKeyPair: %v", err)
	}

	if _, err := mgr.GetGMKeyPairByID(rsaKP.Kid); err == nil {
		t.Fatal("gm lookup accepted an rsa_ kid")
	}
	if _, err := mgr.GetKeyPairByID(gmKP.Kid); err == nil {
		t.Fatal("rsa lookup accepted a gm_ kid")
	}
}

// testSM2PublicKeyWithLeadingZeroCoordinate generates keys until one coordinate
// starts with a zero byte. big.Int.Bytes() drops that byte, so this is the
// fixture that exposes a short JWK coordinate. It needs ~128 attempts on
// average; the cap keeps a bad RNG from hanging the test.
func testSM2PublicKeyWithLeadingZeroCoordinate(t *testing.T) *platformcrypto.SM2PublicKey {
	t.Helper()

	for attempt := 1; attempt <= 2000; attempt++ {
		key, err := platformcrypto.GenerateSM2Key()
		if err != nil {
			t.Fatalf("GenerateSM2Key: %v", err)
		}
		publicKey := key.Public()
		x, y, ok := publicKey.Coordinates()
		if !ok {
			t.Fatal("generated SM2 key has no public coordinates")
		}
		if len(x.Bytes()) < sm2CoordinateBytes || len(y.Bytes()) < sm2CoordinateBytes {
			t.Logf("leading-zero SM2 key found after %d attempt(s): x=%d bytes, y=%d bytes",
				attempt, len(x.Bytes()), len(y.Bytes()))
			return publicKey
		}
	}
	t.Fatal("no SM2 key with a leading-zero coordinate in 2000 attempts")
	return nil
}

// TestPublicKeyToECJWKPadsLeadingZeroCoordinates is the regression for GM R3:
// a coordinate whose high byte is zero must still be published as exactly the
// SM2 field size, otherwise strict JOSE consumers reject the key.
func TestPublicKeyToECJWKPadsLeadingZeroCoordinates(t *testing.T) {
	publicKey := testSM2PublicKeyWithLeadingZeroCoordinate(t)
	x, y, ok := publicKey.Coordinates()
	if !ok {
		t.Fatal("SM2 public key has no coordinates")
	}

	jwk := publicKeyToECJWK(publicKey, KidPrefixGM+"leading-zero", SigningAlgorithmSM2SM3)
	if jwk.Kty != "EC" || jwk.Crv != "SM2" || jwk.Alg != SigningAlgorithmSM2SM3 ||
		jwk.Kid != KidPrefixGM+"leading-zero" {
		t.Fatalf("JWK metadata changed: %+v", jwk)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		t.Fatalf("decode JWK x: %v", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		t.Fatalf("decode JWK y: %v", err)
	}
	if len(xBytes) != sm2CoordinateBytes {
		t.Fatalf("JWK x is %d bytes, want %d (raw big.Int was %d bytes): strict RPs reject it",
			len(xBytes), sm2CoordinateBytes, len(x.Bytes()))
	}
	if len(yBytes) != sm2CoordinateBytes {
		t.Fatalf("JWK y is %d bytes, want %d (raw big.Int was %d bytes): strict RPs reject it",
			len(yBytes), sm2CoordinateBytes, len(y.Bytes()))
	}

	wantX := make([]byte, sm2CoordinateBytes)
	x.FillBytes(wantX)
	if !bytes.Equal(xBytes, wantX) {
		t.Fatalf("JWK x = %x, want left-zero-padded %x", xBytes, wantX)
	}
	wantY := make([]byte, sm2CoordinateBytes)
	y.FillBytes(wantY)
	if !bytes.Equal(yBytes, wantY) {
		t.Fatalf("JWK y = %x, want left-zero-padded %x", yBytes, wantY)
	}
}
