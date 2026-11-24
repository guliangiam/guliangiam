package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DB-061 / BE-B-21: the advisory key mapping must stay stable across releases.
// Changing the fold (e.g. masking the high bit) would change every lock key,
// so during a rolling deployment old and new replicas would no longer exclude
// each other on the same syncer/config.
func TestAdvisoryKey_StablePinnedValues(t *testing.T) {
	assert.Equal(t, int64(2051018801021146027), advisoryKey("ldapsync:config:abc"))
	assert.Equal(t, int64(6004466405093138898), advisoryKey("syncers:syncer:abc"))
}

func TestAdvisoryKey_DeterministicAndDistinct(t *testing.T) {
	a1 := advisoryKey("ldapsync:config:1")
	a2 := advisoryKey("ldapsync:config:1")
	assert.Equal(t, a1, a2, "same name must fold to the same key")
	assert.NotEqual(t, a1, advisoryKey("ldapsync:config:2"))
	assert.NotEqual(t, advisoryKey("ldapsync:config:1"), advisoryKey("syncers:syncer:1"),
		"the namespace prefix must separate the two lock families")
}

// A nil pool (MySQL branch) must surface as an error, never a panic in a
// background goroutine, and must not report the lock as acquired.
func TestAdvisoryLockProvider_NilPool_Errors(t *testing.T) {
	p := NewAdvisoryLockProvider(nil)
	release, acquired, err := p.TryLock(context.Background(), "ldapsync:config:1")
	require.Error(t, err)
	assert.False(t, acquired)
	assert.Nil(t, release)
}
