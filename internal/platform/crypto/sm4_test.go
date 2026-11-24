package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSM4GCM_Roundtrip 验证 SM4-GCM envelope 加密/解密 roundtrip
func TestSM4GCM_Roundtrip(t *testing.T) {
	keks := map[int][]byte{
		1: make([]byte, 16), // SM4 = 16-byte
	}
	enc, err := NewEnvelopeEncryptorWithAlgorithm(keks, 1, AlgorithmSM4GCM)
	require.NoError(t, err)

	token, err := enc.Encrypt("hello sm4 world")
	require.NoError(t, err)
	assert.True(t, len(token) > 20)

	plain, err := enc.Decrypt(token)
	require.NoError(t, err)
	assert.Equal(t, "hello sm4 world", plain)
}

// TestSM4GCM_MultipleVersions 验证 SM4 支持多 KEK 版本
func TestSM4GCM_MultipleVersions(t *testing.T) {
	keks := map[int][]byte{
		0: make([]byte, 16),
		1: make([]byte, 16),
	}
	enc, err := NewEnvelopeEncryptorWithAlgorithm(keks, 1, AlgorithmSM4GCM)
	require.NoError(t, err)

	token, err := enc.Encrypt("multi version sm4")
	require.NoError(t, err)

	plain, err := enc.Decrypt(token)
	require.NoError(t, err)
	assert.Equal(t, "multi version sm4", plain)
	assert.Equal(t, AlgorithmSM4GCM, enc.Algorithm())
}

// TestSM4_KeySizeValidation SM4 key 必须是 16 bytes，32-byte（AES-256 长度）应该报错
func TestSM4_KeySizeValidation(t *testing.T) {
	_, err := NewEnvelopeEncryptorWithAlgorithm(
		map[int][]byte{1: make([]byte, 32)}, 1, AlgorithmSM4GCM)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be exactly 16 bytes")
}

// TestSM4_AES_Default 不传算法时默认 AES-256（向后兼容）
func TestSM4_AES_Default(t *testing.T) {
	keks := map[int][]byte{1: make([]byte, 32)}
	enc, err := NewEnvelopeEncryptor(keks, 1) // 旧构造函数
	require.NoError(t, err)

	token, err := enc.Encrypt("default aes")
	require.NoError(t, err)
	plain, err := enc.Decrypt(token)
	require.NoError(t, err)
	assert.Equal(t, "default aes", plain)

	// 算法字段确实是 AES
	assert.Equal(t, AlgorithmAES256GCM, enc.Algorithm())
}

// TestSM4_CrossAlgorithmDecrypt 不同算法的 token 不能互相解
func TestSM4_CrossAlgorithmDecrypt(t *testing.T) {
	aesEnc, _ := NewEnvelopeEncryptor(
		map[int][]byte{1: make([]byte, 32)}, 1)
	sm4Enc, _ := NewEnvelopeEncryptorWithAlgorithm(
		map[int][]byte{1: make([]byte, 16)}, 1, AlgorithmSM4GCM)

	aesToken, err := aesEnc.Encrypt("aes secret")
	require.NoError(t, err)
	_, err = sm4Enc.Decrypt(aesToken)
	require.Error(t, err) // 不同算法 key size 不同 → Decrypt 会失败

	sm4Token, err := sm4Enc.Encrypt("sm4 secret")
	require.NoError(t, err)
	_, err = aesEnc.Decrypt(sm4Token)
	require.Error(t, err)
}

// TestAlgorithmKeySize 验证 algorithmKeySize 映射
func TestAlgorithmKeySize(t *testing.T) {
	sz, err := algorithmKeySize(AlgorithmAES256GCM)
	assert.NoError(t, err)
	assert.Equal(t, 32, sz)

	sz, err = algorithmKeySize(AlgorithmSM4GCM)
	assert.NoError(t, err)
	assert.Equal(t, 16, sz)

	_, err = algorithmKeySize(Algorithm(99))
	assert.Error(t, err)
}

// TestAlgName 验证 algName 映射
func TestAlgName(t *testing.T) {
	assert.Equal(t, "AES-256-GCM", algName(AlgorithmAES256GCM))
	assert.Equal(t, "SM4-GCM", algName(AlgorithmSM4GCM))
	assert.Equal(t, "unknown", algName(Algorithm(99)))
}

// TestSM4_AES_BothOnSameKEKVersion 同一 version map 结构，验证只有 key size 区分
func TestSM4_AES_BothOnSameKEKVersion(t *testing.T) {
	aesEnc, err := NewEnvelopeEncryptor(
		map[int][]byte{0: make([]byte, 32), 1: make([]byte, 32)}, 1)
	require.NoError(t, err)
	assert.Equal(t, AlgorithmAES256GCM, aesEnc.Algorithm())
	assert.Equal(t, 32, aesEnc.dekSize)

	sm4Enc, err := NewEnvelopeEncryptorWithAlgorithm(
		map[int][]byte{0: make([]byte, 16), 1: make([]byte, 16)}, 1, AlgorithmSM4GCM)
	require.NoError(t, err)
	assert.Equal(t, AlgorithmSM4GCM, sm4Enc.Algorithm())
	assert.Equal(t, 16, sm4Enc.dekSize)
}

// TestSM4_Reencrypt SM4 token 重新加密应该保持 SM4
func TestSM4_Reencrypt(t *testing.T) {
	enc, err := NewEnvelopeEncryptorWithAlgorithm(
		map[int][]byte{0: make([]byte, 16), 1: make([]byte, 16)}, 1, AlgorithmSM4GCM)
	require.NoError(t, err)

	token, err := enc.Encrypt("reencrypt me")
	require.NoError(t, err)

	newToken, changed, err := enc.Reencrypt(token)
	require.NoError(t, err)
	assert.False(t, changed) // 已经是 current version

	plain, err := enc.Decrypt(newToken)
	require.NoError(t, err)
	assert.Equal(t, "reencrypt me", plain)
}

// TestSM4_PayloadHandling 空字符串处理
func TestSM4_PayloadHandling(t *testing.T) {
	enc, err := NewEnvelopeEncryptorWithAlgorithm(
		map[int][]byte{1: make([]byte, 16)}, 1, AlgorithmSM4GCM)
	require.NoError(t, err)

	token, err := enc.Encrypt("")
	require.NoError(t, err)
	assert.Empty(t, token)

	plain, err := enc.Decrypt("")
	require.NoError(t, err)
	assert.Empty(t, plain)
}

// TestSM4_CurrentVersion 验证 CurrentVersion 和 HasVersion
func TestSM4_CurrentVersion(t *testing.T) {
	enc, err := NewEnvelopeEncryptorWithAlgorithm(
		map[int][]byte{0: make([]byte, 16), 2: make([]byte, 16)}, 2, AlgorithmSM4GCM)
	require.NoError(t, err)

	assert.Equal(t, 2, enc.CurrentVersion())
	assert.True(t, enc.HasVersion(0))
	assert.True(t, enc.HasVersion(2))
	assert.False(t, enc.HasVersion(1))
}
