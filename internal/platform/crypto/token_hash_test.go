package crypto

import (
	"testing"
)

func TestHashTokenValue_Properties(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		token := "gl_pat_abc123"
		h1 := HashTokenValue(token)
		h2 := HashTokenValue(token)
		if h1 != h2 {
			t.Error("same input should always produce same hash")
		}
	})

	t.Run("collision_resistance", func(t *testing.T) {
		tokens := []string{
			"gl_pat_token_a",
			"gl_pat_token_b",
			"gl_pat_token_c",
			"gl_prt_refresh_1",
			"gl_prt_refresh_2",
		}
		hashes := make(map[string]string)
		for _, token := range tokens {
			hash := HashTokenValue(token)
			if existing, ok := hashes[hash]; ok {
				t.Errorf("hash collision: %q and %q produce same hash", existing, token)
			}
			hashes[hash] = token
		}
	})

	t.Run("sha256_hex_length", func(t *testing.T) {
		hash := HashTokenValue("test")
		if len(hash) != 64 {
			t.Errorf("hex-encoded SHA-256 length = %d, want 64", len(hash))
		}
	})

	t.Run("empty_token", func(t *testing.T) {
		hash := HashTokenValue("")
		if len(hash) != 64 {
			t.Errorf("empty token hash length = %d, want 64", len(hash))
		}
	})

	t.Run("long_token", func(t *testing.T) {
		longToken := make([]byte, 1024)
		for i := range longToken {
			longToken[i] = byte(i % 256)
		}
		hash := HashTokenValue(string(longToken))
		if len(hash) != 64 {
			t.Errorf("long token hash length = %d, want 64", len(hash))
		}
	})
}
