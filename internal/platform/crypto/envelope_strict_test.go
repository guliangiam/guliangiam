package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"testing"
)

func newStrictTestEncryptor(t *testing.T) *FieldEncryptor {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(key))
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("IS_PRODUCTION", "false")
	return NewFieldEncryptor()
}

func TestIsEnvelopeToken(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)
	valid, err := encryptor.EncryptField("envelope-shape")
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"generated", valid, true},
		{"leading-zero-version", "v01:AAAA:BBBB:CCCC", true},
		{"tagged-aes", "v1-aes:AAAA:BBBB:CCCC", true},
		{"tagged-sm4", "v1-sm4:AAAA:BBBB:CCCC", true},
		{"aad-v2", "v1-aes-aad:AAAA:BBBB:CCCC", true},
		{"aad-v2-sm4", "v1-sm4-aad:AAAA:BBBB:CCCC", true},
		{"aad-without-algorithm", "v1-aad:AAAA:BBBB:CCCC", false},
		{"aad-unknown-suffix", "v1-aes-xyz:AAAA:BBBB:CCCC", false},
		{"empty", "", false},
		{"plaintext", "client-secret", false},
		{"three-parts", "v1:AAAA:BBBB", false},
		{"five-parts", "v1:AAAA:BBBB:CCCC:DDDD", false},
		{"bad-version-prefix", "x1:AAAA:BBBB:CCCC", false},
		{"url-safe-base64", "v1:AA-A:BB_B:CCCC", false},
		{"ciphertext-with-space", "v1:AAAA:BBBB:CC CC", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEnvelopeToken(tt.token); got != tt.want {
				t.Errorf("IsEnvelopeToken(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestDecryptEnvelopeField_RoundTrip(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)

	token, err := encryptor.EncryptField("s3cr3t-value")
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}
	plaintext, err := encryptor.DecryptEnvelopeField(token)
	if err != nil {
		t.Fatalf("DecryptEnvelopeField: %v", err)
	}
	if plaintext != "s3cr3t-value" {
		t.Errorf("got %q, want %q", plaintext, "s3cr3t-value")
	}
}

func TestDecryptEnvelopeField_Empty(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)

	plaintext, err := encryptor.DecryptEnvelopeField("")
	if err != nil {
		t.Fatalf("DecryptEnvelopeField empty: %v", err)
	}
	if plaintext != "" {
		t.Errorf("expected empty string, got %q", plaintext)
	}
}

func TestDecryptEnvelopeField_RejectsPlaintext(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)

	_, err := encryptor.DecryptEnvelopeField("legacy-plaintext")
	if !errors.Is(err, ErrNotEnvelope) {
		t.Fatalf("expected ErrNotEnvelope, got %v", err)
	}
}

// legacyDirectKEKToken reproduces the pre-envelope format: base64(nonce+ciphertext)
// encrypted directly with the KEK.
func legacyDirectKEKToken(t *testing.T, plaintext string) string {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	// #nosec G407 -- this reproduces the legacy on-disk format; the fixed nonce
	// only protects a test fixture and must never be copied into production code.
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed)
}

func TestDecryptEnvelopeField_RejectsLegacyToken(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)
	legacy := legacyDirectKEKToken(t, "legacy-secret")

	if _, err := encryptor.DecryptField(legacy); err != nil {
		t.Fatalf("DecryptField must keep legacy compatibility: %v", err)
	}
	if _, err := encryptor.DecryptEnvelopeField(legacy); !errors.Is(err, ErrNotEnvelope) {
		t.Fatalf("expected ErrNotEnvelope for legacy token, got %v", err)
	}
}

func TestDecryptEnvelopeField_WrongKeyFailsClosed(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)
	token, err := encryptor.EncryptField("key-bound-secret")
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}

	otherKey := make([]byte, 32)
	for i := range otherKey {
		otherKey[i] = byte(i + 2)
	}
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(otherKey))
	otherEncryptor := NewFieldEncryptor()

	_, err = otherEncryptor.DecryptEnvelopeField(token)
	if err == nil {
		t.Fatal("expected decryption to fail with a different key")
	}
	if errors.Is(err, ErrNotEnvelope) {
		t.Fatalf("envelope-shaped token must not be reported as plaintext: %v", err)
	}
}
