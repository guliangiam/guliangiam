package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

const (
	aadProviderID     = "0190a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b"
	aadOtherRowID     = "0190a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6c"
	plainClientSecret = "client-secret-plain"
	plainAppSecret    = "app-secret-plain"
)

// GM-G7: MySQL mirrors the PostgreSQL AAD口径 — new credential writes are v2
// tokens bound to (table, column, provider ID) and reads authenticate that
// binding without touching the database.
func TestProviderSecrets_SealOpenBoundToRow(t *testing.T) {
	provider := &domain.Provider{
		ID:           aadProviderID,
		ClientSecret: plainClientSecret,
		AppSecret:    plainAppSecret,
	}

	values, err := sealProviderSecrets(provider)
	require.NoError(t, err)
	for _, sealed := range []string{values.clientSecret, values.appSecret} {
		require.True(t, crypto.IsEnvelopeToken(sealed))
		assert.Contains(t, sealed, "-aad:", "new column writes must be v2")
	}

	opened, err := openProviderSecret(provider.ID, providerColumnClientSecret, values.clientSecret)
	require.NoError(t, err)
	assert.Equal(t, plainClientSecret, opened)

	opened, err = openProviderSecret(provider.ID, providerColumnAppSecret, values.appSecret)
	require.NoError(t, err)
	assert.Equal(t, plainAppSecret, opened)

	assert.Equal(t, plainClientSecret, provider.ClientSecret, "sealing must not mutate the domain object")
}

// A ciphertext moved to another row/column must fail and the mapper must not
// swallow the crypto sentinel.
func TestProviderSecrets_CrossRowAndCrossColumnRejected(t *testing.T) {
	provider := &domain.Provider{ID: aadProviderID, ClientSecret: plainClientSecret}
	values, err := sealProviderSecrets(provider)
	require.NoError(t, err)

	_, err = openProviderSecret(aadOtherRowID, providerColumnClientSecret, values.clientSecret)
	require.ErrorIs(t, err, crypto.ErrAADMismatch)

	_, err = openProviderSecret(provider.ID, providerColumnAppSecret, values.clientSecret)
	require.ErrorIs(t, err, crypto.ErrAADMismatch)
}
