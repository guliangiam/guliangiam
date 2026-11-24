package crypto

import (
	"os"
	"testing"
)

// BenchmarkHashTokenValue benchmarks SHA-256 token hashing.
// Spec §4.1: JWT签发 < 10ms → token hash must be sub-microsecond.
func BenchmarkHashTokenValue(b *testing.B) {
	token := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImFiYzEyMyJ9.eyJzdWIiOiJ1c2VyLTEyMzQiLCJvcmdfaWQiOiJvcmctNTY3OCIsInNjb3BlIjoib3BlbmlkIHByb2ZpbGUiLCJleHAiOjE3MDAwMDAwMDAsImlhdCI6MTcwMDAwMDAwMCwianRpIjoiY29ycmVsYXRpb24taWQifQ.signature-placeholder"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		HashTokenValue(token)
	}
}

// BenchmarkHashTokenValueShort benchmarks SHA-256 on a short token (authorization code).
func BenchmarkHashTokenValueShort(b *testing.B) {
	token := "abc123-short-auth-code"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		HashTokenValue(token)
	}
}

// BenchmarkFieldEncryption benchmarks AES-256-GCM field encryption.
// This tests the hot-path for encrypting sensitive fields like client_secret.
func BenchmarkFieldEncryption(b *testing.B) {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		os.Setenv("ENCRYPTION_KEY", "dGVzdC1rZXktZm9yLWJlbmNobWFyay10ZXN0aW5nMzI=")
		defer os.Unsetenv("ENCRYPTION_KEY")
	}

	encryptor := NewFieldEncryptor()
	plaintext := "my-super-secret-client-secret-value-12345"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := encryptor.EncryptField(plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFieldDecryption benchmarks AES-256-GCM field decryption.
func BenchmarkFieldDecryption(b *testing.B) {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		os.Setenv("ENCRYPTION_KEY", "dGVzdC1rZXktZm9yLWJlbmNobWFyay10ZXN0aW5nMzI=")
		defer os.Unsetenv("ENCRYPTION_KEY")
	}

	encryptor := NewFieldEncryptor()
	plaintext := "my-super-secret-client-secret-value-12345"
	ciphertext, err := encryptor.EncryptField(plaintext)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := encryptor.DecryptField(ciphertext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFieldEncryptionRoundtrip benchmarks the full encrypt→decrypt cycle.
func BenchmarkFieldEncryptionRoundtrip(b *testing.B) {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		os.Setenv("ENCRYPTION_KEY", "dGVzdC1rZXktZm9yLWJlbmNobWFyay10ZXN0aW5nMzI=")
		defer os.Unsetenv("ENCRYPTION_KEY")
	}

	encryptor := NewFieldEncryptor()
	plaintext := "my-super-secret-client-secret-value-12345"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ct, err := encryptor.EncryptField(plaintext)
		if err != nil {
			b.Fatal(err)
		}
		_, err = encryptor.DecryptField(ct)
		if err != nil {
			b.Fatal(err)
		}
	}
}
