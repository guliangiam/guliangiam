package crypto

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// 算法切换重叠窗口：AES 旧密文必须在新进程（写 SM4）中继续可读，
// 新写入使用 SM4，两种密文同一进程内共存。
func TestEnvelopeEncryptor_AlgorithmSwitchOverlap(t *testing.T) {
	aesKey := make([]byte, 32)
	for i := range aesKey {
		aesKey[i] = byte(i + 1)
	}
	sm4Key := make([]byte, 16)
	for i := range sm4Key {
		sm4Key[i] = byte(i + 40)
	}

	aesEncryptor, err := NewEnvelopeEncryptorWithAlgorithm(
		map[int][]byte{1: aesKey}, 1, AlgorithmAES256GCM)
	if err != nil {
		t.Fatalf("AES encryptor: %v", err)
	}
	oldToken, err := aesEncryptor.Encrypt("legacy-aes-secret")
	if err != nil {
		t.Fatalf("encrypt with AES: %v", err)
	}
	if !strings.HasPrefix(oldToken, "v1-aes:") {
		t.Fatalf("AES token = %q, want v1-aes: header", oldToken)
	}

	switched, err := NewEnvelopeEncryptorMultiAlgorithm(map[Algorithm]map[int][]byte{
		AlgorithmSM4GCM:    {1: sm4Key},
		AlgorithmAES256GCM: {1: aesKey},
	}, 1, AlgorithmSM4GCM)
	if err != nil {
		t.Fatalf("multi-algorithm encryptor: %v", err)
	}

	plaintext, err := switched.Decrypt(oldToken)
	if err != nil {
		t.Fatalf("decrypt AES token after switching to SM4: %v", err)
	}
	if plaintext != "legacy-aes-secret" {
		t.Fatalf("decrypted = %q, want legacy-aes-secret", plaintext)
	}

	newToken, err := switched.Encrypt("new-sm4-secret")
	if err != nil {
		t.Fatalf("encrypt with SM4: %v", err)
	}
	if !strings.HasPrefix(newToken, "v1-sm4:") {
		t.Fatalf("SM4 token = %q, want v1-sm4: header", newToken)
	}
	if plaintext, err = switched.Decrypt(newToken); err != nil || plaintext != "new-sm4-secret" {
		t.Fatalf("decrypt SM4 token = %q, %v", plaintext, err)
	}
}

// ENCRYPTION_ALGORITHM 从 aes-256-gcm 切到 sm4-gcm 的完整装配路径：
// 旧密文靠 ENCRYPTION_KEY_COMPAT 解密，新密文使用 SM4。
func TestFieldEncryptor_AlgorithmSwitchViaEnv(t *testing.T) {
	aesKey := make([]byte, 32)
	for i := range aesKey {
		aesKey[i] = byte(i + 7)
	}
	sm4Key := make([]byte, 16)
	for i := range sm4Key {
		sm4Key[i] = byte(i + 60)
	}

	t.Setenv("ENCRYPTION_ALGORITHM", "aes-256-gcm")
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(aesKey))
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("ENCRYPTION_KEY_COMPAT", "")
	t.Setenv("IS_PRODUCTION", "false")

	aesEncryptor := NewFieldEncryptor()
	oldToken, err := aesEncryptor.EncryptField("legacy-env-secret")
	if err != nil {
		t.Fatalf("encrypt with AES: %v", err)
	}

	t.Setenv("ENCRYPTION_ALGORITHM", "sm4-gcm")
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(sm4Key))
	t.Setenv("ENCRYPTION_KEY_COMPAT", base64.StdEncoding.EncodeToString(aesKey))

	switched := NewFieldEncryptor()
	plaintext, err := switched.DecryptField(oldToken)
	if err != nil {
		t.Fatalf("decrypt AES token after ENCRYPTION_ALGORITHM=sm4-gcm: %v", err)
	}
	if plaintext != "legacy-env-secret" {
		t.Fatalf("decrypted = %q, want legacy-env-secret", plaintext)
	}

	newToken, err := switched.EncryptField("new-env-secret")
	if err != nil {
		t.Fatalf("encrypt with SM4: %v", err)
	}
	if !strings.HasPrefix(newToken, "v1-sm4:") {
		t.Fatalf("SM4 token = %q, want v1-sm4: header", newToken)
	}
	if plaintext, err = switched.DecryptField(newToken); err != nil || plaintext != "new-env-secret" {
		t.Fatalf("decrypt SM4 token = %q, %v", plaintext, err)
	}
}

// SM4 能力不可用时拒绝启用，不得静默退化为 AES。
func TestFieldEncryptor_SM4UnavailablePanics(t *testing.T) {
	original := sm4CapabilityProbe
	sm4CapabilityProbe = func() error { return errors.New("sm4 unavailable") }
	defer func() { sm4CapabilityProbe = original }()

	t.Setenv("ENCRYPTION_ALGORITHM", "sm4-gcm")
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(make([]byte, 16)))
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_COMPAT", "")
	t.Setenv("IS_PRODUCTION", "false")

	defer func() {
		if recover() == nil {
			t.Error("expected panic when SM4 is unavailable (no silent downgrade to AES)")
		}
	}()

	NewFieldEncryptor()
}
