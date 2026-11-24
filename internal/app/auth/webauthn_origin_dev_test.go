package auth

import (
	"testing"

	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
	"github.com/stretchr/testify/assert"
)

// 自服务/登录面与管理员面必须使用同一份开发期来源默认值，否则 CORS 为空时会
// 出现「管理员能注册、登录不能用」的割裂。
func TestWebAuthnAllowedOriginsMatchesAdminFaceInDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("ISSUER_URL", "http://localhost:8000")

	service := &Service{IssuerURL: "http://localhost:8000"}

	assert.ElementsMatch(t, platformwebauthn.ConfiguredAllowedOrigins(), service.webAuthnAllowedOrigins())
	assert.Contains(t, service.webAuthnAllowedOrigins(), "http://localhost:7101")
}

func TestWebAuthnAllowedOriginsInjectsNoDevelopmentOriginInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://iam.example.com")
	t.Setenv("ISSUER_URL", "https://iam.example.com")

	service := &Service{
		IssuerURL:              "https://iam.example.com",
		WebAuthnAllowedOrigins: []string{"https://iam.example.com"},
	}

	origins := service.webAuthnAllowedOrigins()
	assert.Contains(t, origins, "https://iam.example.com")
	assert.NotContains(t, origins, "http://localhost:7101")
}
