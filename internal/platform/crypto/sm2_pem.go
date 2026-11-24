package crypto

import (
	"encoding/pem"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ParseSM2PrivateKeyPEM parses a PEM-encoded SM2 private key and returns the
// opaque handle consumed by CryptoSuite. It is the only exported entry point so
// callers outside this package never import a national-crypto library directly.
//
// PKCS#8 ("PRIVATE KEY") is the canonical shape. SEC1 ("EC PRIVATE KEY") is
// accepted as well because operator tooling commonly emits it.
func ParseSM2PrivateKeyPEM(pemData string) (*SM2PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, apperrors.NewInvalidArgument("SM2 private key is not valid PEM")
	}
	key, err := parseSM2PrivateKey(block.Bytes)
	if err != nil {
		return nil, apperrors.WrapInvalidArgumentf(err, "parse SM2 private key")
	}
	return &SM2PrivateKey{key: key}, nil
}
