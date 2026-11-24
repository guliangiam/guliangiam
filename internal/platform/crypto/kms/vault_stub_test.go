package kms

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 版本号解析失败时不能把该版本当作「v0」参与最高版本比较，
// 否则 ResolveLatest 可能返回空版本或错误密钥材料。
func TestVaultStubAdapter_ResolveLatest_SkipsUnparsableVersions(t *testing.T) {
	a := NewVaultStubAdapter()
	a.PreseedVersion("key-a", "v1", []byte("one"))
	a.PreseedVersion("key-a", "v2", []byte("two"))
	a.PreseedVersion("key-a", "abc", []byte("bogus"))

	got, err := a.ResolveLatest(context.Background(), "key-a")
	require.NoError(t, err)
	assert.Equal(t, "v2", got.Descriptor.Version)
	assert.Equal(t, []byte("two"), got.Plaintext)
}

func TestVaultStubAdapter_ResolveLatest_AllVersionsUnparsable(t *testing.T) {
	a := NewVaultStubAdapter()
	a.PreseedVersion("key-b", "abc", []byte("bogus"))
	a.PreseedVersion("key-b", "v", []byte("bogus2"))

	_, err := a.ResolveLatest(context.Background(), "key-b")
	require.Error(t, err)
	var notFound *KeyNotFoundError
	require.ErrorAs(t, err, &notFound)
}
