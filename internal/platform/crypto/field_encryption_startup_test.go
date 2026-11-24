package crypto

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

func clearFieldEncryptionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("ENCRYPTION_KEY_COMPAT", "")
	t.Setenv("ENCRYPTION_ALGORITHM", "")
	t.Setenv("CRYPTO_PROFILE", "")
}

// R8: 启动路径必须用校验入口构造 FieldEncryptor，非法 base64 在启动期失败，
// 而不是等到首次字段读写才 panic。
func TestNewFieldEncryptorChecked_RejectsInvalidBase64Key(t *testing.T) {
	clearFieldEncryptionEnv(t)
	const malformed = "not-base64-!!-key-material"
	t.Setenv("ENCRYPTION_KEY_CURRENT", malformed)

	encryptor, err := NewFieldEncryptorChecked()
	if err == nil {
		t.Fatal("invalid base64 key must fail startup validation")
	}
	if encryptor != nil {
		t.Fatal("invalid base64 key must not return an encryptor")
	}
	if !strings.Contains(err.Error(), "ENCRYPTION_KEY_CURRENT") ||
		!strings.Contains(err.Error(), "base64") {
		t.Fatalf("err = %q, want the offending variable and the base64 reason", err.Error())
	}
	if strings.Contains(err.Error(), malformed) {
		t.Fatalf("err = %q, must not echo key material", err.Error())
	}
}

// R8: 错误长度必须在启动期被拒绝。默认档是 AES-256，16 字节的 KEK 不合法。
func TestNewFieldEncryptorChecked_RejectsWrongKeyLength(t *testing.T) {
	clearFieldEncryptionEnv(t)
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(make([]byte, 16)))

	encryptor, err := NewFieldEncryptorChecked()
	if err == nil {
		t.Fatal("a 16-byte key under AES-256 must fail startup validation")
	}
	if encryptor != nil {
		t.Fatal("wrong-length key must not return an encryptor")
	}
	for _, want := range []string{"ENCRYPTION_KEY_CURRENT", "exactly 32 bytes", "got 16 bytes"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %q, want substring %q", err.Error(), want)
		}
	}
}

// R8: default 档显式 sm4-gcm 但 SM4 实现不可用时，启动期返回错误且不静默降级。
func TestNewFieldEncryptorChecked_RejectsSM4WhenUnavailable(t *testing.T) {
	clearFieldEncryptionEnv(t)
	t.Setenv("ENCRYPTION_ALGORITHM", "sm4-gcm")
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(make([]byte, 16)))

	originalProbe := sm4CapabilityProbe
	t.Cleanup(func() { sm4CapabilityProbe = originalProbe })
	sm4CapabilityProbe = func() error { return errors.New("sm4 unavailable") }

	encryptor, err := NewFieldEncryptorChecked()
	if err == nil {
		t.Fatal("sm4-gcm with an unavailable SM4 implementation must fail startup validation")
	}
	if encryptor != nil {
		t.Fatal("unavailable SM4 must not return an AES encryptor (no silent downgrade)")
	}
	if !strings.Contains(err.Error(), "SM4") {
		t.Fatalf("err = %q, want the SM4 reason", err.Error())
	}
}

// R8: 启动期构造并发布后，懒加载入口必须复用同一实例，不能再构造第二份。
func TestInitFieldEncryptor_ReusesStartupInstance(t *testing.T) {
	clearFieldEncryptionEnv(t)
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)

	globalEncryptorMu.Lock()
	previous := globalEncryptor
	globalEncryptor = nil
	globalEncryptorMu.Unlock()
	t.Cleanup(func() {
		globalEncryptorMu.Lock()
		globalEncryptor = previous
		globalEncryptorMu.Unlock()
	})

	initialized, err := InitFieldEncryptor()
	if err != nil {
		t.Fatalf("InitFieldEncryptor: %v", err)
	}
	if got := GlobalFieldEncryptor(); got != initialized {
		t.Fatal("GlobalFieldEncryptor must reuse the instance constructed at startup")
	}
}
