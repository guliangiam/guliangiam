package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F2-01c Batch 1 pins the AAD-bound read columns to the sealed-payload
// registry so the application read paths cannot drift from the
// rotation/verify ownership.
func TestSealedPayloadAAD_MatchesRegistry(t *testing.T) {
	cases := []struct {
		name   string
		column EncryptedColumn
	}{
		{"llm_providers.api_key_sealed", LLMProviderAPIKeySealedColumn},
		{"pay_channel_configs.config_sealed", PayChannelConfigSealedColumn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var registered SealedPayloadColumn
			for _, col := range SealedPayloadColumns {
				if col.Table == tc.column.Table {
					registered = col
				}
			}
			require.NotEmpty(t, registered.Table, "sealed registry is missing %s", tc.column.Table)
			assert.Equal(t, registered.Table, tc.column.Table)
			assert.Equal(t, registered.Column, tc.column.Column)
			assert.Equal(t, "id", registered.PKColumn)
			assert.Equal(t, registered.PKColumn, tc.column.PKColumn)

			aad := tc.column.AAD("row-1")
			assert.Equal(t, registered.Table, aad.Table)
			assert.Equal(t, registered.Column, aad.Column)
			assert.Equal(t, "row-1", aad.PKValue)
		})
	}
}

// This is the F2-01c Batch 1 behavior contrast, exercised against the real
// envelope implementation rather than a mock:
//
//   - v1 has no row identity, so the old DecryptField read and the new
//     DecryptColumn read both open it even with a wrong row AAD. Existing rows
//     therefore stay readable, which is why this batch is zero-risk.
//   - v2 inverts that: DecryptField cannot open it at all (ErrAADContext), and
//     DecryptColumn opens it only with the exact row binding (ErrAADMismatch).
func TestSealedPayloadReadPath_V1IgnoresAADAndV2RequiresIt(t *testing.T) {
	enc := newPayloadEncryptor(t)
	rowA := LLMProviderAPIKeySealedColumn.AAD("p-a")
	rowB := LLMProviderAPIKeySealedColumn.AAD("p-b")

	v1, err := enc.EncryptField("sk-legacy")
	require.NoError(t, err)
	require.NotContains(t, v1, "-aad:")

	opened, err := enc.DecryptField(v1)
	require.NoError(t, err)
	assert.Equal(t, "sk-legacy", opened)

	opened, err = enc.DecryptColumn(rowB, v1)
	require.NoError(t, err)
	assert.Equal(t, "sk-legacy", opened)

	v2, err := enc.EncryptColumn(rowA, "sk-bound")
	require.NoError(t, err)
	require.Contains(t, v2, "-aad:")

	_, err = enc.DecryptField(v2)
	require.ErrorIs(t, err, ErrAADContext)

	_, err = enc.DecryptColumn(rowB, v2)
	require.ErrorIs(t, err, ErrAADMismatch)

	opened, err = enc.DecryptColumn(rowA, v2)
	require.NoError(t, err)
	assert.Equal(t, "sk-bound", opened)
}

// A v2 token from one registered column must not be replayable in the other
// column even when both rows share the same primary key value.
func TestSealedPayloadReadPath_V2CrossColumnRejected(t *testing.T) {
	enc := newPayloadEncryptor(t)

	apiKey, err := enc.EncryptColumn(LLMProviderAPIKeySealedColumn.AAD("row-1"), "sk-cross")
	require.NoError(t, err)
	_, err = enc.DecryptColumn(PayChannelConfigSealedColumn.AAD("row-1"), apiKey)
	require.ErrorIs(t, err, ErrAADMismatch)

	channelConfig, err := enc.EncryptColumn(PayChannelConfigSealedColumn.AAD("row-1"), `{"apiKey":"x"}`)
	require.NoError(t, err)
	_, err = enc.DecryptColumn(LLMProviderAPIKeySealedColumn.AAD("row-1"), channelConfig)
	require.ErrorIs(t, err, ErrAADMismatch)
}
