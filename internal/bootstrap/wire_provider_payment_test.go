package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	domainpayment "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bootstrapStripeSecret = "sk_test_bootstrap_probe_secret"

func TestNewPaymentConnectionTesterProbesStripe(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		assert.Equal(t, "Bearer "+bootstrapStripeSecret, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tester := newPaymentConnectionTester(server.URL, server.Client())
	err := tester(context.Background(), &domainpayment.PayChannelConfig{Channel: "stripe"}, `{"apiKey":"`+bootstrapStripeSecret+`"}`)
	require.NoError(t, err)
	assert.Equal(t, "/v1/balance", path)
}

func TestNewPaymentConnectionTesterRejectsUnsupportedChannels(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tester := newPaymentConnectionTester(server.URL, server.Client())
	for _, channel := range []string{"alipay", "wechat_pay", "paypal"} {
		err := tester(context.Background(), &domainpayment.PayChannelConfig{Channel: channel}, bootstrapStripeSecret)
		require.Error(t, err, "channel %q must not report connectivity", channel)
		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeUnimplemented, appErr.Code)
	}
	assert.Zero(t, requests, "unsupported channels must not reach the Stripe API")
}

func TestNewPaymentConnectionTesterRedactsStripeRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	tester := newPaymentConnectionTester(server.URL, server.Client())
	err := tester(context.Background(), &domainpayment.PayChannelConfig{Channel: "stripe"}, bootstrapStripeSecret)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), bootstrapStripeSecret)
}

func TestWireProviderConnectivityProbesAttachesTester(t *testing.T) {
	service := appproviders.NewService(nil, nil, nil)
	runtime := &Runtime{Services: AppServices{ProviderService: service}}
	wireProviderConnectivityProbes(runtime)
	require.NotNil(t, runtime.Services.ProviderService.Unified)
	require.NotNil(t, runtime.Services.ProviderService.Unified.PaymentTester)

	// The guard paths must stay panic-free when the composition root is
	// partially assembled (nil runtime or missing unified service).
	wireProviderConnectivityProbes(nil)
	wireProviderConnectivityProbes(&Runtime{})
}
