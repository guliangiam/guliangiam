package cred

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBcryptProvider_Algorithm(t *testing.T) {
	p := NewBcryptProvider()
	assert.Equal(t, AlgoBcrypt, p.Algorithm())
}

func TestBcryptProvider_DefaultCost(t *testing.T) {
	p := NewBcryptProvider()
	assert.Equal(t, 10, p.Cost)
}

func TestBcryptProvider_Hash(t *testing.T) {
	p := NewBcryptProvider()

	hash, err := p.Hash("my-secure-password")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Contains(t, hash, "$2a$")
}

func TestBcryptProvider_Hash_DifferentPasswords(t *testing.T) {
	p := NewBcryptProvider()

	hash1, err := p.Hash("password1")
	require.NoError(t, err)

	hash2, err := p.Hash("password2")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestBcryptProvider_Hash_SamePasswordDifferentSalts(t *testing.T) {
	p := NewBcryptProvider()

	hash1, err := p.Hash("same-password")
	require.NoError(t, err)

	hash2, err := p.Hash("same-password")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestBcryptProvider_Verify_Correct(t *testing.T) {
	p := NewBcryptProvider()

	password := "verify-me"
	hash, err := p.Hash(password)
	require.NoError(t, err)

	ok, err := p.Verify(password, hash)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestBcryptProvider_Verify_Incorrect(t *testing.T) {
	p := NewBcryptProvider()

	hash, err := p.Hash("correct-password")
	require.NoError(t, err)

	ok, err := p.Verify("wrong-password", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestBcryptProvider_Verify_InvalidHash(t *testing.T) {
	p := NewBcryptProvider()

	ok, err := p.Verify("password", "not-a-bcrypt-hash")
	require.NoError(t, err)
	assert.False(t, ok)
}
