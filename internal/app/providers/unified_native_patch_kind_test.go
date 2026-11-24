package providers

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 回归：PATCH 回传厂商写法 providerType（如 AlibabaSMS）时，kind 必须与创建
// 路径同样走 canonicalIdentityKind；裸 ToLower 会派生出闭集外的 alibabasms，
// 导致 validateIdentityKind 422，编辑保存必败。
func TestApplyProviderPatch_KindMatchesCreateCanonicalization(t *testing.T) {
	cases := []struct {
		providerType string
		wantKind     domain.Kind
	}{
		{"AlibabaSMS", domain.Kind("alibaba_sms")},
		{"AzureAD", domain.Kind("azure_ad")},
		{"TwilioSMS", domain.Kind("twilio_sms")},
		{"Google", domain.Kind("google")},
	}
	for _, c := range cases {
		t.Run(c.providerType, func(t *testing.T) {
			provider := &Provider{
				Registration: domain.Registration{
					OrganizationID: "org-1",
					Name:           "p1",
					DisplayName:    "P1",
					Family:         domain.FamilyIdentity,
					Kind:           canonicalIdentityKind(c.providerType),
					Status:         domain.StatusActive,
				},
				Detail: &domain.Provider{
					Category:     domain.CategorySMS,
					ProviderType: domain.ProviderType(c.providerType),
				},
			}
			err := applyProviderPatch(provider, PatchProviderCommand{ProviderType: stringPtr(c.providerType)})
			require.NoError(t, err)
			assert.Equal(t, c.wantKind, provider.Registration.Kind)
			require.NoError(t, validateIdentityKind(provider.Registration.Kind))
		})
	}
}
