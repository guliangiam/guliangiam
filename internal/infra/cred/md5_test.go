package cred

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMD5Provider_Algorithm(t *testing.T) {
	p := NewMD5Provider()
	assert.Equal(t, AlgoMD5, p.Algorithm())
}

func TestMD5Provider_Hash(t *testing.T) {
	p := NewMD5Provider()

	hash, err := p.Hash("hello")
	require.NoError(t, err)
	assert.Equal(t, "5d41402abc4b2a76b9719d911017c592", hash)
}

func TestMD5Provider_Hash_Empty(t *testing.T) {
	p := NewMD5Provider()

	hash, err := p.Hash("")
	require.NoError(t, err)
	assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", hash)
	assert.Len(t, hash, 32, "MD5 hash should be 32 hex characters")
}

func TestMD5Provider_Hash_Deterministic(t *testing.T) {
	p := NewMD5Provider()

	hash1, err := p.Hash("same-password")
	require.NoError(t, err)

	hash2, err := p.Hash("same-password")
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "MD5 is unsalted, same password should produce same hash")
}

func TestMD5Provider_Verify_Correct(t *testing.T) {
	p := NewMD5Provider()

	password := "verify-me"
	hash, err := p.Hash(password)
	require.NoError(t, err)

	ok, err := p.Verify(password, hash)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMD5Provider_Verify_Incorrect(t *testing.T) {
	p := NewMD5Provider()

	hash, err := p.Hash("correct-password")
	require.NoError(t, err)

	ok, err := p.Verify("wrong-password", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}
