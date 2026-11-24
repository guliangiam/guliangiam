package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testProxyHeader = "X-Forwarded-Client-Cert"
	testProxyCIDR   = "198.51.100.0/24"
	testProxyPeer   = "198.51.100.7:8443"
)

func certPEM(t *testing.T, der []byte) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func forwardedRequest(t *testing.T, header, value, remoteAddr string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/oauth2/token", nil)
	req.RemoteAddr = remoteAddr
	req.Header.Set(header, value)
	return req
}

func enableForwardedMode(t *testing.T) {
	t.Helper()
	t.Setenv("MTLS_PROXY_CLIENT_CERT_HEADER", testProxyHeader)
	t.Setenv("TRUSTED_PROXY_CIDRS", testProxyCIDR)
}

func TestPeerCertificatesFromRequest_ForwardedDisabledByDefault(t *testing.T) {
	t.Setenv("MTLS_PROXY_CLIENT_CERT_HEADER", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", testProxyCIDR)

	pki := newTestPKI(t)
	value := `By=spiffe://proxy;Cert="` + url.PathEscape(string(certPEM(t, pki.clientA.Certificate[0]))) + `"`
	req := forwardedRequest(t, testProxyHeader, value, testProxyPeer)

	assert.Empty(t, PeerCertificatesFromRequest(req))
}

func TestPeerCertificatesFromRequest_ForwardedFromUntrustedPeerIgnored(t *testing.T) {
	enableForwardedMode(t)
	pki := newTestPKI(t)
	value := `By=spiffe://proxy;Cert="` + url.PathEscape(string(certPEM(t, pki.clientA.Certificate[0]))) + `"`
	req := forwardedRequest(t, testProxyHeader, value, "203.0.113.9:443")

	assert.Empty(t, PeerCertificatesFromRequest(req))
}

func TestPeerCertificatesFromRequest_ForwardedXFCCFromTrustedProxy(t *testing.T) {
	enableForwardedMode(t)
	pki := newTestPKI(t)
	value := `By=spiffe://proxy;Hash=abc;Cert="` + url.PathEscape(string(certPEM(t, pki.clientA.Certificate[0]))) + `"`
	req := forwardedRequest(t, testProxyHeader, value, testProxyPeer)

	chain := PeerCertificatesFromRequest(req)
	require.Len(t, chain, 1)
	assert.Equal(t, pki.clientA.Certificate[0], chain[0].Raw)
}

func TestPeerCertificatesFromRequest_ForwardedBarePEM(t *testing.T) {
	enableForwardedMode(t)
	pki := newTestPKI(t)
	value := url.PathEscape(string(certPEM(t, pki.clientB.Certificate[0])))
	req := forwardedRequest(t, testProxyHeader, value, testProxyPeer)

	chain := PeerCertificatesFromRequest(req)
	require.Len(t, chain, 1)
	assert.Equal(t, pki.clientB.Certificate[0], chain[0].Raw)
}

// A proxy that appends its element to a client-supplied header must not let the
// client's (earlier) element authenticate. Only the last element is read.
func TestPeerCertificatesFromRequest_LastXFCCElementWins(t *testing.T) {
	enableForwardedMode(t)
	pki := newTestPKI(t)
	forged := `Cert="` + url.PathEscape(string(certPEM(t, pki.clientB.Certificate[0]))) + `"`
	proxy := `By=spiffe://proxy;Cert="` + url.PathEscape(string(certPEM(t, pki.clientA.Certificate[0]))) + `"`
	req := forwardedRequest(t, testProxyHeader, forged+","+proxy, testProxyPeer)

	chain := PeerCertificatesFromRequest(req)
	require.Len(t, chain, 1)
	assert.Equal(t, pki.clientA.Certificate[0], chain[0].Raw)
}

// A malformed chain forwarded by the trusted proxy must not fall back to the
// proxy's own in-process TLS certificate: that certificate belongs to the proxy,
// not to the end client.
func TestPeerCertificatesFromRequest_MalformedForwardedHeaderFailsClosed(t *testing.T) {
	enableForwardedMode(t)
	req := forwardedRequest(t, testProxyHeader, `By=spiffe://proxy;Cert="not-a-certificate"`, testProxyPeer)
	req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{{Raw: []byte("proxy-own-cert")}}}

	assert.Empty(t, PeerCertificatesFromRequest(req))
}

func TestPeerCertificatesFromRequest_OversizedForwardedHeaderFailsClosed(t *testing.T) {
	enableForwardedMode(t)
	req := forwardedRequest(t, testProxyHeader, strings.Repeat("A", maxForwardedChainBytes+1), testProxyPeer)

	assert.Empty(t, PeerCertificatesFromRequest(req))
}

func TestPeerCertificatesFromRequest_TrustedProxyWithoutHeaderFallsBackToTLS(t *testing.T) {
	enableForwardedMode(t)
	peer := &x509.Certificate{Raw: []byte("tls-peer")}
	req := httptest.NewRequest(http.MethodPost, "/oauth2/token", nil)
	req.RemoteAddr = testProxyPeer
	req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}}

	chain := PeerCertificatesFromRequest(req)
	require.Len(t, chain, 1)
	assert.Same(t, peer, chain[0])
}

func TestThumbprintFromRequest_UsesForwardedChain(t *testing.T) {
	enableForwardedMode(t)
	pki := newTestPKI(t)
	leaf, err := x509.ParseCertificate(pki.clientA.Certificate[0])
	require.NoError(t, err)
	expected, err := Thumbprint(leaf)
	require.NoError(t, err)

	value := `Cert="` + url.PathEscape(string(certPEM(t, pki.clientA.Certificate[0]))) + `"`
	req := forwardedRequest(t, testProxyHeader, value, testProxyPeer)

	thumbprint, err := ThumbprintFromRequest(req)
	require.NoError(t, err)
	assert.Equal(t, expected, thumbprint)
}
