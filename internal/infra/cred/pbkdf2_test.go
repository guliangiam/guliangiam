package cred

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPBKDF2Provider_Algorithm(t *testing.T) {
	p := PBKDF2Provider{Iterations: 1000, SaltSize: 16, KeyLen: 32}
	assert.Equal(t, AlgoPBKDF2, p.Algorithm())
}

func TestPBKDF2Provider_Hash(t *testing.T) {
	p := PBKDF2Provider{Iterations: 1000, SaltSize: 16, KeyLen: 32}

	hash, err := p.Hash("my-secure-password")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Contains(t, hash, "pbkdf2-sha256$i=")
	assert.Contains(t, hash, "$l=")
}

func TestPBKDF2Provider_Hash_DifferentPasswords(t *testing.T) {
	p := PBKDF2Provider{Iterations: 1000, SaltSize: 16, KeyLen: 32}

	hash1, err := p.Hash("password1")
	require.NoError(t, err)

	hash2, err := p.Hash("password2")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestPBKDF2Provider_Verify_Correct(t *testing.T) {
	p := PBKDF2Provider{Iterations: 1000, SaltSize: 16, KeyLen: 32}

	password := "verify-me"
	hash, err := p.Hash(password)
	require.NoError(t, err)

	ok, err := p.Verify(password, hash)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestPBKDF2Provider_Verify_Incorrect(t *testing.T) {
	p := PBKDF2Provider{Iterations: 1000, SaltSize: 16, KeyLen: 32}

	hash, err := p.Hash("correct-password")
	require.NoError(t, err)

	ok, err := p.Verify("wrong-password", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPBKDF2Provider_Verify_InvalidFormat(t *testing.T) {
	p := PBKDF2Provider{Iterations: 1000, SaltSize: 16, KeyLen: 32}

	ok, err := p.Verify("password", "invalid-hash")
	require.Error(t, err)
	assert.False(t, ok)
}

func TestPBKDF2Provider_Verify_InvalidIterations(t *testing.T) {
	p := PBKDF2Provider{Iterations: 1000, SaltSize: 16, KeyLen: 32}

	ok, err := p.Verify("password", "pbkdf2-sha256$i=notanumber$l=c2FsdA$a2V5")
	require.Error(t, err)
	assert.False(t, ok)
}
