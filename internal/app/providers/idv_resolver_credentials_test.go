package providers

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A configured provider with missing credentials must surface as an actionable
// 422 (not 400/503), and meta.field must name the exact form control to fill so
// the UI can highlight it. Regression for the real-name "服务不可用" masking.
func TestCredentialsFromProvider_IncompleteIsUnprocessable(t *testing.T) {
	cases := []struct {
		name      string
		provider  *domain.Provider
		wantField string
	}{
		{"aliyun missing secret", &domain.Provider{Kind: "aliyun_idv", Admin: "ak"}, "password"},
		{"aliyun missing key", &domain.Provider{Kind: "aliyun_idv", Password: "sk"}, "admin"},
		{"tencent missing secret", &domain.Provider{Kind: "tencent_idv", Admin: "ak"}, "password"},
		{"jumio missing secret", &domain.Provider{Kind: "jumio", ClientID: "tok"}, "clientSecret"},
		{"veriff missing key", &domain.Provider{Kind: "veriff", ClientSecret: "sec"}, "clientId"},
		{"onfido missing token", &domain.Provider{Kind: "onfido"}, "clientId"},
		{"persona missing token", &domain.Provider{Kind: "persona"}, "clientId"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CredentialsFromProvider(tc.provider)
			require.Error(t, err)
			appErr, ok := err.(*apperrors.Error)
			require.True(t, ok, "expected *apperrors.Error, got %T", err)
			assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
			assert.Equal(t, tc.wantField, appErr.Meta["field"])
		})
	}
}

func TestCredentialsFromProvider_CompleteAliyunMapsAdminPassword(t *testing.T) {
	creds, err := CredentialsFromProvider(&domain.Provider{
		Kind:     "aliyun_idv",
		Admin:    "access-key-id",
		Password: "access-key-secret",
	})
	require.NoError(t, err)
	assert.Equal(t, "access-key-id", creds.APIKey)
	assert.Equal(t, "access-key-secret", creds.APISecret)
}
