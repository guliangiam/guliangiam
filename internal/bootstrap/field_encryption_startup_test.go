package bootstrap

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

func clearFieldEncryptionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("CRYPTO_PROFILE", "")
	t.Setenv("ENCRYPTION_ALGORITHM", "")
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("ENCRYPTION_KEY_COMPAT", "")
}

// R8: 非法密钥内容必须在创建监听器之前中止启动，而不是首次字段读写才 panic。
func TestApplyFieldEncryption_RejectsInvalidKeyMaterial(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantSub []string
	}{
		{
			name:    "invalid base64",
			key:     "not-base64-!!-key-material",
			wantSub: []string{"ENCRYPTION_KEY_CURRENT", "base64"},
		},
		{
			name:    "wrong length",
			key:     base64.StdEncoding.EncodeToString(make([]byte, 16)),
			wantSub: []string{"exactly 32 bytes", "got 16 bytes"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearFieldEncryptionEnv(t)
			t.Setenv("ENCRYPTION_KEY_CURRENT", tt.key)

			if err := applyFieldEncryption(); err == nil {
				t.Fatal("applyFieldEncryption must return a startup error for invalid key material")
			} else {
				for _, want := range tt.wantSub {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("err = %q, want substring %q", err.Error(), want)
					}
				}
				if strings.Contains(err.Error(), tt.key) {
					t.Fatalf("err = %q, must not echo key material", err.Error())
				}
			}
		})
	}
}

// R8: 构造阶段的任何失败（例如 default 档 sm4-gcm 但 SM4 不可用）都必须让
// 启动闸门返回错误；真实能力探测由 platform/crypto 的单测覆盖。
func TestApplyFieldEncryption_StopsStartupWhenConstructorFails(t *testing.T) {
	original := constructFieldEncryptor
	t.Cleanup(func() { constructFieldEncryptor = original })
	constructFieldEncryptor = func() (*platformcrypto.FieldEncryptor, error) {
		return nil, errors.New("ENCRYPTION_ALGORITHM=SM4 but the implementation is unavailable")
	}

	if err := applyFieldEncryption(); err == nil {
		t.Fatal("a constructor failure must abort startup")
	}
}

// R8: KMS provider 必须在配置校验之前解析（Vault 托管密钥不能被误判为 env
// 缺失），字段加密器必须在能力裁决之后、监听器创建之前构造。
func TestApplyStartupConfig_OrdersProviderBeforeKeyCheck(t *testing.T) {
	resetCryptoHealth(t)
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)

	originalInit, originalValidate := initKMSProvider, validateConfig
	originalProfile, originalConstruct := validateCryptoProfile, constructFieldEncryptor
	t.Cleanup(func() {
		initKMSProvider = originalInit
		validateConfig = originalValidate
		validateCryptoProfile = originalProfile
		constructFieldEncryptor = originalConstruct
	})

	var order []string
	initKMSProvider = func(name string) string {
		order = append(order, "kms")
		return name
	}
	validateConfig = func() {
		order = append(order, "config")
	}
	validateCryptoProfile = func() (platformcrypto.ProfileReport, error) {
		order = append(order, "profile")
		return platformcrypto.ProfileReport{
			Profile:   string(platformcrypto.ProfileDefault),
			Available: true,
		}, nil
	}
	constructFieldEncryptor = func() (*platformcrypto.FieldEncryptor, error) {
		order = append(order, "field-encryption")
		return platformcrypto.NewFieldEncryptor(), nil
	}

	if err := applyStartupConfig(); err != nil {
		t.Fatalf("applyStartupConfig: %v", err)
	}
	want := []string{"kms", "config", "profile", "field-encryption"}
	if len(order) != len(want) {
		t.Fatalf("startup order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("startup order = %v, want %v", order, want)
		}
	}
}

// R8: 字段加密器构造失败时，applyStartupConfig 必须把错误上抛给 runServer，
// 后者在 NewHTTPServer 之前返回 1（不创建监听器）。
func TestApplyStartupConfig_AbortsOnFieldEncryptionFailure(t *testing.T) {
	resetCryptoHealth(t)
	originalInit, originalValidate := initKMSProvider, validateConfig
	originalProfile, originalConstruct := validateCryptoProfile, constructFieldEncryptor
	t.Cleanup(func() {
		initKMSProvider = originalInit
		validateConfig = originalValidate
		validateCryptoProfile = originalProfile
		constructFieldEncryptor = originalConstruct
	})

	initKMSProvider = func(string) string { return "stub" }
	validateConfig = func() {}
	validateCryptoProfile = func() (platformcrypto.ProfileReport, error) {
		return platformcrypto.ProfileReport{
			Profile:   string(platformcrypto.ProfileDefault),
			Available: true,
		}, nil
	}
	constructFieldEncryptor = func() (*platformcrypto.FieldEncryptor, error) {
		return nil, errors.New("field encryption key is invalid")
	}

	if err := applyStartupConfig(); err == nil {
		t.Fatal("applyStartupConfig must propagate the field-encryption startup error")
	}
}
