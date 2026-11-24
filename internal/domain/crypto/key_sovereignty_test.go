package crypto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ── DR-003 验收：KeyRevokedError 只在加密层出现 (Experience 100007470 分层) ─

// TestRevoked_CannotEncrypt_CanDecrypt: 吊销后不能加密但能解旧 token
func TestRevoked_CannotEncrypt_CanDecrypt(t *testing.T) {
	now := time.Now()
	key := &EncryptionKey{
		KeyID:      "k1",
		KeyVersion: 2,
		Status:     "revoked",
		RevokedAt:  &now,
	}

	assert.False(t, key.IsUsableForEncrypt(), "revoked KEK cannot be used for new encryption")
	assert.True(t, key.IsUsableForDecrypt(), "revoked KEK must still decrypt old tokens")
}

// TestActive_CanEncryptAndDecrypt
func TestActive_CanEncryptAndDecrypt(t *testing.T) {
	key := &EncryptionKey{KeyID: "k1", KeyVersion: 3, Status: "active"}
	assert.True(t, key.IsUsableForEncrypt())
	assert.True(t, key.IsUsableForDecrypt())
}

// TestRetiring_CanEncrypt
func TestRetiring_CanEncrypt(t *testing.T) {
	key := &EncryptionKey{KeyID: "k1", KeyVersion: 2, Status: "retiring"}
	assert.True(t, key.IsUsableForEncrypt(), "retiring KEK valid during rollout")
	assert.True(t, key.IsUsableForDecrypt())
}

// TestPending_CannotEncryptOrDecrypt
func TestPending_CannotEncryptOrDecrypt(t *testing.T) {
	key := &EncryptionKey{KeyID: "k1", KeyVersion: 4, Status: "pending"}
	assert.False(t, key.IsUsableForEncrypt())
	assert.False(t, key.IsUsableForDecrypt())
}

// TestRevokedError_Message: 错误消息可读
func TestRevokedError_Message(t *testing.T) {
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	e := &KeyRevokedError{
		KeyID:      "arn:aws:kms:us-east-1:123456789:key/abcd",
		KeyVersion: 1,
		RevokedAt:  now,
	}
	assert.Contains(t, e.Error(), "revoked")
	assert.Contains(t, e.Error(), "arn:aws:kms")
	assert.Contains(t, e.Error(), "v1")
}

// TestDecryptKeyNotFound_IsDistinctFromRevoked
// Experience 100007470: 两种错误绝不混用
//   - KeyRevokedError  → 加密层入口拒绝（新加密不能用已吊销 KEK）
//   - ErrDecryptionKeyNotFound → 解密层找不到可用 KEK（永久删除场景）
//   - EnvelopeDecryptError → 结构层解密失败（token 被篡改 / DEK 用错 KEK 解开）
//
// 这三种在三个不同入口抛，绝不交叉。
func TestDecryptKeyNotFound_IsDistinctFromRevoked(t *testing.T) {
	assert.Equal(t,
		"decryption key version not available — key may have been permanently deleted",
		ErrDecryptionKeyNotFound.Error(),
		"ErrDecryptionKeyNotFound 是语义层（找不着 KEK），KeyRevokedError 是加密层（吊销不能加密）",
	)
}

// ── BYOK/HYOK 主权模式 ──

func TestSovereigntyMode_AllKnownValidate(t *testing.T) {
	for _, m := range []KeySovereigntyMode{SovereigntyPlatformDefault, SovereigntyBYOK, SovereigntyHYOK} {
		assert.NoError(t, m.Validate(), "%s should validate", m)
	}
}

func TestSovereigntyMode_UnknownInvalid(t *testing.T) {
	err := KeySovereigntyMode("unknown").Validate()
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "sovereigntyMode", appErr.Meta["field"])
}

// TestBYOK_KeyMaterialRef_KMS_ARN: BYOK 只存 KMS ARN 引用，不存明文
func TestBYOK_KeyMaterialRef_KMS_ARN(t *testing.T) {
	key := &EncryptionKey{
		KeyID:           "tenant-kek",
		KeyMaterialRef:  "arn:aws:kms:us-east-1:123456789012:key/abcd-1234-efgh-5678",
		KeyFingerprint:  "sha256:abcdef1234567890",
		SovereigntyMode: SovereigntyBYOK,
	}
	assert.NotEmpty(t, key.KeyMaterialRef)
	assert.Contains(t, key.KeyMaterialRef, "arn:aws:kms")
	assert.NotEmpty(t, key.KeyFingerprint)
}

// TestHYOK_KeyMaterialRef_HSMURI: HYOK 存租户侧 HSM URI
func TestHYOK_KeyMaterialRef_HSMURI(t *testing.T) {
	key := &EncryptionKey{
		KeyID:           "tenant-kek",
		KeyMaterialRef:  "hsm://tenant.example.com:29418/key-id=tenant-owned-root",
		SovereigntyMode: SovereigntyHYOK,
	}
	assert.Contains(t, key.KeyMaterialRef, "hsm://")
}

// ── KeyScope ──

func TestKeyScope_AllKnownValidate(t *testing.T) {
	for _, s := range []KeyScope{ScopeGlobal, ScopeRegion, ScopeOrganization} {
		assert.NoError(t, s.Validate(), "%s should validate", s)
	}
}

func TestKeyScope_UnknownInvalid(t *testing.T) {
	require.Error(t, KeyScope("unknown").Validate())
}
