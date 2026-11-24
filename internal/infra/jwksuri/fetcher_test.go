package jwksuri

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

func testFetcher(server *httptest.Server) *Fetcher {
	client := ssrf.SafeHTTPClientWithAllowlist(2*time.Second, []string{"127.0.0.1/32"})
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test server
	fetcher := NewFetcher(client)
	fetcher.now = func() time.Time { return time.Unix(1000, 0) }
	return fetcher
}

func TestFetcher_FetchesAndCachesKeys(t *testing.T) {
	var calls int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"kid-1","alg":"RS256","n":"abc","e":"AQAB"}]}`))
	}))
	t.Cleanup(server.Close)
	fetcher := testFetcher(server)

	keys, err := fetcher.Fetch(context.Background(), server.URL+"/jwks.json")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, "kid-1", keys[0].Kid)
	require.Equal(t, "RS256", keys[0].Alg)

	_, err = fetcher.Fetch(context.Background(), server.URL+"/jwks.json")
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls), "second fetch must hit the cache")
}

func TestFetcher_FailsClosed(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/empty":
			_, _ = w.Write([]byte(`{"keys":[]}`))
		case "/large":
			_, _ = w.Write([]byte(`{"keys":[{"kid":"` + strings.Repeat("x", defaultMaxBodySize+1) + `"}]}`))
		}
	}))
	t.Cleanup(server.Close)
	fetcher := testFetcher(server)

	_, err := fetcher.Fetch(context.Background(), server.URL+"/error")
	require.Error(t, err)
	_, err = fetcher.Fetch(context.Background(), server.URL+"/empty")
	require.Error(t, err)
	_, err = fetcher.Fetch(context.Background(), server.URL+"/large")
	require.Error(t, err)
	_, err = fetcher.Fetch(context.Background(), "http://example.com/jwks")
	require.Error(t, err)
}
