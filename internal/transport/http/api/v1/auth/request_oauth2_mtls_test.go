package auth

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPeerCertificatesFromRequest_IgnoresProxyHeaders pins the default trust
// boundary: without an explicit trusted-proxy configuration, only the
// in-process TLS connection state may authenticate an mTLS client. A client can
// forge any certificate header when TLS terminates upstream, so the default
// must never turn one into a credential.
func TestPeerCertificatesFromRequest_IgnoresProxyHeaders(t *testing.T) {
	t.Setenv("MTLS_PROXY_CLIENT_CERT_HEADER", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "")

	req := httptest.NewRequest(http.MethodPost, "/tokens/client-credentials", nil)
	for _, header := range []string{
		"X-Client-Cert",
		"X-Client-Certificate",
		"X-Forwarded-Client-Cert",
		"X-SSL-Client-Cert",
		"X-SSL-Client-Verify",
		"X-SSL-Client-S-DN",
	} {
		req.Header.Set(header, "forged-certificate-material")
	}

	assert.Empty(t, platformmtls.PeerCertificatesFromRequest(req), "proxy headers must not become a client certificate")

	cmd := ClientAuthRequest{ClientID: "gl_cid_forged"}.toCommand(req)
	assert.Empty(t, cmd.PeerCertificates)
	assert.Empty(t, cmd.ClientSecret)
	assert.Empty(t, cmd.ClientAssertion)
}

// The same helper forwards the certificate actually negotiated during the
// in-process TLS handshake.
func TestPeerCertificatesFromRequest_ReadsTLSState(t *testing.T) {
	t.Setenv("MTLS_PROXY_CLIENT_CERT_HEADER", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "")

	peer := &x509.Certificate{Raw: []byte("der-from-tls-state")}
	req := httptest.NewRequest(http.MethodPost, "/tokens/client-credentials", nil)
	req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}}

	certificates := platformmtls.PeerCertificatesFromRequest(req)
	require.Len(t, certificates, 1)
	assert.Same(t, peer, certificates[0])

	cmd := ClientAuthRequest{ClientID: "gl_cid_mtls"}.toCommand(req)
	require.Len(t, cmd.PeerCertificates, 1)
	assert.Same(t, peer, cmd.PeerCertificates[0])
}

func TestPeerCertificatesFromRequest_NilRequest(t *testing.T) {
	assert.Empty(t, platformmtls.PeerCertificatesFromRequest(nil))
}
