package scim

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSCIMClientBlocksPrivateTargetByDefault(t *testing.T) {
	t.Setenv("SCIM_ALLOW_PRIVATE_TARGETS", "false")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", 1000, 0)
	_, _, err := client.Do(context.Background(), http.MethodGet, "/Users", nil)
	require.Error(t, err, "loopback target must be rejected by the SSRF-safe transport")
}

func TestSCIMClientAllowsPrivateTargetWhenOptedIn(t *testing.T) {
	t.Setenv("SCIM_ALLOW_PRIVATE_TARGETS", "true")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", 1000, 0)
	status, _, err := client.Do(context.Background(), http.MethodGet, "/Users", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
}

func TestSCIMClientRejectsOversizedResponseBody(t *testing.T) {
	t.Setenv("SCIM_ALLOW_PRIVATE_TARGETS", "true")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, maxResponseBodyBytes+1))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", 1000, 0)
	status, body, err := client.Do(context.Background(), http.MethodGet, "/Users", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
	assert.Equal(t, http.StatusOK, status)
	assert.Nil(t, body)
}
