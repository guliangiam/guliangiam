package ssf

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRevocationEvent(t *testing.T) {
	now := time.Date(2026, 10, 20, 1, 2, 3, 0, time.UTC)
	event, err := NewRevocationEvent("https://op.example.com", "org-1", "user-1", "sess-1", "logout", now)
	require.NoError(t, err)
	assert.Equal(t, "ssf_event", event.AggregateType)
	assert.Equal(t, "sess-1", event.AggregateID)
	require.NotNil(t, event.OrganizationID)
	assert.Equal(t, "org-1", *event.OrganizationID)
	var payload struct {
		Type    string         `json:"type"`
		Issuer  string         `json:"issuer"`
		Subject map[string]any `json:"subject"`
		Data    map[string]any `json:"eventData"`
	}
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	assert.Equal(t, EventSessionRevoked, payload.Type)
	assert.Equal(t, "https://op.example.com", payload.Issuer)
	assert.Equal(t, "user-1", payload.Subject["sub"])
	assert.Equal(t, "sess-1", payload.Subject["session_id"])
	assert.Equal(t, "logout", payload.Data["reason"])
}

func TestNewRevocationEvent_RequiresIssuerAndOrg(t *testing.T) {
	_, err := NewRevocationEvent("", "org-1", "user", "sess", "logout", time.Now())
	require.Error(t, err)
	_, err = NewRevocationEvent("https://op.example.com", "", "user", "sess", "logout", time.Now())
	require.Error(t, err)
}
