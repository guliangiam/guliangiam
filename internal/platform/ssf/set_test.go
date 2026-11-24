package ssf

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSigner struct {
	claims    map[string]any
	tokenType string
}

func (f *fakeSigner) SignToken(claims map[string]any, tokenType string) (string, error) {
	f.claims = claims
	f.tokenType = tokenType
	return "signed-token", nil
}

func TestBuildClaims_RFC8417Shape(t *testing.T) {
	issuedAt := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	claims, err := BuildClaims(Event{
		Type:      EventSessionRevoked,
		Issuer:    "https://op.example.com",
		Audience:  "https://receiver.example.com",
		EventID:   "evt-1",
		IssuedAt:  issuedAt,
		Subject:   map[string]any{"sub": "user-1", "session_id": "sid-1"},
		EventData: map[string]any{"reason": "logout"},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://op.example.com", claims["iss"])
	assert.Equal(t, "evt-1", claims["jti"])
	assert.Equal(t, issuedAt.Unix(), claims["iat"])
	assert.Equal(t, "https://receiver.example.com", claims["aud"])
	events, ok := claims["events"].(map[string]any)
	require.True(t, ok)
	payload, ok := events[EventSessionRevoked].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "logout", payload["reason"])
	subject, ok := payload["subject"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user-1", subject["sub"])
}

func TestBuildClaims_RequiresTypeAndIssuerAndID(t *testing.T) {
	_, err := BuildClaims(Event{Issuer: "iss", EventID: "id"})
	require.Error(t, err)
	_, err = BuildClaims(Event{Type: EventSessionRevoked, EventID: "id"})
	require.Error(t, err)
	_, err = BuildClaims(Event{Type: EventSessionRevoked, Issuer: "iss"})
	require.Error(t, err)
}

func TestSignEvent_UsesSecEventJWT(t *testing.T) {
	signer := &fakeSigner{}
	token, err := SignEvent(signer, Event{
		Type: EventCredentialChange, Issuer: "https://op.example.com", EventID: "evt-2",
	})
	require.NoError(t, err)
	assert.Equal(t, "signed-token", token)
	assert.Equal(t, "secevent+jwt", signer.tokenType)
	require.Contains(t, signer.claims, "events")
}
