package webauthn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfiguredAllowedOriginsAddsDevelopmentConsoleOrigin(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("ISSUER_URL", "http://localhost:8000")

	origins := ConfiguredAllowedOrigins()
	assert.Contains(t, origins, "http://localhost:8000")
	assert.Contains(t, origins, defaultDevelopmentConsoleOrigin)
}

func TestConfiguredAllowedOriginsDoesNotAddConsoleOriginOutsideDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://iam.example.com")
	t.Setenv("ISSUER_URL", "https://iam.example.com")

	origins := ConfiguredAllowedOrigins()
	assert.Contains(t, origins, "https://iam.example.com")
	assert.NotContains(t, origins, defaultDevelopmentConsoleOrigin)
}

// 开发期默认来源只有一个入口：管理员面与自服务/登录面都调用该函数。
func TestWithDevelopmentOriginsIsTheSharedDevelopmentDefault(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	base := []string{"https://iam.example.com"}
	origins := WithDevelopmentOrigins(base)
	assert.Contains(t, origins, defaultDevelopmentConsoleOrigin)
	assert.Equal(t, []string{"https://iam.example.com"}, base, "caller slice must not be mutated")
}

func TestWithDevelopmentOriginsInjectsNothingOutsideDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	assert.Empty(t, WithDevelopmentOrigins(nil))
	assert.Equal(t, []string{"https://iam.example.com"}, WithDevelopmentOrigins([]string{"https://iam.example.com"}))
}
