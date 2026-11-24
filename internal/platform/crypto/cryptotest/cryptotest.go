// Package cryptotest provides the fixed development encryption key used by test
// binaries.
//
// DB-058 forbids degrading confidential columns to plaintext, so a missing
// ENCRYPTION_KEY_CURRENT aborts startup in every environment. Tests therefore
// configure a dedicated key instead of disabling the encryption path.
package cryptotest

import (
	"crypto/rand"
	"encoding/pem"
	"os"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/smx509"
)

// Key is a deterministic 32-byte AES-256 key (the bytes 0x00..0x1f, base64
// encoded). It is public on purpose: it must never protect real data.
const Key = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="

// Configure points the field encryptor at Key. It must run before the first
// encrypted read or write, i.e. from TestMain.
func Configure() {
	_ = os.Setenv("ENCRYPTION_KEY_CURRENT", Key)
}

// GenerateSM2PrivateKeyPEM generates a fresh SM2 PKCS#8 private key in PEM
// form. Test fixtures must be generated at runtime, and this helper keeps
// packages outside platform/crypto from importing gmsm directly.
func GenerateSM2PrivateKeyPEM() (string, error) {
	key, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	der, err := smx509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}
