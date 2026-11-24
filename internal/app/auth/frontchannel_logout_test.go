package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFrontChannelURL(t *testing.T) {
	got, err := buildFrontChannelURL("https://rp.example.com/frontchannel-logout", "https://op.example.com", "sid-1", true)
	require.NoError(t, err)
	assert.Equal(t, "https://rp.example.com/frontchannel-logout?iss=https%3A%2F%2Fop.example.com&sid=sid-1", got)

	got, err = buildFrontChannelURL("https://rp.example.com/fc", "https://op.example.com", "sid-1", false)
	require.NoError(t, err)
	assert.NotContains(t, got, "sid=")
}

func TestBuildFrontChannelURL_RejectsRelative(t *testing.T) {
	_, err := buildFrontChannelURL("/fc", "https://op.example.com", "sid-1", true)
	require.Error(t, err)
}
