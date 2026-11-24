package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/requestmeta"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// runRequestMetadata mounts the middleware behind a route metadata stamp, the
// same order the router uses (route first, requestmeta second).
func runRequestMetadata(t *testing.T, request *http.Request) requestmeta.Metadata {
	t.Helper()
	request = request.WithContext(route.WithRequestMetadata(request.Context(), route.RequestMetadata{
		Surface: route.SurfaceProtocol, RequestID: "req-1", TraceID: "trace-1",
	}))
	var captured requestmeta.Metadata
	handler := RequestMetadataMiddleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = requestmeta.From(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	return captured
}

func requestMetadataRequest(remoteAddr, userAgent string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/oauth2/token", http.NoBody)
	request.RemoteAddr = remoteAddr
	request.Header.Set("User-Agent", userAgent)
	return request
}

// A request from an untrusted peer resolves to the socket address: a
// client-supplied X-Forwarded-For must not be believed.
func TestRequestMetadataMiddleware_UntrustedPeerUsesRemoteAddr(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,192.168.0.0/16")
	request := requestMetadataRequest("198.51.100.10:1234", "agent/1.0")
	request.Header.Set("X-Forwarded-For", "203.0.113.7")

	meta := runRequestMetadata(t, request)
	assert.Equal(t, "198.51.100.10", meta.IP)
}

// A request that carries no forwarding headers resolves to the socket address.
func TestRequestMetadataMiddleware_RemoteAddrOnly(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,192.168.0.0/16")
	meta := runRequestMetadata(t, requestMetadataRequest("203.0.113.10:5678", "agent/2.0"))

	assert.Equal(t, "203.0.113.10", meta.IP)
	assert.Equal(t, "agent/2.0", meta.UserAgent)
	assert.Equal(t, "protocol", meta.Surface)
	assert.Equal(t, "req-1", meta.RequestID)
	assert.Equal(t, "trace-1", meta.TraceID)
}

// A request forwarded by a whitelisted proxy keeps the client address from the
// header after the trusted hops are stripped from the right.
func TestRequestMetadataMiddleware_TrustedProxyResolvesForwardedClient(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,192.168.0.0/16")
	request := requestMetadataRequest("192.168.1.10:1234", "agent/3.0")
	request.Header.Set("X-Forwarded-For", "203.0.113.7, 192.168.1.5")

	meta := runRequestMetadata(t, request)
	assert.Equal(t, "203.0.113.7", meta.IP)
}

// Without the route metadata stamp the middleware still captures the origin
// evidence; route.RequestMetadataFrom falls back to the management surface and
// empty correlation ids (matching the DDL defaults).
func TestRequestMetadataMiddleware_WithoutRouteMetadata(t *testing.T) {
	var captured requestmeta.Metadata
	handler := RequestMetadataMiddleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = requestmeta.From(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), requestMetadataRequest("203.0.113.10:5678", "agent/4.0"))

	require.NotEmpty(t, captured.IP)
	assert.Equal(t, "203.0.113.10", captured.IP)
	assert.Equal(t, "agent/4.0", captured.UserAgent)
	assert.Equal(t, "management", captured.Surface)
	assert.Empty(t, captured.RequestID)
}

// records.user_agent is VARCHAR(255): an over-long User-Agent must be cut to
// the column width here, otherwise the audit INSERT fails and takes the
// business transaction down with it.
func TestRequestMetadataMiddleware_TruncatesLongASCIIUserAgent(t *testing.T) {
	meta := runRequestMetadata(t, requestMetadataRequest("203.0.113.10:5678", strings.Repeat("a", 300)))

	assert.Equal(t, 255, utf8.RuneCountInString(meta.UserAgent))
	assert.Equal(t, strings.Repeat("a", 255), meta.UserAgent)
}

// Multi-byte truncation is counted in runes, not bytes: the stored value must
// stay at most 255 characters and must never end in half a character.
func TestRequestMetadataMiddleware_TruncatesMultiByteUserAgentByRune(t *testing.T) {
	userAgent := strings.Repeat("浏览器", 100) + strings.Repeat("🚀", 100)
	meta := runRequestMetadata(t, requestMetadataRequest("203.0.113.10:5678", userAgent))

	require.LessOrEqual(t, utf8.RuneCountInString(meta.UserAgent), 255)
	assert.Equal(t, 255, utf8.RuneCountInString(meta.UserAgent))
	assert.True(t, utf8.ValidString(meta.UserAgent), "truncation must not split a multi-byte character")
	assert.Equal(t, []rune(userAgent)[:255], []rune(meta.UserAgent))
}

// X-Request-Id is client-supplied as well and lands in records.request_id
// VARCHAR(255): it is truncated while the response envelope keeps the id chi
// generated, so correlation in logs stays intact.
func TestRequestMetadataMiddleware_TruncatesLongRequestID(t *testing.T) {
	request := requestMetadataRequest("203.0.113.10:5678", "agent/5.0")
	request = request.WithContext(route.WithRequestMetadata(request.Context(), route.RequestMetadata{
		Surface:   route.SurfaceProtocol,
		RequestID: strings.Repeat("r", 400),
		TraceID:   "trace-1",
	}))

	var captured requestmeta.Metadata
	handler := RequestMetadataMiddleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = requestmeta.From(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)

	assert.Equal(t, 255, utf8.RuneCountInString(captured.RequestID))
	assert.Equal(t, strings.Repeat("r", 255), captured.RequestID)
	assert.Equal(t, "trace-1", captured.TraceID)
}
