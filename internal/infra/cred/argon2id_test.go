package cred

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgon2idProvider_Algorithm(t *testing.T) {
	p := NewArgon2idProvider()
	assert.Equal(t, AlgoArgon2id, p.Algorithm())
}

func TestArgon2idProvider_Hash(t *testing.T) {
	p := NewArgon2idProvider()

	hash, err := p.Hash("my-secure-password")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Contains(t, hash, "$argon2id$")
}

func TestArgon2idProvider_Hash_DifferentPasswords(t *testing.T) {
	p := NewArgon2idProvider()

	hash1, err := p.Hash("password1")
	require.NoError(t, err)

	hash2, err := p.Hash("password2")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "different passwords should produce different hashes")
}

func TestArgon2idProvider_Hash_SamePasswordDifferentSalts(t *testing.T) {
	p := NewArgon2idProvider()

	hash1, err := p.Hash("same-password")
	require.NoError(t, err)

	hash2, err := p.Hash("same-password")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "same password should produce different hashes due to random salt")
}

func TestArgon2idProvider_Verify_Correct(t *testing.T) {
	p := NewArgon2idProvider()

	password := "verify-me"
	hash, err := p.Hash(password)
	require.NoError(t, err)

	ok, err := p.Verify(password, hash)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestArgon2idProvider_Verify_Incorrect(t *testing.T) {
	p := NewArgon2idProvider()

	hash, err := p.Hash("correct-password")
	require.NoError(t, err)

	ok, err := p.Verify("wrong-password", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestArgon2idProvider_Verify_EmptyPassword(t *testing.T) {
	p := NewArgon2idProvider()

	hash, err := p.Hash("")
	require.NoError(t, err)

	ok, err := p.Verify("", hash)
	require.NoError(t, err)
	assert.True(t, ok)
}
