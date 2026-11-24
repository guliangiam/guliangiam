package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
)

type fakeProviderTokenCipher struct {
	sealed  string
	opened  string
	encrypt bool
}

func (f *fakeProviderTokenCipher) EncryptField(plaintext string) (string, error) {
	f.sealed = "sealed:" + plaintext
	return f.sealed, nil
}

func (f *fakeProviderTokenCipher) DecryptField(ciphertext string) (string, error) {
	f.opened = ciphertext
	return "token-from:" + ciphertext, nil
}

func TestSocialSessionDeviceInfoSealsAccessToken(t *testing.T) {
	cipher := &fakeProviderTokenCipher{}
	service := Service{ProviderTokenCipher: cipher}
	provider := &providerdomain.Provider{ID: "provider-1", ProviderType: providerdomain.ProviderCustom}
	userInfo := &SocialUserInfo{AccessToken: "upstream-token"}

	info := service.socialSessionDeviceInfo(context.Background(), provider, userInfo)
	assert.Equal(t, "provider-1", info[sessiondomain.DeviceInfoProviderID])
	assert.Equal(t, "sealed:upstream-token", info[sessiondomain.DeviceInfoProviderAccessToken])
	assert.Equal(t, "sealed:upstream-token", cipher.sealed)
}

func TestSocialSessionDeviceInfoOmitsTokenWithoutCipher(t *testing.T) {
	service := Service{}
	provider := &providerdomain.Provider{ID: "provider-1", ProviderType: providerdomain.ProviderCustom}

	info := service.socialSessionDeviceInfo(context.Background(), provider, &SocialUserInfo{AccessToken: "upstream-token"})
	assert.Equal(t, "provider-1", info[sessiondomain.DeviceInfoProviderID])
	assert.NotContains(t, info, sessiondomain.DeviceInfoProviderAccessToken)
}

func TestEnsureProviderUsable(t *testing.T) {
	assert.NoError(t, ensureProviderUsable(&providerdomain.Provider{Name: "github", Enabled: true}))
	assert.Error(t, ensureProviderUsable(&providerdomain.Provider{Name: "github", Enabled: false}))
	assert.Error(t, ensureProviderUsable(&providerdomain.Provider{Name: "github", Enabled: true, State: "Disabled"}))
}
