package providers

import (
	"context"
	"errors"
	"testing"

	domainpayment "github.com/guliangiam/guliangiam/internal/domain/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const paymentTestSecret = "sk_test_payment_probe_secret"

type staticPaymentCipher struct {
	plaintext  string
	decryptErr error
}

func (c staticPaymentCipher) EncryptField(plaintext string) (string, error) {
	return "sealed:" + plaintext, nil
}

func (c staticPaymentCipher) DecryptField(ciphertext string) (string, error) {
	if c.decryptErr != nil {
		return "", c.decryptErr
	}
	if ciphertext == "" {
		return "", errors.New("empty ciphertext")
	}
	return c.plaintext, nil
}

func (c staticPaymentCipher) DecryptColumn(_ crypto.AADContext, ciphertext string) (string, error) {
	return c.DecryptField(ciphertext)
}

func (c staticPaymentCipher) CurrentKeyVersion() int { return 1 }

func paymentTestProvider(channel, sealed string) *Provider {
	return &Provider{
		Registration: domain.Registration{
			ID: "provider-1", OrganizationID: "org-1", Family: domain.FamilyPayment,
		},
		Detail: &domainpayment.PayChannelConfig{
			ID: "config-1", OrganizationID: "org-1", Channel: channel,
			Environment: "production", ConfigSealed: sealed,
		},
	}
}

// The composition root owns the real probe. A strategy without one must not
// report success after a config-only check; it has to say the probe is missing.
func TestPaymentStrategyTestConnectionRequiresRealProbe(t *testing.T) {
	strategy := &paymentStrategy{cipher: func() FieldCipher {
		return staticPaymentCipher{plaintext: paymentTestSecret}
	}}
	result, err := strategy.TestConnection(context.Background(), paymentTestProvider("stripe", "sealed-config"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Connected)
	assert.Contains(t, result.Error, "real payment connectivity probe is not configured")
	assert.NotContains(t, result.Error, paymentTestSecret)
}

func TestPaymentStrategyTestConnectionClassifiesProbeOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		testerErr  error
		wantConn   bool
		wantPhrase string
	}{
		{name: "real probe succeeds", testerErr: nil, wantConn: true},
		{name: "probe unavailable", testerErr: apperrors.NewUnimplemented("probe not implemented"), wantPhrase: "not supported"},
		{name: "config invalid", testerErr: apperrors.NewInvalidArgument("apiKey is required"), wantPhrase: "config verification failed"},
		{name: "probe failed", testerErr: apperrors.NewUnavailable("upstream unreachable"), wantPhrase: "connectivity probe failed"},
		{name: "probe failed with opaque error", testerErr: errors.New("boom " + paymentTestSecret), wantPhrase: "connectivity probe failed"},
		{name: "classified non-retryable", testerErr: apperrors.WithRetryable(apperrors.NewUnauthorized("api key rejected"), false), wantPhrase: "connectivity probe failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPlaintext string
			strategy := &paymentStrategy{
				cipher: func() FieldCipher {
					return staticPaymentCipher{plaintext: paymentTestSecret}
				},
				tester: func(_ context.Context, _ *domainpayment.PayChannelConfig, plaintext string) error {
					gotPlaintext = plaintext
					return tt.testerErr
				},
			}
			result, err := strategy.TestConnection(context.Background(), paymentTestProvider("stripe", "sealed-config"))
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantConn, result.Connected)
			assert.Equal(t, paymentTestSecret, gotPlaintext)
			assert.NotContains(t, result.Error, paymentTestSecret)
			if tt.wantPhrase != "" {
				assert.Contains(t, result.Error, tt.wantPhrase)
			}
			if tt.wantConn {
				assert.Empty(t, result.Error)
			}
		})
	}
}

// Retryable upstream failures (429 / 5xx / timeout) are raised to the
// management face so route.WriteError can answer with error.meta.retryable and
// the upstream Retry-After. Non-retryable outcomes keep the connected=false
// envelope.
func TestPaymentStrategyTestConnectionPropagatesRetryableProbeFailure(t *testing.T) {
	rateLimited := apperrors.WithRetryAfterSeconds(
		apperrors.WithRetryable(apperrors.NewTooManyRequests("stripe rate limited"), true), 5)
	strategy := &paymentStrategy{
		cipher: func() FieldCipher { return staticPaymentCipher{plaintext: paymentTestSecret} },
		tester: func(context.Context, *domainpayment.PayChannelConfig, string) error {
			return rateLimited
		},
	}

	result, err := strategy.TestConnection(context.Background(), paymentTestProvider("stripe", "sealed-config"))
	require.Error(t, err)
	assert.Nil(t, result)
	assert.NotContains(t, err.Error(), paymentTestSecret)

	seconds, ok := apperrors.RetryAfterSeconds(err)
	assert.True(t, ok)
	assert.Equal(t, 5, seconds)
	retryable, ok := apperrors.IsRetryable(err)
	assert.True(t, ok)
	assert.True(t, retryable)
}

func TestPaymentStrategyTestConnectionMissingConfigIsConfigFailure(t *testing.T) {
	tests := []struct {
		name           string
		sealed         string
		cipher         FieldCipher
		wantConfigFail bool
	}{
		{
			name:           "empty sealed config",
			sealed:         "",
			cipher:         staticPaymentCipher{plaintext: paymentTestSecret},
			wantConfigFail: true,
		},
		{
			name:           "undecryptable sealed config",
			sealed:         "sealed-config",
			cipher:         staticPaymentCipher{decryptErr: errors.New("decrypt failed")},
			wantConfigFail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			strategy := &paymentStrategy{
				cipher: func() FieldCipher { return tt.cipher },
				tester: func(context.Context, *domainpayment.PayChannelConfig, string) error {
					called = true
					return nil
				},
			}
			result, err := strategy.TestConnection(context.Background(), paymentTestProvider("stripe", tt.sealed))
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.False(t, result.Connected)
			assert.Contains(t, result.Error, "config verification failed")
			assert.False(t, called, "a missing config must fail before any network probe")
		})
	}
}
