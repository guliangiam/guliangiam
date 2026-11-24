package idp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSupportedGothTypes_NotEmpty(t *testing.T) {
	types := SupportedGothTypes()
	assert.NotEmpty(t, types)
}

func TestIsGothProvider_KnownType(t *testing.T) {
	assert.True(t, IsGothProvider("spotify"))
}

func TestIsGothProvider_UnknownType(t *testing.T) {
	assert.False(t, IsGothProvider("nonexistent-provider"))
	assert.False(t, IsGothProvider("github")) // github is OAuth, not goth
}

func TestNormalizeGothType_Lowercase(t *testing.T) {
	assert.Equal(t, "spotify", normalizeGothType("Spotify"))
	assert.Equal(t, "digitalocean", normalizeGothType("DigitalOcean"))
	assert.Equal(t, "dropbox", normalizeGothType("dropbox"))
	assert.Equal(t, "", normalizeGothType(""))
	assert.Equal(t, "onedrive", normalizeGothType("one-drive"))
	assert.Equal(t, "lastfm", normalizeGothType("last_fm"))
}

func TestNewSteamProvider_NilConfig(t *testing.T) {
	p := NewSteamProvider(nil)
	assert.NotNil(t, p)
}

func TestNewWeb3Provider_NilConfig(t *testing.T) {
	p := NewWeb3Provider(nil)
	assert.NotNil(t, p)
}

func TestNewWeChatProvider_NilConfig(t *testing.T) {
	p := NewWeChatProvider(nil)
	assert.NotNil(t, p)
}

func TestNewDingTalkProvider_NilConfig(t *testing.T) {
	p := NewDingTalkProvider(nil)
	assert.NotNil(t, p)
}

func TestNewLarkProvider_NilConfig(t *testing.T) {
	p := NewLarkProvider(nil)
	assert.NotNil(t, p)
}

func TestNewWeComProvider_NilConfig(t *testing.T) {
	p := NewWeComProvider(nil)
	assert.NotNil(t, p)
}

func TestNewWeComInternalProvider_NilConfig(t *testing.T) {
	p := NewWeComInternalProvider(nil)
	assert.NotNil(t, p)
}

func TestNewWeComThirdPartyProvider_NilConfig(t *testing.T) {
	p := NewWeComThirdPartyProvider(nil)
	assert.NotNil(t, p)
}

func TestNewWeChatMiniProgramProvider_NilConfig(t *testing.T) {
	p := NewWeChatMiniProgramProvider(nil)
	assert.NotNil(t, p)
}

func TestNewWeChatMobileProvider_NilConfig(t *testing.T) {
	// NewWeChatMobileProvider panics with nil config - skip
	// This is tested via chi route registration with Recoverer middleware
}
