package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// RFC 4231 Test Case 1 与 OpenSSL 4.0.3 `openssl dgst -sm3 -mac HMAC`
// 交叉核对过的 SM3-HMAC 向量。
func TestDeriveKey_KnownVectors(t *testing.T) {
	key := bytes.Repeat([]byte{0x0b}, 20)
	data := []byte("Hi There")

	tests := []struct {
		algorithm string
		expected  string
	}{
		{DeriveAlgorithmHMACSHA256, "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"},
		{DeriveAlgorithmHMACSM3, "51b00d1fb49832bfb01c3ce27848e59f871d9ba938dc563b338ca964755cce70"},
	}
	for _, tt := range tests {
		t.Run(tt.algorithm, func(t *testing.T) {
			got, err := DeriveKey(tt.algorithm, key, data)
			if err != nil {
				t.Fatalf("DeriveKey(%s) error = %v", tt.algorithm, err)
			}
			if hex.EncodeToString(got) != tt.expected {
				t.Fatalf("DeriveKey(%s) = %s, want %s", tt.algorithm, hex.EncodeToString(got), tt.expected)
			}
		})
	}
}

func TestDeriveKey_RejectsUnknownAlgorithm(t *testing.T) {
	_, err := DeriveKey("md5", []byte("key"), []byte("data"))
	if err == nil {
		t.Fatal("expected unknown algorithm to fail")
	}
}

func TestDeriveKey_RejectsEmptyKey(t *testing.T) {
	_, err := DeriveKey(DeriveAlgorithmHMACSHA256, nil, []byte("data"))
	if err == nil {
		t.Fatal("expected empty key to fail")
	}
}
