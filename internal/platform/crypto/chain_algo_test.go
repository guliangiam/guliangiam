package crypto

import (
	"crypto/sha256"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestHashByChainAlgorithm(t *testing.T) {
	data := []byte("audit-chain")

	sha, err := HashByChainAlgorithm(ChainAlgoSHA256, data)
	if err != nil {
		t.Fatalf("sha256: %v", err)
	}
	wantSHA := sha256.Sum256(data)
	if string(sha) != string(wantSHA[:]) {
		t.Fatal("sha256 digest does not match crypto/sha256")
	}

	sm3Digest, err := HashByChainAlgorithm(ChainAlgoSM3, data)
	if err != nil {
		t.Fatalf("sm3: %v", err)
	}
	if string(sm3Digest) != string(SM3Sum(data)) {
		t.Fatal("sm3 digest does not match SM3Sum")
	}

	for _, algo := range []string{"", "md5", "SHA256"} {
		if _, err := HashByChainAlgorithm(algo, data); err == nil {
			t.Fatalf("HashByChainAlgorithm(%q) accepted an unknown algorithm", algo)
		}
	}
}

func TestNormalizeChainAlgorithm(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"legacy empty row maps to sha256", "", ChainAlgoSHA256},
		{"sha256 passes through", ChainAlgoSHA256, ChainAlgoSHA256},
		{"sm3 passes through", ChainAlgoSM3, ChainAlgoSM3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeChainAlgorithm(tc.in)
			if err != nil {
				t.Fatalf("NormalizeChainAlgorithm(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeChainAlgorithm(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	// No trimming or case folding: values outside the schema constraint stay
	// rejected instead of being silently coerced into a supported algorithm.
	for _, in := range []string{"md5", "SHA256", " sm3", "sm3 ", "sha-256", "null"} {
		t.Run("rejects "+in, func(t *testing.T) {
			got, err := NormalizeChainAlgorithm(in)
			if err == nil {
				t.Fatalf("NormalizeChainAlgorithm(%q) accepted an unknown algorithm as %q", in, got)
			}
			if got != "" {
				t.Fatalf("NormalizeChainAlgorithm(%q) returned %q on error", in, got)
			}
			if code := apperrors.FromError(err).Code; code != apperrors.CodeInvalidArgument {
				t.Fatalf("NormalizeChainAlgorithm(%q) code = %s, want %s", in, code, apperrors.CodeInvalidArgument)
			}
			if msg := err.Error(); !strings.Contains(msg, "unsupported audit chain algorithm") || !strings.Contains(msg, in) {
				t.Fatalf("NormalizeChainAlgorithm(%q) message %q lost the algorithm name", in, msg)
			}
		})
	}
}

func TestDefaultChainAlgorithm(t *testing.T) {
	t.Setenv("CRYPTO_PROFILE", "default")
	if got := DefaultChainAlgorithm(); got != ChainAlgoSHA256 {
		t.Fatalf("default profile = %s, want sha256", got)
	}

	t.Setenv("CRYPTO_PROFILE", "gm")
	if got := DefaultChainAlgorithm(); got != ChainAlgoSM3 {
		t.Fatalf("gm profile = %s, want sm3", got)
	}
}
