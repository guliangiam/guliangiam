package aiupstream

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transportRoundTripper func(*http.Request) (*http.Response, error)

func (f transportRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Header-auth adapters must not render the *url.Error either: the URL is not
// where the key lives, but it can still leak internal endpoints, and the same
// sanitizer keeps the timeout/cancel chain intact.
func TestOpenAICompatAdapter_TransportError_RedactsURL(t *testing.T) {
	const (
		baseURL = "https://internal.example.invalid/v1"
		apiKey  = "openai-key-secret"
	)
	adapter := NewOpenAICompat(baseURL, apiKey, time.Second)
	adapter.httpClient = &http.Client{Transport: transportRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: connection refused")
	})}

	_, err := adapter.Chat(context.Background(), baseCanonicalRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnavailable, ae.Code)
	assert.Contains(t, err.Error(), "transport failure")
	assert.NotContains(t, err.Error(), apiKey)
	assert.NotContains(t, err.Error(), "internal.example.invalid")
	assert.NotContains(t, err.Error(), "/chat/completions")
}

func TestAnthropicMessagesAdapter_TransportError_RedactsURL(t *testing.T) {
	const (
		baseURL = "https://internal.example.invalid"
		apiKey  = "anthropic-key-secret"
	)
	adapter := NewAnthropicMessages(baseURL, apiKey, time.Second)
	adapter.httpClient = &http.Client{Transport: transportRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: connection refused")
	})}

	_, err := adapter.ChatStream(context.Background(), baseAnthropicRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnavailable, ae.Code)
	assert.Contains(t, err.Error(), "transport failure")
	assert.NotContains(t, err.Error(), apiKey)
	assert.NotContains(t, err.Error(), "internal.example.invalid")
	assert.NotContains(t, err.Error(), "/v1/messages")
}

// Header-auth upstreams can reflect the Authorization / x-api-key header into
// their error body. The body is classified by status only and never echoed.
func TestOpenAICompatAdapter_UpstreamBodyDoesNotEchoCredential(t *testing.T) {
	const apiKey = "test-key-xyz" // matches newOpenAIFixture
	fx := newOpenAIFixture(t, map[string]http.HandlerFunc{
		"/chat/completions": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid api key: ` + apiKey + `","type":"authentication_error"}}`))
		},
	})

	_, err := fx.adapter.Chat(context.Background(), baseCanonicalRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnauthorized, ae.Code)
	assert.Contains(t, err.Error(), "upstream 401")
	assert.NotContains(t, err.Error(), apiKey)
	assert.NotContains(t, err.Error(), "invalid api key")
}

func TestAnthropicMessagesAdapter_UpstreamBodyDoesNotEchoCredential(t *testing.T) {
	const apiKey = "anthropic-key-xyz" // matches newAnthropicFixture
	a, _ := newAnthropicFixture(t, map[string]http.HandlerFunc{
		"/v1/messages": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid x-api-key: ` + apiKey + `","type":"authentication_error"}}`))
		},
	})

	_, err := a.Chat(context.Background(), baseAnthropicRequest())
	require.Error(t, err)
	var ae *apperrors.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperrors.CodeUnauthorized, ae.Code)
	assert.Contains(t, err.Error(), "upstream 401")
	assert.NotContains(t, err.Error(), apiKey)
	assert.NotContains(t, err.Error(), "invalid x-api-key")
}
