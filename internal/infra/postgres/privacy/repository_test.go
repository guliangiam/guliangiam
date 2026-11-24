package privacy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeEventPayload(t *testing.T) {
	encoded, err := encodeEventPayload(nil)
	require.NoError(t, err)
	assert.Nil(t, encoded)

	encoded, err = encodeEventPayload(map[string]any{"from": "received", "to": "verifying"})
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, "received", decoded["from"])
	assert.Equal(t, "verifying", decoded["to"])
}
