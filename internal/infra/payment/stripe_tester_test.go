package payment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const stripeTestSecret = "sk_test_stripe_probe_secret"

func stripeTestProvider(channel string) *domain.PayChannelConfig {
	return &domain.PayChannelConfig{ID: "config-1", OrganizationID: "org-1", Channel: channel, Environment: "production"}
}

func newStripeTestTester(t *testing.T, server *httptest.Server) *StripeConnectivityTester {
	t.Helper()
	tester := NewStripeConnectivityTester(server.Client())
	tester.BaseURL = server.URL
	return tester
}

func TestStripeConnectivityTesterProbesBalance(t *testing.T) {
	var method, path, authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, authorization = r.Method, r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"object":"balance","available":[],"pending":[]}`))
	}))
	defer server.Close()

	err := newStripeTestTester(t, server).Probe(context.Background(), stripeTestProvider("stripe"), `{"apiKey":"`+stripeTestSecret+`"}`)
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, method)
	assert.Equal(t, "/v1/balance", path)
	assert.Equal(t, "Bearer "+stripeTestSecret, authorization)
}

func TestStripeConnectivityTesterAcceptsBareAndAliasedKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	tester := newStripeTestTester(t, server)

	for _, plaintext := range []string{
		stripeTestSecret,
		`{"secretKey":"` + stripeTestSecret + `"}`,
		`{"secret_key":"` + stripeTestSecret + `"}`,
	} {
		require.NoError(t, tester.Probe(context.Background(), stripeTestProvider("stripe"), plaintext))
	}
}

func TestStripeConnectivityTesterRejectedCredentialsAreRedacted(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":{"message":"Invalid API Key provided: ` + stripeTestSecret + `"}}`))
			}))
			defer server.Close()

			err := newStripeTestTester(t, server).Probe(context.Background(), stripeTestProvider("stripe"), stripeTestSecret)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "stripe")
			assert.NotContains(t, err.Error(), stripeTestSecret)
			assert.NotContains(t, err.Error(), "Invalid API Key provided")
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Contains(t, []apperrors.Code{apperrors.CodeUnauthorized, apperrors.CodeForbidden}, appErr.Code)
		})
	}
}

func TestStripeConnectivityTesterRateLimitCarriesRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	err := newStripeTestTester(t, server).Probe(context.Background(), stripeTestProvider("stripe"), stripeTestSecret)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeTooManyRequests, appErr.Code)
	retryable, ok := apperrors.IsRetryable(err)
	assert.True(t, ok)
	assert.True(t, retryable)
	seconds, ok := apperrors.RetryAfterSeconds(err)
	assert.True(t, ok)
	assert.Equal(t, 7, seconds)
}

// 解析失败的上游 Retry-After（HTTP-date / 小数 / 空值）必须丢弃：分类仍是
// rate_limited，但不得写任何数值。
func TestStripeConnectivityTesterDropsUnparsableRetryAfter(t *testing.T) {
	for _, header := range []string{"", "1.5", "Wed, 21 Oct 2015 07:28:00 GMT", "0", "-3"} {
		t.Run("header="+header, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if header != "" {
					w.Header().Set("Retry-After", header)
				}
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer server.Close()

			err := newStripeTestTester(t, server).Probe(context.Background(), stripeTestProvider("stripe"), stripeTestSecret)
			require.Error(t, err)
			retryable, ok := apperrors.IsRetryable(err)
			assert.True(t, ok)
			assert.True(t, retryable)
			if _, ok := apperrors.RetryAfterSeconds(err); ok {
				t.Fatalf("unparsable Retry-After %q must be dropped", header)
			}
		})
	}
}

func TestStripeConnectivityTesterUpstreamFailureIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := newStripeTestTester(t, server).Probe(context.Background(), stripeTestProvider("stripe"), stripeTestSecret)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), stripeTestSecret)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnavailable, appErr.Code)
	retryable, ok := apperrors.IsRetryable(err)
	assert.True(t, ok)
	assert.True(t, retryable)
}

func TestStripeConnectivityTesterTimeoutAndNetworkFailure(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		err := newStripeTestTester(t, server).Probe(ctx, stripeTestProvider("stripe"), stripeTestSecret)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), stripeTestSecret)
	})

	t.Run("network error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		baseURL := server.URL
		server.Close()

		tester := NewStripeConnectivityTester(http.DefaultClient)
		tester.BaseURL = baseURL
		err := tester.Probe(context.Background(), stripeTestProvider("stripe"), stripeTestSecret)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), stripeTestSecret)
		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeUnavailable, appErr.Code)
	})
}

func TestStripeConnectivityTesterInvalidConfigFailsBeforeNetwork(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	for _, plaintext := range []string{
		"",
		"   ",
		"not-json-without-key",
		`{"webhookSecret":"whsec_abc"}`,
		`{"apiKey":""}`,
		`{`,
	} {
		err := newStripeTestTester(t, server).Probe(context.Background(), stripeTestProvider("stripe"), plaintext)
		require.Error(t, err, "config %q must be rejected", plaintext)
		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
		assert.NotContains(t, err.Error(), stripeTestSecret)
	}
	assert.Zero(t, requests, "config validation must not reach the Stripe API")
}

func TestStripeConnectivityTesterUnsupportedChannel(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	tester := newStripeTestTester(t, server)

	err := tester.Probe(context.Background(), stripeTestProvider("alipay"), stripeTestSecret)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnimplemented, appErr.Code)
	assert.Contains(t, err.Error(), "alipay")

	err = tester.Probe(context.Background(), nil, stripeTestSecret)
	require.Error(t, err)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnimplemented, appErr.Code)
	assert.Zero(t, requests, "an unsupported channel must not reach the Stripe API")
}

func TestStripeConnectivityTesterRejectsInvalidBaseURL(t *testing.T) {
	tester := NewStripeConnectivityTester(http.DefaultClient)
	tester.BaseURL = "ftp://api.stripe.com"
	err := tester.Probe(context.Background(), stripeTestProvider("stripe"), stripeTestSecret)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}
