package providers

import (
	"errors"
	"testing"

	domainpayment "github.com/guliangiam/guliangiam/internal/domain/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func paymentProviderWithDetailID(detailID, sealed string) *Provider {
	return &Provider{
		Registration: domain.Registration{ID: "provider-1", OrganizationID: "org-1", Family: domain.FamilyPayment},
		Detail: &domainpayment.PayChannelConfig{
			ID: detailID, OrganizationID: "org-1", Channel: "stripe",
			Environment: "production", ConfigSealed: sealed,
		},
	}
}

// The stored-row read uses the channel-config row AAD. v1/v0 values remain
// readable (v1 ignores the binding), while a v2 token moved to another row or
// column fails with ErrAADMismatch.
func TestPaymentConfigFromSealed_ReadsWithRowAAD(t *testing.T) {
	enc, err := crypto.NewFieldEncryptorChecked()
	require.NoError(t, err)

	const config = `{"apiKey":"sk-test"}`
	v1, err := enc.EncryptField(config)
	require.NoError(t, err)
	require.NotContains(t, v1, "-aad:")

	// Existing v1 rows stay readable; v1 carries no row identity, so a moved
	// token still opens. That is the compatibility property of Batch 1 and the
	// gap Batch 3's v2-only read closes.
	for _, id := range []string{"config-1", "config-2"} {
		plaintext, err := paymentConfigFromSealed(enc, paymentProviderWithDetailID(id, v1))
		require.NoError(t, err)
		assert.Equal(t, config, plaintext)
	}

	v2, err := enc.EncryptColumn(crypto.PayChannelConfigSealedColumn.AAD("config-1"), config)
	require.NoError(t, err)
	require.Contains(t, v2, "-aad:")

	plaintext, err := paymentConfigFromSealed(enc, paymentProviderWithDetailID("config-1", v2))
	require.NoError(t, err)
	assert.Equal(t, config, plaintext)

	_, err = paymentConfigFromSealed(enc, paymentProviderWithDetailID("config-2", v2))
	require.ErrorIs(t, err, crypto.ErrAADMismatch)

	crossed, err := enc.EncryptColumn(crypto.LLMProviderAPIKeySealedColumn.AAD("config-1"), config)
	require.NoError(t, err)
	_, err = paymentConfigFromSealed(enc, paymentProviderWithDetailID("config-1", crossed))
	require.ErrorIs(t, err, crypto.ErrAADMismatch)

	// The pre-Batch-1 unbound reader cannot open v2 at all.
	_, err = enc.DecryptField(v2)
	require.ErrorIs(t, err, crypto.ErrAADContext)
}

type recordingPaymentCipher struct {
	calls int
	aad   crypto.AADContext
}

func (c *recordingPaymentCipher) EncryptField(plaintext string) (string, error) {
	return "sealed:" + plaintext, nil
}

func (c *recordingPaymentCipher) DecryptField(string) (string, error) {
	return "", errors.New("the unbound read must not be used for a stored row")
}

func (c *recordingPaymentCipher) DecryptColumn(aad crypto.AADContext, _ string) (string, error) {
	c.calls++
	c.aad = aad
	return "plain", nil
}

func (c *recordingPaymentCipher) CurrentKeyVersion() int { return 1 }

// paymentConfigFromSealed must call DecryptColumn with the registered
// (table, column, detail.ID) binding and must never touch DecryptField.
func TestPaymentConfigFromSealed_UsesRegisteredRowAAD(t *testing.T) {
	cipher := &recordingPaymentCipher{}
	plaintext, err := paymentConfigFromSealed(cipher, paymentProviderWithDetailID("config-1", "sealed"))
	require.NoError(t, err)
	assert.Equal(t, "plain", plaintext)
	assert.Equal(t, 1, cipher.calls)
	assert.Equal(t, crypto.AADContext{
		Table: "guliangiam.pay_channel_configs", Column: "config_sealed", PKValue: "config-1",
	}, cipher.aad)
}
