package faceid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalUniFaceVerifier_CompareFace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/compare", r.URL.Path)
		assert.Equal(t, "Bearer local-key", r.Header.Get("Authorization"))

		var body localUniFaceCompareRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "image-a", body.ImageA)
		assert.Equal(t, "image-b", body.ImageB)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"matched":true,"score":0.92}`))
	}))
	defer server.Close()

	verifier := NewLocalUniFaceVerifier(server.URL+"/", "local-key")
	verifier.HTTPClient = server.Client()

	result, err := verifier.CompareFace(context.Background(), "image-a", "image-b")
	require.NoError(t, err)
	assert.True(t, result.IsMatch)
	assert.InDelta(t, 0.92, result.Confidence, 0.001)
}

func TestLocalUniFaceVerifier_CompareFaceRejectsErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"model unavailable"}`))
	}))
	defer server.Close()

	verifier := NewLocalUniFaceVerifier(server.URL, "")
	verifier.HTTPClient = server.Client()

	_, err := verifier.CompareFace(context.Background(), "a", "b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestLocalUniFaceVerifier_EmptyEndpoint(t *testing.T) {
	verifier := NewLocalUniFaceVerifier("", "")
	_, err := verifier.CompareFace(context.Background(), "a", "b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint is empty")
}

func TestLocalUniFace_VerifyLivenessFailsClosed(t *testing.T) {
	verifier := NewLocalUniFaceVerifier("http://127.0.0.1:8100", "")
	result, err := verifier.VerifyLiveness(context.Background(), "image", "device")
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "does not implement liveness detection")
}
