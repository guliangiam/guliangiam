package providers

import (
	"strings"
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

// GM-G7: the provider write path seals every credential column as a v2 token
// bound to (table, column, provider ID), and the read path opens it with the
// same AAD. This exercises the mapper's seal/open pair without a database.
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

// GM-G7 acceptance: a ciphertext moved to another row or column must fail with
// the crypto sentinel, and the mapper must surface it instead of returning an
// empty value or falling back to v1.
func TestProviderSecrets_CrossRowAndCrossColumnRejected(t *testing.T) {
	provider := &domain.Provider{ID: aadProviderID, ClientSecret: plainClientSecret}
	values, err := sealProviderSecrets(provider)
	require.NoError(t, err)

	_, err = openProviderSecret(aadOtherRowID, providerColumnClientSecret, values.clientSecret)
	require.ErrorIs(t, err, crypto.ErrAADMismatch)

	_, err = openProviderSecret(provider.ID, providerColumnAppSecret, values.clientSecret)
	require.ErrorIs(t, err, crypto.ErrAADMismatch)

	require.False(t, strings.HasPrefix(values.clientSecret, "v1-aes:"), "v1 must not be written for credentials")
}
