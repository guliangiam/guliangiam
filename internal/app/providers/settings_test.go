package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

func TestProviderSettings_RoundTrip(t *testing.T) {
	provider := &domain.Provider{
		EmailRegex:                  "^.*@example\\.com$",
		EnablePkce:                  true,
		EnableProxy:                 true,
		HttpHeaders:                 map[string]string{"X-Tenant": "acme"},
		RequireMessageAuthenticator: true,
		EnableSignAuthnRequest:      true,
		CustomLogoutUrl:             "https://idp.example.com/logout",
		CustomLogo:                  "https://cdn.example.com/logo.svg",
		Cert:                        "-----BEGIN CERTIFICATE-----",
		State:                       "active",
	}

	settings := settingsFromProvider(provider)
	config, err := settings.config("provider-1")
	require.NoError(t, err)
	assert.Equal(t, settingsConfigType, config.ConfigType)

	decoded, err := decodeProviderSettings(config)
	require.NoError(t, err)

	restored := &domain.Provider{}
	decoded.applyTo(restored)
	assert.Equal(t, provider.EmailRegex, restored.EmailRegex)
	assert.True(t, restored.EnablePkce)
	assert.True(t, restored.EnableProxy)
	assert.Equal(t, provider.HttpHeaders, restored.HttpHeaders)
	assert.True(t, restored.RequireMessageAuthenticator)
	assert.True(t, restored.EnableSignAuthnRequest)
	assert.Equal(t, provider.CustomLogoutUrl, restored.CustomLogoutUrl)
	assert.Equal(t, provider.CustomLogo, restored.CustomLogo)
	assert.Equal(t, provider.Cert, restored.Cert)
	assert.Equal(t, provider.State, restored.State)
}

func TestProviderSettings_ZeroValueHasNoValues(t *testing.T) {
	assert.False(t, providerSettings{}.hasValues())
	assert.True(t, providerSettings{EmailRegex: "."}.hasValues())
}
