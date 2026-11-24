package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 5869 Appendix A.1, Test Case 1 (SHA-256).
func TestHKDF_SHA256_RFC5869TestCase1(t *testing.T) {
	ikm := bytes.Repeat([]byte{0x0b}, 22)
	salt, err := hex.DecodeString("000102030405060708090a0b0c")
	require.NoError(t, err)
	info, err := hex.DecodeString("f0f1f2f3f4f5f6f7f8f9")
	require.NoError(t, err)

	prk := Extract(ikm, salt)
	assert.Equal(t, "077709362c2e32df0ddc3f0dc47bba6390b6c73bb50f9c3122ec844ad7c2b3e5",
		hex.EncodeToString(prk))

	okm, err := Expand(prk, info, 42)
	require.NoError(t, err)
	assert.Equal(t, "3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf34007208d5b887185865",
		hex.EncodeToString(okm))
}

func TestHKDF_SHA256_ExpandRejectsInvalidLength(t *testing.T) {
	prk := Extract([]byte("secret"), nil)
	_, err := Expand(prk, nil, -1)
	require.Error(t, err)
	_, err = Expand(prk, nil, hkdfMaxExpandLength+1)
	require.Error(t, err)

	empty, err := Expand(prk, nil, 0)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// Expansion must stay a prefix-stable stream: a longer request starts with the
// shorter one, which is what the WebPush CEK/nonce derivation relies on.
func TestHKDF_SHA256_ExpandIsPrefixStable(t *testing.T) {
	prk := Extract([]byte("secret"), []byte("salt"))
	short, err := Expand(prk, []byte("info"), 16)
	require.NoError(t, err)
	long, err := Expand(prk, []byte("info"), 32)
	require.NoError(t, err)
	assert.Equal(t, short, long[:16])
}
