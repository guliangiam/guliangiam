package config

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

func TestValidateOrPanic_MissingDSN(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_ENV", "development")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "should panic when DSN is missing")
}

func TestValidateOrPanic_ValidConfig(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db")
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("APP_ENV", "development")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)

	assert.NotPanics(t, func() {
		ValidateOrPanic()
	}, "should not panic with valid config")
}

func TestValidateOrPanic_MissingEncryptionKey(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db")
	t.Setenv("APP_ENV", "development")
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "should panic without an encryption key in every environment (DB-058)")
}

func TestValidateOrPanic_InvalidPort(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("HTTP_PORT", "not-a-number")
	t.Setenv("APP_ENV", "development")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "should panic with invalid port")
}

func TestValidateOrPanic_ProductionMissingIssuerURL(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("APP_ENV", "production")
	t.Setenv("ISSUER_URL", "")
	t.Setenv("SESSION_HMAC_KEY", "0123456789012345678901234567890123456789")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "should panic in production without ISSUER_URL")
}

func TestValidateOrPanic_ISProductionContradictsAppEnv(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("APP_ENV", "development")
	t.Setenv("IS_PRODUCTION", "true")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "should panic when IS_PRODUCTION contradicts APP_ENV (ADR-021)")
}

func TestValidateOrPanic_ProductionValid(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("APP_ENV", "production")
	t.Setenv("IS_PRODUCTION", "true")
	t.Setenv("ISSUER_URL", "https://auth.example.com")
	t.Setenv("METRICS_SCRAPE_TOKEN", "scrape-token")
	t.Setenv("SESSION_HMAC_KEY", "0123456789012345678901234567890123456789")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("PASSWORD_HISTORY_PEPPER", "deployment-specific-pepper")

	assert.NotPanics(t, func() {
		ValidateOrPanic()
	}, "should not panic with a complete production config")
}

func TestValidateOrPanic_ProductionMissingPasswordHistoryPepper(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("APP_ENV", "production")
	t.Setenv("ISSUER_URL", "https://auth.example.com")
	t.Setenv("METRICS_SCRAPE_TOKEN", "scrape-token")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("PASSWORD_HISTORY_PEPPER", "")

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "should panic in production without PASSWORD_HISTORY_PEPPER")

	issue := PasswordHistoryPepperIssue()
	assert.Contains(t, issue, "PASSWORD_HISTORY_PEPPER")
	assert.Contains(t, issue, "required in production")
	assert.NotContains(t, issue, DefaultPasswordHistoryPepper,
		"the error message must not echo the pepper value")
}

func TestValidateOrPanic_ProductionPasswordHistoryPepperSentinel(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("APP_ENV", "production")
	t.Setenv("ISSUER_URL", "https://auth.example.com")
	t.Setenv("METRICS_SCRAPE_TOKEN", "scrape-token")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("PASSWORD_HISTORY_PEPPER", DefaultPasswordHistoryPepper)

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "should panic when production uses the built-in development pepper")

	issue := PasswordHistoryPepperIssue()
	assert.Contains(t, issue, "must not be the built-in development pepper")
	assert.NotContains(t, issue, DefaultPasswordHistoryPepper,
		"the error message must not echo the pepper value")
}

func TestValidateOrPanic_DevelopmentMissingPasswordHistoryPepper(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("APP_ENV", "development")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("PASSWORD_HISTORY_PEPPER", "")

	assert.NotPanics(t, func() {
		ValidateOrPanic()
	}, "non-production must keep the existing warn-only pepper fallback")
}

func TestPasswordHistoryPepperIssue(t *testing.T) {
	tests := []struct {
		name    string
		appEnv  string
		pepper  string
		wantMsg string
	}{
		{
			name:    "production missing pepper",
			appEnv:  "production",
			pepper:  "",
			wantMsg: "PASSWORD_HISTORY_PEPPER is required in production",
		},
		{
			name:    "production whitespace pepper",
			appEnv:  "production",
			pepper:  "   ",
			wantMsg: "PASSWORD_HISTORY_PEPPER is required in production",
		},
		{
			name:    "production development sentinel",
			appEnv:  "production",
			pepper:  DefaultPasswordHistoryPepper,
			wantMsg: "PASSWORD_HISTORY_PEPPER must not be the built-in development pepper",
		},
		{
			name:   "production deployment pepper",
			appEnv: "production",
			pepper: "deployment-specific-pepper",
		},
		{
			name:   "development missing pepper",
			appEnv: "development",
			pepper: "",
		},
		{
			name:   "test missing pepper",
			appEnv: "test",
			pepper: "",
		},
		{
			name:   "staging missing pepper",
			appEnv: "staging",
			pepper: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tt.appEnv)
			t.Setenv("PASSWORD_HISTORY_PEPPER", tt.pepper)

			got := PasswordHistoryPepperIssue()
			if tt.wantMsg == "" {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, got, tt.wantMsg)
			if tt.pepper != "" {
				assert.NotContains(t, got, tt.pepper,
					"the error message must not echo the pepper value")
			}
		})
	}
}

func TestValidateOrPanic_ProductionMissingMetricsToken(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("APP_ENV", "production")
	t.Setenv("ISSUER_URL", "https://auth.example.com")
	t.Setenv("METRICS_SCRAPE_TOKEN", "")
	t.Setenv("SESSION_HMAC_KEY", "0123456789012345678901234567890123456789")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "should panic in production without METRICS_SCRAPE_TOKEN (SEC-A4)")
}

func TestWebAuthnRequireUV(t *testing.T) {
	for raw, want := range map[string]bool{
		"true":  true,
		"TRUE":  true,
		"1":     true,
		"false": false,
		"0":     false,
		"":      false,
		// 写错的值按「未开启」处理，但启动校验会让它直接失败（见下一个用例）。
		"yes": false,
	} {
		t.Setenv("WEBAUTHN_REQUIRE_UV", raw)
		if got := WebAuthnRequireUV(); got != want {
			t.Errorf("WebAuthnRequireUV() with %q = %v, want %v", raw, got, want)
		}
	}
}

func TestValidateOrPanic_InvalidWebAuthnRequireUV(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("APP_ENV", "development")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	// 拼错的布尔开关等于静默关掉 UV 策略：必须在启动时失败，而不是当成 false 放行。
	t.Setenv("WEBAUTHN_REQUIRE_UV", "yes")

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "should panic when WEBAUTHN_REQUIRE_UV is not a boolean literal")
}

// R8: KMS_PROVIDER=vault 时，密钥存在性校验必须走已选 provider。env 未设置但
// Vault 提供了 encryption_key_current 时不得误报“ENCRYPTION_KEY 缺失”。
// 这里用 httptest 模拟 Vault KV v2，不连接真实 Vault。
func TestValidateOrPanic_VaultProviderSuppliesEncryptionKey(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("KMS_PROVIDER", "vault")
	t.Setenv("VAULT_TOKEN", "test-token")
	t.Setenv("VAULT_KEYS_PATH", "secrets/data/guliangiam/test/keys")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "test-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"data":{"encryption_key_current":"`+cryptotest.Key+`"}}}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("VAULT_ADDR", server.URL)

	previous := currentKeyProvider
	t.Cleanup(func() { currentKeyProvider = previous })
	// bootstrap 的顺序：先解析 KMS provider，再执行配置校验。
	InitKMSProvider("vault")
	if got := EncryptionKeyCurrent(); got != cryptotest.Key {
		t.Fatalf("Vault-provided key = %q, want the fixture key", got)
	}

	assert.NotPanics(t, func() {
		ValidateOrPanic()
	}, "a Vault-hosted KEK must not be reported as a missing environment variable")
}

// R8: provider 选择一致不等于跳过校验。Vault 读不到密钥时仍必须 fail-fast。
func TestValidateOrPanic_VaultProviderWithoutKeyStillFails(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("POSTGRES_DSN", "postgres://localhost/db")
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("KMS_PROVIDER", "vault")
	t.Setenv("VAULT_TOKEN", "test-token")
	t.Setenv("VAULT_KEYS_PATH", "secrets/data/guliangiam/test/keys")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"data":{}}}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("VAULT_ADDR", server.URL)

	previous := currentKeyProvider
	t.Cleanup(func() { currentKeyProvider = previous })
	InitKMSProvider("vault")

	assert.Panics(t, func() {
		ValidateOrPanic()
	}, "an empty Vault key set must still fail the encryption-key presence check")
}
