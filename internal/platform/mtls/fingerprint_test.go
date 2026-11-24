package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func selfSignedCert(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "mtls-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

func TestThumbprint(t *testing.T) {
	cert := selfSignedCert(t)
	got, err := Thumbprint(cert)
	require.NoError(t, err)
	assert.Len(t, got, 43) // base64url SHA-256 without padding
	again, err := Thumbprint(cert)
	require.NoError(t, err)
	assert.Equal(t, got, again, "thumbprint must be deterministic")
}

func TestThumbprintFromRequest(t *testing.T) {
	cert := selfSignedCert(t)
	r := &http.Request{TLS: &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}}
	got, err := ThumbprintFromRequest(r)
	require.NoError(t, err)
	assert.Len(t, got, 43)

	_, err = ThumbprintFromRequest(&http.Request{})
	assert.ErrorIs(t, err, ErrNoPeerCertificate)
	_, err = Thumbprint(nil)
	assert.ErrorIs(t, err, ErrNoPeerCertificate)
}

func TestBindTokenToClientCertificate(t *testing.T) {
	metadata := map[string]any{"cnf": map[string]any{"jkt": "dpop-jkt"}}
	bound := BindTokenToClientCertificate(metadata, "x5t-thumbprint")
	cnf, ok := bound["cnf"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "x5t-thumbprint", cnf["x5t#S256"])
	assert.Equal(t, "dpop-jkt", cnf["jkt"], "existing DPoP confirmation must be preserved")
	assert.Equal(t, "mTLS", bound["tokenBinding"])
	assert.Equal(t, "x5t-thumbprint", ClientCertificateThumbprintFromMetadata(bound))
}

func TestClientCertificateThumbprintFromMetadata_Unbound(t *testing.T) {
	assert.Empty(t, ClientCertificateThumbprintFromMetadata(nil))
	assert.Empty(t, ClientCertificateThumbprintFromMetadata(map[string]any{"cnf": map[string]any{}}))
}
