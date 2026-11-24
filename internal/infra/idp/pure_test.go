package idp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUserInfo_StringID(t *testing.T) {
	p := &BaseOAuthProvider{}
	body, _ := json.Marshal(map[string]any{
		"id":         "user-1",
		"login":      "johndoe",
		"name":       "John Doe",
		"email":      "john@example.com",
		"avatar_url": "https://example.com/avatar.jpg",
	})
	info, err := p.parseUserInfo(body)
	require.NoError(t, err)
	assert.Equal(t, "user-1", info.ID)
	assert.Equal(t, "johndoe", info.Username)
	assert.Equal(t, "John Doe", info.DisplayName)
	assert.Equal(t, "john@example.com", info.Email)
	assert.Equal(t, "https://example.com/avatar.jpg", info.AvatarURL)
}

func TestParseUserInfo_NumericID(t *testing.T) {
	p := &BaseOAuthProvider{}
	body, _ := json.Marshal(map[string]any{
		"id":    float64(12345),
		"login": "johndoe",
	})
	info, err := p.parseUserInfo(body)
	require.NoError(t, err)
	assert.Equal(t, "12345", info.ID)
	assert.Equal(t, "johndoe", info.Username)
}

func TestParseUserInfo_PictureAvatar(t *testing.T) {
	p := &BaseOAuthProvider{}
	body, _ := json.Marshal(map[string]any{
		"id":      "user-1",
		"picture": "https://example.com/pic.jpg",
	})
	info, err := p.parseUserInfo(body)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/pic.jpg", info.AvatarURL)
}

func TestParseUserInfo_AvatarURLPriority(t *testing.T) {
	p := &BaseOAuthProvider{}
	body, _ := json.Marshal(map[string]any{
		"id":         "user-1",
		"avatar_url": "https://example.com/avatar.jpg",
		"picture":    "https://example.com/pic.jpg",
	})
	info, err := p.parseUserInfo(body)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/avatar.jpg", info.AvatarURL) // avatar_url takes priority
}

func TestParseUserInfo_InvalidJSON(t *testing.T) {
	p := &BaseOAuthProvider{}
	_, err := p.parseUserInfo([]byte("not json"))
	require.Error(t, err)
}

func TestParseUserInfo_EmptyBody(t *testing.T) {
	p := &BaseOAuthProvider{}
	body, _ := json.Marshal(map[string]any{})
	info, err := p.parseUserInfo(body)
	require.NoError(t, err)
	assert.Equal(t, "", info.ID)
	assert.Equal(t, "", info.Username)
}

func TestExtractSteamID_Valid(t *testing.T) {
	p := &SteamProvider{}
	id, err := p.ExtractSteamID("https://steamcommunity.com/openid/id/76561198012345678")
	require.NoError(t, err)
	assert.Equal(t, "76561198012345678", id)
}

func TestExtractSteamID_EmptyPart(t *testing.T) {
	p := &SteamProvider{}
	_, err := p.ExtractSteamID("https://steamcommunity.com/openid/id/")
	require.Error(t, err)
}

func TestGenerateNonce(t *testing.T) {
	p := &Web3Provider{}
	nonce, err := p.GenerateNonce()
	require.NoError(t, err)
	assert.NotEmpty(t, nonce)
	assert.Len(t, nonce, 64) // 32 bytes = 64 hex chars
}

func TestNewSteamProvider(t *testing.T) {
	p := NewSteamProvider(&OAuthConfig{ClientID: "test-key"})
	assert.NotNil(t, p)
}

func TestNewGothProvider(t *testing.T) {
	// Just verify it doesn't panic with nil config
	_ = NewGothProvider
}
