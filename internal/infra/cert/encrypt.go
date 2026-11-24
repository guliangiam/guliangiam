package cert

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// EncryptPrivateKey encrypts a private key string using AES-256-GCM.
// The key must be exactly 32 bytes (AES-256).
// Returns a base64-encoded string containing the nonce prepended to the ciphertext.
func EncryptPrivateKey(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", apperrors.NewInvalidArgumentf("key must be 32 bytes for AES-256, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create AES cipher")
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create GCM")
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", apperrors.WrapInternalf(err, "generate nonce")
	}

	// Seal appends the ciphertext to the nonce, so the result is nonce||ciphertext||tag
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptPrivateKey decrypts a private key string using AES-256-GCM.
// The key must be exactly 32 bytes (AES-256).
// The encoded string must be base64-encoded and contain nonce||ciphertext||tag.
func DecryptPrivateKey(encoded string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", apperrors.NewInvalidArgumentf("key must be 32 bytes for AES-256, got %d", len(key))
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "base64 decode")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create AES cipher")
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create GCM")
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", apperrors.NewInternal("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "decrypt")
	}

	return string(plaintext), nil
}
