package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainproviders "github.com/guliangiam/guliangiam/internal/domain/providers"
)

type captureState struct {
	path, host, auth string
}

// newCapture points providers at a local HTTP server and records the request
// shape the S3-compatible endpoint actually received (no real cloud involved).
func newCapture(t *testing.T) (*captureState, *httptest.Server) {
	t.Helper()
	state := &captureState{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.path = r.URL.Path
		state.host = r.Host
		state.auth = r.Header.Get("Authorization")
		w.Header().Set("ETag", `"p7-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return state, server
}

func TestP7MinIOUsesPathStyleAndSigV4(t *testing.T) {
	state, server := newCapture(t)
	p, err := NewMinIOStorage(context.Background(), "minioadmin", "secret", "", server.URL, "test-bucket")
	require.NoError(t, err)
	require.NoError(t, p.Put(context.Background(), "docs/a.txt", []byte("p7"), "text/plain"))

	// MinIO 默认 path-style：bucket 必须出现在请求路径而非 Host 子域。
	assert.Equal(t, "/test-bucket/docs/a.txt", state.path)
	assert.False(t, strings.HasPrefix(state.host, "test-bucket."), "MinIO must not use virtual-host addressing")
	assert.True(t, strings.HasPrefix(state.auth, "AWS4-HMAC-SHA256"), "SigV4 Authorization expected")
	assert.Contains(t, state.auth, "/us-east-1/s3/aws4_request")
}

func TestP7CloudflareR2RequiresEndpoint(t *testing.T) {
	_, err := NewCloudflareR2Storage(context.Background(), "ak", "sk", "auto", "", "bucket")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "r2")
}

func TestP7CloudflareR2AutoRegionSigV4(t *testing.T) {
	state, server := newCapture(t)
	p, err := NewCloudflareR2Storage(context.Background(), "ak", "sk", "", server.URL, "test-bucket")
	require.NoError(t, err)
	require.NoError(t, p.Put(context.Background(), "docs/a.txt", []byte("p7"), "text/plain"))

	// R2 文档：region 固定 auto 参与 SigV4 scope。httptest 端点为 IP 主机，
	// SDK 自动回退 path-style（真实 R2 端点为域名时走 bucket-style，两者均被 R2 支持）。
	assert.Contains(t, state.auth, "/auto/s3/aws4_request")
	assert.Equal(t, "/test-bucket/docs/a.txt", state.path)
}

func TestP7BackblazeB2EndpointDerivation(t *testing.T) {
	assert.Equal(t, "https://s3.eu00.backblazeb2.com", b2Endpoint("EU00"))

	_, err := NewBackblazeB2Storage(context.Background(), "ak", "sk", "", "", "bucket")
	require.Error(t, err)

	state, server := newCapture(t)
	p, err := NewBackblazeB2Storage(context.Background(), "ak", "sk", "us-west-004", server.URL, "test-bucket")
	require.NoError(t, err)
	require.NoError(t, p.Put(context.Background(), "docs/a.txt", []byte("p7"), "text/plain"))
	assert.Contains(t, state.auth, "/us-west-004/s3/aws4_request", "B2 region name must enter the SigV4 scope")
}

func TestP7DigitalOceanSpacesEndpointDerivation(t *testing.T) {
	assert.Equal(t, "https://nyc3.digitaloceanspaces.com", spacesEndpoint("NYC3"))

	_, err := NewDigitalOceanSpacesStorage(context.Background(), "ak", "sk", "", "", "bucket")
	require.Error(t, err)

	state, server := newCapture(t)
	p, err := NewDigitalOceanSpacesStorage(context.Background(), "ak", "sk", "sgp1", server.URL, "test-bucket")
	require.NoError(t, err)
	require.NoError(t, p.Put(context.Background(), "docs/a.txt", []byte("p7"), "text/plain"))
	assert.Contains(t, state.auth, "/sgp1/s3/aws4_request")
}

func TestP7ResolverBuildsNewStorageKinds(t *testing.T) {
	_, server := newCapture(t)
	for _, providerType := range []domainproviders.ProviderType{
		domainproviders.ProviderCloudflareR2,
		domainproviders.ProviderBackblazeB2,
		domainproviders.ProviderDigitalOceanSpaces,
	} {
		resolver := NewProviderResolver(stubProviderLister{rows: []*domainproviders.Provider{{
			ID: "p1", Revision: 1, Enabled: true,
			ProviderType: providerType, Category: domainproviders.CategoryStorage,
			AppId: "bucket", Region: "eu00", Host: server.URL, Admin: "ak", Password: "sk",
		}}})
		provider, err := resolver.Resolve(context.Background(), "org-1")
		require.NoError(t, err, "kind %s", providerType)
		require.NotNil(t, provider, "kind %s", providerType)
	}
}
