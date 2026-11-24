package cred

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSHA256Provider_Algorithm(t *testing.T) {
	p := NewSHA256Provider()
	assert.Equal(t, AlgoSHA256, p.Algorithm())
}

func TestSHA256Provider_Hash(t *testing.T) {
	p := NewSHA256Provider()

	hash, err := p.Hash("my-secure-password")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Contains(t, hash, "{SHA256}")
}

func TestSHA256Provider_Hash_DifferentPasswords(t *testing.T) {
	p := NewSHA256Provider()

	hash1, err := p.Hash("password1")
	require.NoError(t, err)

	hash2, err := p.Hash("password2")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestSHA256Provider_Hash_SamePasswordDifferentSalts(t *testing.T) {
	p := NewSHA256Provider()

	hash1, err := p.Hash("same-password")
	require.NoError(t, err)

	hash2, err := p.Hash("same-password")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "same password should produce different hashes due to random salt")
}

func TestSHA256Provider_Verify_Correct(t *testing.T) {
	p := NewSHA256Provider()

	password := "verify-me"
	hash, err := p.Hash(password)
	require.NoError(t, err)

	ok, err := p.Verify(password, hash)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestSHA256Provider_Verify_Incorrect(t *testing.T) {
	p := NewSHA256Provider()

	hash, err := p.Hash("correct-password")
	require.NoError(t, err)

	ok, err := p.Verify("wrong-password", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSHA256Provider_Verify_InvalidFormat(t *testing.T) {
	p := NewSHA256Provider()

	ok, err := p.Verify("password", "{SHA256}invalid")
	require.Error(t, err)
	assert.False(t, ok)
}
