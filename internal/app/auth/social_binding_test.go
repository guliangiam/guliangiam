package auth

import (
	"testing"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateProviderEmail(t *testing.T) {
	provider := &providerdomain.Provider{EmailRegex: `^[^@]+@example\.com$`}
	require.NoError(t, validateProviderEmail(provider, "alice@example.com"))

	err := validateProviderEmail(provider, "alice@other.com")
	require.Error(t, err)
	assert.Equal(t, 401, appErrorStatus(t, err))

	require.NoError(t, validateProviderEmail(&providerdomain.Provider{}, "anything"))
	require.NoError(t, validateProviderEmail(nil, "anything"))
}

func TestValidateProviderEmailRejectsPatternInvalidAtLoginTime(t *testing.T) {
	err := validateProviderEmail(&providerdomain.Provider{EmailRegex: "["}, "alice@example.com")
	require.Error(t, err)
	assert.Equal(t, 500, appErrorStatus(t, err))
}

func TestProviderItemForApplicationMatchesIDOrName(t *testing.T) {
	provider := &providerdomain.Provider{ID: "provider-1", Name: "github"}
	application := &applicationdomain.Application{ProviderItems: []applicationdomain.ProviderItem{
		{Provider: "other", CanSignIn: true},
		{Provider: "provider-1", CanSignIn: true},
	}}

	assert.Equal(t, "provider-1", providerItemForApplication(application, provider).Provider)

	application.ProviderItems = []applicationdomain.ProviderItem{{Provider: "github", CanSignIn: true}}
	assert.Equal(t, "github", providerItemForApplication(application, provider).Provider)

	application.ProviderItems = nil
	assert.Nil(t, providerItemForApplication(application, provider))
	assert.Nil(t, providerItemForApplication(nil, provider))
}

func TestSocialBindingRules(t *testing.T) {
	require.Equal(t, []string{"Email"}, socialBindingRules(nil))
	require.Equal(t, []string{"Email"}, socialBindingRules(&applicationdomain.ProviderItem{}))
	require.Empty(t, socialBindingRules(&applicationdomain.ProviderItem{BindingRule: []string{}}))
	require.Equal(t,
		[]string{"email", "phone", "name"},
		socialBindingRules(&applicationdomain.ProviderItem{BindingRule: []string{" Email ", "PHONE", "name", ""}}),
	)
}

func TestSocialProviderConfigCarriesRuntimeOptions(t *testing.T) {
	provider := &providerdomain.Provider{
		Category:     providerdomain.CategoryOAuth,
		ProviderType: providerdomain.ProviderGitHub,
		ClientID:     "client",
		ClientSecret: "secret",
		EnablePkce:   true,
		EnableProxy:  true,
		HttpHeaders:  map[string]string{"X-Tenant": "acme"},
	}
	config := socialProviderConfig(provider, "https://iam.example.com/callback")
	assert.True(t, config.EnablePkce)
	assert.True(t, config.EnableProxy)
	assert.Equal(t, "acme", config.HTTPHeaders["X-Tenant"])
	assert.Equal(t, "https://iam.example.com/callback", config.RedirectURI)
}

func appErrorStatus(t *testing.T, err error) int {
	t.Helper()
	type statusCoder interface{ HTTPStatus() int }
	coder, ok := err.(statusCoder)
	require.True(t, ok, "error %T does not expose HTTP status", err)
	return coder.HTTPStatus()
}
