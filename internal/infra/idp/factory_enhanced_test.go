package idp

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactory_GetProvider_MoreOAuthTypes(t *testing.T) {
	f := NewFactory()
	types := []string{
		"wecom", "infoflow", "web3", "metamask",
		"telegram", "line", "steam", "reddit", "apple",
		"bitbucket", "gitee", "weibo", "qq", "bilibili",
		"douyin", "okta", "auth0", "amazon",
		"wechat_miniprogram", "wechat_mobile",
		"wecom_internal", "wecom_third_party",
		"azuread_b2c", "adfs", "baidu", "kwai",
	}
	for _, pt := range types {
		t.Run(pt, func(t *testing.T) {
			p, err := f.GetProvider(&providers.Provider{
				Category:     "OAuth",
				ProviderType: providers.ProviderType(pt),
			})
			require.NoError(t, err)
			assert.NotNil(t, p)
		})
	}
}

func TestFactory_GetProvider_OIDC(t *testing.T) {
	f := NewFactory()
	p, err := f.GetProvider(&providers.Provider{
		Category:     "OIDC",
		ProviderType: "okta",
	})
	require.NoError(t, err)
	assert.IsType(t, &OktaProvider{}, p)
}

func TestFactory_GetProvider_GenericOIDC(t *testing.T) {
	f := NewFactory()
	for _, providerType := range []string{"oidc", "OIDC"} {
		t.Run(providerType, func(t *testing.T) {
			p, err := f.GetProvider(&providers.Provider{
				Category:     "OIDC",
				ProviderType: providers.ProviderType(providerType),
			})
			require.NoError(t, err)
			assert.IsType(t, &OIDCProvider{}, p)
		})
	}
}

func TestFactory_GetProvider_UnknownOAuthFallsbackToCustom(t *testing.T) {
	f := NewFactory()
	p, err := f.GetProvider(&providers.Provider{
		Category:     "OAuth",
		ProviderType: "unknown_provider",
	})
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestFactory_GetProvider_CaseInsensitive(t *testing.T) {
	f := NewFactory()
	p, err := f.GetProvider(&providers.Provider{
		Category:     "OAUTH",
		ProviderType: "GITHUB",
	})
	require.NoError(t, err)
	assert.NotNil(t, p)
}
