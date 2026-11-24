package cred

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlainProvider_Algorithm(t *testing.T) {
	p := NewPlainProvider()
	assert.Equal(t, AlgoPlain, p.Algorithm())
}

func TestPlainProvider_Hash(t *testing.T) {
	p := NewPlainProvider()

	hash, err := p.Hash("my-password")
	require.NoError(t, err)
	assert.Equal(t, "my-password", hash, "plain hash should return the password as-is")
}

func TestPlainProvider_Hash_Empty(t *testing.T) {
	p := NewPlainProvider()

	hash, err := p.Hash("")
	require.NoError(t, err)
	assert.Equal(t, "", hash)
}

func TestPlainProvider_Verify_Correct(t *testing.T) {
	p := NewPlainProvider()

	ok, err := p.Verify("password", "password")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestPlainProvider_Verify_Incorrect(t *testing.T) {
	p := NewPlainProvider()

	ok, err := p.Verify("wrong", "password")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPlainProvider_Verify_EmptyMatches(t *testing.T) {
	p := NewPlainProvider()

	ok, err := p.Verify("", "")
	require.NoError(t, err)
	assert.True(t, ok)
}
