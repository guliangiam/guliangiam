package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPKI struct {
	caCert     *x509.Certificate
	caKey      *ecdsa.PrivateKey
	caPEM      []byte
	serverCert tls.Certificate
	clientA    tls.Certificate
	clientB    tls.Certificate
}

func newTestPKI(t *testing.T) *testPKI {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	issue := func(serial int64, cn string, dnsNames []string, ips []net.IP, usage x509.ExtKeyUsage) tls.Certificate {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		template := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: cn},
			DNSNames:     dnsNames,
			IPAddresses:  ips,
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		}
		der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
		require.NoError(t, err)
		return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	}

	return &testPKI{
		caCert:     caCert,
		caKey:      caKey,
		caPEM:      pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		serverCert: issue(2, "server", []string{"example.com"}, []net.IP{net.ParseIP("127.0.0.1")}, x509.ExtKeyUsageServerAuth),
		clientA:    issue(3, "client-a", nil, nil, x509.ExtKeyUsageClientAuth),
		clientB:    issue(4, "client-b", nil, nil, x509.ExtKeyUsageClientAuth),
	}
}

func startMTLSServer(t *testing.T, pki *testPKI) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		thumbprint, err := ThumbprintFromRequest(r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprint(w, thumbprint) //nolint:gosec // G705: test handler returns a plain-text thumbprint, not HTML
	}))
	pool := x509.NewCertPool()
	pool.AddCert(pki.caCert)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{pki.serverCert},
		ClientCAs:    pool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
		MinVersion:   tls.VersionTLS13,
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}

func clientWithCert(t *testing.T, server *httptest.Server, cert tls.Certificate) *http.Client {
	t.Helper()
	transport := server.Client().Transport.(*http.Transport).Clone()
	tlsConfig := server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	tlsConfig.Certificates = []tls.Certificate{cert}
	tlsConfig.MinVersion = tls.VersionTLS13
	transport.TLSClientConfig = tlsConfig
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport}
}

func TestTLSClientCertificateBoundToken(t *testing.T) {
	pki := newTestPKI(t)
	server := startMTLSServer(t, pki)

	response, err := clientWithCert(t, server, pki.clientA).Get(server.URL)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	thumbprintA := string(body)
	require.Len(t, thumbprintA, 43)

	metadata := BindTokenToClientCertificate(nil, thumbprintA)
	require.NoError(t, ValidateCertificateBinding(metadata, thumbprintA))

	// Certificate rotation: the old token must not validate against client B's
	// certificate; the client must re-authorize and obtain a newly bound token.
	thumbprintB, err := Thumbprint(pki.clientB.Leaf)
	if err != nil {
		// Leaf is not populated when parsed from DER by issue(); derive from
		// Certificate[0] instead.
		certB, parseErr := x509.ParseCertificate(pki.clientB.Certificate[0])
		require.NoError(t, parseErr)
		thumbprintB, err = Thumbprint(certB)
		require.NoError(t, err)
	}
	assert.Error(t, ValidateCertificateBinding(metadata, thumbprintB))
}

func TestTLSClientCertificateOptional(t *testing.T) {
	pki := newTestPKI(t)
	server := startMTLSServer(t, pki)

	response, err := server.Client().Get(server.URL)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
}
