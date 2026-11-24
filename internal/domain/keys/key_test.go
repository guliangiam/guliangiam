package keys

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyJSONOmitsPrivateKey(t *testing.T) {
	raw, err := json.Marshal(Key{PrivateKey: "sensitive-private-material"})
	require.NoError(t, err)
	require.NotContains(t, string(raw), "sensitive-private-material")
	require.NotContains(t, string(raw), "privateKey")
}
